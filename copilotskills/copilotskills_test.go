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

	// Wait a moment so any rewrite would produce a different mod time
	time.Sleep(50 * time.Millisecond)

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

	// Wait so rewrite produces a different mod time
	time.Sleep(50 * time.Millisecond)

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
