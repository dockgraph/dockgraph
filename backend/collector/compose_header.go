package collector

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/template"
	"go.yaml.in/yaml/v4"
)

// composeHeader is what is read from a compose file before it is loaded:
// its project's name, its includes and what its extends copy.
type composeHeader struct {
	name      string            // the project name the loader reads
	documents []composeDocument // in order
	// extendsNodes is what extends between the services of each of the
	// file's documents copies, as the loader applies it document by
	// document.
	extendsNodes int
	// largestService is the most one extends of a service in the file
	// copies: its largest service, with that service's own extends in the
	// file applied.
	largestService int
}

// composeDocument is a document of a compose file.
type composeDocument struct {
	line    int // the line it starts on
	include any // its include entries, nil when it has none
}

// usesInclude reports whether a document of the file has an include.
func (h composeHeader) usesInclude() bool {
	return slices.ContainsFunc(h.documents, func(d composeDocument) bool { return d.include != nil })
}

// decodeComposeHeader decodes the header of a compose file. Like the
// loader's own read of the project name, it builds no model of the file. A
// file with more than maxComposeDocuments documents fails with
// errTooManyDocuments.
func decodeComposeHeader(data []byte) (composeHeader, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var header composeHeader
	// The loader reads the name from the documents before the first one it
	// cannot read a name from (projectName in compose-go's loader/loader.go).
	naming := true
	whole := make(map[string]*sectionServices)
	for {
		var doc yaml.Node
		err := decoder.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return composeHeader{}, err
		}
		if len(header.documents) == maxComposeDocuments {
			return composeHeader{}, errTooManyDocuments
		}
		if naming {
			var named struct {
				Name string `yaml:"name"`
			}
			if err := doc.Decode(&named); err != nil {
				naming = false
			} else if named.Name != "" {
				header.name = named.Name
			}
		}
		include, err := documentInclude(&doc)
		if err != nil {
			return composeHeader{}, err
		}
		header.documents = append(header.documents, composeDocument{line: doc.Line, include: include})
		for section, services := range documentServices(&doc) {
			copied, _ := services.extendsCopies()
			header.extendsNodes += copied
			if whole[section] == nil {
				whole[section] = newSectionServices()
			}
			whole[section].add(services)
		}
	}
	// Loaded as an extends file, the file's documents are merged first.
	for _, services := range whole {
		_, largest := services.extendsCopies()
		header.largestService = max(header.largestService, largest)
	}
	return header, nil
}

// composeHeaders returns the header of data, the compose file at path, and
// whether it decodes. One that does not is left for the loader to report;
// one with too many documents is refused.
func composeHeaders(path string, data []byte) (composeHeader, bool, error) {
	header, err := decodeComposeHeader(data)
	switch {
	case errors.Is(err, errTooManyDocuments):
		return composeHeader{}, false, fmt.Errorf("%s: %w", path, err)
	case err != nil:
		return composeHeader{}, false, nil
	}
	return header, true, nil
}

// documentInclude returns the include entries of a compose file document:
// none where it has none or the loader drops them, tagged !reset.
func documentInclude(doc *yaml.Node) (any, error) {
	var fields struct {
		Include yaml.Node `yaml:"include"`
	}
	// An absent include leaves the node's kind unset.
	if doc.Decode(&fields) != nil || fields.Include.Kind == 0 {
		return nil, nil // the loader reports a document that is not a mapping
	}
	node := &fields.Include
	if node.Kind == yaml.AliasNode && node.Alias != nil {
		node = node.Alias
	}
	if node.Tag == "!reset" {
		return nil, nil
	}
	var include any
	err := node.Decode(&include)
	return include, err
}

// composeProjectName returns the name the loader gives the project of a
// compose file (see projectName in compose-go's loader/loader.go): the name
// set for it, else the one its documents give, interpolated and normalized,
// else, for a stack, the file's basename.
func composeProjectName(path string, header composeHeader, opts composeParseOptions) (string, error) {
	if opts.ProjectName != "" {
		return loader.NormalizeProjectName(opts.ProjectName), nil
	}
	// The project name is not yet part of the environment the name is
	// interpolated from.
	name, err := template.Substitute(header.name, func(string) (string, bool) { return "", false })
	if err != nil {
		return "", fmt.Errorf("name: %w", err)
	}
	if name = loader.NormalizeProjectName(name); name == "" && opts.Swarm {
		name = loader.NormalizeProjectName(fileBaseName(path))
	}
	return name, nil
}

// fileBaseName returns the name of the file at path without its extension.
func fileBaseName(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}
