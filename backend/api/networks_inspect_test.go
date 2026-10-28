package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/swarm"
	"github.com/dockgraph/dockgraph/collector"
)

// stubNetworkInspector implements NetworkInspector for tests.
type stubNetworkInspector struct {
	result networktypes.Inspect
	err    error
}

func (s *stubNetworkInspector) NetworkInspect(_ context.Context, _ string, _ networktypes.InspectOptions) (networktypes.Inspect, error) {
	return s.result, s.err
}

// stubNetworkMembers implements NetworkMemberLister for tests. TaskList
// honours the desired-state filter, as the daemon does.
type stubNetworkMembers struct {
	services []swarm.Service
	tasks    []swarm.Task
	nodes    []swarm.Node
	taskErr  error
	calls    int
}

func (s *stubNetworkMembers) ServiceList(_ context.Context, _ swarm.ServiceListOptions) ([]swarm.Service, error) {
	s.calls++
	return s.services, nil
}

func (s *stubNetworkMembers) TaskList(_ context.Context, opts swarm.TaskListOptions) ([]swarm.Task, error) {
	s.calls++
	if s.taskErr != nil {
		return nil, s.taskErr
	}
	var out []swarm.Task
	for _, t := range s.tasks {
		if opts.Filters.ExactMatch("desired-state", string(t.DesiredState)) {
			out = append(out, t)
		}
	}
	return out, nil
}

func (s *stubNetworkMembers) NodeList(_ context.Context, _ swarm.NodeListOptions) ([]swarm.Node, error) {
	s.calls++
	return s.nodes, nil
}

// newNetworkInspectServer registers the handler on a mux so PathValue works.
func newNetworkInspectServer(inspector NetworkInspector) *http.ServeMux {
	return newSwarmNetworkInspectServer(inspector, nil)
}

// newSwarmNetworkInspectServer registers the handler as the server does in swarm mode.
func newSwarmNetworkInspectServer(inspector NetworkInspector, members NetworkMemberLister) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/networks/{name}", HandleNetworkInspect(inspector, members))
	return mux
}

// swarmNetworkFixture is the overlay network app_net as a manager sees it:
// its own daemon knows one task of shop_web, a standalone container and the
// load balancer, while the swarm API knows every task of the network's services.
func swarmNetworkFixture() (networktypes.Inspect, *stubNetworkMembers) {
	network := networktypes.Inspect{
		Name: "app_net", ID: "net1", Driver: "overlay", Scope: "swarm",
		Containers: map[string]networktypes.EndpointResource{
			"cid-web-1":  {Name: "shop_web.1.task1", IPv4Address: "10.0.9.12/24"},
			"cid-tool":   {Name: "debug-tool", IPv4Address: "10.0.9.40/24"},
			"lb-app_net": {Name: "app_net-endpoint", IPv4Address: "10.0.9.48/24"},
		},
	}

	onNet := func(id, name string, networkID string) swarm.Service {
		svc := swarm.Service{ID: id}
		svc.Spec.Name = name
		svc.Spec.Mode.Replicated = &swarm.ReplicatedService{Replicas: u64(2)}
		svc.Spec.TaskTemplate.Networks = []swarm.NetworkAttachmentConfig{{Target: networkID}}
		return svc
	}
	web := onNet("svc-web", "shop_web", "net1")
	web.Endpoint.VirtualIPs = []swarm.EndpointVirtualIP{{NetworkID: "net1", Addr: "10.0.9.11/24"}}
	self := onNet("svc-dg", "dg_server", "net1")
	self.Spec.Labels = map[string]string{collector.SelfExcludeLabel: "true"}
	other := onNet("svc-db", "shop_db", "net2")

	task := func(id, serviceID string, slot int, nodeID string, desired, state swarm.TaskState, containerID, addr string) swarm.Task {
		t := swarm.Task{ID: id, ServiceID: serviceID, Slot: slot, NodeID: nodeID, DesiredState: desired}
		t.Status.State = state
		t.Status.ContainerStatus = &swarm.ContainerStatus{ContainerID: containerID}
		t.NetworksAttachments = []swarm.NetworkAttachment{{Network: swarm.Network{ID: "net1"}, Addresses: []string{addr}}}
		return t
	}
	running := swarm.TaskStateRunning
	members := &stubNetworkMembers{
		services: []swarm.Service{web, self, other},
		tasks: []swarm.Task{
			task("task2", "svc-web", 2, "node-w1", running, running, "cid-web-2", "10.0.9.13/24"),
			task("task1", "svc-web", 1, "node-m1", running, running, "cid-web-1", "10.0.9.12/24"),
			// History: a replaced task that is no longer on the network.
			task("task0", "svc-web", 1, "node-w1", swarm.TaskStateShutdown, swarm.TaskStateFailed, "cid-web-0", "10.0.9.5/24"),
			task("task-dg", "svc-dg", 1, "node-m1", running, running, "cid-dg", "10.0.9.20/24"),
		},
		nodes: []swarm.Node{
			{ID: "node-m1", Description: swarm.NodeDescription{Hostname: "m1"}},
			{ID: "node-w1", Description: swarm.NodeDescription{Hostname: "w1"}},
		},
	}
	return network, members
}

