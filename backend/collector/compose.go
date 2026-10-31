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
	cancel  context.CancelFunc // ends the watcher and a scan in progress; set by Start
	wg      sync.WaitGroup
	// parsed holds the graph of each file as the last scan had it, used
	// by scans only.
	parsed map[string]GraphSnapshot
}

// NewComposeCollector creates a collector that watches the given paths
// for compose file changes. Each path can be a file or a directory, optionally
// prefixed with an explicit project/stack name as "name=/path" — the name
// overrides the files' top-level `name`, like `docker stack deploy -c file name`.
func NewComposeCollector(paths []string) *ComposeCollector {
	c := &ComposeCollector{
		names:   make(map[string]string),
		updates: make(chan StateUpdate, 16),
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

// Start launches a background goroutine that scans the compose paths, at
// once and whenever their files change. It returns without waiting for the
// first scan: a parse may take up to composeParseTimeout, and nothing else
// waits on compose files.
func (c *ComposeCollector) Start(ctx context.Context) error {
	ctx, c.cancel = context.WithCancel(ctx)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.watchFiles(ctx)
	}()
	return nil
}

// Stop ends a scan in progress, signals the file watcher to exit and waits
// for it to finish.
func (c *ComposeCollector) Stop() error {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	return nil
}

func (c *ComposeCollector) scan(ctx context.Context) error {
	files, err := resolveComposePaths(c.paths)
	if err != nil {
		return err
	}
	roots := c.readRoots()

	var allNodes []Node
	var allEdges []Edge
	parsed := make(map[string]GraphSnapshot, len(files))

	for _, f := range files {
		sourceName := filepath.Base(f)
		snap, err := parseComposeFileWith(ctx, f, sourceName, composeParseOptions{
			ProjectName: c.projectNameFor(f),
			Swarm:       c.swarm,
			ReadRoots:   roots,
		})
		if ctx.Err() != nil {
			// A scan cut short sends nothing: its graph would lack the
			// services of the files it did not parse.
			return ctx.Err()
		}
		if err != nil {
			last, ok := c.parsed[f]
			if !ok || !isProcessError(err) {
				log.Printf("warning: failed to parse %s: %v", sourceName, err)
				continue
			}
			// The process parsing the file failed, which may have nothing
			// to do with the file: its services are not dropped for it.
			log.Printf("warning: failed to parse %s: %v; keeping its services as last parsed", sourceName, err)
			snap = last
		}
		parsed[f] = snap
		allNodes = append(allNodes, snap.Nodes...)
		allEdges = append(allEdges, snap.Edges...)
	}
	c.parsed = parsed

	snapshot := GraphSnapshot{Nodes: allNodes, Edges: allEdges}
	select {
	case c.updates <- StateUpdate{Snapshot: &snapshot}:
	case <-ctx.Done():
	}
	return nil
}

// readRoots returns the directories parses may read files under: the
// compose paths, with a path naming a file standing for its directory.
func (c *ComposeCollector) readRoots() []string {
	roots := make([]string, 0, len(c.paths))
	for _, p := range c.paths {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			p = filepath.Dir(p)
		}
		roots = append(roots, filepath.Clean(p))
	}
	return roots
}

// scanLogged scans, logging a failure unless ctx ended it.
func (c *ComposeCollector) scanLogged(ctx context.Context) {
	if err := c.scan(ctx); err != nil && ctx.Err() == nil {
		log.Printf("compose scan error: %v", err)
	}
}

// watchFiles scans the compose paths, then again whenever their files
// change, until ctx ends or Stop is called.
func (c *ComposeCollector) watchFiles(ctx context.Context) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("fsnotify error: %v", err)
		c.scanLogged(ctx)
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

	// The first scan comes after the watches are in place, so that a file
	// changed while it runs is scanned again.
	c.scanLogged(ctx)

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
			c.scanLogged(ctx)
		case err := <-watcher.Errors:
			log.Printf("watcher error: %v", err)
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
