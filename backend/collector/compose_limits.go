package collector

import (
	"fmt"
	"io"
	"os"
	"syscall"
	"time"
)

// maxComposeFileSize bounds the size of a compose file and, together, of
// the compose files one parse reads: the file, its includes and its
// extends files.
const maxComposeFileSize = 1 << 20

// Includes are loaded file by file, each able to include more, so these
// bound, with the total size above, the work of one parse however its
// includes are arranged.
const (
	// maxIncludeDepth bounds how deeply includes nest.
	maxIncludeDepth = 10
	// maxIncludedFiles bounds the included files one parse processes,
	// counting every path of every include entry, a file loaded before too.
	maxIncludedFiles = 256
)

// maxComposeDocuments bounds the YAML documents of a compose file, far
// above what a compose file uses: the loader merges and validates the whole
// model once per document, so its work grows with their number squared.
const maxComposeDocuments = 64

// errTooManyDocuments refuses a compose file with more than
// maxComposeDocuments documents.
var errTooManyDocuments = fmt.Errorf("more than %d YAML documents", maxComposeDocuments)

// maxExtendsFiles bounds the extends files one parse uses, each path the
// loader opens counting as a file of its own.
const maxExtendsFiles = 64

// maxExtendsNodes bounds the YAML nodes extends copies into one parse's
// model, as many as one document may hold: every service extending another
// takes in a copy of it, so a few lines of a file could otherwise copy a
// large service hundreds of times.
const maxExtendsNodes = 100_000

// composeParseTimeout bounds how long a parse may take: the process doing
// it is killed then (see parseComposeInChild).
const composeParseTimeout = 30 * time.Second

// openComposeFile opens the compose file at path, refusing anything but a
// regular file of at most maxComposeFileSize bytes. The file is opened
// without blocking and checked once open, so a FIFO is refused rather than
// waited on, even one put in place after the path was listed.
func openComposeFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	switch {
	case err != nil:
	case !info.Mode().IsRegular():
		err = fmt.Errorf("%s: not a regular file", path)
	case info.Size() > maxComposeFileSize:
		err = fmt.Errorf("%s: larger than %d bytes", path, maxComposeFileSize)
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// readComposeFile reads the compose file at path, refusing one that
// openComposeFile refuses.
func readComposeFile(path string) ([]byte, error) {
	f, err := openComposeFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// The file may have grown since it was checked.
	data, err := io.ReadAll(io.LimitReader(f, maxComposeFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxComposeFileSize {
		return nil, fmt.Errorf("%s: larger than %d bytes", path, maxComposeFileSize)
	}
	return data, nil
}

// composeBudget counts what one parse reads and what extends copies into
// its model.
type composeBudget struct {
	bytes        int // bytes of compose files read
	extendsNodes int // YAML nodes extends copies
}

// read reads the compose file at path, counting it against the bytes one
// parse may read.
func (b *composeBudget) read(path string) ([]byte, error) {
	data, err := readComposeFile(path)
	if err != nil {
		return nil, err
	}
	if b.bytes += len(data); b.bytes > maxComposeFileSize {
		return nil, fmt.Errorf("%s: compose files larger than %d bytes in total", path, maxComposeFileSize)
	}
	return data, nil
}

// extend adds nodes, the YAML nodes extends copies in or from the file at
// path, to the parse's total, failing past maxExtendsNodes.
func (b *composeBudget) extend(path string, nodes int) error {
	if b.extendsNodes += nodes; b.extendsNodes > maxExtendsNodes {
		return fmt.Errorf("%s: extends copies more than %d YAML nodes in total", path, maxExtendsNodes)
	}
	return nil
}