// networkMembersBody is the part of the inspect response that lists members.
type networkMembersBody struct {
	Services []struct {
		Name  string `json:"name"`
		Mode  string `json:"mode"`
		VIP   string `json:"vip"`
		Tasks []struct {
			ID           string   `json:"id"`
			Slot         int      `json:"slot"`
			NodeHostname string   `json:"nodeHostname"`
			Addresses    []string `json:"addresses"`
		} `json:"tasks"`
	} `json:"services"`
	Containers []struct {
		ID string `json:"id"`
	} `json:"containers"`
	ServicesUnavailable bool `json:"servicesUnavailable"`
}

func inspectNetworkMembers(t *testing.T, mux *http.ServeMux) networkMembersBody {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/networks/app_net", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body networkMembersBody
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body
}

func TestHandleNetworkInspect_SwarmNetworkListsServicesClusterWide(t *testing.T) {
	network, members := swarmNetworkFixture()
	body := inspectNetworkMembers(t, newSwarmNetworkInspectServer(&stubNetworkInspector{result: network}, members))

	// shop_web only: shop_db is on another network and DockGraph hides itself.
	if len(body.Services) != 1 {
		t.Fatalf("services = %+v, want shop_web only", body.Services)
	}
	web := body.Services[0]
	if web.Name != "shop_web" || web.Mode != "replicated" || web.VIP != "10.0.9.11/24" {
		t.Errorf("service = %s %s %s, want shop_web replicated 10.0.9.11/24", web.Name, web.Mode, web.VIP)
	}
	// Both current tasks, on any node, in slot order; not the replaced one.
	if len(web.Tasks) != 2 {
		t.Fatalf("tasks = %+v, want task1 and task2", web.Tasks)
	}
	for i, want := range []struct {
		id, host, addr string
		slot           int
	}{{"task1", "m1", "10.0.9.12/24", 1}, {"task2", "w1", "10.0.9.13/24", 2}} {
		got := web.Tasks[i]
		if got.ID != want.id || got.Slot != want.slot || got.NodeHostname != want.host || len(got.Addresses) != 1 || got.Addresses[0] != want.addr {
			t.Errorf("task %d = %+v, want %+v", i, got, want)
		}
	}
	// Task containers are listed under their service; the load balancer is no container.
	if len(body.Containers) != 1 || body.Containers[0].ID != "cid-tool" {
		t.Errorf("containers = %+v, want only the standalone cid-tool", body.Containers)
	}
	if body.ServicesUnavailable {
		t.Error("servicesUnavailable set on success")
	}
}

func TestHandleNetworkInspect_SwarmMembersUnavailable(t *testing.T) {
	network, members := swarmNetworkFixture()
	members.taskErr = errors.New("manager unreachable")
	body := inspectNetworkMembers(t, newSwarmNetworkInspectServer(&stubNetworkInspector{result: network}, members))

	if !body.ServicesUnavailable || body.Services != nil {
		t.Errorf("servicesUnavailable = %v, services = %+v; want flagged and no services", body.ServicesUnavailable, body.Services)
	}
	// The local view is all there is: keep the task container.
	if len(body.Containers) != 2 {
		t.Errorf("containers = %+v, want cid-web-1 and cid-tool", body.Containers)
	}
}

func TestHandleNetworkInspect_LocalNetworkSkipsSwarmLookup(t *testing.T) {
	_, members := swarmNetworkFixture()
	body := inspectNetworkMembers(t, newSwarmNetworkInspectServer(&stubNetworkInspector{result: validNetworkInspect()}, members))

	if members.calls != 0 {
		t.Errorf("swarm API called %d times for a local network", members.calls)
	}
	if body.Services != nil || body.ServicesUnavailable {
		t.Errorf("services = %+v, unavailable = %v; want neither for a local network", body.Services, body.ServicesUnavailable)
	}
}

