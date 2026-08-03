package covergate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Baseline is the recorded coverage floor for a repository.
type Baseline struct {
	// Total is the minimum acceptable overall percentage.
	Total float64 `json:"total"`
	// Packages maps a package import path to its minimum percentage.
	Packages map[string]float64 `json:"packages"`
	// Exclude records the glob patterns used when the baseline was written,
	// so a later check cannot silently widen them.
	Exclude []string `json:"exclude,omitempty"`
	// Note is free-form context for humans reading the file.
	Note string `json:"note,omitempty"`
}

// ErrNoBaseline is returned when a baseline file does not exist.
var ErrNoBaseline = errors.New("coverage baseline not found")

// ScopeTotal is the scope name used for repository-wide coverage, as opposed
// to a package import path.
const ScopeTotal = "total"

// LoadBaseline reads a baseline file. It returns ErrNoBaseline if the file is
// absent, which callers can treat as "record one now".
//
// The path comes from build tooling rather than untrusted input.
func LoadBaseline(name string) (Baseline, error) {
	data, err := os.ReadFile(name) // #nosec G304 -- build-tool supplied path
	if errors.Is(err, os.ErrNotExist) {
		return Baseline{}, fmt.Errorf("%s: %w", name, ErrNoBaseline)
	}
	if err != nil {
		return Baseline{}, fmt.Errorf("reading baseline: %w", err)
	}

	var b Baseline
	if err := json.Unmarshal(data, &b); err != nil {
		return Baseline{}, fmt.Errorf("parsing baseline %s: %w", name, err)
	}
	if b.Packages == nil {
		b.Packages = map[string]float64{}
	}
	return b, nil
}

// SaveBaseline writes a baseline file atomically, with a trailing newline so
// it stays diff-friendly.
func SaveBaseline(name string, b Baseline) error {
	if b.Packages == nil {
		b.Packages = map[string]float64{}
	}

	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding baseline: %w", err)
	}
	data = append(data, '\n')

	if dir := filepath.Dir(name); dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("creating baseline directory: %w", err)
		}
	}

	tmp, err := os.CreateTemp(filepath.Dir(name), ".coverage-baseline-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temporary baseline: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		if cerr := tmp.Close(); cerr != nil {
			return fmt.Errorf("writing temporary baseline: %w (closing it also failed: %v)", err, cerr)
		}
		return fmt.Errorf("writing temporary baseline: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temporary baseline: %w", err)
	}
	if err := os.Rename(tmpName, name); err != nil {
		return fmt.Errorf("replacing baseline: %w", err)
	}
	return nil
}

// BaselineFrom derives a baseline from a report, keeping the report's own
// exclude patterns for later verification.
func BaselineFrom(r Report, exclude []string, note string) Baseline {
	pkgs := make(map[string]float64, len(r.Packages))
	for name, stats := range r.Packages {
		pkgs[name] = stats.Percent()
	}
	return Baseline{
		Total:    r.Total.Percent(),
		Packages: pkgs,
		Exclude:  exclude,
		Note:     note,
	}
}

// CheckOptions tunes ratchet enforcement.
type CheckOptions struct {
	// Tolerance is the percentage drop allowed before a regression is
	// reported. It absorbs rounding noise from non-deterministic test
	// selection. Zero means any drop fails.
	Tolerance float64
	// IgnoreNewPackages disables the "new package below total" rule. By
	// default a package absent from the baseline must at least meet the
	// baseline total, so new code cannot dilute overall coverage.
	IgnoreNewPackages bool
	// AllowExcludeDrift permits the report's exclude patterns to differ from
	// those recorded in the baseline. By default a mismatch fails, because
	// widening exclusions is the easiest way to fake a passing gate.
	AllowExcludeDrift bool
}

// Delta is a change in coverage for one scope, in either direction.
type Delta struct {
	// Scope is "total" or a package import path.
	Scope string
	// Baseline is the recorded floor.
	Baseline float64
	// Current is the measured value.
	Current float64
	// New reports whether the scope is absent from the baseline.
	New bool
}

