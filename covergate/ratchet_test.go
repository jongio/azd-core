package covergate

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// reportFrom builds a Report whose packages have the given percentages, using
// 100 statements per package so percentages are exact.
func reportFrom(t *testing.T, pkgs map[string]float64) Report {
	t.Helper()

	var blocks []Block
	for name, pct := range pkgs {
		covered := int(pct)
		if covered > 0 {
			blocks = append(blocks, Block{
				File: name + "/covered.go", Statements: covered, Count: 1,
			})
		}
		if rest := 100 - covered; rest > 0 {
			blocks = append(blocks, Block{
				File: name + "/uncovered.go", Statements: rest, Count: 0,
			})
		}
	}

	report := Aggregate(blocks, Options{})
	return report
}

func TestCheckPassesWhenCoverageHolds(t *testing.T) {
	t.Parallel()

	report := reportFrom(t, map[string]float64{"m/a": 80, "m/b": 90})
	baseline := Baseline{Total: 85, Packages: map[string]float64{"m/a": 80, "m/b": 90}}

	if err := Check(report, baseline, CheckOptions{}); err != nil {
		t.Fatalf("expected the gate to pass: %v", err)
	}
}

func TestCheckPassesWhenCoverageRises(t *testing.T) {
	t.Parallel()

	report := reportFrom(t, map[string]float64{"m/a": 95})
	baseline := Baseline{Total: 80, Packages: map[string]float64{"m/a": 80}}

	if err := Check(report, baseline, CheckOptions{}); err != nil {
		t.Fatalf("rising coverage must pass: %v", err)
	}
}

func TestCheckFailsOnTotalRegression(t *testing.T) {
	t.Parallel()

	report := reportFrom(t, map[string]float64{"m/a": 70})
	baseline := Baseline{Total: 80, Packages: map[string]float64{"m/a": 70}}

	err := Check(report, baseline, CheckOptions{})
	if err == nil {
		t.Fatal("expected a total regression to fail")
	}

	var checkErr *CheckError
	if !errors.As(err, &checkErr) {
		t.Fatalf("expected *CheckError, got %T", err)
	}
	if checkErr.Regressions[0].Scope != "total" {
		t.Errorf("expected the total scope, got %q", checkErr.Regressions[0].Scope)
	}
	if !strings.Contains(err.Error(), "fell below the baseline") {
		t.Errorf("unhelpful message: %v", err)
	}
}

func TestCheckFailsOnPackageRegression(t *testing.T) {
	t.Parallel()

	// The total still holds, but one package slipped. Per-package floors are
	// what stop a refactor from gutting one area and hiding it in the average.
	report := reportFrom(t, map[string]float64{"m/a": 60, "m/b": 100})
	baseline := Baseline{Total: 80, Packages: map[string]float64{"m/a": 80, "m/b": 80}}

	err := Check(report, baseline, CheckOptions{})
	if err == nil {
		t.Fatal("expected a package regression to fail even when the total holds")
	}
	if !strings.Contains(err.Error(), "m/a") {
		t.Errorf("expected m/a to be named: %v", err)
	}
	if strings.Contains(err.Error(), "m/b") {
		t.Errorf("m/b improved and should not be reported: %v", err)
	}
}

func TestCheckTolerance(t *testing.T) {
	t.Parallel()

	report := reportFrom(t, map[string]float64{"m/a": 79})
	baseline := Baseline{Total: 80, Packages: map[string]float64{"m/a": 80}}

	if err := Check(report, baseline, CheckOptions{Tolerance: 1.0}); err != nil {
		t.Fatalf("a drop within tolerance must pass: %v", err)
	}
	if err := Check(report, baseline, CheckOptions{Tolerance: 0.5}); err == nil {
		t.Fatal("a drop beyond tolerance must fail")
	}
}

func TestCheckNewPackageMustMeetBaselineTotal(t *testing.T) {
	t.Parallel()

	report := reportFrom(t, map[string]float64{"m/a": 90, "m/new": 10})
	baseline := Baseline{Total: 80, Packages: map[string]float64{"m/a": 90}}

	err := Check(report, baseline, CheckOptions{})
	if err == nil {
		t.Fatal("a poorly covered new package must fail")
	}
	if !strings.Contains(err.Error(), "new package") {
		t.Errorf("expected the new-package reason: %v", err)
	}

	if err := Check(report, baseline, CheckOptions{IgnoreNewPackages: true}); err != nil {
		// The total is (90+10)/200 = 50%, still below the 80% floor, so the
		// total rule alone should catch it.
		if !strings.Contains(err.Error(), "total") {
			t.Errorf("expected only the total rule to fire: %v", err)
		}
	}
}

