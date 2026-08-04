package pack

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"

	"github.com/jongio/azd-core/manifest"
	"gopkg.in/yaml.v3"
)

const (
	packManifestPath = "extension.yaml"
	packGoModPath    = "../go.mod"
)

// packDisqualifyingKeys are the keys whose presence makes azd treat this
// manifest as an ordinary extension rather than a pack. See isExtensionPack in
// azd's extensions/microsoft.azd.extensions/internal/cmd/build.go: pack mode is
// inferred from the absence of executable metadata, so any one of these silently
// demotes the pack to an extension with no code behind it.
var packDisqualifyingKeys = []string{"capabilities", "namespace", "language", "entryPoint"}

// expectedDependencies is the family this pack exists to install. Written out
// rather than derived so that dropping an extension from the pack has to be a
// deliberate edit in two places.
var expectedDependencies = []string{"jongio.azd.app", "jongio.azd.copilot", "jongio.azd.rest"}

// versionFloor matches a floor constraint such as ">= 0.20.0". Exact pins are
// rejected on purpose: the extensions release on their own cadence, and a pin
// would make every extension patch require a pack release to stay installable.
var versionFloor = regexp.MustCompile(`^>=\s*\d+\.\d+\.\d+$`)

type packManifest struct {
	Dependencies []struct {
		ID      string `yaml:"id"`
		Version string `yaml:"version"`
	} `yaml:"dependencies"`
}

func loadPack(t *testing.T) *manifest.Manifest {
	t.Helper()

	m, err := manifest.Load(packManifestPath)
	if err != nil {
		t.Fatalf("load %s: %v", packManifestPath, err)
	}

	return m
}

func TestPackManifestHasSchemaRequiredKeys(t *testing.T) {
	m := loadPack(t)

	for _, key := range []string{"id", "version", "displayName", "description"} {
		if !slices.Contains(m.Keys, key) {
			t.Errorf("extension.schema.json requires %q; manifest has %v", key, m.Keys)
		}
	}

	if m.ID != "jongio.azd" {
		t.Errorf("pack id = %q, want jongio.azd", m.ID)
	}
	if m.Version == "" {
		t.Error("pack version is empty")
	}
}

// TestPackManifestOmitsExecutableMetadata is the load bearing test in this file.
// A pack that grows any of these keys still validates against the schema and
// still publishes; it just stops being a pack.
func TestPackManifestOmitsExecutableMetadata(t *testing.T) {
	m := loadPack(t)

	for _, key := range packDisqualifyingKeys {
		if slices.Contains(m.Keys, key) {
			t.Errorf(
				"manifest declares %q, which makes azd treat it as an extension rather than a pack; "+
					"a pack is defined by having dependencies and no executable metadata",
				key,
			)
		}
	}
}

func TestPackManifestDeclaresTheFamily(t *testing.T) {
	data, err := os.ReadFile(packManifestPath)
	if err != nil {
		t.Fatalf("read %s: %v", packManifestPath, err)
	}

	var parsed packManifest
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse %s: %v", packManifestPath, err)
	}

	if len(parsed.Dependencies) == 0 {
		t.Fatal("pack declares no dependencies, so azd will not treat it as a pack at all")
	}

	var got []string
	for _, dep := range parsed.Dependencies {
		got = append(got, dep.ID)

		if dep.Version == "" {
			t.Errorf("dependency %s has no version constraint, so any version satisfies it", dep.ID)
			continue
		}
		if !versionFloor.MatchString(dep.Version) {
			t.Errorf(
				"dependency %s version %q is not a floor constraint like \">= 1.2.3\"; "+
					"exact pins force a pack release for every extension patch",
				dep.ID, dep.Version,
			)
		}
	}

	slices.Sort(got)
	want := slices.Clone(expectedDependencies)
	slices.Sort(want)

	if !slices.Equal(got, want) {
		t.Errorf("pack dependencies = %v, want %v", got, want)
	}
}

// TestPackRequiredAzdVersionMatchesModule keeps the pack's floor equal to the
// azure-dev module azd-core builds against, the same rule the three extensions
// follow. The pack calls no azd APIs itself, but it installs things that do.
func TestPackRequiredAzdVersionMatchesModule(t *testing.T) {
	if _, err := os.Stat(filepath.Clean(packGoModPath)); err != nil {
		t.Fatalf("go.mod not found at %s: %v", packGoModPath, err)
	}

	if err := manifest.CheckRequiredAzdVersion(packManifestPath, packGoModPath); err != nil {
		t.Error(err)
	}
}

func TestPackManifestHasNoUnknownKeys(t *testing.T) {
	m := loadPack(t)

	if unknown := m.UnknownKeys(); len(unknown) > 0 {
		t.Errorf("manifest has keys not in extension.schema.json: %v", unknown)
	}
}