// Change returns Current minus Baseline, rounded to one decimal place.
func (d Delta) Change() float64 { return round1(d.Current - d.Baseline) }

// Regression is a coverage drop below the recorded floor.
type Regression Delta

func (r Regression) String() string {
	if r.New {
		return fmt.Sprintf("%s: %.1f%% is below the baseline total of %.1f%% (new package)",
			r.Scope, r.Current, r.Baseline)
	}
	return fmt.Sprintf("%s: %.1f%% fell below the baseline of %.1f%% (-%.1f)",
		r.Scope, r.Current, r.Baseline, round1(r.Baseline-r.Current))
}

// CheckError reports one or more coverage regressions.
type CheckError struct {
	Regressions []Regression
}

func (e *CheckError) Error() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "coverage regressed in %d place(s):", len(e.Regressions))
	for _, r := range e.Regressions {
		fmt.Fprintf(&sb, "\n  %s", r)
	}
	sb.WriteString("\n\nRaise coverage back to the baseline, or if the drop is intentional")
	sb.WriteString("\nre-record the baseline and explain why in the commit message.")
	return sb.String()
}

// Check enforces the ratchet: every scope in the baseline must still meet its
// recorded floor, and by default any package new since the baseline must meet
// the baseline total.
//
// It returns a *CheckError when coverage regressed, so callers can inspect the
// individual regressions with errors.As.
func Check(r Report, b Baseline, opts CheckOptions) error {
	if !opts.AllowExcludeDrift {
		if err := compareExcludes(b.Exclude, r); err != nil {
			return err
		}
	}

	var regressions []Regression

	if current := r.Total.Percent(); current < b.Total-opts.Tolerance {
		regressions = append(regressions, Regression{
			Scope: ScopeTotal, Baseline: b.Total, Current: current,
		})
	}

	for _, name := range r.PackageNames() {
		current := r.Packages[name].Percent()

		floor, known := b.Packages[name]
		if !known {
			if opts.IgnoreNewPackages {
				continue
			}
			if current < b.Total-opts.Tolerance {
				regressions = append(regressions, Regression{
					Scope: name, Baseline: b.Total, Current: current, New: true,
				})
			}
			continue
		}

		if current < floor-opts.Tolerance {
			regressions = append(regressions, Regression{
				Scope: name, Baseline: floor, Current: current,
			})
		}
	}

	if len(regressions) > 0 {
		sort.SliceStable(regressions, func(i, j int) bool {
			di := regressions[i].Baseline - regressions[i].Current
			dj := regressions[j].Baseline - regressions[j].Current
			return di > dj
		})
		return &CheckError{Regressions: regressions}
	}
	return nil
}

// compareExcludes fails when a report was produced with different exclusions
// than the baseline recorded.
func compareExcludes(baseline []string, r Report) error {
	current := r.excludeUsed

	if len(baseline) == 0 && len(current) == 0 {
		return nil
	}

	a := append([]string(nil), baseline...)
	c := append([]string(nil), current...)
	sort.Strings(a)
	sort.Strings(c)

	if strings.Join(a, "\x00") == strings.Join(c, "\x00") {
		return nil
	}
	return fmt.Errorf(
		"exclude patterns changed since the baseline was recorded\n  baseline: %v\n  current:  %v\n"+
			"widening exclusions hides regressions, so re-record the baseline deliberately if this is intended",
		baseline, current)
}

// Improvements lists scopes that now exceed their baseline by at least
// minDelta, so a caller can prompt the user to re-record and lock in the gain.
func Improvements(r Report, b Baseline, minDelta float64) []Delta {
	var gains []Delta

	if delta := r.Total.Percent() - b.Total; delta >= minDelta {
		gains = append(gains, Delta{
			Scope: ScopeTotal, Baseline: b.Total, Current: r.Total.Percent(),
		})
	}

	for _, name := range r.PackageNames() {
		floor, known := b.Packages[name]
		if !known {
			continue
		}
		if delta := r.Packages[name].Percent() - floor; delta >= minDelta {
			gains = append(gains, Delta{
				Scope: name, Baseline: floor, Current: r.Packages[name].Percent(),
			})
		}
	}
	return gains
}
