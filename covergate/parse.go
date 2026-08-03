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

// ParseProfile reads a Go coverage profile produced by "go test -coverprofile".
//
// The leading "mode:" line is required, matching the format that
// "go tool cover" itself accepts. Blank lines are ignored.
func ParseProfile(r io.Reader) ([]Block, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var (
		blocks   []Block
		sawMode  bool
		lineNum  int
		firstErr error
	)

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if !sawMode {
			if !strings.HasPrefix(line, "mode:") {
				return nil, fmt.Errorf("line %d: expected a \"mode:\" header, got %q", lineNum, line)
			}
			sawMode = true
			continue
		}

		m := profileLine.FindStringSubmatch(line)
		if m == nil {
			return nil, fmt.Errorf("line %d: malformed coverage block %q", lineNum, line)
		}

		statements, err := strconv.Atoi(m[2])
		if err != nil {
			return nil, fmt.Errorf("line %d: bad statement count: %w", lineNum, err)
		}
		count, err := strconv.Atoi(m[3])
		if err != nil {
			return nil, fmt.Errorf("line %d: bad execution count: %w", lineNum, err)
		}

		blocks = append(blocks, Block{File: m[1], Statements: statements, Count: count})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading coverage profile: %w", err)
	}
	if !sawMode {
		return nil, fmt.Errorf("empty or invalid coverage profile: no \"mode:\" header found")
	}
	return blocks, firstErr
}

// ParseProfileFile reads and parses a coverage profile from disk.
//
// The path comes from build tooling rather than untrusted input, so it is
// opened directly.
func ParseProfileFile(name string) ([]Block, error) {
	f, err := os.Open(name) // #nosec G304 -- build-tool supplied path
	if err != nil {
		return nil, fmt.Errorf("opening coverage profile: %w", err)
	}
	defer f.Close()

	blocks, err := ParseProfile(f)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", name, err)
	}
	return blocks, nil
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
