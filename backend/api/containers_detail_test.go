package api

import (
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/go-connections/nat"
)

// networkSettingsWithPorts creates a NetworkSettings with the given port map,
// avoiding direct use of the deprecated NetworkSettingsBase struct literal.
func networkSettingsWithPorts(ports nat.PortMap) *containertypes.NetworkSettings {
	ns := &containertypes.NetworkSettings{}
	ns.Ports = ports
	return ns
}

func TestBuildPorts(t *testing.T) {
	t.Run("nil bindings for exposed port", func(t *testing.T) {
		info := containertypes.InspectResponse{
			NetworkSettings: networkSettingsWithPorts(nat.PortMap{
				"80/tcp": nil,
			}),
		}

		ports := buildPorts(info)
		if len(ports) != 0 {
			t.Errorf("expected 0 ports for nil bindings, got %d", len(ports))
		}
	})

	t.Run("empty port map", func(t *testing.T) {
		info := containertypes.InspectResponse{
			NetworkSettings: networkSettingsWithPorts(nat.PortMap{}),
		}

		ports := buildPorts(info)
		if len(ports) != 0 {
			t.Errorf("expected 0 ports, got %d", len(ports))
		}
	})

	t.Run("single port with single binding", func(t *testing.T) {
		info := containertypes.InspectResponse{
			NetworkSettings: networkSettingsWithPorts(nat.PortMap{
				"80/tcp": []nat.PortBinding{
					{HostIP: "0.0.0.0", HostPort: "8080"},
				},
			}),
		}

		ports := buildPorts(info)
		if len(ports) != 1 {
			t.Fatalf("expected 1 port, got %d", len(ports))
		}
		if ports[0]["hostPort"] != "8080" {
			t.Errorf("expected hostPort 8080, got %v", ports[0]["hostPort"])
		}
		if ports[0]["containerPort"] != "80" {
			t.Errorf("expected containerPort 80, got %v", ports[0]["containerPort"])
		}
		if ports[0]["protocol"] != "tcp" {
			t.Errorf("expected protocol tcp, got %v", ports[0]["protocol"])
		}
	})

	t.Run("multiple ports with multiple bindings", func(t *testing.T) {
		info := containertypes.InspectResponse{
			NetworkSettings: networkSettingsWithPorts(nat.PortMap{
				"80/tcp": []nat.PortBinding{
					{HostIP: "0.0.0.0", HostPort: "8080"},
					{HostIP: "0.0.0.0", HostPort: "9090"},
				},
				"443/tcp": []nat.PortBinding{
					{HostIP: "0.0.0.0", HostPort: "8443"},
				},
			}),
		}

		ports := buildPorts(info)
		if len(ports) != 3 {
			t.Fatalf("expected 3 port entries, got %d", len(ports))
		}

		// Collect all hostPorts to verify all bindings present
		hostPorts := make(map[string]bool)
		for _, p := range ports {
			hostPorts[p["hostPort"].(string)] = true
		}
		for _, expected := range []string{"8080", "9090", "8443"} {
			if !hostPorts[expected] {
				t.Errorf("expected hostPort %s in results", expected)
			}
		}
	})

	t.Run("udp protocol", func(t *testing.T) {
		info := containertypes.InspectResponse{
			NetworkSettings: networkSettingsWithPorts(nat.PortMap{
				"53/udp": []nat.PortBinding{
					{HostIP: "0.0.0.0", HostPort: "5353"},
				},
			}),
		}

		ports := buildPorts(info)
		if len(ports) != 1 {
			t.Fatalf("expected 1 port, got %d", len(ports))
		}
		if ports[0]["protocol"] != "udp" {
			t.Errorf("expected protocol udp, got %v", ports[0]["protocol"])
		}
	})
}

