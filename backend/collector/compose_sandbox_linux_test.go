//go:build linux

package collector

import (
	"testing"

	"golang.org/x/sys/unix"
)

// The child's CPU limit covers two cores for the parent's whole deadline.
func TestComposeParseCPULimit(t *testing.T) {
	if composeParseCPU != 60 {
		t.Errorf("composeParseCPU = %d seconds, want 60", composeParseCPU)
	}
}

// Signals are scoped only on a kernel that lets the threads of a scoped
// process signal one another, which restricting every thread relies on.
func TestLandlockRulesetScopesSignalsOnlyWithFix(t *testing.T) {
	cases := []struct {
		abi    int
		errata uintptr
		want   uint64
	}{
		{5, landlockErratumSignal, 0},
		{6, 0, unix.LANDLOCK_SCOPE_ABSTRACT_UNIX_SOCKET},
		{6, landlockErratumSignal, unix.LANDLOCK_SCOPE_ABSTRACT_UNIX_SOCKET | unix.LANDLOCK_SCOPE_SIGNAL},
		{7, 1, unix.LANDLOCK_SCOPE_ABSTRACT_UNIX_SOCKET},
		{7, 0xf, unix.LANDLOCK_SCOPE_ABSTRACT_UNIX_SOCKET | unix.LANDLOCK_SCOPE_SIGNAL},
	}
	for _, c := range cases {
		if got := landlockRuleset(c.abi, c.errata).Scoped; got != c.want {
			t.Errorf("ABI %d, errata %#x: scoped = %#x, want %#x", c.abi, c.errata, got, c.want)
		}
	}
}
