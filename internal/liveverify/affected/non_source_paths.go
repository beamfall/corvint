package affected

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"unicode"
	"unicode/utf8"
)

// NonSourcePathsPath is the repository's declaration of tracked paths that are
// output rather than input, such as retained evidence (AZU-V0-002): no build
// runs them and no test reads them except the units the reader rules already
// name.
const NonSourcePathsPath = ".corvint/non-source-paths.json"

// NonSourcePathsProfile is the declaration's closed profile name.
const NonSourcePathsProfile = "corvint-non-source-paths/0"

// ClassDeclaredNonSource is the NonSource class of a dirty path the repository
// declares in NonSourcePathsPath (AZU-V0-002).
const ClassDeclaredNonSource = "DECLARED_NON_SOURCE"

// nonSourceReason prefixes the declared reason of every ClassDeclaredNonSource
// group.
const nonSourceReason = "declared non-source path (" + NonSourcePathsPath + "); every unit whose embed, enclosing directory, path literal, declared read scope or unbounded read reaches it is selected, and the repository declares that nothing else builds, runs or reads it; it stays unowned for the language frontier rules"

// NonSourcePath is one declared entry: a repository-relative file, or a
// directory when it ends in `/`, and the project's stated reason.
type NonSourcePath struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type nonSourcePathsFile struct {
	Profile string          `json:"profile"`
	Entries []NonSourcePath `json:"entries"`
}

// readNonSourcePaths returns the declared entries: nil with no declaration, an
// error for one that is unreadable, oversized, not a regular file, or invalid
// in any member. It takes the runtime-readers grammar and bounds: entries are
// strictly ascending by path, each with a non-empty printable reason, and none
// may be or cover `.corvint/`, a Corvint declaration, anything at or under
// `.github/`, or a make file.
func readNonSourcePaths(root *Source) ([]NonSourcePath, error) {
	if root == nil {
		return nil, nil
	}
	body, err := root.ReadBounded(NonSourcePathsPath, maxRuntimeReadersBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", NonSourcePathsPath, err)
	}
	var declared nonSourcePathsFile
	if err := json.UnmarshalRead(bytes.NewReader(body), &declared, json.RejectUnknownMembers(true)); err != nil {
		return nil, fmt.Errorf("%s: %w", NonSourcePathsPath, err)
	}
	if declared.Profile != NonSourcePathsProfile || len(declared.Entries) == 0 || len(declared.Entries) > maxRuntimeReadersEntries {
		return nil, fmt.Errorf("%s: profile must be %s with 1 to %d entries", NonSourcePathsPath, NonSourcePathsProfile, maxRuntimeReadersEntries)
	}
	previous := ""
	for _, entry := range declared.Entries {
		if !ValidReadScopeEntry(entry.Path) || entry.Path <= previous || !validRuntimeReaderPath(entry.Path) || !validNonSourcePath(entry.Path) {
			return nil, fmt.Errorf("%s: path %q", NonSourcePathsPath, entry.Path)
		}
		if entry.Reason == "" || len(entry.Reason) > maxRuntimeReaderReason || !utf8.ValidString(entry.Reason) || strings.ContainsFunc(entry.Reason, unicode.IsControl) {
			return nil, fmt.Errorf("%s: reason of %q", NonSourcePathsPath, entry.Path)
		}
		previous = entry.Path
	}
	return declared.Entries, nil
}

// validNonSourcePath refuses an entry that is, or as a directory covers, a
// Corvint declaration validRuntimeReaderPath does not already name.
func validNonSourcePath(entry string) bool {
	trimmed := strings.TrimSuffix(entry, "/")
	for _, declaration := range []string{UnboundedReaderExclusionsPath, DeclaredSelectionPath, RuntimeEdgesPath, ExistenceReadersPath} {
		if trimmed == declaration || (strings.HasSuffix(entry, "/") && strings.HasPrefix(declaration, entry)) {
			return false
		}
	}
	return true
}

// nonSourceEntry returns the index of the declared entry that holds path, or
// -1.
func (graph *Graph) nonSourceEntry(path string) int {
	for index, entry := range graph.nonSource {
		if path == entry.Path || (strings.HasSuffix(entry.Path, "/") && strings.HasPrefix(path, entry.Path)) {
			return index
		}
	}
	return -1
}

// declaredUnindexedOutput gives project-owned output declarations precedence
// over suffix-only language claims. Indexed units, driver inputs and task
// projections retain their source/ownership treatment (AZU-V0-9921).
func (graph *Graph) declaredUnindexedOutput(path string) bool {
	return graph.owner[path] == "" && !DriverInput(path) && !IsTaskStoreProjectionPath(path) && graph.nonSourceEntry(path) >= 0
}

// nonSourcePaths removes, from the unknown entries left after
// runtimeReaderPaths, each UNOWNED_DIRTY_PATH entry whose path a declared entry
// holds, and returns those paths as one ClassDeclaredNonSource group per entry,
// in declaration order (AZU-V0-002). The reader rules ran before this step, so
// every unit they name stays selected. Project declarations outrank a suffix-only
// claim on unindexed output (AZU-V0-9921). An indexed source, frontier entry,
// task store projection and driver input are never reclassified. The class is
// kept out of the frontier rules' non-source set,
// so a declared path still bears the AFP-V0-511 cascade like an unowned one.
func (graph *Graph) nonSourcePaths(unknown []Unknown) ([]NonSource, []Unknown) {
	if len(graph.nonSource) == 0 {
		return nil, unknown
	}
	byEntry := make(map[int][]string)
	kept := make([]Unknown, 0, len(unknown))
	for _, entry := range unknown {
		if entry.Reason == UnknownUnownedDirtyPath && !IsTaskStoreProjectionPath(entry.Detail) && !DriverInput(entry.Detail) {
			if index := graph.nonSourceEntry(entry.Detail); index >= 0 {
				byEntry[index] = append(byEntry[index], entry.Detail)
				continue
			}
		}
		kept = append(kept, entry)
	}
	var groups []NonSource
	for index, entry := range graph.nonSource {
		if paths := byEntry[index]; len(paths) != 0 {
			groups = append(groups, NonSource{Class: ClassDeclaredNonSource, Reason: nonSourceReason + ": " + entry.Reason, Paths: paths})
		}
	}
	return groups, kept
}