func TestCheckWellCoveredNewPackagePasses(t *testing.T) {
	t.Parallel()

	report := reportFrom(t, map[string]float64{"m/a": 90, "m/new": 95})
	baseline := Baseline{Total: 80, Packages: map[string]float64{"m/a": 90}}

	if err := Check(report, baseline, CheckOptions{}); err != nil {
		t.Fatalf("a well covered new package must pass: %v", err)
	}
}

func TestCheckDetectsWidenedExclusions(t *testing.T) {
	t.Parallel()

	// Someone tries to pass the gate by excluding the weak package instead of
	// testing it. The recorded patterns must not silently change.
	report := Aggregate([]Block{
		{File: "m/a/x.go", Statements: 100, Count: 1},
		{File: "m/weak/x.go", Statements: 100, Count: 0},
	}, Options{Exclude: []string{"**/gen/**", "**/weak/**"}})

	baseline := Baseline{
		Total:    50,
		Packages: map[string]float64{"m/a": 100, "m/weak": 0},
		Exclude:  []string{"**/gen/**"},
	}

	checkErr := Check(report, baseline, CheckOptions{})
	if checkErr == nil {
		t.Fatal("widening exclusions must fail the gate")
	}
	if !strings.Contains(checkErr.Error(), "exclude patterns changed") {
		t.Errorf("expected an exclusion drift message: %v", checkErr)
	}

	if err := Check(report, baseline, CheckOptions{AllowExcludeDrift: true}); err != nil {
		t.Fatalf("drift should be permitted when explicitly allowed: %v", err)
	}
}

func TestCheckAllowsIdenticalExclusionsInAnyOrder(t *testing.T) {
	t.Parallel()

	report := Aggregate([]Block{{File: "m/a/x.go", Statements: 1, Count: 1}},
		Options{Exclude: []string{"b", "a"}})

	baseline := Baseline{Total: 100, Packages: map[string]float64{"m/a": 100}, Exclude: []string{"a", "b"}}
	if err := Check(report, baseline, CheckOptions{}); err != nil {
		t.Fatalf("pattern order must not matter: %v", err)
	}
}

func TestCheckPassesWhenCounterModesMatch(t *testing.T) {
	t.Parallel()

	report := reportFrom(t, map[string]float64{"m/a": 80})
	report.Mode = "atomic"
	baseline := Baseline{Total: 80, Packages: map[string]float64{"m/a": 80}, Mode: "atomic"}

	if err := Check(report, baseline, CheckOptions{}); err != nil {
		t.Fatalf("matching modes must pass: %v", err)
	}
}

func TestCheckDetectsCounterModeDrift(t *testing.T) {
	t.Parallel()

	// Atomic reports higher than set for the same tests, so comparing across
	// modes silently inflates coverage and hides real regressions.
	report := reportFrom(t, map[string]float64{"m/a": 80})
	report.Mode = "set"
	baseline := Baseline{Total: 80, Packages: map[string]float64{"m/a": 80}, Mode: "atomic"}

	err := Check(report, baseline, CheckOptions{})
	if err == nil {
		t.Fatal("expected a counter mode mismatch to fail the gate")
	}
	if !strings.Contains(err.Error(), "counter mode changed") {
		t.Fatalf("error should name the mode drift, got: %v", err)
	}
	if !strings.Contains(err.Error(), "atomic") || !strings.Contains(err.Error(), "set") {
		t.Fatalf("error should report both modes, got: %v", err)
	}
}

func TestCheckAllowsCounterModeDriftWhenOptedIn(t *testing.T) {
	t.Parallel()

	report := reportFrom(t, map[string]float64{"m/a": 80})
	report.Mode = "set"
	baseline := Baseline{Total: 80, Packages: map[string]float64{"m/a": 80}, Mode: "atomic"}

	if err := Check(report, baseline, CheckOptions{AllowModeDrift: true}); err != nil {
		t.Fatalf("AllowModeDrift must bypass the mode check: %v", err)
	}
}

