package collector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	composetypes "github.com/compose-spec/compose-go/v2/types"
)

func testdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "testdata")
}

func TestParseComposeFile(t *testing.T) {
	snap, err := parseComposeFile(context.Background(), filepath.Join(testdataDir(), "simple.yaml"), "simple.yaml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	containers := filterNodes(snap.Nodes, "container")
	networks := filterNodes(snap.Nodes, "network")
	volumes := filterNodes(snap.Nodes, "volume")

	if len(containers) != 3 {
		t.Errorf("expected 3 containers, got %d", len(containers))
	}
	if len(networks) != 2 {
		t.Errorf("expected 2 networks, got %d", len(networks))
	}
	if len(volumes) != 1 {
		t.Errorf("expected 1 volume, got %d", len(volumes))
	}

	for _, n := range snap.Nodes {
		if n.Source != "simple.yaml" {
			t.Errorf("expected source simple.yaml, got %s for node %s", n.Source, n.ID)
		}
	}
}

func TestParseDependsOnEdges(t *testing.T) {
	snap, err := parseComposeFile(context.Background(), filepath.Join(testdataDir(), "simple.yaml"), "simple.yaml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	dependsOn := filterEdges(snap.Edges, "depends_on")
	if len(dependsOn) != 2 {
		t.Errorf("expected 2 depends_on edges, got %d", len(dependsOn))
	}
}

// The side panel renders ComposeConfig.DependsOn as clickable links that
// navigate to "container:<value>". Those values must equal the full container
// node names (project-prefixed, replica-suffixed) so the link resolves to an
// actual node — bare service names like "postgres" match nothing.
func TestComposeConfigDependsOnUsesFullContainerNames(t *testing.T) {
	snap, err := parseComposeFile(context.Background(), filepath.Join(testdataDir(), "simple.yaml"), "simple.yaml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	nodeIDs := make(map[string]bool)
	for _, n := range snap.Nodes {
		nodeIDs[n.ID] = true
	}

	var checked int
	for _, n := range snap.Nodes {
		if n.Compose == nil {
			continue
		}
		for _, dep := range n.Compose.DependsOn {
			checked++
			if !nodeIDs["container:"+dep] {
				t.Errorf("node %s depends_on %q resolves to missing node container:%s", n.ID, dep, dep)
			}
		}
	}
	if checked == 0 {
		t.Fatal("expected at least one depends_on entry to verify")
	}
}

// The side panel renders ComposeConfig.Volumes as mount rows, with named
// volumes linking to "volume:<name>". The Name field must hold the full,
// project-prefixed volume node name so the link resolves to an actual node.
func TestComposeConfigVolumesUseFullVolumeNodeNames(t *testing.T) {
	snap, err := parseComposeFile(context.Background(), filepath.Join(testdataDir(), "simple.yaml"), "simple.yaml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	nodeIDs := make(map[string]bool)
	for _, n := range snap.Nodes {
		nodeIDs[n.ID] = true
	}

	var checked int
	for _, n := range snap.Nodes {
		if n.Compose == nil {
			continue
		}
		for _, m := range n.Compose.Volumes {
			if m.Type != mountTypeVolume {
				continue
			}
			checked++
			if m.Name == "" {
				t.Errorf("node %s named volume mount to %q has empty Name", n.ID, m.Destination)
				continue
			}
			if !nodeIDs["volume:"+m.Name] {
				t.Errorf("node %s volume mount %q resolves to missing node volume:%s", n.ID, m.Name, m.Name)
			}
		}
	}
	if checked == 0 {
		t.Fatal("expected at least one named volume mount to verify")
	}
}

// Exercises the full mount mapping that the parse-based tests can't reach with
// the shared fixture: a read-only named volume and a bind mount.
func TestBuildComposeConfigVolumeMounts(t *testing.T) {
	naming := composeNaming{project: "proj"}
	svc := composetypes.ServiceConfig{
		Name: "api",
		ContainerSpec: composetypes.ContainerSpec{
			Volumes: []composetypes.ServiceVolumeConfig{
				{Type: mountTypeVolume, Source: "db_data", Target: "/data", ReadOnly: true},
				{Type: mountTypeBind, Source: "/etc/app", Target: "/config"},
			},
		},
	}

	cfg := buildComposeConfig(svc, naming)
	if len(cfg.Volumes) != 2 {
		t.Fatalf("expected 2 mounts, got %d", len(cfg.Volumes))
	}

	want := []ComposeMount{
		{Type: mountTypeVolume, Source: "db_data", Destination: "/data", RW: false, Name: "proj_db_data"},
		{Type: mountTypeBind, Source: "/etc/app", Destination: "/config", RW: true, Name: ""},
	}
	for i, w := range want {
		if cfg.Volumes[i] != w {
			t.Errorf("mount[%d] = %+v, want %+v", i, cfg.Volumes[i], w)
		}
	}
}

func TestBuildComposeConfigMasksSecretsAndCapturesLabels(t *testing.T) {
	secret := "s3cr3t"
	level := "info"
	svc := composetypes.ServiceConfig{
		Name: "api",
		ContainerSpec: composetypes.ContainerSpec{
			Environment: composetypes.MappingWithEquals{
				"DB_PASSWORD": &secret,
				"LOG_LEVEL":   &level,
			},
			Labels: composetypes.Labels{
				"com.example.team": "platform",
				"traefik.http.middlewares.auth.basicauth.users": "admin:$apr1$hash",
			},
		},
	}

	cfg := buildComposeConfig(svc, composeNaming{project: "proj"})

	if got := cfg.Environment["DB_PASSWORD"]; got != "********" {
		t.Errorf("expected masked password, got %q", got)
	}
	if got := cfg.Environment["LOG_LEVEL"]; got != "info" {
		t.Errorf("expected LOG_LEVEL=info, got %q", got)
	}
	if got := cfg.Labels["com.example.team"]; got != "platform" {
		t.Errorf("expected label captured, got %q", got)
	}
	if got := cfg.Labels["traefik.http.middlewares.auth.basicauth.users"]; got != "********" {
		t.Errorf("expected credential label masked, got %q", got)
	}
}

// env_file and label_file would be read from DockGraph's own filesystem: a
// compose file must not be able to surface those files' contents.
func TestParseComposeFileIgnoresEnvAndLabelFiles(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "ignored.env")
	labelFile := filepath.Join(dir, "ignored.labels")
	if err := os.WriteFile(envFile, []byte("FROM_FILE=env-file-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(labelFile, []byte("from_file=label-file-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	compose := filepath.Join(dir, "compose.yml")
	content := "name: app\nservices:\n" +
		"  app:\n    image: alpine\n" +
		"    environment:\n      SHOWN: inline\n" +
		"    env_file: [" + envFile + "]\n" +
		"    label_file: [" + labelFile + "]\n"
	if err := os.WriteFile(compose, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	snap, err := parseComposeFile(context.Background(), compose, "compose.yml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var cfg *ComposeConfig
	for _, n := range snap.Nodes {
		if n.Compose != nil && n.Compose.Service == "app" {
			cfg = n.Compose
		}
	}
	if cfg == nil {
		t.Fatal("service app not found")
	}
	if cfg.Environment["SHOWN"] != "inline" {
		t.Errorf("inline environment missing: %v", cfg.Environment)
	}
	for k, v := range cfg.Environment {
		if strings.Contains(k+v, "env-file-value") || k == "FROM_FILE" {
			t.Errorf("env_file content shown: %s=%s", k, v)
		}
	}
	if _, ok := cfg.Labels["from_file"]; ok {
		t.Errorf("label_file content shown: %v", cfg.Labels)
	}
}

// writeFiles writes name → content files into dir.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// composeServices returns the compose services of a snapshot, sorted.
func composeServices(snap GraphSnapshot) []string {
	var services []string
	for _, n := range snap.Nodes {
		if n.Compose != nil {
			services = append(services, n.Compose.Service)
		}
	}
	slices.Sort(services)
	return services
}

// Included files are part of the project, so a service may depend on one
// of theirs. An include's env_file, or the .env next to the included file,
// would be read from DockGraph's own filesystem to fill in the included
// file's variables; it never is.
func TestParseComposeFileIncludeWithoutItsEnvFiles(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"ignored.env": "FROM_FILE=env-file-value\n",
		"sub/.env":    "FROM_FILE=dot-env-value\n",
		"sub/db.yml": "services:\n  db:\n    image: \"img-${FROM_FILE}\"\n" +
			"    labels: {shown: \"${FROM_FILE}\"}\n    environment: [FROM_FILE]\n" +
			"networks:\n  backend: {}\nvolumes:\n  data: {}\n",
		"sub/cache.yml": "services:\n  cache:\n    image: \"img-${FROM_FILE}\"\n",
		"compose.yml": "name: app\ninclude:\n  - path: sub/db.yml\n    env_file: ignored.env\n  - sub/cache.yml\n" +
			"services:\n  web:\n    image: nginx\n    depends_on: [db, cache]\n" +
			"    networks: [backend]\n    volumes: [data:/data]\n",
	})

	snap, err := parseComposeFile(context.Background(), filepath.Join(dir, "compose.yml"), "compose.yml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if got := composeServices(snap); !slices.Equal(got, []string{"cache", "db", "web"}) {
		t.Errorf("services = %v, want the included ones too", got)
	}
	for _, n := range snap.Nodes {
		raw, _ := json.Marshal(n)
		if strings.Contains(string(raw), "env-file-value") || strings.Contains(string(raw), "dot-env-value") {
			t.Errorf("include env file content shown: %s", raw)
		}
	}
}

// parseProject writes files into a new directory, {{DIR}} in their content
// standing for that directory, and parses its compose.yml.
func parseProject(t *testing.T, files map[string]string) (GraphSnapshot, error) {
	t.Helper()
	dir := t.TempDir()
	written := make(map[string]string, len(files))
	for name, content := range files {
		written[name] = strings.ReplaceAll(content, "{{DIR}}", dir)
	}
	writeFiles(t, dir, written)
	return parseComposeFile(context.Background(), filepath.Join(dir, "compose.yml"), "compose.yml")
}

// composeNode returns the snapshot's node of compose service name.
func composeNode(t *testing.T, snap GraphSnapshot, name string) Node {
	t.Helper()
	for _, n := range snap.Nodes {
		if n.Compose != nil && n.Compose.Service == name {
			return n
		}
	}
	t.Fatalf("service %s not found in %v", name, composeServices(snap))
	return Node{}
}

// An included file's own includes are relative to it.
func TestParseComposeFileNestedInclude(t *testing.T) {
	snap, err := parseProject(t, map[string]string{
		"sub/a.yml":   "include: [b.yml]\nservices:\n  a:\n    image: alpine\n",
		"sub/b.yml":   "services:\n  b:\n    image: alpine\n",
		"compose.yml": "name: app\ninclude: [sub/a.yml]\nservices:\n  web:\n    image: nginx\n",
	})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if got := composeServices(snap); !slices.Equal(got, []string{"a", "b", "web"}) {
		t.Errorf("services = %v", got)
	}
}

// Like compose, an include cycle is an error.
func TestParseComposeFileIncludeCycle(t *testing.T) {
	_, err := parseProject(t, map[string]string{
		"sub/a.yml":   "include: [../compose.yml]\nservices:\n  a:\n    image: alpine\n",
		"compose.yml": "name: app\ninclude: [sub/a.yml]\nservices:\n  web:\n    image: nginx\n",
	})
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Errorf("err = %v, want an include cycle", err)
	}
}

// Included services are part of the file before its extends are applied.
func TestParseComposeFileExtendsIncludedService(t *testing.T) {
	snap, err := parseProject(t, map[string]string{
		"sub/base.yml": "services:\n  base:\n    image: alpine\n",
		"compose.yml":  "name: app\ninclude: [sub/base.yml]\nservices:\n  web:\n    extends: {service: base}\n",
	})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if got := composeNode(t, snap, "web").Image; got != "alpine" {
		t.Errorf("web image = %q, want alpine from the included base", got)
	}
}

// An included file's relative paths (extends files, bind mounts) are
// relative to its own directory.
func TestParseComposeFileIncludedRelativePaths(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"sub/base.yml": "services:\n  base:\n    image: postgres\n",
		"sub/db.yml": "services:\n  db:\n    extends: {file: base.yml, service: base}\n" +
			"    volumes: [./data:/var/lib/postgresql/data]\n",
		"compose.yml": "name: app\ninclude: [sub/db.yml]\nservices:\n  web:\n    image: nginx\n",
	})
	snap, err := parseComposeFile(context.Background(), filepath.Join(dir, "compose.yml"), "compose.yml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	db := composeNode(t, snap, "db")
	if db.Image != "postgres" {
		t.Errorf("db image = %q, want postgres from sub/base.yml", db.Image)
	}
	if vols := db.Compose.Volumes; len(vols) != 1 || vols[0].Source != filepath.Join(dir, "sub", "data") {
		t.Errorf("db volumes = %+v, want the bind source under sub/", vols)
	}
}

// The compose file's own relative paths are relative to its directory, as
// with `docker compose -f`, whatever DockGraph's working directory: an
// extends file, the paths in it relative to it, and bind mounts.
func TestParseComposeFileRelativePaths(t *testing.T) {
	for name, include := range map[string]string{"without include": "", "with include": "include: [inc/side.yml]\n"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, map[string]string{
				"compose.yml": "name: app\n" + include + "services:\n" +
					"  web:\n    extends: {file: common/base.yml, service: base}\n    image: nginx\n    volumes: [./html:/html]\n" +
					"  api:\n    extends: {file: common/base.yml, service: chained}\n",
				"common/base.yml": "services:\n  base:\n    volumes: [./conf:/conf]\n" +
					"  chained:\n    extends: {file: deeper/root.yml, service: root}\n    image: api\n",
				"common/deeper/root.yml": "services:\n  root:\n    volumes: [./data:/data]\n",
				// An included file using include: its paths are relative to it.
				"inc/side.yml": "include: [more.yml]\nservices:\n  side:\n    image: busybox\n    volumes: [./logs:/logs]\n",
				"inc/more.yml": "services:\n  more:\n    image: alpine\n",
			})
			snap, err := parseComposeFile(context.Background(), filepath.Join(dir, "compose.yml"), "compose.yml")
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			want := map[string][]string{
				"web": {filepath.Join(dir, "common", "conf"), filepath.Join(dir, "html")},
				"api": {filepath.Join(dir, "common", "deeper", "data")},
			}
			if include != "" {
				want["side"] = []string{filepath.Join(dir, "inc", "logs")}
			}
			for service, want := range want {
				var got []string
				for _, v := range composeNode(t, snap, service).Compose.Volumes {
					got = append(got, v.Source)
				}
				if !slices.Equal(got, want) {
					t.Errorf("%s bind sources = %v, want %v", service, got, want)
				}
			}
			if got := composeNode(t, snap, "api").Image; got != "api" {
				t.Errorf("api image = %q, want api from common/base.yml", got)
			}
		})
	}
}

