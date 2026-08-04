// Package pack holds the jongio.azd extension pack manifest.
//
// The package carries no runtime code. It exists so the manifest sitting beside
// it can be guarded by tests: azd decides whether a manifest is a pack by what
// it does not contain, which makes the failure mode silent, and a silent failure
// mode needs a test rather than a comment.
package pack
