package affected

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"slices"
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

// ClassCorvintDeclaration is the NonSource class of the documentation
// declaration itself when it is a dirty path no unit names (AFP-V0-443).
const ClassCorvintDeclaration = "CORVINT_DECLARATION"

// corvintDeclarationReason is the stated reason of every
// ClassCorvintDeclaration entry.
const corvintDeclarationReason = "Corvint declaration read only by the affected planner, which classified this plan's paths by its current bytes; no unit's path literal, declared read scope or unbounded read reaches it, so it is no build or test input"

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
	_, paths, err := readDocumentationDeclaration(root)
	return paths, err
}

// ReadDocumentationDeclaration reads the declaration under root by the
// AFP-V0-160 (1) rules and returns the SHA-256 of its bytes and its entries,
// or an empty digest and nil entries when root declares nothing, so a plan
// that applied it can record which declaration excluded a path (TJAA-V0-101).
func ReadDocumentationDeclaration(root string) (string, []string, error) {
	body, paths, err := readDocumentationDeclaration(DiskSource(root))
	if err != nil || paths == nil {
		return "", nil, err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), paths, nil
}

// DeclaredDocumentation returns the documentation entries the graph was built
// with (AFP-V0-160), or nil when the repository declares none.
func (graph *Graph) DeclaredDocumentation() []string {
	return slices.Clone(graph.documentation)
}

func readDocumentationDeclaration(root *Source) ([]byte, []string, error) {
	if root == nil {
		return nil, nil, nil
	}
	body, err := root.ReadBounded(DocumentationPathsPath, maxDocumentationBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", DocumentationPathsPath, err)
	}
	var declared documentationFile
	if err := json.UnmarshalRead(bytes.NewReader(body), &declared, json.RejectUnknownMembers(true)); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", DocumentationPathsPath, err)
	}
	if declared.Profile != DocumentationPathsProfile || declared.Paths == nil || len(declared.Paths) > maxDocumentationEntries {
		return nil, nil, fmt.Errorf("%s: profile must be %s with at most %d paths", DocumentationPathsPath, DocumentationPathsProfile, maxDocumentationEntries)
	}
	previous := ""
	for _, entry := range declared.Paths {
		trimmed := strings.TrimSuffix(entry, "/")
		if !ValidReadScopeEntry(entry) || entry <= previous || trimmed == ".corvint" || strings.HasPrefix(trimmed, ".corvint/") {
			return nil, nil, fmt.Errorf("%s: path %q", DocumentationPathsPath, entry)
		}
		previous = entry
	}
	return body, declared.Paths, nil
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

// declarationPath removes, from the unknown entries left after
// documentationPaths and runtimeReaderPaths, the UNOWNED_DIRTY_PATH entry of
// each Corvint declaration (DocumentationPathsPath, and RuntimeReadersPath by
// AFP-V0-602 (4)) that no unit names, and returns them as one
// ClassCorvintDeclaration group, or no group otherwise (AFP-V0-443). As for a
// Tasks store projection (AFP-V0-088), a unit whose path literal resolves to
// the declaration or whose declared read scope holds it keeps the unknown
// entry: that unit reads it. Unlike a projection, any unit the change would
// reach, including a directory-fragment reader or an unbounded reader, also
// keeps it.
func (graph *Graph) declarationPath(unknown []Unknown) ([]NonSource, []Unknown) {
	var paths []string
	kept := unknown
	for _, declaration := range []string{DocumentationPathsPath, RuntimeReadersPath} {
		index := slices.IndexFunc(kept, func(entry Unknown) bool {
			return entry.Reason == UnknownUnownedDirtyPath && entry.Detail == declaration
		})
		if index < 0 || graph.named(declaration, graph.pathTokens()) || len(graph.ReachedUnitIDs([]string{declaration})) != 0 {
			continue
		}
		kept = slices.Delete(slices.Clone(kept), index, index+1)
		paths = append(paths, declaration)
	}
	if len(paths) == 0 {
		return nil, unknown
	}
	return []NonSource{{Class: ClassCorvintDeclaration, Reason: corvintDeclarationReason, Paths: paths}}, kept
}
