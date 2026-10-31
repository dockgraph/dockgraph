package collector

import (
	"slices"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
)

func TestPortRanges(t *testing.T) {
	cases := map[string]struct {
		in, want []PortMapping
	}{
		"none":   {nil, nil},
		"single": {[]PortMapping{{Host: 80, Container: 80}}, []PortMapping{{Host: 80, Container: 80}}},
		"run": {
			[]PortMapping{{Host: 8002, Container: 82}, {Host: 8000, Container: 80}, {Host: 8001, Container: 81}},
			[]PortMapping{{Host: 8000, HostEnd: 8002, Container: 80, ContainerEnd: 82}},
		},
		"gaps and other offsets stay apart": {
			[]PortMapping{{Host: 8000, Container: 80}, {Host: 8002, Container: 82}, {Host: 8003, Container: 90}},
			[]PortMapping{{Host: 8000, Container: 80}, {Host: 8002, Container: 82}, {Host: 8003, Container: 90}},
		},
		"one run per protocol": {
			[]PortMapping{
				{Host: 10000, Container: 10000, Protocol: "tcp"},
				{Host: 10000, Container: 10000, Protocol: "udp"},
				{Host: 10001, Container: 10001, Protocol: "tcp"},
				{Host: 10001, Container: 10001, Protocol: "udp"},
			},
			[]PortMapping{
				{Host: 10000, HostEnd: 10001, Container: 10000, ContainerEnd: 10001, Protocol: "tcp"},
				{Host: 10000, HostEnd: 10001, Container: 10000, ContainerEnd: 10001, Protocol: "udp"},
			},
		},
		"a host range for one container port is kept": {
			[]PortMapping{{Host: 8000, HostEnd: 8010, Container: 80}, {Host: 8011, Container: 81}},
			[]PortMapping{{Host: 8000, HostEnd: 8010, Container: 80}, {Host: 8011, Container: 81}},
		},
		"unpublished ports stay apart": {
			[]PortMapping{{Container: 5000}, {Container: 5001}},
			[]PortMapping{{Container: 5000}, {Container: 5001}},
		},
		"sorted by host port": {
			[]PortMapping{{Host: 8080, Container: 80}, {Host: 443, Container: 443}},
			[]PortMapping{{Host: 443, Container: 443}, {Host: 8080, Container: 80}},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := PortRanges(c.in); !slices.Equal(got, c.want) {
				t.Errorf("PortRanges = %+v, want %+v", got, c.want)
			}
		})
	}
}

// A range of every port is one mapping, whatever the source.
func TestPortRangesOfWholeRange(t *testing.T) {
	var docker []containertypes.Port
	var service []swarm.PortConfig
	for port := uint16(1); port != 0; port++ {
		docker = append(docker, containertypes.Port{PublicPort: port, PrivatePort: port, Type: "udp"})
		service = append(service, swarm.PortConfig{PublishedPort: uint32(port), TargetPort: uint32(port), Protocol: "udp"})
	}
	want := PortMapping{Host: 1, HostEnd: 65535, Container: 1, ContainerEnd: 65535}
	if got := extractPorts(docker); len(got) != 1 || got[0] != want {
		t.Errorf("container ports = %+v, want [%+v]", got, want)
	}
	want.Protocol = "udp"
	if got := servicePorts(service); len(got) != 1 || got[0] != want {
		t.Errorf("service ports = %+v, want [%+v]", got, want)
	}
}
