// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package browser

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestAwaitOpen_Succeeds checks that a successful open stays silent.
func TestAwaitOpen_Succeeds(t *testing.T) {
	done := make(chan error, 1)
	done <- nil

	var buf bytes.Buffer

	awaitOpen(done, time.Minute, "http://localhost:4280", &buf)

	if buf.Len() != 0 {
		t.Errorf("a successful open wrote to the user; got %q", buf.String())
	}
}

// TestAwaitOpen_ReportsFailure checks the failure path names the error and
// falls back to printing the URL.
func TestAwaitOpen_ReportsFailure(t *testing.T) {
	done := make(chan error, 1)
	done <- errors.New("no browser found")

	var buf bytes.Buffer

	awaitOpen(done, time.Minute, "http://localhost:4280", &buf)

	out := buf.String()
	if !strings.Contains(out, "no browser found") {
		t.Errorf("failure output did not name the underlying error; got %q", out)
	}

	if !strings.Contains(out, "http://localhost:4280") {
		t.Errorf("failure output did not fall back to the URL; got %q", out)
	}
}

// TestAwaitOpen_TimesOut drives the timeout branch by never sending on the
// channel, so it does not depend on how fast the host opens a browser.
func TestAwaitOpen_TimesOut(t *testing.T) {
	done := make(chan error) // never written

	var buf bytes.Buffer

	awaitOpen(done, time.Millisecond, "http://localhost:4280", &buf)

	out := buf.String()
	if !strings.Contains(out, "timed out") {
		t.Errorf("timeout output did not say it timed out; got %q", out)
	}

	if !strings.Contains(out, "http://localhost:4280") {
		t.Errorf("timeout output did not fall back to the URL; got %q", out)
	}
}

// TestLaunch_UsesOpener checks that a launching target actually reaches the
// opener, and does it against a stub so the suite does not open a browser on
// the machine running it.
func TestLaunch_UsesOpener(t *testing.T) {
	var (
		mu     sync.Mutex
		gotURL string
	)

	called := make(chan struct{})

	restore := stubOpener(t, func(url string) error {
		mu.Lock()
		gotURL = url
		mu.Unlock()
		close(called)

		return nil
	})
	defer restore()

	if err := Launch(LaunchOptions{
		URL:     "http://localhost:4280",
		Target:  TargetSystem,
		Timeout: time.Minute,
	}); err != nil {
		t.Fatalf("Launch returned %v", err)
	}

	select {
	case <-called:
	case <-time.After(10 * time.Second):
		t.Fatal("Launch never reached the opener")
	}

	mu.Lock()
	defer mu.Unlock()

	if gotURL != "http://localhost:4280" {
		t.Errorf("opener got URL %q, want %q", gotURL, "http://localhost:4280")
	}
}

// TestLaunch_NoneTargetSkipsOpener checks the none target short circuits before
// the opener rather than opening and discarding.
func TestLaunch_NoneTargetSkipsOpener(t *testing.T) {
	var opened bool

	restore := stubOpener(t, func(string) error {
		opened = true

		return nil
	})
	defer restore()

	if err := Launch(LaunchOptions{
		URL:     "http://localhost:4280",
		Target:  TargetNone,
		Timeout: time.Minute,
	}); err != nil {
		t.Fatalf("Launch returned %v", err)
	}

	// Launch is asynchronous, so give a wrongly started goroutine time to run.
	time.Sleep(50 * time.Millisecond)

	if opened {
		t.Error("the none target reached the opener")
	}
}

// stubOpener swaps the package opener for the duration of a test.
func stubOpener(t *testing.T, fn func(string) error) func() {
	t.Helper()

	prev := openURL
	openURL = fn

	return func() { openURL = prev }
}
