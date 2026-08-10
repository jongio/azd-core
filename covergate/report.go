package covergate

import (
	"fmt"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

// Options controls how a coverage profile is aggregated into a Report.
type Options struct {
	// Exclude holds slash-separated glob patterns matched against each
	// block's file path. Matching blocks are dropped before any percentage
	// is computed. Use this for generated code, for example "**/gen/**".
	Exclude []string
}

// Stats is a statement count and its covered subset.
type Stats struct {
	Covered    int `json:"covered"`
	Statements int `json:"statements"`
}

// Percent returns covered statements as a percentage, rounded to one decimal
// place. A unit with no statements is defined as fully covered, which keeps
// empty or excluded-to-nothing packages from failing the gate.
func (s Stats) Percent() float64 {
	if s.Statements == 0 {
		return 100
	}
	return round1(float64(s.Covered) / float64(s.Statements) * 100)
}

// Report is the aggregated coverage of one profile.
type Report struct {
	// Total is coverage across every retained block.
	Total Stats `json:"total"`
	// Packages maps a package import path to its coverage.
	Packages map[string]Stats `json:"packages"`
	// Mode is the counter mode of the profile this report came from, such as
	// "atomic". Aggregate leaves it empty; Profile fills it in from the
	// profile header.
	Mode string `json:"mode,omitempty"`
	// OS is the GOOS the profile was measured on. Aggregate leaves it empty;
	// Profile fills it in from the running process, because a profile is only
	// ever produced by tests running on the current platform.
	OS string `json:"os,omitempty"`
	// Excluded is the number of blocks dropped by the Exclude patterns.
	Excluded int `json:"-"`

	// excludeUsed records the patterns this report was built with so the
	// ratchet can detect exclusions being widened to fake a pass.
	excludeUsed []string
}

// ExcludePatterns returns the glob patterns used to build this report.
func (r Report) ExcludePatterns() []string {
	return append([]string(nil), r.excludeUsed...)
}

// PackageNames returns the report's package paths in sorted order.
func (r Report) PackageNames() []string {
	names := make([]string, 0, len(r.Packages))
	for name := range r.Packages {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Aggregate folds blocks into a Report, applying opts.Exclude.
func Aggregate(blocks []Block, opts Options) Report {
	patterns := make([]*regexp.Regexp, 0, len(opts.Exclude))
	for _, glob := range opts.Exclude {
		patterns = append(patterns, globToRegexp(glob))
	}

	report := Report{
		Packages:    map[string]Stats{},
		excludeUsed: append([]string(nil), opts.Exclude...),
	}

	for _, b := range blocks {
		if matchesAny(patterns, b.File) {
			report.Excluded++
			continue
		}

		covered := 0
		if b.Covered() {
			covered = b.Statements
		}

		report.Total.Statements += b.Statements
		report.Total.Covered += covered

		pkg := b.Package()
		stats := report.Packages[pkg]
		stats.Statements += b.Statements
		stats.Covered += covered
		report.Packages[pkg] = stats
	}

	return report
}

// Profile parses a coverage profile file and aggregates it in one step,
// carrying the profile's counter mode and the measuring platform onto the
// report.
func Profile(profilePath string, opts Options) (Report, error) {
	data, err := ParseProfileFile(profilePath)
	if err != nil {
		return Report{}, err
	}
	report := Aggregate(data.Blocks, opts)
	report.Mode = data.Mode
	report.OS = runtime.GOOS
	return report, nil
}

func matchesAny(patterns []*regexp.Regexp, file string) bool {
	for _, re := range patterns {
		if re.MatchString(file) {
			return true
		}
	}
	return false
}

// round1 rounds to one decimal place, away from zero at the midpoint.
func round1(v float64) float64 {
	scaled := v * 10
	if scaled >= 0 {
		scaled = float64(int64(scaled + 0.5))
	} else {
		scaled = float64(int64(scaled - 0.5))
	}
	return scaled / 10
}

// FormatReport renders a human-readable summary, worst packages first.
func FormatReport(r Report) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "total: %.1f%% (%d/%d statements)\n",
		r.Total.Percent(), r.Total.Covered, r.Total.Statements)
	if r.Excluded > 0 {
		fmt.Fprintf(&sb, "excluded blocks: %d\n", r.Excluded)
	}

	names := r.PackageNames()
	sort.SliceStable(names, func(i, j int) bool {
		return r.Packages[names[i]].Percent() < r.Packages[names[j]].Percent()
	})
	for _, name := range names {
		s := r.Packages[name]
		fmt.Fprintf(&sb, "  %6.1f%%  %s\n", s.Percent(), name)
	}
	return sb.String()
}
