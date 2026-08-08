// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package logutil

import (
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/azure/azure-dev/cli/azd/pkg/azdext"
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
	EnvDebug          = "AZD_DEBUG"
	envValueYes       = "yes"
	logLevelDebugName = "debug"
	logLevelInfoName  = "info"
	logLevelWarnName  = "warn"
	logLevelErrorName = "error"
)

var (
	mu           sync.RWMutex
	globalLogger *slog.Logger
	currentLevel           = LevelInfo
	isStructured           = false
	outputWriter io.Writer = newSyncWriter(os.Stderr)
)

// writeMu serializes every write this package makes to its configured writer.
//
// It exists because slog's own serialization does not reach across loggers.
// A slog handler locks its own mutex around the write, so loggers derived from
// one handler are serialized against each other. But azdext.NewLogger builds a
// fresh handler each time, so every component logger gets an independent mutex,
// and two components logging at once write to the shared writer concurrently
// with nothing in between.
//
// That would leave the concurrency safety promised by SetOutput and
// SetupLoggerWithWriter conditional on the caller supplying a writer that is
// itself safe for concurrent use. The obvious writer to pass is a bytes.Buffer,
// which is not, so the promise would fail exactly where callers are most likely
// to rely on it.
//
// One package-level mutex rather than one per writer, so that loggers holding a
// writer from before a SetOutput call still serialize against loggers holding
// the current one. Writers can alias, and correctness there is worth more than
// the contention avoided by splitting the lock.
var writeMu sync.Mutex

// syncWriter serializes concurrent writes to the wrapped writer.
//
// Safe to nest: taking writeMu is the only thing it does beyond delegating, and
// no code path holds writeMu while calling back into a handler, so there is no
// lock ordering to invert.
type syncWriter struct {
	w io.Writer
}

// newSyncWriter wraps w so that concurrent writes are serialized. Wrapping an
// already-wrapped writer is avoided so repeated SetOutput calls do not build a
// chain of locks around the same underlying writer.
func newSyncWriter(w io.Writer) io.Writer {
	if _, ok := w.(*syncWriter); ok {
		return w
	}

	return &syncWriter{w: w}
}

func (s *syncWriter) Write(p []byte) (int, error) {
	writeMu.Lock()
	defer writeMu.Unlock()

	return s.w.Write(p)
}

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
	outputWriter = newSyncWriter(os.Stderr)

	applyAzdextLogging(level, structured, outputWriter)
}

// applyAzdextLogging installs the process-wide logger through the SDK and
// mirrors it into globalLogger.
//
// azdext.SetupLogging takes a Debug boolean rather than a level, so it can only
// express debug and info. Warn and error fall back to constructing the handler
// here. Caller must hold mu.
func applyAzdextLogging(level slog.Level, structured bool, w io.Writer) {
	if level == slog.LevelDebug || level == slog.LevelInfo {
		azdext.SetupLogging(azdext.LoggerOptions{
			Debug:      level == slog.LevelDebug,
			Structured: structured,
			Writer:     w,
		})

		globalLogger = slog.Default()

		return
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if structured {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
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

	outputWriter = newSyncWriter(w)
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

	applyAzdextLogging(level, isStructured, outputWriter)
}

// SetupLoggerWithWriter configures the logger with a custom writer.
// This is useful for testing or redirecting logs.
// This function is safe for concurrent use.
func SetupLoggerWithWriter(w io.Writer, debug, structured bool) {
	mu.Lock()
	defer mu.Unlock()

	outputWriter = newSyncWriter(w)

	var level slog.Level
	if debug {
		level = slog.LevelDebug
		currentLevel = LevelDebug
	} else {
		level = slog.LevelInfo
		currentLevel = LevelInfo
	}

	isStructured = structured

	applyAzdextLogging(level, structured, outputWriter)
}

// IsDebugEnabled returns true if debug logging is enabled.
// This checks both the programmatic setting and the AZD_DEBUG environment variable.
// This function is safe for concurrent use.
func IsDebugEnabled() bool {
	mu.RLock()
	level := currentLevel
	mu.RUnlock()

	return level == LevelDebug || isDebugEnv()
}

// isDebugEnv reads AZD_DEBUG using the same rules as the azd extension SDK.
//
// This used to compare against the literal string "true", so AZD_DEBUG=1 and
// AZD_DEBUG=yes silently did nothing even though the framework honors them.
func isDebugEnv() bool {
	v := os.Getenv(EnvDebug)
	if v == "" {
		return false
	}

	b, err := strconv.ParseBool(v)
	if err != nil {
		return strings.EqualFold(v, envValueYes)
	}

	return b
}

// getLogger returns the global logger under read lock for safe concurrent access.
func getLogger() *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return globalLogger
}

// Debug logs a debug message with optional key-value pairs.
// Debug messages are only logged when debug mode is enabled.
//
// Example:
//
//	logutil.Debug("processing request", "method", "GET", "path", "/api/users")
func Debug(msg string, args ...any) {
	if IsDebugEnabled() {
		getLogger().Debug(msg, args...)
	}
}

// Info logs an info message with optional key-value pairs.
//
// Example:
//
//	logutil.Info("server started", "port", 8080)
func Info(msg string, args ...any) {
	getLogger().Info(msg, args...)
}

// Warn logs a warning message with optional key-value pairs.
//
// Example:
//
//	logutil.Warn("deprecated API called", "endpoint", "/v1/users")
func Warn(msg string, args ...any) {
	getLogger().Warn(msg, args...)
}

// Error logs an error message with optional key-value pairs.
//
// Example:
//
//	logutil.Error("failed to connect", "error", err, "host", dbHost)
func Error(msg string, args ...any) {
	getLogger().Error(msg, args...)
}

// ParseLevel parses a string into a Level.
// Valid values are: "debug", "info", "warn", "warning", "error".
// Returns LevelInfo for unrecognized values.
func ParseLevel(s string) Level {
	switch strings.ToLower(s) {
	case logLevelDebugName:
		return LevelDebug
	case logLevelInfoName:
		return LevelInfo
	case logLevelWarnName, "warning":
		return LevelWarn
	case logLevelErrorName:
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

	applyAzdextLogging(slogLevel, isStructured, outputWriter)
}

// Logger returns the underlying slog.Logger for advanced usage.
// This function is safe for concurrent use.
func Logger() *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return globalLogger
}