func TestCheckRejectsBaselineRecordedBeforeModeTracking(t *testing.T) {
	t.Parallel()

	report := reportFrom(t, map[string]float64{"m/a": 80})
	report.Mode = "atomic"
	baseline := Baseline{Total: 80, Packages: map[string]float64{"m/a": 80}}

	err := Check(report, baseline, CheckOptions{})
	if err == nil {
		t.Fatal("a baseline with no recorded mode must not silently pass")
	}
	if !strings.Contains(err.Error(), "predates counter-mode tracking") {
		t.Fatalf("error should tell the user to re-record, got: %v", err)
	}
}

func TestCheckDetectsPlatformDrift(t *testing.T) {
	t.Parallel()

	// Windows-only branches are unreachable, and so uncovered, on Linux. A
	// baseline recorded on one platform and gated on another reports those
	// gaps as regressions that no amount of new tests can close.
	report := reportFrom(t, map[string]float64{"m/a": 80})
	report.OS = "linux"
	baseline := Baseline{Total: 80, Packages: map[string]float64{"m/a": 80}, OS: "windows"}

	err := Check(report, baseline, CheckOptions{})
	if err == nil {
		t.Fatal("expected a platform mismatch to fail the gate")
	}
	if !strings.Contains(err.Error(), "recorded on a different platform") {
		t.Fatalf("error should name the platform drift, got: %v", err)
	}
	if !strings.Contains(err.Error(), "linux") || !strings.Contains(err.Error(), "windows") {
		t.Fatalf("error should report both platforms, got: %v", err)
	}
}

func TestCheckAllowsPlatformDriftWhenOptedIn(t *testing.T) {
	t.Parallel()

	report := reportFrom(t, map[string]float64{"m/a": 80})
	report.OS = "linux"
	baseline := Baseline{Total: 80, Packages: map[string]float64{"m/a": 80}, OS: "windows"}

	if err := Check(report, baseline, CheckOptions{AllowOSDrift: true}); err != nil {
		t.Fatalf("AllowOSDrift must bypass the platform check: %v", err)
	}
}

func TestCheckRejectsBaselineRecordedBeforePlatformTracking(t *testing.T) {
	t.Parallel()

	report := reportFrom(t, map[string]float64{"m/a": 80})
	report.OS = "linux"
	baseline := Baseline{Total: 80, Packages: map[string]float64{"m/a": 80}}

	err := Check(report, baseline, CheckOptions{})
	if err == nil {
		t.Fatal("a baseline with no recorded platform must not silently pass")
	}
	if !strings.Contains(err.Error(), "predates GOOS tracking") {
		t.Fatalf("error should tell the user to re-record, got: %v", err)
	}
}

func TestProfileStampsMeasuringPlatform(t *testing.T) {
	t.Parallel()

	// Without this stamp an unrecorded platform compares empty against empty
	// and the drift guard passes silently, which is the failure it exists to
	// prevent.
	path := writeProfile(t, t.TempDir(), "m/a/f.go:1.1,2.2 1 1")

	report, err := Profile(path, Options{})
	if err != nil {
		t.Fatalf("Profile: %v", err)
	}
	if report.OS != runtime.GOOS {
		t.Errorf("OS = %q, want %q", report.OS, runtime.GOOS)
	}
}

func TestCheckRegressionsSortedByLargestDrop(t *testing.T) {
	t.Parallel()

	report := reportFrom(t, map[string]float64{"m/small": 78, "m/big": 20})
	baseline := Baseline{
		Total:    49,
		Packages: map[string]float64{"m/small": 80, "m/big": 80},
	}

	err := Check(report, baseline, CheckOptions{})
	if err == nil {
		t.Fatal("expected regressions")
	}

	var checkErr *CheckError
	if !errors.As(err, &checkErr) {
		t.Fatalf("expected *CheckError, got %T", err)
	}
	if checkErr.Regressions[0].Scope != "m/big" {
		t.Errorf("largest drop should sort first, got %q", checkErr.Regressions[0].Scope)
	}
}

func TestBaselineRoundTrip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "coverage-baseline.json")
	want := Baseline{
		Total:    84.9,
		Packages: map[string]float64{"m/a": 90.1},
		Exclude:  []string{"**/gen/**"},
		Note:     "recorded before the azdext migration",
	}

	if err := SaveBaseline(path, want); err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}

	got, err := LoadBaseline(path)
	if err != nil {
		t.Fatalf("LoadBaseline: %v", err)
	}
	if got.Total != want.Total {
		t.Errorf("Total = %v, want %v", got.Total, want.Total)
	}
	if got.Packages["m/a"] != want.Packages["m/a"] {
		t.Errorf("Packages = %v, want %v", got.Packages, want.Packages)
	}
	if got.Note != want.Note {
		t.Errorf("Note = %q, want %q", got.Note, want.Note)
	}
}

