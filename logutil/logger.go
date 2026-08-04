// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package logutil

import (
	"log/slog"

	"github.com/azure/azure-dev/cli/azd/pkg/azdext"
)

// ComponentLogger provides component-scoped structured logging.
//
// It wraps azdext.Logger so that a component logger produced here behaves the
// same as one produced by the SDK and can be handed to any API expecting one,
// via AzdextLogger.
//
// Level filtering stays here rather than in the SDK logger. azdext.LoggerOptions
// carries a single Debug boolean, so it can express only debug and info; there
// is no way to ask it for warn or error. The wrapped logger is therefore built
// to pass everything through, and the package level set by SetLevel decides what
// actually gets emitted. Without this, SetLevel(LevelError) would silently keep
// emitting info lines.
type ComponentLogger struct {
	logger    *azdext.Logger
	component string
}

// newComponentLogger builds the wrapped SDK logger against the package's
// current writer and format.
//
// The writer has to be passed explicitly. azdext.NewLogger does not inherit
// from slog.Default despite what its documentation says: it constructs a fresh
// handler and defaults to stderr. Omitting the writer here would make
// SetOutput and SetupLoggerWithWriter silently stop working, which would break
// every test that captures log output and leak those lines to stderr instead.
func newComponentLogger(component string) *ComponentLogger {
	mu.RLock()
	writer := outputWriter
	structured := isStructured
	mu.RUnlock()

	return &ComponentLogger{
		logger: azdext.NewLogger(component, azdext.LoggerOptions{
			// Always pass through; the package level does the filtering.
			Debug:      true,
			Structured: structured,
			Writer:     writer,
		}),
		component: component,
	}
}

// NewLogger creates a Logger scoped to a named component.
//
// The logger captures the output writer and format in effect at the time of the
// call. Call SetOutput or SetupLoggerWithWriter before constructing a logger you
// intend to capture.
func NewLogger(component string) *ComponentLogger {
	return newComponentLogger(component)
}

// enabled reports whether a message at the given level should be emitted.
func (l *ComponentLogger) enabled(level Level) bool {
	if level == LevelDebug {
		return IsDebugEnabled()
	}

	return level >= GetLevel()
}

// derive wraps an already-derived azdext logger, preserving the component name.
func (l *ComponentLogger) derive(logger *azdext.Logger) *ComponentLogger {
	return &ComponentLogger{logger: logger, component: l.component}
}

// WithService returns a new Logger with the service context added.
//
// There is no azdext equivalent. azdext.Logger.WithComponent reparents the
// logger under a new component name, which is a different relationship.
func (l *ComponentLogger) WithService(name string) *ComponentLogger {
	return l.derive(l.logger.With("service", name))
}

// WithOperation returns a new Logger with the operation context added.
func (l *ComponentLogger) WithOperation(name string) *ComponentLogger {
	return l.derive(l.logger.WithOperation(name))
}

// WithFields returns a new Logger with additional fields.
// Fields are provided as alternating key-value pairs.
func (l *ComponentLogger) WithFields(fields ...any) *ComponentLogger {
	return l.derive(l.logger.With(fields...))
}

// WithComponent returns a new Logger under a different component name,
// recording the current one as parent_component.
func (l *ComponentLogger) WithComponent(name string) *ComponentLogger {
	return &ComponentLogger{
		logger:    l.logger.WithComponent(name),
		component: name,
	}
}

// Component returns the component name for this logger.
func (l *ComponentLogger) Component() string {
	return l.component
}

// AzdextLogger returns the wrapped SDK logger, for passing to APIs that accept
// one. Note that it does not apply the package level, so writing through it
// bypasses SetLevel.
func (l *ComponentLogger) AzdextLogger() *azdext.Logger {
	return l.logger
}

// Slogger returns the underlying slog.Logger for libraries that accept one.
// The same caveat as AzdextLogger applies.
func (l *ComponentLogger) Slogger() *slog.Logger {
	return l.logger.Slogger()
}

// Debug logs a message at debug level.
func (l *ComponentLogger) Debug(msg string, args ...any) {
	if l.enabled(LevelDebug) {
		l.logger.Debug(msg, args...)
	}
}

// Info logs a message at info level.
func (l *ComponentLogger) Info(msg string, args ...any) {
	if l.enabled(LevelInfo) {
		l.logger.Info(msg, args...)
	}
}

// Warn logs a message at warn level.
func (l *ComponentLogger) Warn(msg string, args ...any) {
	if l.enabled(LevelWarn) {
		l.logger.Warn(msg, args...)
	}
}

// Error logs a message at error level.
func (l *ComponentLogger) Error(msg string, args ...any) {
	if l.enabled(LevelError) {
		l.logger.Error(msg, args...)
	}
}
