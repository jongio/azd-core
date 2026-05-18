package cmdutil

import (
	"bytes"
	"io"
	"sync"
)

// lineWriter wraps an OutputLineHandler as an io.Writer.
// It buffers partial lines and calls the handler for each complete line.
type lineWriter struct {
	output  io.Writer
	handler OutputLineHandler
	buf     []byte
	mu      sync.Mutex
}

func newLineWriter(handler OutputLineHandler) *lineWriter {
	return &lineWriter{
		output:  io.Discard,
		handler: handler,
	}
}

func (lw *lineWriter) Write(p []byte) (n int, err error) {
	lw.mu.Lock()

	// Capture references under the lock so they can't change mid-call.
	output := lw.output
	handler := lw.handler

	// Buffer incoming data and extract complete lines while holding the lock.
	lw.buf = append(lw.buf, p...)
	var lines []string
	for {
		idx := bytes.IndexByte(lw.buf, '\n')
		if idx < 0 {
			break
		}
		lines = append(lines, string(lw.buf[:idx]))
		lw.buf = lw.buf[idx+1:]
	}
	lw.mu.Unlock()

	// Perform blocking I/O outside the lock to avoid holding it across
	// potentially slow writes or handler callbacks.
	if output != nil {
		n, err = output.Write(p)
		if err != nil {
			return n, err
		}
	} else {
		n = len(p)
	}

	for _, line := range lines {
		if handler != nil {
			handler(line)
		}
	}

	return n, nil
}

// Flush processes any remaining buffered data as a final line.
func (lw *lineWriter) Flush() {
	lw.mu.Lock()
	handler := lw.handler
	var remaining string
	if len(lw.buf) > 0 {
		remaining = string(lw.buf)
		lw.buf = nil
	}
	lw.mu.Unlock()

	if remaining != "" && handler != nil {
		handler(remaining)
	}
}
