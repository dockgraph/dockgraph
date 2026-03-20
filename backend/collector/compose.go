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

func findComposeFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
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
		return snap, fmt.Errorf("%w (does the file have a top-level 'name' field?)", err)
	}

	projectName := project.Name

	// Docker Compose naming conventions:
	//   networks: {project}_{name}   (underscore)
	//   volumes:  {project}_{name}   (underscore)
	//   containers: {project}-{service}-1  (hyphens)
	networkFullName := func(name string) string { return projectName + "_" + name }
	volumeFullName := func(name string) string { return projectName + "_" + name }
	containerFullName := func(name string) string { return projectName + "-" + name + "-1" }

	networkNames := make(map[string]bool)
	for name := range project.Networks {
		if name == "default" {
			continue
		}
		networkNames[name] = true
		fullName := networkFullName(name)
		snap.Nodes = append(snap.Nodes, Node{
			ID:     "network:" + fullName,
			Type:   "network",
			Name:   fullName,
			Source: sourceName,
		})
	}

	for name := range project.Volumes {
		fullName := volumeFullName(name)
		snap.Nodes = append(snap.Nodes, Node{
			ID:     "volume:" + fullName,
			Type:   "volume",
			Name:   fullName,
			Source: sourceName,
		})
	}

	for _, svc := range project.AllServices() {
		if svc.Labels[selfLabel] == "true" {
			continue
		}

		svcName := containerFullName(svc.Name)
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
			node.NetworkID = "network:" + networkFullName(primaryNet)
		}
		snap.Nodes = append(snap.Nodes, node)

		for _, netName := range secondaryNets {
			fullNetName := networkFullName(netName)
			snap.Edges = append(snap.Edges, Edge{
				ID:     "e:net:" + svcName + ":" + fullNetName,
				Type:   "secondary_network",
				Source: containerID,
				Target: "network:" + fullNetName,
			})
		}

		for depName := range svc.DependsOn {
			depFullName := containerFullName(depName)
			snap.Edges = append(snap.Edges, Edge{
				ID:     "e:dep:" + svcName + ":" + depFullName,
				Type:   "depends_on",
				Source: containerID,
				Target: "container:" + depFullName,
			})
		}

		for _, v := range svc.Volumes {
			if v.Type == "volume" {
				fullVolName := volumeFullName(v.Source)
				snap.Edges = append(snap.Edges, Edge{
					ID:        "e:vol:" + fullVolName + ":" + svcName,
					Type:      "volume_mount",
					Source:    "volume:" + fullVolName,
					Target:    containerID,
					MountPath: v.Target,
				})
			}
		}
	}

	return snap, nil
}
