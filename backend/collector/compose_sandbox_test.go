package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// composeChild, set by -compose-child, has the tests' compose parses done in
// a child process, as DockGraph does them, rather than in the test process.
var composeChild = flag.Bool("compose-child", false, "parse compose files in a child process, as DockGraph does")

// testChildWork are the commands a child started by these tests serves
// besides composeParseCommand: restricted as a parse child is, each does
// what a parse gone wrong could.
var testChildWork = map[string]func(composeParseRequest) composeParseResponse{
	"__test-hang": func(composeParseRequest) composeParseResponse {
		time.Sleep(time.Hour)
		return composeParseResponse{}
	},
	"__test-panic": func(composeParseRequest) composeParseResponse {
		panic("parse went wrong")
	},
	"__test-alloc": func(composeParseRequest) composeParseResponse {
		var held [][]byte
		for range 1024 {
			held = append(held, bytes.Repeat([]byte{1}, 1<<20))
		}
		return composeParseResponse{Error: fmt.Sprintf("allocated %d MiB", len(held))}
	},
	"__test-oom-score": func(composeParseRequest) composeParseResponse {
		data, err := os.ReadFile("/proc/self/oom_score_adj")
		return composeParseResponse{Error: strings.TrimSpace(string(data)) + errorText(err)}
	},
	"__test-env": func(composeParseRequest) composeParseResponse {
		return composeParseResponse{Error: strings.Join(os.Environ(), "\n")}
	},
	"__test-read": func(req composeParseRequest) composeParseResponse {
		_, err := os.ReadFile(req.Path)
		return composeParseResponse{Error: errorText(err)}
	},
	"__test-write": func(req composeParseRequest) composeParseResponse {
		return composeParseResponse{Error: errorText(os.WriteFile(req.Path, []byte("x"), 0o600))}
	},
	"__test-dial": func(req composeParseRequest) composeParseResponse {
		conn, err := net.DialTimeout("tcp", req.Path, 5*time.Second)
		if err == nil {
			conn.Close()
		}
		return composeParseResponse{Error: errorText(err)}
	},
	"__test-flood": func(composeParseRequest) composeParseResponse {
		chunk := bytes.Repeat([]byte("x"), 1<<20)
		for range maxComposeParseOutput>>20 + 1 {
			os.Stdout.Write(chunk)
		}
		return composeParseResponse{}
	},
	"__test-chatty-panic": func(composeParseRequest) composeParseResponse {
		line := strings.Repeat("w", 99) + "\n"
		for range 2 * maxComposeParseStderr / len(line) {
			os.Stderr.WriteString(line)
		}
		panic("parse went wrong")
	},
	"__test-chatty": func(composeParseRequest) composeParseResponse {
		line := strings.Repeat("w", 99) + "\n"
		for range 2 * maxComposeParseStderr / len(line) {
			os.Stderr.WriteString(line)
		}
		return composeParseResponse{Snapshot: &GraphSnapshot{}}
	},
}

// errorText returns err's message, "" for nil.
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestMain(m *testing.M) {
	// Started by a test as a child, the test binary standing in for
	// DockGraph's.
	ServeComposeParse()
	if len(os.Args) == 2 {
		if work, ok := testChildWork[os.Args[1]]; ok {
			os.Exit(serveComposeChild(os.Stdin, os.Stdout, work))
		}
	}
	flag.Parse()
	if !*composeChild {
		composeParseRunner = parseComposeInProcess
	}
	os.Exit(m.Run())
}

// runTestChild has a child serve command for req, failing t if it does not
// answer within a minute.
func runTestChild(t *testing.T, command string, req composeParseRequest) (composeParseResponse, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	return runComposeChild(ctx, command, req)
}

// landlockABI returns the Landlock ABI a child reports it is restricted
// with, skipping t when the child's reads are not restricted.
func landlockABI(t *testing.T, sandbox string) int {
	t.Helper()
	match := regexp.MustCompile(`reads limited to the compose paths \(Landlock ABI (\d+)\)`).FindStringSubmatch(sandbox)
	if match == nil {
		t.Skipf("reads not restricted here: %s", sandbox)
	}
	abi, _ := strconv.Atoi(match[1])
	return abi
}