func validNetworkInspect() networktypes.Inspect {
	return networktypes.Inspect{
		Name:       "my-network",
		ID:         "abc123network",
		Driver:     "bridge",
		Scope:      "local",
		Internal:   false,
		EnableIPv6: false,
		Created:    time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
		Options:    map[string]string{"com.docker.network.bridge.name": "docker0"},
		Labels:     map[string]string{"project": "myapp", "env": "dev"},
		IPAM: networktypes.IPAM{
			Driver: "default",
			Config: []networktypes.IPAMConfig{
				{
					Subnet:  "172.18.0.0/16",
					Gateway: "172.18.0.1",
				},
			},
		},
		Containers: map[string]networktypes.EndpointResource{
			"container1": {
				Name:        "web-server",
				IPv4Address: "172.18.0.2/16",
				IPv6Address: "",
				MacAddress:  "02:42:ac:12:00:02",
			},
		},
	}
}

func TestHandleNetworkInspect_ValidNetwork(t *testing.T) {
	stub := &stubNetworkInspector{result: validNetworkInspect()}
	mux := newNetworkInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/networks/my-network", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["name"] != "my-network" {
		t.Errorf("name = %q, want %q", body["name"], "my-network")
	}
	if body["id"] != "abc123network" {
		t.Errorf("id = %q, want %q", body["id"], "abc123network")
	}
	if body["driver"] != "bridge" {
		t.Errorf("driver = %q, want %q", body["driver"], "bridge")
	}
	if body["scope"] != "local" {
		t.Errorf("scope = %q, want %q", body["scope"], "local")
	}
	if body["internal"] != false {
		t.Errorf("internal = %v, want false", body["internal"])
	}
	if body["enableIPv6"] != false {
		t.Errorf("enableIPv6 = %v, want false", body["enableIPv6"])
	}

	// Verify options.
	opts, ok := body["options"].(map[string]any)
	if !ok {
		t.Fatalf("options is not an object: %T", body["options"])
	}
	if opts["com.docker.network.bridge.name"] != "docker0" {
		t.Errorf("options bridge name = %q, want %q", opts["com.docker.network.bridge.name"], "docker0")
	}

	// Verify labels.
	labels, ok := body["labels"].(map[string]any)
	if !ok {
		t.Fatalf("labels is not an object: %T", body["labels"])
	}
	if labels["project"] != "myapp" {
		t.Errorf("labels[project] = %q, want %q", labels["project"], "myapp")
	}

	// Verify IPAM.
	ipam, ok := body["ipam"].(map[string]any)
	if !ok {
		t.Fatalf("ipam is not an object: %T", body["ipam"])
	}
	if ipam["driver"] != "default" {
		t.Errorf("ipam.driver = %q, want %q", ipam["driver"], "default")
	}
	configs, ok := ipam["config"].([]any)
	if !ok {
		t.Fatalf("ipam.config is not an array: %T", ipam["config"])
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 IPAM config, got %d", len(configs))
	}
	cfg0 := configs[0].(map[string]any)
	if cfg0["subnet"] != "172.18.0.0/16" {
		t.Errorf("subnet = %q, want %q", cfg0["subnet"], "172.18.0.0/16")
	}
	if cfg0["gateway"] != "172.18.0.1" {
		t.Errorf("gateway = %q, want %q", cfg0["gateway"], "172.18.0.1")
	}

	// Verify containers.
	containers, ok := body["containers"].([]any)
	if !ok {
		t.Fatalf("containers is not an array: %T", body["containers"])
	}
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	c0 := containers[0].(map[string]any)
	if c0["id"] != "container1" {
		t.Errorf("container id = %q, want %q", c0["id"], "container1")
	}
	if c0["name"] != "web-server" {
		t.Errorf("container name = %q, want %q", c0["name"], "web-server")
	}
	if c0["ipv4Address"] != "172.18.0.2/16" {
		t.Errorf("ipv4Address = %q, want %q", c0["ipv4Address"], "172.18.0.2/16")
	}
	if c0["macAddress"] != "02:42:ac:12:00:02" {
		t.Errorf("macAddress = %q, want %q", c0["macAddress"], "02:42:ac:12:00:02")
	}
}

func TestHandleNetworkInspect_InvalidName(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"single char", "n"},
		{"starts with dot", ".internal"},
		{"starts with dash", "-net"},
	}

	stub := &stubNetworkInspector{}
	mux := newNetworkInspectServer(stub)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/networks/"+tc.input, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for name %q, got %d", tc.input, w.Code)
			}

			var errBody map[string]any
			if err := json.NewDecoder(w.Body).Decode(&errBody); err != nil {
				t.Fatalf("decode error body: %v", err)
			}
			if errBody["error"] != "invalid network name" {
				t.Errorf("error = %q, want %q", errBody["error"], "invalid network name")
			}
		})
	}
}

