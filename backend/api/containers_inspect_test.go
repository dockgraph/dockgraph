package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
)

// stubContainerInspector implements ContainerInspector for tests.
type stubContainerInspector struct {
	result containertypes.InspectResponse
	err    error
}

func (s *stubContainerInspector) ContainerInspect(_ context.Context, _ string) (containertypes.InspectResponse, error) {
	return s.result, s.err
}

// newContainerInspectServer registers the handler on a mux so PathValue works.
func newContainerInspectServer(inspector ContainerInspector) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/containers/{id}", HandleContainerInspect(inspector))
	return mux
}

func validContainerInspectResponse() containertypes.InspectResponse {
	ns := &containertypes.NetworkSettings{
		Networks: map[string]*networktypes.EndpointSettings{
			"bridge": {
				IPAddress:   "172.17.0.2",
				Gateway:     "172.17.0.1",
				MacAddress:  "02:42:ac:11:00:02",
				IPPrefixLen: 16,
			},
		},
	}
	ns.Ports = nat.PortMap{
		"80/tcp": []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: "8080"},
		},
	}

	return containertypes.InspectResponse{
		ContainerJSONBase: &containertypes.ContainerJSONBase{
			Name: "/my-container",
			State: &containertypes.State{
				Status:     "running",
				Running:    true,
				Paused:     false,
				Restarting: false,
				OOMKilled:  false,
				Pid:        1234,
				ExitCode:   0,
				StartedAt:  "2025-01-15T10:30:00Z",
				FinishedAt: "0001-01-01T00:00:00Z",
			},
			HostConfig: &containertypes.HostConfig{
				RestartPolicy: containertypes.RestartPolicy{Name: "always"},
				NetworkMode:   "bridge",
				Resources: containertypes.Resources{
					CPUQuota:          50000,
					CPUPeriod:         100000,
					NanoCPUs:          0,
					Memory:            536870912,
					MemoryReservation: 268435456,
				},
			},
		},
		Config: &containertypes.Config{
			Image:      "nginx:latest",
			Cmd:        []string{"nginx", "-g", "daemon off;"},
			Entrypoint: []string{"/docker-entrypoint.sh"},
			WorkingDir: "/usr/share/nginx",
			User:       "www-data",
			Env:        []string{"PATH=/usr/bin", "NGINX_VERSION=1.25"},
			Labels:     map[string]string{"app": "web", "env": "prod"},
		},
		NetworkSettings: ns,
		Mounts: []containertypes.MountPoint{
			{
				Type:        mount.TypeVolume,
				Name:        "data-vol",
				Source:      "/var/lib/docker/volumes/data-vol/_data",
				Destination: "/data",
				RW:          true,
			},
		},
	}
}

