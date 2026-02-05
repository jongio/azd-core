// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package logutil

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestSetupLogger(t *testing.T) {
	// Test debug mode
	SetupLogger(true, false)
	if !IsDebugEnabled() {
		t.Error("expected debug to be enabled")
	}
	if currentLevel != LevelDebug {
		t.Errorf("expected LevelDebug, got %v", currentLevel)
	}

	// Test non-debug mode
	SetupLogger(false, false)
	if currentLevel != LevelInfo {
		t.Errorf("expected LevelInfo, got %v", currentLevel)
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
	}{
		{"debug", LevelDebug},
		{"DEBUG", LevelDebug},
		{"info", LevelInfo},
		{"INFO", LevelInfo},
		{"warn", LevelWarn},
		{"warning", LevelWarn},
		{"error", LevelError},
		{"ERROR", LevelError},
		{"unknown", LevelInfo},
		{"", LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseLevel(tt.input)
			if result != tt.expected {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsDebugEnabledEnvVar(t *testing.T) {
	// Save and restore original value
	original := os.Getenv(EnvDebug)
	defer os.Setenv(EnvDebug, original)

	// Test with env var set
	SetupLogger(false, false)
	os.Setenv(EnvDebug, "true")
	if !IsDebugEnabled() {
		t.Error("expected debug to be enabled via env var")
	}

	// Test with env var unset
	os.Setenv(EnvDebug, "")
	if IsDebugEnabled() {
		t.Error("expected debug to be disabled")
	}
}

func TestLogOutputText(t *testing.T) {
	var buf bytes.Buffer

	// Create a fresh logger writing to our buffer
	SetupLoggerWithWriter(&buf, true, false)

	Debug("test debug message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test debug message") {
		t.Errorf("expected log output to contain message, got: %s", output)
	}
	if !strings.Contains(output, "key=value") {
		t.Errorf("expected log output to contain key=value, got: %s", output)
	}
}

func TestStructuredLogging(t *testing.T) {
	var buf bytes.Buffer

	// Create a fresh logger writing to our buffer
	SetupLoggerWithWriter(&buf, false, true)

	Info("test message", "count", 42)

	output := buf.String()
	if !strings.Contains(output, `"msg":"test message"`) {
		t.Errorf("expected JSON output with msg field, got: %s", output)
	}
	if !strings.Contains(output, `"count":42`) {
		t.Errorf("expected JSON output with count field, got: %s", output)
	}
}

func TestSetLevel(t *testing.T) {
	// Reset to default state first
	SetupLogger(false, false)

	SetLevel(LevelWarn)
	if GetLevel() != LevelWarn {
		t.Errorf("expected LevelWarn, got %v", GetLevel())
	}

	SetLevel(LevelDebug)
	if GetLevel() != LevelDebug {
		t.Errorf("expected LevelDebug, got %v", GetLevel())
	}
	if !IsDebugEnabled() {
		t.Error("expected debug to be enabled after SetLevel(LevelDebug)")
	}
}

func TestSetOutput(t *testing.T) {
	var buf bytes.Buffer

	SetupLogger(true, false)
	SetOutput(&buf)

	Debug("test message after SetOutput")

	output := buf.String()
	if !strings.Contains(output, "test message after SetOutput") {
		t.Errorf("expected output to contain message after SetOutput, got: %s", output)
	}
}

func TestLogger(t *testing.T) {
	SetupLogger(false, false)
	logger := Logger()
	if logger == nil {
		t.Error("Logger() returned nil")
	}
}

func TestWarn(t *testing.T) {
	var buf bytes.Buffer
	SetupLoggerWithWriter(&buf, false, false)

	Warn("test warning", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test warning") {
		t.Errorf("expected output to contain warning message, got: %s", output)
	}
}

func TestError(t *testing.T) {
	var buf bytes.Buffer
	SetupLoggerWithWriter(&buf, false, false)

	Error("test error", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test error") {
		t.Errorf("expected output to contain error message, got: %s", output)
	}
}

func TestInfo(t *testing.T) {
	var buf bytes.Buffer
	SetupLoggerWithWriter(&buf, false, false)

	Info("test info", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test info") {
		t.Errorf("expected output to contain info message, got: %s", output)
	}
}

func TestDebugWhenDisabled(t *testing.T) {
	var buf bytes.Buffer
	SetupLoggerWithWriter(&buf, false, false)

	// Clear AZD_DEBUG env var
	original := os.Getenv(EnvDebug)
	defer os.Setenv(EnvDebug, original)
	os.Setenv(EnvDebug, "")

	Debug("should not appear")

	output := buf.String()
	if strings.Contains(output, "should not appear") {
		t.Errorf("debug message should not appear when debug is disabled, got: %s", output)
	}
}

func TestSetLevelError(t *testing.T) {
	var buf bytes.Buffer
	SetupLoggerWithWriter(&buf, false, false)

	SetLevel(LevelError)
	if GetLevel() != LevelError {
		t.Errorf("expected LevelError, got %v", GetLevel())
	}
}

func TestSetLevelInfo(t *testing.T) {
	var buf bytes.Buffer
	SetupLoggerWithWriter(&buf, false, false)

	SetLevel(LevelInfo)
	if GetLevel() != LevelInfo {
		t.Errorf("expected LevelInfo, got %v", GetLevel())
	}
}