func TestHandleNetworkInspect_DockerError(t *testing.T) {
	// Only a missing network is a 404: a daemon timeout says nothing about
	// whether it exists.
	tests := []struct {
		name     string
		err      error
		wantCode int
		wantMsg  string
	}{
		{"not found", cerrdefs.ErrNotFound.WithMessage("network nonexistent-net not found"), http.StatusNotFound, "network not found"},
		{"daemon timeout", context.DeadlineExceeded, http.StatusBadGateway, "network inspect failed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mux := newNetworkInspectServer(&stubNetworkInspector{err: tc.err})
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/networks/nonexistent-net", nil))

			var errBody map[string]any
			if err := json.NewDecoder(w.Body).Decode(&errBody); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if w.Code != tc.wantCode || errBody["error"] != tc.wantMsg {
				t.Errorf("got %d %q, want %d %q", w.Code, errBody["error"], tc.wantCode, tc.wantMsg)
			}
		})
	}
}

func TestHandleNetworkInspect_MissingPathVariable(t *testing.T) {
	stub := &stubNetworkInspector{}
	handler := HandleNetworkInspect(stub, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/networks/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing path variable, got %d", w.Code)
	}
}

func TestHandleNetworkInspect_NilIPAMConfig(t *testing.T) {
	network := validNetworkInspect()
	network.IPAM = networktypes.IPAM{
		Driver: "default",
		Config: nil,
	}

	stub := &stubNetworkInspector{result: network}
	mux := newNetworkInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/networks/my-network", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// When IPAM.Config is nil, the ipam key should not be present.
	if _, exists := body["ipam"]; exists {
		t.Error("ipam should not be present when IPAM.Config is nil")
	}
}

func TestHandleNetworkInspect_NilContainers(t *testing.T) {
	network := validNetworkInspect()
	network.Containers = nil

	stub := &stubNetworkInspector{result: network}
	mux := newNetworkInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/networks/my-network", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// When Containers is nil, the containers key should not be present.
	if _, exists := body["containers"]; exists {
		t.Error("containers should not be present when network.Containers is nil")
	}
}

