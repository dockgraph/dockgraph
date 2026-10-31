package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Compose files are parsed in a child process: DockGraph's own binary,
// started with composeParseCommand, parses one file and writes the graph
// back as JSON. The child is held to limits the system enforces (see
// restrictComposeParse), so a parse that takes too much memory or time
// ends it, not DockGraph. It also gets none of DockGraph's environment but
// its time zone and home directory, so no secret of DockGraph's is there
// for a compose file to bring into the graph.

// composeParseCommand is the argument that makes DockGraph's binary serve
// one compose parse for its parent instead of running.
const composeParseCommand = "__parse-compose"

// Bounds on what parent and child send each other. A request names a file
// and a few directories; the graph of a 780 KiB compose file of 400
// services is about 1 MiB of JSON. Of what a child logs past the first 64
// KiB, the last 32 KiB are kept, which hold a crash's reason and stack.
const (
	maxComposeParseRequest    = 1 << 20
	maxComposeParseOutput     = 8 << 20
	maxComposeParseStderr     = 64 << 10
	maxComposeParseStderrTail = 32 << 10
)

// composeChildEnv returns the child's whole environment. GOMAXPROCS keeps
// the parse to two cores and the runtime from adjusting it, which would
// read cgroup files the child is not allowed to read. TZ and HOME, when
// set, are DockGraph's: the loader's log lines take the time zone, and
// paths starting with "~" are expanded as in DockGraph.
func composeChildEnv() []string {
	env := []string{"GOMAXPROCS=2"}
	for _, name := range []string{"TZ", "HOME"} {
		if value, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+value)
		}
	}
	return env
}

// composeParseRequest is the parse a child is asked for, and the
// directories it may read.
type composeParseRequest struct {
	Path        string   `json:"path"`
	SourceName  string   `json:"sourceName"`
	ProjectName string   `json:"projectName,omitempty"`
	Swarm       bool     `json:"swarm,omitempty"`
	ReadRoots   []string `json:"readRoots"`
}

// options returns the request's naming and mode options.
func (r composeParseRequest) options() composeParseOptions {
	return composeParseOptions{ProjectName: r.ProjectName, Swarm: r.Swarm, ReadRoots: r.ReadRoots}
}

// composeParseResponse is a child's answer: the graph or why there is none,
// and how the child was restricted.
type composeParseResponse struct {
	Snapshot *GraphSnapshot `json:"snapshot,omitempty"`
	Error    string         `json:"error,omitempty"`
	Sandbox  string         `json:"sandbox"`
}

// composeParseRunner parses as a request asks. A variable so that tests can
// parse in-process.
var composeParseRunner = parseComposeInChild

// ServeComposeParse serves the compose parse a process was started for by
// its parent (see parseComposeInChild) and exits. In any other process it
// returns at once. main calls it before anything else.
func ServeComposeParse() {
	if len(os.Args) == 2 && os.Args[1] == composeParseCommand {
		os.Exit(serveComposeChild(os.Stdin, os.Stdout, parseComposeRequest))
	}
}

// serveComposeChild reads a request from in, restricts the process to it,
// and writes the response work gives to out. It returns the exit code: not
// 0 only when there is no response to give.
func serveComposeChild(in io.Reader, out io.Writer, work func(composeParseRequest) composeParseResponse) int {
	var req composeParseRequest
	if err := json.NewDecoder(io.LimitReader(in, maxComposeParseRequest)).Decode(&req); err != nil {
		fmt.Fprintln(os.Stderr, "compose parse request:", err)
		return 2
	}
	// The time zone is loaded when first used: load it while the files it
	// is loaded from can still be read.
	_ = time.Local.String()
	sandbox := restrictComposeParse(req.ReadRoots)
	resp := work(req)
	resp.Sandbox = sandbox
	if err := json.NewEncoder(out).Encode(resp); err != nil {
		fmt.Fprintln(os.Stderr, "compose parse response:", err)
		return 2
	}
	return 0
}

// parseComposeRequest does the parse req asks for, in this process.
func parseComposeRequest(req composeParseRequest) composeParseResponse {
	snap, err := parseComposeInProcess(context.Background(), req)
	if err != nil {
		return composeParseResponse{Error: err.Error()}
	}
	return composeParseResponse{Snapshot: &snap}
}

// parseComposeInProcess does the parse req asks for in this process.
func parseComposeInProcess(ctx context.Context, req composeParseRequest) (GraphSnapshot, error) {
	return recoverParse(req.Path, func() (GraphSnapshot, error) {
		return loadComposeFile(ctx, req.Path, req.SourceName, req.options())
	})
}

// parseComposeInChild has a child process do the parse req asks for, and
// kills it when ctx is done.
func parseComposeInChild(ctx context.Context, req composeParseRequest) (GraphSnapshot, error) {
	resp, err := runComposeChild(ctx, composeParseCommand, req)
	if err != nil {
		return GraphSnapshot{}, err
	}
	logComposeSandbox(resp.Sandbox)
	switch {
	case resp.Error != "":
		return GraphSnapshot{}, errors.New(resp.Error)
	case resp.Snapshot == nil:
		return GraphSnapshot{}, processError{fmt.Errorf("%s: parse returned no graph", req.Path)}
	}
	return *resp.Snapshot, nil
}

