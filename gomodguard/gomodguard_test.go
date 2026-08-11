package gomodguard

import (
	"strings"
	"testing"
)

const target = "github.com/jongio/azd-core"

func TestFindReplace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		gomod string
		want  string
	}{
		// The four spellings the go.mod grammar allows. The version-qualified
		// ones are what a prefix match on "replace " or on the bare module
		// path silently misses.
		{
			name:  "single line without version",
			gomod: "module m\n\nreplace " + target + " => ../azd-core\n",
			want:  "replace " + target + " => ../azd-core",
		},
		{
			name:  "single line with version",
			gomod: "module m\n\nreplace " + target + " v0.6.0 => ../azd-core\n",
			want:  "replace " + target + " v0.6.0 => ../azd-core",
		},
		{
			name:  "block without version",
			gomod: "module m\n\nreplace (\n\t" + target + " => ../azd-core\n)\n",
			want:  target + " => ../azd-core",
		},
		{
			name:  "block with version on both sides",
			gomod: "module m\n\nreplace (\n\t" + target + " v0.6.0 => " + target + " v0.5.8-0.20260808190154-f1189c6e3eea\n)\n",
			want:  target + " v0.6.0 => " + target + " v0.5.8-0.20260808190154-f1189c6e3eea",
		},
		{
			name:  "substring module path matches",
			gomod: "module m\n\nreplace " + target + " => ../azd-core\n",
			want:  "replace " + target + " => ../azd-core",
		},

		// Everything that names the module without replacing it.
		{
			name:  "require line naming the module",
			gomod: "module m\n\nrequire (\n\t" + target + " v0.6.0\n)\n",
		},
		{
			name:  "commented out replace",
			gomod: "module m\n\n// replace " + target + " => ../azd-core\n",
		},
		{
			name:  "replace of an unrelated module",
			gomod: "module m\n\nreplace github.com/other/mod => ../other\n",
		},
		{
			name:  "module named only on the replacement side",
			gomod: "module m\n\nreplace github.com/other/mod => " + target + " v0.6.0\n",
		},
		{
			name:  "empty file",
			gomod: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := FindReplace(tc.gomod, target); got != tc.want {
				t.Fatalf("FindReplace() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFindReplaceReturnsTheFirstMatch(t *testing.T) {
	t.Parallel()

	gomod := "module m\n\nreplace (\n\t" + target + " => ../first\n\t" + target + " v0.6.0 => ../second\n)\n"

	if got := FindReplace(gomod, target); !strings.Contains(got, "../first") {
		t.Fatalf("FindReplace() = %q, want the first directive", got)
	}
}

func TestFindReplaceToleratesCarriageReturns(t *testing.T) {
	t.Parallel()

	// A go.mod checked out on Windows can carry CRLF endings, and callers read
	// the file as raw bytes rather than through a normalizing reader.
	gomod := strings.ReplaceAll("module m\n\nreplace "+target+" => ../azd-core\n", "\n", "\r\n")

	want := "replace " + target + " => ../azd-core"
	if got := FindReplace(gomod, target); got != want {
		t.Fatalf("FindReplace() = %q, want %q", got, want)
	}
}
