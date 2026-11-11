// SPDX-License-Identifier: Apache-2.0
package shell

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// symlinkOperandTests are the units of symlinkOperandTree whose output a
// change through a link can alter.
var symlinkOperandTests = []string{
	"dangle_test.sh", "doclister_test.sh", "greader_test.sh", "into_test.sh",
	"lister_test.sh", "reader_test.sh", "listed_test.sh", "sub_test.sh",
}

// symlinkChange labels how a row's change alters the tree. The planner is
// given no change status, so the label only documents the row and decides
// which side of the change a link is read from.
type symlinkChange string

const (
	changeAdded       symlinkChange = "added"
	changeDeleted     symlinkChange = "deleted"
	changeModified    symlinkChange = "modified"
	changeTypeChanged symlinkChange = "type-changed"
)

// symlinkOperandTree builds the head tree of V1-5368: tracked symbolic links
// to directories, one unit listing through each and one reading a file
// through one. Each change of TestSymlinkOperand_V15368 is one dirty path
// against this tree, so an added file is present and a deleted one absent.
func symlinkOperandTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for relative, body := range map[string]string{
		"real/a.sh":                         "true\n",
		"real/new.sh":                       "true\n", // added
		"real/new.txt":                      "x\n",    // added, a data file no plugin owns
		"real/sub/deep.sh":                  "true\n",
		"real/sub/new.sh":                   "true\n", // added
		"listed/a.sh":                       "true\n",
		"listed/sub/s.sh":                   "true\n",
		"listed/sub/new.sh":                 "true\n", // added
		"ext/e.sh":                          "true\n",
		"ext/new.sh":                        "true\n", // added
		"missing/x.sh":                      "true\n", // added below a link's dangling target
		"guide/a.md":                        "x\n",
		"guide/new.md":                      "x\n", // added, declared documentation
		".corvint/documentation-paths.json": `{"profile":"corvint-documentation-paths/0","paths":["guide/"]}` + "\n",
		"lister_test.sh":                    "ls link | wc -l\n",
		"reader_test.sh":                    "cat link/a.sh\n",
		"sub_test.sh":                       "ls link/sub | wc -l\n",
		"listed_test.sh":                    "ls -R listed | wc -l\n",
		"into_test.sh":                      "ls intolisted | wc -l\n",
		"dangle_test.sh":                    "ls dangle | wc -l\n",
		"doclister_test.sh":                 "ls docslink | wc -l\n",
		"greader_test.sh":                   "cat guide/glink/a.sh\n",
	} {
		write(t, root, relative, body)
	}
	for link, target := range map[string]string{
		"link":        "real",       // the suspected case
		"listed/out":  "../ext",     // inside a listed tree, to a directory outside it
		"intolisted":  "listed/sub", // to a directory below another listed directory
		"dangle":      "missing",    // dangling until the change adds below it
		"docslink":    "guide",      // to declared documentation
		"guide/glink": "../real",    // a link that is itself declared documentation
	} {
		if err := os.Symlink(target, filepath.Join(root, filepath.FromSlash(link))); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// V1-5368: a shell unit that lists, or reads a file, through a tracked
// symbolic link to a directory names the link's path, and the planner
// matches tokens by path text, so a dirty path below the link's target is
// matched by no token of the unit. Each row is one change; reads holds the
// tests whose output that change can alter (the ground truth: `ls` follows a
// link named as its operand, `ls -R` does not follow a link it meets while
// recursing), and missed the subset today's plan neither selects nor covers
// by an unknown. A test in missed is a missed test.
func TestSymlinkOperand_V15368(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating a symbolic link needs a privilege Windows test hosts do not grant")
	}
	graph := build(t, symlinkOperandTree(t))
	for _, row := range []struct {
		name   string
		dirty  string
		status symlinkChange
		reads  []string
		missed []string
	}{
		// (a) A file added below the link's target changes `ls link` and
		// selects nothing: CONFIRMED, it can miss lister_test.sh.
		{"a_added_below_target", "real/new.sh", changeAdded, []string{"lister_test.sh"}, []string{"lister_test.sh"}},
		// (a) An added data file is unowned, so UNOWNED_DIRTY_PATH covers it
		// by chance, not by a rule about the link.
		{"a_added_data_below_target", "real/new.txt", changeAdded, []string{"lister_test.sh"}, nil},
		// (b) A deleted shell file raises UNINDEXED_SOURCE_PATH, which covers
		// it by chance.
		{"b_deleted_below_target", "real/gone.sh", changeDeleted, []string{"lister_test.sh"}, nil},
		// (b) A path the repository declares documentation raises no unknown,
		// so a deletion below a link to it is missed: CONFIRMED.
		{"b_deleted_below_documentation_target", "guide/old.md", changeDeleted, []string{"doclister_test.sh"}, []string{"doclister_test.sh"}},
		{"a_added_below_documentation_target", "guide/new.md", changeAdded, []string{"doclister_test.sh"}, []string{"doclister_test.sh"}},
		// (c) An edit of an existing file below the target cannot change a
		// listing, but changes what `cat link/a.sh` reads, and selects
		// nothing: CONFIRMED, it can miss reader_test.sh. The miss is in the
		// content read, not in a listing.
		{"c_edited_below_target", "real/a.sh", changeModified, []string{"reader_test.sh"}, []string{"reader_test.sh"}},
		{"c_edited_deep_below_target", "real/sub/deep.sh", changeModified, nil, nil},
		// A trailing component through the link: CONFIRMED, it can miss.
		{"trailing_component", "real/sub/new.sh", changeAdded, []string{"sub_test.sh"}, []string{"sub_test.sh"}},
		// A link inside a listed tree to a directory outside it: `ls -R`
		// does not follow it, so the addition changes no output. REFUTED.
		{"outside_listed_tree", "ext/new.sh", changeAdded, nil, nil},
		{"outside_listed_tree_link_retargeted", "listed/out", changeTypeChanged, []string{"listed_test.sh"}, nil},
		// A link to a directory below another listed directory: the other
		// listing is selected, the one through the link is not. CONFIRMED.
		{"target_below_listed_directory", "listed/sub/new.sh", changeAdded, []string{"into_test.sh", "listed_test.sh"}, []string{"into_test.sh"}},
		// The link retargeted: its own path is unowned, so
		// UNOWNED_DIRTY_PATH covers the readers through it. REFUTED here...
		{"link_retargeted", "link", changeTypeChanged, []string{"lister_test.sh", "reader_test.sh", "sub_test.sh"}, nil},
		// ...but a link the repository declares documentation raises no
		// unknown, and `cat guide/glink/a.sh` is not selected: CONFIRMED.
		{"documentation_link_retargeted", "guide/glink", changeTypeChanged, []string{"greader_test.sh"}, []string{"greader_test.sh"}},
		// A file added below a dangling link's target: CONFIRMED.
		{"dangling_target_added", "missing/x.sh", changeAdded, []string{"dangle_test.sh"}, []string{"dangle_test.sh"}},
		// The dangling link itself deleted is matched by its own path.
		{"dangling_link_deleted", "dangle", changeDeleted, []string{"dangle_test.sh"}, nil},
	} {
		t.Run(row.name, func(t *testing.T) {
			plan := affected.Select(graph, []string{row.dirty})
			selected := plan.SelectedTests()
			var missed []string
			if plan.Scope != affected.ScopeUnknown {
				for _, test := range row.reads {
					if !slices.Contains(selected, test) {
						missed = append(missed, test)
					}
				}
			}
			if !slices.Equal(missed, row.missed) {
				t.Errorf("%s %s: missed %v, want %v (selected %v, unknown %v)", row.dirty, row.status, missed, row.missed, selected, plan.Unknown)
			}
			for _, test := range selected {
				if !slices.Contains(symlinkOperandTests, test) {
					t.Errorf("%s: unexpected unit %s selected", row.dirty, test)
				}
			}
		})
	}
}