func TestHandleContainerInspect_ValidContainer(t *testing.T) {
	stub := &stubContainerInspector{result: validContainerInspectResponse()}
	mux := newContainerInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/containers/abc123def", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Name should have leading "/" trimmed.
	if body["name"] != "my-container" {
		t.Errorf("name = %q, want %q", body["name"], "my-container")
	}
	if body["image"] != "nginx:latest" {
		t.Errorf("image = %q, want %q", body["image"], "nginx:latest")
	}
	if body["status"] != "running" {
		t.Errorf("status = %q, want %q", body["status"], "running")
	}
	if body["running"] != true {
		t.Errorf("running = %v, want true", body["running"])
	}
	if body["paused"] != false {
		t.Errorf("paused = %v, want false", body["paused"])
	}

	// Verify PID is present.
	if pid, ok := body["pid"].(float64); !ok || pid != 1234 {
		t.Errorf("pid = %v, want 1234", body["pid"])
	}

	// Verify exit code.
	if ec, ok := body["exitCode"].(float64); !ok || ec != 0 {
		t.Errorf("exitCode = %v, want 0", body["exitCode"])
	}

	if body["startedAt"] != "2025-01-15T10:30:00Z" {
		t.Errorf("startedAt = %q, want %q", body["startedAt"], "2025-01-15T10:30:00Z")
	}

	// Verify ports exist and have content.
	ports, ok := body["ports"].([]any)
	if !ok {
		t.Fatalf("ports is not an array: %T", body["ports"])
	}
	if len(ports) != 1 {
		t.Fatalf("expected 1 port, got %d", len(ports))
	}
	port0 := ports[0].(map[string]any)
	if port0["hostPort"] != "8080" {
		t.Errorf("port hostPort = %q, want %q", port0["hostPort"], "8080")
	}
	if port0["containerPort"] != "80" {
		t.Errorf("port containerPort = %q, want %q", port0["containerPort"], "80")
	}
	if port0["protocol"] != "tcp" {
		t.Errorf("port protocol = %q, want %q", port0["protocol"], "tcp")
	}

	// Verify mounts.
	mounts, ok := body["mounts"].([]any)
	if !ok {
		t.Fatalf("mounts is not an array: %T", body["mounts"])
	}
	if len(mounts) != 1 {
		t.Fatalf("expected 1 mount, got %d", len(mounts))
	}
	mount0 := mounts[0].(map[string]any)
	if mount0["type"] != "volume" {
		t.Errorf("mount type = %q, want %q", mount0["type"], "volume")
	}
	if mount0["name"] != "data-vol" {
		t.Errorf("mount name = %q, want %q", mount0["name"], "data-vol")
	}
	if mount0["destination"] != "/data" {
		t.Errorf("mount destination = %q, want %q", mount0["destination"], "/data")
	}

	// Verify networks.
	networks, ok := body["networks"].([]any)
	if !ok {
		t.Fatalf("networks is not an array: %T", body["networks"])
	}
	if len(networks) != 1 {
		t.Fatalf("expected 1 network, got %d", len(networks))
	}
	net0 := networks[0].(map[string]any)
	if net0["name"] != "bridge" {
		t.Errorf("network name = %q, want %q", net0["name"], "bridge")
	}
	if net0["ipAddress"] != "172.17.0.2" {
		t.Errorf("network ipAddress = %q, want %q", net0["ipAddress"], "172.17.0.2")
	}

	// Verify security section.
	security, ok := body["security"].(map[string]any)
	if !ok {
		t.Fatalf("security is not an object: %T", body["security"])
	}
	if security["privileged"] != false {
		t.Errorf("security.privileged = %v, want false", security["privileged"])
	}

	// Verify resources section.
	resources, ok := body["resources"].(map[string]any)
	if !ok {
		t.Fatalf("resources is not an object: %T", body["resources"])
	}
	if cpuQuota, ok := resources["cpuQuota"].(float64); !ok || cpuQuota != 50000 {
		t.Errorf("resources.cpuQuota = %v, want 50000", resources["cpuQuota"])
	}
	if memLimit, ok := resources["memoryLimit"].(float64); !ok || memLimit != 536870912 {
		t.Errorf("resources.memoryLimit = %v, want 536870912", resources["memoryLimit"])
	}

	// Verify restart policy.
	rp, ok := body["restartPolicy"].(map[string]any)
	if !ok {
		t.Fatalf("restartPolicy is not an object: %T", body["restartPolicy"])
	}
	if rp["Name"] != "always" {
		t.Errorf("restartPolicy.Name = %q, want %q", rp["Name"], "always")
	}

	// Verify env filtering.
	env, ok := body["env"].([]any)
	if !ok {
		t.Fatalf("env is not an array: %T", body["env"])
	}
	if len(env) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(env))
	}

	// Verify labels.
	labels, ok := body["labels"].(map[string]any)
	if !ok {
		t.Fatalf("labels is not an object: %T", body["labels"])
	}
	if labels["app"] != "web" {
		t.Errorf("labels[app] = %q, want %q", labels["app"], "web")
	}

	// Health should NOT be present when State.Health is nil.
	if _, exists := body["health"]; exists {
		t.Error("expected health to be absent when State.Health is nil")
	}
}

func TestHandleContainerInspect_NameTrimsLeadingSlash(t *testing.T) {
	resp := validContainerInspectResponse()
	resp.Name = "/leading-slash-container"

	stub := &stubContainerInspector{result: resp}
	mux := newContainerInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/containers/abc123def", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["name"] != "leading-slash-container" {
		t.Errorf("name = %q, want leading slash trimmed to %q", body["name"], "leading-slash-container")
	}
}

func TestHandleContainerInspect_NameWithoutSlashUnchanged(t *testing.T) {
	resp := validContainerInspectResponse()
	resp.Name = "no-slash"

	stub := &stubContainerInspector{result: resp}
	mux := newContainerInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/containers/abc123def", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["name"] != "no-slash" {
		t.Errorf("name = %q, want %q", body["name"], "no-slash")
	}
}

