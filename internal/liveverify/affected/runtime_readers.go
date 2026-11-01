package affected

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// RuntimeReadersPath is the repository's declaration of the data paths whose
// every runtime reader is a unit the reader rules already name (AFP-V0-602).
const RuntimeReadersPath = ".corvint/runtime-readers.json"

// RuntimeReadersProfile is the declaration's closed profile name.
const RuntimeReadersProfile = "corvint-runtime-readers/0"

// ClassDeclaredRuntimeReaders is the NonSource class of a dirty path the
// repository declares in RuntimeReadersPath (AFP-V0-602).
const ClassDeclaredRuntimeReaders = "DECLARED_RUNTIME_READERS"

// runtimeReadersReason prefixes the declared reason of every
// ClassDeclaredRuntimeReaders group.
const runtimeReadersReason = "declared runtime-readers path (" + RuntimeReadersPath + "); every unit whose embed, enclosing directory, path literal, declared read scope or unbounded read reaches it is selected, and the repository declares that no other test reads it; a repository-wide check CI runs unconditionally is no test selection"

const (
	maxRuntimeReadersBytes   = 64 << 10
	maxRuntimeReadersEntries = 256
	maxRuntimeReaderReason   = 512
)

// RuntimeReader is one declared entry: a repository-relative file, or a
// directory when it ends in `/`, and the project's stated reason.
type RuntimeReader struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
	// NoUnboundedReaders is the project's statement, in
	// UnboundedReaderExclusionsPath, that no unbounded reader (AFP-V0-012 rule
	// (d)) reads the path: a dirty path the entry holds then selects no
	// unbounded reader and follows no root locator (AFP-V0-772). It is never
	// read from RuntimeReadersPath, whose closed grammar older planners share.
	NoUnboundedReaders bool `json:"noUnboundedReaders,omitempty"`
}

// UnboundedReaderExclusionsPath is the repository's declaration of the
// runtime-readers entries no unbounded reader reads (AFP-V0-772). It is a
// file of its own, so a planner that predates it ignores it and keeps the
// AFP-V0-602 selection rather than refusing the runtime-readers declaration.
const UnboundedReaderExclusionsPath = ".corvint/unbounded-reader-exclusions.json"

// UnboundedReaderExclusionsProfile is that declaration's closed profile name.
const UnboundedReaderExclusionsProfile = "corvint-unbounded-reader-exclusions/0"

type runtimeReaderFileEntry struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type unboundedExclusionsFile struct {
	Profile string   `json:"profile"`
	Paths   []string `json:"paths"`
}

// applyUnboundedExclusions marks each runtime-readers entry the exclusions
// declaration names (AFP-V0-772). Absent, it marks nothing; one that is
// unreadable, oversized, not exactly its grammar, or that names a path no
// runtime-readers entry declares, refuses the graph build.
func applyUnboundedExclusions(root *Source, entries []RuntimeReader) error {
	body, err := root.ReadBounded(UnboundedReaderExclusionsPath, maxRuntimeReadersBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%s: %w", UnboundedReaderExclusionsPath, err)
	}
	var declared unboundedExclusionsFile
	if err := json.UnmarshalRead(bytes.NewReader(body), &declared, json.RejectUnknownMembers(true)); err != nil {
		return fmt.Errorf("%s: %w", UnboundedReaderExclusionsPath, err)
	}
	if declared.Profile != UnboundedReaderExclusionsProfile || len(declared.Paths) == 0 || len(declared.Paths) > maxRuntimeReadersEntries {
		return fmt.Errorf("%s: profile must be %s with 1 to %d paths", UnboundedReaderExclusionsPath, UnboundedReaderExclusionsProfile, maxRuntimeReadersEntries)
	}
	previous := ""
	for _, path := range declared.Paths {
		index := slices.IndexFunc(entries, func(entry RuntimeReader) bool { return entry.Path == path })
		if path <= previous || index < 0 {
			return fmt.Errorf("%s: path %q is not a declared runtime-readers entry in order", UnboundedReaderExclusionsPath, path)
		}
		entries[index].NoUnboundedReaders = true
		previous = path
	}
	return nil
}

// unboundedDirty returns dirty without the paths an entry with
// NoUnboundedReaders holds (AFP-V0-772).
func (graph *Graph) unboundedDirty(dirty []string) []string {
	kept := make([]string, 0, len(dirty))
	for _, path := range dirty {
		if index := graph.runtimeReaderEntry(path); index < 0 || !graph.runtimeReaders[index].NoUnboundedReaders {
			kept = append(kept, path)
		}
	}
	return kept
}

type runtimeReadersFile struct {
	Profile string                   `json:"profile"`
	Entries []runtimeReaderFileEntry `json:"entries"`
}