func TestHandleNetworkInspect_SkipsLoadBalancerEndpoints(t *testing.T) {
	network := validNetworkInspect()
	network.Containers["lb-my-network"] = networktypes.EndpointResource{Name: "my-network-endpoint", IPv4Address: "10.0.9.48/24"}
	network.Containers["ingress-sbox"] = networktypes.EndpointResource{Name: "ingress-endpoint", IPv4Address: "10.0.0.2/24"}

	mux := newNetworkInspectServer(&stubNetworkInspector{result: network})
	req := httptest.NewRequest(http.MethodGet, "/api/networks/my-network", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var body struct {
		Containers []struct {
			ID string `json:"id"`
		} `json:"containers"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Containers) != 1 || body.Containers[0].ID != "container1" {
		t.Errorf("containers = %+v, want only container1", body.Containers)
	}
}

func TestHandleNetworkInspect_MultipleContainers(t *testing.T) {
	network := validNetworkInspect()
	network.Containers = map[string]networktypes.EndpointResource{
		"cid1": {
			Name:        "web",
			IPv4Address: "172.18.0.2/16",
			IPv6Address: "fd00::2/64",
			MacAddress:  "02:42:ac:12:00:02",
		},
		"cid2": {
			Name:        "api",
			IPv4Address: "172.18.0.3/16",
			IPv6Address: "",
			MacAddress:  "02:42:ac:12:00:03",
		},
	}

	stub := &stubNetworkInspector{result: network}
	mux := newNetworkInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/networks/my-network", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	containers, ok := body["containers"].([]any)
	if !ok {
		t.Fatalf("containers is not an array: %T", body["containers"])
	}
	if len(containers) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(containers))
	}

	// Build a map by container ID for order-independent checks.
	byID := make(map[string]map[string]any)
	for _, c := range containers {
		cm := c.(map[string]any)
		byID[cm["id"].(string)] = cm
	}

	web, ok := byID["cid1"]
	if !ok {
		t.Fatal("expected container cid1 in response")
	}
	if web["name"] != "web" {
		t.Errorf("cid1 name = %q, want %q", web["name"], "web")
	}
	if web["ipv6Address"] != "fd00::2/64" {
		t.Errorf("cid1 ipv6Address = %q, want %q", web["ipv6Address"], "fd00::2/64")
	}

	api, ok := byID["cid2"]
	if !ok {
		t.Fatal("expected container cid2 in response")
	}
	if api["name"] != "api" {
		t.Errorf("cid2 name = %q, want %q", api["name"], "api")
	}
}

func TestHandleNetworkInspect_IPAMWithAuxAddresses(t *testing.T) {
	network := validNetworkInspect()
	network.IPAM = networktypes.IPAM{
		Driver: "default",
		Config: []networktypes.IPAMConfig{
			{
				Subnet:     "10.0.0.0/24",
				Gateway:    "10.0.0.1",
				IPRange:    "10.0.0.128/25",
				AuxAddress: map[string]string{"host1": "10.0.0.5", "host2": "10.0.0.6"},
			},
		},
	}

	stub := &stubNetworkInspector{result: network}
	mux := newNetworkInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/networks/my-network", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	ipam := body["ipam"].(map[string]any)
	configs := ipam["config"].([]any)
	cfg := configs[0].(map[string]any)

	if cfg["ipRange"] != "10.0.0.128/25" {
		t.Errorf("ipRange = %q, want %q", cfg["ipRange"], "10.0.0.128/25")
	}

	aux, ok := cfg["auxAddresses"].(map[string]any)
	if !ok {
		t.Fatalf("auxAddresses is not a map: %T", cfg["auxAddresses"])
	}
	if aux["host1"] != "10.0.0.5" {
		t.Errorf("auxAddresses[host1] = %q, want %q", aux["host1"], "10.0.0.5")
	}
	if aux["host2"] != "10.0.0.6" {
		t.Errorf("auxAddresses[host2] = %q, want %q", aux["host2"], "10.0.0.6")
	}
}

func TestHandleNetworkInspect_MultipleIPAMConfigs(t *testing.T) {
	network := validNetworkInspect()
	network.IPAM = networktypes.IPAM{
		Driver: "default",
		Config: []networktypes.IPAMConfig{
			{Subnet: "172.20.0.0/16", Gateway: "172.20.0.1"},
			{Subnet: "fd00::/64", Gateway: "fd00::1"},
		},
	}

	stub := &stubNetworkInspector{result: network}
	mux := newNetworkInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/networks/my-network", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	ipam := body["ipam"].(map[string]any)
	configs := ipam["config"].([]any)
	if len(configs) != 2 {
		t.Fatalf("expected 2 IPAM configs, got %d", len(configs))
	}

	// Verify both configs are present (order may vary since they're in a slice).
	subnets := make(map[string]bool)
	for _, c := range configs {
		cm := c.(map[string]any)
		subnets[cm["subnet"].(string)] = true
	}
	if !subnets["172.20.0.0/16"] {
		t.Error("expected IPv4 subnet 172.20.0.0/16")
	}
	if !subnets["fd00::/64"] {
		t.Error("expected IPv6 subnet fd00::/64")
	}
}

func TestHandleNetworkInspect_InternalNetwork(t *testing.T) {
	network := validNetworkInspect()
	network.Internal = true
	network.Driver = "overlay"
	network.Scope = "swarm"
	network.EnableIPv6 = true

	stub := &stubNetworkInspector{result: network}
	mux := newNetworkInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/networks/my-network", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["internal"] != true {
		t.Errorf("internal = %v, want true", body["internal"])
	}
	if body["driver"] != "overlay" {
		t.Errorf("driver = %q, want %q", body["driver"], "overlay")
	}
	if body["scope"] != "swarm" {
		t.Errorf("scope = %q, want %q", body["scope"], "swarm")
	}
	if body["enableIPv6"] != true {
		t.Errorf("enableIPv6 = %v, want true", body["enableIPv6"])
	}
}

func TestHandleNetworkInspect_EmptyContainersMap(t *testing.T) {
	network := validNetworkInspect()
	network.Containers = map[string]networktypes.EndpointResource{}

	stub := &stubNetworkInspector{result: network}
	mux := newNetworkInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/networks/my-network", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// An empty (non-nil) map still produces the containers key.
	containers, ok := body["containers"].([]any)
	if !ok {
		t.Fatalf("containers is not an array: %T", body["containers"])
	}
	if len(containers) != 0 {
		t.Errorf("expected 0 containers, got %d", len(containers))
	}
}

func TestHandleNetworkInspect_NilOptionsAndLabels(t *testing.T) {
	network := validNetworkInspect()
	network.Options = nil
	network.Labels = nil

	stub := &stubNetworkInspector{result: network}
	mux := newNetworkInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/networks/my-network", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Nil maps are valid — they should serialize as null.
	if _, exists := body["options"]; !exists {
		t.Error("options key should be present even when nil")
	}
	if _, exists := body["labels"]; !exists {
		t.Error("labels key should be present even when nil")
	}
}
