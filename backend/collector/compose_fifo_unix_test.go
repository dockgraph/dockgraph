//go:build unix

package collector

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// parseWithin parses the compose file at path, failing t if that takes
// longer than limit.
func parseWithin(t *testing.T, path string, limit time.Duration) error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		_, err := parseComposeFile(context.Background(), path, "compose.yml")
		done <- err
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(limit):
		t.Fatalf("parse still running after %s", limit)
		return nil
	}
}

// A FIFO is refused rather than waited on, wherever it is named: waiting
// would block the parse, and with it the scan, for good.
func TestParseComposeFileRefusesFifo(t *testing.T) {
	for _, name := range []string{"compose file", "include", "extends file"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path, fifo := filepath.Join(dir, "compose.yml"), filepath.Join(dir, "pipe.yml")
			switch name {
			case "compose file":
				fifo = path
			case "include":
				writeFiles(t, dir, map[string]string{
					"compose.yml": "name: app\ninclude: [pipe.yml]\nservices:\n  web:\n    image: nginx\n",
				})
			case "extends file":
				writeFiles(t, dir, map[string]string{
					"compose.yml": "name: app\nservices:\n  web:\n    extends: {file: " + fifo + ", service: base}\n",
				})
			}
			if err := syscall.Mkfifo(fifo, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := parseWithin(t, path, 10*time.Second); err == nil || !strings.Contains(err.Error(), "not a regular file") {
				t.Errorf("err = %v, want the FIFO refused", err)
			}
		})
	}
}

// An extends file is checked as the loader opens it: an absolute path as
// written, which the system resolves through symlinks before "..", so it
// may name another file than the same path cleaned.
func TestParseComposeFileExtendsCheckedAsOpened(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a", "b"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "a", "b"), filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	writeFiles(t, dir, map[string]string{
		"a/big.yml": "services:\n  base:\n    image: alpine\n# " + strings.Repeat("x", maxComposeFileSize) + "\n",
	})
	fifo := filepath.Join(dir, "a", "pipe.yml")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	// Should a parse wait on the FIFO, opening it for writing lets it go.
	t.Cleanup(func() {
		if w, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			w.Close()
		}
	})
	cases := map[string]struct{ ref, want string }{
		"symlink and .. to a large file": {filepath.Join(dir, "link") + "/../big.yml", "larger than"},
		"symlink and .. to a FIFO":       {filepath.Join(dir, "link") + "/../pipe.yml", "not a regular file"},
	}
	if _, err := os.Stat("/proc/self/root"); err == nil {
		cases["root link and .. to a device"] = struct{ ref, want string }{"/proc/self/root/../dev/null", "not a regular file"}
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, "compose.yml")
			writeFiles(t, dir, map[string]string{
				"compose.yml": "name: app\nservices:\n  web:\n    extends: {file: " + c.ref + ", service: base}\n",
			})
			if err := parseWithin(t, path, 10*time.Second); !refusedFor(err, c.want) {
				t.Errorf("err = %v, want %q", err, c.want)
			}
		})
	}
}
