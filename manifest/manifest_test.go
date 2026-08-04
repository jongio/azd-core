package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
	return path
}

const goModWithAzd = `module example.com/thing

go 1.24

require (
	github.com/azure/azure-dev/cli/azd v1.29.0
	gopkg.in/yaml.v3 v3.0.1
)
`

func TestLoadReadsTheFieldsTheChecksNeed(t *testing.T) {
	path := write(t, "extension.yaml", `id: jongio.azd.rest
version: 0.5.0
requiredAzdVersion: ">= 1.29.0"
capabilities:
  - metadata
`)

	parsed, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if parsed.ID != "jongio.azd.rest" {
		t.Errorf("ID = %q", parsed.ID)
	}
	if parsed.Version != "0.5.0" {
		t.Errorf("Version = %q", parsed.Version)
	}
	if parsed.RequiredAzdVersion != ">= 1.29.0" {
		t.Errorf("RequiredAzdVersion = %q", parsed.RequiredAzdVersion)
	}
}

// TestLoadKeepsKeysAzdWouldDiscard is the whole reason this package parses
// into a node tree instead of a struct. A typed decode drops unrecognized
// keys, which is exactly the failure being hunted.
func TestLoadKeepsKeysAzdWouldDiscard(t *testing.T) {
	path := write(t, "extension.yaml", `id: a
version: 1.0.0
minAzdVersion: 1.10.0
notAThing: true
`)

	parsed, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	for _, want := range []string{"minAzdVersion", "notAThing"} {
		var found bool
		for _, key := range parsed.Keys {
			if key == want {
				found = true
			}
		}
		if !found {
			t.Errorf("%s was discarded, so UnknownKeys could never report it", want)
		}
	}
}

func TestLoadRejectsNonMappings(t *testing.T) {
	path := write(t, "extension.yaml", "- one\n- two\n")

	if _, err := Load(path); err == nil {
		t.Fatal("a yaml sequence was accepted as a manifest")
	}
}

func TestLoadReportsMissingFiles(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "absent.yaml")); err == nil {
		t.Fatal("a missing manifest was accepted")
	}
}

func TestUnknownKeysFindsInertFields(t *testing.T) {
	path := write(t, "extension.yaml", `id: a
version: 1.0.0
displayName: A
description: B
minAzdVersion: 1.10.0
homepage: https://example.com
`)

	parsed, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	unknown := parsed.UnknownKeys()
	if strings.Join(unknown, ",") != "homepage,minAzdVersion" {
		t.Errorf("UnknownKeys() = %v, want [homepage minAzdVersion]", unknown)
	}
}

func TestUnknownKeysHonorsDeliberateExceptions(t *testing.T) {
	path := write(t, "extension.yaml", `id: a
version: 1.0.0
homepage: https://example.com
minAzdVersion: 1.10.0
`)

	parsed, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	unknown := parsed.UnknownKeys("homepage")
	if strings.Join(unknown, ",") != "minAzdVersion" {
		t.Errorf("UnknownKeys(homepage) = %v, want [minAzdVersion]", unknown)
	}
}

// TestKnownKeysAcceptsAFullManifest guards the list itself. If it drifts from
// the schema, every extension test starts reporting a real key as unknown.
func TestKnownKeysAcceptsAFullManifest(t *testing.T) {
	var builder strings.Builder
	for _, key := range KnownKeys {
		builder.WriteString(key)
		builder.WriteString(": x\n")
	}

	parsed, err := Load(write(t, "extension.yaml", builder.String()))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if unknown := parsed.UnknownKeys(); len(unknown) != 0 {
		t.Errorf("KnownKeys does not accept its own members: %v", unknown)
	}
}

func TestAzdModuleVersionReadsGoMod(t *testing.T) {
	version, err := AzdModuleVersion(write(t, "go.mod", goModWithAzd))
	if err != nil {
		t.Fatalf("AzdModuleVersion: %v", err)
	}

	if version != "1.29.0" {
		t.Errorf("AzdModuleVersion = %q, want 1.29.0", version)
	}
}