// An extends of a file using include, or of a file it includes that uses
// include, counts once against the extends budget.
func TestParseComposeFileIncludingFileExtendsCountedOnce(t *testing.T) {
	base := "services:\n  base:\n    image: alpine\n    x-pad: [a" + strings.Repeat(",a", maxExtendsNodes*6/10) + "]\n"
	for name, files := range map[string]map[string]string{
		"including file": {
			"base.yml":    base,
			"side.yml":    "services:\n  side:\n    image: busybox\n",
			"compose.yml": "name: app\ninclude: [side.yml]\nservices:\n  web:\n    extends: {file: base.yml, service: base}\n",
		},
		"included file using include": {
			"sub/base.yml": base,
			"sub/side.yml": "services:\n  side:\n    image: busybox\n",
			"sub/app.yml":  "include: [side.yml]\nservices:\n  web:\n    extends: {file: base.yml, service: base}\n",
			"compose.yml":  "name: app\ninclude: [sub/app.yml]\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			snap, err := parseProject(t, files)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if got := composeNode(t, snap, "web").Image; got != "alpine" {
				t.Errorf("web image = %q, want alpine from base.yml", got)
			}
		})
	}
}

// The compose file's relative extends files are checked as the loader
// opens them, relative to the file.
func TestParseComposeFileRelativeExtendsChecked(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"big.yml":     "services:\n  base:\n    image: alpine\n# " + strings.Repeat("x", maxComposeFileSize) + "\n",
		"compose.yml": "name: app\nservices:\n  web:\n    extends: {file: big.yml, service: base}\n",
	})
	_, err := parseComposeFile(context.Background(), filepath.Join(dir, "compose.yml"), "compose.yml")
	if err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Errorf("err = %v, want the extends file refused for its size", err)
	}
}

