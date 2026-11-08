// SPDX-License-Identifier: Apache-2.0
package testevidence

import (
	"errors"
	"path/filepath"
	"strings"
	"unicode"
)

// Native is the only evidence surface supplied by a plugin (RSP-V0-005).
// There is deliberately no writer capability and no plugin-supplied trace field.
type Native struct {
	Executed Component[ExecutedFile]
	Regions  Component[RegionsFile]
	Loaded   Component[Blob]
}

// Trace is supplied by the core, independently of native artifacts.
type Trace struct {
	Reads    Component[Blob]
	Listings Component[Path]
	Lookups  Component[Path]
	Writes   Component[Write]
	Execs    Component[Executable]
	External Component[Contact]
	Outside  OutsideCounts
}

func Assemble(header Record, native Native, trace Trace) (Record, error) {
	header.Executed, header.Regions, header.Loaded = native.Executed, native.Regions, native.Loaded
	header.Reads, header.Listings, header.Lookups = trace.Reads, trace.Listings, trace.Lookups
	header.Writes, header.Execs, header.External, header.Outside = trace.Writes, trace.Execs, trace.External, trace.Outside
	if err := header.Validate(); err != nil {
		return Record{}, err
	}
	return header, nil
}

// Attribution is a core-verified process-tree/marker observation (RSP-V0-009).
// MarkersQualified means qualification covers these in-process start/end markers.
type Attribution struct {
	Tests             []string
	MarkedTest        string
	MarkersQualified  bool
	BuildPhase        bool
	BuildGraphModeled bool
	BuildLoaded       Component[Blob]
}

func Attribute(r Record, a Attribution) ([]Record, error) {
	if len(a.Tests) == 0 {
		return nil, errors.New("attribution-tests-missing")
	}
	seen := map[string]bool{}
	for _, test := range a.Tests {
		if test == "" || seen[test] {
			return nil, errors.New("attribution-tests-invalid")
		}
		seen[test] = true
	}
	tests := a.Tests
	if !a.BuildPhase && a.MarkersQualified && a.MarkedTest != "" {
		if !seen[a.MarkedTest] {
			return nil, errors.New("attribution-marker-outside-tree")
		}
		tests = []string{a.MarkedTest}
	}
	if a.BuildPhase && a.BuildGraphModeled {
		// RSP-V0-009: modeled build activity contributes loaded edges as well
		// as its trace dependencies. Preserve every component and its state;
		// modeling never certifies an absent or incomplete trace as empty.
		r.Loaded = a.BuildLoaded
	} // RSP-V0-054: unmodeled build trace stays on every test, including reads.
	out := make([]Record, 0, len(tests))
	for _, test := range tests {
		copy := r
		copy.Test = test
		if err := copy.Validate(); err != nil {
			return nil, err
		}
		out = append(out, copy)
	}
	return out, nil
}

// WriteProof describes facts already established in order by one trace backend.
// It cannot be inferred from a write kind alone (RSP-V0-007).
type WriteProof struct {
	Write                  Write
	SameTree               bool
	BeforeRead             bool
	Succeeded              bool
	ExclusiveCreate        bool
	OwnFailedLookup        bool
	RenamedTreeCreatedFile bool
}

func DropRead(read Blob, proof WriteProof) bool {
	// Creation of target content never proves independence of a lexical link.
	if read.Path.Lexical != read.Path.Resolved {
		return false
	}
	if !validBlob(read) || !proof.SameTree || !proof.BeforeRead || !proof.Succeeded || read.Path.Repository != proof.Write.Path.Repository || read.Path.Resolved != proof.Write.Path.Resolved {
		return false
	}
	switch proof.Write.Kind {
	case Create:
		return proof.ExclusiveCreate || proof.OwnFailedLookup
	case Replace:
		return proof.RenamedTreeCreatedFile
	default:
		return false
	}
}

// Repository roots are declared and resolved by the caller. The most specific
// declared root wins; outside paths are counted without retaining their names.
type Repository struct {
	ID, Root        string
	CaseInsensitive bool
}
type OutsideClass string

const (
	Toolchain OutsideClass = "toolchain"
	Cache     OutsideClass = "cache"
	Temporary OutsideClass = "temporary"
	Other     OutsideClass = "other"
)

// foldPath chooses one representative for each Unicode simple-fold cycle.
// Lowercasing alone would leave final sigma and sigma as different keys.
func foldPath(s string) string {
	return strings.Map(func(r rune) rune {
		best := r
		for next := unicode.SimpleFold(r); next != r; next = unicode.SimpleFold(next) {
			if next < best {
				best = next
			}
		}
		return unicode.ToLower(best)
	}, s)
}

func relative(root, name string, fold bool) (string, bool) {
	if fold {
		root, name = foldPath(root), foldPath(name)
	}
	rel, err := filepath.Rel(root, name)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// CaptureRead classifies a resolved read against declared repositories
// (RSP-V0-049). Unrepresentable cross-repository links remain incomplete rather
// than silently losing their lexical dependency.
func CaptureRead(reads *Component[Blob], counts *OutsideCounts, repos []Repository, lexical, resolved, oid, lexicalOID string, outside OutsideClass) {
	fail := func(reason string) { reads.State = Incomplete; reads.Reason = reason }
	if !filepath.IsAbs(lexical) || !filepath.IsAbs(resolved) {
		fail("read-path-unresolved")
		return
	}
	seen := map[string]bool{}
	best := -1
	lexicalInside := false
	for i, repo := range repos {
		if repo.ID == "" || seen[repo.ID] || !filepath.IsAbs(repo.Root) {
			fail("repository-declaration-invalid")
			return
		}
		seen[repo.ID] = true
		if _, ok := relative(repo.Root, lexical, repo.CaseInsensitive); ok {
			lexicalInside = true
		}
		if _, ok := relative(repo.Root, resolved, repo.CaseInsensitive); ok && (best < 0 || len(repo.Root) > len(repos[best].Root)) {
			best = i
		}
	}
	if best < 0 {
		switch outside {
		case Toolchain:
			counts.Toolchain++
		case Cache:
			counts.Cache++
		case Temporary:
			counts.Temporary++
		default:
			counts.Other++
			fail("undeclared-outside-read")
		}
		// Count-only reads require both names outside every repository and a
		// known outside class. A repository link can change under a diff even
		// when its target cannot; this record cannot represent that target.
		if lexicalInside {
			fail("cross-repository-link")
		}
		return
	}
	repo := repos[best]
	lex, ok := relative(repo.Root, lexical, repo.CaseInsensitive)
	if !ok {
		fail("cross-repository-link")
		return
	}
	res, _ := relative(repo.Root, resolved, repo.CaseInsensitive)
	b := Blob{Path: Path{Repository: repo.ID, Lexical: lex, Resolved: res}, OID: oid, LexicalOID: lexicalOID}
	if !validBlob(b) {
		fail("read-blob-malformed")
		return
	}
	if reads.State != Present && reads.State != Incomplete {
		fail("read-capture-not-started")
	}
	reads.Items = append(reads.Items, b)
}
