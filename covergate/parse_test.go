package covergate

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseProfile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    []Block
		wantErr string
	}{
		{
			name:  "single covered block",
			input: "mode: set\nexample.com/m/a.go:1.1,2.2 3 1\n",
			want:  []Block{{File: "example.com/m/a.go", Statements: 3, Count: 1}},
		},
		{
			name:  "uncovered block",
			input: "mode: set\nexample.com/m/a.go:1.1,2.2 3 0\n",
			want:  []Block{{File: "example.com/m/a.go", Statements: 3, Count: 0}},
		},
		{
			name:  "atomic mode with counts above one",
			input: "mode: atomic\nexample.com/m/a.go:1.1,2.2 2 47\n",
			want:  []Block{{File: "example.com/m/a.go", Statements: 2, Count: 47}},
		},
		{
			name:  "blank lines are skipped",
			input: "mode: set\n\nexample.com/m/a.go:1.1,2.2 3 1\n\n",
			want:  []Block{{File: "example.com/m/a.go", Statements: 3, Count: 1}},
		},
		{
			name:  "path containing a colon",
			input: "mode: set\nexample.com/m/weird:name.go:1.1,2.2 1 1\n",
			want:  []Block{{File: "example.com/m/weird:name.go", Statements: 1, Count: 1}},
		},
		{
			name:    "missing mode header",
			input:   "example.com/m/a.go:1.1,2.2 3 1\n",
			wantErr: "expected a \"mode:\" header",
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: "no \"mode:\" header",
		},
		{
			name:    "malformed block",
			input:   "mode: set\nthis is not a coverage line\n",
			wantErr: "malformed coverage block",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseProfile(strings.NewReader(tt.input))

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected an error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d blocks, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("block %d: got %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseProfileFileMissing(t *testing.T) {
	t.Parallel()

	_, err := ParseProfileFile(filepath.Join(t.TempDir(), "absent.out"))
	if err == nil {
		t.Fatal("expected an error for a missing profile")
	}
	if !strings.Contains(err.Error(), "opening coverage profile") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBlockPackage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file string
		want string
	}{
		{"example.com/m/pkg/a.go", "example.com/m/pkg"},
		{"example.com/m/a.go", "example.com/m"},
		{"a.go", ""},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()
			if got := (Block{File: tt.file}).Package(); got != tt.want {
				t.Errorf("Package() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBlockCovered(t *testing.T) {
	t.Parallel()

	if (Block{Count: 0}).Covered() {
		t.Error("count 0 should not be covered")
	}
	if !(Block{Count: 1}).Covered() {
		t.Error("count 1 should be covered")
	}
}

func TestStatsPercent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		stats Stats
		want  float64
	}{
		{"no statements is fully covered", Stats{}, 100},
		{"half", Stats{Covered: 1, Statements: 2}, 50},
		{"none", Stats{Covered: 0, Statements: 4}, 0},
		{"all", Stats{Covered: 4, Statements: 4}, 100},
		{"rounds to one decimal", Stats{Covered: 2, Statements: 3}, 66.7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.stats.Percent(); got != tt.want {
				t.Errorf("Percent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAggregateExcludesGeneratedCode(t *testing.T) {
	t.Parallel()

	blocks := []Block{
		{File: "m/src/a.go", Statements: 10, Count: 1},
		{File: "m/src/gen/proto/b.go", Statements: 90, Count: 0},
	}

	withoutExclude := Aggregate(blocks, Options{})
	if got := withoutExclude.Total.Percent(); got != 10 {
		t.Errorf("without exclusions: got %v%%, want 10%%", got)
	}

	withExclude := Aggregate(blocks, Options{Exclude: []string{"**/gen/**"}})
	if got := withExclude.Total.Percent(); got != 100 {
		t.Errorf("with exclusions: got %v%%, want 100%%", got)
	}
	if withExclude.Excluded != 1 {
		t.Errorf("Excluded = %d, want 1", withExclude.Excluded)
	}
}

func TestGlobToRegexp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		glob  string
		path  string
		match bool
	}{
		{"**/gen/**", "m/src/gen/proto/b.go", true},
		{"**/gen/**", "gen/b.go", true},
		{"**/gen/**", "m/src/generated/b.go", false},
		{"**/gen/**", "m/src/a.go", false},
		{"*.go", "a.go", true},
		{"*.go", "sub/a.go", false},
		{"**/*.pb.go", "m/x/a.pb.go", true},
		{"**/*.pb.go", "a.pb.go", true},
		{"?.go", "a.go", true},
		{"?.go", "ab.go", false},
		{"m/**", "m/a/b/c.go", true},
		{"a.go", "a.go", true},
		{"a.go", "b.go", false},
	}

	for _, tt := range tests {
		t.Run(tt.glob+"_"+tt.path, func(t *testing.T) {
			t.Parallel()

			re := globToRegexp(tt.glob)
			if got := re.MatchString(tt.path); got != tt.match {
				t.Errorf("%q matching %q = %v, want %v", tt.glob, tt.path, got, tt.match)
			}
		})
	}
}

func TestGlobToRegexpEscapesRegexMetacharacters(t *testing.T) {
	t.Parallel()

	// A pattern that would be an invalid or surprising regexp if passed
	// through unescaped must be treated as literal text instead.
	re := globToRegexp("m/[weird].go")
	if !re.MatchString("m/[weird].go") {
		t.Error("brackets should match literally")
	}
	if re.MatchString("m/w.go") {
		t.Error("brackets must not be interpreted as a character class")
	}

	if got := globToRegexp("a+b.go"); !got.MatchString("a+b.go") {
		t.Error("plus should match literally")
	}
}

func TestAggregateGroupsByPackage(t *testing.T) {
	t.Parallel()

	report := Aggregate([]Block{
		{File: "m/a/x.go", Statements: 1, Count: 1},
		{File: "m/a/y.go", Statements: 1, Count: 0},
		{File: "m/b/z.go", Statements: 2, Count: 2},
	}, Options{})

	if got := report.Packages["m/a"].Percent(); got != 50 {
		t.Errorf("m/a = %v%%, want 50%%", got)
	}
	if got := report.Packages["m/b"].Percent(); got != 100 {
		t.Errorf("m/b = %v%%, want 100%%", got)
	}
	if got := report.Total.Percent(); got != 75 {
		t.Errorf("total = %v%%, want 75%%", got)
	}

	names := report.PackageNames()
	if len(names) != 2 || names[0] != "m/a" || names[1] != "m/b" {
		t.Errorf("PackageNames() = %v, want [m/a m/b]", names)
	}
}

func TestReportExcludePatternsIsACopy(t *testing.T) {
	t.Parallel()

	report := Aggregate(nil, Options{Exclude: []string{"**/gen/**"}})

	got := report.ExcludePatterns()
	got[0] = "mutated"

	if report.ExcludePatterns()[0] != "**/gen/**" {
		t.Error("ExcludePatterns() exposed internal state to mutation")
	}
}

func TestRound1(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   float64
		want float64
	}{
		{66.666, 66.7},
		{66.644, 66.6},
		{0, 0},
		{100, 100},
		{-1.25, -1.3},
	}

	for _, tt := range tests {
		if got := round1(tt.in); got != tt.want {
			t.Errorf("round1(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestFormatReportOrdersWorstFirst(t *testing.T) {
	t.Parallel()

	report := Aggregate([]Block{
		{File: "m/good/a.go", Statements: 1, Count: 1},
		{File: "m/bad/a.go", Statements: 1, Count: 0},
	}, Options{Exclude: []string{"**/none/**"}})

	out := FormatReport(report)
	if !strings.Contains(out, "total: 50.0%") {
		t.Errorf("missing total line:\n%s", out)
	}
	if strings.Index(out, "m/bad") > strings.Index(out, "m/good") {
		t.Errorf("worst package should be listed first:\n%s", out)
	}
}

func TestFormatReportShowsExcludedCount(t *testing.T) {
	t.Parallel()

	report := Aggregate([]Block{
		{File: "m/gen/a.go", Statements: 1, Count: 0},
	}, Options{Exclude: []string{"**/gen/**"}})

	if !strings.Contains(FormatReport(report), "excluded blocks: 1") {
		t.Errorf("expected an excluded-blocks line:\n%s", FormatReport(report))
	}
}

func TestErrorsAsCheckError(t *testing.T) {
	t.Parallel()

	report := reportFrom(t, map[string]float64{"m/a": 50})
	baseline := Baseline{Total: 90, Packages: map[string]float64{"m/a": 90}}

	err := Check(report, baseline, CheckOptions{})

	var checkErr *CheckError
	if !errors.As(err, &checkErr) {
		t.Fatalf("expected a *CheckError, got %T: %v", err, err)
	}
	if len(checkErr.Regressions) == 0 {
		t.Error("expected at least one regression")
	}
}
