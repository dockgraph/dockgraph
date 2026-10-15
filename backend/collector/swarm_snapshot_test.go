package collector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/mount"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/swarm"
	volumetypes "github.com/docker/docker/api/types/volume"
)

// --- fixtures ---

func uptr(n uint64) *uint64 { return &n }

// stackService builds a replicated stack service attached to the given network IDs.
func stackService(id, stack, svc string, replicas uint64, netIDs ...string) swarm.Service {
	s := swarm.Service{ID: id}
	s.Spec.Name = stack + "_" + svc
	s.Spec.Labels = map[string]string{StackNamespaceLabel: stack}
	s.Spec.Mode.Replicated = &swarm.ReplicatedService{Replicas: uptr(replicas)}
	s.Spec.TaskTemplate.ContainerSpec = &swarm.ContainerSpec{Image: "nginx:1.25@sha256:abcdef"}
	for _, n := range netIDs {
		s.Spec.TaskTemplate.Networks = append(s.Spec.TaskTemplate.Networks, swarm.NetworkAttachmentConfig{Target: n})
	}
	return s
}

func task(id, serviceID, nodeID string, slot int, state, desired swarm.TaskState) swarm.Task {
	return swarm.Task{
		ID:           id,
		ServiceID:    serviceID,
		NodeID:       nodeID,
		Slot:         slot,
		DesiredState: desired,
		Status:       swarm.TaskStatus{State: state, Timestamp: time.Unix(1700000000, 0)},
	}
}

func swarmNode(id, hostname string) swarm.Node {
	n := swarm.Node{ID: id}
	n.Description.Hostname = hostname
	n.Status.State = swarm.NodeStateReady
	n.Spec.Availability = swarm.NodeAvailabilityActive
	return n
}

func findNodeByID(nodes []Node, id string) *Node {
	for i := range nodes {
		if nodes[i].ID == id {
			return &nodes[i]
		}
	}
	return nil
}

func hasEdge(edges []Edge, id string) bool {
	for _, e := range edges {
		if e.ID == id {
			return true
		}
	}
	return false
}

// shopResources is a two-service "shop" stack on a two-node cluster.
func shopResources() swarmResources {
	web := stackService("svc-web", "shop", "web", 3, "net-front", "net-back", "net-ingress")
	web.Endpoint.Ports = []swarm.PortConfig{
		{TargetPort: 80, PublishedPort: 8080, Protocol: swarm.PortConfigProtocolTCP},
		{TargetPort: 9000}, // unpublished
	}
	web.Spec.TaskTemplate.ContainerSpec.Mounts = []mount.Mount{
		{Type: mount.TypeVolume, Source: "shop_data", Target: "/data"},
		{Type: mount.TypeBind, Source: "/host", Target: "/host"},
	}
	db := stackService("svc-db", "shop", "db", 1, "net-back")

	return swarmResources{
		dockerResources: dockerResources{
			networks: []networktypes.Summary{
				{ID: "net-front", Name: "shop_front", Driver: "overlay", Labels: map[string]string{StackNamespaceLabel: "shop"}},
				{ID: "net-back", Name: "shop_back", Driver: "overlay", Labels: map[string]string{StackNamespaceLabel: "shop"}},
				{ID: "net-ingress", Name: "ingress", Driver: "overlay", Ingress: true},
				{ID: "net-gw", Name: "docker_gwbridge", Driver: "bridge"},
			},
			containers: []containertypes.Summary{
				// Task container of shop_web on this node: represented by the service.
				{ID: "c1", Names: []string{"/shop_web.1.abc"}, State: "running", Labels: map[string]string{
					swarmTaskIDLabel: "t1", StackNamespaceLabel: "shop",
				}},
				// Standalone container: still shown.
				{ID: "c2", Names: []string{"/tool"}, State: "running"},
			},
		},
		services: []swarm.Service{web, db},
		tasks: []swarm.Task{
			task("t1", "svc-web", "n1", 1, swarm.TaskStateRunning, swarm.TaskStateRunning),
			task("t2", "svc-web", "n2", 2, swarm.TaskStateRunning, swarm.TaskStateRunning),
			task("t3", "svc-web", "n2", 3, swarm.TaskStatePreparing, swarm.TaskStateRunning),
			task("t0", "svc-web", "n1", 3, swarm.TaskStateShutdown, swarm.TaskStateShutdown), // history, hidden
			task("t4", "svc-db", "n1", 1, swarm.TaskStateRunning, swarm.TaskStateRunning),
		},
		nodes: []swarm.Node{swarmNode("n1", "manager-1"), swarmNode("n2", "worker-1")},
	}
}

