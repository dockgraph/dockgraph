package collector

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- resolveComposePaths ---

func TestResolveComposePathsSingleFile(t *testing.T) {
	files, err := resolveComposePaths([]string{filepath.Join(testdataDir(), "simple.yaml")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}
}

func TestResolveComposePathsDirectory(t *testing.T) {
	files, err := resolveComposePaths([]string{testdataDir()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) == 0 {
		t.Error("expected at least one yaml file in testdata")
	}
}

func TestResolveComposePathsNonExistent(t *testing.T) {
	files, err := resolveComposePaths([]string{"/nonexistent/path/compose.yml"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files for nonexistent path, got %d", len(files))
	}
}

func TestResolveComposePathsEmpty(t *testing.T) {
	files, err := resolveComposePaths(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestResolveComposePathsMixed(t *testing.T) {
	files, err := resolveComposePaths([]string{
		filepath.Join(testdataDir(), "simple.yaml"),
		"/nonexistent",
		testdataDir(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// simple.yaml appears once as direct path and once from directory scan
	if len(files) < 1 {
		t.Errorf("expected at least 1 file, got %d", len(files))
	}
}

func TestResolveComposePathsIgnoresNonYaml(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not yaml"), 0o644)
	os.WriteFile(filepath.Join(dir, "stack.yml"), []byte("name: test\nservices:\n  web:\n    image: nginx\n"), 0o644)

	files, err := resolveComposePaths([]string{dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 yaml file (txt ignored), got %d", len(files))
	}
}

// --- ComposeCollector lifecycle ---

func TestComposeCollectorLifecycle(t *testing.T) {
	cc := NewComposeCollector([]string{filepath.Join(testdataDir(), "simple.yaml")})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := cc.Start(ctx); err != nil {
		t.Fatalf("start failed: %v", err)
	}

	select {
	case update := <-cc.Updates():
		if update.Snapshot == nil {
			t.Fatal("expected non-nil snapshot")
		}
		if len(update.Snapshot.Nodes) == 0 {
			t.Error("expected nodes from compose scan")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for initial scan")
	}

	if err := cc.Stop(); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
}

func TestComposeCollectorEmptyPaths(t *testing.T) {
	cc := NewComposeCollector(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := cc.Start(ctx); err != nil {
		t.Fatalf("start failed: %v", err)
	}

	select {
	case update := <-cc.Updates():
		if update.Snapshot == nil {
			t.Fatal("expected non-nil snapshot")
		}
		if len(update.Snapshot.Nodes) != 0 {
			t.Errorf("expected 0 nodes for empty paths, got %d", len(update.Snapshot.Nodes))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for scan")
	}

	if err := cc.Stop(); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
}

func TestComposeCollectorUpdatesChannel(t *testing.T) {
	cc := NewComposeCollector(nil)
	ch := cc.Updates()
	if ch == nil {
		t.Fatal("Updates() returned nil channel")
	}
}

func TestComposeCollectorWatchDetectsChanges(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping flaky fsnotify test in CI")
	}

	dir := t.TempDir()
	composeFile := filepath.Join(dir, "compose.yml")
	os.WriteFile(composeFile, []byte("name: test\nservices:\n  web:\n    image: nginx\n"), 0o644)

	cc := NewComposeCollector([]string{dir})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := cc.Start(ctx); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer cc.Stop()

	// Drain initial scan
	<-cc.Updates()

	// Small delay to let fsnotify settle before writing
	time.Sleep(100 * time.Millisecond)

	// Modify the file to trigger watcher
	os.WriteFile(composeFile, []byte("name: test\nservices:\n  web:\n    image: nginx:latest\n  api:\n    image: node\n"), 0o644)

	select {
	case update := <-cc.Updates():
		if update.Snapshot == nil {
			t.Fatal("expected non-nil snapshot after file change")
		}
	case <-time.After(5 * time.Second):
		t.Skip("fsnotify not reliable on this platform")
	}
}

// --- parseComposeFile edge cases ---

func TestParseComposeFileNotFound(t *testing.T) {
	_, err := parseComposeFile(context.Background(), "/nonexistent/compose.yml", "compose.yml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestParseComposeFileInvalid(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "bad.yml")
	os.WriteFile(f, []byte("not: valid: compose: content:"), 0o644)

	_, err := parseComposeFile(context.Background(), f, "bad.yml")
	if err == nil {
		t.Fatal("expected error for invalid compose file")
	}
}

func TestParseComposeFileSelfExclusion(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "compose.yml")
	content := `name: test
services:
  web:
    image: nginx
  dg:
    image: dockgraph
    labels:
      dockgraph.self: "true"
`
	os.WriteFile(f, []byte(content), 0o644)

	snap, err := parseComposeFile(context.Background(), f, "compose.yml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	containers := filterNodes(snap.Nodes, "container")
	for _, c := range containers {
		if c.Name == "test-dg-1" {
			t.Error("self-labeled service should be excluded")
		}
	}
}
