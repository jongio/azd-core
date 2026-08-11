//go:build mage

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"

	"github.com/jongio/azd-core/covergate"
)

// coverageConfig is the repository's coverage ratchet. Coverage may rise
// freely but may not fall below the recorded baseline.
//
// COVERAGE_PROFILE overrides the profile path so CI can gate the profile it
// already produced instead of running the suite a second time.
func coverageConfig() covergate.Config {
	profile := os.Getenv("COVERAGE_PROFILE")
	if profile == "" {
		profile = "coverage.out"
	}
	return covergate.Config{
		Profile:      profile,
		BaselineFile: "coverage-baseline.json",
		Check:        covergate.CheckOptions{Tolerance: 0.5},
		// Coverage is measured per platform, and CI records and enforces on
		// ubuntu. A developer on another platform gets the report and a
		// notice rather than a failure they cannot act on.
		SkipOnForeignOS: true,
	}
}

// CoverageGate checks an existing coverage profile against the baseline without
// running the tests. CI uses this after its own test step.
func CoverageGate() error {
	return covergate.Gate(coverageConfig())
}

var Default = All

// All runs lint, test, and vet.
func All() error {
	if err := Lint(); err != nil {
		return err
	}
	if err := Vet(); err != nil {
		return err
	}
	return Test()
}

// Coverage runs the tests and fails if coverage dropped below the baseline.
func Coverage() error {
	if err := Test(); err != nil {
		return err
	}
	fmt.Println("==> Checking coverage against the baseline...")
	return covergate.Gate(coverageConfig())
}

// CoverageRecord re-records the coverage baseline from the current profile.
// Run this only when a coverage change is deliberate, and say why in the
// commit message.
//
// Records from a race-enabled run because that is what CI gates. The race
// detector changes which tests run, since tests can skip themselves under it,
// so a profile recorded without it can record coverage CI is unable to reach
// and the gate then fails on a drop nobody introduced. Ordinary local checking
// stays race-free so it does not require a C toolchain; a plain run covers at
// least as much, so it still clears a baseline recorded under race.
//
// The baseline also belongs to one platform. Platform-specific code is
// unreachable, and so uncovered, everywhere else, so this target refuses to
// overwrite a baseline recorded on a different operating system. Record on the
// platform CI gates, using the container command the refusal prints.
func CoverageRecord() error {
	if err := TestRace(); err != nil {
		return err
	}
	fmt.Println("==> Recording a new coverage baseline...")
	return covergate.Record(coverageConfig(), "recorded by mage coverageRecord under -race, matching CI")
}

// coveragePreflight gates coverage during preflight. TestRace has already
// written the profile by this point, so it does not re-run the tests.
func coveragePreflight() error {
	return covergate.Gate(coverageConfig())
}

// Preflight runs all quality checks before release — mirrors dispatch's preflight pattern.
func Preflight() error {
	fmt.Println("🚀 Running preflight checks...")
	fmt.Println()

	checks := []struct {
		name string
		fn   func() error
	}{
		// Repository hygiene
		{"Checking .gitignore", preflightCheckGitIgnore},
		{"Checking .gitattributes", preflightCheckGitAttributes},

		// Go dependency integrity
		{"Verifying Go modules", preflightModVerify},
		{"Checking go.mod/go.sum tidiness", preflightModTidy},

		// Go code quality
		{"Checking code format (gofmt)", preflightFmtCheck},
		{"Checking strict format (gofumpt)", preflightGofumpt},
		{"Running vet", Vet},
		{"Running linter", Lint},
		{"Running security scan (gosec)", preflightGosec},
		{"Checking for known vulnerabilities", preflightVulncheck},
		{"Checking for dead code", preflightDeadcode},

		// Tests
		{"Running tests with race detector", TestRace},
		{"Checking coverage against the baseline", coveragePreflight},
	}

	for i, check := range checks {
		fmt.Printf("📋 Step %d/%d: %s...\n", i+1, len(checks), check.name)
		if err := check.fn(); err != nil {
			return fmt.Errorf("step %d/%d (%s) failed: %w", i+1, len(checks), check.name, err)
		}
		fmt.Println()
	}

	fmt.Println("✅ All preflight checks passed!")
	fmt.Println("🎉 Ready to ship!")
	return nil
}

// Test runs all tests with coverage.
func Test() error {
	fmt.Println("==> Testing...")
	return sh("go", "test", "-covermode=atomic", "-coverprofile=coverage.out", "./...")
}

// TestRace runs all tests with the race detector enabled.
func TestRace() error {
	fmt.Println("==> Testing with race detector...")
	return sh("go", "test", "-race", "-covermode=atomic", "-coverprofile=coverage.out", "./...")
}

// Lint runs golangci-lint.
func Lint() error {
	fmt.Println("==> Linting...")
	return sh("golangci-lint", "run", "./...")
}

// Vet runs go vet.
func Vet() error {
	fmt.Println("==> Running go vet...")
	return sh("go", "vet", "./...")
}

// Fmt formats all Go source files.
func Fmt() error {
	fmt.Println("==> Formatting...")
	return sh("gofmt", "-w", "-s", ".")
}

