// SPDX-License-Identifier: Apache-2.0
package shell

import (
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AFP-V0-9133: a bare directory operand of a listing command names the files
// below it, so a file added to or deleted from the directory selects the
// lister; a file elsewhere does not.
func TestShellListingDirectoryOperand_AFPV09133(t *testing.T) {
	root := t.TempDir()
	write(t, root, "script/a.sh", "true\n")
	write(t, root, "script/b.sh", "true\n")
	write(t, root, "script/new_test.sh", "true\n")
	write(t, root, "docs/x.md", "x\n")
	write(t, root, "lister_test.sh", "test \"$(ls script | wc -l)\" -eq 3\n")
	write(t, root, "finder_test.sh", "find script -maxdepth 1 -type f -name '*.sh' -exec sh {} \\\\;\n")
	write(t, root, "searcher_test.sh", "! grep -rn FIXME script\n")
	write(t, root, "reader_test.sh", "cat script/a.sh\n")
	// A helper source keeps the previous relation: no bare-operand token.
	write(t, root, "tools/list-helper.sh", "ls script | wc -l\n")
	graph := build(t, root)
	if unit, ok := graph.Unit("shell:tools/list-helper.sh"); !ok || slices.Contains(unit.PathTokens, "/script/") {
		t.Errorf("list-helper tokens %v; want no /script/ token on a helper", unit.PathTokens)
	}
	for _, id := range []string{"shell:lister_test.sh", "shell:finder_test.sh", "shell:searcher_test.sh"} {
		unit, ok := graph.Unit(id)
		if !ok || !slices.Contains(unit.PathTokens, "/script/") || unit.UnboundedReads != "" {
			t.Fatalf("%s: tokens %v unbounded %q; want /script/ and bounded", id, unit.PathTokens, unit.UnboundedReads)
		}
	}
	listers := []string{"lister_test.sh", "finder_test.sh", "searcher_test.sh"}
	// An added file is present in the checkout; a deleted one is absent.
	for _, dirty := range []string{"script/new_test.sh", "script/gone.sh"} {
		selected := affected.Select(graph, []string{dirty}).SelectedTests()
		for _, lister := range listers {
			if !slices.Contains(selected, lister) {
				t.Errorf("%s: %s not selected: %v", dirty, lister, selected)
			}
		}
		if slices.Contains(selected, "reader_test.sh") {
			t.Errorf("%s: reader_test.sh selected: %v", dirty, selected)
		}
	}
	selected := affected.Select(graph, []string{"docs/x.md"}).SelectedTests()
	for _, lister := range listers {
		if slices.Contains(selected, lister) {
			t.Errorf("docs/x.md: %s selected: %v", lister, selected)
		}
	}
}

// AFP-V0-9134: a listing, walking or searching command rooted at the working
// directory makes the unit an unbounded reader, selected for a file added,
// deleted or edited anywhere; one rooted at a literal directory stays bounded.
func TestShellTreeWalkerUnbounded_AFPV09134(t *testing.T) {
	walkers := map[string]string{
		"find_count":  "test \"$(find . -name '*.sh' | wc -l)\" -eq 2\n",
		"grep_tree":   "! grep -rn FIXME .\n",
		"find_md":     "find . -name '*.md' | wc -l\n",
		"find_bare":   "find -name '*.sh' -exec sh {} \\\\;\n",
		"rg_default":  "! rg FIXME\n",
		"ag_dot":      "! ag FIXME ./\n",
		"du_cwd":      "du -sh\n",
		"ls_r":        "ls -R . | wc -l\n",
		"grep_r_only": "! grep -r FIXME\n",
	}
	bounded := map[string]string{
		"find_dir":    "find script -name '*.sh' | wc -l\n",
		"grep_dir":    "! grep -rn FIXME script\n",
		"grep_plain":  "! grep -n FIXME script/a.sh\n",
		"ls_plain":    "ls . | wc -l\n",
		"rg_file":     "! rg FIXME script/a.sh\n",
		"find_temp":   "tmp=$(mktemp -d)\nfind \"$tmp\" -name '*.sh' | wc -l\n",
		"cat_file":    "cat script/a.sh\n",
		"grep_dash":   "grep -Fq -- \"- N-1 release precedes\" \"$scratch/err\"\n",
		"grep_quoted": "grep -Fq '-r' script/a.sh\n",
	}
	root := t.TempDir()
	write(t, root, "script/a.sh", "true\n")
	write(t, root, "script/new.sh", "true\n")
	write(t, root, "internal/x.go", "package x\n")
	write(t, root, "docs/new.md", "x\n")
	// A helper source that walks the tree keeps its bounded relation.
	write(t, root, "script/walk-helper.sh", "find . -name '*.sh' | wc -l\n")
	for name, body := range walkers {
		write(t, root, name+"_test.sh", body)
	}
	for name, body := range bounded {
		write(t, root, name+"_test.sh", body)
	}
	graph := build(t, root)
	for name := range walkers {
		unit, ok := graph.Unit("shell:" + name + "_test.sh")
		if !ok || unit.UnboundedReads != UnboundedTreeWalk {
			t.Errorf("%s: unbounded %q; want %s", name, unit.UnboundedReads, UnboundedTreeWalk)
		}
	}
	for name := range bounded {
		unit, ok := graph.Unit("shell:" + name + "_test.sh")
		if !ok || unit.UnboundedReads != "" {
			t.Errorf("%s: unbounded %q; want bounded", name, unit.UnboundedReads)
		}
	}
	if unit, ok := graph.Unit("shell:script/walk-helper.sh"); !ok || unit.UnboundedReads != "" {
		t.Errorf("walk-helper: unbounded %q; want a bounded source", unit.UnboundedReads)
	}
	for _, dirty := range []string{"script/new.sh", "internal/x.go", "docs/new.md"} {
		selected := affected.Select(graph, []string{dirty}).SelectedTests()
		for name := range walkers {
			if !slices.Contains(selected, name+"_test.sh") {
				t.Errorf("%s: %s not selected: %v", dirty, name, selected)
			}
		}
		if slices.Contains(selected, "cat_file_test.sh") {
			t.Errorf("%s: cat_file selected: %v", dirty, selected)
		}
		if dirty == "docs/new.md" && slices.Contains(selected, "find_dir_test.sh") {
			t.Errorf("%s: find_dir selected: %v", dirty, selected)
		}
		if dirty == "script/new.sh" && !slices.Contains(selected, "find_dir_test.sh") {
			t.Errorf("%s: find_dir not selected: %v", dirty, selected)
		}
	}
}
