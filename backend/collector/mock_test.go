package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	dockertypes "github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	imagetypes "github.com/docker/docker/api/types/image"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/swarm"
	systemtypes "github.com/docker/docker/api/types/system"
	volumetypes "github.com/docker/docker/api/types/volume"
)

// stubDockerClient implements DockerClient for testing.
type stubDockerClient struct {
	containers []containertypes.Summary
	networks   []networktypes.Summary
	volumes    []*volumetypes.Volume

	services []swarm.Service
	tasks    []swarm.Task
	nodes    []swarm.Node

	containerErr error
	networkErr   error
	volumeErr    error
	serviceErr   error
	taskErr      error
	nodeErr      error

	// taskListCalls counts TaskList invocations (guarded by mu).
	mu            sync.Mutex
	taskListCalls int

	eventsCh <-chan events.Message
	errCh    <-chan error

	// statsFn, when set, overrides the default ContainerStats behavior.
	// It receives the containerID and returns the response or error.
	statsFn func(containerID string) (containertypes.StatsResponseReader, error)
}

func (s *stubDockerClient) ContainerList(_ context.Context, _ containertypes.ListOptions) ([]containertypes.Summary, error) {
	return s.containers, s.containerErr
}

func (s *stubDockerClient) NetworkList(_ context.Context, _ networktypes.ListOptions) ([]networktypes.Summary, error) {
	return s.networks, s.networkErr
}

func (s *stubDockerClient) VolumeList(_ context.Context, _ volumetypes.ListOptions) (volumetypes.ListResponse, error) {
	return volumetypes.ListResponse{Volumes: s.volumes}, s.volumeErr
}

func (s *stubDockerClient) Events(_ context.Context, _ events.ListOptions) (<-chan events.Message, <-chan error) {
	if s.eventsCh != nil {
		return s.eventsCh, s.errCh
	}
	ch := make(chan events.Message)
	errCh := make(chan error)
	return ch, errCh
}

func (s *stubDockerClient) ContainerStats(_ context.Context, containerID string, _ bool) (containertypes.StatsResponseReader, error) {
	if s.statsFn != nil {
		return s.statsFn(containerID)
	}
	return containertypes.StatsResponseReader{Body: io.NopCloser(io.LimitReader(nil, 0))}, fmt.Errorf("not implemented in stub")
}

func (s *stubDockerClient) ContainerInspect(_ context.Context, _ string) (containertypes.InspectResponse, error) {
	return containertypes.InspectResponse{}, fmt.Errorf("not implemented in stub")
}

func (s *stubDockerClient) ContainerLogs(_ context.Context, _ string, _ containertypes.LogsOptions) (io.ReadCloser, error) {
	return nil, fmt.Errorf("not implemented in stub")
}

func (s *stubDockerClient) VolumeInspect(_ context.Context, _ string) (volumetypes.Volume, error) {
	return volumetypes.Volume{}, fmt.Errorf("not implemented in stub")
}

func (s *stubDockerClient) NetworkInspect(_ context.Context, _ string, _ networktypes.InspectOptions) (networktypes.Inspect, error) {
	return networktypes.Inspect{}, fmt.Errorf("not implemented in stub")
}

func (s *stubDockerClient) Info(_ context.Context) (systemtypes.Info, error) {
	return systemtypes.Info{}, nil
}

func (s *stubDockerClient) DiskUsage(_ context.Context, _ dockertypes.DiskUsageOptions) (dockertypes.DiskUsage, error) {
	return dockertypes.DiskUsage{}, nil
}

func (s *stubDockerClient) ImageList(_ context.Context, _ imagetypes.ListOptions) ([]imagetypes.Summary, error) {
	return nil, nil
}

func (s *stubDockerClient) ServiceList(_ context.Context, _ swarm.ServiceListOptions) ([]swarm.Service, error) {
	return s.services, s.serviceErr
}

func (s *stubDockerClient) TaskList(_ context.Context, _ swarm.TaskListOptions) ([]swarm.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.taskListCalls++
	return s.tasks, s.taskErr
}

func (s *stubDockerClient) NodeList(_ context.Context, _ swarm.NodeListOptions) ([]swarm.Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nodes, s.nodeErr
}

// setNodes replaces the node list returned by NodeList (safe for concurrent use).
func (s *stubDockerClient) setNodes(nodes []swarm.Node) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes = nodes
}

// setTasks replaces the task list returned by TaskList (safe for concurrent use).
func (s *stubDockerClient) setTasks(tasks []swarm.Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks = tasks
}

func (s *stubDockerClient) Close() error { return nil }

// errClient returns a stub that fails on the specified resource.
func errClient(resource string) *stubDockerClient {
	c := &stubDockerClient{}
	switch resource {
	case "containers":
		c.containerErr = fmt.Errorf("container list failed")
	case "networks":
		c.networkErr = fmt.Errorf("network list failed")
	case "volumes":
		c.volumeErr = fmt.Errorf("volume list failed")
	case "services":
		c.serviceErr = fmt.Errorf("service list failed")
	case "tasks":
		c.taskErr = fmt.Errorf("task list failed")
	case "nodes":
		c.nodeErr = fmt.Errorf("node list failed")
	}
	return c
}

// fakeStatsBody returns a StatsResponseReader whose Body contains the given
// StatsResponse encoded as JSON. This is the format the Docker daemon uses
// for one-shot (stream=false) stats responses.
func fakeStatsBody(stats containertypes.StatsResponse) containertypes.StatsResponseReader {
	buf, _ := json.Marshal(stats)
	return containertypes.StatsResponseReader{
		Body: io.NopCloser(bytes.NewReader(buf)),
	}
}

// fakeStatsBodyRaw returns a StatsResponseReader whose Body contains the given
// raw bytes, useful for testing malformed JSON responses.
func fakeStatsBodyRaw(data []byte) containertypes.StatsResponseReader {
	return containertypes.StatsResponseReader{
		Body: io.NopCloser(bytes.NewReader(data)),
	}
}
