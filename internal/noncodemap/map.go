// SPDX-License-Identifier: Apache-2.0

// Package noncodemap is the non-code map of docs/specs/non-code-map-v1.md
// (NCM-V1-001 to NCM-V1-009): the strict reader of `.corvint/data-map.json`
// (profile `corvint-data-map/0`), the status of each entry, the paths no entry
// may narrow, the per-path decision the AFP-V0-012 selector prints, and the
// proposal written from recorded test reads. Every function is pure over its
// inputs, or reads one bounded file, and writes nothing (NCM-V1-009).
package noncodemap

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	// Path is the repository's map file.
	Path = ".corvint/data-map.json"
	// Profile is the one profile version a 1.0 reader admits (NCM-V1-007).
	Profile = "corvint-data-map/0"
	// CodeInvalid is the refusal code of a map the reader does not admit.
	CodeInvalid = "data-map-invalid"
	// MaxBytes bounds the map file.
	MaxBytes = 256 << 10
	// MaxEntries bounds the number of entries.
	MaxEntries = 4096
	// maxTests bounds the tests of one entry, and maxField one string member.
	maxTests = 1024
	maxField = 1024
)

// Entry is one map entry (NCM-V1-002): a path or a glob, the tests that read
// it, the cited evidence, who accepted it and when, the commit it was pinned
// at, and the fault case that proved it. An entry missing any acceptance
// member or its proof is read, and reported `proposed`.
type Entry struct {
	Path         string   `json:"path,omitempty"`
	Glob         string   `json:"glob,omitempty"`
	Tests        []string `json:"tests"`
	Evidence     string   `json:"evidence"`
	AcceptedBy   string   `json:"accepted_by,omitempty"`
	AcceptedAt   string   `json:"accepted_at,omitempty"`
	PinnedCommit string   `json:"pinned_commit,omitempty"`
	Proof        string   `json:"proof,omitempty"`
}

// Key is the entry's path or glob.
func (entry Entry) Key() string {
	if entry.Glob != "" {
		return entry.Glob
	}
	return entry.Path
}

// Covers reports whether the entry names p: the same path, or a glob that
// matches it with path.Match semantics.
func (entry Entry) Covers(p string) bool {
	if entry.Glob == "" {
		return entry.Path == p
	}
	matched, err := path.Match(entry.Glob, p)
	return err == nil && matched
}

// Map is a read map.
type Map struct {
	Profile string  `json:"profile"`
	Entries []Entry `json:"entries"`
}

// InvalidError is the `data-map-invalid` refusal (NCM-V1-007).
type InvalidError struct{ Detail string }

func (err *InvalidError) Error() string { return CodeInvalid + ": " + err.Detail }

func invalid(format string, arguments ...any) error {
	return &InvalidError{Detail: fmt.Sprintf(format, arguments...)}
}

// IsInvalid reports whether err is a `data-map-invalid` refusal.
func IsInvalid(err error) bool {
	var target *InvalidError
	return errors.As(err, &target)
}

var commitPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

// Read reads the map under root. A missing map is an empty map, so every
// non-code path runs everything (the design's rollback); a map that is not a
// regular file, is larger than MaxBytes or fails Parse is refused, never read
// as empty.
func Read(root string) (Map, error) {
	name := filepath.Join(root, filepath.FromSlash(Path))
	info, err := os.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return Map{Profile: Profile, Entries: []Entry{}}, nil
	}
	if err != nil {
		return Map{}, invalid("%s: %v", Path, err)
	}
	if !info.Mode().IsRegular() {
		return Map{}, invalid("%s is not a regular file", Path)
	}
	if info.Size() > MaxBytes {
		return Map{}, invalid("%s is larger than %d bytes", Path, MaxBytes)
	}
	file, err := os.Open(name)
	if err != nil {
		return Map{}, invalid("%s: %v", Path, err)
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, MaxBytes+1))
	if err != nil {
		return Map{}, invalid("%s: %v", Path, err)
	}
	if len(body) > MaxBytes {
		return Map{}, invalid("%s is larger than %d bytes", Path, MaxBytes)
	}
	return Parse(body)
}