func TestBuildMounts(t *testing.T) {
	t.Run("named volume", func(t *testing.T) {
		info := containertypes.InspectResponse{
			Mounts: []containertypes.MountPoint{
				{
					Type:        mount.TypeVolume,
					Name:        "my-data",
					Source:      "/var/lib/docker/volumes/my-data/_data",
					Destination: "/data",
					RW:          true,
				},
			},
		}

		mounts := buildMounts(info)
		if len(mounts) != 1 {
			t.Fatalf("expected 1 mount, got %d", len(mounts))
		}
		m := mounts[0]
		if m["type"] != "volume" {
			t.Errorf("expected type volume, got %v", m["type"])
		}
		if m["name"] != "my-data" {
			t.Errorf("expected name my-data, got %v", m["name"])
		}
		if m["source"] != "/var/lib/docker/volumes/my-data/_data" {
			t.Errorf("unexpected source %v", m["source"])
		}
		if m["destination"] != "/data" {
			t.Errorf("expected destination /data, got %v", m["destination"])
		}
		if m["rw"] != true {
			t.Errorf("expected rw true, got %v", m["rw"])
		}
	})

	t.Run("bind mount", func(t *testing.T) {
		info := containertypes.InspectResponse{
			Mounts: []containertypes.MountPoint{
				{
					Type:        mount.TypeBind,
					Source:      "/host/path",
					Destination: "/container/path",
					RW:          false,
					Propagation: mount.PropagationRPrivate,
				},
			},
		}

		mounts := buildMounts(info)
		if len(mounts) != 1 {
			t.Fatalf("expected 1 mount, got %d", len(mounts))
		}
		m := mounts[0]
		if m["type"] != "bind" {
			t.Errorf("expected type bind, got %v", m["type"])
		}
		if _, hasName := m["name"]; hasName {
			t.Error("bind mount without name should not have 'name' key")
		}
		if m["rw"] != false {
			t.Errorf("expected rw false, got %v", m["rw"])
		}
		if m["propagation"] != "rprivate" {
			t.Errorf("expected propagation rprivate, got %v", m["propagation"])
		}
	})

	t.Run("empty name not included", func(t *testing.T) {
		info := containertypes.InspectResponse{
			Mounts: []containertypes.MountPoint{
				{
					Type:        mount.TypeBind,
					Name:        "",
					Source:      "/host",
					Destination: "/container",
					RW:          true,
				},
			},
		}

		mounts := buildMounts(info)
		if _, hasName := mounts[0]["name"]; hasName {
			t.Error("mount with empty name should not have 'name' key in result")
		}
	})

	t.Run("empty mounts", func(t *testing.T) {
		info := containertypes.InspectResponse{
			Mounts: []containertypes.MountPoint{},
		}

		mounts := buildMounts(info)
		if len(mounts) != 0 {
			t.Errorf("expected 0 mounts, got %d", len(mounts))
		}
	})

	t.Run("multiple mounts", func(t *testing.T) {
		info := containertypes.InspectResponse{
			Mounts: []containertypes.MountPoint{
				{
					Type:        mount.TypeVolume,
					Name:        "vol1",
					Source:      "/var/lib/docker/volumes/vol1/_data",
					Destination: "/app/data",
					RW:          true,
				},
				{
					Type:        mount.TypeBind,
					Source:      "/etc/config",
					Destination: "/app/config",
					RW:          false,
				},
			},
		}

		mounts := buildMounts(info)
		if len(mounts) != 2 {
			t.Fatalf("expected 2 mounts, got %d", len(mounts))
		}
	})
}