// Fmtstrict formats all Go source files with gofumpt (stricter than gofmt).
func Fmtstrict() error {
	fmt.Println("==> Formatting with gofumpt...")
	if _, err := exec.LookPath("gofumpt"); err != nil {
		return fmt.Errorf("gofumpt not installed. Install with: go install mvdan.cc/gofumpt@latest")
	}
	return sh("gofumpt", "-w", ".")
}

// Clean removes build artifacts.
func Clean() error {
	fmt.Println("==> Cleaning...")
	os.RemoveAll("coverage.out")
	return nil
}

// --- Preflight check helpers ---

func preflightCheckGitIgnore() error {
	if _, err := os.Stat(".gitignore"); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf(".gitignore file not found at repository root")
	}
	fmt.Println("   ✅ .gitignore exists")
	return nil
}

func preflightCheckGitAttributes() error {
	if _, err := os.Stat(".gitattributes"); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf(".gitattributes file not found — required for proper line ending configuration")
	}
	fmt.Println("   ✅ .gitattributes exists")
	return nil
}

func preflightModVerify() error {
	if err := sh("go", "mod", "verify"); err != nil {
		return fmt.Errorf("go mod verify failed: %w", err)
	}
	fmt.Println("   ✅ Go module checksums verified")
	return nil
}

func preflightModTidy() error {
	goModBefore, _ := os.ReadFile("go.mod")
	goSumBefore, _ := os.ReadFile("go.sum")

	if err := sh("go", "mod", "tidy"); err != nil {
		return fmt.Errorf("go mod tidy failed: %w", err)
	}

	goModAfter, _ := os.ReadFile("go.mod")
	goSumAfter, _ := os.ReadFile("go.sum")

	if string(goModBefore) != string(goModAfter) || string(goSumBefore) != string(goSumAfter) {
		return fmt.Errorf("go.mod or go.sum changed after 'go mod tidy' — commit the changes")
	}
	fmt.Println("   ✅ go.mod and go.sum are tidy")
	return nil
}

func preflightFmtCheck() error {
	cmd := exec.Command("gofmt", "-l", ".")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("gofmt check failed: %w", err)
	}
	output := strings.TrimSpace(string(out))
	if output != "" {
		fmt.Println("   Unformatted files:")
		for _, f := range strings.Split(output, "\n") {
			fmt.Printf("   • %s\n", f)
		}
		return fmt.Errorf("code is not formatted. Run 'mage fmt' to fix")
	}
	fmt.Println("   ✅ Code is formatted")
	return nil
}

func preflightGofumpt() error {
	if _, err := exec.LookPath("gofumpt"); err != nil {
		fmt.Println("   ⚠️  gofumpt not installed — skipping strict format check")
		fmt.Println("      Install with: go install mvdan.cc/gofumpt@latest")
		return nil
	}
	cmd := exec.Command("gofumpt", "-l", ".")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("gofumpt check failed: %w", err)
	}
	output := strings.TrimSpace(string(out))
	if output != "" {
		fmt.Println("   Files not formatted with gofumpt:")
		for _, f := range strings.Split(output, "\n") {
			fmt.Printf("   • %s\n", f)
		}
		return fmt.Errorf("code is not gofumpt-formatted. Run 'mage fmtstrict' to fix")
	}
	fmt.Println("   ✅ Code is gofumpt-formatted")
	return nil
}

func preflightGosec() error {
	if _, err := exec.LookPath("gosec"); err != nil {
		fmt.Println("   ⚠️  gosec not installed — skipping security scan")
		fmt.Println("      Install with: go install github.com/securego/gosec/v2/cmd/gosec@latest")
		return nil
	}
	if err := sh("gosec", "-quiet", "./..."); err != nil {
		fmt.Println("   ⚠️  Security scan found issues (non-fatal)")
	} else {
		fmt.Println("   ✅ Security scan passed")
	}
	return nil
}

func preflightVulncheck() error {
	if _, err := exec.LookPath("govulncheck"); err != nil {
		fmt.Println("   ⚠️  govulncheck not installed — skipping vulnerability check")
		fmt.Println("      Install with: go install golang.org/x/vuln/cmd/govulncheck@latest")
		return nil
	}
	if err := sh("govulncheck", "./..."); err != nil {
		fmt.Println("   ⚠️  Known vulnerabilities found!")
		return err
	}
	fmt.Println("   ✅ No known vulnerabilities")
	return nil
}

func preflightDeadcode() error {
	if _, err := exec.LookPath("deadcode"); err != nil {
		fmt.Println("   ⚠️  deadcode not installed — skipping dead code check")
		fmt.Println("      Install with: go install golang.org/x/tools/cmd/deadcode@latest")
		return nil
	}
	cmd := exec.Command("deadcode", "-test", "./...")
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		fmt.Println("   ⚠️  Dead code found:")
		fmt.Println(output)
		fmt.Println("   ⚠️  Dead code check completed with findings (non-fatal)")
		return nil
	}
	if output != "" {
		fmt.Println("   ⚠️  Potential dead code found:")
		fmt.Println(output)
	} else {
		fmt.Println("   ✅ No dead code detected")
	}
	return nil
}

// sh runs a command with stdout/stderr connected.
func sh(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = "."
	fmt.Printf("  %s %s\n", name, strings.Join(args, " "))
	return cmd.Run()
}
