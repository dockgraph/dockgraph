package collector

import (
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	volumetypes "github.com/docker/docker/api/types/volume"
)

func TestProjectOf(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   string
	}{
		{"nil", nil, ""},
		{"none", map[string]string{"x": "y"}, ""},
		{"compose", map[string]string{composeProjectLabel: "app"}, "app"},
		{"stack fallback", map[string]string{StackNamespaceLabel: "shop"}, "shop"},
		{"compose wins", map[string]string{composeProjectLabel: "app", StackNamespaceLabel: "shop"}, "app"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ProjectOf(tt.labels); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProjectLabels(t *testing.T) {
	if got := projectLabels(map[string]string{"other": "x"}); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
	// The stack namespace is carried by Node.Stack, not the payload labels.
	if got := projectLabels(map[string]string{StackNamespaceLabel: "b"}); got != nil {
		t.Errorf("stack namespace forwarded: %v", got)
	}
	got := projectLabels(map[string]string{composeProjectLabel: "a", StackNamespaceLabel: "b", "other": "x"})
	if len(got) != 1 || got[composeProjectLabel] != "a" {
		t.Errorf("got %v", got)
	}
}

func TestResolveNetworkNamesSkipsSwarmPlumbing(t *testing.T) {
	got := resolveNetworkNames([]networktypes.Summary{
		{ID: "1", Name: "ingress"},
		{ID: "2", Name: "docker_gwbridge"},
		{ID: "3", Name: "custom_mesh", Ingress: true},
		{ID: "4", Name: "app_net"},
	})
	if len(got) != 1 || got["4"] != "app_net" {
		t.Errorf("got %v", got)
	}
}

// Standalone mode: stack-labelled resources get Node.Stack; the stack label
// itself stays out of the payload labels.
func TestAssembleSnapshotSetsStack(t *testing.T) {
	stack := map[string]string{StackNamespaceLabel: "shop"}
	res := dockerResources{
		containers: []containertypes.Summary{
			{Names: []string{"/shop_web.1.x"}, State: "running", Labels: stack},
			{Names: []string{"/api-1"}, State: "running", Labels: map[string]string{composeProjectLabel: "app"}},
		},
		networks: []networktypes.Summary{{ID: "n", Name: "shop_net", Labels: stack}},
		volumes:  []*volumetypes.Volume{{Name: "shop_data", Labels: stack}},
	}
	snap := assembleSnapshot(res, nil)

	for _, id := range []string{"container:shop_web.1.x", "network:shop_net", "volume:shop_data"} {
		n := findNodeByID(snap.Nodes, id)
		if n == nil {
			t.Fatalf("%s missing", id)
		}
		if n.Stack != "shop" || n.Labels != nil {
			t.Errorf("%s: stack=%q labels=%v", id, n.Stack, n.Labels)
		}
	}
	if n := findNodeByID(snap.Nodes, "container:api-1"); n.Stack != "app" {
		t.Errorf("compose container stack = %q", n.Stack)
	}
}

func TestSelfOnlyProjectsCoversStacks(t *testing.T) {
	got := selfOnlyProjects([]projectMember{
		{project: "dg", self: true},
		{project: "mixed", self: true},
		{project: "mixed", self: false},
		{project: "", self: true},
	})
	if len(got) != 1 || !got["dg"] {
		t.Errorf("got %v", got)
	}
}
