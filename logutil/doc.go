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
//   - Set AZD_DEBUG=true environment variable
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
package logutil
