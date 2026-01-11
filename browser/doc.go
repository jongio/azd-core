// Package browser provides secure cross-platform utilities for launching URLs in web browsers.
//
// This package handles the complexity of opening URLs in the system default browser
// across Windows, macOS, and Linux, with built-in security validation and timeout
// handling. It prevents command injection and validates that only http/https URLs
// are opened.
//
// # Key Features
//
//   - Launch URLs in system default browser
//   - Cross-platform command construction (Windows/macOS/Linux)
//   - Non-blocking launch with configurable timeout
//   - Target options (default browser, system browser, none)
//   - URL validation (http/https only, prevents file:// and javascript:)
//   - Command injection prevention
//
// # Security Considerations
//
// This package enforces strict URL validation to prevent security issues:
//   - Only http:// and https:// schemes are allowed
//   - URLs are validated before passing to shell commands
//   - No user-controlled data is directly interpolated into commands
//
// # Cross-Platform Behavior
//
// On Windows:
//   - Uses "cmd /c start" command with empty title to avoid URL parsing issues
//   - Handles URL escaping for cmd.exe
//
// On macOS:
//   - Uses "open" command
//   - Automatically opens in default browser
//
// On Linux:
//   - Uses "xdg-open" command (most common)
//   - Falls back to "sensible-browser" if xdg-open unavailable
//
// # Browser Targets
//
// The package supports three browser targets:
//   - TargetDefault: Uses the system default browser (alias for TargetSystem)
//   - TargetSystem: Uses the system default browser
//   - TargetNone: Disables browser launching
//
// # Example Usage
//
// Basic usage - launch in default browser:
//
//	err := browser.Launch(browser.LaunchOptions{
//	    URL:    "https://example.com",
//	    Target: browser.TargetDefault,
//	})
//	if err != nil {
//	    log.Printf("Failed to launch browser: %v", err)
//	}
//
// Launch with custom timeout:
//
//	err := browser.Launch(browser.LaunchOptions{
//	    URL:     "https://example.com",
//	    Target:  browser.TargetSystem,
//	    Timeout: 10 * time.Second,
//	})
//	if err != nil {
//	    log.Printf("Failed to launch browser: %v", err)
//	}
//
// Disable browser launching:
//
//	err := browser.Launch(browser.LaunchOptions{
//	    URL:    "https://example.com",
//	    Target: browser.TargetNone,
//	})
//	// No browser will be launched, err will be nil
//
// Validate browser target from user input:
//
//	userInput := "default"
//	if !browser.IsValid(userInput) {
//	    log.Fatalf("Invalid browser target. Valid targets: %s",
//	        browser.FormatValidTargets())
//	}
//
// Get display name for user feedback:
//
//	target := browser.TargetSystem
//	fmt.Printf("Opening in %s...\n", browser.GetTargetDisplayName(target))
//	// Output: Opening in default browser...
//
// # Error Handling
//
// The Launch function is non-blocking and returns immediately. Any errors during
// the actual browser launch are logged to stderr but do not cause the program to fail.
// This is intentional because browser launching is typically a non-critical operation.
//
// However, Launch will return an error immediately for invalid URL schemes:
//
//	err := browser.Launch(browser.LaunchOptions{
//	    URL:    "file:///etc/passwd", // Invalid scheme
//	    Target: browser.TargetSystem,
//	})
//	if err != nil {
//	    // Error: invalid URL scheme: URL must start with http:// or https://
//	}
//
// # Thread Safety
//
// All functions in this package are safe for concurrent use.
package browser