// processError is the failure of a parse child that gave no answer of its
// own: it could not start, was killed, crashed, or answered with more or
// less than a graph. Unlike an error the loader reports, it tells nothing
// certain about the file parsed.
type processError struct{ err error }

func (e processError) Error() string { return e.err.Error() }
func (e processError) Unwrap() error { return e.err }

// isProcessError reports whether err is the failure of a parse child that
// gave no answer of its own.
func isProcessError(err error) bool {
	return errors.As(err, new(processError))
}

// runComposeChild starts DockGraph's binary with command, sends it req and
// reads its response. A child that crashes, hits a limit or outlives ctx is
// a processError; what it logged is passed on to the log when it answers.
func runComposeChild(ctx context.Context, command string, req composeParseRequest) (composeParseResponse, error) {
	var resp composeParseResponse
	in, err := json.Marshal(req)
	if err != nil {
		return resp, err
	}
	cmd := exec.CommandContext(ctx, selfExecutable(), command)
	cmd.Env = composeChildEnv()
	cmd.SysProcAttr = composeChildAttr()
	cmd.Stdin = bytes.NewReader(in)
	stdout := &boundedBuffer{limit: maxComposeParseOutput}
	stderr := &boundedBuffer{limit: maxComposeParseStderr, tailLimit: maxComposeParseStderrTail}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	// The child has no children of its own holding its output open, so its
	// output ends soon after it does.
	cmd.WaitDelay = time.Second

	runErr := cmd.Run()
	switch {
	case ctx.Err() != nil:
		return resp, processError{fmt.Errorf("%s: parse did not finish: %w", req.Path, ctx.Err())}
	case runErr != nil:
		return resp, processError{fmt.Errorf("%s: parse failed: %v%s", req.Path, runErr, crashReason(stderr))}
	case stdout.truncated:
		return resp, processError{fmt.Errorf("%s: parse result larger than %d bytes", req.Path, maxComposeParseOutput)}
	}
	relayComposeLog(req.Path, stderr)
	if err := json.Unmarshal(stdout.buf.Bytes(), &resp); err != nil {
		return resp, processError{fmt.Errorf("%s: reading the parse result: %w", req.Path, err)}
	}
	return resp, nil
}

// selfExecutable returns the path that starts DockGraph's own binary. On
// Linux that is the running binary whatever has become of its file.
func selfExecutable() string {
	if runtime.GOOS == "linux" {
		return "/proc/self/exe"
	}
	if exe, err := os.Executable(); err == nil {
		return exe
	}
	return os.Args[0]
}

// crashReason returns the line of a crashed child's output saying why it
// crashed (the runtime's "fatal error: …" or "panic: …"), for the error
// message, or "" when there is none. The runtime writes it last, so it may
// be past what stderr kept from the start, in what it kept of the end.
func crashReason(stderr *boundedBuffer) string {
	for _, part := range [][]byte{stderr.buf.Bytes(), stderr.tail} {
		for line := range strings.Lines(string(part)) {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "fatal error:") || strings.HasPrefix(line, "panic:") {
				return ": " + line
			}
		}
	}
	return ""
}

// relayComposeLog passes on what a child that answered logged, such as the
// loader's warnings about obsolete attributes.
func relayComposeLog(path string, stderr *boundedBuffer) {
	logged := stderr.buf.Bytes()
	if stderr.truncated {
		// Up to the last whole line.
		logged = logged[:bytes.LastIndexByte(logged, '\n')+1]
	}
	if len(logged) > 0 {
		_, _ = log.Writer().Write(logged)
	}
	if stderr.truncated {
		log.Printf("%s: more parse warnings were not logged", path)
	}
}

// composeSandboxLogged makes the child's restrictions logged once.
var composeSandboxLogged sync.Once

// logComposeSandbox logs, once, how compose parses are restricted.
func logComposeSandbox(sandbox string) {
	composeSandboxLogged.Do(func() {
		log.Printf("INFO  compose files are parsed in a separate process: %s", sandbox)
	})
}

// boundedBuffer keeps the first limit bytes written to it and, of the rest,
// the last tailLimit bytes, so that a child writing without end cannot fill
// the parent's memory; the child is not made to fail its writes.
type boundedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
	tail      []byte // the last bytes written past limit
	tailLimit int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	room := max(b.limit-b.buf.Len(), 0)
	if len(p) <= room {
		return b.buf.Write(p)
	}
	b.buf.Write(p[:room])
	b.truncated = true
	if b.tailLimit > 0 {
		b.tail = append(b.tail, p[room:]...)
		if over := len(b.tail) - b.tailLimit; over > 0 {
			b.tail = append(b.tail[:0], b.tail[over:]...)
		}
	}
	return len(p), nil
}
