package collector

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/compose-spec/compose-go/v2/loader"
	composetypes "github.com/compose-spec/compose-go/v2/types"
	"github.com/fsnotify/fsnotify"
)

type ComposeCollector struct {
	dir     string
	updates chan StateUpdate
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

func NewComposeCollector(dir string) *ComposeCollector {
	return &ComposeCollector{
		dir:     dir,
		updates: make(chan StateUpdate, 16),
		stopCh:  make(chan struct{}),
	}
}

func (c *ComposeCollector) Updates() <-chan StateUpdate {
	return c.updates
}

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

	if err := watcher.Add(c.dir); err != nil {
		log.Printf("cannot watch %s: %v", c.dir, err)
		return
	}

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

func findComposeFiles(dir string) ([]string, error) {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext == ".yaml" || ext == ".yml" {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	return files, nil
}

func parseComposeFile(path, sourceName string) (GraphSnapshot, error) {
	var snap GraphSnapshot

	data, err := os.ReadFile(path)
	if err != nil {
		return snap, err
	}

	project, err := loader.LoadWithContext(context.Background(), composetypes.ConfigDetails{
		ConfigFiles: []composetypes.ConfigFile{
			{Filename: path, Content: data},
		},
	})
	if err != nil {
		return snap, err
	}

	networkNames := make(map[string]bool)
	for name := range project.Networks {
		if name == "default" {
			continue
		}
		networkNames[name] = true
		snap.Nodes = append(snap.Nodes, Node{
			ID:     "network:" + name,
			Type:   "network",
			Name:   name,
			Source: sourceName,
		})
	}

	for name := range project.Volumes {
		snap.Nodes = append(snap.Nodes, Node{
			ID:     "volume:" + name,
			Type:   "volume",
			Name:   name,
			Source: sourceName,
		})
	}

	for _, svc := range project.Services {
		if svc.Labels[selfLabel] == "true" {
			continue
		}

		svcName := svc.Name
		containerID := "container:" + svcName

		var trackedNets []string
		for netName := range svc.Networks {
			if networkNames[netName] {
				trackedNets = append(trackedNets, netName)
			}
		}
		sort.Strings(trackedNets)

		var primaryNet string
		var secondaryNets []string
		for _, netName := range trackedNets {
			if primaryNet == "" {
				primaryNet = netName
			} else {
				secondaryNets = append(secondaryNets, netName)
			}
		}

		var ports []PortMapping
		for _, p := range svc.Ports {
			if p.Published != "" {
				var hostPort int
				fmt.Sscanf(p.Published, "%d", &hostPort)
				ports = append(ports, PortMapping{
					Host:      hostPort,
					Container: int(p.Target),
				})
			}
		}

		node := Node{
			ID:     containerID,
			Type:   "container",
			Name:   svcName,
			Image:  svc.Image,
			Status: "not_running",
			Ports:  ports,
			Source: sourceName,
		}
		if primaryNet != "" {
			node.NetworkID = "network:" + primaryNet
		}
		snap.Nodes = append(snap.Nodes, node)

		for _, netName := range secondaryNets {
			snap.Edges = append(snap.Edges, Edge{
				ID:     "e:net:" + svcName + ":" + netName,
				Type:   "secondary_network",
				Source: containerID,
				Target: "network:" + netName,
			})
		}

		for depName := range svc.DependsOn {
			snap.Edges = append(snap.Edges, Edge{
				ID:     "e:dep:" + svcName + ":" + depName,
				Type:   "depends_on",
				Source: containerID,
				Target: "container:" + depName,
			})
		}

		for _, v := range svc.Volumes {
			if v.Type == "volume" {
				snap.Edges = append(snap.Edges, Edge{
					ID:        "e:vol:" + v.Source + ":" + svcName,
					Type:      "volume_mount",
					Source:    "volume:" + v.Source,
					Target:    containerID,
					MountPath: v.Target,
				})
			}
		}
	}

	return snap, nil
}
