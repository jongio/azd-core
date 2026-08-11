// Package manifest reads and checks azd extension manifests.
//
// It exists because extension.yaml fails quietly. azd's published
// extension.schema.json does not set additionalProperties to false, and the Go
// model azd builds from the manifest silently drops any field it does not
// recognize. A misspelled or invented key therefore validates cleanly, loads
// cleanly, and does nothing. azd-rest carried "minAzdVersion: 1.10.0" for
// several releases believing it declared a floor on the azd host; no such
// field exists, so the extension advertised no constraint at all.
//
// The checks here turn that class of mistake into a test failure. They are
// meant to be called from a single test in each extension repo:
//
//	func TestRequiredAzdVersionTracksTheSdk(t *testing.T) {
//		err := manifest.CheckRequiredAzdVersion(
//			filepath.Join("..", "extension.yaml"),
//			filepath.Join("..", "go.mod"),
//		)
//		require.NoError(t, err)
//	}
package manifest