func TestSaveBaselineIsDiffFriendly(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "b.json")
	if err := SaveBaseline(path, Baseline{Total: 50}); err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading baseline: %v", err)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Error("baseline should end with a newline")
	}
	if !strings.Contains(string(data), "\n  ") {
		t.Error("baseline should be indented")
	}
	if !json.Valid(data) {
		t.Error("baseline is not valid JSON")
	}
}

func TestSaveBaselineCreatesParentDirectories(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "deeper", "b.json")
	if err := SaveBaseline(path, Baseline{Total: 1}); err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("baseline not written: %v", err)
	}
}

func TestSaveBaselineLeavesNoTempFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := SaveBaseline(filepath.Join(dir, "b.json"), Baseline{Total: 1}); err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading directory: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("temporary file left behind: %s", e.Name())
		}
	}
}

func TestLoadBaselineMissing(t *testing.T) {
	t.Parallel()

	_, err := LoadBaseline(filepath.Join(t.TempDir(), "absent.json"))
	if !errors.Is(err, ErrNoBaseline) {
		t.Fatalf("expected ErrNoBaseline, got %v", err)
	}
}

func TestLoadBaselineMalformed(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "b.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	if _, err := LoadBaseline(path); err == nil {
		t.Fatal("expected a parse error")
	} else if !strings.Contains(err.Error(), "parsing baseline") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadBaselineNormalizesNilPackages(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "b.json")
	if err := os.WriteFile(path, []byte(`{"total": 50}`), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	b, err := LoadBaseline(path)
	if err != nil {
		t.Fatalf("LoadBaseline: %v", err)
	}
	if b.Packages == nil {
		t.Error("Packages should be non-nil so callers can index it safely")
	}
}

func TestBaselineFromReport(t *testing.T) {
	t.Parallel()

	report := reportFrom(t, map[string]float64{"m/a": 80, "m/b": 60})
	report.Mode = "atomic"
	report.OS = "linux"
	b := BaselineFrom(report, []string{"**/gen/**"}, "why")

	if b.Total != 70 {
		t.Errorf("Total = %v, want 70", b.Total)
	}
	if b.Packages["m/a"] != 80 || b.Packages["m/b"] != 60 {
		t.Errorf("Packages = %v", b.Packages)
	}
	if len(b.Exclude) != 1 || b.Exclude[0] != "**/gen/**" {
		t.Errorf("Exclude = %v", b.Exclude)
	}
	if b.Mode != "atomic" {
		t.Errorf("Mode = %q, want atomic", b.Mode)
	}
	if b.OS != "linux" {
		t.Errorf("OS = %q, want linux", b.OS)
	}
	if b.Note != "why" {
		t.Errorf("Note = %q", b.Note)
	}
}

func TestImprovements(t *testing.T) {
	t.Parallel()

	report := reportFrom(t, map[string]float64{"m/a": 95, "m/b": 80})
	baseline := Baseline{Total: 80, Packages: map[string]float64{"m/a": 80, "m/b": 80}}

	gains := Improvements(report, baseline, 1.0)

	var sawA, sawB bool
	for _, g := range gains {
		if g.Scope == "m/a" {
			sawA = true
			if g.Change() != 15 {
				t.Errorf("m/a change = %v, want 15", g.Change())
			}
		}
		if g.Scope == "m/b" {
			sawB = true
		}
	}
	if !sawA {
		t.Error("expected m/a to be reported as improved")
	}
	if sawB {
		t.Error("m/b did not improve and should not be reported")
	}
}

func TestImprovementsIgnoresUnknownPackages(t *testing.T) {
	t.Parallel()

	report := reportFrom(t, map[string]float64{"m/new": 100})
	baseline := Baseline{Total: 50, Packages: map[string]float64{}}

	for _, g := range Improvements(report, baseline, 1.0) {
		if g.Scope == "m/new" {
			t.Error("packages absent from the baseline are not improvements")
		}
	}
}

