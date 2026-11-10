// SPDX-License-Identifier: Apache-2.0

// Package repopath holds the canonical repository-relative path form, the
// fixed walk and per-file source bounds and the bounded no-follow source read
// that the affected planner, the docs corpus and the MCP planning snapshot
// share. It is a leaf: it imports nothing of Corvint, so a shared-layer
// package can validate or read a repository path without the planner.
//
// The error texts keep their original `affected:` wording: the planner
// exposes these values under its own names and callers match them by value.
package repopath

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

var (
	// ErrNotCanonical reports a path that is not in canonical form, or a read
	// of something that is not a regular file. The planner names it
	// affected.ErrInvalidUnit.
	ErrNotCanonical = errors.New("affected: unit is not canonical")
	// ErrWalkLimit reports a repository larger than the fixed walk bound, or a
	// source over the per-file bound. The planner names it affected.ErrWalkLimit.
	ErrWalkLimit = errors.New("affected: repository walk exceeds its bound")
	// ErrNameListMalformed reports a NUL-delimited path list Corvint could not
	// decode exactly. The planner names it affected.ErrStatusMalformed.
	ErrNameListMalformed = errors.New("affected: worktree status is malformed")
)

const (
	// MaxWalkEntries bounds one repository walk.
	MaxWalkEntries = 400_000
	// MaxSourceBytes bounds one source file read for imports.
	MaxSourceBytes = 4 << 20
)

// Valid reports whether value is the canonical repository-relative form
// accepted everywhere: non-empty, valid UTF-8, slash separated, no leading
// slash, no "." or ".." component, and no empty component.
func Valid(value string) bool {
	if value == "" || !utf8.ValidString(value) || strings.HasPrefix(value, "/") {
		return false
	}
	if strings.ContainsRune(value, 0) || strings.Contains(value, `\`) {
		return false
	}
	for _, component := range strings.Split(value, "/") {
		if component == "" || component == "." || component == ".." {
			return false
		}
	}
	return true
}

// Normalize sorts and deduplicates a path set and drops anything not in
// canonical repository-relative form.
func Normalize(values []string) []string {
	seen := make(map[string]bool, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		if !Valid(value) || seen[value] {
			continue
		}
		seen[value] = true
		normalized = append(normalized, value)
	}
	sort.Strings(normalized)
	return normalized
}

// DecodeNameList parses a NUL-delimited path list such as
// `git diff --name-only -z` emits, admitting only valid relative paths.
func DecodeNameList(raw []byte) ([]string, error) {
	fields := bytes.Split(raw, []byte{0})
	if len(fields) != 0 && len(fields[len(fields)-1]) == 0 {
		fields = fields[:len(fields)-1]
	}
	paths := make([]string, 0, len(fields))
	for _, field := range fields {
		value := DisplayPath(field)
		if !Valid(value) {
			return nil, fmt.Errorf("%w: path %q", ErrNameListMalformed, value)
		}
		paths = append(paths, value)
	}
	return Normalize(paths), nil
}

// DisplayPath is a Git path as valid UTF-8: each invalid byte run becomes
// U+FFFD, so a refusal can name the path without emitting raw bytes.
func DisplayPath(raw []byte) string { return strings.ToValidUTF8(string(raw), "�") }

// ReadSource reads one repository-relative source file under root, bounded by
// MaxSourceBytes. A file at or over the bound is refused rather than truncated,
// because a truncated read would silently drop import edges.
func ReadSource(root, relative string) ([]byte, error) {
	if !Valid(relative) {
		return nil, fmt.Errorf("%w: path %q", ErrNotCanonical, relative)
	}
	return readSourceFile(filepath.Join(root, filepath.FromSlash(relative)), relative)
}
