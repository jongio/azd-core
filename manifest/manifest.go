package manifest

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// KnownKeys are the top-level manifest keys azd actually reads.
//
// Most come from extension.schema.json. "language" is the exception: it is
// absent from the published schema but present on azd's ExtensionSchema model,
// where "azd x build" uses it to pick a toolchain. Leaving it out here would
// flag a field that does real work.
var KnownKeys = []string{
	"capabilities",
	"dependencies",
	"description",
	"displayName",
	"entryPoint",
	"examples",
	"id",
	"language",
	"mcp",
	"namespace",
	"platforms",
	"providers",
	"requiredAzdVersion",
	"tags",
	"usage",
	"version",
}

// Manifest is the subset of extension.yaml these checks need, plus every
// top-level key as it appeared on disk.
type Manifest struct {
	ID                 string
	Version            string
	RequiredAzdVersion string

	// Keys holds every top-level key in file order, including ones azd
	// ignores. UnknownKeys reads this.
	Keys []string
}

// Load parses an extension manifest.
func Load(path string) (*Manifest, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading extension manifest: %w", err)
	}

	// A MapSlice-style decode preserves key order and, more importantly, keeps
	// keys the typed struct would discard.
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if len(document.Content) == 0 || document.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s is not a yaml mapping", path)
	}

	root := document.Content[0]
	parsed := &Manifest{}

	for i := 0; i+1 < len(root.Content); i += 2 {
		key := root.Content[i].Value
		value := root.Content[i+1]

		parsed.Keys = append(parsed.Keys, key)

		switch key {
		case "id":
			parsed.ID = value.Value
		case "version":
			parsed.Version = value.Value
		case "requiredAzdVersion":
			parsed.RequiredAzdVersion = value.Value
		}
	}

	return parsed, nil
}

// UnknownKeys returns the top-level keys azd does not read, excluding any the
// caller names in allowed.
//
// allowed exists so a repo that deliberately keeps an inert key has to say so
// out loud in its test, rather than the key drifting in unnoticed.
func (m *Manifest) UnknownKeys(allowed ...string) []string {
	known := map[string]bool{}
	for _, key := range KnownKeys {
		known[key] = true
	}
	for _, key := range allowed {
		known[key] = true
	}

	var unknown []string
	for _, key := range m.Keys {
		if !known[key] {
			unknown = append(unknown, key)
		}
	}

	sort.Strings(unknown)
	return unknown
}

var azdModuleVersion = regexp.MustCompile(
	`(?m)^\s*github\.com/azure/azure-dev/cli/azd\s+v([0-9]+\.[0-9]+\.[0-9]+)\s*$`)

// AzdModuleVersion returns the azure-dev module version a go.mod requires.
//
// It deliberately rejects pseudo-versions and replace targets by matching only
// a plain semantic version, because a floor derived from a commit hash would
// mean nothing to a user installing the extension.
func AzdModuleVersion(goModPath string) (string, error) {
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return "", fmt.Errorf("reading go.mod: %w", err)
	}

	match := azdModuleVersion.FindStringSubmatch(string(content))
	if match == nil {
		return "", fmt.Errorf(
			"no plain github.com/azure/azure-dev/cli/azd version found in %s", goModPath)
	}

	return match[1], nil
}

// CheckRequiredAzdVersion verifies the manifest declares the azd host version
// the extension is actually built against.
//
// The rule is equality with the azure-dev module in go.mod, not merely a
// satisfiable constraint. An extension compiled against a given azdext calls
// gRPC services that only that azd release serves, so the module version is
// the floor. Declaring anything lower lets azd install the extension onto a
// host that cannot run it, and the user learns about it as a runtime error
// rather than a clear install-time message.
//
// The cost of this rule is that a go.mod bump taken purely for a fix still
// raises the floor. That is the intended trade: a floor that is too high
// inconveniences someone, a floor that is too low breaks them.
func CheckRequiredAzdVersion(manifestPath, goModPath string) error {
	parsed, err := Load(manifestPath)
	if err != nil {
		return err
	}

	moduleVersion, err := AzdModuleVersion(goModPath)
	if err != nil {
		return err
	}

	expected := fmt.Sprintf(">= %s", moduleVersion)

	if parsed.RequiredAzdVersion == "" {
		return fmt.Errorf(
			"%s declares no requiredAzdVersion; it is built against azd %s, so it should say %q",
			manifestPath, moduleVersion, expected)
	}

	if normalizeConstraint(parsed.RequiredAzdVersion) != normalizeConstraint(expected) {
		return fmt.Errorf(
			"%s declares requiredAzdVersion %q but is built against azd %s; expected %q",
			manifestPath, parsed.RequiredAzdVersion, moduleVersion, expected)
	}

	return nil
}

// normalizeConstraint makes ">=1.29.0" and ">= 1.29.0" compare equal, since
// both are valid and the difference is not worth failing a build over.
func normalizeConstraint(constraint string) string {
	return strings.Join(strings.Fields(constraint), "")
}
