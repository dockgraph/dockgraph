package collector

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func stackFile() string { return filepath.Join(testdataDir(), "stacks", "shop.yml") }

func TestParseStackFileBasenameFallback(t *testing.T) {
	snap, err := parseComposeFileWith(context.Background(), stackFile(), "shop.yml", composeParseOptions{Swarm: true})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	web := findNodeByID(snap.Nodes, "service:shop_web")
	if web == nil {
		t.Fatalf("ghost service missing: %+v", snap.Nodes)
	}
	if web.Type != "service" || web.Name != "shop_web" || web.Stack != "shop" || web.Status != "not_running" {
		t.Errorf("ghost = %+v", web)
	}
	if web.Service == nil || web.Service.Mode != "replicated" || web.Service.Replicas.Desired != 3 {
		t.Errorf("service info = %+v", web.Service)
	}
	if web.NetworkID != "network:shop_back" {
		t.Errorf("networkId = %s", web.NetworkID)
	}

	if db := findNodeByID(snap.Nodes, "service:shop_db"); db == nil || db.Service.Replicas.Desired != 1 {
		t.Errorf("db default replicas: %+v", db)
	}
	if agent := findNodeByID(snap.Nodes, "service:shop_agent"); agent == nil || agent.Service.Mode != "global" {
		t.Errorf("global service: %+v", agent)
	}
	if findNodeByID(snap.Nodes, "service:shop_dockgraph") != nil {
		t.Error("deploy.labels self-exclusion not applied")
	}
	for _, id := range []string{"network:shop_front", "network:shop_back", "volume:shop_data"} {
		if n := findNodeByID(snap.Nodes, id); n == nil || n.Stack != "shop" {
			t.Errorf("%s missing or without stack: %+v", id, n)
		}
	}
	if len(filterNodes(snap.Nodes, "container")) != 0 {
		t.Error("swarm mode must not produce container ghosts")
	}

	for _, id := range []string{"e:dep:shop_web:shop_db", "e:vol:shop_data:shop_web", "e:net:shop_web:shop_front"} {
		if !hasEdge(snap.Edges, id) {
			t.Errorf("missing edge %s in %+v", id, snap.Edges)
		}
	}
	for _, e := range snap.Edges {
		if e.Type == "depends_on" && e.Target != "service:shop_db" {
			t.Errorf("depends_on target = %s", e.Target)
		}
	}
}

func TestParseStackFileExplicitName(t *testing.T) {
	snap, err := parseComposeFileWith(context.Background(), stackFile(), "shop.yml", composeParseOptions{ProjectName: "prod", Swarm: true})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if findNodeByID(snap.Nodes, "service:prod_web") == nil {
		t.Errorf("explicit name not applied: %+v", snap.Nodes)
	}
}

func TestParseComposeFileExplicitNameOverridesFile(t *testing.T) {
	snap, err := parseComposeFileWith(context.Background(), filepath.Join(testdataDir(), "simple.yaml"), "simple.yaml", composeParseOptions{ProjectName: "Renamed"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	api := findNodeByID(snap.Nodes, "container:renamed-api-1")
	if api == nil {
		t.Fatalf("explicit name should override file name (normalized): %+v", snap.Nodes)
	}
	if api.Stack != "renamed" {
		t.Errorf("stack = %q", api.Stack)
	}
}

func TestParseComposeFileStandaloneStillRequiresName(t *testing.T) {
	if _, err := parseComposeFile(context.Background(), stackFile(), "shop.yml"); err == nil {
		t.Error("standalone compose file without name should still fail")
	}
}

func TestSplitNamedPath(t *testing.T) {
	tests := []struct {
		in, name, path string
	}{
		{"/srv/stack.yml", "", "/srv/stack.yml"},
		{"shop=/srv/stack.yml", "shop", "/srv/stack.yml"},
		{"/srv/a=b.yml", "", "/srv/a=b.yml"},
		{"bad name=/x.yml", "", "bad name=/x.yml"},
		{"shop=", "", "shop="},
	}
	for _, tt := range tests {
		name, path := splitNamedPath(tt.in)
		if name != tt.name || path != tt.path {
			t.Errorf("splitNamedPath(%q) = %q, %q; want %q, %q", tt.in, name, path, tt.name, tt.path)
		}
	}
}

func TestComposeCollectorProjectNameFor(t *testing.T) {
	c := NewComposeCollector([]string{"shop=/srv/shop.yml", "blog=/srv/blog", "/srv/plain"})
	if len(c.paths) != 3 || c.paths[0] != "/srv/shop.yml" {
		t.Fatalf("paths = %v", c.paths)
	}
	tests := map[string]string{
		"/srv/shop.yml":        "shop",
		"/srv/blog/stack.yml":  "blog",
		"/srv/blogger/x.yml":   "",
		"/srv/plain/other.yml": "",
	}
	for file, want := range tests {
		if got := c.projectNameFor(file); got != want {
			t.Errorf("projectNameFor(%q) = %q, want %q", file, got, want)
		}
	}
}

func TestComposeCollectorSwarmScan(t *testing.T) {
	dir := t.TempDir()
	data, err := os.ReadFile(stackFile())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "stack.yml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	c := NewComposeCollector([]string{"blog=" + path})
	c.EnableSwarm()
	if err := c.scan(context.Background()); err != nil {
		t.Fatalf("scan: %v", err)
	}
	u := <-c.Updates()
	if findNodeByID(u.Snapshot.Nodes, "service:blog_web") == nil {
		t.Errorf("named stack path not applied: %+v", u.Snapshot.Nodes)
	}
}

func TestParseStackFileExternalResources(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.yml")
	content := `services:
  web:
    image: nginx
    networks: [proxy, back]
    volumes:
      - shared:/shared
      - own:/own
networks:
  proxy:
    external: true
  back:
volumes:
  shared:
    external: true
  own:
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	snap, err := parseComposeFileWith(context.Background(), path, "app.yml", composeParseOptions{ProjectName: "prod", Swarm: true})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n := findNodeByID(snap.Nodes, "network:proxy"); n == nil || n.Stack != "" {
		t.Errorf("external network should keep its own name and no stack: %+v", n)
	}
	if n := findNodeByID(snap.Nodes, "volume:shared"); n == nil || n.Stack != "" {
		t.Errorf("external volume should keep its own name and no stack: %+v", n)
	}
	for _, id := range []string{"network:prod_back", "volume:prod_own"} {
		if n := findNodeByID(snap.Nodes, id); n == nil || n.Stack != "prod" {
			t.Errorf("%s missing or without stack: %+v", id, n)
		}
	}
	if findNodeByID(snap.Nodes, "network:prod_proxy") != nil || findNodeByID(snap.Nodes, "volume:prod_shared") != nil {
		t.Error("external resources must not get the stack prefix")
	}
	if !hasEdge(snap.Edges, "e:vol:shared:prod_web") {
		t.Errorf("missing external volume edge in %+v", snap.Edges)
	}
}
