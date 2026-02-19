// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package browser

import (
	"testing"
	"time"
)

func TestIsValid(t *testing.T) {
	tests := []struct {
		name   string
		target string
		want   bool
	}{
		{"default is valid", "default", true},
		{"system is valid", "system", true},
		{"none is valid", "none", true},
		{"invalid target", "invalid", false},
		{"empty string", "", false},
		{"chrome not valid", "chrome", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValid(tt.target); got != tt.want {
				t.Errorf("IsValid(%q) = %v, want %v", tt.target, got, tt.want)
			}
		})
	}
}

func TestResolveTarget(t *testing.T) {
	tests := []struct {
		name   string
		target Target
		want   Target
	}{
		{
			name:   "none always returns none",
			target: TargetNone,
			want:   TargetNone,
		},
		{
			name:   "default converts to system",
			target: TargetDefault,
			want:   TargetSystem,
		},
		{
			name:   "system stays system",
			target: TargetSystem,
			want:   TargetSystem,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveTarget(tt.target)
			if got != tt.want {
				t.Errorf("ResolveTarget(%q) = %q, want %q", tt.target, got, tt.want)
			}
		})
	}
}

func TestGetTargetDisplayName(t *testing.T) {
	tests := []struct {
		name   string
		target Target
		want   string
	}{
		{
			name:   "system target",
			target: TargetSystem,
			want:   "default browser",
		},
		{
			name:   "default target",
			target: TargetDefault,
			want:   "default browser",
		},
		{
			name:   "none target",
			target: TargetNone,
			want:   "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetTargetDisplayName(tt.target)
			if got != tt.want {
				t.Errorf("GetTargetDisplayName(%q) = %q, want %q", tt.target, got, tt.want)
			}
		})
	}
}

func TestLaunch(t *testing.T) {
	tests := []struct {
		name    string
		opts    LaunchOptions
		wantErr bool
	}{
		{
			name: "none target does not launch",
			opts: LaunchOptions{
				URL:     "http://localhost:4280",
				Target:  TargetNone,
				Timeout: 100 * time.Millisecond,
			},
			wantErr: false,
		},
		{
			name: "valid URL with system target",
			opts: LaunchOptions{
				URL:     "http://localhost:4280",
				Target:  TargetSystem,
				Timeout: 100 * time.Millisecond,
			},
			wantErr: false, // Launch is async, so no error is returned immediately
		},
		{
			name: "invalid URL scheme returns error",
			opts: LaunchOptions{
				URL:     "file:///etc/passwd",
				Target:  TargetSystem,
				Timeout: 100 * time.Millisecond,
			},
			wantErr: true, // Should reject non-http(s) URLs
		},
		{
			name: "ftp URL scheme returns error",
			opts: LaunchOptions{
				URL:     "ftp://example.com/file",
				Target:  TargetSystem,
				Timeout: 100 * time.Millisecond,
			},
			wantErr: true,
		},
		{
			name: "https URL is valid",
			opts: LaunchOptions{
				URL:     "https://localhost:4280",
				Target:  TargetNone, // Use none to avoid actually launching
				Timeout: 100 * time.Millisecond,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Launch(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("Launch() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Give async launch a moment to start
			time.Sleep(50 * time.Millisecond)
		})
	}
}

func TestFormatValidTargets(t *testing.T) {
	result := FormatValidTargets()

	// Check that all valid targets are present
	for _, target := range ValidTargets() {
		targetStr := string(target)
		if !containsString(result, targetStr) {
			t.Errorf("FormatValidTargets() missing %q, got: %q", targetStr, result)
		}
	}
}

func TestValidTargets(t *testing.T) {
	targets := ValidTargets()
	if len(targets) != 3 {
		t.Errorf("ValidTargets() returned %d targets, want 3", len(targets))
	}

	// Check that all expected targets are present
	expectedTargets := map[Target]bool{
		TargetDefault: false,
		TargetSystem:  false,
		TargetNone:    false,
	}

	for _, target := range targets {
		if _, exists := expectedTargets[target]; exists {
			expectedTargets[target] = true
		}
	}

	for target, found := range expectedTargets {
		if !found {
			t.Errorf("ValidTargets() missing %q", target)
		}
	}
}

// Helper functions

func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) &&
		(haystack == needle || len(haystack) > 0 && contains(haystack, needle))
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
