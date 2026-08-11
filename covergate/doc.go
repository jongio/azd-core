// Package covergate parses Go coverage profiles and enforces a ratchet:
// coverage may rise freely but may never fall below a recorded baseline.
//
// It is designed for repositories that want to lock in the coverage they
// already have before starting a large refactor, without committing to an
// absolute target such as 100 percent.
//
// Typical use from a magefile:
//
//	report, err := covergate.Report("coverage.out", covergate.Options{
//		Exclude: []string{"**/gen/**"},
//	})
//	if err != nil {
//		return err
//	}
//	return covergate.Check(report, "coverage-baseline.json", covergate.CheckOptions{})
package covergate
