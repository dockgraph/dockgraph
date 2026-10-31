package api

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Docker log streams come in two shapes. A container (or service) without a
// TTY sends a multiplexed stream: frames of an 8-byte header [stream type(1),
// zero padding(3), big-endian payload size(4)] followed by the payload. A
// container with a TTY sends its output raw, since a terminal merges stdout
// and stderr. The client API returns only the body, without the Content-Type
// that tells them apart, so the shape is read from the first byte: a frame
// header starts with a stream type (0-3), while every raw line starts with
// the RFC 3339 timestamp requested through LogsOptions.Timestamps (or, for a
// daemon-side error, with "Error grabbing logs").
const (
	frameHeaderSize = 8
	// maxStreamType is the highest stream type Docker multiplexes
	// (stdin=0, stdout=1, stderr=2, systemerr=3).
	maxStreamType = 3
	// maxLogFrameSize caps one multiplexed frame. Docker splits log messages
	// at 16 KiB, so a larger size means a corrupt or misread stream; it is
	// rejected rather than allocated.
	maxLogFrameSize = 1 << 20
	// maxLogLineSize caps a line at the 16 KiB Docker's log drivers split
	// messages at, plus its timestamp, so every buffer of lines (a history
	// page, an aggregate stream's queue) is bounded by its line count. The
	// rest of a longer line (a raw TTY stream, drivers such as awslogs) is
	// dropped: split off as more lines, it would make a history page longer
	// than the lines Docker counted for it.
	maxLogLineSize = 16<<10 + len("2006-01-02T15:04:05.000000000Z ")
	// logReadBufferSize is the read buffer of a stream. Every followed
	// container has one, mostly idle, so it is small; a longer raw line is
	// read through it in pieces.
	logReadBufferSize = 4 << 10
)

var (
	// errInvalidLogFrame reports a multiplexed frame header that is not one.
	errInvalidLogFrame = errors.New("invalid log frame header")
	// errLogFrameTooLarge reports a frame over maxLogFrameSize.
	errLogFrameTooLarge = errors.New("log frame too large")
)

// logLineReader reads a Docker log stream, multiplexed or raw, one line at a
// time.
type logLineReader struct {
	r        *bufio.Reader
	detected bool
	raw      bool
	header   [frameHeaderSize]byte
	// pending holds the lines of the current multiplexed frame not yet
	// returned, all from pendingStream.
	pending       []string
	pendingStream string
}

func newLogLineReader(r io.Reader) *logLineReader {
	return &logLineReader{r: bufio.NewReaderSize(r, logReadBufferSize)}
}

// next returns the next line, without its line ending, and the stream it
// came from ("stdout" or "stderr"). It returns io.EOF at the end of the
// stream and errInvalidLogFrame or errLogFrameTooLarge for a corrupt one.
func (l *logLineReader) next() (stream, line string, err error) {
	if !l.detected {
		first, err := l.r.Peek(1)
		if err != nil {
			return "", "", err
		}
		l.raw = first[0] > maxStreamType
		l.detected = true
	}
	if l.raw {
		return l.nextRaw()
	}
	return l.nextMultiplexed()
}

// nextRaw reads one line of a raw (TTY) stream. A TTY reports everything on
// stdout. A line is kept whole, so it keeps its leading timestamp, up to
// maxLogLineSize; anything beyond is dropped.
func (l *logLineReader) nextRaw() (string, string, error) {
	var line []byte
	for {
		chunk, err := l.r.ReadSlice('\n')
		if room := maxLogLineSize - len(line); room > 0 {
			line = append(line, chunk[:min(len(chunk), room)]...)
		}
		switch {
		case errors.Is(err, bufio.ErrBufferFull):
			continue // the line goes on
		case err == nil, errors.Is(err, io.EOF) && len(line) > 0:
			// The last line of a stream may lack its newline.
			return streamStdout, string(trimLineEnding(line)), nil
		default:
			return "", "", err
		}
	}
}

// nextMultiplexed returns the next line of the current frame, reading frames
// until one holds a line.
func (l *logLineReader) nextMultiplexed() (string, string, error) {
	for len(l.pending) == 0 {
		if err := l.readFrame(); err != nil {
			return "", "", err
		}
	}
	line := l.pending[0]
	l.pending = l.pending[1:]
	return l.pendingStream, line, nil
}

// readFrame reads one multiplexed frame into pending.
func (l *logLineReader) readFrame() error {
	if _, err := io.ReadFull(l.r, l.header[:]); err != nil {
		return err
	}
	if l.header[0] > maxStreamType || l.header[1] != 0 || l.header[2] != 0 || l.header[3] != 0 {
		return errInvalidLogFrame
	}
	size := binary.BigEndian.Uint32(l.header[4:])
	if size > maxLogFrameSize {
		return fmt.Errorf("%w: %d bytes", errLogFrameTooLarge, size)
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(l.r, payload); err != nil {
		return err
	}

	l.pendingStream = streamStdout
	if l.header[0] >= 2 { // stderr, or the daemon's own error stream
		l.pendingStream = streamStderr
	}
	l.pending = splitLines(payload)
	return nil
}

// splitLines splits a frame payload into lines without their endings, each
// cut at maxLogLineSize. A trailing newline ends the last line rather than
// starting an empty one.
func splitLines(payload []byte) []string {
	payload = bytes.TrimSuffix(payload, []byte{'\n'})
	if len(payload) == 0 {
		return nil
	}
	parts := bytes.Split(payload, []byte{'\n'})
	lines := make([]string, len(parts))
	for i, p := range parts {
		p = trimLineEnding(p)
		lines[i] = string(p[:min(len(p), maxLogLineSize)])
	}
	return lines
}

// trimLineEnding drops a trailing "\n" or "\r\n" (a TTY ends lines with
// "\r\n").
func trimLineEnding(b []byte) []byte {
	b = bytes.TrimSuffix(b, []byte{'\n'})
	return bytes.TrimSuffix(b, []byte{'\r'})
}
