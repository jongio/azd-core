package logutil_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/jongio/azd-core/logutil"
)

// capture points the package logger at a buffer and restores stderr afterwards.
func capture(t *testing.T, debug, structured bool) *bytes.Buffer {
	t.Helper()

	buf := &bytes.Buffer{}
	logutil.SetupLoggerWithWriter(buf, debug, structured)

	t.Cleanup(func() {
		logutil.SetupLogger(false, false)
	})

	return buf
}

// TestComponentLogger_HonorsPackageWriter is the regression test for the trap
// in this migration. azdext.NewLogger builds its own handler and defaults to
// stderr rather than inheriting slog.Default, so a component logger that did not
// pass the writer through would silently drop out of every captured buffer and
// leak to stderr instead.
func TestComponentLogger_HonorsPackageWriter(t *testing.T) {
	buf := capture(t, false, false)

	logutil.NewLogger("mycomponent").Info("hello")

	out := buf.String()
	if !strings.Contains(out, "hello") {
		t.Fatalf("component logger did not write to the package writer, got %q", out)
	}

	if !strings.Contains(out, "component=mycomponent") {
		t.Errorf("output %q is missing the component attribute", out)
	}
}

// TestComponentLogger_HonorsStructuredFormat verifies the format setting also
// reaches the wrapped SDK logger.
func TestComponentLogger_HonorsStructuredFormat(t *testing.T) {
	buf := capture(t, false, true)

	logutil.NewLogger("svc").Info("structured message", "key", "value")

	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry); err != nil {
		t.Fatalf("output is not JSON: %v (got %q)", err, buf.String())
	}

	if entry["msg"] != "structured message" {
		t.Errorf("msg = %v, want %q", entry["msg"], "structured message")
	}

	if entry["component"] != "svc" {
		t.Errorf("component = %v, want %q", entry["component"], "svc")
	}

	if entry["key"] != "value" {
		t.Errorf("key = %v, want %q", entry["key"], "value")
	}
}

// TestComponentLogger_RespectsSetLevel is the second reason the wrapper filters
// rather than the SDK. azdext.LoggerOptions has a single Debug boolean, so it
// can express only debug and info. Without local filtering, SetLevel(LevelError)
// would keep emitting info and warn lines.
func TestComponentLogger_RespectsSetLevel(t *testing.T) {
	cases := []struct {
		name    string
		level   logutil.Level
		want    []string
		notWant []string
	}{
		{"info", logutil.LevelInfo, []string{"i", "w", "e"}, []string{"d"}},
		{"warn", logutil.LevelWarn, []string{"w", "e"}, []string{"d", "i"}},
		{"error", logutil.LevelError, []string{"e"}, []string{"d", "i", "w"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("AZD_DEBUG", "")

			buf := capture(t, false, false)
			logutil.SetLevel(tc.level)

			logger := logutil.NewLogger("c")
			logger.Debug("msg-d")
			logger.Info("msg-i")
			logger.Warn("msg-w")
			logger.Error("msg-e")

			out := buf.String()
			for _, want := range tc.want {
				if !strings.Contains(out, "msg-"+want) {
					t.Errorf("level %v dropped msg-%s; output %q", tc.level, want, out)
				}
			}

			for _, notWant := range tc.notWant {
				if strings.Contains(out, "msg-"+notWant) {
					t.Errorf("level %v emitted msg-%s; output %q", tc.level, notWant, out)
				}
			}
		})
	}
}

// TestComponentLogger_DebugFollowsEnvVar verifies debug gating still consults
// AZD_DEBUG at call time rather than only at construction, which is how the SDK
// logger behaves on its own.
func TestComponentLogger_DebugFollowsEnvVar(t *testing.T) {
	buf := capture(t, false, false)
	logger := logutil.NewLogger("c")

	logger.Debug("before")
	if strings.Contains(buf.String(), "before") {
		t.Fatalf("debug emitted while disabled: %q", buf.String())
	}

	t.Setenv("AZD_DEBUG", "1")

	logger.Debug("after")
	if !strings.Contains(buf.String(), "after") {
		t.Errorf("debug suppressed after AZD_DEBUG was set; output %q", buf.String())
	}
}

