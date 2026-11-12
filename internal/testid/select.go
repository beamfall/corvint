// SPDX-License-Identifier: Apache-2.0
package testid

import (
	"strconv"
)

// ReasonDuplicate is the reason printed when a selection names a duplicated
// ID and widens to its whole file (CTI-V1-010).
const ReasonDuplicate = "duplicate-test-id"

// Entry is one test of a listing or report under its final ID: Duplicate
// marks every member of a group that produced the same ID, and Reason is
// ReasonUnmapped for a test the report named only by its file.
type Entry struct {
	ID        string `json:"id"`
	Duplicate bool   `json:"duplicate,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// List disambiguates one listing or report (CTI-V1-010): when two or more
// tests produce the same ID, the second and later get a final segment `#2`,
// `#3` in report order and every member of the group is marked duplicate.
// A test named only by its file (ReasonUnmapped) is already file-level and
// is never suffixed.
func List(named []Named) []Entry {
	count := map[string]int{}
	for _, n := range named {
		if n.Reason == "" {
			count[n.ID.String()]++
		}
	}
	seen := map[string]int{}
	entries := make([]Entry, 0, len(named))
	for _, n := range named {
		base := n.ID.String()
		if n.Reason != "" {
			entries = append(entries, Entry{ID: base, Reason: n.Reason})
			continue
		}
		seen[base]++
		entry := Entry{ID: base, Duplicate: count[base] > 1}
		if seen[base] > 1 {
			entry.ID = n.ID.Child("#" + strconv.Itoa(seen[base])).String()
		}
		entries = append(entries, entry)
	}
	return entries
}

// Scope is how much a resolved selection runs.
type Scope string

const (
	// ScopeTest runs the one test the selection names.
	ScopeTest Scope = "test"
	// ScopeFile runs every test of the selection's file (package for Go).
	ScopeFile Scope = "file"
	// ScopeRunner runs every test of the selection's runner.
	ScopeRunner Scope = "runner"
	// ScopeAll runs every test: the selection names no known runner.
	ScopeAll Scope = "all"
)

// Hint is what the caller knows about a selection besides its ID text: the
// runner and repository-relative file it came from, when known.
type Hint struct {
	Runner string
	File   string
}

// Selection is a requested ID resolved against a listing. Selector is the ID
// that runs (the test, or its file-level ID) and is empty for ScopeRunner
// and ScopeAll. Reason names why the selection widened, and Dangling marks
// a requested test the listing no longer holds (CTI-V1-011).
type Selection struct {
	Scope    Scope  `json:"scope"`
	Selector string `json:"selector,omitempty"`
	Runner   string `json:"runner,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Dangling bool   `json:"dangling,omitempty"`
}

// Resolve resolves one requested ID against listing, the current entries of
// its runner (CTI-V1-010, CTI-V1-011). Only a test-level ID the listing
// holds once is selected alone. Every other request widens and never
// narrows by guess:
//   - an ID that names a duplicated test, or its `#n` form, selects its file
//     with ReasonDuplicate;
//   - a well-formed ID the listing does not hold (a renamed or deleted
//     test) is Dangling and selects its file with ReasonUnmapped;
//   - a test of a file the listing names only at file level (its report
//     gives no name path) selects that file with ReasonUnmapped;
//   - an ID that does not parse, or names an unknown runner, selects
//     hint.File when it is known, otherwise every test of hint.Runner,
//     otherwise every test, with ReasonUnmapped.
//
// A file-level request selects its file and needs no listing.
func Resolve(requested string, listing []Entry, hint Hint) Selection {
	id, err := Parse(requested)
	if err != nil {
		if hinted, err := New(hint.Runner, hint.File); err == nil {
			return Selection{Scope: ScopeFile, Selector: hinted.String(), Runner: hinted.Runner, Reason: ReasonUnmapped}
		}
		if KnownRunner(hint.Runner) {
			return Selection{Scope: ScopeRunner, Runner: hint.Runner, Reason: ReasonUnmapped}
		}
		return Selection{Scope: ScopeAll, Reason: ReasonUnmapped}
	}
	file := Selection{Scope: ScopeFile, Selector: id.FileLevel().String(), Runner: id.Runner}
	if id.IsFileLevel() {
		return file
	}
	base := requested
	if last := id.Segments[len(id.Segments)-1]; len(id.Segments) > 1 && len(last) > 1 && last[0] == '#' {
		if _, err := strconv.Atoi(last[1:]); err == nil {
			base = ID{Runner: id.Runner, File: id.File, Segments: id.Segments[:len(id.Segments)-1]}.String()
		}
	}
	held, duplicate, unmapped := false, false, false
	for _, entry := range listing {
		switch {
		case entry.Reason != "" && entry.ID == file.Selector:
			unmapped = true
		case entry.ID == requested:
			held = true
			duplicate = duplicate || entry.Duplicate
		case entry.ID == base && entry.Duplicate:
			duplicate = true
		}
	}
	switch {
	case duplicate:
		file.Reason = ReasonDuplicate
		return file
	case !held && unmapped:
		file.Reason = ReasonUnmapped
		return file
	case !held:
		file.Reason, file.Dangling = ReasonUnmapped, true
		return file
	}
	return Selection{Scope: ScopeTest, Selector: requested, Runner: id.Runner}
}
