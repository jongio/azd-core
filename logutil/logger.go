// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package logutil

import "log/slog"

// ComponentLogger provides component-scoped structured logging.
// It wraps slog.Logger with convenient context chaining.
type ComponentLogger struct {
	slogger   *slog.Logger
	component string
}

// NewLogger creates a Logger scoped to a named component.
func NewLogger(component string) *ComponentLogger {
	return &ComponentLogger{
		slogger:   Logger().With("component", component),
		component: component,
	}
}

// WithService returns a new Logger with the service context added.
func (l *ComponentLogger) WithService(name string) *ComponentLogger {
	return &ComponentLogger{
		slogger:   l.slogger.With("service", name),
		component: l.component,
	}
}

// WithOperation returns a new Logger with the operation context added.
func (l *ComponentLogger) WithOperation(name string) *ComponentLogger {
	return &ComponentLogger{
		slogger:   l.slogger.With("operation", name),
		component: l.component,
	}
}

// WithFields returns a new Logger with additional fields.
// Fields are provided as alternating key-value pairs.
func (l *ComponentLogger) WithFields(fields ...any) *ComponentLogger {
	return &ComponentLogger{
		slogger:   l.slogger.With(fields...),
		component: l.component,
	}
}

// Component returns the component name for this logger.
func (l *ComponentLogger) Component() string {
	return l.component
}

// Debug logs a message at debug level.
func (l *ComponentLogger) Debug(msg string, args ...any) {
	l.slogger.Debug(msg, args...)
}

// Info logs a message at info level.
func (l *ComponentLogger) Info(msg string, args ...any) {
	l.slogger.Info(msg, args...)
}

// Warn logs a message at warn level.
func (l *ComponentLogger) Warn(msg string, args ...any) {
	l.slogger.Warn(msg, args...)
}

// Error logs a message at error level.
func (l *ComponentLogger) Error(msg string, args ...any) {
	l.slogger.Error(msg, args...)
}
