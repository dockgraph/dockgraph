package collector

import (
	"context"
	"fmt"

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
