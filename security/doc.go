// Package security provides security validation utilities for Azure Developer CLI extensions.
//
// This package implements critical security checks to prevent common vulnerabilities
// including path traversal attacks, command injection, SSRF, and insecure file
// permissions. All user-provided input should be validated using these utilities
// before use in file operations, command execution, or network requests.
//
// # Key Features
//
// - Path validation (prevents directory traversal attacks)
// - Symbolic link resolution and validation
// - Service name validation (DNS-safe, container-safe identifiers)
// - Package manager name validation (allowlist-based)
// - Script name validation (rejects shell metacharacters and traversal)
// - Container environment detection
// - File permission validation (detects world-writable files)
//
// # Security Model
//
// This package follows defense-in-depth principles:
//  1. Validate all user input at boundaries
//  2. Resolve symbolic links to prevent TOCTOU attacks
//  3. Use allowlists instead of denylists where possible
//  4. Fail securely (deny by default)
//  5. Provide clear error messages without leaking sensitive info
//
// # Path Validation
//
// Path traversal prevention:
//   - Detects ".." sequences in paths
//   - Resolves symbolic links to canonical paths
//   - Validates paths are within expected boundaries
//   - Handles both absolute and relative paths
//
// # Input Sanitization
//
// Service names:
//   - Must start with alphanumeric character
//   - May contain: a-z, A-Z, 0-9, hyphen (-)
//   - DNS-safe (RFC 1123)
//   - Container-safe (Docker naming rules)
//
// Script names:
//   - No shell metacharacters: ; | & $ ` \ < > ( ) { }
//   - Prevents command injection
//   - Safe for use in shell commands
//
// Package managers:
//   - Allowlist: npm, pip, maven, gradle, dotnet, go
//   - Prevents arbitrary package manager execution
//
// # Example Usage
//
//	// Validate user-provided path
//	safePath, err := security.ValidatePath("/base/dir", userPath)
//	if err != nil {
//	    return fmt.Errorf("invalid path: %w", err)
//	}
//
//	// Validate a service name. Pass true to accept an empty name for an
//	// optional parameter.
//	if err := security.ValidateServiceName(name, false); err != nil {
//	    return fmt.Errorf("invalid service name: %w", err)
//	}
//
//	// Reject script names carrying shell metacharacters or traversal
//	if err := security.ValidateScriptName(scriptName); err != nil {
//	    return err
//	}
//
//	// Detect world-writable files
//	if err := security.ValidateFilePermissions("config.json"); err != nil {
//	    if errors.Is(err, security.ErrInsecureFilePermissions) {
//	        log.Warn("File has insecure permissions")
//	    } else {
//	        return err
//	    }
//	}
//
// # Testing
//
// This package requires ≥95% test coverage including:
//   - Attack vectors (path traversal, command injection)
//   - Edge cases (symlinks, permissions, Unicode)
//   - Fuzz testing for input validation
//   - Cross-platform scenarios
package security
