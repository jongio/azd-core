// Command covergate enforces a coverage ratchet against a recorded baseline.
//
// It exists so CI can gate a coverage profile it already produced, without
// installing mage or re-running the test suite:
//
//	go run github.com/jongio/azd-core/covergate/cmd/covergate \
//	  -profile coverage.txt -baseline coverage-baseline.json
//
// Use -record to write a new baseline instead of checking against one.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jongio/azd-core/covergate"
)

// excludeFlag collects repeated -exclude values.
type excludeFlag []string

func (e *excludeFlag) String() string { return strings.Join(*e, ",") }

func (e *excludeFlag) Set(v string) error {
	if v == "" {
		return fmt.Errorf("exclude pattern must not be empty")
	}
	*e = append(*e, v)
	return nil
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("covergate", flag.ContinueOnError)
	fs.SetOutput(stderr)

	profile := fs.String("profile", "coverage.out", "coverage profile to read")
	baseline := fs.String("baseline", covergate.DefaultBaselineFile, "baseline file")
	tolerance := fs.Float64("tolerance", 0.5, "points of slack absorbed before a drop counts as a regression")
	record := fs.Bool("record", false, "write a new baseline instead of checking against one")
	note := fs.String("note", "", "note stored alongside a recorded baseline")
	ignoreNew := fs.Bool("ignore-new-packages", false, "allow new packages to fall below the baseline total")
	allowDrift := fs.Bool("allow-exclude-drift", false, "allow exclude patterns to differ from the baseline")

	var excludes excludeFlag
	fs.Var(&excludes, "exclude", "glob pattern to exclude, repeatable")

	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg := covergate.Config{
		Profile:      *profile,
		BaselineFile: *baseline,
		Exclude:      excludes,
		Check: covergate.CheckOptions{
			Tolerance:         *tolerance,
			IgnoreNewPackages: *ignoreNew,
			AllowExcludeDrift: *allowDrift,
		},
		Out: stdout,
	}

	if *record {
		return covergate.Record(cfg, *note)
	}
	return covergate.Gate(cfg)
}