// Include paths are interpolated like the rest of the file, so a default
// applies.
func TestParseComposeFileIncludePathDefault(t *testing.T) {
	snap, err := parseProject(t, map[string]string{
		"sub/db.yml":  "services:\n  db:\n    image: postgres\n",
		"compose.yml": "name: app\ninclude: [\"${LIB:-sub}/db.yml\"]\nservices:\n  web:\n    image: nginx\n    depends_on: [db]\n",
	})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if got := composeServices(snap); !slices.Equal(got, []string{"db", "web"}) {
		t.Errorf("services = %v", got)
	}
}

// Only the including file names the project.
func TestParseComposeFileIncludedNameIgnored(t *testing.T) {
	snap, err := parseProject(t, map[string]string{
		"sub/db.yml":  "name: lib\nservices:\n  db:\n    image: postgres\n",
		"compose.yml": "name: app\ninclude: [sub/db.yml]\nservices:\n  web:\n    image: nginx\n",
	})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if got := composeNode(t, snap, "db").Name; got != "app-db-1" {
		t.Errorf("db container = %q, want app-db-1", got)
	}
}

// An included or extended file naming something other than a compose file
// of sane size, such as a device, fails rather than being read without end.
// The loader reads extends files itself; they are held to the same limits.
func TestParseComposeFileReferencedFileLimits(t *testing.T) {
	for name, c := range map[string]struct{ compose, want string }{
		"include device":    {"include: [/dev/zero]\nservices:\n  web:\n    image: nginx\n", "not a regular file"},
		"include too large": {"include: [{{DIR}}/big.yml]\nservices:\n  web:\n    image: nginx\n", "larger than"},
		"extends device":    {"services:\n  web:\n    extends: {file: /dev/null, service: base}\n", "not a regular file"},
		"extends too large": {"services:\n  web:\n    extends: {file: {{DIR}}/big.yml, service: base}\n", "larger than"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseProject(t, map[string]string{
				"big.yml":     "services:\n  base:\n    image: alpine\n# " + strings.Repeat("x", maxComposeFileSize) + "\n",
				"compose.yml": "name: app\n" + c.compose,
			})
			if !refusedFor(err, c.want) {
				t.Errorf("err = %v, want %q", err, c.want)
			}
		})
	}
}

