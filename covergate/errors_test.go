package covergate

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// failingReader returns an error partway through, exercising the scanner's
// error path rather than a clean end of input.
type failingReader struct{ err error }

func (f failingReader) Read([]byte) (int, error) { return 0, f.err }

func TestParseProfileReadError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("disk fell over")

	_, err := ParseProfile(failingReader{err: sentinel})
	if err == nil {
		t.Fatal("expected a read error to surface")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected the underlying error to be wrapped, got %v", err)
	}
}

func TestParseProfileBadStatementCount(t *testing.T) {
	t.Parallel()

	// A count wide enough to overflow int64 passes the regexp but fails
	// conversion, which is the only way to reach these branches.
	huge := strings.Repeat("9", 40)

	_, err := ParseProfile(strings.NewReader("mode: set\nm/a.go:1.1,2.2 " + huge + " 1\n"))
	if err == nil || !strings.Contains(err.Error(), "bad statement count") {
		t.Fatalf("expected a statement count error, got %v", err)
	}

	_, err = ParseProfile(strings.NewReader("mode: set\nm/a.go:1.1,2.2 1 " + huge + "\n"))
	if err == nil || !strings.Contains(err.Error(), "bad execution count") {
		t.Fatalf("expected an execution count error, got %v", err)
	}
}

func TestParseProfileFileWrapsParseErrors(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "bad.out")
	if err := os.WriteFile(path, []byte("not a profile\n"), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	_, err := ParseProfileFile(path)
	if err == nil {
		t.Fatal("expected a parse error")
	}
	if !strings.Contains(err.Error(), "bad.out") {
		t.Errorf("error should name the offending file: %v", err)
	}
}

func TestLoadBaselineReadError(t *testing.T) {
	t.Parallel()

	// A directory reads as a path that exists but cannot be read as a file.
	dir := t.TempDir()

	_, err := LoadBaseline(dir)
	if err == nil {
		t.Fatal("expected an error when the baseline path is a directory")
	}
	if errors.Is(err, ErrNoBaseline) {
		t.Error("a directory is not the same as a missing baseline")
	}
}

func TestSaveBaselineNormalizesNilPackages(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "b.json")
	if err := SaveBaseline(path, Baseline{Total: 10}); err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading baseline: %v", err)
	}
	if !strings.Contains(string(data), `"packages": {}`) {
		t.Errorf("nil packages should serialize as an empty object:\n%s", data)
	}
}

func TestSaveBaselineUnwritableDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permissions are not enforced the same way on Windows")
	}
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "readonly")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatalf("creating fixture: %v", err)
	}

	err := SaveBaseline(filepath.Join(dir, "b.json"), Baseline{Total: 1})
	if err == nil {
		t.Fatal("expected a write failure in an unwritable directory")
	}
}

func TestRecordPropagatesProfileErrors(t *testing.T) {
	t.Parallel()

	err := Record(Config{
		Profile:      filepath.Join(t.TempDir(), "absent.out"),
		BaselineFile: filepath.Join(t.TempDir(), "b.json"),
		Out:          &bytes.Buffer{},
	}, "")

	if err == nil {
		t.Fatal("expected a missing profile to fail")
	}
}

func TestRecordPropagatesSaveErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	profile := writeProfile(t, dir, "m/a/x.go:1.1,2.2 1 1")

	// Pointing the baseline at an existing directory makes the atomic rename
	// fail without needing permission tricks.
	err := Record(Config{
		Profile:      profile,
		BaselineFile: dir,
		Out:          &bytes.Buffer{},
	}, "")

	if err == nil {
		t.Fatal("expected saving over a directory to fail")
	}
}

func TestGatePropagatesBaselineLoadErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	profile := writeProfile(t, dir, "m/a/x.go:1.1,2.2 1 1")

	badBaseline := filepath.Join(dir, "b.json")
	if err := os.WriteFile(badBaseline, []byte("{broken"), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	err := Gate(Config{Profile: profile, BaselineFile: badBaseline, Out: &bytes.Buffer{}})
	if err == nil {
		t.Fatal("expected a malformed baseline to fail")
	}
	if !strings.Contains(err.Error(), "parsing baseline") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGateQuietWhenImprovementIsSmall(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// 500 statements so a single covered block moves the needle by 0.2 points,
	// which is below the one-point reporting threshold.
	profile := writeProfile(t, dir,
		"m/a/x.go:1.1,2.2 400 1",
		"m/a/y.go:1.1,2.2 100 0",
	)
	baselineFile := filepath.Join(dir, "b.json")

	var out bytes.Buffer
	cfg := Config{Profile: profile, BaselineFile: baselineFile, Out: &out}
	if err := Record(cfg, ""); err != nil {
		t.Fatalf("Record: %v", err)
	}

	writeProfile(t, dir,
		"m/a/x.go:1.1,2.2 401 1",
		"m/a/y.go:1.1,2.2 99 0",
	)

	out.Reset()
	if err := Gate(cfg); err != nil {
		t.Fatalf("Gate: %v", err)
	}
	if strings.Contains(out.String(), "Re-record the baseline") {
		t.Errorf("a sub-point gain should not nag:\n%s", out.String())
	}
}

func TestCheckErrorMessageIsActionable(t *testing.T) {
	t.Parallel()

	err := &CheckError{Regressions: []Regression{
		{Scope: "m/a", Baseline: 80, Current: 70},
	}}

	msg := err.Error()
	if !strings.Contains(msg, "coverage regressed in 1 place") {
		t.Errorf("missing a summary count: %s", msg)
	}
	if !strings.Contains(msg, "re-record the baseline") {
		t.Errorf("missing remediation guidance: %s", msg)
	}
}