// --- fetchSwarmResources ---

func TestFetchSwarmResources(t *testing.T) {
	cli := &stubDockerClient{
		services: []swarm.Service{{ID: "s1"}},
		tasks:    []swarm.Task{{ID: "t1"}},
		nodes:    []swarm.Node{{ID: "n1"}},
		networks: []networktypes.Summary{{ID: "net", Name: "net"}},
	}
	res, err := fetchSwarmResources(context.Background(), cli)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.services) != 1 || len(res.tasks) != 1 || len(res.nodes) != 1 || len(res.networks) != 1 {
		t.Errorf("unexpected resources: %+v", res)
	}
}

func TestFetchSwarmResourcesErrors(t *testing.T) {
	for _, resource := range []string{"containers", "networks", "volumes", "services", "tasks", "nodes"} {
		t.Run(resource, func(t *testing.T) {
			if _, err := fetchSwarmResources(context.Background(), errClient(resource)); err == nil {
				t.Fatalf("expected error for %s failure", resource)
			}
		})
	}
}

// --- buildSwarmSnapshot ---

func TestBuildSwarmSnapshotServiceNode(t *testing.T) {
	snap := buildSwarmSnapshot(shopResources())

	web := findNodeByID(snap.Nodes, "service:shop_web")
	if web == nil {
		t.Fatalf("service node missing: %+v", snap.Nodes)
	}
	if web.Type != "service" || web.Stack != "shop" {
		t.Errorf("type/stack = %s/%s", web.Type, web.Stack)
	}
	if web.Image != "nginx:1.25" {
		t.Errorf("image digest not stripped: %s", web.Image)
	}
	if web.Labels != nil {
		t.Errorf("stack label forwarded in payload labels: %v", web.Labels)
	}
	if len(web.Ports) != 1 || web.Ports[0] != (PortMapping{Host: 8080, Container: 80, Protocol: "tcp"}) {
		t.Errorf("ports = %+v", web.Ports)
	}
	if web.Service == nil || web.Service.Mode != "replicated" {
		t.Fatalf("service info = %+v", web.Service)
	}
	if web.Service.Replicas != (ReplicaCount{Running: 2, Desired: 3}) {
		t.Errorf("replicas = %+v", web.Service.Replicas)
	}
	if web.Status != "degraded" {
		t.Errorf("status = %s, want degraded", web.Status)
	}

	// Shutdown history is hidden; active tasks carry node hostnames, sorted by slot.
	tasks := web.Service.Tasks
	if len(tasks) != 3 {
		t.Fatalf("tasks = %+v", tasks)
	}
	if tasks[0].ID != "t1" || tasks[0].NodeHostname != "manager-1" || tasks[1].NodeHostname != "worker-1" {
		t.Errorf("unexpected task order/hostnames: %+v", tasks)
	}

	db := findNodeByID(snap.Nodes, "service:shop_db")
	if db == nil || db.Status != StateRunning {
		t.Errorf("db node = %+v", db)
	}
}

func TestBuildSwarmSnapshotNetworks(t *testing.T) {
	snap := buildSwarmSnapshot(shopResources())

	for _, id := range []string{"network:ingress", "network:docker_gwbridge"} {
		if findNodeByID(snap.Nodes, id) != nil {
			t.Errorf("%s should be skipped", id)
		}
	}
	back := findNodeByID(snap.Nodes, "network:shop_back")
	if back == nil || back.Stack != "shop" {
		t.Fatalf("stack network missing or without stack: %+v", back)
	}

	web := findNodeByID(snap.Nodes, "service:shop_web")
	if web.NetworkID != "network:shop_back" {
		t.Errorf("primary network = %s, want network:shop_back (alphabetical among own-stack)", web.NetworkID)
	}
	if !hasEdge(snap.Edges, "e:net:shop_web:shop_front") {
		t.Errorf("missing secondary network edge: %+v", snap.Edges)
	}
	if hasEdge(snap.Edges, "e:net:shop_web:ingress") {
		t.Error("ingress must not produce an edge")
	}
}