// refusedFor reports whether err refuses a file for want. Parsing in a
// child that may read only under the compose paths (-compose-child), a file
// elsewhere, such as a device, is refused before it is looked at.
func refusedFor(err error, want string) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), want) || *composeChild && strings.Contains(err.Error(), "permission denied")
}

// A compose file, an included or extended one too, is held to
// maxComposeDocuments YAML documents.
func TestParseComposeFileDocumentLimit(t *testing.T) {
	documents := func(n int) string {
		return "services:\n  web:\n    image: nginx\n" + strings.Repeat("---\nx-a: 1\n", n-1)
	}
	if _, err := parseProject(t, map[string]string{"compose.yml": "name: app\n" + documents(maxComposeDocuments)}); err != nil {
		t.Errorf("parse error within the limit: %v", err)
	}
	for name, files := range map[string]map[string]string{
		"compose file":  {"compose.yml": "name: app\n" + documents(maxComposeDocuments+1)},
		"included file": {"db.yml": documents(maxComposeDocuments + 1), "compose.yml": "name: app\ninclude: [db.yml]\n"},
		// The loader takes a name it resets; it is no string to read.
		"undecoded name": {"compose.yml": "name: app\n" + documents(2) + "---\nname: !reset {}\n" + documents(maxComposeDocuments)},
		"extends file": {
			"base.yml":    documents(maxComposeDocuments + 1),
			"compose.yml": "name: app\nservices:\n  app:\n    extends: {file: {{DIR}}/base.yml, service: web}\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseProject(t, files)
			if err == nil || !strings.Contains(err.Error(), "YAML documents") {
				t.Errorf("err = %v, want too many documents refused", err)
			}
		})
	}
}

// The extends files of one parse are bounded in number and in total size,
// as the loader keeps each one it loads.
func TestParseComposeFileExtendsBudget(t *testing.T) {
	extendsFiles := func(n int, padding string) map[string]string {
		files := map[string]string{}
		var services strings.Builder
		for i := range n {
			name := fmt.Sprintf("base%d.yml", i)
			files[name] = "services:\n  base:\n    image: alpine\n" + padding
			fmt.Fprintf(&services, "  s%d:\n    extends: {file: {{DIR}}/%s, service: base}\n", i, name)
		}
		files["compose.yml"] = "name: app\nservices:\n" + services.String()
		return files
	}
	if _, err := parseProject(t, extendsFiles(maxExtendsFiles, "")); err != nil {
		t.Errorf("parse error within the budget: %v", err)
	}
	if _, err := parseProject(t, extendsFiles(maxExtendsFiles+1, "")); err == nil || !strings.Contains(err.Error(), "extends files") {
		t.Errorf("err = %v, want too many extends files refused", err)
	}
	padding := "# " + strings.Repeat("x", maxComposeFileSize/2) + "\n"
	if _, err := parseProject(t, extendsFiles(2, padding)); err == nil || !strings.Contains(err.Error(), "in total") {
		t.Errorf("err = %v, want extends files too large in total refused", err)
	}

	// The loader reads each spelling of a path as a file of its own.
	spellings := map[string]string{"base.yml": "services:\n  base:\n    image: alpine\n"}
	var services strings.Builder
	for i := range maxExtendsFiles + 1 {
		fmt.Fprintf(&services, "  s%d:\n    extends: {file: {{DIR}}%s/base.yml, service: base}\n", i, strings.Repeat("/.", i))
	}
	spellings["compose.yml"] = "name: app\nservices:\n" + services.String()
	if _, err := parseProject(t, spellings); err == nil || !strings.Contains(err.Error(), "extends files") {
		t.Errorf("err = %v, want one file spelled too many ways refused", err)
	}
}

// Every service extending another takes in a copy of it, so what extends
// copies into a parse's model is bounded too, wherever the base is.
func TestParseComposeFileExtendsCopyBudget(t *testing.T) {
	const env = 2000 // a base of about as many YAML nodes
	var base strings.Builder
	base.WriteString("  base:\n    image: alpine\n    environment:\n")
	for i := range env {
		fmt.Fprintf(&base, "      - V%d=x\n", i)
	}
	project := func(mode string, services int) map[string]string {
		files := map[string]string{"base.yml": "services:\n" + base.String()}
		var b strings.Builder
		b.WriteString("name: app\n")
		if mode == "included base" {
			b.WriteString("include: [base.yml]\n")
		}
		b.WriteString("services:\n")
		if mode == "same file" {
			b.WriteString(base.String())
		}
		for i := range services {
			if mode == "other file" {
				fmt.Fprintf(&b, "  s%d:\n    extends: {file: {{DIR}}/base.yml, service: base}\n", i)
			} else {
				fmt.Fprintf(&b, "  s%d:\n    extends: base\n", i)
			}
		}
		files["compose.yml"] = b.String()
		return files
	}
	for _, mode := range []string{"same file", "other file", "included base"} {
		t.Run(mode, func(t *testing.T) {
			if _, err := parseProject(t, project(mode, 10)); err != nil {
				t.Errorf("parse error within the budget: %v", err)
			}
			if _, err := parseProject(t, project(mode, maxExtendsNodes/env+1)); err == nil || !strings.Contains(err.Error(), "extends copies") {
				t.Errorf("err = %v, want extends copying too much refused", err)
			}
		})
	}
}