// TestAzdModuleVersionRejectsPseudoVersions keeps a commit hash from becoming
// a user-facing version constraint.
func TestAzdModuleVersionRejectsPseudoVersions(t *testing.T) {
	path := write(t, "go.mod", `module example.com/thing

require github.com/azure/azure-dev/cli/azd v0.0.0-20240101120000-abcdef123456
`)

	if _, err := AzdModuleVersion(path); err == nil {
		t.Fatal("a pseudo-version was accepted as a floor")
	}
}

func TestAzdModuleVersionReportsAbsence(t *testing.T) {
	path := write(t, "go.mod", "module example.com/thing\n")

	if _, err := AzdModuleVersion(path); err == nil {
		t.Fatal("a go.mod without the azd module was accepted")
	}
}

func TestCheckRequiredAzdVersionAcceptsAMatch(t *testing.T) {
	manifestPath := write(t, "extension.yaml", "id: a\nversion: 1.0.0\nrequiredAzdVersion: \">= 1.29.0\"\n")
	goModPath := write(t, "go.mod", goModWithAzd)

	if err := CheckRequiredAzdVersion(manifestPath, goModPath); err != nil {
		t.Fatalf("CheckRequiredAzdVersion: %v", err)
	}
}

func TestCheckRequiredAzdVersionIgnoresSpacing(t *testing.T) {
	manifestPath := write(t, "extension.yaml", "id: a\nversion: 1.0.0\nrequiredAzdVersion: \">=1.29.0\"\n")
	goModPath := write(t, "go.mod", goModWithAzd)

	if err := CheckRequiredAzdVersion(manifestPath, goModPath); err != nil {
		t.Fatalf(">=1.29.0 should compare equal to >= 1.29.0: %v", err)
	}
}

// TestCheckRequiredAzdVersionRejectsALowFloor is the case that motivated the
// package. A floor below the module version lets azd install the extension
// onto a host that cannot serve the gRPC calls it makes.
func TestCheckRequiredAzdVersionRejectsALowFloor(t *testing.T) {
	manifestPath := write(t, "extension.yaml", "id: a\nversion: 1.0.0\nrequiredAzdVersion: \">= 1.10.0\"\n")
	goModPath := write(t, "go.mod", goModWithAzd)

	err := CheckRequiredAzdVersion(manifestPath, goModPath)
	if err == nil {
		t.Fatal("a floor 19 minor versions below the module was accepted")
	}
	if !strings.Contains(err.Error(), "1.29.0") {
		t.Errorf("error does not name the expected version: %v", err)
	}
}

func TestCheckRequiredAzdVersionRejectsAHighFloor(t *testing.T) {
	manifestPath := write(t, "extension.yaml", "id: a\nversion: 1.0.0\nrequiredAzdVersion: \">= 1.40.0\"\n")
	goModPath := write(t, "go.mod", goModWithAzd)

	if err := CheckRequiredAzdVersion(manifestPath, goModPath); err == nil {
		t.Fatal("a floor above the module version was accepted, which excludes hosts that would work")
	}
}

// TestCheckRequiredAzdVersionRejectsSilence covers the state all three
// extensions were in: no constraint at all.
func TestCheckRequiredAzdVersionRejectsSilence(t *testing.T) {
	manifestPath := write(t, "extension.yaml", "id: a\nversion: 1.0.0\n")
	goModPath := write(t, "go.mod", goModWithAzd)

	err := CheckRequiredAzdVersion(manifestPath, goModPath)
	if err == nil {
		t.Fatal("a manifest with no requiredAzdVersion was accepted")
	}
	if !strings.Contains(err.Error(), "declares no requiredAzdVersion") {
		t.Errorf("error does not explain the omission: %v", err)
	}
}

func TestCheckRequiredAzdVersionSurfacesLoadFailures(t *testing.T) {
	goModPath := write(t, "go.mod", goModWithAzd)

	if err := CheckRequiredAzdVersion(filepath.Join(t.TempDir(), "absent.yaml"), goModPath); err == nil {
		t.Fatal("a missing manifest was accepted")
	}

	manifestPath := write(t, "extension.yaml", "id: a\nversion: 1.0.0\nrequiredAzdVersion: \">= 1.29.0\"\n")
	if err := CheckRequiredAzdVersion(manifestPath, filepath.Join(t.TempDir(), "absent.mod")); err == nil {
		t.Fatal("a missing go.mod was accepted")
	}
}
