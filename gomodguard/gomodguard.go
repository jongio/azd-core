// Package gomodguard inspects a go.mod for replace directives that must not
// survive into a release.
//
// An extension under development commonly points azd-core at a local
// checkout so library and extension can move together. That replace is
// invisible to consumers: it is honored only in the main module, so a release
// built with one still resolves to whatever the require line names, and the
// tested code is not the shipped code. A preflight guard catches it.
//
// The parsing lives here rather than in each consumer's magefile because a
// magefile sits behind the mage build tag, which no test job compiles. A
// guard written there cannot be covered by the suite that is meant to protect
// the release.
//
// Typical use from a magefile:
//
//	data, err := os.ReadFile("go.mod")
//	if err != nil {
//		return err
//	}
//	if line := gomodguard.FindReplace(string(data), "jongio/azd-core"); line != "" {
//		return fmt.Errorf("go.mod still replaces azd-core:\n  %s", line)
//	}
package gomodguard

import "strings"

// FindReplace returns the replace directive that rewrites modulePath, or an
// empty string when none does. modulePath may be a full module path or any
// substring that identifies it, such as "jongio/azd-core".
//
// A replace directive is "<module> [version] => <replacement> [version]". The
// leading "replace " keyword appears only on the single-line form; inside a
// "replace ( ... )" block the line starts with the module path directly, and
// either form may carry a version on the left. Matching on the arrow and
// looking only to its left covers all four spellings, and it cannot mistake a
// require line for a replace because a require line never contains an arrow.
//
// The returned line is trimmed of surrounding whitespace so it can be quoted
// directly into an error message.
func FindReplace(gomod, modulePath string) string {
	for _, line := range strings.Split(gomod, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if strings.HasPrefix(line, "//") {
			continue
		}
		arrow := strings.Index(line, "=>")
		if arrow < 0 {
			continue
		}
		if !strings.Contains(line[:arrow], modulePath) {
			continue
		}
		return line
	}
	return ""
}