// The guard checks an absolute extends file and has the loader read it
// where it is, with the paths in it relative to the file itself.
func TestExtendsFileGuardChecksAbsoluteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "base.yml")
	writeFiles(t, dir, map[string]string{"base.yml": "services:\n  base:\n    image: alpine\n"})
	guard := extendsFileGuard{files: newExtendsFiles(&composeBudget{})}
	if !guard.Accept(path) {
		t.Fatal("absolute extends file left to the local loader")
	}
	local, err := guard.Load(context.Background(), path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if local != path {
		t.Errorf("local = %q, want the checked file %q", local, path)
	}
	if got := guard.Dir(path); got != dir {
		t.Errorf("dir = %q, want %q", got, dir)
	}
}

// Errors about an extends file name the file.
func TestParseComposeFileExtendsErrorsNameTheFile(t *testing.T) {
	for name, c := range map[string]struct{ base, want string }{
		"no services":    {"x-a: 1\n", "no services section"},
		"does not parse": {"services: [\n", "failed to parse"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			base := filepath.Join(dir, "base.yml")
			writeFiles(t, dir, map[string]string{
				"base.yml":    c.base,
				"compose.yml": "name: app\nservices:\n  web:\n    extends: {file: " + base + ", service: base}\n",
			})
			_, err := parseComposeFile(context.Background(), filepath.Join(dir, "compose.yml"), "compose.yml")
			if err == nil || !strings.Contains(err.Error(), c.want) || !strings.Contains(err.Error(), base) || strings.Contains(err.Error(), "/proc/") {
				t.Errorf("err = %v, want %q naming %s", err, c.want, base)
			}
		})
	}
}

// panicChildEnv marks the test process TestParseComposeFileLoaderPanic runs
// its cases in.
const panicChildEnv = "DOCKGRAPH_TEST_PANIC_CHILD"

// A panic in the loader, as on some mistyped fields, fails the parse rather
// than ending the process, and leaves the file free to be parsed again. A
// panic not recovered would end the test binary, so the cases run in a
// process of their own.
func TestParseComposeFileLoaderPanic(t *testing.T) {
	if os.Getenv(panicChildEnv) == "" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestParseComposeFileLoaderPanic$", "-test.v")
		cmd.Env = append(os.Environ(), panicChildEnv+"=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cases failed: %v\n%s", err, out)
		}
		return
	}
	for name, c := range map[string]struct {
		files map[string]string
		want  string
	}{
		"extends file not a string": {map[string]string{
			"compose.yml": "name: app\nservices:\n  web:\n    extends: {file: 1, service: base}\n",
		}, "loading failed"},
		"extends file not a string in an extends file": {map[string]string{
			"base.yml":    "services:\n  base:\n    image: alpine\n    extends: {file: 1, service: x}\n",
			"compose.yml": "name: app\nservices:\n  web:\n    extends: {file: {{DIR}}/base.yml, service: base}\n",
		}, "loading failed"},
		"bind source not a string in an extends file": {map[string]string{
			"base.yml":    "services:\n  base:\n    image: alpine\n    volumes: [{type: bind, source: 1, target: /x}]\n",
			"compose.yml": "name: app\nservices:\n  web:\n    extends: {file: {{DIR}}/base.yml, service: base}\n",
		}, "loading failed"},
		// Validated before its extends are applied, this one fails plainly.
		"extends file not a string in a file using include": {map[string]string{
			"db.yml":      "services:\n  db:\n    image: postgres\n",
			"compose.yml": "name: app\ninclude: [db.yml]\nservices:\n  web:\n    extends: {file: 1, service: base}\n",
		}, "must be a string"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			files := make(map[string]string, len(c.files))
			for name, content := range c.files {
				files[name] = strings.ReplaceAll(content, "{{DIR}}", dir)
			}
			writeFiles(t, dir, files)
			path := filepath.Join(dir, "compose.yml")
			for range 2 {
				if _, err := parseComposeFile(context.Background(), path, "compose.yml"); err == nil || !strings.Contains(err.Error(), c.want) {
					t.Errorf("err = %v, want %q", err, c.want)
				}
			}
		})
	}
}

// A document whose name the loader does not read as a string keeps the
// file's includes, as in compose.
func TestParseComposeFileIncludeWithUnreadName(t *testing.T) {
	snap, err := parseProject(t, map[string]string{
		"db.yml":      "services:\n  db:\n    image: postgres\n",
		"compose.yml": "name: app\ninclude: [db.yml]\nservices:\n  web:\n    image: nginx\n---\nname: !reset {}\n",
	})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if got := composeServices(snap); !slices.Equal(got, []string{"db", "web"}) {
		t.Errorf("services = %v, want the included db too", got)
	}
}

// A document's includes come into that document, as in compose: they win
// over the documents before it and lose to it and those after it.
func TestParseComposeFileIncludesInLaterDocuments(t *testing.T) {
	snap, err := parseProject(t, map[string]string{
		"a.yml": "services:\n  db:\n    image: from-a\n    environment: {A: \"1\"}\n",
		"b.yml": "services:\n  db:\n    image: from-b\n    environment: {B: \"1\"}\n  cache:\n    image: from-b\n",
		"compose.yml": "name: app\ninclude: [a.yml]\nservices:\n  db:\n    environment: {D1: \"1\"}\n" +
			"---\n# second document\ninclude: [b.yml]\nservices:\n  cache:\n    image: from-doc2\n" +
			"---\nservices:\n  db:\n    environment: {D3: \"1\"}\n",
	})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	db := composeNode(t, snap, "db")
	if db.Image != "from-b" {
		t.Errorf("db image = %q, want from-b, the later document's include", db.Image)
	}
	if want := map[string]string{"A": "1", "B": "1", "D1": "1", "D3": "1"}; !maps.Equal(db.Compose.Environment, want) {
		t.Errorf("db environment = %v, want %v", db.Compose.Environment, want)
	}
	if got := composeNode(t, snap, "cache").Image; got != "from-doc2" {
		t.Errorf("cache image = %q, want from-doc2, the document over its include", got)
	}
}

