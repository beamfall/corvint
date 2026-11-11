// SPDX-License-Identifier: Apache-2.0

package liverepo_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/testsupport"
)

// Neither probe revision may match a guard predicate: no "git", no
// hyphenated or upper-snake word and no refuse name.
const (
	treePredicateProbeBase = "package probe\n\nconst probe = 1\n"
	treePredicateProbeHead = "package probe\n\nconst probe = 2\n"
)

// TestRepositoryTreePredicatesSelectOnlyMatchingEdits_AFP_V0_3300 is V1-5174
// criterion 3: for every content predicate this repository declares in
// .corvint/test-read-scope-trees.json, an edit under the tree whose content
// matches the guard-derived sample selects the guard, and an edit that matches
// at neither endpoint does not (AFP-V0-3300, AFP-V0-3302).
func TestRepositoryTreePredicatesSelectOnlyMatchingEdits_AFP_V0_3300(t *testing.T) {
	testsupport.RequireLiveTier(t)
	root := repositoryRoot(t)
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(affected.ReadScopeTreesPath)))
	if err != nil {
		t.Fatal(err)
	}
	trees, err := affected.ParseReadScopeTrees(body)
	if err != nil {
		t.Fatal(err)
	}
	graph, module := repositoryNarrowingGraph(t)
	packages := make([]string, 0, len(trees))
	for pkg := range trees {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)
	checked := 0
	for _, pkg := range packages {
		names := make([]string, 0, len(trees[pkg]))
		for tree := range trees[pkg] {
			names = append(names, tree)
		}
		sort.Strings(names)
		for _, tree := range names {
			rule := trees[pkg][tree].Rule
			if rule == nil || rule.ContentPredicate == "" {
				continue
			}
			checked++
			t.Run(pkg+"/"+tree, func(t *testing.T) {
				id := "go:" + module + "/" + pkg
				unit, found := graph.Unit(id)
				if !found {
					t.Fatalf("guard %s is not a graph unit", id)
				}
				if !slices.ContainsFunc(unit.ReadScopeTrees, func(admitted affected.ReadScopeTree) bool {
					return admitted.Tree == tree && admitted.ContentPredicate == rule.ContentPredicate
				}) {
					t.Fatalf("graph did not admit the declared predicate %q for %s: %+v", rule.ContentPredicate, tree, unit.ReadScopeTrees)
				}
				predicate := regexp.MustCompile(rule.ContentPredicate)
				if len(rule.Samples) == 0 || !predicate.MatchString(rule.Samples[0]) {
					t.Fatalf("predicate %q has no matching guard sample: %q", rule.ContentPredicate, rule.Samples)
				}
				if predicate.MatchString(treePredicateProbeBase) || predicate.MatchString(treePredicateProbeHead) {
					t.Fatalf("probe text matches %q; choose a probe the predicate cannot match", rule.ContentPredicate)
				}
				dirty := tree + "readscopeprobe/probe.go"
				for _, edit := range []struct {
					name, base, head string
					want             bool
				}{
					{"sample added", treePredicateProbeBase, treePredicateProbeBase + "// " + rule.Samples[0] + "\n", true},
					{"sample removed", treePredicateProbeBase + "// " + rule.Samples[0] + "\n", treePredicateProbeBase, true},
					{"neither matches", treePredicateProbeBase, treePredicateProbeHead, false},
				} {
					base := affected.FSSource(fstest.MapFS{dirty: {Data: []byte(edit.base)}})
					head := affected.FSSource(fstest.MapFS{dirty: {Data: []byte(edit.head)}})
					selected := false
					var witness affected.Witness
					for _, selection := range affected.Select(graph.WithReadScopeSources(base, head, []string{dirty}), []string{dirty}).Selected {
						if selection.UnitID == id {
							selected, witness = true, selection.Witness
						}
					}
					if selected != edit.want {
						t.Errorf("%s: %s selected=%v want %v (witness %+v)", edit.name, pkg, selected, edit.want, witness)
					}
				}
			})
		}
	}
	if checked == 0 {
		t.Fatalf("%s declares no content predicate; this test would be vacuous", affected.ReadScopeTreesPath)
	}
}
