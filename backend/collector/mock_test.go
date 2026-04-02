package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	networktypes "github.com/docker/docker/api/types/network"
	volumetypes "github.com/docker/docker/api/types/volume"
)

// stubDockerClient implements DockerClient for testing.
type stubDockerClient struct {
	containers []containertypes.Summary
	networks   []networktypes.Summary
	volumes    []*volumetypes.Volume

	containerErr error
	networkErr   error
	volumeErr    error

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
