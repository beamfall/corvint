// SPDX-License-Identifier: Apache-2.0

// Package testid formats, parses and matches the canonical test ID of the
// seven blocking runners (docs/specs/canonical-test-id-v1.md):
// `<runner>:<file>::<segment>[::<segment>...]`, with `<runner>:<file>` the
// file-level ID (the package-level ID for Go). It is library-only, imports no
// other Corvint package, and never reads the file system.
package testid

import (
	"errors"
	"fmt"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The runner tokens of the seven blocking runners (CTI-V1-001).
const (
	Go         = "go"
	Vitest     = "vitest"
	Jest       = "jest"
	Playwright = "playwright"
	Pytest     = "pytest"
	RSpec      = "rspec"
	Minitest   = "minitest"
)

// Runners lists the runner tokens in the order the spec names them.
var Runners = []string{Go, Vitest, Jest, Playwright, Pytest, RSpec, Minitest}

// KnownRunner reports whether token is one of the seven runner tokens.
func KnownRunner(token string) bool {
	for _, runner := range Runners {
		if token == runner {
			return true
		}
	}
	return false
}

// separator joins the file and each segment of an ID.
const separator = "::"

// ID is one parsed canonical test ID. File and Segments hold the unescaped
// text; String escapes them. An ID with no segments is file-level.
type ID struct {
	Runner   string
	File     string
	Segments []string
}

// ErrMalformed is wrapped by every Parse and New refusal.
var ErrMalformed = errors.New("malformed test id")

// New validates and returns the ID of runner, file and segments: runner is
// one of the seven tokens and file a clean repository-relative slash path.
func New(runner, file string, segments ...string) (ID, error) {
	if !KnownRunner(runner) {
		return ID{}, fmt.Errorf("%w: unknown runner %q", ErrMalformed, runner)
	}
	if !CleanFile(file) {
		return ID{}, fmt.Errorf("%w: file %q is not a clean repository-relative slash path", ErrMalformed, file)
	}
	return ID{Runner: runner, File: file, Segments: append([]string(nil), segments...)}, nil
}

// CleanFile reports whether file is a clean repository-relative slash path:
// not empty, not absolute, no `..` element and unchanged by path.Clean. The
// repository root itself is ".", the directory of a Go root package.
func CleanFile(file string) bool {
	if file == "" || strings.HasPrefix(file, "/") || path.Clean(file) != file {
		return false
	}
	return file != ".." && !strings.HasPrefix(file, "../")
}

// String is the escaped canonical form (CTI-V1-001, CTI-V1-002).
func (id ID) String() string {
	var out strings.Builder
	out.WriteString(id.Runner)
	out.WriteByte(':')
	out.WriteString(Escape(id.File))
	for _, segment := range id.Segments {
		out.WriteString(separator)
		out.WriteString(Escape(segment))
	}
	return out.String()
}

// FileLevel is the file-level ID of id: its runner and file, no segments.
func (id ID) FileLevel() ID { return ID{Runner: id.Runner, File: id.File} }

// IsFileLevel reports whether id names a whole file (a whole package for Go).
func (id ID) IsFileLevel() bool { return len(id.Segments) == 0 }

// Child is id with segment appended.
func (id ID) Child(segment string) ID {
	return ID{Runner: id.Runner, File: id.File, Segments: append(append([]string(nil), id.Segments...), segment)}
}

// Granularity is how much one ID names.
type Granularity string

const (
	// GranularityTest names one test (an ID with segments).
	GranularityTest Granularity = "test"
	// GranularityFile names every test of one file.
	GranularityFile Granularity = "file"
	// GranularityPackage names every test of one Go package directory.
	GranularityPackage Granularity = "package"
)

// Granularity is what id names: one test, one file, or for Go one package.
func (id ID) Granularity() Granularity {
	switch {
	case !id.IsFileLevel():
		return GranularityTest
	case id.Runner == Go:
		return GranularityPackage
	}
	return GranularityFile
}

// Escape writes `%` as `%25`, `:` as `%3A`, and every control character and
// invalid UTF-8 byte as `%XX` in upper-case hexadecimal; nothing else is
// escaped (CTI-V1-002). A control character outside ASCII is escaped byte
// by byte of its UTF-8 encoding.
func Escape(text string) string {
	var out strings.Builder
	for index := 0; index < len(text); {
		r, size := utf8.DecodeRuneInString(text[index:])
		if r == '%' || r == ':' || unicode.IsControl(r) || (r == utf8.RuneError && size == 1) {
			for _, b := range []byte(text[index : index+size]) {
				fmt.Fprintf(&out, "%%%02X", b)
			}
		} else {
			out.WriteString(text[index : index+size])
		}
		index += size
	}
	return out.String()
}

// Unescape decodes Escape's output. It refuses any text Escape could not have
// written: a raw `%`, `:`, control character or invalid UTF-8 byte, a
// lower-case or truncated escape, or an escape of a character that is not
// escaped, so Escape(Unescape(text)) == text whenever it succeeds.
func Unescape(text string) (string, error) {
	var out strings.Builder
	for index := 0; index < len(text); index++ {
		c := text[index]
		if c != '%' {
			out.WriteByte(c)
			continue
		}
		if index+2 >= len(text) || !upperHex(text[index+1]) || !upperHex(text[index+2]) {
			return "", fmt.Errorf("%w: %q has a malformed escape", ErrMalformed, text)
		}
		out.WriteByte(hexValue(text[index+1])<<4 | hexValue(text[index+2]))
		index += 2
	}
	decoded := out.String()
	if Escape(decoded) != text {
		return "", fmt.Errorf("%w: %q is not in canonical escaped form", ErrMalformed, text)
	}
	return decoded, nil
}

func upperHex(c byte) bool { return '0' <= c && c <= '9' || 'A' <= c && c <= 'F' }

func hexValue(c byte) byte {
	if c <= '9' {
		return c - '0'
	}
	return c - 'A' + 10
}

// Parse reads one canonical ID. It refuses an unknown runner token, a file
// that is not a clean repository-relative slash path, a stray `:` and any
// text not in canonical escaped form, so Parse(s).String() == s whenever it
// succeeds (CTI-V1-002). An empty segment is kept: Playwright names a test of
// a configuration with no projects with an empty first segment.
func Parse(text string) (ID, error) {
	runner, rest, ok := strings.Cut(text, ":")
	if !ok {
		return ID{}, fmt.Errorf("%w: %q has no runner token", ErrMalformed, text)
	}
	if !KnownRunner(runner) {
		return ID{}, fmt.Errorf("%w: unknown runner %q", ErrMalformed, runner)
	}
	parts := strings.Split(rest, separator)
	decoded := make([]string, len(parts))
	for index, part := range parts {
		value, err := Unescape(part)
		if err != nil {
			return ID{}, err
		}
		decoded[index] = value
	}
	id, err := New(runner, decoded[0], decoded[1:]...)
	if err != nil {
		return ID{}, err
	}
	if id.String() != text {
		return ID{}, fmt.Errorf("%w: %q is not in canonical form", ErrMalformed, text)
	}
	return id, nil
}

// Matches reports whether selection selects id (CTI-V1-008): both parse, and
// id equals selection or lies under it, `selection::...`. A file-level
// selection matches every test of its file, a test-level one the test and
// its subtests. The comparison is on the escaped forms, and an ID that does
// not parse is never matched by guess (CTI-V1-011).
func Matches(selection, id string) bool {
	if _, err := Parse(selection); err != nil {
		return false
	}
	if _, err := Parse(id); err != nil {
		return false
	}
	return id == selection || strings.HasPrefix(id, selection+separator)
}

// MatchesInstance reports whether id belongs to the runner instance of runner
// rooted at the repository-relative directory root (CTI-V1-008): its runner
// is runner and its file is root or lies below it; root "." is the whole
// repository.
func MatchesInstance(runner, root, id string) bool {
	parsed, err := Parse(id)
	if err != nil || parsed.Runner != runner || !CleanFile(root) {
		return false
	}
	file, escapedRoot := Escape(parsed.File), Escape(root)
	return root == "." || file == escapedRoot || strings.HasPrefix(file, escapedRoot+"/")
}