func TestBuildSwarmSnapshotPrefersOwnStackNetwork(t *testing.T) {
	res := shopResources()
	// "aaa_shared" sorts first but belongs to another stack.
	res.networks = append(res.networks, networktypes.Summary{
		ID: "net-shared", Name: "aaa_shared", Driver: "overlay", Labels: map[string]string{StackNamespaceLabel: "other"},
	})
	res.services[1].Spec.TaskTemplate.Networks = append(res.services[1].Spec.TaskTemplate.Networks, swarm.NetworkAttachmentConfig{Target: "net-shared"})

	snap := buildSwarmSnapshot(res)
	db := findNodeByID(snap.Nodes, "service:shop_db")
	if db.NetworkID != "network:shop_back" {
		t.Errorf("primary = %s, want own-stack network", db.NetworkID)
	}
	if !hasEdge(snap.Edges, "e:net:shop_db:aaa_shared") {
		t.Error("shared network should be a secondary edge")
	}
}

func TestBuildSwarmSnapshotTaskContainersSuppressed(t *testing.T) {
	snap := buildSwarmSnapshot(shopResources())
	if findNodeByID(snap.Nodes, "container:shop_web.1.abc") != nil {
		t.Error("task container should be represented by its service")
	}
	if findNodeByID(snap.Nodes, "container:tool") == nil {
		t.Error("standalone container should be kept")
	}
}

func TestBuildSwarmSnapshotVolumes(t *testing.T) {
	res := shopResources()
	res.volumes = []*volumetypes.Volume{{Name: "local_only", Driver: "local"}}

	snap := buildSwarmSnapshot(res)
	vol := findNodeByID(snap.Nodes, "volume:shop_data")
	if vol == nil {
		t.Fatal("volume node for service mount missing")
	}
	if vol.Stack != "shop" {
		t.Errorf("volume stack = %q", vol.Stack)
	}
	if !hasEdge(snap.Edges, "e:vol:shop_data:shop_web") {
		t.Errorf("missing volume_mount edge: %+v", snap.Edges)
	}
	if findNodeByID(snap.Nodes, "volume:local_only") == nil {
		t.Error("local volume should still be listed")
	}
	if len(filterEdges(snap.Edges, "volume_mount")) != 1 {
		t.Error("bind mounts must not produce volume edges")
	}
}

func TestBuildSwarmSnapshotSelfExclusion(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*swarm.Service)
	}{
		{"service labels", func(s *swarm.Service) { s.Spec.Labels[SelfExcludeLabel] = "true" }},
		{"container labels", func(s *swarm.Service) {
			s.Spec.TaskTemplate.ContainerSpec.Labels = map[string]string{SelfExcludeLabel: "true"}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			self := stackService("svc-dg", "dockgraph", "server", 1, "net-dg")
			tt.mutate(&self)
			res := swarmResources{
				dockerResources: dockerResources{
					networks: []networktypes.Summary{
						{ID: "net-dg", Name: "dockgraph_default", Labels: map[string]string{StackNamespaceLabel: "dockgraph"}},
					},
					volumes: []*volumetypes.Volume{
						{Name: "dockgraph_data", Labels: map[string]string{StackNamespaceLabel: "dockgraph"}},
					},
					// The local task container carries only the container-spec
					// labels, so with deploy-level labels it isn't marked self.
					containers: []containertypes.Summary{{
						ID: "c-dg", Names: []string{"/dockgraph_server.1.abc"}, State: "running",
						Labels: map[string]string{StackNamespaceLabel: "dockgraph", swarmTaskIDLabel: "abc"},
					}},
				},
				services: []swarm.Service{self},
			}

			snap := buildSwarmSnapshot(res)
			if len(snap.Nodes) != 0 {
				t.Errorf("self-only stack should be hidden entirely, got %+v", snap.Nodes)
			}
		})
	}
}

func TestBuildSwarmSnapshotSelfSharedStackKeepsResources(t *testing.T) {
	self := stackService("svc-dg", "ops", "dockgraph", 1, "net-ops")
	self.Spec.Labels[SelfExcludeLabel] = "true"
	other := stackService("svc-mon", "ops", "monitor", 1, "net-ops")
	res := swarmResources{
		dockerResources: dockerResources{
			networks: []networktypes.Summary{{ID: "net-ops", Name: "ops_default", Labels: map[string]string{StackNamespaceLabel: "ops"}}},
		},
		services: []swarm.Service{self, other},
	}

	snap := buildSwarmSnapshot(res)
	if findNodeByID(snap.Nodes, "service:ops_dockgraph") != nil {
		t.Error("self service should be hidden")
	}
	if findNodeByID(snap.Nodes, "service:ops_monitor") == nil || findNodeByID(snap.Nodes, "network:ops_default") == nil {
		t.Error("shared stack resources should stay visible")
	}
}

