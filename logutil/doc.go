// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

// Package logutil provides a structured logging abstraction built on top of slog.
//
// This package provides a simple, consistent logging interface for azd extensions.
// It wraps the standard library's slog package with convenience functions and
// environment-aware configuration.
//
// # Basic Usage
//
//	// Initialize logging (typically in main.go)
//	logutil.SetupLogger(debug, structured)
//
//	// Log messages at different levels
//	logutil.Debug("processing item", "id", itemID)
//	logutil.Info("operation completed", "duration", elapsed)
//	logutil.Warn("deprecated feature used", "feature", name)
//	logutil.Error("operation failed", "error", err)
//
// # Debug Mode
//
// Debug logging can be enabled in two ways:
//   - Pass debug=true to SetupLogger
//   - Set the AZD_DEBUG environment variable to a truthy value
//
// AZD_DEBUG accepts anything strconv.ParseBool accepts, plus "yes", matching
// the azd extension framework. Unlike the level set through SetupLogger or
// SetLevel, it is consulted on every call, so it can be flipped at runtime.
//
// # Structured Logging
//
// When structured=true is passed to SetupLogger, logs are output as JSON:
//
//	{"time":"2024-01-15T10:30:00Z","level":"INFO","msg":"operation completed","duration":"1.5s"}
//
// Otherwise, logs use a human-readable text format:
//
//	time=2024-01-15T10:30:00Z level=INFO msg="operation completed" duration=1.5s
//
// # Component loggers
//
// NewLogger returns a ComponentLogger that tags every line with a component
// name and supports chaining service, operation, and arbitrary fields:
//
//	log := logutil.NewLogger("deploy").WithService("api").WithOperation("push")
//	log.Info("starting", "revision", rev)
//
// ComponentLogger wraps azdext.Logger and exposes it through AzdextLogger, so a
// logger created here can be handed to any SDK API that expects one.
//
// # Relationship to azdext
//
// Setup delegates to azdext.SetupLogging, but level filtering and the output
// writer are handled here rather than by the SDK, for two reasons.
//
// azdext.LoggerOptions carries a single Debug boolean, so it can express only
// debug and info. Delegating the filter would make SetLevel(LevelWarn) and
// SetLevel(LevelError) silently ineffective.
//
// azdext.NewLogger constructs a fresh handler and defaults to stderr rather than
// inheriting slog.Default, contrary to its documentation. This package passes
// the configured writer explicitly; without that, SetOutput would be a no-op for
// component loggers and their output would leak to stderr.
//
// A ComponentLogger captures the writer and format in effect when it is
// constructed, so configure logging before creating loggers you intend to
// capture.
package logutil