// canonicalJSON returns snap as JSON with its nodes, edges and the lists
// taken from maps in order, so that two parses of a file compare equal.
func canonicalJSON(t *testing.T, snap GraphSnapshot) string {
	t.Helper()
	snap.Nodes = slices.Clone(snap.Nodes)
	slices.SortFunc(snap.Nodes, func(a, b Node) int { return strings.Compare(a.ID, b.ID) })
	for i, n := range snap.Nodes {
		if n.Compose != nil {
			c := *n.Compose
			c.DependsOn = slices.Sorted(slices.Values(c.DependsOn))
			c.Networks = slices.Sorted(slices.Values(c.Networks))
			snap.Nodes[i].Compose = &c
		}
	}
	snap.Edges = slices.Clone(snap.Edges)
	slices.SortFunc(snap.Edges, func(a, b Edge) int { return strings.Compare(a.ID, b.ID) })
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// A child parses a file as the parsing in-process does: same graph, same
// errors.
func TestComposeChildParsesAsInProcess(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"compose.yml": `version: "3.8"
name: app
include:
  - inc/db.yml
services:
  web:
    extends: {file: ` + filepath.Join(dir, "base.yml") + `, service: base}
    image: nginx
    ports: ["8080-8082:80-82", "443:443/udp"]
    environment:
      API_TOKEN: very-secret
      PLAIN: x
      UNSET:
    volumes:
      - ./html:/usr/share/nginx/html:ro
      - ~/data:/data
      - cache:/cache
    networks: [front, back]
    depends_on: [db]
networks:
  front: {}
  back: {}
volumes:
  cache: {}
`,
		"base.yml": "services:\n  base:\n    labels: {tier: web}\n    restart: always\n",
		"inc/db.yml": "services:\n  db:\n    extends: {file: common.yml, service: base}\n    image: postgres\n" +
			"    networks: [back]\nnetworks:\n  back: {}\n",
		"inc/common.yml":     "services:\n  base:\n    user: postgres\n",
		"missing.yml":        "name: app\ninclude: [nowhere.yml]\nservices:\n  web:\n    image: nginx\n",
		"invalid.yml":        "name: app\nservices: [web]\n",
		"stack/shop.yml":     "services:\n  api:\n    image: api\n    deploy: {mode: global}\n",
		"stack/selfless.yml": "name: dg\nservices:\n  dg:\n    image: dockgraph\n    labels: {dockgraph.self: \"true\"}\n",
	})
	cases := map[string]composeParseRequest{
		"include and extends": {Path: filepath.Join(dir, "compose.yml"), SourceName: "compose.yml"},
		"project name":        {Path: filepath.Join(dir, "compose.yml"), SourceName: "compose.yml", ProjectName: "Renamed"},
		"stack":               {Path: filepath.Join(dir, "stack", "shop.yml"), SourceName: "shop.yml", Swarm: true},
		"testdata stack":      {Path: stackFile(), SourceName: "shop.yml", Swarm: true},
		"testdata":            {Path: filepath.Join(testdataDir(), "simple.yaml"), SourceName: "simple.yaml"},
		"self excluded":       {Path: filepath.Join(dir, "stack", "selfless.yml"), SourceName: "selfless.yml"},
		"missing include":     {Path: filepath.Join(dir, "missing.yml"), SourceName: "missing.yml"},
		"invalid":             {Path: filepath.Join(dir, "invalid.yml"), SourceName: "invalid.yml"},
		"missing file":        {Path: filepath.Join(dir, "none.yml"), SourceName: "none.yml"},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			req.ReadRoots = []string{dir, testdataDir()}
			want, wantErr := parseComposeInProcess(context.Background(), req)
			got, err := parseComposeInChild(context.Background(), req)
			if errorText(err) != errorText(wantErr) {
				t.Fatalf("err = %v, want %v", err, wantErr)
			}
			if canonicalJSON(t, got) != canonicalJSON(t, want) {
				t.Errorf("graph =\n%s\nwant\n%s", canonicalJSON(t, got), canonicalJSON(t, want))
			}
		})
	}
}

