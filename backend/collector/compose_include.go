package collector

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/compose-spec/compose-go/v2/consts"
	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/override"
	"github.com/compose-spec/compose-go/v2/template"
	"github.com/compose-spec/compose-go/v2/tree"
	composetypes "github.com/compose-spec/compose-go/v2/types"
	"go.yaml.in/yaml/v4"
)

// composeMainFile returns data, the compose file at path, as a file for the
// loader, and the project's name as the loader would give it, if the file
// decodes. A file using include comes with its model, the included files
// seeing the project's name too (see includeLoader.model): the loader's own
// include would read an include's env_file, or an included project's .env,
// from DockGraph's filesystem. budget and extends are the parse's.
func composeMainFile(ctx context.Context, path string, data []byte, opts composeParseOptions, budget *composeBudget, extends *extendsFiles) (composetypes.ConfigFile, string, error) {
	file := composetypes.ConfigFile{Filename: path, Content: data}
	header, decoded, err := composeHeaders(path, data)
	if !decoded {
		return file, "", err
	}
	name, err := composeProjectName(path, header, opts)
	if err != nil {
		return file, "", nil // the loader, reading the name itself, reports it
	}
	if !header.usesInclude() {
		return file, name, budget.extend(path, header.extendsNodes)
	}
	includes := &includeLoader{
		project: name,
		loaded:  make(map[string]map[string]any),
		budget:  budget,
		extends: extends,
	}
	// Include paths and the rest of the file's paths are relative to it.
	dir := filepath.Dir(path)
	file.Config, err = includes.model(ctx, path, data, header, dir, nil)
	return file, name, err
}

// includeLoader loads the projects a compose file includes, and those they
// include, as the loader's own include does, but from an empty environment
// holding only the including project's name: neither an include's env_file
// nor the .env of an included project is read. It keeps the projects one
// parse has loaded, so that a file reachable through several include paths
// is loaded once, and counts what the parse has included against the
// include limits.
type includeLoader struct {
	project string                    // the including project's name
	loaded  map[string]map[string]any // included projects' models by paths and project directory
	files   int                       // included files processed
	budget  *composeBudget            // the parse's
	extends *extendsFiles             // the parse's extends files
}

// options returns the loader options of a model that becomes part of
// another file: one of an included project, or a file using include.
func (l *includeLoader) options(workingDir string) func(*loader.Options) {
	return func(o *loader.Options) {
		baseLoadOptions(workingDir, l.extends)(o)
		o.SkipNormalization = true
		o.SkipConsistencyCheck = true
		o.SetProjectName(l.project, true)
	}
}

// model returns the model of data, the compose file at path, with the
// resources of the projects it includes, as the loader's own include adds
// them: each document's own definitions are merged over an included one of
// the same name. Its paths, include paths too, are relative to dir; chain
// lists the files that include it.
func (l *includeLoader) model(ctx context.Context, path string, data []byte, header composeHeader, dir string, chain []string) (map[string]any, error) {
	chain = append(slices.Clip(chain), path)
	// imports[k] holds the resources document k's includes bring in.
	imports := make([]map[string]any, len(header.documents))
	for k, doc := range header.documents {
		if doc.include == nil {
			continue
		}
		includes, err := l.includeConfigs(doc.include)
		if err != nil {
			return nil, err
		}
		imports[k] = make(map[string]any)
		for _, inc := range includes {
			project, err := l.load(ctx, inc, dir, chain)
			if err != nil {
				return nil, err
			}
			if err := importResources(imports[k], project); err != nil {
				return nil, err
			}
		}
	}
	files, err := documentFiles(path, data, header, imports)
	if err != nil {
		return nil, err
	}
	// Loading the documents after the resources their includes bring in
	// holds each to the loader's node limit and lets its !reset and
	// !override tags act on those resources. Extends, defaults and relative
	// paths are left for the load of the including file, where extends can
	// use included services; resolving them here too would count extends
	// files twice against the parse's bounds.
	model, err := loader.LoadModelWithContext(ctx, composetypes.ConfigDetails{
		WorkingDir:  dir,
		ConfigFiles: files,
	}, l.options(dir), func(o *loader.Options) {
		o.SkipExtends = true
		o.SkipDefaultValues = true
		o.ResolvePaths = false
	})
	if err != nil {
		return nil, err
	}
	delete(model, "include")
	if err := l.budget.extend(path, modelExtendsCopies(model)); err != nil {
		return nil, err
	}
	// That load interpolates the model once more.
	return escapeInterpolation(model), nil
}

