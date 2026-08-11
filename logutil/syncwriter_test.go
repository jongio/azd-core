// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package logutil

import (
	"bytes"
	"sync"
	"testing"
)

// TestNewSyncWriter_WrapsOnce checks that re-wrapping an already wrapped writer
// returns it unchanged. Without this, every SetOutput call would add another
// layer of locking around the same writer.
func TestNewSyncWriter_WrapsOnce(t *testing.T) {
	inner := &bytes.Buffer{}

	first := newSyncWriter(inner)
	if _, ok := first.(*syncWriter); !ok {
		t.Fatalf("newSyncWriter returned %T, want *syncWriter", first)
	}

	second := newSyncWriter(first)
	if second != first {
		t.Error("wrapping an already wrapped writer produced a new layer")
	}
}

// TestSyncWriter_SerializesWrites checks the wrapper actually serializes, using
// a writer that reports when it is entered concurrently.
func TestSyncWriter_SerializesWrites(t *testing.T) {
	probe := &concurrencyProbe{}
	w := newSyncWriter(probe)

	var wg sync.WaitGroup

	for range 32 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			_, _ = w.Write([]byte("x"))
		}()
	}

	wg.Wait()

	if probe.overlapped() {
		t.Error("two writes were inside the writer at the same time")
	}

	if got := probe.count(); got != 32 {
		t.Errorf("writer saw %d writes, want 32", got)
	}
}

// TestSyncWriter_PropagatesResult checks the wrapper does not swallow what the
// underlying writer reported.
func TestSyncWriter_PropagatesResult(t *testing.T) {
	inner := &bytes.Buffer{}
	w := newSyncWriter(inner)

	n, err := w.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Write returned %v", err)
	}

	if n != 5 {
		t.Errorf("Write reported %d bytes, want 5", n)
	}

	if inner.String() != "hello" {
		t.Errorf("underlying writer holds %q, want %q", inner.String(), "hello")
	}
}

// concurrencyProbe records whether more than one goroutine was ever inside
// Write at the same time.
type concurrencyProbe struct {
	mu     sync.Mutex
	inside int
	seen   bool
	writes int
}

func (p *concurrencyProbe) Write(b []byte) (int, error) {
	p.mu.Lock()
	p.inside++
	p.writes++

	if p.inside > 1 {
		p.seen = true
	}
	p.mu.Unlock()

	p.mu.Lock()
	p.inside--
	p.mu.Unlock()

	return len(b), nil
}

func (p *concurrencyProbe) overlapped() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.seen
}

func (p *concurrencyProbe) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.writes
}
