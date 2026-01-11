// Package fileutil provides secure file system utilities for Azure Developer CLI extensions.
//
// This package offers atomic file operations, JSON handling, and directory management
// with built-in security considerations and retry logic. All operations are designed
// to be safe in concurrent environments and protect against common file system issues.
//
// # Key Features
//
//   - Atomic file writes with retry logic to prevent partial writes
//   - JSON read/write with graceful handling of missing files
//   - Directory creation with secure permissions (0750)
//   - File existence checks (single, any, all patterns)
//   - File extension detection
//   - Text containment checks with security validation
//
// # Security Considerations
//
// The package integrates with the security package to validate paths before reading
// files, preventing path traversal attacks. Files are created with 0644 permissions
// and directories with 0750 permissions to prevent unauthorized access.
//
// # Atomic Write Operations
//
// AtomicWriteJSON and AtomicWriteFile ensure that files are never left in a partial
// state by writing to a temporary file first, then atomically renaming it to the
// target path. This approach includes:
//
//   - Unique temporary file names to avoid concurrent writer collisions
//   - Explicit sync operations to ensure data is flushed to disk
//   - Retry logic (5 attempts with 20ms backoff) for rename operations
//   - Automatic cleanup of temporary files on failure
//
// # Example Usage
//
//	// Write configuration as JSON atomically
//	config := map[string]interface{}{
//	    "version": "1.0",
//	    "services": []string{"api", "web"},
//	}
//	if err := fileutil.AtomicWriteJSON("config.json", config); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Read JSON with graceful handling of missing files
//	var data map[string]interface{}
//	if err := fileutil.ReadJSON("config.json", &data); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Ensure directory exists with secure permissions
//	if err := fileutil.EnsureDir("./cache/data"); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Check if project file exists
//	if fileutil.FileExists(".", "package.json") {
//	    fmt.Println("Node.js project detected")
//	}
//
//	// Check for any C# project file
//	if fileutil.HasFileWithExt(".", ".csproj") {
//	    fmt.Println(".NET project detected")
//	}
//
//	// Check if all required files exist
//	if fileutil.FilesExistAll(".", "package.json", "tsconfig.json") {
//	    fmt.Println("TypeScript project detected")
//	}
//
//	// Check for framework-specific configuration
//	if fileutil.ContainsTextInFile(".", "package.json", "\"next\"") {
//	    fmt.Println("Next.js project detected")
//	}
//
//	// Write raw file data atomically
//	data := []byte("Hello, World!")
//	if err := fileutil.AtomicWriteFile("output.txt", data, 0644); err != nil {
//	    log.Fatal(err)
//	}
//
// # File Permissions
//
// The package uses secure default permissions:
//
//   - DirPermission (0750): rwxr-x--- - Owner can read/write/execute, group can read/execute
//   - FilePermission (0644): rw-r--r-- - Owner can read/write, others can read only
//
// These defaults prevent unauthorized modification while allowing appropriate access.
//
// # Concurrency Safety
//
// Atomic write operations are designed to be safe when called concurrently:
//
//   - Temporary files use unique names based on process ID and timestamp
//   - Rename operations are atomic on most file systems
//   - Retry logic handles transient failures from concurrent access
//
// # Error Handling
//
// Functions return descriptive errors with context:
//
//   - ReadJSON returns nil (no error) for missing files
//   - ContainsText returns false for invalid paths or read errors
//   - Atomic writes clean up temporary files on any failure
//
// All errors are wrapped with context using fmt.Errorf and %w for proper error chains.
package fileutil