// An extends file's own extends files and bind mounts are relative to it,
// as in compose.
func TestParseComposeFileNestedRelativeExtends(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"base/x.yml":  "services:\n  x:\n    extends: {file: y.yml, service: y}\n    volumes: [\"./data:/data\"]\n",
		"base/y.yml":  "services:\n  y:\n    image: alpine\n    volumes: [\"./cache:/cache\"]\n",
		"compose.yml": "name: app\nservices:\n  web:\n    extends: {file: " + filepath.Join(dir, "base", "x.yml") + ", service: x}\n",
	})
	snap, err := parseComposeFile(context.Background(), filepath.Join(dir, "compose.yml"), "compose.yml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	web := composeNode(t, snap, "web")
	if web.Image != "alpine" {
		t.Errorf("web image = %q, want alpine from base/y.yml", web.Image)
	}
	var sources []string
	for _, v := range web.Compose.Volumes {
		sources = append(sources, v.Source)
	}
	slices.Sort(sources)
	if want := []string{filepath.Join(dir, "base", "cache"), filepath.Join(dir, "base", "data")}; !slices.Equal(sources, want) {
		t.Errorf("web bind sources = %v, want %v", sources, want)
	}
}

// An include cycle running through an include's override file, the
// including file listed among them, is an error too.
func TestParseComposeFileIncludeCycleThroughOverrideFile(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"own include": {
			"b.yml":       "services:\n  b:\n    image: alpine\n",
			"compose.yml": "name: app\ninclude:\n  - path: [b.yml, compose.yml]\nservices:\n  web:\n    image: nginx\n",
		},
		"nested include": {
			"sub/b.yml":   "services:\n  b:\n    image: alpine\n",
			"sub/x.yml":   "include:\n  - path: [b.yml, x.yml]\nservices:\n  x:\n    image: alpine\n",
			"compose.yml": "name: app\ninclude: [sub/x.yml]\nservices:\n  web:\n    image: nginx\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseProject(t, files)
			if err == nil || !strings.Contains(err.Error(), "cycle") {
				t.Errorf("err = %v, want an include cycle", err)
			}
		})
	}
}

// includeChain returns a compose project whose file includes f1.yml, which
// includes f2.yml, and so on, depth files deep.
func includeChain(depth int) map[string]string {
	files := map[string]string{"compose.yml": "name: app\ninclude: [f1.yml]\nservices:\n  web:\n    image: nginx\n"}
	for i := 1; i <= depth; i++ {
		include := ""
		if i < depth {
			include = fmt.Sprintf("include: [f%d.yml]\n", i+1)
		}
		files[fmt.Sprintf("f%d.yml", i)] = include + fmt.Sprintf("services:\n  s%d:\n    image: alpine\n", i)
	}
	return files
}

// Includes nest up to maxIncludeDepth files deep.
func TestParseComposeFileIncludeDepthLimit(t *testing.T) {
	snap, err := parseProject(t, includeChain(maxIncludeDepth))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if got := len(composeServices(snap)); got != maxIncludeDepth+1 {
		t.Errorf("%d services, want %d", got, maxIncludeDepth+1)
	}
	_, err = parseProject(t, includeChain(maxIncludeDepth+1))
	if err == nil || !strings.Contains(err.Error(), "nested") {
		t.Errorf("err = %v, want includes nested too deep refused", err)
	}
}

// A file reachable through several include paths is loaded once per parse:
// a diamond of includes, each level's two files including both of the
// next, has 2^(depth+1) paths through it, far more than the included files
// a parse may load.
func TestParseComposeFileIncludeDiamond(t *testing.T) {
	const depth = maxIncludeDepth
	files := map[string]string{"compose.yml": "name: app\ninclude: [a1.yml, b1.yml]\nservices:\n  web:\n    image: nginx\n"}
	for i := 1; i <= depth; i++ {
		include := ""
		if i < depth {
			include = fmt.Sprintf("include: [a%d.yml, b%d.yml]\n", i+1, i+1)
		}
		for _, side := range []string{"a", "b"} {
			files[fmt.Sprintf("%s%d.yml", side, i)] = include + fmt.Sprintf("services:\n  %s%d:\n    image: alpine\n", side, i)
		}
	}
	snap, err := parseProject(t, files)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if got := len(composeServices(snap)); got != 2*depth+1 {
		t.Errorf("%d services, want %d", got, 2*depth+1)
	}
}

// The included files of one parse are bounded in number, an include of a
// file loaded before counting too, and in total size.
func TestParseComposeFileIncludeBudget(t *testing.T) {
	includeDB := func(times int) map[string]string {
		paths := strings.TrimSuffix(strings.Repeat("db.yml, ", times), ", ")
		return map[string]string{
			"db.yml":      "services:\n  db:\n    image: postgres\n",
			"compose.yml": "name: app\ninclude: [" + paths + "]\nservices:\n  web:\n    image: nginx\n",
		}
	}
	if _, err := parseProject(t, includeDB(maxIncludedFiles)); err != nil {
		t.Errorf("parse error within the budget: %v", err)
	}
	if _, err := parseProject(t, includeDB(maxIncludedFiles+1)); err == nil || !strings.Contains(err.Error(), "included files") {
		t.Errorf("err = %v, want too many included files refused", err)
	}

	padding := "# " + strings.Repeat("x", maxComposeFileSize/2) + "\n"
	_, err := parseProject(t, map[string]string{
		"a.yml":       "services:\n  a:\n    image: alpine\n" + padding,
		"b.yml":       "services:\n  b:\n    image: alpine\n" + padding,
		"compose.yml": "name: app\ninclude: [a.yml, b.yml]\nservices:\n  web:\n    image: nginx\n",
	})
	if err == nil || !strings.Contains(err.Error(), "in total") {
		t.Errorf("err = %v, want included files too large in total refused", err)
	}
}