// readRuntimeReaders returns the declared entries: nil with no declaration,
// an error for one that is unreadable, oversized, not a regular file, or
// invalid in any member (AFP-V0-602 (1)). Entries are strictly ascending by
// path and each carries a non-empty printable reason. No entry may be
// `.corvint/`, either declaration file or a directory holding one, anything at
// or under `.github/`, or a make file, so the declaration can never classify
// itself or the documentation declaration. A driver input below a declared
// directory is left unowned per path by runtimeReaderPaths. Another file under
// `.corvint/` may be declared one by one, as this repository declares its
// read-scope declaration.
func readRuntimeReaders(root *Source) ([]RuntimeReader, error) {
	if root == nil {
		return nil, nil
	}
	body, err := root.ReadBounded(RuntimeReadersPath, maxRuntimeReadersBytes)
	if errors.Is(err, fs.ErrNotExist) {
		// The exclusions are read, and so stamped, either way; with no entry
		// to name, any exclusion refuses (AFP-V0-772).
		return nil, applyUnboundedExclusions(root, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", RuntimeReadersPath, err)
	}
	var declared runtimeReadersFile
	if err := json.UnmarshalRead(bytes.NewReader(body), &declared, json.RejectUnknownMembers(true)); err != nil {
		return nil, fmt.Errorf("%s: %w", RuntimeReadersPath, err)
	}
	if declared.Profile != RuntimeReadersProfile || declared.Entries == nil || len(declared.Entries) > maxRuntimeReadersEntries {
		return nil, fmt.Errorf("%s: profile must be %s with at most %d entries", RuntimeReadersPath, RuntimeReadersProfile, maxRuntimeReadersEntries)
	}
	previous := ""
	for _, entry := range declared.Entries {
		if !ValidReadScopeEntry(entry.Path) || entry.Path <= previous || !validRuntimeReaderPath(entry.Path) {
			return nil, fmt.Errorf("%s: path %q", RuntimeReadersPath, entry.Path)
		}
		if entry.Reason == "" || len(entry.Reason) > maxRuntimeReaderReason || !utf8.ValidString(entry.Reason) || strings.ContainsFunc(entry.Reason, unicode.IsControl) {
			return nil, fmt.Errorf("%s: reason of %q", RuntimeReadersPath, entry.Path)
		}
		previous = entry.Path
	}
	entries := make([]RuntimeReader, 0, len(declared.Entries))
	for _, entry := range declared.Entries {
		entries = append(entries, RuntimeReader{Path: entry.Path, Reason: entry.Reason})
	}
	if err := applyUnboundedExclusions(root, entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// validRuntimeReaderPath refuses an entry that is, or as a directory covers,
// a Corvint declaration or a driver input: `.corvint/` itself, either
// declaration file, anything at or under `.github/`, or a make file.
func validRuntimeReaderPath(entry string) bool {
	trimmed := strings.TrimSuffix(entry, "/")
	if trimmed == "" || trimmed == ".corvint" || trimmed == ".github" || strings.HasPrefix(trimmed, ".github/") || driverInput(trimmed) {
		return false
	}
	for _, declaration := range []string{RuntimeReadersPath, DocumentationPathsPath} {
		if trimmed == declaration || (strings.HasSuffix(entry, "/") && strings.HasPrefix(declaration, entry)) {
			return false
		}
	}
	return true
}

// runtimeReaderEntry returns the index of the declared entry that holds path,
// or -1.
func (graph *Graph) runtimeReaderEntry(path string) int {
	for index, entry := range graph.runtimeReaders {
		if path == entry.Path || (strings.HasSuffix(entry.Path, "/") && strings.HasPrefix(path, entry.Path)) {
			return index
		}
	}
	return -1
}

// runtimeReaderPaths removes, from the unknown entries left after
// documentationPaths, each UNOWNED_DIRTY_PATH entry whose path a declared
// entry holds, and returns those paths as one ClassDeclaredRuntimeReaders
// group per entry, in declaration order, each carrying the entry's reason
// (AFP-V0-602 (2)). The reader rules ran before this step, so every unit they
// name is already selected; the declaration is the project's authority that
// no other test process, in any language, reads the path by a computed name.
// A path a plugin claims as source (UNINDEXED_SOURCE_PATH), a frontier entry,
// a task store projection and a driver input are never reclassified.
func (graph *Graph) runtimeReaderPaths(unknown []Unknown) ([]NonSource, []Unknown) {
	if len(graph.runtimeReaders) == 0 {
		return nil, unknown
	}
	byEntry := make(map[int][]string)
	kept := make([]Unknown, 0, len(unknown))
	for _, entry := range unknown {
		if entry.Reason == UnknownUnownedDirtyPath && !IsTaskStoreProjectionPath(entry.Detail) && !driverInput(entry.Detail) {
			if index := graph.runtimeReaderEntry(entry.Detail); index >= 0 {
				byEntry[index] = append(byEntry[index], entry.Detail)
				continue
			}
		}
		kept = append(kept, entry)
	}
	var groups []NonSource
	for index, entry := range graph.runtimeReaders {
		if paths := byEntry[index]; len(paths) != 0 {
			groups = append(groups, NonSource{Class: ClassDeclaredRuntimeReaders, Reason: runtimeReadersReason + ": " + entry.Reason, Paths: paths})
		}
	}
	return groups, kept
}
