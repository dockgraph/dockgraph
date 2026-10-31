//go:build linux

package collector

import (
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Limits a compose parse child holds itself to. composeParseMemory bounds
// the memory it maps (RLIMIT_DATA, which counts the Go heap, and some 75
// MiB the runtime maps at start but does not use): mapped in all, a parse
// of a realistic compose file of 1 MiB, 520 services, takes about 130 MB,
// one of 1 MiB of the tiniest YAML nodes about 200 MB. Beyond a parse that
// runs away, only port ranges reach it: the loader turns each published port
// into structures of its own, some 2 KB, so a file of two services
// publishing all 65535 ports fits (the graph keeps ranges as ranges, see
// PortRanges) and one of three does not. Below it the soft limit makes the
// runtime collect sooner, keeping the child's share of the memory DockGraph
// is given low.
// composeParseCPU bounds its CPU time should the parent's deadline not
// stop it.
const (
	composeParseMemory     = 256 << 20
	composeParseSoftMemory = 128 << 20
	composeParseCPU        = 60 // seconds: two cores for the parent's deadline
	composeParseOpenFiles  = 64
)

// composeChildAttr returns how a parse child is started: killed should
// DockGraph end first, since outside a container nothing else would end a
// child waiting on a read.
func composeChildAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
}

// restrictComposeParse holds the process to the limits above, lets it read
// nothing outside roots (with Landlock, where the kernel and the binary
// allow it), and returns how it is restricted, for the parent to log.
func restrictComposeParse(roots []string) string {
	var notes []string
	// Should DockGraph's container run out of memory all the same, the
	// kernel ends the child rather than DockGraph.
	if err := os.WriteFile("/proc/self/oom_score_adj", []byte("1000"), 0); err != nil {
		notes = append(notes, fmt.Sprintf("OOM score not raised: %v", err))
	}
	debug.SetMemoryLimit(composeParseSoftMemory)
	limits := []struct {
		resource int
		value    uint64
	}{
		{unix.RLIMIT_CPU, composeParseCPU},
		{unix.RLIMIT_FSIZE, 0}, // the child writes no file, only to its pipes
		{unix.RLIMIT_CORE, 0},
	}
	if raceBuild {
		notes = append(notes, "no memory limit in a race detector build")
	} else {
		limits = append(limits, struct {
			resource int
			value    uint64
		}{unix.RLIMIT_DATA, composeParseMemory})
		notes = append(notes, fmt.Sprintf("memory %d MiB", composeParseMemory>>20))
	}
	for _, l := range limits {
		if err := unix.Setrlimit(l.resource, &unix.Rlimit{Cur: l.value, Max: l.value}); err != nil {
			notes = append(notes, fmt.Sprintf("limit %d not set: %v", l.resource, err))
		}
	}

	switch abi, err := landlockConfine(roots); {
	case err == nil:
		notes = append(notes, fmt.Sprintf("reads limited to the compose paths (Landlock ABI %d)", abi))
	case errors.Is(err, syscall.ENOTSUP) && abi > 0:
		notes = append(notes, "reads not limited: this binary is built with cgo")
	default:
		notes = append(notes, fmt.Sprintf("reads not limited: Landlock unavailable (%v)", err))
	}
	// After landlockConfine, which opens a file for each root.
	if err := unix.Setrlimit(unix.RLIMIT_NOFILE, &unix.Rlimit{Cur: composeParseOpenFiles, Max: composeParseOpenFiles}); err != nil {
		notes = append(notes, fmt.Sprintf("open files not limited: %v", err))
	}
	return strings.Join(notes, ", ")
}