func TestBuildNetworks(t *testing.T) {
	t.Run("nil NetworkSettings", func(t *testing.T) {
		info := containertypes.InspectResponse{
			NetworkSettings: nil,
		}

		nets := buildNetworks(info)
		if len(nets) != 0 {
			t.Errorf("expected 0 networks for nil NetworkSettings, got %d", len(nets))
		}
	})

	t.Run("empty networks map", func(t *testing.T) {
		info := containertypes.InspectResponse{
			NetworkSettings: &containertypes.NetworkSettings{
				Networks: map[string]*network.EndpointSettings{},
			},
		}

		nets := buildNetworks(info)
		if len(nets) != 0 {
			t.Errorf("expected 0 networks, got %d", len(nets))
		}
	})

	t.Run("single network", func(t *testing.T) {
		info := containertypes.InspectResponse{
			NetworkSettings: &containertypes.NetworkSettings{
				Networks: map[string]*network.EndpointSettings{
					"bridge": {
						IPAddress:   "172.17.0.2",
						Gateway:     "172.17.0.1",
						MacAddress:  "02:42:ac:11:00:02",
						IPPrefixLen: 16,
					},
				},
			},
		}

		nets := buildNetworks(info)
		if len(nets) != 1 {
			t.Fatalf("expected 1 network, got %d", len(nets))
		}
		n := nets[0]
		if n["name"] != "bridge" {
			t.Errorf("expected name bridge, got %v", n["name"])
		}
		if n["ipAddress"] != "172.17.0.2" {
			t.Errorf("expected ipAddress 172.17.0.2, got %v", n["ipAddress"])
		}
		if n["gateway"] != "172.17.0.1" {
			t.Errorf("expected gateway 172.17.0.1, got %v", n["gateway"])
		}
		if n["macAddress"] != "02:42:ac:11:00:02" {
			t.Errorf("expected macAddress 02:42:ac:11:00:02, got %v", n["macAddress"])
		}
		if n["ipPrefixLen"] != 16 {
			t.Errorf("expected ipPrefixLen 16, got %v", n["ipPrefixLen"])
		}
	})

	t.Run("multiple networks", func(t *testing.T) {
		info := containertypes.InspectResponse{
			NetworkSettings: &containertypes.NetworkSettings{
				Networks: map[string]*network.EndpointSettings{
					"frontend": {
						IPAddress: "10.0.1.5",
						Gateway:   "10.0.1.1",
					},
					"backend": {
						IPAddress: "10.0.2.10",
						Gateway:   "10.0.2.1",
					},
				},
			},
		}

		nets := buildNetworks(info)
		if len(nets) != 2 {
			t.Fatalf("expected 2 networks, got %d", len(nets))
		}

		// Collect network names to verify both are present (order is map-dependent)
		names := make(map[string]bool)
		for _, n := range nets {
			names[n["name"].(string)] = true
		}
		if !names["frontend"] {
			t.Error("expected 'frontend' network in results")
		}
		if !names["backend"] {
			t.Error("expected 'backend' network in results")
		}
	})
}

func TestBuildSecurity(t *testing.T) {
	t.Run("privileged with capabilities", func(t *testing.T) {
		info := containertypes.InspectResponse{
			ContainerJSONBase: &containertypes.ContainerJSONBase{
				HostConfig: &containertypes.HostConfig{
					Privileged:     true,
					ReadonlyRootfs: true,
					CapAdd:         strslice.StrSlice{"NET_ADMIN", "SYS_PTRACE"},
					CapDrop:        strslice.StrSlice{"ALL"},
				},
			},
		}

		sec := buildSecurity(info)
		if sec["privileged"] != true {
			t.Errorf("expected privileged true, got %v", sec["privileged"])
		}
		if sec["readonlyRootfs"] != true {
			t.Errorf("expected readonlyRootfs true, got %v", sec["readonlyRootfs"])
		}

		capAdd, ok := sec["capAdd"].(strslice.StrSlice)
		if !ok {
			t.Fatal("expected capAdd to be strslice.StrSlice")
		}
		if len(capAdd) != 2 {
			t.Errorf("expected 2 capAdd entries, got %d", len(capAdd))
		}

		capDrop, ok := sec["capDrop"].(strslice.StrSlice)
		if !ok {
			t.Fatal("expected capDrop to be strslice.StrSlice")
		}
		if len(capDrop) != 1 || capDrop[0] != "ALL" {
			t.Errorf("expected capDrop [ALL], got %v", capDrop)
		}
	})

	t.Run("default unprivileged", func(t *testing.T) {
		info := containertypes.InspectResponse{
			ContainerJSONBase: &containertypes.ContainerJSONBase{
				HostConfig: &containertypes.HostConfig{},
			},
		}

		sec := buildSecurity(info)
		if sec["privileged"] != false {
			t.Errorf("expected privileged false, got %v", sec["privileged"])
		}
		if sec["readonlyRootfs"] != false {
			t.Errorf("expected readonlyRootfs false, got %v", sec["readonlyRootfs"])
		}
	})

	t.Run("nil capabilities", func(t *testing.T) {
		info := containertypes.InspectResponse{
			ContainerJSONBase: &containertypes.ContainerJSONBase{
				HostConfig: &containertypes.HostConfig{
					CapAdd:  nil,
					CapDrop: nil,
				},
			},
		}

		sec := buildSecurity(info)
		// nil StrSlice should be returned as-is (nil), not panic
		if sec["capAdd"] != nil {
			capAdd := sec["capAdd"].(strslice.StrSlice)
			if capAdd != nil {
				t.Errorf("expected nil capAdd, got %v", capAdd)
			}
		}
		if sec["capDrop"] != nil {
			capDrop := sec["capDrop"].(strslice.StrSlice)
			if capDrop != nil {
				t.Errorf("expected nil capDrop, got %v", capDrop)
			}
		}
	})
}

