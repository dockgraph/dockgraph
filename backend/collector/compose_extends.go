package collector

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"

	"go.yaml.in/yaml/v4"
)

// loaderPath returns the path the loader opens for an extends file named
// path, relative to workingDir: an absolute path as it is, a relative one
// joined to workingDir, and so cleaned (localResourceLoader.abs in
// compose-go's loader/loader.go). The system resolves symlinks before "..",
// so only that path names the file the loader reads.
func loaderPath(workingDir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(workingDir, path)
}

// extendsFiles holds the extends files of one parse, each checked once by
// the path the loader opens it by, against the limits of a compose file
// and, together, maxExtendsFiles.
type extendsFiles struct {
	budget *composeBudget
	files  map[string]*extendsFile // by the path the loader opens
	read   int                     // files read
}

// extendsFile is an extends file as checked.
type extendsFile struct {
	nodes int   // the most one use of it copies (composeHeader.largestService)
	err   error // why it is refused
}

func newExtendsFiles(budget *composeBudget) *extendsFiles {
	return &extendsFiles{
		budget: budget,
		files:  make(map[string]*extendsFile),
	}
}

// use counts a use of the extends file at path, the path the loader opens,
// against the parse's bounds.
func (e *extendsFiles) use(path string) error {
	f := e.file(path)
	if f.err != nil {
		return f.err
	}
	return e.budget.extend(path, f.nodes)
}

// file returns the extends file at path, checking it the first time.
func (e *extendsFiles) file(path string) *extendsFile {
	f, ok := e.files[path]
	if !ok {
		f = e.load(path)
		e.files[path] = f
	}
	return f
}

// load reads and checks the extends file at path before the loader does.
func (e *extendsFiles) load(path string) *extendsFile {
	if e.read == maxExtendsFiles {
		return &extendsFile{err: fmt.Errorf("%s: more than %d extends files", path, maxExtendsFiles)}
	}
	data, err := e.budget.read(path)
	if err != nil {
		return &extendsFile{err: err}
	}
	e.read++
	// A file that does not decode is the loader's to report.
	header, err := decodeComposeHeader(data)
	if errors.Is(err, errTooManyDocuments) {
		return &extendsFile{err: fmt.Errorf("%s: %w", path, err)}
	}
	return &extendsFile{nodes: header.largestService}
}

// extendsFileGuard is a loader.ResourceLoader, asked before the local one,
// that checks each extends file against the parse's bounds before the
// loader reads it, so a refused file fails with an error saying why. The
// loader then reads the file itself, so a file changed after the check is
// read as it is by then; the process the parse runs in bounds what that can
// cost (see restrictComposeParse). A relative extends file is claimed only
// to refuse it: the loader also asks whether a relative extends file named
// in an extends file is remote, and claiming it would keep it from being
// resolved against the file that names it.
type extendsFileGuard struct {
	workingDir string        // the load's; relative extends files are relative to it
	files      *extendsFiles // the parse's
}

// Accept implements loader.ResourceLoader.
func (g extendsFileGuard) Accept(path string) bool {
	if filepath.IsAbs(path) {
		return true
	}
	err := g.files.use(loaderPath(g.workingDir, path))
	// A missing file is left for the local loader to report.
	return err != nil && !errors.Is(err, fs.ErrNotExist)
}

// Load implements loader.ResourceLoader, returning the path the loader
// reads an absolute extends file by, and failing for a refused one.
func (g extendsFileGuard) Load(_ context.Context, path string) (string, error) {
	opened := loaderPath(g.workingDir, path)
	err := g.files.use(opened)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", err // as the local loader reports it
	case err != nil:
		return "", fmt.Errorf("extends: %w", err)
	case !filepath.IsAbs(path):
		return "", fmt.Errorf("extends: %s: refused", opened)
	}
	return opened, nil
}

// Dir implements loader.ResourceLoader: the directory of the extends file
// itself, which the paths in it are relative to.
func (g extendsFileGuard) Dir(path string) string {
	return filepath.Dir(loaderPath(g.workingDir, path))
}

// serviceSections are the sections of a compose file whose entries can
// extend one another.
var serviceSections = []string{"services", "jobs"}

// sectionServices are the entries of a service section, sized in YAML
// nodes, each with the entry it extends in the same file.
type sectionServices struct {
	size map[string]int
	base map[string]string
}

func newSectionServices() *sectionServices {
	return &sectionServices{size: make(map[string]int), base: make(map[string]string)}
}