// A parse that outlives its deadline is killed with its process.
func TestComposeChildKilledAtDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := runComposeChild(ctx, "__test-hang", composeParseRequest{Path: "compose.yml"})
	if !errors.Is(err, context.DeadlineExceeded) || !isProcessError(err) || !strings.Contains(err.Error(), "did not finish") {
		t.Errorf("err = %v, want the parse stopped at its deadline", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("took %s to stop the parse", elapsed)
	}
}

// A child that crashes is an error saying why, even after logging more
// than is kept of its log, and DockGraph carries on.
func TestComposeChildCrashReported(t *testing.T) {
	for _, command := range []string{"__test-panic", "__test-chatty-panic"} {
		_, err := runTestChild(t, command, composeParseRequest{Path: "compose.yml"})
		if err == nil || !isProcessError(err) || !strings.Contains(err.Error(), "parse failed") || !strings.Contains(err.Error(), "panic: parse went wrong") {
			t.Errorf("%s: err = %v, want the crash and its reason", command, err)
		}
	}
}

// The parser's own verdict on a file is not the failure of its process.
func TestComposeChildParseErrorIsNotProcessError(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"compose.yml": "name: app\nservices: [web]\n"})
	req := composeParseRequest{Path: filepath.Join(dir, "compose.yml"), SourceName: "compose.yml", ReadRoots: []string{dir}}
	if _, err := parseComposeInChild(context.Background(), req); err == nil || isProcessError(err) {
		t.Errorf("err = %v, want the loader's error", err)
	}
}

// A file whose parse process fails keeps the services it last had; one the
// parser refuses loses them.
func TestComposeScanKeepsServicesWhenProcessFails(t *testing.T) {
	results := []error{nil, processError{errors.New("parse failed: signal: killed")}, errors.New("services must be a mapping")}
	call := 0
	prevRunner := composeParseRunner
	composeParseRunner = func(context.Context, composeParseRequest) (GraphSnapshot, error) {
		err := results[call]
		call++
		if err != nil {
			return GraphSnapshot{}, err
		}
		return GraphSnapshot{Nodes: []Node{{ID: "container:app-web-1", Type: "container"}}}, nil
	}
	t.Cleanup(func() { composeParseRunner = prevRunner })

	cc := NewComposeCollector([]string{filepath.Join(testdataDir(), "simple.yaml")})
	for i, want := range []int{1, 1, 0} {
		if err := cc.scan(context.Background()); err != nil {
			t.Fatal(err)
		}
		if got := len((<-cc.Updates()).Snapshot.Nodes); got != want {
			t.Errorf("scan %d: %d nodes, want %d", i+1, got, want)
		}
	}
}

// A parse taking more memory than its limit ends its process.
func TestComposeChildMemoryLimited(t *testing.T) {
	if runtime.GOOS != "linux" || raceBuild {
		t.Skip("the memory limit is set on Linux, outside race detector builds")
	}
	_, err := runTestChild(t, "__test-alloc", composeParseRequest{Path: "compose.yml"})
	if err == nil || !strings.Contains(err.Error(), "parse failed") || !strings.Contains(err.Error(), "memory") {
		t.Errorf("err = %v, want the parse ended for its memory", err)
	}
}

