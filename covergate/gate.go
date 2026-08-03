package covergate

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// DefaultBaselineFile is the conventional baseline location for a repository.
const DefaultBaselineFile = "coverage-baseline.json"

// Config describes a repository's coverage gate.
type Config struct {
	// Profile is the coverage profile to read. Defaults to "coverage.out".
	Profile string
	// BaselineFile is where the recorded floor lives. Defaults to
	// DefaultBaselineFile.
	BaselineFile string
	// Exclude holds glob patterns for code that should not count, such as
	// generated sources.
	Exclude []string
	// Check tunes ratchet enforcement.
	Check CheckOptions
	// Out receives human-readable progress. Defaults to os.Stdout.
	Out io.Writer
}

func (c Config) profile() string {
	if c.Profile == "" {
		return "coverage.out"
	}
	return c.Profile
}

func (c Config) baselineFile() string {
	if c.BaselineFile == "" {
		return DefaultBaselineFile
	}
	return c.BaselineFile
}

func (c Config) out() io.Writer {
	if c.Out == nil {
		return os.Stdout
	}
	return c.Out
}

// Gate parses the profile and enforces the ratchet against the recorded
// baseline. It is the function a magefile's coverage target should call.
//
// If no baseline exists yet, Gate fails with instructions rather than silently
// passing, so an unrecorded repository cannot appear to be protected.
func Gate(c Config) error {
	report, err := Profile(c.profile(), Options{Exclude: c.Exclude})
	if err != nil {
		return err
	}

	fmt.Fprint(c.out(), FormatReport(report))

	baseline, err := LoadBaseline(c.baselineFile())
	if errors.Is(err, ErrNoBaseline) {
		return fmt.Errorf(
			"no coverage baseline recorded at %s\nRun the coverage record target to create one",
			c.baselineFile())
	}
	if err != nil {
		return err
	}

	if err := Check(report, baseline, c.Check); err != nil {
		return err
	}

	fmt.Fprintf(c.out(), "\ncoverage gate passed: %.1f%% (baseline %.1f%%)\n",
		report.Total.Percent(), baseline.Total)

	if gains := Improvements(report, baseline, 1.0); len(gains) > 0 {
		fmt.Fprintf(c.out(),
			"\n%d scope(s) improved by at least 1 point. Re-record the baseline to lock this in:\n",
			len(gains))
		for _, g := range gains {
			fmt.Fprintf(c.out(), "  %s: %.1f%% -> %.1f%% (+%.1f)\n",
				g.Scope, g.Baseline, g.Current, g.Change())
		}
	}
	return nil
}

// Record measures the current profile and writes it as the new baseline.
func Record(c Config, note string) error {
	report, err := Profile(c.profile(), Options{Exclude: c.Exclude})
	if err != nil {
		return err
	}

	baseline := BaselineFrom(report, c.Exclude, note)
	if err := SaveBaseline(c.baselineFile(), baseline); err != nil {
		return err
	}

	fmt.Fprint(c.out(), FormatReport(report))
	fmt.Fprintf(c.out(), "\nrecorded baseline to %s: %.1f%% total across %d package(s)\n",
		c.baselineFile(), baseline.Total, len(baseline.Packages))
	return nil
}
