package affected

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// deletedGoTest reports a dirty `_test.go` path that only the Go plugin
// claims, that is absent from the checkout the graph was observed from, and
// whose directory still holds a Go package the graph observes (AFP-V0-620).
// The path is a test file deleted or moved away since the change's base. Go
// compiles a `_test.go` file only into the test binary of the package in its
// own directory, and a directory holds one package, so that package is the
// file's base package; rule (a) (goSourceRule) already selects it. Such a
// path is attributed and raises no UNINDEXED_SOURCE_PATH. A graph without a
// disk root, a path present on disk (a new or build-excluded file the graph
// cannot see into), a path under a directory the plugin does not observe, a
// path another plugin also claims, or a directory with no observed package
// keeps the entry, because the base package then cannot be named.
func (graph *Graph) deletedGoTest(path string, packages map[string]string) bool {
	if graph.root == "" || !strings.HasSuffix(path, "_test.go") || !fs.ValidPath(path) {
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
