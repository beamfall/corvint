package affected

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// admittedDeletion reports a dirty path no unit declares that seed attributes
// as a deletion, without UNINDEXED_SOURCE_PATH, although a plugin's Owns claims
// it: a deleted Go source of a surviving package (deletedGoSource,
// AFP-V0-2610), a wholly deleted Go package (deletedGoPackage, AFP-V0-821) or a
// deleted fixture (deletedFixture, AFP-V0-2740) or a deleted shell script
// (deletedShellScript, AFP-V0-9992). Such a path stays unowned for
// frontierUnknowns (AFP-V0-2749): seed and participants share this one
// predicate, so every path the plan admits this way is still checked against
// every other plugin's unowned-path entries (AFP-V0-511), such as a test in
// another language that reads the deleted file by a computed name, which the
// UNINDEXED_SOURCE_PATH fallback covered before the path was admitted.
func (graph *Graph) admittedDeletion(path string, packages map[string]string) bool {
	return graph.deletedGoSource(path, packages) || graph.deletedGoPackage(path, packages) || graph.deletedFixture(path, packages) || graph.deletedShellScript(path)
}

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
// and raises no UNINDEXED_SOURCE_PATH; it stays unowned for every other
// plugin's frontier checks (admittedDeletion, AFP-V0-2749). A graph without a disk root, a path
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
// as any other unowned path (admittedDeletion, AFP-V0-2749). A command edge (AFP-V0-037) names only an
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
