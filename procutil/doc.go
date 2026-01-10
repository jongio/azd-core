// Package procutil provides cross-platform process utilities for Azure Developer CLI extensions.
//
// This package offers utilities for managing and querying process state across
// Windows, macOS, Linux, FreeBSD, OpenBSD, Solaris, and AIX. It uses
// github.com/shirou/gopsutil for reliable cross-platform process detection.
//
// # Key Features
//
//   - Cross-platform process running check (Windows/Linux/macOS/BSD/Solaris/AIX)
//   - Reliable process existence validation using gopsutil
//   - Handles stale PIDs correctly on Windows
//   - Consistent behavior across all supported platforms
//
// # Implementation
//
// This package wraps github.com/shirou/gopsutil/v4/process to provide a simple,
// reliable API for process detection. gopsutil uses platform-specific APIs:
//
//   - Windows: Native Windows API (OpenProcess, GetExitCodeProcess)
//   - Linux: /proc filesystem
//   - macOS/BSD: sysctl system calls
//   - Solaris: kstat API
//   - AIX: procfs
//
// This approach provides accurate process detection without the stale PID issues
// that affect os.FindProcess + Signal(0) on Windows.
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