// Should the memory run out, the kernel ends a child first.
func TestComposeChildEndedFirstOnOOM(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("set on Linux")
	}
	resp, err := runTestChild(t, "__test-oom-score", composeParseRequest{Path: "compose.yml", ReadRoots: []string{"/proc/self"}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error != "1000" {
		t.Errorf("child oom_score_adj = %q, want 1000", resp.Error)
	}
}

// realisticCompose returns a compose file of services services, each with
// the settings of a typical service.
func realisticCompose(services int) string {
	var b strings.Builder
	b.WriteString("name: big\nnetworks:\n  front: {}\n  back: {}\nvolumes:\n")
	for i := range services {
		fmt.Fprintf(&b, "  data%d: {}\n", i)
	}
	b.WriteString("services:\n")
	for i := range services {
		fmt.Fprintf(&b, "  svc%d:\n    image: registry.example.com/team/service-%d:1.2.3\n", i, i)
		fmt.Fprintf(&b, "    command: [\"/app/bin/server\", \"--port\", \"8080\", \"--name\", \"svc%d\"]\n", i)
		b.WriteString("    restart: unless-stopped\n    user: \"1000:1000\"\n    working_dir: /app\n")
		fmt.Fprintf(&b, "    ports: [\"%d:8080\", \"%d:9090\"]\n    environment:\n", 10000+i, 20000+i)
		for j := range 20 {
			fmt.Fprintf(&b, "      SETTING_NUMBER_%d: \"value-for-setting-%d-of-service-%d\"\n", j, j, i)
		}
		b.WriteString("    labels:\n")
		for j := range 8 {
			fmt.Fprintf(&b, "      com.example.label%d: \"label-value-%d\"\n", j, j)
		}
		fmt.Fprintf(&b, "    volumes:\n      - data%d:/var/lib/data\n      - ./config/svc%d:/etc/app:ro\n", i, i)
		b.WriteString("    networks: [front, back]\n")
		if i > 0 {
			fmt.Fprintf(&b, "    depends_on: [svc%d]\n", i-1)
		}
	}
	return b.String()
}

// The largest compose files a parse allows fit in a child's memory: one of
// typical services, and one with a document of as many YAML nodes as the
// loader allows.
func TestComposeChildMemoryFitsLargestFiles(t *testing.T) {
	if runtime.GOOS != "linux" || raceBuild {
		t.Skip("the memory limit is set on Linux, outside race detector builds")
	}
	files := map[string]string{
		"services.yml": realisticCompose(520),
		"nodes.yml":    "name: app\nservices:\n  web:\n    image: nginx\nx-big: [" + strings.Repeat("a,", 99_900) + "a]\n",
	}
	dir := t.TempDir()
	writeFiles(t, dir, files)
	for name, content := range files {
		if len(content) > maxComposeFileSize {
			t.Fatalf("%s is %d bytes, more than a parse allows", name, len(content))
		}
		req := composeParseRequest{Path: filepath.Join(dir, name), SourceName: name, ReadRoots: []string{dir}}
		if _, err := parseComposeInChild(context.Background(), req); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// A child sees nothing of DockGraph's environment but its time zone and
// home directory.
func TestComposeChildEnvironment(t *testing.T) {
	t.Setenv("DG_TEST_SECRET", "hunter2")
	t.Setenv("TZ", "UTC")
	t.Setenv("HOME", "/home/dg")
	resp, err := runTestChild(t, "__test-env", composeParseRequest{Path: "compose.yml"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error != "GOMAXPROCS=2\nTZ=UTC\nHOME=/home/dg" {
		t.Errorf("child environment = %q, want only GOMAXPROCS, TZ and HOME", resp.Error)
	}
}

// A child reads nothing but the files under its read roots, through a
// symlink or not, and writes nothing.
func TestComposeChildReadsOnlyRoots(t *testing.T) {
	root, other := t.TempDir(), t.TempDir()
	writeFiles(t, root, map[string]string{"compose.yml": "services: {}\n"})
	writeFiles(t, other, map[string]string{"secret.yml": "services: {}\n"})
	if err := os.Symlink(filepath.Join(other, "secret.yml"), filepath.Join(root, "link.yml")); err != nil {
		t.Fatal(err)
	}
	read := func(path string) composeParseResponse {
		t.Helper()
		resp, err := runTestChild(t, "__test-read", composeParseRequest{Path: path, ReadRoots: []string{root}})
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	landlockABI(t, read(filepath.Join(root, "compose.yml")).Sandbox)

	if resp := read(filepath.Join(root, "compose.yml")); resp.Error != "" {
		t.Errorf("reading a file under the root: %s", resp.Error)
	}
	for _, path := range []string{
		filepath.Join(other, "secret.yml"),
		filepath.Join(root, "link.yml"),
		fmt.Sprintf("/proc/%d/environ", os.Getpid()),
	} {
		if resp := read(path); !strings.Contains(resp.Error, "permission denied") {
			t.Errorf("reading %s: err = %q, want permission denied", path, resp.Error)
		}
	}

	created := filepath.Join(root, "new.yml")
	resp, err := runTestChild(t, "__test-write", composeParseRequest{Path: created, ReadRoots: []string{root}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Error, "permission denied") {
		t.Errorf("writing %s: err = %q, want permission denied", created, resp.Error)
	}
	if _, err := os.Stat(created); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the child created %s (stat: %v)", created, err)
	}
}

// A child connects to nothing, where the kernel's Landlock ABI restricts
// TCP.
func TestComposeChildNoNetwork(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	resp, err := runTestChild(t, "__test-dial", composeParseRequest{Path: ln.Addr().String(), ReadRoots: []string{t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}
	if abi := landlockABI(t, resp.Sandbox); abi < 4 {
		t.Skipf("Landlock ABI %d does not restrict TCP", abi)
	}
	if !strings.Contains(resp.Error, "permission denied") {
		t.Errorf("dialing %s: err = %q, want permission denied", ln.Addr(), resp.Error)
	}
}

// What a child writes is bounded: a result past the bound is refused, and
// a flood of warnings is logged only up to its bound.
func TestComposeChildOutputBounded(t *testing.T) {
	if _, err := runTestChild(t, "__test-flood", composeParseRequest{Path: "compose.yml"}); err == nil || !strings.Contains(err.Error(), "parse result larger than") {
		t.Errorf("err = %v, want an oversized result refused", err)
	}

	logged := captureLog(t)
	if _, err := runTestChild(t, "__test-chatty", composeParseRequest{Path: "compose.yml"}); err != nil {
		t.Fatal(err)
	}
	if logged.Len() > maxComposeParseStderr+200 || !strings.Contains(logged.String(), "compose.yml: more parse warnings were not logged") {
		t.Errorf("logged %d bytes, ending %q; want at most %d and a note of the rest", logged.Len(), logged.String()[max(logged.Len()-200, 0):], maxComposeParseStderr)
	}
}

// What the loader logs while parsing in a child is logged by DockGraph.
func TestComposeChildWarningsLogged(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"compose.yml": "version: \"3\"\nname: app\nservices:\n  web:\n    image: nginx\n"})
	logged := captureLog(t)
	req := composeParseRequest{Path: filepath.Join(dir, "compose.yml"), SourceName: "compose.yml", ReadRoots: []string{dir}}
	if _, err := parseComposeInChild(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logged.String(), "the attribute `version` is obsolete") {
		t.Errorf("log = %q, want the loader's warning", logged.String())
	}
}

// Start does not wait for the first scan, and Stop ends a scan in progress.
func TestComposeCollectorStartsWithoutWaitingForScan(t *testing.T) {
	parsing := make(chan struct{})
	prevRunner := composeParseRunner
	composeParseRunner = func(ctx context.Context, _ composeParseRequest) (GraphSnapshot, error) {
		close(parsing)
		<-ctx.Done()
		return GraphSnapshot{}, ctx.Err()
	}
	t.Cleanup(func() { composeParseRunner = prevRunner })

	cc := NewComposeCollector([]string{filepath.Join(testdataDir(), "simple.yaml")})
	started := make(chan error, 1)
	go func() { started <- cc.Start(context.Background()) }()
	select {
	case err := <-started:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start waited for the scan")
	}
	<-parsing

	stopped := make(chan struct{})
	go func() {
		cc.Stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not end the scan in progress")
	}
	select {
	case update := <-cc.Updates():
		t.Errorf("a scan Stop ended sent %+v", update)
	default:
	}
}
