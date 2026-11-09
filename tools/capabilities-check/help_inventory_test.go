// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cliapp"
	"github.com/Beamfall/corvint/internal/cliprofile"
	"github.com/Beamfall/corvint/internal/clitree"
)

type helpRow struct {
	tree clitree.Tree
	node clitree.Node
}

func inventoryHelpRows(trees []clitree.Tree) []helpRow {
	var rows []helpRow
	for _, tree := range trees {
		for _, node := range tree.Nodes {
			rows = append(rows, helpRow{tree, node})
		}
	}
	return rows
}

// Compare the execution table against the entire inventory before executing it.
// A filtered, duplicate or stale row cannot silently reduce the denominator.
func helpTableProblems(trees []clitree.Tree, rows []helpRow) []string {
	key := func(binary string, path []string) string { return binary + " " + strings.Join(path, " ") }
	want := map[string]bool{}
	for _, tree := range trees {
		for _, node := range tree.Nodes {
			want[key(tree.Binary, node.Path)] = true
		}
	}
	seen := map[string]bool{}
	var problems []string
	for _, row := range rows {
		k := key(row.tree.Binary, row.node.Path)
		if !want[k] || seen[k] {
			problems = append(problems, "unexpected or duplicate row: "+k)
		}
		seen[k] = true
	}
	for k := range want {
		if !seen[k] {
			problems = append(problems, "missing row: "+k)
		}
	}
	return problems
}

type unreadableHelpInput struct{}

func (unreadableHelpInput) Read([]byte) (int, error) { panic("help read stdin") }

// CAP-V0-8800: the table covers the code-scanned inventory, not just whichever
// binaries a hand-maintained test list remembers. Process conformance remains in
// conformance/cli-errors-v0; this focused test also checks every field at depth.
func TestCAPV08800EveryInventoryNodeAnswersHelp(t *testing.T) {
	inv, err := Collect(helpPath, mainPath, cmdPath)
	if err != nil {
		t.Fatal(err)
	}
	if findings := CheckTrees("../..", inv.Binaries); len(findings) != 0 {
		t.Fatalf("inventory is incomplete: %v", findings)
	}
	if findings := CheckMachineCatalog("../.."); len(findings) != 0 {
		t.Fatalf("machine discovery is incomplete: %v", findings)
	}
	trees, err := readTrees("../../" + treesDir)
	if err != nil {
		t.Fatal(err)
	}
	rows := inventoryHelpRows(trees)
	if problems := helpTableProblems(trees, rows); len(problems) != 0 || len(rows) == 0 {
		t.Fatalf("help table: %v (%d rows)", problems, len(rows))
	}
	binDir := t.TempDir() // No native executable or repository is available.
	for _, row := range rows {
		t.Run(row.node.Command, func(t *testing.T) {
			for _, flag := range []string{"--help", "-h"} {
				args := append(append([]string{}, row.node.Path...), flag)
				var out, errout bytes.Buffer
				var code int
				if row.tree.Binary == "corvint-cli" {
					var handled bool
					code, handled = cliapp.Answer(row.tree.Binary, args, &out, &errout)
					if !handled {
						t.Fatal("adapter did not handle its own help")
					}
				} else {
					args = append([]string{"--binary", row.tree.Binary}, args...)
					code = cliprofile.Profile(context.Background(), binDir, args, unreadableHelpInput{}, &out, &errout)
				}
				if code != 0 || errout.Len() != 0 {
					t.Fatalf("%v: exit=%d stderr=%q", args, code, errout.String())
				}
				for _, fields := range [][]string{{row.node.Command, "Usage:", "Flags:", "Required:", "Example:", row.node.Example}, row.node.Usage, row.node.Flags, row.node.Required} {
					for _, field := range fields {
						if !strings.Contains(out.String(), field) {
							t.Errorf("%v omitted own contract %q: %s", args, field, out.String())
						}
					}
				}
			}
		})
	}
	// A known parent must not answer for an unknown child at any depth.
	for _, row := range rows {
		if row.tree.Binary == "corvint-cli" {
			continue // The adapter's native fallthrough is covered in cliapp.
		}
		args := append([]string{"--binary", row.tree.Binary}, row.node.Path...)
		args = append(args, "zz-missing-inventory-node", "--help")
		var out, errout bytes.Buffer
		if code := cliprofile.Profile(context.Background(), binDir, args, unreadableHelpInput{}, &out, &errout); code != 2 || out.Len() != 0 || !strings.Contains(errout.String(), `"code":"unknown-verb"`) {
			t.Errorf("unknown node %v: exit=%d stdout=%q stderr=%q", args, code, out.String(), errout.String())
		}
	}
}

func TestCAPV08800HelpTableOmissionFails(t *testing.T) {
	trees := []clitree.Tree{{Binary: "corvint-x", Nodes: []clitree.Node{{Path: []string{"alpha"}}, {Path: []string{"alpha", "beta"}}}}}
	rows := inventoryHelpRows(trees)
	for _, tc := range []struct {
		rows []helpRow
		bad  bool
	}{
		{rows, false}, {rows[:1], true}, {nil, true}, {append(append([]helpRow{}, rows...), rows[0]), true},
		{[]helpRow{{tree: trees[0], node: clitree.Node{Path: []string{"missing"}}}}, true},
	} {
		t.Run(fmt.Sprintf("rows=%d/bad=%t", len(tc.rows), tc.bad), func(t *testing.T) {
			if problems := helpTableProblems(trees, tc.rows); (len(problems) > 0) != tc.bad {
				t.Fatalf("bad=%t problems=%v", tc.bad, problems)
			}
		})
	}
}

// CAP-V0-8800: corvint-cli must reject an unknown child below a known
// two-word node, rather than serving the parent's otherwise valid help page.
func TestCAPV08800DeepUnknownChild(t *testing.T) {
	for _, flag := range []string{"--help", "-h"} {
		for _, unknown := range []bool{false, true} {
			args := []string{"--binary", "corvint", "docs", "corpus"}
			if unknown {
				args = append(args, "zz-unknown")
			}
			args = append(args, flag)
			var out, errout bytes.Buffer
			code := cliprofile.Profile(context.Background(), t.TempDir(), args, unreadableHelpInput{}, &out, &errout)
			if unknown {
				if code != 2 || out.Len() != 0 || !strings.Contains(errout.String(), `"code":"unknown-verb"`) {
					t.Fatalf("%v: exit=%d stdout=%q stderr=%q", args, code, out.String(), errout.String())
				}
			} else if code != 0 || !strings.Contains(out.String(), "corvint docs corpus") || errout.Len() != 0 {
				t.Fatalf("%v: exit=%d stdout=%q stderr=%q", args, code, out.String(), errout.String())
			}
		}
	}
}
