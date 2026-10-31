package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	volumetypes "github.com/docker/docker/api/types/volume"
)

// stubVolumeInspector implements VolumeInspector for tests.
type stubVolumeInspector struct {
	result volumetypes.Volume
	err    error
}

func (s *stubVolumeInspector) VolumeInspect(_ context.Context, _ string) (volumetypes.Volume, error) {
	return s.result, s.err
}

// newVolumeInspectServer registers the handler on a mux so PathValue works.
func newVolumeInspectServer(inspector VolumeInspector) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/volumes/{name}", HandleVolumeInspect(inspector))
	return mux
}

func validVolume() volumetypes.Volume {
	return volumetypes.Volume{
		Name:       "my-volume",
		Driver:     "local",
		Mountpoint: "/var/lib/docker/volumes/my-volume/_data",
		CreatedAt:  "2025-01-15T10:00:00Z",
		Scope:      "local",
		Labels:     map[string]string{"app": "database", "tier": "storage"},
		Options:    map[string]string{"type": "nfs", "device": ":/exports/data"},
		Status:     map[string]any{"mounted": true},
	}
}

func TestHandleVolumeInspect_MasksDriverCredentials(t *testing.T) {
	vol := validVolume()
	vol.Options = map[string]string{"type": "cifs", "device": "//nas/share", "o": "addr=10.0.0.5,username=app,password=CifsPass1"}
	vol.Labels = map[string]string{"backup.token": "tok-123", "app": "files"}

	w := httptest.NewRecorder()
	newVolumeInspectServer(&stubVolumeInspector{result: vol}).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/volumes/my-volume", nil))

	var body struct {
		Options map[string]string `json:"options"`
		Labels  map[string]string `json:"labels"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := body.Options["o"]; got != "addr=10.0.0.5,username=app,password=********" {
		t.Errorf(`options["o"] = %q`, got)
	}
	if body.Labels["backup.token"] != "********" || body.Labels["app"] != "files" {
		t.Errorf("labels = %v", body.Labels)
	}
}

// Driver status is free-form and masked like options.
func TestHandleVolumeInspect_MasksDriverStatus(t *testing.T) {
	vol := validVolume()
	vol.Status = map[string]any{"mounted": true, "token": "tok-123", "source": "smb://app:SmbPass1@nas/share"}

	w := httptest.NewRecorder()
	newVolumeInspectServer(&stubVolumeInspector{result: vol}).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/volumes/my-volume", nil))

	var body struct {
		Status map[string]any `json:"status"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status["token"] != "********" || body.Status["source"] != "smb://app:********@nas/share" || body.Status["mounted"] != true {
		t.Errorf("status = %v", body.Status)
	}
}

// Nested status values are masked too.
func TestHandleVolumeInspect_MasksNestedDriverStatus(t *testing.T) {
	vol := validVolume()
	vol.Status = map[string]any{
		"mount": map[string]any{"password": "NestedPass1", "server": "nas"},
		"args":  []any{"--password", "ListPass1", "--port", "445"},
		"mixed": []any{"-o", "--password", "MixedPass1", 1},
		"urls":  []any{"smb://app:UrlPass1@nas/share"},
	}

	w := httptest.NewRecorder()
	newVolumeInspectServer(&stubVolumeInspector{result: vol}).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/volumes/my-volume", nil))
	body := w.Body.String()
	for _, secret := range []string{"NestedPass1", "ListPass1", "MixedPass1", "UrlPass1"} {
		if strings.Contains(body, secret) {
			t.Errorf("%s in %s", secret, body)
		}
	}
	for _, kept := range []string{`"server":"nas"`, `"445"`} {
		if !strings.Contains(body, kept) {
			t.Errorf("%s missing from %s", kept, body)
		}
	}
}

func TestHandleVolumeInspect_ValidVolume(t *testing.T) {
	stub := &stubVolumeInspector{result: validVolume()}
	mux := newVolumeInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/volumes/my-volume", nil)
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

	if body["name"] != "my-volume" {
		t.Errorf("name = %q, want %q", body["name"], "my-volume")
	}
	if body["driver"] != "local" {
		t.Errorf("driver = %q, want %q", body["driver"], "local")
	}
	if body["mountpoint"] != "/var/lib/docker/volumes/my-volume/_data" {
		t.Errorf("mountpoint = %q, want %q", body["mountpoint"], "/var/lib/docker/volumes/my-volume/_data")
	}
	if body["createdAt"] != "2025-01-15T10:00:00Z" {
		t.Errorf("createdAt = %q, want %q", body["createdAt"], "2025-01-15T10:00:00Z")
	}
	if body["scope"] != "local" {
		t.Errorf("scope = %q, want %q", body["scope"], "local")
	}

	// Verify labels.
	labels, ok := body["labels"].(map[string]any)
	if !ok {
		t.Fatalf("labels is not an object: %T", body["labels"])
	}
	if labels["app"] != "database" {
		t.Errorf("labels[app] = %q, want %q", labels["app"], "database")
	}
	if labels["tier"] != "storage" {
		t.Errorf("labels[tier] = %q, want %q", labels["tier"], "storage")
	}

	// Verify options.
	opts, ok := body["options"].(map[string]any)
	if !ok {
		t.Fatalf("options is not an object: %T", body["options"])
	}
	if opts["type"] != "nfs" {
		t.Errorf("options[type] = %q, want %q", opts["type"], "nfs")
	}
	if opts["device"] != ":/exports/data" {
		t.Errorf("options[device] = %q, want %q", opts["device"], ":/exports/data")
	}

	// Verify status.
	status, ok := body["status"].(map[string]any)
	if !ok {
		t.Fatalf("status is not an object: %T", body["status"])
	}
	if status["mounted"] != true {
		t.Errorf("status[mounted] = %v, want true", status["mounted"])
	}

	// UsageData should not be present when nil.
	if _, exists := body["usageSize"]; exists {
		t.Error("usageSize should not be present when UsageData is nil")
	}
	if _, exists := body["usageRefCount"]; exists {
		t.Error("usageRefCount should not be present when UsageData is nil")
	}
}

func TestHandleVolumeInspect_InvalidName(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"single char", "v"},
		{"starts with dot", ".hidden-vol"},
		{"starts with dash", "-vol"},
	}

	stub := &stubVolumeInspector{}
	mux := newVolumeInspectServer(stub)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/volumes/"+tc.input, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for name %q, got %d", tc.input, w.Code)
			}

			var errBody map[string]any
			if err := json.NewDecoder(w.Body).Decode(&errBody); err != nil {
				t.Fatalf("decode error body: %v", err)
			}
			if errBody["error"] != "invalid volume name" {
				t.Errorf("error = %q, want %q", errBody["error"], "invalid volume name")
			}
		})
	}
}

