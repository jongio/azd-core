// Package procutil provides cross-platform process utilities for Azure Developer CLI extensions.
//
// This package offers utilities for managing and querying process state across
// Windows, macOS, and Linux. It handles platform-specific differences in process
// APIs and provides consistent behavior across operating systems.
//
// # Key Features
//
//   - Cross-platform process running check (handles Windows/Unix differences)
//   - Process existence validation using Signal(0) pattern
//   - Documented platform limitations (Windows stale PID detection)
//
// # Cross-Platform Behavior
//
// On Unix (Linux/macOS):
//   - Uses Signal(0) to check if process exists and is accessible
//   - Returns false if process doesn't exist or caller lacks permission
//   - Reliable process existence detection
//
// On Windows:
//   - Signal(0) opens process handle with minimal permissions
//   - May return true for stale PIDs if process hasn't been reaped
//   - Use with caution for critical process lifecycle decisions
//
// # Known Limitations
//
// Windows Stale PID Issue:
//
//	Windows may report a process as running even after it has exited if the
//	process ID has not been recycled. This is a platform limitation, not a
//	bug in this package. For critical process lifecycle management on Windows,
//	consider using alternative approaches (e.g., Windows API OpenProcess,
//	or github.com/shirou/gopsutil library).
//
// Permission Handling:
//
//	IsProcessRunning may return false if caller lacks permission to signal
//	the process, even if the process is running. This is expected Unix behavior.
//
// # Example Usage
//
//	// Check if process is running
//	pid := 12345
//	if procutil.IsProcessRunning(pid) {
//	    fmt.Printf("Process %d is running\n", pid)
//	} else {
//	    fmt.Printf("Process %d is not running or not accessible\n", pid)
//	}
//
//	// Check current process
//	currentPID := os.Getpid()
//	if procutil.IsProcessRunning(currentPID) {
//	    fmt.Println("Current process is running")
//	}
//
//	// Validate PID before attempting operations
//	if procutil.IsProcessRunning(pid) {
//	    // Safe to proceed with process operations
//	    // ... perform operations on process
//	} else {
//	    fmt.Println("Process has exited or is not accessible")
//	}
package procutil