// Parse decodes and checks one map document: exactly the members of Map and
// Entry, the profile Profile, at most MaxEntries entries, and in each entry
// exactly one of a clean repository-relative path or a valid glob, one to
// maxTests distinct printable tests, a printable evidence string, an
// accepted_at that is a date or an RFC 3339 time, and a pinned_commit that is
// a full object name. No two entries share a key.
func Parse(body []byte) (Map, error) {
	if len(body) > MaxBytes {
		return Map{}, invalid("%s is larger than %d bytes", Path, MaxBytes)
	}
	var probe struct {
		Profile *string `json:"profile"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return Map{}, invalid("%s: %v", Path, err)
	}
	if probe.Profile == nil {
		return Map{}, invalid("%s has no profile; it must be %s", Path, Profile)
	}
	if *probe.Profile != Profile {
		return Map{}, invalid("%s profile %q is not %s, the one version this reader admits", Path, *probe.Profile, Profile)
	}
	var read Map
	if err := json.UnmarshalRead(bytes.NewReader(body), &read, json.RejectUnknownMembers(true)); err != nil {
		return Map{}, invalid("%s: %v", Path, err)
	}
	if read.Entries == nil {
		return Map{}, invalid("%s has no entries member", Path)
	}
	if len(read.Entries) > MaxEntries {
		return Map{}, invalid("%s has %d entries; at most %d", Path, len(read.Entries), MaxEntries)
	}
	keys := make(map[string]int, len(read.Entries))
	for index, entry := range read.Entries {
		if err := checkEntry(entry); err != nil {
			return Map{}, invalid("%s entry %d: %s", Path, index+1, err.Error())
		}
		if first, seen := keys[entry.Key()]; seen {
			return Map{}, invalid("%s entry %d repeats the key %q of entry %d", Path, index+1, entry.Key(), first)
		}
		keys[entry.Key()] = index + 1
	}
	return read, nil
}

func checkEntry(entry Entry) error {
	switch {
	case (entry.Path == "") == (entry.Glob == ""):
		return errors.New("needs exactly one of path or glob")
	case entry.Path != "" && !CleanPath(entry.Path):
		return fmt.Errorf("path %q is not a clean repository-relative file path", entry.Path)
	case entry.Glob != "" && !validGlob(entry.Glob):
		return fmt.Errorf("glob %q is not a valid repository-relative pattern", entry.Glob)
	case len(entry.Tests) == 0 || len(entry.Tests) > maxTests:
		return fmt.Errorf("needs 1 to %d tests", maxTests)
	case !printable(entry.Evidence):
		return errors.New("needs printable evidence")
	case entry.AcceptedBy != "" && !printable(entry.AcceptedBy):
		return errors.New("accepted_by is not printable")
	case entry.AcceptedAt != "" && !validTime(entry.AcceptedAt):
		return fmt.Errorf("accepted_at %q is neither a date nor an RFC 3339 time", entry.AcceptedAt)
	case entry.PinnedCommit != "" && !commitPattern.MatchString(entry.PinnedCommit):
		return fmt.Errorf("pinned_commit %q is not a full lower-case object name", entry.PinnedCommit)
	case entry.Proof != "" && !printable(entry.Proof):
		return errors.New("proof is not printable")
	}
	seen := make(map[string]bool, len(entry.Tests))
	for _, test := range entry.Tests {
		if !printable(test) || strings.ContainsRune(test, ' ') {
			return fmt.Errorf("test %q is not a printable name without spaces", test)
		}
		if seen[test] {
			return fmt.Errorf("test %q is listed twice", test)
		}
		seen[test] = true
	}
	return nil
}

// CleanPath reports whether p is a clean, local, slash-separated file path
// with no backslash or control character.
func CleanPath(p string) bool {
	return p != "." && len(p) <= maxField && fs.ValidPath(p) && !strings.ContainsRune(p, '\\') && !strings.ContainsFunc(p, unicode.IsControl)
}

func validGlob(glob string) bool {
	if !CleanPath(glob) {
		return false
	}
	_, err := path.Match(glob, "")
	return err == nil
}

func validTime(value string) bool {
	if _, err := time.Parse(time.DateOnly, value); err == nil {
		return true
	}
	_, err := time.Parse(time.RFC3339, value)
	return err == nil
}

func printable(value string) bool {
	return value != "" && len(value) <= maxField && utf8.ValidString(value) && strings.TrimSpace(value) == value && !strings.ContainsFunc(value, unicode.IsControl)
}

// sortedEntries returns the entries in key order.
func (read Map) sortedEntries() []Entry {
	entries := slices.Clone(read.Entries)
	slices.SortFunc(entries, func(a, b Entry) int { return strings.Compare(a.Key(), b.Key()) })
	return entries
}