// Loading includes stops once the parse's context is done.
func TestParseComposeFileIncludeStopsWithContext(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"db.yml":      "services:\n  db:\n    image: postgres\n",
		"compose.yml": "name: app\ninclude: [db.yml]\nservices:\n  web:\n    image: nginx\n",
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := parseComposeFile(ctx, filepath.Join(dir, "compose.yml"), "compose.yml"); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want the parse canceled", err)
	}
}

// A file using include is held to the loader's limit on the YAML nodes of a
// document, as a file that does not use include is.
func TestParseComposeFileIncludeNodeLimit(t *testing.T) {
	big := "x-big: [" + strings.Repeat("a,", 150_000) + "a]\n"
	for name, include := range map[string]string{"without include": "", "with include": "include: [db.yml]\n"} {
		t.Run(name, func(t *testing.T) {
			_, err := parseProject(t, map[string]string{
				"db.yml":      "services:\n  db:\n    image: postgres\n",
				"compose.yml": "name: app\n" + include + "services:\n  web:\n    image: nginx\n" + big,
			})
			if err == nil || !strings.Contains(err.Error(), "node visit limit") {
				t.Errorf("err = %v, want the node limit exceeded", err)
			}
		})
	}
}

// The including file's !reset and !override tags apply, as in compose, to
// the included definitions it overrides and to its own earlier documents.
func TestParseComposeFileIncludeResetAndOverride(t *testing.T) {
	snap, err := parseProject(t, map[string]string{
		"db.yml": "services:\n  db:\n    image: postgres\n    ports: [\"5432:5432\"]\n" +
			"    environment: {A: \"1\", B: \"2\"}\n    labels: {tier: data}\n",
		"compose.yml": "name: app\ninclude: [db.yml]\nservices:\n  web:\n    image: nginx\n    ports: [\"80:80\"]\n" +
			"  db:\n    ports: !reset []\n    environment: !override {C: \"3\"}\n" +
			"---\nservices:\n  web:\n    ports: !reset []\n",
	})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	db := composeNode(t, snap, "db")
	if len(db.Ports) != 0 {
		t.Errorf("db ports = %v, want them reset", db.Ports)
	}
	if want := map[string]string{"C": "3"}; !maps.Equal(db.Compose.Environment, want) {
		t.Errorf("db environment = %v, want %v", db.Compose.Environment, want)
	}
	if db.Compose.Labels["tier"] != "data" {
		t.Errorf("db labels = %v, want the included ones", db.Compose.Labels)
	}
	if web := composeNode(t, snap, "web"); len(web.Ports) != 0 {
		t.Errorf("web ports = %v, want them reset by the second document", web.Ports)
	}
}

// An included file's COMPOSE_PROJECT_NAME is the including project's name,
// as in compose.
func TestParseComposeFileIncludedProjectName(t *testing.T) {
	snap, err := parseProject(t, map[string]string{
		"sub/db.yml": "services:\n  db:\n    image: postgres\n" +
			"    labels: {project: \"${COMPOSE_PROJECT_NAME}\"}\n    environment: [COMPOSE_PROJECT_NAME]\n",
		"compose.yml": "name: app\ninclude: [sub/db.yml]\nservices:\n  web:\n    image: nginx\n",
	})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	db := composeNode(t, snap, "db").Compose
	if db.Labels["project"] != "app" || db.Environment["COMPOSE_PROJECT_NAME"] != "app" {
		t.Errorf("db labels %v, environment %v: want the project name app", db.Labels, db.Environment)
	}
}

// A file that names a missing include is reported, as compose reports it.
func TestParseComposeFileMissingInclude(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"compose.yml": "name: app\ninclude: [other.yml]\nservices:\n  web:\n    image: nginx\n",
	})
	_, err := parseComposeFile(context.Background(), filepath.Join(dir, "compose.yml"), "compose.yml")
	if err == nil || !strings.Contains(err.Error(), "other.yml") {
		t.Errorf("err = %v, want one naming the missing include", err)
	}
}

// An env_file path is relative to the host, where DockGraph usually can't
// see it; that must not keep the file's services off the graph.
func TestParseComposeFileWithUnreachableEnvFile(t *testing.T) {
	compose := filepath.Join(t.TempDir(), "compose.yml")
	content := "name: app\nservices:\n  web:\n    image: nginx\n    env_file: [./does-not-exist.env]\n"
	if err := os.WriteFile(compose, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := parseComposeFile(context.Background(), compose, "compose.yml"); err != nil {
		t.Fatalf("parse error: %v", err)
	}
}

func TestBuildComposeConfigMasksEmbeddedCredentials(t *testing.T) {
	dbURL := "postgres://app:Pg-Secret@db:5432/app"
	svc := composetypes.ServiceConfig{
		Name: "api",
		ContainerSpec: composetypes.ContainerSpec{
			Environment: composetypes.MappingWithEquals{"DATABASE_URL": &dbURL},
			Command:     composetypes.ShellCommand{"app", "--api-token", "tok-123"},
			Entrypoint:  composetypes.ShellCommand{"sh", "-c", "PGPASSWORD=pw exec app"},
		},
	}

	cfg := buildComposeConfig(svc, composeNaming{project: "proj"})

	if got := cfg.Environment["DATABASE_URL"]; got != "postgres://app:********@db:5432/app" {
		t.Errorf("DATABASE_URL = %q", got)
	}
	if got := strings.Join(cfg.Command, " "); got != "app --api-token ********" {
		t.Errorf("command = %q", got)
	}
	if got := cfg.Entrypoint[2]; got != "PGPASSWORD=******** exec app" {
		t.Errorf("entrypoint = %q", got)
	}
}

// The command follows the entrypoint on the container's command line, so
// the value of a flag ending the entrypoint is the command's first word.
func TestBuildComposeConfigMasksCommandAfterEntrypoint(t *testing.T) {
	svc := composetypes.ServiceConfig{
		Name: "cache",
		ContainerSpec: composetypes.ContainerSpec{
			Entrypoint: composetypes.ShellCommand{"redis-server", "--requirepass"},
			Command:    composetypes.ShellCommand{"ComposeSecret1", "--port", "6379"},
		},
	}
	cfg := buildComposeConfig(svc, composeNaming{project: "proj"})
	if got := strings.Join(cfg.Command, " "); got != "******** --port 6379" {
		t.Errorf("command = %q, want the value after the entrypoint's flag masked", got)
	}
}

func TestParseComposePortsProtocol(t *testing.T) {
	ports := []composetypes.ServicePortConfig{
		{Published: "8070", Target: 80}, // unset → defaults to tcp
		{Published: "5353", Target: 53, Protocol: "udp"},
	}

	// Sorted by host port.
	got := parseComposePorts(ports)
	if len(got) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(got))
	}
	if got[0].Host != 5353 || got[0].Protocol != "udp" {
		t.Errorf("expected 5353 with protocol udp, got %+v", got[0])
	}
	if got[1].Host != 8070 || got[1].Protocol != "tcp" {
		t.Errorf("expected 8070 with default protocol tcp, got %+v", got[1])
	}
}

