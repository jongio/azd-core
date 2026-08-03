// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package pathutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	toolNode   = "node"
	toolPNPM   = "pnpm"
	toolNPM    = "npm"
	toolPython = "python"
	toolPip    = "pip"
	toolDocker = "docker"
	toolGit    = "git"
	toolDotnet = "dotnet"
	toolAspire = "aspire"
	toolAzd    = "azd"
	toolFunc   = "func"
	toolJava   = "java"
)

// RefreshPATH refreshes the current process's PATH environment variable
// by reading from the system and user environment variables.
// Returns the new PATH value and any error encountered.
func RefreshPATH() (string, error) {
	if runtime.GOOS == "windows" {
		return refreshWindowsPATH()
	}
	return refreshUnixPATH()
}

// refreshWindowsPATH refreshes PATH on Windows by reading from registry.
func refreshWindowsPATH() (string, error) {
	// Get Machine PATH - RemoteSigned is more secure than Bypass
	machinePath, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "RemoteSigned", "-Command",
		"[Environment]::GetEnvironmentVariable('PATH', 'Machine')").Output()
	if err != nil {
		return "", fmt.Errorf("failed to get machine PATH: %w", err)
	}

	// Get User PATH - RemoteSigned is more secure than Bypass
	userPath, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "RemoteSigned", "-Command",
		"[Environment]::GetEnvironmentVariable('PATH', 'User')").Output()
	if err != nil {
		return "", fmt.Errorf("failed to get user PATH: %w", err)
	}

	// Combine and clean up
	newPath := combineWindowsPATH(string(machinePath), string(userPath))

	// Set the new PATH
	if err := os.Setenv("PATH", newPath); err != nil {
		return "", fmt.Errorf("failed to set PATH: %w", err)
	}

	return newPath, nil
}

// combineWindowsPATH joins the Machine and User PATH values in the order Windows
// itself resolves them, skipping either side when it is empty so the result never
// contains a stray separator. Both inputs are trimmed before use.
func combineWindowsPATH(machine, user string) string {
	machine = strings.TrimSpace(machine)
	user = strings.TrimSpace(user)

	switch {
	case machine != "" && user != "":
		return machine + ";" + user
	case machine != "":
		return machine
	default:
		return user
	}
}

// refreshUnixPATH refreshes PATH on Unix-like systems.
// Note: This doesn't actually source shell profiles because Go processes can't
// easily inherit sourced environment variables. Instead, we just return the current PATH.
// The user should restart their shell for permanent changes.
func refreshUnixPATH() (string, error) {
	// On Unix systems, we can't easily refresh the PATH from shell profiles
	// because that requires sourcing files in a shell context.
	// The best we can do is return the current PATH.
	currentPath := os.Getenv("PATH")
	return currentPath, nil
}

// SearchToolInSystemPath searches for a tool in common system directories.
// This is useful for finding tools that are installed but not in the current PATH.
// Returns the full path to the executable if found, empty string otherwise.
func SearchToolInSystemPath(toolName string) string {
	// Add .exe extension on Windows
	exeName := toolName
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(toolName), ".exe") {
		exeName = toolName + ".exe"
	}

	// Define common search paths based on OS
	var searchPaths []string
	if runtime.GOOS == "windows" {
		searchPaths = []string{
			"C:\\Program Files\\nodejs",
			"C:\\Program Files\\Docker\\Docker\\resources\\bin",
			"C:\\Program Files\\Git\\cmd",
			"C:\\Program Files\\Python312",
			"C:\\Program Files\\Python311",
			"C:\\Program Files\\Python310",
			"C:\\Program Files\\dotnet",
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Python"),
			filepath.Join(os.Getenv("APPDATA"), toolNPM),
			filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming", toolNPM),
			filepath.Join(os.Getenv("USERPROFILE"), "go", "bin"), // Go tools installed via 'go install'
		}
	} else {
		homeDir, _ := os.UserHomeDir()
		searchPaths = []string{
			"/usr/local/bin",
			"/usr/bin",
			"/bin",
			"/opt/homebrew/bin",
			"/usr/local/opt",
			filepath.Join(homeDir, ".local", "bin"),
			filepath.Join(homeDir, ".cargo", "bin"),
			filepath.Join(homeDir, "go", "bin"),
		}
	}

	// Search in each path
	for _, dir := range searchPaths {
		fullPath := filepath.Join(dir, exeName)
		// #nosec G703 -- fullPath is built from fixed system directories and a tool executable name.
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath
		}
	}

	return ""
}

// GetInstallSuggestion returns a suggestion for how to install a missing tool.
func GetInstallSuggestion(toolName string) string {
	suggestions := map[string]string{
		toolNode:   "Install from https://nodejs.org/",
		toolPNPM:   "Install from https://pnpm.io/installation",
		toolNPM:    "Install Node.js from https://nodejs.org/",
		"yarn":     "Install from https://yarnpkg.com/getting-started/install",
		toolPython: "Install from https://www.python.org/downloads/",
		toolPip:    "Install Python from https://www.python.org/downloads/",
		"poetry":   "Install from https://python-poetry.org/docs/#installation",
		"uv":       "Install from https://docs.astral.sh/uv/getting-started/installation/",
		"pipenv":   "Install from https://pipenv.pypa.io/en/latest/installation.html",
		toolDocker: "Install Docker Desktop from https://www.docker.com/products/docker-desktop",
		toolGit:    "Install from https://git-scm.com/downloads",
		"go":       "Install from https://go.dev/dl/",
		toolDotnet: "Install from https://dotnet.microsoft.com/download",
		toolAspire: "Install from https://learn.microsoft.com/dotnet/aspire/fundamentals/setup-tooling",
		toolAzd:    "Install from https://aka.ms/install-azd",
		"az":       "Install from https://aka.ms/installazurecli",
		"air":      "Install from https://github.com/air-verse/air#installation",
		toolFunc:   "Install from https://learn.microsoft.com/azure/azure-functions/functions-run-local#install-the-azure-functions-core-tools",
		toolJava:   "Install from https://adoptium.net/",
		"mvn":      "Install from https://maven.apache.org/install.html",
		"gradle":   "Install from https://gradle.org/install/",
		"gh":       "Install from https://cli.github.com/",
	}

	if suggestion, ok := suggestions[toolName]; ok {
		return suggestion
	}
	return fmt.Sprintf("Please install %s manually", toolName)
}