// landlockConfine lets every thread of the process read files and list
// directories under roots, and nothing else: no reads elsewhere, no writes,
// no new files, no execution, and, as far as the kernel's Landlock ABI goes,
// no network or UNIX socket connections and no signals to other processes
// (see landlockRuleset).
// It returns the ABI version, 0 when Landlock is not available.
func landlockConfine(roots []string) (abi int, err error) {
	version, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, 0, 0, unix.LANDLOCK_CREATE_RULESET_VERSION)
	if errno != 0 {
		return 0, errno
	}
	abi = int(version)
	// A kernel too old to report its errata has fixed none.
	errata, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, 0, 0, unix.LANDLOCK_CREATE_RULESET_ERRATA)
	if errno != 0 {
		errata = 0
	}
	attr := landlockRuleset(abi, errata)
	ruleset, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, uintptr(unsafe.Pointer(&attr)), unsafe.Sizeof(attr), 0)
	if errno != 0 {
		return abi, fmt.Errorf("create ruleset: %w", errno)
	}
	defer unix.Close(int(ruleset))

	for _, root := range roots {
		dir, err := unix.Open(root, unix.O_PATH|unix.O_CLOEXEC|unix.O_DIRECTORY, 0)
		if err != nil {
			continue // nothing there to read
		}
		rule := unix.LandlockPathBeneathAttr{
			Allowed_access: unix.LANDLOCK_ACCESS_FS_READ_FILE | unix.LANDLOCK_ACCESS_FS_READ_DIR,
			Parent_fd:      int32(dir),
		}
		_, _, errno := unix.Syscall6(unix.SYS_LANDLOCK_ADD_RULE, ruleset, unix.LANDLOCK_RULE_PATH_BENEATH, uintptr(unsafe.Pointer(&rule)), 0, 0, 0)
		unix.Close(dir)
		if errno != 0 {
			return abi, fmt.Errorf("add %s: %w", root, errno)
		}
	}

	// Both are per thread: every thread of the runtime takes them, and
	// threads it starts later inherit them. AllThreadsSyscall returns
	// ENOTSUP in a binary built with cgo, which starts threads it does not
	// know of.
	if _, _, errno := syscall.AllThreadsSyscall6(syscall.SYS_PRCTL, unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0, 0); errno != 0 {
		return abi, errno
	}
	if _, _, errno := syscall.AllThreadsSyscall(unix.SYS_LANDLOCK_RESTRICT_SELF, ruleset, 0, 0); errno != 0 {
		return abi, errno
	}
	return abi, nil
}

// landlockErratumSignal is the bit of the kernel's fixed Landlock errata
// for erratum 2: before its fix, a thread scoped for signals cannot signal
// the other threads of its process. The runtime restricts the threads one
// by one, signalling each from the first, which then waits for them all
// without end; so signals are scoped only once the kernel has the fix.
const landlockErratumSignal = 1 << 1

// landlockRuleset returns the rights a ruleset handles, so denies unless a
// rule allows them: every right the kernel's Landlock ABI knows of, since
// the kernel refuses a right it does not. errata are the Landlock errata
// the kernel has fixed.
func landlockRuleset(abi int, errata uintptr) unix.LandlockRulesetAttr {
	attr := unix.LandlockRulesetAttr{
		Handled_access_fs: unix.LANDLOCK_ACCESS_FS_EXECUTE |
			unix.LANDLOCK_ACCESS_FS_WRITE_FILE |
			unix.LANDLOCK_ACCESS_FS_READ_FILE |
			unix.LANDLOCK_ACCESS_FS_READ_DIR |
			unix.LANDLOCK_ACCESS_FS_REMOVE_DIR |
			unix.LANDLOCK_ACCESS_FS_REMOVE_FILE |
			unix.LANDLOCK_ACCESS_FS_MAKE_CHAR |
			unix.LANDLOCK_ACCESS_FS_MAKE_DIR |
			unix.LANDLOCK_ACCESS_FS_MAKE_REG |
			unix.LANDLOCK_ACCESS_FS_MAKE_SOCK |
			unix.LANDLOCK_ACCESS_FS_MAKE_FIFO |
			unix.LANDLOCK_ACCESS_FS_MAKE_BLOCK |
			unix.LANDLOCK_ACCESS_FS_MAKE_SYM,
	}
	if abi >= 2 {
		attr.Handled_access_fs |= unix.LANDLOCK_ACCESS_FS_REFER
	}
	if abi >= 3 {
		attr.Handled_access_fs |= unix.LANDLOCK_ACCESS_FS_TRUNCATE
	}
	if abi >= 4 {
		attr.Handled_access_net = unix.LANDLOCK_ACCESS_NET_BIND_TCP | unix.LANDLOCK_ACCESS_NET_CONNECT_TCP
	}
	if abi >= 5 {
		attr.Handled_access_fs |= unix.LANDLOCK_ACCESS_FS_IOCTL_DEV
	}
	if abi >= 6 {
		attr.Scoped = unix.LANDLOCK_SCOPE_ABSTRACT_UNIX_SOCKET
		if errata&landlockErratumSignal != 0 {
			attr.Scoped |= unix.LANDLOCK_SCOPE_SIGNAL
		}
	}
	if abi >= 9 {
		attr.Handled_access_fs |= unix.LANDLOCK_ACCESS_FS_RESOLVE_UNIX
	}
	if abi >= 10 {
		attr.Handled_access_net |= unix.LANDLOCK_ACCESS_NET_BIND_UDP | unix.LANDLOCK_ACCESS_NET_CONNECT_SEND_UDP
	}
	return attr
}