func TestDeltaChange(t *testing.T) {
	t.Parallel()

	if got := (Delta{Baseline: 80, Current: 84.9}).Change(); got != 4.9 {
		t.Errorf("Change() = %v, want 4.9", got)
	}
	if got := (Delta{Baseline: 84.9, Current: 80}).Change(); got != -4.9 {
		t.Errorf("Change() = %v, want -4.9", got)
	}
}

func TestRegressionString(t *testing.T) {
	t.Parallel()

	drop := Regression{Scope: "m/a", Baseline: 80, Current: 70}
	if !strings.Contains(drop.String(), "-10.0") {
		t.Errorf("expected the delta in the message: %s", drop)
	}

	fresh := Regression{Scope: "m/new", Baseline: 80, Current: 10, New: true}
	if !strings.Contains(fresh.String(), "new package") {
		t.Errorf("expected the new-package wording: %s", fresh)
	}
}

// --- Gate and Record ---

func writeProfile(t *testing.T, dir string, lines ...string) string {
	t.Helper()

	path := filepath.Join(dir, "coverage.out")
	body := "mode: set\n" + strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing profile: %v", err)
	}
	return path
}

func TestRecordFailsOnUnreadableBaseline(t *testing.T) {
	t.Parallel()

	// A corrupt baseline must stop the record, not be quietly replaced. The
	// existing file is the only record of what coverage used to be, so
	// overwriting it on a read error would destroy the evidence.
	dir := t.TempDir()
	profile := writeProfile(t, dir, "m/a/x.go:1.1,2.2 8 1")
	baselineFile := filepath.Join(dir, "coverage-baseline.json")

	if err := os.WriteFile(baselineFile, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("writing corrupt baseline: %v", err)
	}

	var out bytes.Buffer
	err := Record(Config{Profile: profile, BaselineFile: baselineFile, Out: &out}, "why")
	if err == nil {
		t.Fatal("expected Record to fail on a corrupt baseline")
	}
	if !strings.Contains(err.Error(), "parsing baseline") {
		t.Fatalf("error should name the parse failure, got: %v", err)
	}

	data, err := os.ReadFile(baselineFile)
	if err != nil {
		t.Fatalf("reading baseline: %v", err)
	}
	if string(data) != "{not json" {
		t.Errorf("corrupt baseline was overwritten: %q", data)
	}
}

func TestRecordRefusesToOverwriteAnotherPlatformsBaseline(t *testing.T) {
	t.Parallel()

	// Re-recording on a developer machine would replace the numbers CI can
	// reach with numbers only that machine can reach, breaking the gate for
	// everyone.
	dir := t.TempDir()
	profile := writeProfile(t, dir, "m/a/x.go:1.1,2.2 8 1")
	baselineFile := filepath.Join(dir, "coverage-baseline.json")

	foreign := runtime.GOOS + "-not"
	if err := SaveBaseline(baselineFile, Baseline{Total: 80, OS: foreign}); err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}

	var out bytes.Buffer
	err := Record(Config{Profile: profile, BaselineFile: baselineFile, Out: &out}, "why")
	if err == nil {
		t.Fatal("expected Record to refuse a cross-platform overwrite")
	}
	if !strings.Contains(err.Error(), "refusing to re-record") {
		t.Fatalf("error should explain the refusal, got: %v", err)
	}

	reloaded, err := LoadBaseline(baselineFile)
	if err != nil {
		t.Fatalf("LoadBaseline: %v", err)
	}
	if reloaded.OS != foreign {
		t.Errorf("baseline was overwritten: OS = %q, want %q", reloaded.OS, foreign)
	}
}

func TestGateSkipsEnforcementOnForeignPlatformWhenOptedIn(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	profile := writeProfile(t, dir, "m/a/x.go:1.1,2.2 2 0", "m/a/y.go:1.1,2.2 8 0")
	baselineFile := filepath.Join(dir, "coverage-baseline.json")

	// A floor this run cannot possibly meet, to prove the skip is what let it
	// pass rather than the coverage itself.
	if err := SaveBaseline(baselineFile, Baseline{
		Total: 99, Packages: map[string]float64{"m/a": 99}, Mode: "set", OS: runtime.GOOS + "-not",
	}); err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}

	var out bytes.Buffer
	cfg := Config{Profile: profile, BaselineFile: baselineFile, Out: &out, SkipOnForeignOS: true}
	if err := Gate(cfg); err != nil {
		t.Fatalf("Gate should skip rather than fail on a foreign platform: %v", err)
	}
	if !strings.Contains(out.String(), "coverage gate skipped") {
		t.Errorf("the skip must be visible, not silent:\n%s", out.String())
	}

	// Without the opt-in the same run must fail, so CI stays authoritative.
	out.Reset()
	cfg.SkipOnForeignOS = false
	if err := Gate(cfg); err == nil {
		t.Fatal("expected the gate to fail without SkipOnForeignOS")
	}
}

