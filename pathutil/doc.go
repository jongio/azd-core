// Package pathutil provides cross-platform PATH environment variable management utilities.
//
// This package handles the complexity of PATH manipulation across Windows, macOS, and Linux,
// including registry access on Windows and shell profile handling on Unix systems. It provides
// utilities for finding tools in PATH, refreshing PATH from system sources, searching common
// installation directories, and suggesting installation URLs for missing dependencies.
//
// # Key Features
//
//   - Refresh PATH from system sources (Windows registry, Unix environment)
//   - Find executables in PATH with cross-platform executable detection
//   - Search common system directories for tools not in PATH
//   - Installation suggestions for popular development tools
//   - Automatic handling of Windows executable extensions (.exe)
//
// # Cross-Platform Behavior
//
// On Windows:
//   - Reads PATH from both Machine and User registry variables via PowerShell
//   - Automatically appends .exe extension when searching for executables
//   - Searches common installation directories (Program Files, AppData, etc.)
//   - Combines Machine and User PATH variables in proper order
//
// On Unix (Linux/macOS):
//   - Returns current PATH (cannot source shell profiles from Go process)
//   - Searches standard directories (/usr/local/bin, /usr/bin, /bin, homebrew)
//   - Checks user-specific directories (~/.local/bin, ~/.cargo/bin, ~/go/bin)
//   - Note: Users should restart shell to pick up profile changes
//
// # Example: Finding a Tool
//
//	// Find a tool in the current PATH
//	toolPath := pathutil.FindToolInPath("node")
//	if toolPath == "" {
//	    // Not found in PATH, try common system directories
//	    toolPath = pathutil.SearchToolInSystemPath("node")
//	    if toolPath == "" {
//	        // Still not found, suggest installation
//	        fmt.Println(pathutil.GetInstallSuggestion("node"))
//	    } else {
//	        fmt.Printf("Found node at: %s\n", toolPath)
//	    }
//	} else {
//	    fmt.Printf("Found node in PATH: %s\n", toolPath)
//	}
//
// # Example: Refreshing PATH
//
//	// Refresh PATH after installing a new tool
//	newPath, err := pathutil.RefreshPATH()
//	if err != nil {
//	    log.Printf("Warning: could not refresh PATH: %v", err)
//	    // Continue with current PATH
//	} else {
//	    log.Printf("PATH refreshed: %s", newPath)
//	}
//
// # Example: Complete Tool Detection Flow
//
//	func ensureTool(toolName string) (string, error) {
//	    // First check PATH
//	    if path := pathutil.FindToolInPath(toolName); path != "" {
//	        return path, nil
//	    }
//
//	    // Refresh PATH and try again (especially useful on Windows after install)
//	    if _, err := pathutil.RefreshPATH(); err == nil {
//	        if path := pathutil.FindToolInPath(toolName); path != "" {
//	            return path, nil
//	        }
//	    }
//
//	    // Search common installation directories
//	    if path := pathutil.SearchToolInSystemPath(toolName); path != "" {
//	        return path, nil
//	    }
//
//	    // Not found, provide installation suggestion
//	    return "", fmt.Errorf("tool '%s' not found. %s",
//	        toolName, pathutil.GetInstallSuggestion(toolName))
//	}
//
// # Supported Installation Suggestions
//
// The package provides installation URLs for common development tools:
//
//   - Node.js ecosystem: node, npm, pnpm, yarn
//   - Python ecosystem: python, pip, poetry, pipenv, uv
//   - Containers: docker
//   - Version control: git, gh
//   - Cloud: azd, az, func
//   - Languages: go, dotnet, java
//   - Build tools: mvn, gradle
//   - Development: air, aspire
//
// # Security Considerations
//
// On Windows, PowerShell commands are executed with security flags:
//   - -NoProfile: Don't load user profile scripts
//   - -NonInteractive: Don't wait for user input
//   - -ExecutionPolicy Bypass: Allow script execution for this command only
//
// These flags ensure the operation is safe and doesn't execute untrusted code.
//
// # Limitations
//
//   - On Unix systems, RefreshPATH cannot source shell profiles (inherent Go limitation)
//   - SearchToolInSystemPath only checks predefined common directories
//   - Windows .exe extension is added automatically; other extensions (.cmd, .bat) are not
//   - Installation suggestions are URLs only, not automated installation
package pathutil
