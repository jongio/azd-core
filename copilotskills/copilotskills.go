// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

// Package copilotskills installs agentskills.io-compliant SKILL.md files
// from an embedded filesystem to ~/.copilot/skills/{name}/.
package copilotskills

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jongio/azd-core/fileutil"
)

// namePattern enforces the agentskills.io naming spec: lowercase, hyphens, digits only.
var namePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Install writes embedded skill files to ~/.copilot/skills/{name}/.
// It checks a .version file — if it matches the given version, it skips (no I/O).
// If the version differs or .version doesn't exist, it overwrites all files and writes .version.
func Install(name, version string, skillFS embed.FS, root string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	destDir := filepath.Join(home, ".copilot", "skills", name)
	return installTo(destDir, name, version, skillFS, root)
}

// installTo is the internal implementation that writes skill files to the given destDir.
// Exported Install calls this with ~/.copilot/skills/{name}; tests call it directly with t.TempDir().
func installTo(destDir, name, version string, skillFS embed.FS, root string) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("invalid skill name %q: must be lowercase letters, digits, and hyphens only (agentskills.io spec)", name)
	}

	// Check .version file — skip if it matches
	versionFile := filepath.Join(destDir, ".version")
	existing, err := os.ReadFile(versionFile)
	if err == nil && strings.TrimSpace(string(existing)) == version {
		return nil // already installed at this version
	}

	// Ensure destination directory exists
	if err := fileutil.EnsureDir(destDir); err != nil {
		return fmt.Errorf("failed to create skill directory: %w", err)
	}

	// Walk the embedded FS under root and write all files
	err = fs.WalkDir(skillFS, root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("failed to walk embedded filesystem: %w", walkErr)
		}

		// Compute relative path by stripping the root prefix
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("failed to compute relative path: %w", err)
		}

		destPath := filepath.Join(destDir, rel)

		if d.IsDir() {
			if err := fileutil.EnsureDir(destPath); err != nil {
				return fmt.Errorf("failed to create subdirectory %q: %w", rel, err)
			}
			return nil
		}

		data, err := skillFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read embedded file %q: %w", path, err)
		}

		if err := fileutil.AtomicWriteFile(destPath, data, fileutil.FilePermission); err != nil {
			return fmt.Errorf("failed to write skill file %q: %w", rel, err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Write .version file
	if err := fileutil.AtomicWriteFile(versionFile, []byte(version), fileutil.FilePermission); err != nil {
		return fmt.Errorf("failed to write version file: %w", err)
	}

	return nil
}
