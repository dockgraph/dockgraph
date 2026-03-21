package collector

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// ComposeCollector monitors Docker Compose files in a directory tree and
// produces graph snapshots by parsing their service definitions.
type ComposeCollector struct {
	dir     string
	updates chan StateUpdate
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// NewComposeCollector creates a collector that watches the given directory
// for compose file changes.
func NewComposeCollector(dir string) *ComposeCollector {
	return &ComposeCollector{
		dir:     dir,
		updates: make(chan StateUpdate, 16),
		stopCh:  make(chan struct{}),
	}
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
	files, err := findComposeFiles(c.dir)
	if err != nil {
		return err
	}

	var allNodes []Node
	var allEdges []Edge

	for _, f := range files {
		sourceName := filepath.Base(f)
		snap, err := parseComposeFile(f, sourceName)
		if err != nil {
			log.Printf("warning: failed to parse %s: %v", f, err)
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

	filepath.WalkDir(c.dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		if watchErr := watcher.Add(path); watchErr != nil {
			log.Printf("cannot watch %s: %v", path, watchErr)
		}
		return nil
	})

	for {
		select {
		case event := <-watcher.Events:
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) != 0 {
				log.Printf("compose file changed: %s", event.Name)
				if err := c.scan(ctx); err != nil {
					log.Printf("rescan error: %v", err)
				}
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

// findComposeFiles recursively finds all .yaml/.yml files in a directory tree.
func findComposeFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		ext := filepath.Ext(d.Name())
		if ext == ".yaml" || ext == ".yml" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return files, nil
}
