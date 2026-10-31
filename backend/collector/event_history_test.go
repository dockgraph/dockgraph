package collector

import (
	"strings"
	"testing"
	"time"
)

func TestEventHistory_AddAndRecent(t *testing.T) {
	h := NewEventHistory(100)

	h.Add(DockerEvent{
		Timestamp: time.Now(),
		Action:    "start",
		Type:      "container",
		Name:      "web",
	})
	h.Add(DockerEvent{
		Timestamp: time.Now(),
		Action:    "connect",
		Type:      "network",
		Name:      "app-net",
	})

	events := h.Recent(10)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Name != "app-net" {
		t.Errorf("first event name = %q, want %q", events[0].Name, "app-net")
	}
}

func TestEventHistory_KeepsOnlyEventAttributes(t *testing.T) {
	h := NewEventHistory(10)
	attrs := map[string]string{
		"name":                       "web",
		"image":                      "nginx:1.27",
		"exitCode":                   "137",
		"signal":                     "9",
		"com.docker.compose.project": "shop",
		"app.db.password":            "LabelPass1",
	}
	h.Add(DockerEvent{Timestamp: time.Now(), Action: "die", Type: "container", Name: "web", Attributes: attrs})

	got := h.Recent(1)[0].Attributes
	want := map[string]string{"name": "web", "image": "nginx:1.27", "exitCode": "137", "signal": "9"}
	if len(got) != len(want) {
		t.Fatalf("stored attributes = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("attribute %q = %q, want %q", k, got[k], v)
		}
	}
	if len(attrs) != 6 || attrs["app.db.password"] != "LabelPass1" {
		t.Error("Add must not modify the caller's map")
	}
}

func TestEventHistory_KeepsNetworkAndVolumeAttributes(t *testing.T) {
	h := NewEventHistory(10)
	h.Add(DockerEvent{Action: "connect", Type: "network", Name: "app-net", Attributes: map[string]string{
		"name": "app-net", "type": "bridge", "container": "abc123",
	}})
	h.Add(DockerEvent{Action: "mount", Type: "volume", Name: "data", Attributes: map[string]string{
		"driver": "local", "container": "abc123", "destination": "/var/lib/data", "propagation": "", "read/write": "true",
	}})

	events := h.Recent(2)
	vol, net := events[0].Attributes, events[1].Attributes
	if net["type"] != "bridge" || net["container"] != "abc123" || net["name"] != "app-net" {
		t.Errorf("network attributes = %v", net)
	}
	if len(vol) != 2 || vol["driver"] != "local" || vol["container"] != "abc123" {
		t.Errorf("volume attributes = %v, want only driver and container", vol)
	}
}

func TestEventHistory_DropsEmptyAttributes(t *testing.T) {
	h := NewEventHistory(10)
	h.Add(DockerEvent{Action: "start", Type: "container", Name: "web", Attributes: map[string]string{
		"com.docker.compose.project": "shop",
	}})
	if got := h.Recent(1)[0].Attributes; got != nil {
		t.Errorf("attributes = %v, want nil when none is kept", got)
	}
}

func TestEventHistory_CapsNameAndAttributeValues(t *testing.T) {
	h := NewEventHistory(10)
	long := strings.Repeat("a", 254) + "é" + strings.Repeat("b", 1000)
	h.Add(DockerEvent{Action: "start", Type: "container", Name: long, Attributes: map[string]string{
		"name":  long,
		"image": long,
	}})

	e := h.Recent(1)[0]
	want := strings.Repeat("a", 254)
	if e.Name != want {
		t.Errorf("name has %d bytes, want the %d-byte prefix", len(e.Name), len(want))
	}
	if e.Attributes["name"] != want || e.Attributes["image"] != want {
		t.Errorf("attribute values have %d and %d bytes, want %d", len(e.Attributes["name"]), len(e.Attributes["image"]), len(want))
	}
}

func TestEventHistory_MasksKeptAttributes(t *testing.T) {
	h := NewEventHistory(10)
	h.Add(DockerEvent{Action: "start", Type: "container", Name: "web", Attributes: map[string]string{
		"image": "postgres://admin:S3cr3tPass@db:5432/app",
	}})
	if got := h.Recent(1)[0].Attributes["image"]; strings.Contains(got, "S3cr3tPass") {
		t.Errorf("image = %q, want the credential masked", got)
	}
}

func TestEventHistory_LimitEnforced(t *testing.T) {
	h := NewEventHistory(5)

	for i := 0; i < 10; i++ {
		h.Add(DockerEvent{
			Timestamp: time.Now(),
			Action:    "start",
			Type:      "container",
			Name:      "c",
		})
	}

	events := h.Recent(100)
	if len(events) != 5 {
		t.Fatalf("expected 5 events (capacity), got %d", len(events))
	}
}

func TestEventHistory_RecentLimit(t *testing.T) {
	h := NewEventHistory(100)

	for i := 0; i < 20; i++ {
		h.Add(DockerEvent{
			Timestamp: time.Now(),
			Action:    "start",
			Type:      "container",
			Name:      "c",
		})
	}

	events := h.Recent(5)
	if len(events) != 5 {
		t.Errorf("expected 5 events, got %d", len(events))
	}
}