func TestHandleContainerInspect_WithHealth(t *testing.T) {
	resp := validContainerInspectResponse()
	resp.State.Health = &containertypes.Health{
		Status:        "healthy",
		FailingStreak: 0,
		Log: []*containertypes.HealthcheckResult{
			{
				Start:    time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
				End:      time.Date(2025, 1, 15, 10, 30, 1, 0, time.UTC),
				ExitCode: 0,
				Output:   "OK",
			},
		},
	}

	stub := &stubContainerInspector{result: resp}
	mux := newContainerInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/containers/abc123def", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	health, ok := body["health"].(map[string]any)
	if !ok {
		t.Fatal("expected health key in response when Health is set")
	}

	if health["status"] != "healthy" {
		t.Errorf("health.status = %q, want %q", health["status"], "healthy")
	}

	if fs, ok := health["failingStreak"].(float64); !ok || fs != 0 {
		t.Errorf("health.failingStreak = %v, want 0", health["failingStreak"])
	}

	logs, ok := health["log"].([]any)
	if !ok {
		t.Fatalf("health.log is not an array: %T", health["log"])
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 health log entry, got %d", len(logs))
	}

	entry := logs[0].(map[string]any)
	if ec, ok := entry["exitCode"].(float64); !ok || ec != 0 {
		t.Errorf("health log exitCode = %v, want 0", entry["exitCode"])
	}
	if entry["output"] != "OK" {
		t.Errorf("health log output = %q, want %q", entry["output"], "OK")
	}
}

func TestHandleContainerInspect_HealthOmittedWhenNil(t *testing.T) {
	resp := validContainerInspectResponse()
	resp.State.Health = nil

	stub := &stubContainerInspector{result: resp}
	mux := newContainerInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/containers/abc123def", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if _, exists := body["health"]; exists {
		t.Error("health should not be present when State.Health is nil")
	}
}

func TestHandleContainerInspect_InvalidID(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"single char", "a"},
		{"starts with dot", ".hidden"},
		{"starts with dash", "-flag"},
	}

	stub := &stubContainerInspector{}
	mux := newContainerInspectServer(stub)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/containers/"+tc.id, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for id %q, got %d", tc.id, w.Code)
			}

			var errBody map[string]any
			if err := json.NewDecoder(w.Body).Decode(&errBody); err != nil {
				t.Fatalf("failed to decode error body: %v", err)
			}
			if errBody["error"] != "invalid container ID" {
				t.Errorf("error = %q, want %q", errBody["error"], "invalid container ID")
			}
		})
	}
}

func TestHandleContainerInspect_DockerError(t *testing.T) {
	stub := &stubContainerInspector{
		err: fmt.Errorf("container not found: no such container"),
	}
	mux := newContainerInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/containers/abc123def", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 on docker error, got %d", w.Code)
	}

	var errBody map[string]any
	if err := json.NewDecoder(w.Body).Decode(&errBody); err != nil {
		t.Fatalf("failed to decode error body: %v", err)
	}
	if errBody["error"] != "container not found" {
		t.Errorf("error = %q, want %q", errBody["error"], "container not found")
	}
}

func TestHandleContainerInspect_MissingPathVariable(t *testing.T) {
	// Call the handler directly without a mux, so PathValue returns "".
	stub := &stubContainerInspector{}
	handler := HandleContainerInspect(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/containers/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing path variable, got %d", w.Code)
	}
}

func TestHandleContainerInspect_EnvVarsMasked(t *testing.T) {
	resp := validContainerInspectResponse()
	resp.Config.Env = []string{
		"PATH=/usr/bin",
		"DB_PASSWORD=supersecret",
		"API_KEY=mykey123",
		"NORMAL_VAR=visible",
	}

	stub := &stubContainerInspector{result: resp}
	mux := newContainerInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/containers/abc123def", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	env, ok := body["env"].([]any)
	if !ok {
		t.Fatalf("env is not an array: %T", body["env"])
	}

	for _, entry := range env {
		e := entry.(map[string]any)
		key := e["key"].(string)
		val := e["value"].(string)

		switch key {
		case "PATH":
			if val != "/usr/bin" {
				t.Errorf("PATH should not be masked, got %q", val)
			}
		case "DB_PASSWORD", "API_KEY":
			if val != "********" {
				t.Errorf("%s should be masked, got %q", key, val)
			}
		case "NORMAL_VAR":
			if val != "visible" {
				t.Errorf("NORMAL_VAR should not be masked, got %q", val)
			}
		}
	}
}

