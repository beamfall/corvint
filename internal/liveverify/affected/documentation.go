package affected

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"strings"
)

// DocumentationPathsPath is the repository's declaration of the paths that
// are documentation rather than build or test inputs (AFP-V0-160).
const DocumentationPathsPath = ".corvint/documentation-paths.json"

// DocumentationPathsProfile is the declaration's closed profile name.
const DocumentationPathsProfile = "corvint-documentation-paths/0"

// ClassDeclaredDocumentation is the NonSource class of a dirty path the
// repository declares as documentation (AFP-V0-160).
const ClassDeclaredDocumentation = "DECLARED_DOCUMENTATION"

// declaredDocumentationReason is the stated reason of every
// ClassDeclaredDocumentation entry.
const declaredDocumentationReason = "declared documentation path (" + DocumentationPathsPath + "); every unit whose path literal, enclosing directory or declared read scope reaches it is selected, and no other build or test input names it"

const (
	maxDocumentationBytes   = 64 << 10
	maxDocumentationEntries = 256
)

type documentationFile struct {
	Profile string   `json:"profile"`
	Paths   []string `json:"paths"`
}

// readDocumentationPaths returns the declared documentation entries: nil with
// no declaration, an error for one that is unreadable, oversized, not a
// regular file, or invalid in any member (AFP-V0-160 (1)). An entry is a
// repository-relative file, or a directory when it ends in `/`; the entries
// are strictly ascending, and none may be or lie under `.corvint/`, so the
// declaration can never classify itself or another Corvint declaration.
func readDocumentationPaths(root *Source) ([]string, error) {
	if root == nil {
		return nil, nil
	}
	body, err := root.ReadBounded(DocumentationPathsPath, maxDocumentationBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", DocumentationPathsPath, err)
	}
	var declared documentationFile
	if err := json.UnmarshalRead(bytes.NewReader(body), &declared, json.RejectUnknownMembers(true)); err != nil {
		return nil, fmt.Errorf("%s: %w", DocumentationPathsPath, err)
	}
	if declared.Profile != DocumentationPathsProfile || declared.Paths == nil || len(declared.Paths) > maxDocumentationEntries {
		return nil, fmt.Errorf("%s: profile must be %s with at most %d paths", DocumentationPathsPath, DocumentationPathsProfile, maxDocumentationEntries)
	}
	previous := ""
	for _, entry := range declared.Paths {
		trimmed := strings.TrimSuffix(entry, "/")
		if !ValidReadScopeEntry(entry) || entry <= previous || trimmed == ".corvint" || strings.HasPrefix(trimmed, ".corvint/") {
			return nil, fmt.Errorf("%s: path %q", DocumentationPathsPath, entry)
		}
		previous = entry
	}
	return declared.Paths, nil
}

// declaredDocumentation reports whether path is a declared documentation
// entry or lies under a declared directory entry.
func (graph *Graph) declaredDocumentation(path string) bool {
	for _, entry := range graph.documentation {
		if path == entry || (strings.HasSuffix(entry, "/") && strings.HasPrefix(path, entry)) {
			return true
		}
	}
	return false
}

// documentation removes, from the unknown entries left after taskState, each
// UNOWNED_DIRTY_PATH entry whose path the repository declares as
// documentation, and returns those paths, in dirty-path order, as one
// ClassDeclaredDocumentation group, or no group when there are none
// (AFP-V0-160). Unlike a task store projection, a declared documentation path
// that a unit names is still classified: the reader rules have already
// selected every unit that names it, which are its documentation checks, and
// the declaration states that nothing else reads it. A path a plugin claims as
// source (UNINDEXED_SOURCE_PATH) is never reclassified.
func (graph *Graph) documentationPaths(unknown []Unknown) ([]NonSource, []Unknown) {
	if len(graph.documentation) == 0 {
		return nil, unknown
	}
	paths := make([]string, 0)
	kept := make([]Unknown, 0, len(unknown))
	for _, entry := range unknown {
		if entry.Reason == UnknownUnownedDirtyPath && !IsTaskStoreProjectionPath(entry.Detail) && graph.declaredDocumentation(entry.Detail) {
			paths = append(paths, entry.Detail)
			continue
		}
		kept = append(kept, entry)
	}
	if len(paths) == 0 {
		return nil, kept
	}
	return []NonSource{{Class: ClassDeclaredDocumentation, Reason: declaredDocumentationReason, Paths: paths}}, kept
}
