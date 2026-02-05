// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package logutil

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// Level represents the logging level.
type Level int

const (
	// LevelDebug is for debug messages.
	LevelDebug Level = iota
	// LevelInfo is for informational messages.
	LevelInfo
	// LevelWarn is for warnings.
	LevelWarn
	// LevelError is for errors.
	LevelError
)

// Environment variable names for logging configuration.
const (
	// EnvDebug enables debug logging when set to "true".
	EnvDebug = "AZD_DEBUG"
)

var (
	mu           sync.RWMutex
	globalLogger *slog.Logger
	currentLevel           = LevelInfo
	isStructured           = false
	outputWriter io.Writer = os.Stderr
)

func init() {
	SetupLogger(false, false)
}

// SetupLogger configures the global logger.
//
// Parameters:
//   - debug: When true, enables debug-level logging
//   - structured: When true, outputs JSON-formatted logs; otherwise uses text format
//
// The logger writes to stderr by default.
// This function is safe for concurrent use.
func SetupLogger(debug, structured bool) {
	mu.Lock()
	defer mu.Unlock()

	var level slog.Level
	if debug {
		level = slog.LevelDebug
		currentLevel = LevelDebug
	} else {
		level = slog.LevelInfo
		currentLevel = LevelInfo
	}

	isStructured = structured
	outputWriter = os.Stderr

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: level,
	}

	if structured {
		handler = slog.NewJSONHandler(outputWriter, opts)
	} else {
		handler = slog.NewTextHandler(outputWriter, opts)
	}

	globalLogger = slog.New(handler)
	slog.SetDefault(globalLogger)
}

// SetOutput sets the output writer for the logger.
// This is useful for testing or redirecting logs.
// This function is safe for concurrent use.
func SetOutput(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()

	outputWriter = w
	// Recreate logger with new output (without holding lock again)
	setupLoggerInternal()
}

// setupLoggerInternal is the non-locking version for internal use.
// Caller must hold mu.Lock().
func setupLoggerInternal() {
	var level slog.Level
	if currentLevel == LevelDebug {
		level = slog.LevelDebug
	} else {
		level = slog.LevelInfo
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: level,
	}

	if isStructured {
		handler = slog.NewJSONHandler(outputWriter, opts)
	} else {
		handler = slog.NewTextHandler(outputWriter, opts)
	}

	globalLogger = slog.New(handler)
	slog.SetDefault(globalLogger)
}

// SetupLoggerWithWriter configures the logger with a custom writer.
// This is useful for testing or redirecting logs.
// This function is safe for concurrent use.
func SetupLoggerWithWriter(w io.Writer, debug, structured bool) {
	mu.Lock()
	defer mu.Unlock()

	outputWriter = w

	var level slog.Level
	if debug {
		level = slog.LevelDebug
		currentLevel = LevelDebug
	} else {
		level = slog.LevelInfo
		currentLevel = LevelInfo
	}

	isStructured = structured

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: level,
	}

	if structured {
		handler = slog.NewJSONHandler(outputWriter, opts)
	} else {
		handler = slog.NewTextHandler(outputWriter, opts)
	}

	globalLogger = slog.New(handler)
	slog.SetDefault(globalLogger)
}

// IsDebugEnabled returns true if debug logging is enabled.
// This checks both the programmatic setting and the AZD_DEBUG environment variable.
// This function is safe for concurrent use.
func IsDebugEnabled() bool {
	mu.RLock()
	level := currentLevel
	mu.RUnlock()
	return level == LevelDebug || os.Getenv(EnvDebug) == "true"
}

// Debug logs a debug message with optional key-value pairs.
// Debug messages are only logged when debug mode is enabled.
//
// Example:
//
//	logutil.Debug("processing request", "method", "GET", "path", "/api/users")
func Debug(msg string, args ...any) {
	if IsDebugEnabled() {
		globalLogger.Debug(msg, args...)
	}
}

// Info logs an info message with optional key-value pairs.
//
// Example:
//
//	logutil.Info("server started", "port", 8080)
func Info(msg string, args ...any) {
	globalLogger.Info(msg, args...)
}

// Warn logs a warning message with optional key-value pairs.
//
// Example:
//
//	logutil.Warn("deprecated API called", "endpoint", "/v1/users")
func Warn(msg string, args ...any) {
	globalLogger.Warn(msg, args...)
}

// Error logs an error message with optional key-value pairs.
//
// Example:
//
//	logutil.Error("failed to connect", "error", err, "host", dbHost)
func Error(msg string, args ...any) {
	globalLogger.Error(msg, args...)
}

// ParseLevel parses a string into a Level.
// Valid values are: "debug", "info", "warn", "warning", "error".
// Returns LevelInfo for unrecognized values.
func ParseLevel(s string) Level {
	switch strings.ToLower(s) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// GetLevel returns the current logging level.
// This function is safe for concurrent use.
func GetLevel() Level {
	mu.RLock()
	defer mu.RUnlock()
	return currentLevel
}

// SetLevel sets the logging level programmatically.
// This function is safe for concurrent use.
func SetLevel(level Level) {
	mu.Lock()
	defer mu.Unlock()

	currentLevel = level

	// Map our Level to slog.Level
	var slogLevel slog.Level
	switch level {
	case LevelDebug:
		slogLevel = slog.LevelDebug
	case LevelWarn:
		slogLevel = slog.LevelWarn
	case LevelError:
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: slogLevel,
	}

	if isStructured {
		handler = slog.NewJSONHandler(outputWriter, opts)
	} else {
		handler = slog.NewTextHandler(outputWriter, opts)
	}

	globalLogger = slog.New(handler)
	slog.SetDefault(globalLogger)
}

// Logger returns the underlying slog.Logger for advanced usage.
// This function is safe for concurrent use.
func Logger() *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return globalLogger
}
