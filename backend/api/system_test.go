package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	dockertypes "github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	imagetypes "github.com/docker/docker/api/types/image"
	systemtypes "github.com/docker/docker/api/types/system"
)

type stubSystemInfoProvider struct {
	info systemtypes.Info
	err  error
}

func (s *stubSystemInfoProvider) Info(_ context.Context) (systemtypes.Info, error) {
	return s.info, s.err
}

type stubDiskUsageProvider struct {
	du  dockertypes.DiskUsage
	err error
}

func (s *stubDiskUsageProvider) DiskUsage(_ context.Context, _ dockertypes.DiskUsageOptions) (dockertypes.DiskUsage, error) {
	return s.du, s.err
}

type stubImageLister struct {
	images []imagetypes.Summary
	err    error
}

func (s *stubImageLister) ImageList(_ context.Context, _ imagetypes.ListOptions) ([]imagetypes.Summary, error) {
	return s.images, s.err
}

func TestHandleSystemInfo(t *testing.T) {
	stub := &stubSystemInfoProvider{
		info: systemtypes.Info{
			ServerVersion:   "28.5.2",
			OperatingSystem: "Ubuntu 24.04",
			Architecture:    "x86_64",
			KernelVersion:   "6.6.87",
			Driver:          "overlay2",
			NCPU:            8,
			MemTotal:        16777216000,
			CgroupVersion:   "2",
		},
	}
	handler := HandleSystemInfo(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/system/info", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["dockerVersion"] != "28.5.2" {
		t.Errorf("dockerVersion = %v, want 28.5.2", body["dockerVersion"])
	}
	if body["cpus"] != float64(8) {
		t.Errorf("cpus = %v, want 8", body["cpus"])
	}
	if body["os"] != "Ubuntu 24.04" {
		t.Errorf("os = %v, want Ubuntu 24.04", body["os"])
	}
}

func TestHandleSystemInfo_Error(t *testing.T) {
	stub := &stubSystemInfoProvider{err: fmt.Errorf("daemon unreachable")}
	handler := HandleSystemInfo(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/system/info", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleSystemDiskUsage(t *testing.T) {
	stub := &stubDiskUsageProvider{
		du: dockertypes.DiskUsage{
			Images: []*imagetypes.Summary{
				{Size: 1000, RepoTags: []string{"app:latest"}},
				{Size: 2000, RepoTags: []string{"<none>:<none>"}},
			},
			Containers: []*containertypes.Summary{
				{SizeRw: 500, State: "running"},
			},
		},
	}
	handler := HandleSystemDiskUsage(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/system/disk-usage", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	images, ok := body["images"].(map[string]any)
	if !ok {
		t.Fatal("missing images in response")
	}
	if images["count"] != float64(2) {
		t.Errorf("images.count = %v, want 2", images["count"])
	}
	if images["total"] != float64(3000) {
		t.Errorf("images.total = %v, want 3000", images["total"])
	}
}

func TestHandleImages(t *testing.T) {
	stub := &stubImageLister{
		images: []imagetypes.Summary{
			{Size: 1000, RepoTags: []string{"app:latest", "app:v1"}},
			{Size: 500, RepoTags: []string{"<none>:<none>"}},
			{Size: 300, RepoTags: []string{"redis:7"}},
		},
	}
	handler := HandleImages(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/images", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["total"] != float64(3) {
		t.Errorf("total = %v, want 3", body["total"])
	}
	if body["dangling"] != float64(1) {
		t.Errorf("dangling = %v, want 1", body["dangling"])
	}
	if body["uniqueTags"] != float64(3) {
		t.Errorf("uniqueTags = %v, want 3", body["uniqueTags"])
	}
}