func TestRecordThenGate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	profile := writeProfile(t, dir,
		"m/a/x.go:1.1,2.2 8 1",
		"m/a/y.go:1.1,2.2 2 0",
	)
	baselineFile := filepath.Join(dir, "coverage-baseline.json")

	var out bytes.Buffer
	cfg := Config{Profile: profile, BaselineFile: baselineFile, Out: &out}

	if err := Record(cfg, "initial"); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if !strings.Contains(out.String(), "recorded baseline") {
		t.Errorf("expected confirmation output:\n%s", out.String())
	}

	out.Reset()
	if err := Gate(cfg); err != nil {
		t.Fatalf("Gate should pass immediately after Record: %v", err)
	}
	if !strings.Contains(out.String(), "coverage gate passed") {
		t.Errorf("expected a pass message:\n%s", out.String())
	}
}

func TestGateFailsAfterRegression(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	profile := writeProfile(t, dir, "m/a/x.go:1.1,2.2 10 1")
	baselineFile := filepath.Join(dir, "coverage-baseline.json")

	var out bytes.Buffer
	cfg := Config{Profile: profile, BaselineFile: baselineFile, Out: &out}

	if err := Record(cfg, ""); err != nil {
		t.Fatalf("Record: %v", err)
	}

	// Coverage drops to zero.
	writeProfile(t, dir, "m/a/x.go:1.1,2.2 10 0")

	if err := Gate(cfg); err == nil {
		t.Fatal("expected the gate to fail after a regression")
	}
}

func TestGateWithoutBaselineFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	profile := writeProfile(t, dir, "m/a/x.go:1.1,2.2 1 1")

	var out bytes.Buffer
	err := Gate(Config{
		Profile:      profile,
		BaselineFile: filepath.Join(dir, "absent.json"),
		Out:          &out,
	})

	if err == nil {
		t.Fatal("an unrecorded repository must not appear protected")
	}
	if !strings.Contains(err.Error(), "no coverage baseline recorded") {
		t.Errorf("expected actionable guidance: %v", err)
	}
}

func TestGateReportsImprovements(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	profile := writeProfile(t, dir,
		"m/a/x.go:1.1,2.2 5 1",
		"m/a/y.go:1.1,2.2 5 0",
	)
	baselineFile := filepath.Join(dir, "b.json")

	var out bytes.Buffer
	cfg := Config{Profile: profile, BaselineFile: baselineFile, Out: &out}
	if err := Record(cfg, ""); err != nil {
		t.Fatalf("Record: %v", err)
	}

	writeProfile(t, dir, "m/a/x.go:1.1,2.2 10 1")

	out.Reset()
	if err := Gate(cfg); err != nil {
		t.Fatalf("Gate: %v", err)
	}
	if !strings.Contains(out.String(), "Re-record the baseline") {
		t.Errorf("expected a prompt to lock in the gain:\n%s", out.String())
	}
}

func TestGateMissingProfile(t *testing.T) {
	t.Parallel()

	err := Gate(Config{
		Profile:      filepath.Join(t.TempDir(), "absent.out"),
		BaselineFile: filepath.Join(t.TempDir(), "b.json"),
		Out:          &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("expected an error for a missing profile")
	}
}

func TestConfigDefaults(t *testing.T) {
	t.Parallel()

	var c Config
	if c.profile() != "coverage.out" {
		t.Errorf("profile() = %q", c.profile())
	}
	if c.baselineFile() != DefaultBaselineFile {
		t.Errorf("baselineFile() = %q", c.baselineFile())
	}
	if c.out() != os.Stdout {
		t.Error("out() should default to stdout")
	}

	custom := Config{Profile: "p.out", BaselineFile: "b.json", Out: &bytes.Buffer{}}
	if custom.profile() != "p.out" || custom.baselineFile() != "b.json" {
		t.Error("explicit values should win over defaults")
	}
}
