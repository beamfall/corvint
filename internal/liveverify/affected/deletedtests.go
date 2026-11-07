package affected

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// deletedGoSource reports a dirty `.go` path that only the Go plugin claims,
// that is absent from the checkout the graph was observed from, and whose
// directory still holds a Go package the graph observes: a file deleted or
// moved away since the change's base. A directory holds one package, and a
// file of it compiles into no other package whatever its build constraints,
// so that package is the file's base package. Rule (a) (goSourceRule) already
// selects it with a DIRECT_SOURCE_CHANGE witness, a traversal root, so its
// importers are selected too, as for an edit to one of its files; a deleted
// file is never additive (ADN-V0-004). A `_test.go` file only reaches its own
// package's test binary (AFP-V0-620); any other Go file reaches the package's
// importers through that traversal (AFP-V0-2610). Such a path is attributed
// and raises no UNINDEXED_SOURCE_PATH. A graph without a disk root, a path
// present on disk (a new or build-excluded file the graph cannot see into), a
// path under a directory the plugin does not observe, a path another plugin
// also claims, or a directory with no observed package keeps the entry,
// because the base package then cannot be named.
func (graph *Graph) deletedGoSource(path string, packages map[string]string) bool {
	if graph.root == "" || !strings.HasSuffix(path, ".go") || !fs.ValidPath(path) {
		return false
	}
	if claimed := graph.claimantsOf(path); len(claimed) != 1 || claimed[0] != "go" {
		return false
	}
	if _, unobserved := underDirectory(path, graph.unobserved["go"]); unobserved {
		return false
	}
	if _, present := packages[relativeDirectory(path)]; !present {
		return false
	}
	_, err := os.Lstat(filepath.Join(graph.root, filepath.FromSlash(path)))
	return errors.Is(err, fs.ErrNotExist)
}

// deletedGoPackage reports a dirty `.go` path that only the Go plugin claims,
// outside every directory the plugin does not observe, whose directory holds
// no observed package and is itself absent from the checkout (AFP-V0-821):
// the whole package was deleted or moved away since the change's base. Rule
// (a) (goSourceRule) already selects every unit with an edge to the absent
// package, a superset of its importers, so the path raises no
// UNINDEXED_SOURCE_PATH. It still stays unowned, so frontierUnknowns treats it
// as any other unowned path. A command edge (AFP-V0-037) names only an
// observed command, and a declaration naming an absent one is already a
// frontier entry, so no runner of the deleted package goes unnamed. A
// directory still present on disk (a file the graph cannot see into may
// remain) or a graph without a disk root keeps the entry.
func (graph *Graph) deletedGoPackage(path string, packages map[string]string) bool {
	// A sparse checkout's absent directory may only lie outside the cone.
	if graph.root == "" || graph.sparse || !strings.HasSuffix(path, ".go") || !fs.ValidPath(path) {
		return false
	}
	directory := relativeDirectory(path)
	if directory == "." {
		return false
	}
	if claimed := graph.claimantsOf(path); len(claimed) != 1 || claimed[0] != "go" {
		return false
	}
	if _, unobserved := underDirectory(path, graph.unobserved["go"]); unobserved {
		return false
	}
	if _, present := packages[directory]; present {
		return false
	}
	_, err := os.Lstat(filepath.Join(graph.root, filepath.FromSlash(directory)))
	return errors.Is(err, fs.ErrNotExist)
}
