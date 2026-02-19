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
	defer lw.mu.Unlock()

	// Write to the actual output first
	if lw.output != nil {
		n, err = lw.output.Write(p)
		if err != nil {
			return n, err
		}
	} else {
		n = len(p)
	}

	// Add to buffer and process complete lines
	lw.buf = append(lw.buf, p...)
	for {
		idx := bytes.IndexByte(lw.buf, '\n')
		if idx < 0 {
			break
		}
		line := string(lw.buf[:idx])
		lw.buf = lw.buf[idx+1:]
		if lw.handler != nil {
			lw.handler(line)
		}
	}

	return n, nil
}

// Flush processes any remaining buffered data as a final line.
func (lw *lineWriter) Flush() {
	lw.mu.Lock()
	defer lw.mu.Unlock()

	if len(lw.buf) > 0 && lw.handler != nil {
		lw.handler(string(lw.buf))
		lw.buf = nil
	}
}
