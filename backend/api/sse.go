package api

import (
	"fmt"
	"net/http"
	"time"
)

// streamWriteTimeout bounds each write of a long-lived response, and of a
// WebSocket connection. Streams have no overall deadline (see streamWriter),
// but a client that stops reading must not hold its stream, and the Docker
// streams behind it, open. A variable so tests can lower it.
var streamWriteTimeout = 30 * time.Second

// streamWriter wraps the ResponseWriter of a long-lived response (SSE). The
// server's WriteTimeout is an absolute deadline from the start of the
// request, so it would cut every log stream after it expires even while
// lines are flowing; instead each write gets streamWriteTimeout of its own.
type streamWriter struct {
	http.ResponseWriter
	rc *http.ResponseController
}

func newStreamWriter(w http.ResponseWriter) *streamWriter {
	return &streamWriter{ResponseWriter: w, rc: http.NewResponseController(w)}
}

func (s *streamWriter) extend() {
	// Writers without deadlines (test recorders) are left as they are.
	_ = s.rc.SetWriteDeadline(time.Now().Add(streamWriteTimeout))
}

// send writes one SSE data event and flushes it. An error means the client
// is gone or stopped reading; the stream should end.
func (s *streamWriter) send(data []byte) error {
	s.extend()
	if _, err := fmt.Fprintf(s.ResponseWriter, "data: %s\n\n", data); err != nil {
		return err
	}
	return s.FlushError()
}

func (s *streamWriter) Write(p []byte) (int, error) {
	s.extend()
	return s.ResponseWriter.Write(p)
}

// FlushError sends buffered data to the client. http.ResponseController
// (used by httputil.ReverseProxy) calls it in preference to Flush.
func (s *streamWriter) FlushError() error {
	s.extend()
	return s.rc.Flush()
}

// Flush is FlushError for callers of http.Flusher.
func (s *streamWriter) Flush() {
	_ = s.FlushError()
}

// Unwrap gives http.ResponseController access to the wrapped writer.
func (s *streamWriter) Unwrap() http.ResponseWriter {
	return s.ResponseWriter
}