func TestHandleContainerInspect_CmdAndEntrypoint(t *testing.T) {
	resp := validContainerInspectResponse()
	resp.Config.Cmd = []string{"./app", "--port", "3000"}
	resp.Config.Entrypoint = []string{"/bin/sh", "-c"}
	resp.Config.WorkingDir = "/opt/app"
	resp.Config.User = "appuser"

	stub := &stubContainerInspector{result: resp}
	mux := newContainerInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/containers/abc123def", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	cmd, ok := body["cmd"].([]any)
	if !ok {
		t.Fatalf("cmd is not an array: %T", body["cmd"])
	}
	if len(cmd) != 3 || cmd[0] != "./app" {
		t.Errorf("cmd = %v, want [./app --port 3000]", cmd)
	}

	ep, ok := body["entrypoint"].([]any)
	if !ok {
		t.Fatalf("entrypoint is not an array: %T", body["entrypoint"])
	}
	if len(ep) != 2 || ep[0] != "/bin/sh" {
		t.Errorf("entrypoint = %v, want [/bin/sh -c]", ep)
	}

	if body["workingDir"] != "/opt/app" {
		t.Errorf("workingDir = %q, want %q", body["workingDir"], "/opt/app")
	}
	if body["user"] != "appuser" {
		t.Errorf("user = %q, want %q", body["user"], "appuser")
	}
}

func TestHandleContainerInspect_NetworkMode(t *testing.T) {
	resp := validContainerInspectResponse()
	resp.HostConfig.NetworkMode = "host"

	stub := &stubContainerInspector{result: resp}
	mux := newContainerInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/containers/abc123def", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["networkMode"] != "host" {
		t.Errorf("networkMode = %q, want %q", body["networkMode"], "host")
	}
}

func TestHandleContainerInspect_MultiplePorts(t *testing.T) {
	resp := validContainerInspectResponse()
	resp.NetworkSettings.Ports = nat.PortMap{
		"80/tcp": []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: "8080"},
		},
		"443/tcp": []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: "8443"},
		},
	}

	stub := &stubContainerInspector{result: resp}
	mux := newContainerInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/containers/abc123def", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	ports, ok := body["ports"].([]any)
	if !ok {
		t.Fatalf("ports is not an array: %T", body["ports"])
	}
	if len(ports) != 2 {
		t.Errorf("expected 2 ports, got %d", len(ports))
	}
}

func TestHandleContainerInspect_EmptyMounts(t *testing.T) {
	resp := validContainerInspectResponse()
	resp.Mounts = nil

	stub := &stubContainerInspector{result: resp}
	mux := newContainerInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/containers/abc123def", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	mounts, ok := body["mounts"].([]any)
	if !ok {
		t.Fatalf("mounts is not an array: %T", body["mounts"])
	}
	if len(mounts) != 0 {
		t.Errorf("expected 0 mounts, got %d", len(mounts))
	}
}

func TestHandleContainerInspect_BindMount(t *testing.T) {
	resp := validContainerInspectResponse()
	resp.Mounts = []containertypes.MountPoint{
		{
			Type:        mount.TypeBind,
			Source:      "/host/path",
			Destination: "/container/path",
			RW:          false,
		},
	}

	stub := &stubContainerInspector{result: resp}
	mux := newContainerInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/containers/abc123def", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	mounts := body["mounts"].([]any)
	m := mounts[0].(map[string]any)

	if m["type"] != "bind" {
		t.Errorf("mount type = %q, want %q", m["type"], "bind")
	}
	if m["rw"] != false {
		t.Errorf("mount rw = %v, want false", m["rw"])
	}
	// Bind mounts with no Name should not have a name key.
	if _, exists := m["name"]; exists {
		t.Error("bind mount without Name should not have name key")
	}
}

func TestHandleContainerInspect_ExitedContainer(t *testing.T) {
	resp := validContainerInspectResponse()
	resp.State.Status = "exited"
	resp.State.Running = false
	resp.State.ExitCode = 137
	resp.State.OOMKilled = true
	resp.State.FinishedAt = "2025-01-15T12:00:00Z"

	stub := &stubContainerInspector{result: resp}
	mux := newContainerInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/containers/abc123def", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["status"] != "exited" {
		t.Errorf("status = %q, want %q", body["status"], "exited")
	}
	if body["running"] != false {
		t.Errorf("running = %v, want false", body["running"])
	}
	if ec, ok := body["exitCode"].(float64); !ok || ec != 137 {
		t.Errorf("exitCode = %v, want 137", body["exitCode"])
	}
	if body["oomKilled"] != true {
		t.Errorf("oomKilled = %v, want true", body["oomKilled"])
	}
	if body["finishedAt"] != "2025-01-15T12:00:00Z" {
		t.Errorf("finishedAt = %q, want %q", body["finishedAt"], "2025-01-15T12:00:00Z")
	}
}
