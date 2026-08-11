// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package pathutil

import (
	"os"
	"testing"
)

// TestCombineWindowsPATH covers the Machine/User join order and the empty-side
// cases. This logic used to be inline in refreshWindowsPATH, where it was only
// reachable behind two PowerShell invocations and so could not be exercised
// directly.
func TestCombineWindowsPATH(t *testing.T) {
	tests := []struct {
		name    string
		machine string
		user    string
		want    string
	}{
		{
			name:    "both present joins machine first",
			machine: `C:\Windows\System32`,
			user:    `C:\Users\me\bin`,
			want:    `C:\Windows\System32;C:\Users\me\bin`,
		},
		{
			name:    "machine only has no trailing separator",
			machine: `C:\Windows\System32`,
			user:    "",
			want:    `C:\Windows\System32`,
		},
		{
			name:    "user only has no leading separator",
			machine: "",
			user:    `C:\Users\me\bin`,
			want:    `C:\Users\me\bin`,
		},
		{
			name:    "both empty yields empty",
			machine: "",
			user:    "",
			want:    "",
		},
		{
			name:    "surrounding whitespace is trimmed",
			machine: "  C:\\Windows\\System32\r\n",
			user:    "\tC:\\Users\\me\\bin  ",
			want:    `C:\Windows\System32;C:\Users\me\bin`,
		},
		{
			name:    "whitespace-only side is treated as empty",
			machine: "   \r\n",
			user:    `C:\Users\me\bin`,
			want:    `C:\Users\me\bin`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := combineWindowsPATH(tt.machine, tt.user); got != tt.want {
				t.Errorf("combineWindowsPATH(%q, %q) = %q, want %q", tt.machine, tt.user, got, tt.want)
			}
		})
	}
}

// TestRefreshUnixPATH_Direct calls refreshUnixPATH on every platform. Going
// through RefreshPATH would skip it on Windows, leaving the function untested
// on the primary development platform.
func TestRefreshUnixPATH_Direct(t *testing.T) {
	original := os.Getenv("PATH")
	t.Cleanup(func() {
		_ = os.Setenv("PATH", original)
	})

	const sentinel = "/sentinel/bin"
	if err := os.Setenv("PATH", sentinel); err != nil {
		t.Fatalf("failed to set PATH: %v", err)
	}

	got, err := refreshUnixPATH()
	if err != nil {
		t.Fatalf("refreshUnixPATH() returned error: %v", err)
	}
	if got != sentinel {
		t.Errorf("refreshUnixPATH() = %q, want %q", got, sentinel)
	}
}

// TestRefreshUnixPATH_EmptyPATH confirms an unset PATH is reported as empty
// rather than as an error.
func TestRefreshUnixPATH_EmptyPATH(t *testing.T) {
	original := os.Getenv("PATH")
	t.Cleanup(func() {
		_ = os.Setenv("PATH", original)
	})

	if err := os.Setenv("PATH", ""); err != nil {
		t.Fatalf("failed to set PATH: %v", err)
	}

	got, err := refreshUnixPATH()
	if err != nil {
		t.Fatalf("refreshUnixPATH() returned error: %v", err)
	}
	if got != "" {
		t.Errorf("refreshUnixPATH() = %q, want empty", got)
	}
}