// add adds the entries of o, merged over s's as the loader merges
// documents.
func (s *sectionServices) add(o *sectionServices) {
	for name, size := range o.size {
		s.size[name] += size
	}
	maps.Copy(s.base, o.base)
}

// extendsCopies returns the nodes extends between s's entries copies, each
// entry extending another taking in a copy of it with its own extends
// applied, and the largest of the entries so resolved.
func (s *sectionServices) extendsCopies() (copied, largest int) {
	resolved := make(map[string]int, len(s.size))
	var resolve func(name string, depth int) int
	resolve = func(name string, depth int) int {
		if n, ok := resolved[name]; ok {
			return n
		}
		n := s.size[name]
		// A cycle is the loader's to report; depth ends the walk round it.
		if base, ok := s.base[name]; ok && depth < len(s.size) {
			n += resolve(base, depth+1)
		}
		resolved[name] = n
		return n
	}
	for name := range s.size {
		largest = max(largest, resolve(name, 0))
	}
	for _, base := range s.base {
		copied += resolve(base, 0)
	}
	return copied, largest
}

// documentServices returns the service sections of a compose file
// document.
func documentServices(doc *yaml.Node) map[string]*sectionServices {
	var sections struct {
		Services map[string]yaml.Node `yaml:"services"`
		Jobs     map[string]yaml.Node `yaml:"jobs"`
	}
	if doc.Decode(&sections) != nil {
		return nil // the loader reports sections that are not mappings
	}
	counted := make(map[*yaml.Node]int)
	result := make(map[string]*sectionServices, len(serviceSections))
	for section, entries := range map[string]map[string]yaml.Node{"services": sections.Services, "jobs": sections.Jobs} {
		services := newSectionServices()
		for name, def := range entries {
			services.size[name] = nodeCount(&def, counted)
			if base := extendsBase(&def); base != "" {
				services.base[name] = base
			}
		}
		result[section] = services
	}
	return result
}

// nodeCount returns the YAML nodes n stands for with its aliases expanded,
// as the loader decodes it. counted holds the counts of the nodes counted
// before, so that each is walked once.
func nodeCount(n *yaml.Node, counted map[*yaml.Node]int) int {
	if n.Kind == yaml.AliasNode {
		if n.Alias == nil {
			return 1
		}
		return nodeCount(n.Alias, counted)
	}
	if c, ok := counted[n]; ok {
		return c
	}
	counted[n] = 1 // a node within its own alias counts once
	c := 1
	for _, child := range n.Content {
		c += nodeCount(child, counted)
	}
	counted[n] = c
	return c
}

// extendsBase returns the entry a service definition extends in the same
// file: none when it extends none, or one in another file.
func extendsBase(def *yaml.Node) string {
	var fields struct {
		Extends yaml.Node `yaml:"extends"`
	}
	if def.Decode(&fields) != nil {
		return ""
	}
	extends := &fields.Extends
	if extends.Kind == yaml.AliasNode && extends.Alias != nil {
		extends = extends.Alias
	}
	switch extends.Kind {
	case yaml.ScalarNode:
		return extends.Value
	case yaml.MappingNode:
		var ref struct {
			File    yaml.Node `yaml:"file"`
			Service string    `yaml:"service"`
		}
		if extends.Decode(&ref) != nil || ref.File.Kind != 0 {
			return ""
		}
		return ref.Service
	}
	return ""
}

// modelExtendsCopies returns the nodes extends between the services of a
// model copies (see sectionServices.extendsCopies).
func modelExtendsCopies(model map[string]any) int {
	total := 0
	for _, section := range serviceSections {
		entries, _ := model[section].(map[string]any)
		services := newSectionServices()
		for name, def := range entries {
			services.size[name] = modelNodes(def)
			if base := modelExtendsBase(def); base != "" {
				services.base[name] = base
			}
		}
		copied, _ := services.extendsCopies()
		total += copied
	}
	return total
}

// modelNodes returns the YAML nodes v, a value of a model, stands for.
func modelNodes(v any) int {
	n := 1
	switch v := v.(type) {
	case map[string]any:
		for _, e := range v {
			n += 1 + modelNodes(e)
		}
	case []any:
		for _, e := range v {
			n += modelNodes(e)
		}
	}
	return n
}

// modelExtendsBase returns the service a service of a model extends in the
// same file (see extendsBase).
func modelExtendsBase(def any) string {
	service, _ := def.(map[string]any)
	switch extends := service["extends"].(type) {
	case string:
		return extends
	case map[string]any:
		if _, inFile := extends["file"]; inFile {
			return ""
		}
		base, _ := extends["service"].(string)
		return base
	}
	return ""
}
