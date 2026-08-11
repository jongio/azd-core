package covergate

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// Block is a single counted region from a Go coverage profile.
type Block struct {
	// File is the import-path-qualified file name, for example
	// "github.com/jongio/azd-core/env/load.go".
	File string
	// Statements is the number of statements in the block.
	Statements int
	// Count is the number of times the block was executed.
	Count int
}

// Covered reports whether the block was executed at least once.
func (b Block) Covered() bool { return b.Count > 0 }

// Package returns the directory portion of the block's file path, which for
// Go coverage profiles is the package import path.
func (b Block) Package() string {
	dir := path.Dir(b.File)
	if dir == "." {
		return ""
	}
	return dir
}

// profileLine matches "path/file.go:1.2,3.4 5 6" and tolerates colons in the
// path (Windows drive letters never appear here, but import paths are safe).
var profileLine = regexp.MustCompile(`^(.+):\d+\.\d+,\d+\.\d+ (\d+) (\d+)$`)

// ProfileData is a parsed coverage profile: the counter mode declared in its
// header plus the blocks it contains.
//
// The mode matters to the ratchet because "go test -covermode" materially
// changes the reported percentage. Atomic counters survive concurrent updates
// that "set" and "count" mode lose, so the same tests can report several
// points higher under atomic. Comparing a baseline recorded in one mode
// against a profile measured in another produces meaningless deltas.
type ProfileData struct {
	// Mode is the value of the profile's "mode:" header, such as "atomic".
	Mode string
	// Blocks holds every counted region in the profile.
	Blocks []Block
}

// ParseProfile reads a Go coverage profile produced by "go test -coverprofile".
//
// The leading "mode:" line is required, matching the format that
// "go tool cover" itself accepts. Blank lines are ignored.
func ParseProfile(r io.Reader) (ProfileData, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var (
		data    ProfileData
		sawMode bool
		lineNum int
	)

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if !sawMode {
			if !strings.HasPrefix(line, "mode:") {
				return ProfileData{}, fmt.Errorf("line %d: expected a \"mode:\" header, got %q", lineNum, line)
			}
			data.Mode = strings.TrimSpace(strings.TrimPrefix(line, "mode:"))
			sawMode = true
			continue
		}

		m := profileLine.FindStringSubmatch(line)
		if m == nil {
			return ProfileData{}, fmt.Errorf("line %d: malformed coverage block %q", lineNum, line)
		}

		statements, err := strconv.Atoi(m[2])
		if err != nil {
			return ProfileData{}, fmt.Errorf("line %d: bad statement count: %w", lineNum, err)
		}
		count, err := strconv.Atoi(m[3])
		if err != nil {
			return ProfileData{}, fmt.Errorf("line %d: bad execution count: %w", lineNum, err)
		}

		data.Blocks = append(data.Blocks, Block{File: m[1], Statements: statements, Count: count})
	}

	if err := scanner.Err(); err != nil {
		return ProfileData{}, fmt.Errorf("reading coverage profile: %w", err)
	}
	if !sawMode {
		return ProfileData{}, fmt.Errorf("empty or invalid coverage profile: no \"mode:\" header found")
	}
	return data, nil
}

// ParseProfileFile reads and parses a coverage profile from disk.
//
// The path comes from build tooling rather than untrusted input, so it is
// opened directly.
func ParseProfileFile(name string) (ProfileData, error) {
	f, err := os.Open(name) // #nosec G304 -- build-tool supplied path
	if err != nil {
		return ProfileData{}, fmt.Errorf("opening coverage profile: %w", err)
	}
	defer f.Close()

	data, err := ParseProfile(f)
	if err != nil {
		return ProfileData{}, fmt.Errorf("parsing %s: %w", name, err)
	}
	return data, nil
}

// globToRegexp converts a slash-separated glob into an anchored regular
// expression. It supports "**" for any number of path segments, "*" for any
// characters within one segment, and "?" for a single non-separator character.
//
// Every other character is escaped, so the result always compiles and this
// function cannot fail. Character classes such as "[a-z]" are therefore
// matched literally rather than interpreted.
func globToRegexp(glob string) *regexp.Regexp {
	var sb strings.Builder
	sb.WriteString("^")

	for i := 0; i < len(glob); i++ {
		switch glob[i] {
		case '*':
			// "**/" collapses to an optional run of whole segments so that
			// "**/gen/**" matches "gen/x.go" as well as "a/b/gen/x.go".
			if i+1 < len(glob) && glob[i+1] == '*' {
				if i+2 < len(glob) && glob[i+2] == '/' {
					sb.WriteString("(?:.*/)?")
					i += 2
				} else {
					sb.WriteString(".*")
					i++
				}
				continue
			}
			sb.WriteString("[^/]*")
		case '?':
			sb.WriteString("[^/]")
		default:
			sb.WriteString(regexp.QuoteMeta(string(glob[i])))
		}
	}

	sb.WriteString("$")
	return regexp.MustCompile(sb.String())
}
