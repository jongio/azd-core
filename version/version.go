// Package version provides shared version information and a reusable version
// command for azd extensions, eliminating duplicated version boilerplate.
package version

import "fmt"

// Info holds version information for an extension.
type Info struct {
	Version     string `json:"version"`
	BuildDate   string `json:"buildDate"`
	GitCommit   string `json:"gitCommit"`
	ExtensionID string `json:"extensionId"`
	Name        string `json:"name"`
}

// New creates a new Info with default values. Version, BuildDate, GitCommit
// are expected to be set via ldflags at build time.
func New(extensionID, name string) *Info {
	return &Info{
		Version:     "0.0.0-dev",
		BuildDate:   "unknown",
		GitCommit:   "unknown",
		ExtensionID: extensionID,
		Name:        name,
	}
}

// String returns a human-readable version string.
func (i *Info) String() string {
	return fmt.Sprintf("%s version %s (commit: %s, built: %s)", i.Name, i.Version, i.GitCommit, i.BuildDate)
}