func TestBuildResources(t *testing.T) {
	t.Run("limits set", func(t *testing.T) {
		info := containertypes.InspectResponse{
			ContainerJSONBase: &containertypes.ContainerJSONBase{
				HostConfig: &containertypes.HostConfig{
					Resources: containertypes.Resources{
						CPUQuota:          50000,
						CPUPeriod:         100000,
						NanoCPUs:          1500000000,
						Memory:            536870912, // 512MB
						MemoryReservation: 268435456, // 256MB
					},
				},
			},
		}

		res := buildResources(info)
		if res["cpuQuota"] != int64(50000) {
			t.Errorf("expected cpuQuota 50000, got %v", res["cpuQuota"])
		}
		if res["cpuPeriod"] != int64(100000) {
			t.Errorf("expected cpuPeriod 100000, got %v", res["cpuPeriod"])
		}
		if res["nanoCpus"] != int64(1500000000) {
			t.Errorf("expected nanoCpus 1500000000, got %v", res["nanoCpus"])
		}
		if res["memoryLimit"] != int64(536870912) {
			t.Errorf("expected memoryLimit 536870912, got %v", res["memoryLimit"])
		}
		if res["memoryReservation"] != int64(268435456) {
			t.Errorf("expected memoryReservation 268435456, got %v", res["memoryReservation"])
		}
	})

	t.Run("no limits (zero values)", func(t *testing.T) {
		info := containertypes.InspectResponse{
			ContainerJSONBase: &containertypes.ContainerJSONBase{
				HostConfig: &containertypes.HostConfig{},
			},
		}

		res := buildResources(info)
		if res["cpuQuota"] != int64(0) {
			t.Errorf("expected cpuQuota 0, got %v", res["cpuQuota"])
		}
		if res["cpuPeriod"] != int64(0) {
			t.Errorf("expected cpuPeriod 0, got %v", res["cpuPeriod"])
		}
		if res["nanoCpus"] != int64(0) {
			t.Errorf("expected nanoCpus 0, got %v", res["nanoCpus"])
		}
		if res["memoryLimit"] != int64(0) {
			t.Errorf("expected memoryLimit 0, got %v", res["memoryLimit"])
		}
		if res["memoryReservation"] != int64(0) {
			t.Errorf("expected memoryReservation 0, got %v", res["memoryReservation"])
		}
	})
}

