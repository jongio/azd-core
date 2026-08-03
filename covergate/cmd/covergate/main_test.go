package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeProfile writes a minimal coverage profile covering one package.
func writeProfile(t *testing.T, dir string, covered bool) string {
	t.Helper()
	hits := "0"
	if covered {
		hits = "1"
	}
	body := "mode: set\nexample.com/m/pkg/a.go:1.1,2.1 1 " + hits + "\n"
	path := filepath.Join(dir, "coverage.out")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return path
}

func TestRunRecordThenGatePasses(t *testing.T) {
	dir := t.TempDir()
	profile := writeProfile(t, dir, true)
	baseline := filepath.Join(dir, "baseline.json")

	var out bytes.Buffer
	args := []string{"-profile", profile, "-baseline", baseline, "-record", "-note", "initial"}
	if err := run(args, &out, &out); err != nil {
		t.Fatalf("record: %v", err)
	}
	if _, err := os.Stat(baseline); err != nil {
		t.Fatalf("baseline not written: %v", err)
	}

	out.Reset()
	if err := run([]string{"-profile", profile, "-baseline", baseline}, &out, &out); err != nil {
		t.Fatalf("gate should pass: %v", err)
	}
	if !strings.Contains(out.String(), "gate passed") {
		t.Errorf("expected pass message, got %q", out.String())
	}
}

func TestRunGateFailsOnRegression(t *testing.T) {
	dir := t.TempDir()
	baseline := filepath.Join(dir, "baseline.json")

	var out bytes.Buffer
	covered := writeProfile(t, dir, true)
	if err := run([]string{"-profile", covered, "-baseline", baseline, "-record"}, &out, &out); err != nil {
		t.Fatalf("record: %v", err)
	}

	// Same package, now uncovered. The ratchet must reject it.
	uncovered := writeProfile(t, dir, false)
	out.Reset()
	err := run([]string{"-profile", uncovered, "-baseline", baseline}, &out, &out)
	if err == nil {
		t.Fatal("expected the gate to fail after coverage dropped")
	}
	if !strings.Contains(err.Error(), "regressed") {
		t.Errorf("expected a regression error, got %v", err)
	}
}

func TestRunGateFailsWithoutBaseline(t *testing.T) {
	dir := t.TempDir()
	profile := writeProfile(t, dir, true)

	var out bytes.Buffer
	err := run([]string{"-profile", profile, "-baseline", filepath.Join(dir, "missing.json")}, &out, &out)
	if err == nil {
		t.Fatal("expected failure when no baseline is recorded")
	}
	if !strings.Contains(err.Error(), "no coverage baseline") {
		t.Errorf("expected a missing-baseline error, got %v", err)
	}
}

func TestRunRejectsBadFlags(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"-nope"}, &out, &out); err == nil {
		t.Fatal("expected an error for an unknown flag")
	}
}

func TestExcludeFlag(t *testing.T) {
	var e excludeFlag
	if err := e.Set("**/gen/**"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := e.Set("**/mock/**"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := e.String(); got != "**/gen/**,**/mock/**" {
		t.Errorf("String() = %q", got)
	}
	if err := e.Set(""); err == nil {
		t.Error("expected an empty pattern to be rejected")
	}
}

func TestRunMissingProfile(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{"-profile", filepath.Join(t.TempDir(), "absent.out")}, &out, &out)
	if err == nil {
		t.Fatal("expected an error for a missing profile")
	}
}