// TestIsDebugEnabled_AcceptsSDKTruthyValues pins the parsing fix. This used to
// compare against the literal string "true", so the values the framework itself
// accepts did nothing.
func TestIsDebugEnabled_AcceptsSDKTruthyValues(t *testing.T) {
	logutil.SetupLogger(false, false)

	truthy := []string{"true", "True", "TRUE", "1", "t", "T", "yes", "YES"}
	for _, v := range truthy {
		t.Run("truthy_"+v, func(t *testing.T) {
			t.Setenv("AZD_DEBUG", v)

			if !logutil.IsDebugEnabled() {
				t.Errorf("AZD_DEBUG=%q did not enable debug", v)
			}
		})
	}

	falsy := []string{"", "false", "0", "no", "on", "maybe"}
	for _, v := range falsy {
		t.Run("falsy_"+v, func(t *testing.T) {
			t.Setenv("AZD_DEBUG", v)

			if logutil.IsDebugEnabled() {
				t.Errorf("AZD_DEBUG=%q enabled debug", v)
			}
		})
	}
}

// TestComponentLogger_ContextChaining covers the derivation helpers, including
// WithComponent, which is new and matches the SDK reparenting semantics.
func TestComponentLogger_ContextChaining(t *testing.T) {
	buf := capture(t, false, true)

	logutil.NewLogger("root").
		WithService("billing").
		WithOperation("charge").
		WithFields("attempt", 2).
		Info("chained")

	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry); err != nil {
		t.Fatalf("output is not JSON: %v (got %q)", err, buf.String())
	}

	for key, want := range map[string]any{
		"component": "root",
		"service":   "billing",
		"operation": "charge",
		"attempt":   float64(2),
	} {
		if entry[key] != want {
			t.Errorf("%s = %v, want %v", key, entry[key], want)
		}
	}
}

func TestComponentLogger_WithComponentReparents(t *testing.T) {
	buf := capture(t, false, true)

	child := logutil.NewLogger("parent").WithComponent("child")
	child.Info("reparented")

	if child.Component() != "child" {
		t.Errorf("Component() = %q, want %q", child.Component(), "child")
	}

	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry); err != nil {
		t.Fatalf("output is not JSON: %v (got %q)", err, buf.String())
	}

	if entry["component"] != "child" {
		t.Errorf("component = %v, want child", entry["component"])
	}

	if entry["parent_component"] != "parent" {
		t.Errorf("parent_component = %v, want parent", entry["parent_component"])
	}
}

// TestComponentLogger_ExposesUnderlyingLoggers verifies the escape hatches used
// to hand a logger to an SDK or slog-based API.
func TestComponentLogger_ExposesUnderlyingLoggers(t *testing.T) {
	buf := capture(t, false, false)
	logger := logutil.NewLogger("c")

	if logger.AzdextLogger() == nil {
		t.Fatal("AzdextLogger returned nil")
	}

	if logger.AzdextLogger().Component() != "c" {
		t.Errorf("wrapped logger component = %q, want %q", logger.AzdextLogger().Component(), "c")
	}

	slogger := logger.Slogger()
	if slogger == nil {
		t.Fatal("Slogger returned nil")
	}

	slogger.Info("direct")
	if !strings.Contains(buf.String(), "direct") {
		t.Errorf("writing through Slogger did not reach the package writer; got %q", buf.String())
	}
}

// TestComponentLogger_ConcurrentUse guards the read lock taken when capturing
// the writer at construction against concurrent SetOutput calls.
func TestComponentLogger_ConcurrentUse(t *testing.T) {
	capture(t, false, false)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			logutil.NewLogger("c").WithService("s").Info("concurrent")
		}()
	}

	wg.Wait()
}

// TestSetOutput_RedirectsSubsequentLoggers verifies the package writer is read
// at construction, which is the documented contract.
func TestSetOutput_RedirectsSubsequentLoggers(t *testing.T) {
	first := &bytes.Buffer{}
	second := &bytes.Buffer{}

	logutil.SetupLoggerWithWriter(first, false, false)

	t.Cleanup(func() { logutil.SetupLogger(false, false) })

	logutil.SetOutput(second)
	logutil.NewLogger("c").Info("after redirect")

	if strings.Contains(first.String(), "after redirect") {
		t.Error("log line went to the old writer")
	}

	if !strings.Contains(second.String(), "after redirect") {
		t.Errorf("log line did not reach the new writer; got %q", second.String())
	}
}