// documentFiles returns data, the compose file at path, as files for the
// loader, with imports[k], the resources document k's includes bring in,
// put before document k: as with the loader's own include, they win over
// the documents before it, and the document and those after it over them.
func documentFiles(path string, data []byte, header composeHeader, imports []map[string]any) ([]composetypes.ConfigFile, error) {
	resources := func(m map[string]any) composetypes.ConfigFile {
		return composetypes.ConfigFile{Filename: path, Config: escapeInterpolation(m)}
	}
	whole := composetypes.ConfigFile{Filename: path, Content: data}
	if !slices.ContainsFunc(imports[1:], func(m map[string]any) bool { return m != nil }) {
		if imports[0] == nil {
			return []composetypes.ConfigFile{whole}, nil
		}
		return []composetypes.ConfigFile{resources(imports[0]), whole}, nil
	}
	documents, ok := splitDocuments(data, header.documents)
	if !ok {
		// The documents cannot be told apart: all the resources come first,
		// those of earlier documents winning.
		all := make(map[string]any)
		for _, m := range imports {
			if err := importResources(all, m); err != nil {
				return nil, err
			}
		}
		return []composetypes.ConfigFile{resources(all), whole}, nil
	}
	var files []composetypes.ConfigFile
	for k, document := range documents {
		if imports[k] != nil {
			files = append(files, resources(imports[k]))
		}
		files = append(files, composetypes.ConfigFile{Filename: path, Content: document})
	}
	return files, nil
}

// splitDocuments splits data, the content of a compose file, into its
// documents, which start on the lines of documents. Each is preceded by as
// many empty lines as the file has before it, so that errors give lines of
// the file. ok is false if a piece does not decode as just one document.
func splitDocuments(data []byte, documents []composeDocument) (pieces [][]byte, ok bool) {
	starts := make([]int, len(documents)+1)
	starts[len(documents)] = len(data)
	for k := 1; k < len(documents); k++ {
		starts[k] = lineOffset(data, documents[k].line)
		if starts[k] < starts[k-1] {
			return nil, false
		}
	}
	pieces = make([][]byte, len(documents))
	for k := range documents {
		padding := 0
		if k > 0 {
			padding = documents[k].line - 1
		}
		piece := append(bytes.Repeat([]byte("\n"), padding), data[starts[k]:starts[k+1]]...)
		if !oneDocument(piece) {
			return nil, false
		}
		pieces[k] = piece
	}
	return pieces, true
}

// lineOffset returns the offset in data of the start of line, counted from
// one.
func lineOffset(data []byte, line int) int {
	offset := 0
	for range line - 1 {
		i := bytes.IndexByte(data[offset:], '\n')
		if i < 0 {
			return len(data)
		}
		offset += i + 1
	}
	return offset
}

// oneDocument reports whether data decodes as exactly one YAML document.
func oneDocument(data []byte) bool {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if decoder.Decode(&doc) != nil {
		return false
	}
	return errors.Is(decoder.Decode(&doc), io.EOF)
}

// includeConfig is an entry of a compose file's include: the files of the
// included project, the first being its main file, and its project
// directory. Its env_file is deliberately not kept.
type includeConfig struct {
	paths      []string
	projectDir string
}

// includeConfigs reads include entries, short ("- other.yml") or long
// ("- path: …"), interpolating their paths as the loader interpolates the
// rest of the file.
func (l *includeLoader) includeConfigs(raw any) ([]includeConfig, error) {
	entries, ok := raw.([]any)
	if !ok {
		return nil, errors.New("include must be a list")
	}
	configs := make([]includeConfig, 0, len(entries))
	for _, entry := range entries {
		var inc includeConfig
		switch entry := entry.(type) {
		case string:
			inc.paths = []string{entry}
		case map[string]any:
			switch path := entry["path"].(type) {
			case string:
				inc.paths = []string{path}
			case []any:
				for _, p := range path {
					if p, ok := p.(string); ok {
						inc.paths = append(inc.paths, p)
					}
				}
			}
			inc.projectDir, _ = entry["project_directory"].(string)
		}
		if len(inc.paths) == 0 {
			return nil, errors.New("include entry without a path")
		}
		var err error
		for i, p := range inc.paths {
			if inc.paths[i], err = l.interpolate(p); err != nil {
				return nil, fmt.Errorf("include: %w", err)
			}
		}
		if inc.projectDir, err = l.interpolate(inc.projectDir); err != nil {
			return nil, fmt.Errorf("include: %w", err)
		}
		configs = append(configs, inc)
	}
	return configs, nil
}

// interpolate interpolates s as the loader interpolates the files of the
// project here: from an empty environment, but for the project's name.
func (l *includeLoader) interpolate(s string) (string, error) {
	return template.Substitute(s, func(name string) (string, bool) {
		if name == consts.ComposeProjectName {
			return l.project, true
		}
		return "", false
	})
}