func TestParseVolumeMountEdges(t *testing.T) {
	snap, err := parseComposeFile(context.Background(), filepath.Join(testdataDir(), "simple.yaml"), "simple.yaml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	mounts := filterEdges(snap.Edges, "volume_mount")
	if len(mounts) != 1 {
		t.Errorf("expected 1 volume_mount edge, got %d", len(mounts))
	}
	if len(mounts) > 0 && mounts[0].MountPath != "/var/lib/postgresql/data" {
		t.Errorf("expected mount path /var/lib/postgresql/data, got %s", mounts[0].MountPath)
	}
}

func TestComposeVolumeStatus(t *testing.T) {
	snap, err := parseComposeFile(context.Background(), filepath.Join(testdataDir(), "simple.yaml"), "simple.yaml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	volumes := filterNodes(snap.Nodes, "volume")
	if len(volumes) == 0 {
		t.Fatal("expected at least one volume")
	}
	for _, v := range volumes {
		if v.Status != "not_running" {
			t.Errorf("expected compose volume %s to have status not_running, got %s", v.Name, v.Status)
		}
	}
}

func TestParseComposePortsSingle(t *testing.T) {
	ports := parseComposePorts([]composetypes.ServicePortConfig{
		{Published: "8080", Target: 80},
	})
	if len(ports) != 1 || ports[0].Host != 8080 || ports[0].Container != 80 {
		t.Errorf("expected [{8080 80}], got %+v", ports)
	}
}

// A published range for one target port ("9000-9002:9000") is Docker
// choosing one host port of the range for it: one mapping, not one per port.
func TestParseComposePortsHostRange(t *testing.T) {
	ports := parseComposePorts([]composetypes.ServicePortConfig{
		{Published: "9000-9002", Target: 9000},
	})
	want := PortMapping{Host: 9000, HostEnd: 9002, Container: 9000, Protocol: "tcp"}
	if len(ports) != 1 || ports[0] != want {
		t.Errorf("ports = %+v, want [%+v]", ports, want)
	}
}

// A range of ports, which the loader lists port by port, is one mapping.
func TestParseComposePortsRange(t *testing.T) {
	snap, err := parseProject(t, map[string]string{
		"compose.yml": "name: app\nservices:\n  turn:\n    image: coturn\n" +
			"    ports: [\"3478:3478/udp\", \"49152-65535:49152-65535/udp\", \"49152-65535:49152-65535\"]\n",
	})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	want := []PortMapping{
		{Host: 3478, Container: 3478, Protocol: "udp"},
		{Host: 49152, HostEnd: 65535, Container: 49152, ContainerEnd: 65535, Protocol: "tcp"},
		{Host: 49152, HostEnd: 65535, Container: 49152, ContainerEnd: 65535, Protocol: "udp"},
	}
	if got := composeNode(t, snap, "turn").Ports; !slices.Equal(got, want) {
		t.Errorf("ports = %+v, want %+v", got, want)
	}
}

func TestParseComposePortsUnpublishedSkipped(t *testing.T) {
	ports := parseComposePorts([]composetypes.ServicePortConfig{
		{Published: "", Target: 3000},
	})
	if len(ports) != 0 {
		t.Errorf("expected no mappings for unpublished port, got %+v", ports)
	}
}

func TestParseComposePortsInvalidSkipped(t *testing.T) {
	ports := parseComposePorts([]composetypes.ServicePortConfig{
		{Published: "notanumber", Target: 80},
	})
	if len(ports) != 0 {
		t.Errorf("expected no mappings for unparseable port, got %+v", ports)
	}
}

func filterNodes(nodes []Node, nodeType string) []Node {
	var filtered []Node
	for _, n := range nodes {
		if n.Type == nodeType {
			filtered = append(filtered, n)
		}
	}
	return filtered
}

func filterEdges(edges []Edge, edgeType string) []Edge {
	var filtered []Edge
	for _, e := range edges {
		if e.Type == edgeType {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func TestStackServiceInfoDesiredReplicas(t *testing.T) {
	three := 3
	tests := []struct {
		name   string
		deploy *composetypes.DeployConfig
		mode   string
		want   int
	}{
		{"no deploy", nil, serviceModeReplicated, 1},
		{"replicated", &composetypes.DeployConfig{Replicas: &three}, serviceModeReplicated, 3},
		{"replicated job", &composetypes.DeployConfig{Mode: serviceModeReplicatedJob, Replicas: &three}, serviceModeReplicatedJob, 3},
		{"replicated job default", &composetypes.DeployConfig{Mode: serviceModeReplicatedJob}, serviceModeReplicatedJob, 1},
		// The manager computes these per eligible node: no count yet.
		{"global", &composetypes.DeployConfig{Mode: serviceModeGlobal}, serviceModeGlobal, 0},
		{"global job", &composetypes.DeployConfig{Mode: serviceModeGlobalJob}, serviceModeGlobalJob, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := stackServiceInfo(composetypes.ServiceConfig{Name: "s", Deploy: tt.deploy})
			if info.Mode != tt.mode || info.Replicas.Desired != tt.want {
				t.Errorf("mode %q desired %d, want %q %d", info.Mode, info.Replicas.Desired, tt.mode, tt.want)
			}
		})
	}
}
