package collector

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ComposeCollector monitors Docker Compose files and produces graph snapshots
// by parsing their service definitions. Paths can point to individual files
// or directories (scanned recursively for .yml/.yaml files).
type ComposeCollector struct {
	paths   []string
	names   map[string]string // path -> explicit project/stack name
	swarm   bool
	updates chan StateUpdate
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// NewComposeCollector creates a collector that watches the given paths
// for compose file changes. Each path can be a file or a directory, optionally
// prefixed with an explicit project/stack name as "name=/path" — the name
// overrides the files' top-level `name`, like `docker stack deploy -c file name`.
func NewComposeCollector(paths []string) *ComposeCollector {
	c := &ComposeCollector{
		names:   make(map[string]string),
		updates: make(chan StateUpdate, 16),
		stopCh:  make(chan struct{}),
	}
	for _, entry := range paths {
		name, path := splitNamedPath(entry)
		c.paths = append(c.paths, path)
		if name != "" {
			c.names[filepath.Clean(path)] = name
		}
	}
	return c
}

// EnableSwarm makes the collector interpret compose files as swarm stack
// files: services become "service" ghost nodes named {stack}_{service}.
// Must be called before Start.
func (c *ComposeCollector) EnableSwarm() {
	c.swarm = true
}

// validPathName matches the name part of a "name=/path" entry.
var validPathName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

// splitNamedPath splits a "name=/path" entry. Entries without a valid name
// prefix (including paths that merely contain '=') are returned as-is.
func splitNamedPath(entry string) (name, path string) {
	if before, after, ok := strings.Cut(entry, "="); ok && validPathName.MatchString(before) && after != "" {
		return before, after
	}
	return "", entry
}

// projectNameFor returns the explicit name configured for a file, either on
// the file itself or on a directory containing it. Empty means use the file's
// own `name` field.
func (c *ComposeCollector) projectNameFor(file string) string {
	file = filepath.Clean(file)
	best, bestLen := "", -1
	for path, name := range c.names {
		if (file == path || strings.HasPrefix(file, path+string(filepath.Separator))) && len(path) > bestLen {
			best, bestLen = name, len(path)
		}
	}
	return best
}

// Updates returns a read-only channel that emits state updates whenever
// compose files are added, modified, or removed.
func (c *ComposeCollector) Updates() <-chan StateUpdate {
	return c.updates
}

// Start performs an initial scan and launches a background goroutine
// to watch for file changes.
func (c *ComposeCollector) Start(ctx context.Context) error {
	if err := c.scan(ctx); err != nil {
		return err
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.watchFiles(ctx)
	}()

	return nil
}

// Stop signals the file watcher to exit and waits for it to finish.
func (c *ComposeCollector) Stop() error {
	close(c.stopCh)
	c.wg.Wait()
	return nil
}

func (c *ComposeCollector) scan(ctx context.Context) error {
	files, err := resolveComposePaths(c.paths)
	if err != nil {
		return err
	}

	var allNodes []Node
	var allEdges []Edge

	for _, f := range files {
		sourceName := filepath.Base(f)
		snap, err := parseComposeFileWith(ctx, f, sourceName, composeParseOptions{
			ProjectName: c.projectNameFor(f),
			Swarm:       c.swarm,
		})
		if err != nil {
			log.Printf("warning: failed to parse %s: %v", sourceName, err)
			continue
		}
		allNodes = append(allNodes, snap.Nodes...)
		allEdges = append(allEdges, snap.Edges...)
	}

	snapshot := GraphSnapshot{Nodes: allNodes, Edges: allEdges}
	select {
	case c.updates <- StateUpdate{Snapshot: &snapshot}:
	case <-ctx.Done():
	}
	return nil
}

func (c *ComposeCollector) watchFiles(ctx context.Context) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("fsnotify error: %v", err)
		return
	}
	defer watcher.Close()

	for _, p := range c.paths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			// Watch the parent directory so we catch writes to the file
			if watchErr := watcher.Add(filepath.Dir(p)); watchErr != nil {
				log.Printf("cannot watch %s: %v", filepath.Dir(p), watchErr)
			}
			continue
		}
		_ = filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return nil
			}
			if watchErr := watcher.Add(path); watchErr != nil {
				log.Printf("cannot watch %s: %v", path, watchErr)
			}
			return nil
		})
	}

	db := newDebouncer(500 * time.Millisecond)
	defer db.stop()

	for {
		select {
		case event := <-watcher.Events:
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) != 0 {
				// Watch newly created subdirectories so files added later are detected.
				if event.Op&fsnotify.Create != 0 {
					if info, statErr := os.Stat(event.Name); statErr == nil && info.IsDir() {
						_ = watcher.Add(event.Name)
					}
				}
				db.trigger()
			}
		case <-db.notify:
			log.Printf("compose files changed, rescanning")
			if err := c.scan(ctx); err != nil {
				log.Printf("rescan error: %v", err)
			}
		case err := <-watcher.Errors:
			log.Printf("watcher error: %v", err)
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// resolveComposePaths expands a list of paths into concrete compose files.
// Each path can be a .yml/.yaml file or a directory (scanned recursively).
func resolveComposePaths(paths []string) ([]string, error) {
	var files []string
	seen := make(map[string]bool)
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			files = append(files, p)
		}
	}

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if !info.IsDir() {
			add(p)
			continue
		}
		err = filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			ext := filepath.Ext(d.Name())
			if ext == ".yaml" || ext == ".yml" {
				add(path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}
