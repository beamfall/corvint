// SPDX-License-Identifier: Apache-2.0

package liverepo_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/testsupport"
)

// overlayFS serves the fixture's files and falls back to the repository for
// every other path, so a frontier site's own file is read from the tree.
type overlayFS struct {
	files fstest.MapFS
	root  fs.FS
}

func (o overlayFS) Open(name string) (fs.File, error) {
	if _, ok := o.files[name]; ok {
		return o.files.Open(name)
	}
	return o.root.Open(name)
}

// OpenBounded is the bounded open affected.Source requires of an immutable
// source; Source itself enforces the limit after the open.
func (o overlayFS) OpenBounded(name string, _ int) (fs.File, error) { return o.Open(name) }

// flowGuideAlternatives returns, for each alternative of the declared
// tools/flow-guide-check predicate on internal/, one Go line that holds it:
// the literal codes as strings and the refuse alternative as a call.
func flowGuideAlternatives(t *testing.T, root string) []string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(affected.ReadScopeTreesPath)))
	if err != nil {
		t.Fatal(err)
	}
	trees, err := affected.ParseReadScopeTrees(body)
	if err != nil {
		t.Fatal(err)
	}
	value := trees["tools/flow-guide-check"]["internal/"]
	if value.Rule == nil || value.Rule.ContentPredicate == "" {
		t.Fatalf("tools/flow-guide-check declares no internal/ predicate: %+v", value)
	}
	var lines []string
	for _, alternative := range strings.Split(value.Rule.ContentPredicate, "|") {
		if regexp.QuoteMeta(alternative) == alternative {
			lines = append(lines, "var _ = \""+alternative+"\"")
		} else {
			lines = append(lines, "func f() { _ = refuseFixture(\"code\") }")
		}
	}
	return lines
}

// AFP-V0-9980 (V1-5364): over this repository's graph and declarations, a
// comment-only edit of a licensed internal/ file that holds no guide code and
// no refuse call selects no tools/flow-guide-check tests through its internal/
// tree; with the predicate before V1-5364 both files below selected it.
// Adding or removing each alternative the guard reads still selects it, and
// the command-tree snapshot stays a plain read. internal/analyzerruby/lexer.go
// is the V1-5365 reproduction file; this graph holds no TypeScript site edge
// for it, so that chain is covered by
// TestFrontierSiteObserverAppliesTreePredicate_AFPV09982 and the CLI
// measurement in docs/build-log/2026-11-11-guardsel.md.
func TestFlowGuidePredicateSelection_AFPV09980(t *testing.T) {
	testsupport.RequireLiveTier(t)
	graph, module := repositoryNarrowingGraph(t)
	root := repositoryRoot(t)
	const guard = "tools/flow-guide-check"
	repository := os.DirFS(root)
	selected := func(dirty, before, after string) (string, bool) {
		t.Helper()
		base := overlayFS{fstest.MapFS{dirty: {Data: []byte(before)}}, repository}
		head := overlayFS{fstest.MapFS{dirty: {Data: []byte(after)}}, repository}
		planned := graph.WithReadScopeSources(affected.FSSource(base), affected.FSSource(head), []string{dirty})
		for _, selection := range affected.Select(planned, []string{dirty}).Selected {
			if selection.UnitID == "go:"+module+"/"+guard {
				return selection.Witness.Kind + " via " + strings.Join(selection.Witness.Via, " -> "), true
			}
		}
		return "", false
	}
	for _, dirty := range []string{"internal/rubysyntax/lexer.go", "internal/analyzerruby/lexer.go"} {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(dirty)))
		if err != nil {
			t.Fatal(err)
		}
		original := string(raw)
		if kind, hit := selected(dirty, original, original+"\n// a comment-only edit\n"); hit {
			t.Errorf("%s: a comment-only edit selected %s as %s", dirty, guard, kind)
		}
		for _, line := range flowGuideAlternatives(t, root) {
			if _, hit := selected(dirty, original, original+"\n"+line+"\n"); !hit {
				t.Errorf("%s: adding %q did not select %s", dirty, line, guard)
			}
			if _, hit := selected(dirty, original+"\n"+line+"\n", original); !hit {
				t.Errorf("%s: removing %q did not select %s", dirty, line, guard)
			}
		}
	}
	if !strings.HasPrefix(mustRead(t, filepath.Join(root, "internal/rubysyntax/lexer.go")), "// SPDX-License-Identifier: Apache-2.0") {
		t.Fatal("the licensed fixture file lost its licence header")
	}
	// The command-tree snapshot is a plain read: any edit selects the guard.
	const snapshot = "internal/capcatalog/trees/corvint.json"
	tree := mustRead(t, filepath.Join(root, snapshot))
	if _, hit := selected(snapshot, tree, tree+"\n"); !hit {
		t.Errorf("an edit of %s did not select %s", snapshot, guard)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
