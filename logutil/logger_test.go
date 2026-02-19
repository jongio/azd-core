// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package logutil

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewLoggerCreatesWithComponent(t *testing.T) {
	var buf bytes.Buffer
	SetupLoggerWithWriter(&buf, false, false)

	logger := NewLogger("mycomponent")
	if logger == nil {
		t.Fatal("NewLogger returned nil")
	}
	if logger.Component() != "mycomponent" {
		t.Errorf("expected component 'mycomponent', got %q", logger.Component())
	}

	logger.Info("hello")
	output := buf.String()
	if !strings.Contains(output, "component=mycomponent") {
		t.Errorf("expected output to contain component=mycomponent, got: %s", output)
	}
}

func TestWithServiceAddsContext(t *testing.T) {
	var buf bytes.Buffer
	SetupLoggerWithWriter(&buf, false, false)

	logger := NewLogger("comp").WithService("svc1")
	logger.Info("test")

	output := buf.String()
	if !strings.Contains(output, "component=comp") {
		t.Errorf("expected component=comp in output, got: %s", output)
	}
	if !strings.Contains(output, "service=svc1") {
		t.Errorf("expected service=svc1 in output, got: %s", output)
	}
}

func TestWithOperationAddsContext(t *testing.T) {
	var buf bytes.Buffer
	SetupLoggerWithWriter(&buf, false, false)

	logger := NewLogger("comp").WithOperation("deploy")
	logger.Info("test")

	output := buf.String()
	if !strings.Contains(output, "component=comp") {
		t.Errorf("expected component=comp in output, got: %s", output)
	}
	if !strings.Contains(output, "operation=deploy") {
		t.Errorf("expected operation=deploy in output, got: %s", output)
	}
}

func TestWithFieldsAddsArbitraryFields(t *testing.T) {
	var buf bytes.Buffer
	SetupLoggerWithWriter(&buf, false, false)

	logger := NewLogger("comp").WithFields("env", "prod", "region", "eastus")
	logger.Info("test")

	output := buf.String()
	if !strings.Contains(output, "env=prod") {
		t.Errorf("expected env=prod in output, got: %s", output)
	}
	if !strings.Contains(output, "region=eastus") {
		t.Errorf("expected region=eastus in output, got: %s", output)
	}
}

func TestChainingContexts(t *testing.T) {
	var buf bytes.Buffer
	SetupLoggerWithWriter(&buf, false, false)

	logger := NewLogger("orchestrator").WithService("api").WithOperation("health-check")
	logger.Info("chain test")

	output := buf.String()
	if !strings.Contains(output, "component=orchestrator") {
		t.Errorf("expected component=orchestrator, got: %s", output)
	}
	if !strings.Contains(output, "service=api") {
		t.Errorf("expected service=api, got: %s", output)
	}
	if !strings.Contains(output, "operation=health-check") {
		t.Errorf("expected operation=health-check, got: %s", output)
	}
	// Component should still be the original
	if logger.Component() != "orchestrator" {
		t.Errorf("expected component 'orchestrator', got %q", logger.Component())
	}
}

func TestComponentReturnsCorrectName(t *testing.T) {
	SetupLogger(false, false)

	logger := NewLogger("test-component")
	if logger.Component() != "test-component" {
		t.Errorf("expected 'test-component', got %q", logger.Component())
	}

	// Chaining should preserve the component name
	chained := logger.WithService("svc").WithOperation("op")
	if chained.Component() != "test-component" {
		t.Errorf("expected 'test-component' after chaining, got %q", chained.Component())
	}
}

func TestLogLevels(t *testing.T) {
	tests := []struct {
		name    string
		logFunc func(*ComponentLogger, string, ...any)
		level   string
	}{
		{"debug", (*ComponentLogger).Debug, "DEBUG"},
		{"info", (*ComponentLogger).Info, "INFO"},
		{"warn", (*ComponentLogger).Warn, "WARN"},
		{"error", (*ComponentLogger).Error, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			SetupLoggerWithWriter(&buf, true, false) // debug=true to capture all levels

			logger := NewLogger("lvl-test")
			tt.logFunc(logger, "level test msg", "k", "v")

			output := buf.String()
			if !strings.Contains(output, tt.level) {
				t.Errorf("expected level %s in output, got: %s", tt.level, output)
			}
			if !strings.Contains(output, "level test msg") {
				t.Errorf("expected message in output, got: %s", output)
			}
		})
	}
}

func TestLogLevelsStructured(t *testing.T) {
	var buf bytes.Buffer
	SetupLoggerWithWriter(&buf, true, true) // structured JSON

	logger := NewLogger("json-test")
	logger.Info("structured msg", "count", 42)

	output := buf.String()
	if !strings.Contains(output, `"component":"json-test"`) {
		t.Errorf("expected component in JSON output, got: %s", output)
	}
	if !strings.Contains(output, `"msg":"structured msg"`) {
		t.Errorf("expected msg in JSON output, got: %s", output)
	}
	if !strings.Contains(output, `"count":42`) {
		t.Errorf("expected count in JSON output, got: %s", output)
	}
}
