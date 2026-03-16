package collector

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	networktypes "github.com/docker/docker/api/types/network"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

const selfLabel = "dev.dockerflow.self"

func isSelfContainer(image string) bool {
	return strings.Contains(image, "docker-flow")
}

func buildContainerNode(id, name, image, status string, labels map[string]string, ports []PortMapping, primaryNetworkID string) Node {
	return Node{
		ID:        "container:" + id,
		Type:      "container",
		Name:      name,
		Image:     image,
		Status:    status,
		Labels:    labels,
		Ports:     ports,
		NetworkID: primaryNetworkID,
	}
}

func buildNetworkNode(id, name, driver string) Node {
	return Node{
		ID:     "network:" + id,
		Type:   "network",
		Name:   name,
		Driver: driver,
	}
}

func buildVolumeNode(name, driver string) Node {
	return Node{
		ID:     "volume:" + name,
		Type:   "volume",
		Name:   name,
		Driver: driver,
	}
}

type DockerCollector struct {
	client       client.APIClient
	pollInterval time.Duration
	updates      chan StateUpdate
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

func NewDockerCollector(cli client.APIClient, pollInterval time.Duration) *DockerCollector {
	return &DockerCollector{
		client:       cli,
		pollInterval: pollInterval,
		updates:      make(chan StateUpdate, 16),
		stopCh:       make(chan struct{}),
	}
}

func (d *DockerCollector) Updates() <-chan StateUpdate {
	return d.updates
}

func (d *DockerCollector) Start(ctx context.Context) error {
	if err := d.poll(ctx); err != nil {
		return err
	}

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.watchEvents(ctx)
	}()

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		ticker := time.NewTicker(d.pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := d.poll(ctx); err != nil {
					log.Printf("poll error: %v", err)
				}
			case <-d.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (d *DockerCollector) Stop() error {
	close(d.stopCh)
	d.wg.Wait()
	return nil
}

func (d *DockerCollector) poll(ctx context.Context) error {
	snapshot, err := d.buildSnapshot(ctx)
	if err != nil {
		return err
	}
	select {
	case d.updates <- StateUpdate{Snapshot: &snapshot}:
	case <-ctx.Done():
	}
	return nil
}

func (d *DockerCollector) buildSnapshot(ctx context.Context) (GraphSnapshot, error) {
	var snap GraphSnapshot

	containers, err := d.client.ContainerList(ctx, containertypes.ListOptions{All: true})
	if err != nil {
		return snap, err
	}

	networks, err := d.client.NetworkList(ctx, networktypes.ListOptions{})
	if err != nil {
		return snap, err
	}

	volResp, err := d.client.VolumeList(ctx, volumetypes.ListOptions{})
	if err != nil {
		return snap, err
	}

	networkIDToName := make(map[string]string)
	for _, n := range networks {
		if n.Name == "bridge" || n.Name == "host" || n.Name == "none" {
			continue
		}
		networkIDToName[n.ID] = n.Name
		snap.Nodes = append(snap.Nodes, buildNetworkNode(n.ID, n.Name, n.Driver))
	}

	for _, c := range containers {
		image := c.Image
		if isSelfContainer(image) {
			continue
		}
		if c.Labels[selfLabel] == "true" {
			continue
		}

		name := strings.TrimPrefix(c.Names[0], "/")
		status := containerStatus(c.State, c.Status)
		ports := extractPorts(c.Ports)

		var primaryNetID string
		var secondaryNetIDs []string
		if c.NetworkSettings != nil {
			for _, netSettings := range c.NetworkSettings.Networks {
				netID := netSettings.NetworkID
				if _, tracked := networkIDToName[netID]; !tracked {
					continue
				}
				if primaryNetID == "" {
					primaryNetID = netID
				} else {
					secondaryNetIDs = append(secondaryNetIDs, netID)
				}
			}
		}

		node := buildContainerNode(c.ID[:12], name, image, status, c.Labels, ports, "")
		if primaryNetID != "" {
			node.NetworkID = "network:" + primaryNetID
		}
		snap.Nodes = append(snap.Nodes, node)

		for _, netID := range secondaryNetIDs {
			snap.Edges = append(snap.Edges, Edge{
				ID:     "e:net:" + c.ID[:12] + ":" + netID[:12],
				Type:   "secondary_network",
				Source: "container:" + c.ID[:12],
				Target: "network:" + netID,
			})
		}

		for _, m := range c.Mounts {
			if m.Type == "volume" {
				snap.Edges = append(snap.Edges, Edge{
					ID:        "e:vol:" + m.Name + ":" + c.ID[:12],
					Type:      "volume_mount",
					Source:    "volume:" + m.Name,
					Target:    "container:" + c.ID[:12],
					MountPath: m.Destination,
				})
			}
		}
	}

	for _, v := range volResp.Volumes {
		snap.Nodes = append(snap.Nodes, buildVolumeNode(v.Name, v.Driver))
	}

	return snap, nil
}

func (d *DockerCollector) watchEvents(ctx context.Context) {
	eventFilter := filters.NewArgs()
	eventFilter.Add("type", string(events.ContainerEventType))
	eventFilter.Add("type", string(events.NetworkEventType))
	eventFilter.Add("type", string(events.VolumeEventType))

	msgCh, errCh := d.client.Events(ctx, events.ListOptions{Filters: eventFilter})

	for {
		select {
		case <-msgCh:
			if err := d.poll(ctx); err != nil {
				log.Printf("event-triggered poll error: %v", err)
			}
		case err := <-errCh:
			if err != nil && ctx.Err() == nil {
				log.Printf("docker events error: %v, reconnecting...", err)
				time.Sleep(2 * time.Second)
				msgCh, errCh = d.client.Events(ctx, events.ListOptions{Filters: eventFilter})
			}
			return
		case <-d.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

func containerStatus(state, statusStr string) string {
	if strings.Contains(statusStr, "(unhealthy)") {
		return "unhealthy"
	}
	switch state {
	case "running":
		return "running"
	case "paused":
		return "stopped"
	case "exited", "dead", "created":
		return "stopped"
	default:
		return state
	}
}

func extractPorts(apiPorts []containertypes.Port) []PortMapping {
	var ports []PortMapping
	for _, p := range apiPorts {
		if p.PublicPort != 0 {
			ports = append(ports, PortMapping{
				Host:      int(p.PublicPort),
				Container: int(p.PrivatePort),
			})
		}
	}
	return ports
}
