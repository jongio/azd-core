// Package browser provides secure cross-platform utilities for launching URLs in web browsers.
//
// This package delegates to github.com/pkg/browser for the actual browser launching,
// while adding URL validation, target selection, and a consistent API for callers.
//
// # Key Features
//
//   - Launch URLs in system default browser (via github.com/pkg/browser)
//   - Cross-platform support (Windows/macOS/Linux)
//   - Non-blocking launch
//   - Target options (default browser, system browser, none)
//   - URL validation (http/https only, prevents file:// and javascript:)
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
// Browser launching is handled by github.com/pkg/browser, which supports
// Windows (cmd /c start), macOS (open), and Linux (xdg-open).
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