// load loads the project of an include entry as the loader loads an
// included one: relative to its project directory (by default its main
// file's), with its own includes imported, and neither normalized nor
// checked for consistency, which happens once its resources are part of
// the including file. Include paths are relative to dir; chain lists the
// files that include the project. A project the parse loaded before is
// reused.
func (l *includeLoader) load(ctx context.Context, inc includeConfig, dir string, chain []string) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("include: %w", err)
	}
	paths := make([]string, len(inc.paths))
	for i, p := range inc.paths {
		paths[i] = resolvePath(dir, p)
		// An including file listed among an include's override files
		// would include itself as well.
		if slices.Contains(chain, paths[i]) {
			return nil, fmt.Errorf("include cycle detected: %s includes %s", strings.Join(chain, " includes "), paths[i])
		}
	}
	if len(chain) > maxIncludeDepth {
		return nil, fmt.Errorf("include: %s: includes nested more than %d files deep", chain[len(chain)-1], maxIncludeDepth)
	}
	if l.files += len(paths); l.files > maxIncludedFiles {
		return nil, fmt.Errorf("include: more than %d included files", maxIncludedFiles)
	}
	projectDir := filepath.Dir(paths[0])
	if inc.projectDir != "" {
		projectDir = resolvePath(dir, inc.projectDir)
	}
	// The environment, the other input of an included project's model, is
	// the same for the whole parse.
	key := strings.Join(paths, "\x00") + "\x00" + projectDir
	if project, ok := l.loaded[key]; ok {
		return project, nil
	}

	files := make([]composetypes.ConfigFile, len(paths))
	for i, path := range paths {
		data, err := l.read(ctx, path)
		if err != nil {
			return nil, err
		}
		if files[i], err = l.configFile(ctx, path, data, projectDir, chain); err != nil {
			return nil, err
		}
	}
	project, err := loader.LoadModelWithContext(ctx, composetypes.ConfigDetails{
		WorkingDir:  projectDir,
		ConfigFiles: files,
	}, l.options(projectDir))
	if err != nil {
		return nil, err
	}
	l.loaded[key] = project
	return project, nil
}

// read reads an included file, counting it against the bytes the parse
// may read.
func (l *includeLoader) read(ctx context.Context, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("include: %w", err)
	}
	data, err := l.budget.read(path)
	if err != nil {
		return nil, fmt.Errorf("include: %w", err)
	}
	return data, nil
}

// configFile returns data, an included file, for the loader; one using
// include comes with its model (see model). projectDir is the directory of
// the project the file is part of; chain lists the files that include that
// project.
func (l *includeLoader) configFile(ctx context.Context, path string, data []byte, projectDir string, chain []string) (composetypes.ConfigFile, error) {
	file := composetypes.ConfigFile{Filename: path, Content: data}
	header, decoded, err := composeHeaders(path, data)
	switch {
	case !decoded:
		return file, err
	case !header.usesInclude():
		return file, l.budget.extend(path, header.extendsNodes)
	}
	file.Config, err = l.model(ctx, path, data, header, projectDir, chain)
	return file, err
}

// resolvePath returns path as an absolute or cleaned path, relative to dir.
func resolvePath(dir, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(dir, path)
}

// includedResources are the sections an include adds to the including file.
var includedResources = []string{"services", "jobs", "volumes", "networks", "secrets", "configs", "models"}

// importResources adds the resources of an included project's model to
// imported, the resources of the projects included before it. A resource
// both define is merged, the earlier definition winning, as the loader's
// include merges it. project is left as it is, to be reused.
func importResources(imported, project map[string]any) error {
	for _, key := range includedResources {
		from, ok := project[key].(map[string]any)
		if !ok {
			continue
		}
		to, _ := imported[key].(map[string]any)
		if to == nil {
			to = make(map[string]any, len(from))
		}
		for name, resource := range from {
			existing, defined := to[name]
			switch {
			case !defined:
				to[name] = copyModel(resource, nil)
			case !reflect.DeepEqual(resource, existing):
				merged, err := override.MergeYaml(copyModel(resource, nil), existing, tree.NewPath(key, name))
				if err != nil {
					return err
				}
				to[name] = merged
			}
		}
		imported[key] = to
	}
	return nil
}

// escapeInterpolation returns a copy of model with every "$" in its strings
// doubled, so that interpolating it leaves it as it is.
func escapeInterpolation(model map[string]any) map[string]any {
	escape := func(s string) string { return strings.ReplaceAll(s, "$", "$$") }
	return copyModel(model, escape).(map[string]any)
}

// copyModel returns a deep copy of v, a value of a model, with each of its
// strings replaced by replace's result when replace is not nil.
func copyModel(v any, replace func(string) string) any {
	switch v := v.(type) {
	case string:
		if replace != nil {
			return replace(v)
		}
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, e := range v {
			out[k] = copyModel(e, replace)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, e := range v {
			out[i] = copyModel(e, replace)
		}
		return out
	}
	return v
}