// --- replicas / status ---

func TestDesiredReplicas(t *testing.T) {
	global := swarm.Service{}
	global.Spec.Mode.Global = &swarm.GlobalService{}

	globalWithStatus := global
	globalWithStatus.ServiceStatus = &swarm.ServiceStatus{DesiredTasks: 2}

	replicated := swarm.Service{}
	replicated.Spec.Mode.Replicated = &swarm.ReplicatedService{Replicas: uptr(4)}

	job := swarm.Service{}
	job.Spec.Mode.ReplicatedJob = &swarm.ReplicatedJob{}

	tests := []struct {
		name     string
		svc      swarm.Service
		eligible int
		want     int
	}{
		{"replicated", replicated, 5, 4},
		{"global uses node count", global, 5, 5},
		{"global prefers service status", globalWithStatus, 5, 2},
		{"job without status", job, 5, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := desiredReplicas(tt.svc, tt.eligible); got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCountEligibleNodes(t *testing.T) {
	down := swarmNode("n3", "down")
	down.Status.State = swarm.NodeStateDown
	drained := swarmNode("n4", "drained")
	drained.Spec.Availability = swarm.NodeAvailabilityDrain

	if got := countEligibleNodes([]swarm.Node{swarmNode("n1", "a"), swarmNode("n2", "b"), down, drained}); got != 2 {
		t.Errorf("got %d, want 2", got)
	}
}

func TestServiceStatus(t *testing.T) {
	tests := []struct {
		name             string
		running, desired int
		update           *swarm.UpdateStatus
		want             string
	}{
		{"all up", 3, 3, nil, "running"},
		{"partial", 1, 3, nil, "degraded"},
		{"none up", 0, 3, nil, "degraded"},
		{"scaled to zero", 0, 0, nil, "stopped"},
		{"updating", 3, 3, &swarm.UpdateStatus{State: swarm.UpdateStateUpdating}, "updating"},
		{"rolling back", 1, 3, &swarm.UpdateStatus{State: swarm.UpdateStateRollbackStarted}, "updating"},
		{"update completed", 3, 3, &swarm.UpdateStatus{State: swarm.UpdateStateCompleted}, "running"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := serviceStatus(tt.running, tt.desired, tt.update); got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestServiceModeNames(t *testing.T) {
	tests := []struct {
		mode swarm.ServiceMode
		want string
	}{
		{swarm.ServiceMode{Replicated: &swarm.ReplicatedService{}}, "replicated"},
		{swarm.ServiceMode{Global: &swarm.GlobalService{}}, "global"},
		{swarm.ServiceMode{ReplicatedJob: &swarm.ReplicatedJob{}}, "replicated-job"},
		{swarm.ServiceMode{GlobalJob: &swarm.GlobalJob{}}, "global-job"},
	}
	for _, tt := range tests {
		if got := serviceMode(tt.mode); got != tt.want {
			t.Errorf("got %s, want %s", got, tt.want)
		}
	}
}

func TestBuildServiceInfoCapsFailedTasks(t *testing.T) {
	svc := stackService("s", "st", "app", 1)
	var tasks []swarm.Task
	for i := 0; i < 6; i++ {
		ft := task("f"+string(rune('a'+i)), "s", "n1", 1, swarm.TaskStateFailed, swarm.TaskStateShutdown)
		ft.Status.Timestamp = time.Unix(int64(1000+i), 0)
		ft.Status.Err = "exit 1"
		tasks = append(tasks, ft)
	}
	tasks = append(tasks, task("run", "s", "n1", 1, swarm.TaskStateRunning, swarm.TaskStateRunning))

	info := buildServiceInfo(svc, tasks, map[string]string{"n1": "host"}, 1)
	if len(info.Tasks) != 1+maxFailedTasks {
		t.Fatalf("got %d tasks, want %d", len(info.Tasks), 1+maxFailedTasks)
	}
	// The most recent failures are kept.
	ids := map[string]bool{}
	for _, ti := range info.Tasks {
		ids[ti.ID] = true
	}
	for _, id := range []string{"fd", "fe", "ff", "run"} {
		if !ids[id] {
			t.Errorf("expected task %s in %v", id, ids)
		}
	}
	if info.Replicas.Running != 1 {
		t.Errorf("running = %d", info.Replicas.Running)
	}
}

func TestBuildServiceInfoCapsCompletedJobTasks(t *testing.T) {
	svc := stackService("s", "st", "migrate", 1)
	svc.Spec.Mode = swarm.ServiceMode{ReplicatedJob: &swarm.ReplicatedJob{}}
	var tasks []swarm.Task
	// Job tasks keep desired state "complete" once they have finished.
	for i := 0; i < 50; i++ {
		ct := task(fmt.Sprintf("c%02d", i), "s", "n1", i+1, swarm.TaskStateComplete, swarm.TaskStateComplete)
		ct.Status.Timestamp = time.Unix(int64(1000+i), 0)
		tasks = append(tasks, ct)
	}
	// Still running or waiting to: current work, always listed.
	tasks = append(tasks,
		task("run", "s", "n1", 51, swarm.TaskStateRunning, swarm.TaskStateComplete),
		task("wait", "s", "", 52, swarm.TaskStatePending, swarm.TaskStateComplete),
	)

	info := buildServiceInfo(svc, tasks, map[string]string{"n1": "host"}, 1)
	ids := map[string]bool{}
	for _, ti := range info.Tasks {
		ids[ti.ID] = true
	}
	if len(info.Tasks) != 2+maxCompletedTasks {
		t.Fatalf("got %d tasks %v, want %d", len(info.Tasks), ids, 2+maxCompletedTasks)
	}
	// Active tasks and the most recent completions are kept.
	for _, id := range []string{"run", "wait", "c47", "c48", "c49"} {
		if !ids[id] {
			t.Errorf("expected task %s in %v", id, ids)
		}
	}
	if info.Replicas.Running != 1 {
		t.Errorf("running = %d", info.Replicas.Running)
	}
}

func TestStripImageDigest(t *testing.T) {
	tests := map[string]string{
		"nginx:1.25@sha256:abc": "nginx:1.25",
		"nginx":                 "nginx",
		"":                      "",
	}
	for in, want := range tests {
		if got := stripImageDigest(in); got != want {
			t.Errorf("stripImageDigest(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestServiceNetworksFallbackToVirtualIPs(t *testing.T) {
	svc := swarm.Service{}
	svc.Endpoint.VirtualIPs = []swarm.EndpointVirtualIP{{NetworkID: "n1"}, {NetworkID: "ing"}}
	got := serviceNetworks(svc, map[string]string{"n1": "app_net"})
	if len(got) != 1 || got[0] != "app_net" {
		t.Errorf("got %v", got)
	}
}

// --- fingerprint ---

func TestTaskFingerprint(t *testing.T) {
	a := task("t1", "s", "n1", 1, swarm.TaskStateRunning, swarm.TaskStateRunning)
	b := task("t2", "s", "n2", 2, swarm.TaskStateRunning, swarm.TaskStateRunning)

	if taskFingerprint([]swarm.Task{a, b}, nil) != taskFingerprint([]swarm.Task{b, a}, nil) {
		t.Error("fingerprint must not depend on order")
	}
	moved := b
	moved.NodeID = "n1"
	if taskFingerprint([]swarm.Task{a, b}, nil) == taskFingerprint([]swarm.Task{a, moved}, nil) {
		t.Error("fingerprint must change when a task moves")
	}
	failed := b
	failed.Status.State = swarm.TaskStateFailed
	if taskFingerprint([]swarm.Task{a, b}, nil) == taskFingerprint([]swarm.Task{a, failed}, nil) {
		t.Error("fingerprint must change when a task state changes")
	}
}

func TestTaskFingerprintNodes(t *testing.T) {
	tasks := []swarm.Task{task("t1", "s", "n1", 1, swarm.TaskStateRunning, swarm.TaskStateRunning)}
	n1, n2 := swarmNode("n1", "a"), swarmNode("n2", "b")
	base := taskFingerprint(tasks, []swarm.Node{n1, n2})

	if base != taskFingerprint(tasks, []swarm.Node{n2, n1}) {
		t.Error("fingerprint must not depend on node order")
	}

	drained := n2
	drained.Spec.Availability = swarm.NodeAvailabilityDrain
	down := n2
	down.Status.State = swarm.NodeStateDown
	promoted := n2
	promoted.Spec.Role = swarm.NodeRoleManager
	leader := n2
	leader.ManagerStatus = &swarm.ManagerStatus{Leader: true}
	for name, changed := range map[string]swarm.Node{"drain": drained, "down": down, "role": promoted, "leader": leader} {
		if base == taskFingerprint(tasks, []swarm.Node{n1, changed}) {
			t.Errorf("fingerprint must change on node %s", name)
		}
	}
	if base == taskFingerprint(tasks, []swarm.Node{n1}) {
		t.Error("fingerprint must change when a node leaves")
	}
}

// --- collector in swarm mode ---

func TestDockerCollectorSwarmModeSnapshot(t *testing.T) {
	res := shopResources()
	cli := &stubDockerClient{
		containers: res.containers,
		networks:   res.networks,
		services:   res.services,
		tasks:      res.tasks,
		nodes:      res.nodes,
	}
	dc := NewDockerCollector(cli, time.Hour)
	dc.EnableSwarm(0) // no task poller; exercised separately

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := dc.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer dc.Stop()

	select {
	case u := <-dc.Updates():
		if findNodeByID(u.Snapshot.Nodes, "service:shop_web") == nil {
			t.Errorf("swarm snapshot missing service node: %+v", u.Snapshot.Nodes)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for initial update")
	}
}

func TestDockerCollectorCheckTasksResnapshotsOnlyOnChange(t *testing.T) {
	res := shopResources()
	cli := &stubDockerClient{services: res.services, tasks: res.tasks, nodes: res.nodes}
	dc := NewDockerCollector(cli, time.Hour)
	dc.EnableSwarm(time.Hour)
	ctx := context.Background()

	if err := dc.poll(ctx); err != nil {
		t.Fatalf("poll: %v", err)
	}
	<-dc.Updates()

	// Unchanged task set: no re-snapshot.
	dc.checkTasks(ctx)
	select {
	case <-dc.Updates():
		t.Fatal("unexpected re-snapshot for unchanged tasks")
	default:
	}

	// A task moves to another node: re-snapshot.
	moved := append([]swarm.Task(nil), res.tasks...)
	moved[0].NodeID = "n2"
	cli.setTasks(moved)
	dc.checkTasks(ctx)
	select {
	case <-dc.Updates():
	default:
		t.Fatal("expected re-snapshot after task change")
	}

	// A node is drained, tasks unchanged: re-snapshot.
	nodes := append([]swarm.Node(nil), res.nodes...)
	nodes[1].Spec.Availability = swarm.NodeAvailabilityDrain
	cli.setNodes(nodes)
	dc.checkTasks(ctx)
	select {
	case u := <-dc.Updates():
		n := findNodeByID(u.Snapshot.Nodes, "swarmnode:worker-1")
		if n == nil || n.SwarmNode == nil || n.SwarmNode.Availability != "drain" {
			t.Fatalf("drained node not reflected: %+v", n)
		}
	default:
		t.Fatal("expected re-snapshot after node drain")
	}

	// Node list fails: no re-snapshot.
	cli.mu.Lock()
	cli.nodeErr = context.DeadlineExceeded
	cli.mu.Unlock()
	dc.checkTasks(ctx)
	select {
	case <-dc.Updates():
		t.Fatal("unexpected re-snapshot when node list fails")
	default:
	}
}

// --- swarm node graph nodes ---

func TestBuildSwarmSnapshotSwarmNodes(t *testing.T) {
	res := shopResources()

	leader := swarmNode("n1", "manager-1")
	leader.Spec.Role = swarm.NodeRoleManager
	leader.ManagerStatus = &swarm.ManagerStatus{Leader: true, Reachability: swarm.ReachabilityReachable}
	leader.Status.Addr = "10.0.0.1"
	leader.Description.Engine.EngineVersion = "27.3.1"
	leader.Description.Resources = swarm.Resources{NanoCPUs: 4e9, MemoryBytes: 8 << 30}

	down := swarmNode("n2", "worker-1")
	down.Spec.Role = swarm.NodeRoleWorker
	down.Status.State = swarm.NodeStateDown

	drained := swarmNode("n3", "worker-2")
	drained.Spec.Role = swarm.NodeRoleWorker
	drained.Spec.Availability = swarm.NodeAvailabilityDrain

	noState := swarm.Node{ID: "n4"} // no hostname, no state
	noState.Spec.Role = swarm.NodeRoleWorker

	res.nodes = []swarm.Node{leader, down, drained, noState}
	snap := buildSwarmSnapshot(res)

	m := findNodeByID(snap.Nodes, "swarmnode:manager-1")
	if m == nil {
		t.Fatalf("missing leader node: %+v", snap.Nodes)
	}
	want := SwarmNodeInfo{
		ID: "n1", Role: "manager", Leader: true, Availability: "active", State: "ready",
		Addr: "10.0.0.1", EngineVersion: "27.3.1", NanoCPUs: 4e9, MemoryBytes: 8 << 30,
	}
	if m.Type != "swarmnode" || m.Name != "manager-1" || m.Status != "ready" || m.SwarmNode == nil || *m.SwarmNode != want {
		t.Errorf("leader node = %+v / %+v", m, m.SwarmNode)
	}

	w := findNodeByID(snap.Nodes, "swarmnode:worker-1")
	if w == nil || w.Status != "down" || w.SwarmNode.Role != "worker" || w.SwarmNode.Leader {
		t.Errorf("down node = %+v", w)
	}
	d := findNodeByID(snap.Nodes, "swarmnode:worker-2")
	if d == nil || d.Status != "ready" || d.SwarmNode.Availability != "drain" {
		t.Errorf("drained node = %+v", d)
	}
	u := findNodeByID(snap.Nodes, "swarmnode:n4")
	if u == nil || u.Status != "unknown" || u.SwarmNode.State != "unknown" {
		t.Errorf("hostname-less node = %+v", u)
	}
}

func TestBuildSwarmNodeNodesDuplicateHostname(t *testing.T) {
	stale := swarmNode("old", "box")
	stale.Status.State = swarm.NodeStateDown
	stale.UpdatedAt = time.Unix(2000, 0)
	fresh := swarmNode("new", "box")
	fresh.UpdatedAt = time.Unix(1000, 0)

	for _, order := range [][]swarm.Node{{stale, fresh}, {fresh, stale}} {
		got := buildSwarmNodeNodes(order)
		if len(got) != 1 || got[0].SwarmNode.ID != "new" {
			t.Errorf("want only the ready node, got %+v", got)
		}
	}

	older := swarmNode("a", "box")
	older.UpdatedAt = time.Unix(1000, 0)
	newer := swarmNode("b", "box")
	newer.UpdatedAt = time.Unix(2000, 0)
	got := buildSwarmNodeNodes([]swarm.Node{newer, older})
	if len(got) != 1 || got[0].SwarmNode.ID != "b" {
		t.Errorf("want most recently updated node, got %+v", got)
	}
}

func TestSwarmNodeInfoJSON(t *testing.T) {
	b, err := json.Marshal(SwarmNodeInfo{ID: "n1", Role: "worker", Availability: "active", State: "ready"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"id":"n1","role":"worker","leader":false,"availability":"active","state":"ready"}`
	if string(b) != want {
		t.Errorf("json = %s, want %s", b, want)
	}
}

func TestDockerCollectorTaskPollLoop(t *testing.T) {
	cli := &stubDockerClient{}
	dc := NewDockerCollector(cli, time.Hour)
	dc.EnableSwarm(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := dc.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	<-dc.Updates()

	cli.setTasks([]swarm.Task{task("new", "s", "n1", 1, swarm.TaskStateRunning, swarm.TaskStateRunning)})
	select {
	case <-dc.Updates():
	case <-time.After(2 * time.Second):
		t.Fatal("task poller did not trigger a re-snapshot")
	}
	_ = dc.Stop()
}

func TestTopologyEventFilter(t *testing.T) {
	std := topologyEventFilter(false)
	if std.ExactMatch("type", string(events.ServiceEventType)) {
		t.Error("standalone filter must not include service events")
	}
	sw := topologyEventFilter(true)
	for _, typ := range []events.Type{events.ContainerEventType, events.ServiceEventType, events.NodeEventType} {
		if !sw.ExactMatch("type", string(typ)) {
			t.Errorf("swarm filter missing %s", typ)
		}
	}
}

func TestIsTopologyEventSwarmActions(t *testing.T) {
	for _, action := range []string{"update", "remove", "create"} {
		if !isTopologyEvent(events.Action(action)) {
			t.Errorf("%s should be a topology event", action)
		}
	}
}

func TestSelfServicesIsSelfEvent(t *testing.T) {
	calls := 0
	self := NewSelfServices(func(_ context.Context, id string) (swarm.Service, error) {
		calls++
		svc := swarm.Service{ID: id}
		switch id {
		case "dg":
			svc.Spec.Labels = map[string]string{SelfExcludeLabel: "true"}
		case "gone":
			return svc, context.Canceled
		}
		return svc, nil
	})
	ctx := context.Background()
	svcEvent := func(id string) events.Message {
		return events.Message{Type: events.ServiceEventType, Actor: events.Actor{ID: id}}
	}

	// Query twice: the second call must hit the cache.
	for range 2 {
		if !self.IsSelfEvent(ctx, svcEvent("dg")) {
			t.Error("dockgraph service event not filtered")
		}
	}
	if calls != 1 {
		t.Errorf("expected cached lookup, got %d inspect calls", calls)
	}
	if self.IsSelfEvent(ctx, svcEvent("web")) || self.IsSelfEvent(ctx, svcEvent("gone")) {
		t.Error("other service events must be kept")
	}
	task := events.Message{Type: events.ContainerEventType, Actor: events.Actor{ID: "c1", Attributes: map[string]string{swarmServiceIDLabel: "dg"}}}
	if !self.IsSelfEvent(ctx, task) {
		t.Error("dockgraph task container event not filtered")
	}
	plain := events.Message{Type: events.ContainerEventType, Actor: events.Actor{ID: "c2"}}
	if self.IsSelfEvent(ctx, plain) {
		t.Error("standalone container event must be kept")
	}
}

// Events of a removed service must not inspect it again and again: each
// lookup blocks the event loop. Removed services leave the cache.
func TestSelfServicesMissingService(t *testing.T) {
	calls := map[string]int{}
	self := NewSelfServices(func(_ context.Context, id string) (swarm.Service, error) {
		calls[id]++
		svc := swarm.Service{ID: id}
		switch id {
		case "dg":
			svc.Spec.Labels = map[string]string{SelfExcludeLabel: "true"}
		case "gone":
			return svc, cerrdefs.ErrNotFound
		}
		return svc, nil
	})
	ctx := context.Background()
	svcEvent := func(id, action string) events.Message {
		return events.Message{Type: events.ServiceEventType, Action: events.Action(action), Actor: events.Actor{ID: id}}
	}
	taskEvent := func(id, action string) events.Message {
		return events.Message{Type: events.ContainerEventType, Action: events.Action(action), Actor: events.Actor{ID: "c-" + id, Attributes: map[string]string{swarmServiceIDLabel: id}}}
	}

	// A stack rm: the service goes, then its task containers stop.
	for _, msg := range []events.Message{
		svcEvent("gone", "remove"),
		taskEvent("gone", "kill"), taskEvent("gone", "die"), taskEvent("gone", "stop"), taskEvent("gone", "destroy"),
	} {
		if self.IsSelfEvent(ctx, msg) {
			t.Errorf("%s event of a removed service filtered", msg.Action)
		}
	}
	if calls["gone"] != 1 {
		t.Errorf("removed service inspected %d times, want 1", calls["gone"])
	}

	// A removed service's own remove event is still recognised as self,
	// then the service leaves the cache.
	if !self.IsSelfEvent(ctx, svcEvent("dg", "update")) || !self.IsSelfEvent(ctx, svcEvent("dg", "remove")) {
		t.Error("dockgraph service events not filtered")
	}
	if _, ok := self.known["dg"]; ok || calls["dg"] != 1 {
		t.Errorf("removed service still cached (inspect calls %d)", calls["dg"])
	}

	// The cache stays bounded however many services come and go.
	for i := range selfCacheMax + 10 {
		self.IsSelfEvent(ctx, svcEvent(fmt.Sprintf("svc-%d", i), "update"))
	}
	if n := len(self.known); n > selfCacheMax {
		t.Errorf("cache holds %d services, want at most %d", n, selfCacheMax)
	}
}

// A transient inspect failure is retried, but not before selfRetryAfter.
func TestSelfServicesTransientError(t *testing.T) {
	calls := 0
	self := NewSelfServices(func(context.Context, string) (swarm.Service, error) {
		calls++
		return swarm.Service{}, errors.New("daemon busy")
	})
	now := time.Unix(1000, 0)
	self.now = func() time.Time { return now }
	task := events.Message{Type: events.ContainerEventType, Actor: events.Actor{ID: "c1", Attributes: map[string]string{swarmServiceIDLabel: "web"}}}

	for range 3 {
		if self.IsSelfEvent(context.Background(), task) {
			t.Error("failed lookup must count as not self")
		}
	}
	if calls != 1 {
		t.Errorf("transient failure inspected %d times within the TTL, want 1", calls)
	}
	now = now.Add(selfRetryAfter + time.Second)
	self.IsSelfEvent(context.Background(), task)
	if calls != 2 {
		t.Errorf("transient failure not retried after the TTL: %d calls", calls)
	}
}