func TestBuildHealth(t *testing.T) {
	t.Run("healthy with log entries", func(t *testing.T) {
		start := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
		end := time.Date(2025, 6, 15, 10, 0, 1, 0, time.UTC)

		info := containertypes.InspectResponse{
			ContainerJSONBase: &containertypes.ContainerJSONBase{
				State: &containertypes.State{
					Health: &containertypes.Health{
						Status:        "healthy",
						FailingStreak: 0,
						Log: []*containertypes.HealthcheckResult{
							{
								Start:    start,
								End:      end,
								ExitCode: 0,
								Output:   "OK",
							},
						},
					},
				},
			},
		}

		h := buildHealth(info)
		if h["status"] != "healthy" {
			t.Errorf("expected status healthy, got %v", h["status"])
		}
		if h["failingStreak"] != 0 {
			t.Errorf("expected failingStreak 0, got %v", h["failingStreak"])
		}

		logs, ok := h["log"].([]map[string]any)
		if !ok {
			t.Fatal("expected log to be []map[string]any")
		}
		if len(logs) != 1 {
			t.Fatalf("expected 1 log entry, got %d", len(logs))
		}
		entry := logs[0]
		if entry["start"] != start {
			t.Errorf("expected start %v, got %v", start, entry["start"])
		}
		if entry["end"] != end {
			t.Errorf("expected end %v, got %v", end, entry["end"])
		}
		if entry["exitCode"] != 0 {
			t.Errorf("expected exitCode 0, got %v", entry["exitCode"])
		}
		if entry["output"] != "OK" {
			t.Errorf("expected output OK, got %v", entry["output"])
		}
	})

	t.Run("unhealthy with failing streak", func(t *testing.T) {
		info := containertypes.InspectResponse{
			ContainerJSONBase: &containertypes.ContainerJSONBase{
				State: &containertypes.State{
					Health: &containertypes.Health{
						Status:        "unhealthy",
						FailingStreak: 5,
						Log: []*containertypes.HealthcheckResult{
							{
								Start:    time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC),
								End:      time.Date(2025, 6, 15, 10, 0, 3, 0, time.UTC),
								ExitCode: 1,
								Output:   "connection refused",
							},
							{
								Start:    time.Date(2025, 6, 15, 10, 0, 30, 0, time.UTC),
								End:      time.Date(2025, 6, 15, 10, 0, 33, 0, time.UTC),
								ExitCode: 1,
								Output:   "timeout",
							},
						},
					},
				},
			},
		}

		h := buildHealth(info)
		if h["status"] != "unhealthy" {
			t.Errorf("expected status unhealthy, got %v", h["status"])
		}
		if h["failingStreak"] != 5 {
			t.Errorf("expected failingStreak 5, got %v", h["failingStreak"])
		}

		logs := h["log"].([]map[string]any)
		if len(logs) != 2 {
			t.Fatalf("expected 2 log entries, got %d", len(logs))
		}
		if logs[0]["exitCode"] != 1 {
			t.Errorf("expected first entry exitCode 1, got %v", logs[0]["exitCode"])
		}
		if logs[1]["output"] != "timeout" {
			t.Errorf("expected second entry output 'timeout', got %v", logs[1]["output"])
		}
	})

	t.Run("empty health log", func(t *testing.T) {
		info := containertypes.InspectResponse{
			ContainerJSONBase: &containertypes.ContainerJSONBase{
				State: &containertypes.State{
					Health: &containertypes.Health{
						Status:        "starting",
						FailingStreak: 0,
						Log:           []*containertypes.HealthcheckResult{},
					},
				},
			},
		}

		h := buildHealth(info)
		if h["status"] != "starting" {
			t.Errorf("expected status starting, got %v", h["status"])
		}

		logs := h["log"].([]map[string]any)
		if len(logs) != 0 {
			t.Errorf("expected 0 log entries, got %d", len(logs))
		}
	})

	t.Run("nil health log slice", func(t *testing.T) {
		info := containertypes.InspectResponse{
			ContainerJSONBase: &containertypes.ContainerJSONBase{
				State: &containertypes.State{
					Health: &containertypes.Health{
						Status:        "healthy",
						FailingStreak: 0,
						Log:           nil,
					},
				},
			},
		}

		h := buildHealth(info)
		logs := h["log"].([]map[string]any)
		if len(logs) != 0 {
			t.Errorf("expected 0 log entries for nil log, got %d", len(logs))
		}
	})

	t.Run("timestamp precision preserved", func(t *testing.T) {
		precise := time.Date(2025, 6, 15, 10, 30, 45, 123456789, time.UTC)

		info := containertypes.InspectResponse{
			ContainerJSONBase: &containertypes.ContainerJSONBase{
				State: &containertypes.State{
					Health: &containertypes.Health{
						Status: "healthy",
						Log: []*containertypes.HealthcheckResult{
							{
								Start:    precise,
								End:      precise.Add(500 * time.Millisecond),
								ExitCode: 0,
								Output:   "",
							},
						},
					},
				},
			},
		}

		h := buildHealth(info)
		logs := h["log"].([]map[string]any)
		startTime := logs[0]["start"].(time.Time)
		if !startTime.Equal(precise) {
			t.Errorf("timestamp precision lost: expected %v, got %v", precise, startTime)
		}
	})
}
