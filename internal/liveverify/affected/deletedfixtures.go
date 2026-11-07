// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// deletedFixture reports a dirty path that a non-Go plugin claims as source,
// that is absent from the checkout the graph was observed from, and that lies
// below a `testdata` directory a Go unit hosts (AFP-V0-446's hosting rule),
// where no claimant lists a path of the same fixture subtree (fixtureSubtree)
// and none reports the path inside a directory it does not observe
// (AFP-V0-2740). It is the AFP-V0-2440 fixture-only claim of a deleted or
// moved-away file: the Go toolchain builds and runs nothing below `testdata`,
// the hosting package's tests read the file as bytes, and no unit of the
// claimant in that subtree survives to have depended on the file. A unit
// elsewhere that names it by an import or path literal is a rule (c) reader
// of the dirty path (AFP-V0-021), selected as for any unowned path, and the
// enclosing Go package is selected by rule (b). The path raises no
// UNINDEXED_SOURCE_PATH. It still stays unowned, so frontierUnknowns treats it
// as any other unowned path (AFP-V0-511). A path present on disk, a graph with
// no disk root or a sparse checkout (absence may only mean outside the cone),
// a Go claim, a path no plugin claims (already UNOWNED_DIRTY_PATH), a fixture
// no Go unit hosts, and a subtree a claimant still lists keep the entry.
func (graph *Graph) deletedFixture(path string, packages map[string]string) bool {
	if graph.root == "" || graph.sparse || !fs.ValidPath(path) {
		return false
	}
	fixture := fixtureDirectory(path)
	if fixture == "" || !graph.goUnitAbove(fixture, packages) {
		return false
	}
	claimed := graph.claimantsOf(path)
	if len(claimed) == 0 {
		return false
	}
	subtree := fixtureSubtree(path, fixture)
	for _, name := range claimed {
		if name == "go" {
			return false
		}
		if _, unobserved := underDirectory(path, graph.unobserved[name]); unobserved {
			return false
		}
		if graph.listsBelowAny(name, []string{subtree}) {
			return false
		}
	}
	_, err := os.Lstat(filepath.Join(graph.root, filepath.FromSlash(path)))
	return errors.Is(err, fs.ErrNotExist)
}

// fixtureSubtree is the fixture a path below the `testdata` directory fixture
// belongs to: the first directory below fixture on the path, or fixture itself
// for a file directly in it. Fixtures of one `testdata` directory are separate
// trees a test copies or reads on its own, so a plugin's unit in a sibling
// fixture cannot have held the file.
func fixtureSubtree(path, fixture string) string {
	rest := strings.TrimPrefix(path, fixture+"/")
	first, _, nested := strings.Cut(rest, "/")
	if !nested {
		return fixture
	}
	return fixture + "/" + first
}