func TestHandleVolumeInspect_DockerError(t *testing.T) {
	// Only a missing volume is a 404: a daemon timeout says nothing about
	// whether it exists (and, on an agent, would read as "not on this node").
	tests := []struct {
		name     string
		err      error
		wantCode int
		wantMsg  string
	}{
		{"not found", cerrdefs.ErrNotFound.WithMessage("get test-vol: no such volume"), http.StatusNotFound, "volume not found"},
		{"daemon timeout", context.DeadlineExceeded, http.StatusBadGateway, "volume inspect failed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mux := newVolumeInspectServer(&stubVolumeInspector{err: tc.err})
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/volumes/test-vol", nil))

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

func TestHandleVolumeInspect_MissingPathVariable(t *testing.T) {
	stub := &stubVolumeInspector{}
	handler := HandleVolumeInspect(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/volumes/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing path variable, got %d", w.Code)
	}
}

func TestHandleVolumeInspect_WithUsageData(t *testing.T) {
	vol := validVolume()
	vol.UsageData = &volumetypes.UsageData{
		Size:     1073741824, // 1 GiB
		RefCount: 3,
	}

	stub := &stubVolumeInspector{result: vol}
	mux := newVolumeInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/volumes/my-volume", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	usageSize, ok := body["usageSize"].(float64)
	if !ok {
		t.Fatalf("usageSize is not a number: %T", body["usageSize"])
	}
	if int64(usageSize) != 1073741824 {
		t.Errorf("usageSize = %v, want 1073741824", usageSize)
	}

	usageRefCount, ok := body["usageRefCount"].(float64)
	if !ok {
		t.Fatalf("usageRefCount is not a number: %T", body["usageRefCount"])
	}
	if int64(usageRefCount) != 3 {
		t.Errorf("usageRefCount = %v, want 3", usageRefCount)
	}
}

func TestHandleVolumeInspect_NilUsageData(t *testing.T) {
	vol := validVolume()
	vol.UsageData = nil

	stub := &stubVolumeInspector{result: vol}
	mux := newVolumeInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/volumes/my-volume", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if _, exists := body["usageSize"]; exists {
		t.Error("usageSize should not be present when UsageData is nil")
	}
	if _, exists := body["usageRefCount"]; exists {
		t.Error("usageRefCount should not be present when UsageData is nil")
	}
}

func TestHandleVolumeInspect_ZeroUsageData(t *testing.T) {
	vol := validVolume()
	vol.UsageData = &volumetypes.UsageData{
		Size:     0,
		RefCount: 0,
	}

	stub := &stubVolumeInspector{result: vol}
	mux := newVolumeInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/volumes/my-volume", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Zero values should still be present when UsageData pointer is non-nil.
	usageSize, ok := body["usageSize"].(float64)
	if !ok {
		t.Fatalf("usageSize should be present with zero value: %T", body["usageSize"])
	}
	if usageSize != 0 {
		t.Errorf("usageSize = %v, want 0", usageSize)
	}

	usageRefCount, ok := body["usageRefCount"].(float64)
	if !ok {
		t.Fatalf("usageRefCount should be present with zero value: %T", body["usageRefCount"])
	}
	if usageRefCount != 0 {
		t.Errorf("usageRefCount = %v, want 0", usageRefCount)
	}
}

func TestHandleVolumeInspect_NegativeUsageData(t *testing.T) {
	vol := validVolume()
	vol.UsageData = &volumetypes.UsageData{
		Size:     -1, // "not available" sentinel
		RefCount: -1,
	}

	stub := &stubVolumeInspector{result: vol}
	mux := newVolumeInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/volumes/my-volume", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Docker uses -1 as "not available" — the handler should still pass through.
	if size, ok := body["usageSize"].(float64); !ok || int64(size) != -1 {
		t.Errorf("usageSize = %v, want -1", body["usageSize"])
	}
	if rc, ok := body["usageRefCount"].(float64); !ok || int64(rc) != -1 {
		t.Errorf("usageRefCount = %v, want -1", body["usageRefCount"])
	}
}

func TestHandleVolumeInspect_NilLabelsAndOptions(t *testing.T) {
	vol := validVolume()
	vol.Labels = nil
	vol.Options = nil

	stub := &stubVolumeInspector{result: vol}
	mux := newVolumeInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/volumes/my-volume", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Nil maps should still appear as keys (serialized as null).
	if _, exists := body["labels"]; !exists {
		t.Error("labels key should be present even when nil")
	}
	if _, exists := body["options"]; !exists {
		t.Error("options key should be present even when nil")
	}
}

func TestHandleVolumeInspect_EmptyLabelsAndOptions(t *testing.T) {
	vol := validVolume()
	vol.Labels = map[string]string{}
	vol.Options = map[string]string{}

	stub := &stubVolumeInspector{result: vol}
	mux := newVolumeInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/volumes/my-volume", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	labels, ok := body["labels"].(map[string]any)
	if !ok {
		t.Fatalf("labels is not an object: %T", body["labels"])
	}
	if len(labels) != 0 {
		t.Errorf("expected 0 labels, got %d", len(labels))
	}

	opts, ok := body["options"].(map[string]any)
	if !ok {
		t.Fatalf("options is not an object: %T", body["options"])
	}
	if len(opts) != 0 {
		t.Errorf("expected 0 options, got %d", len(opts))
	}
}

func TestHandleVolumeInspect_NilStatus(t *testing.T) {
	vol := validVolume()
	vol.Status = nil

	stub := &stubVolumeInspector{result: vol}
	mux := newVolumeInspectServer(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/volumes/my-volume", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Status key is always present in the response map.
	if _, exists := body["status"]; !exists {
		t.Error("status key should be present even when nil")
	}
}

func TestHandleVolumeInspect_DifferentDrivers(t *testing.T) {
	tests := []struct {
		name   string
		driver string
	}{
		{"local driver", "local"},
		{"nfs driver", "nfs"},
		{"azure file driver", "azurefile"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vol := validVolume()
			vol.Driver = tc.driver

			stub := &stubVolumeInspector{result: vol}
			mux := newVolumeInspectServer(stub)

			req := httptest.NewRequest(http.MethodGet, "/api/volumes/my-volume", nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			var body map[string]any
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("decode: %v", err)
			}

			if body["driver"] != tc.driver {
				t.Errorf("driver = %q, want %q", body["driver"], tc.driver)
			}
		})
	}
}
