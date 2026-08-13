// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package copilotskills

import (
	"embed"
	"os"
	"path/filepath"
	"testing"
	"time"
)

//go:embed testdata/skills/test-skill
var testSkillFS embed.FS

//go:embed testdata/skills/subdir-skill
var subdirSkillFS embed.FS

func TestInstall_FirstTime(t *testing.T) {
	destDir := filepath.Join(t.TempDir(), "test-skill")

	err := installTo(destDir, "test-skill", "1.0.0", testSkillFS, "testdata/skills/test-skill")
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Verify SKILL.md was written
	data, err := os.ReadFile(filepath.Join(destDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("failed to read SKILL.md: %v", err)
	}
	expected, err := testSkillFS.ReadFile("testdata/skills/test-skill/SKILL.md")
	if err != nil {
		t.Fatalf("failed to read expected SKILL.md: %v", err)
	}
	if string(data) != string(expected) {
		t.Errorf("unexpected SKILL.md content: got %q, want %q", string(data), string(expected))
	}

	// Verify .version was written
	ver, err := os.ReadFile(filepath.Join(destDir, ".version"))
	if err != nil {
		t.Fatalf("failed to read .version: %v", err)
	}
	if string(ver) != "1.0.0" {
		t.Errorf("unexpected .version content: %q", string(ver))
	}
}

func TestInstall_SameVersion(t *testing.T) {
	destDir := filepath.Join(t.TempDir(), "test-skill")

	// First install
	if err := installTo(destDir, "test-skill", "1.0.0", testSkillFS, "testdata/skills/test-skill"); err != nil {
		t.Fatalf("first install failed: %v", err)
	}

	// Record mod time of SKILL.md
	info, err := os.Stat(filepath.Join(destDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("failed to stat SKILL.md: %v", err)
	}
	modTime := info.ModTime()

	// Set mod time back so any rewrite would produce a different timestamp
	past := modTime.Add(-1 * time.Second)
	_ = os.Chtimes(filepath.Join(destDir, "SKILL.md"), past, past)
	modTime = past

	// Second install with same version — should skip
	if err := installTo(destDir, "test-skill", "1.0.0", testSkillFS, "testdata/skills/test-skill"); err != nil {
		t.Fatalf("second install failed: %v", err)
	}

	// Verify SKILL.md was NOT rewritten
	info2, err := os.Stat(filepath.Join(destDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("failed to stat SKILL.md after second install: %v", err)
	}
	if !info2.ModTime().Equal(modTime) {
		t.Errorf("SKILL.md was rewritten despite same version (mod time changed from %v to %v)", modTime, info2.ModTime())
	}
}

func TestInstall_DifferentVersion(t *testing.T) {
	destDir := filepath.Join(t.TempDir(), "test-skill")

	// First install
	if err := installTo(destDir, "test-skill", "1.0.0", testSkillFS, "testdata/skills/test-skill"); err != nil {
		t.Fatalf("first install failed: %v", err)
	}

	// Record mod time of SKILL.md
	info, err := os.Stat(filepath.Join(destDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("failed to stat SKILL.md: %v", err)
	}
	modTime := info.ModTime()

	// Set mod time back so rewrite produces a different timestamp
	past2 := modTime.Add(-1 * time.Second)
	_ = os.Chtimes(filepath.Join(destDir, "SKILL.md"), past2, past2)
	modTime = past2

	// Second install with different version — should overwrite
	if err := installTo(destDir, "test-skill", "2.0.0", testSkillFS, "testdata/skills/test-skill"); err != nil {
		t.Fatalf("second install failed: %v", err)
	}

	// Verify SKILL.md was rewritten
	info2, err := os.Stat(filepath.Join(destDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("failed to stat SKILL.md after second install: %v", err)
	}
	if info2.ModTime().Equal(modTime) {
		t.Error("SKILL.md was NOT rewritten despite version change")
	}

	// Verify .version was updated
	ver, err := os.ReadFile(filepath.Join(destDir, ".version"))
	if err != nil {
		t.Fatalf("failed to read .version: %v", err)
	}
	if string(ver) != "2.0.0" {
		t.Errorf("unexpected .version content: %q", string(ver))
	}
}

func TestInstall_InvalidName(t *testing.T) {
	destDir := filepath.Join(t.TempDir(), "bad")

	tests := []struct {
		name string
	}{
		{"BadName"},
		{"bad_name"},
		{"bad name"},
		{"BAD"},
		{"-bad"},
		{"123bad"},
		{""},
	}

	for _, tc := range tests {
		err := installTo(destDir, tc.name, "1.0.0", testSkillFS, "testdata/skills/test-skill")
		if err == nil {
			t.Errorf("expected error for invalid name %q, got nil", tc.name)
		}
	}
}

func TestInstall_InvalidNameCallsInstall(t *testing.T) {
	// Exercise the Install function with an invalid name to trigger validation error
	err := Install("BAD_NAME", "1.0.0", testSkillFS, "testdata/skills/test-skill")
	if err == nil {
		t.Error("Install() with invalid name should return error")
	}
}

func TestInstall_ValidNameCallsInstall(t *testing.T) {
	// Exercise the Install function with a valid name
	// This will install to the real ~/.copilot/skills/ directory
	err := Install("test-skill", "test-version-999", testSkillFS, "testdata/skills/test-skill")
	if err != nil {
		t.Fatalf("Install() failed: %v", err)
	}

	// Verify it was installed
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
	}

	destDir := filepath.Join(home, ".copilot", "skills", "test-skill")
	ver, err := os.ReadFile(filepath.Join(destDir, ".version"))
	if err != nil {
		t.Fatalf("failed to read .version: %v", err)
	}
	if string(ver) != "test-version-999" {
		t.Errorf(".version = %q, want %q", string(ver), "test-version-999")
	}

	// Cleanup
	_ = os.RemoveAll(destDir)
}

func TestInstall_OverwriteWithSubdir(t *testing.T) {
	destDir := filepath.Join(t.TempDir(), "test-skill")

	// Install first
	err := installTo(destDir, "test-skill", "1.0.0", testSkillFS, "testdata/skills/test-skill")
	if err != nil {
		t.Fatalf("first install failed: %v", err)
	}

	// Verify SKILL.md exists
	if _, err := os.Stat(filepath.Join(destDir, "SKILL.md")); err != nil {
		t.Fatalf("SKILL.md should exist: %v", err)
	}

	// Verify .version exists
	ver, err := os.ReadFile(filepath.Join(destDir, ".version"))
	if err != nil {
		t.Fatalf("failed to read .version: %v", err)
	}
	if string(ver) != "1.0.0" {
		t.Errorf(".version = %q, want %q", string(ver), "1.0.0")
	}
}

func TestInstall_WithSubdirectory(t *testing.T) {
	destDir := filepath.Join(t.TempDir(), "subdir-skill")

	err := installTo(destDir, "subdir-skill", "1.0.0", subdirSkillFS, "testdata/skills/subdir-skill")
	if err != nil {
		t.Fatalf("Install with subdirectory failed: %v", err)
	}

	// Verify SKILL.md was written
	data, err := os.ReadFile(filepath.Join(destDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("failed to read SKILL.md: %v", err)
	}
	if string(data) != "# Subdir Skill" {
		t.Errorf("unexpected SKILL.md content: %q", string(data))
	}

	// Verify subdirectory and file were created
	guide, err := os.ReadFile(filepath.Join(destDir, "docs", "guide.md"))
	if err != nil {
		t.Fatalf("failed to read docs/guide.md: %v", err)
	}
	if string(guide) != "# Guide" {
		t.Errorf("unexpected guide.md content: %q", string(guide))
	}

	// Verify .version was written
	ver, err := os.ReadFile(filepath.Join(destDir, ".version"))
	if err != nil {
		t.Fatalf("failed to read .version: %v", err)
	}
	if string(ver) != "1.0.0" {
		t.Errorf(".version = %q, want %q", string(ver), "1.0.0")
	}
}

func TestInstall_VersionCacheHit(t *testing.T) {
	destDir := filepath.Join(t.TempDir(), "cache-hit-skill")

	// First install
	if err := installTo(destDir, "cache-hit-skill", "1.0.0", testSkillFS, "testdata/skills/test-skill"); err != nil {
		t.Fatalf("first install failed: %v", err)
	}

	// Modify SKILL.md to detect if reinstall happens
	sentinel := filepath.Join(destDir, "SKILL.md")
	if err := os.WriteFile(sentinel, []byte("modified"), 0o644); err != nil {
		t.Fatalf("failed to write sentinel: %v", err)
	}

	// Install again with same version — should skip (cache hit)
	if err := installTo(destDir, "cache-hit-skill", "1.0.0", testSkillFS, "testdata/skills/test-skill"); err != nil {
		t.Fatalf("second install failed: %v", err)
	}

	// SKILL.md should still be "modified" because install was skipped
	data, err := os.ReadFile(sentinel)
	if err != nil {
		t.Fatalf("failed to read SKILL.md: %v", err)
	}
	if string(data) != "modified" {
		t.Errorf("SKILL.md was overwritten despite version cache hit: got %q", string(data))
	}
}
