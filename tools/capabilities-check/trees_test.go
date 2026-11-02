// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/clitree"
)

// TestCAPV0047RepositoryCommandTreesMatchDispatch is the command-tree drift gate on this revision:
// every snapshot is well formed and names everything its binary dispatches, and every other
// binary is on the ratchet.
func TestCAPV0047RepositoryCommandTreesMatchDispatch(t *testing.T) {
	inv, _, _ := load(t)
	for _, f := range CheckTrees("../..", inv.Binaries) {
		t.Errorf("%s (add the subcommand or flag to the binary's help, or a reasoned internal entry; see CAP-V0-043)", f)
	}
	trees, err := readTrees("../../" + treesDir)
	if err != nil {
		t.Fatal(err)
	}
	nodes := map[string]int{}
	for _, tree := range trees {
		nodes[tree.Binary] = len(tree.Nodes)
	}
	for binary, least := range map[string]int{"corvint": 200, "corvint-tasks": 150, "corvint-test": 20} {
		if nodes[binary] < least {
			t.Errorf("%s tree has %d nodes, want at least %d", binary, nodes[binary], least)
		}
	}
}

// TestCAPV0048RatchetMatchesCeiling pins the committed ratchet to its ceiling, so removing a row
// must lower the ceiling and the allowlist can only shrink.
func TestCAPV0048RatchetMatchesCeiling(t *testing.T) {
	rows, err := readRatchet("../../" + ratchetPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != ratchetCeiling {
		t.Fatalf("ratchet has %d rows, ceiling is %d: lower ratchetCeiling to the row count; it may never rise", len(rows), ratchetCeiling)
	}
}

const syntheticDispatch = `package main

import (
	"flag"
	"os"
)

func main() {
	args := os.Args[1:]
	switch args[0] {
	case "alpha":
	case "help":
	}
	flag.String("out", "", "output file")
}
`

func syntheticTreeRepository(t *testing.T, dispatch string) map[string]string {
	t.Helper()
	builder := clitree.NewBuilder("corvint-x")
	builder.AddText("corvint-x alpha [--out FILE]\ncorvint-x help [--json]\n", "")
	builder.Declare([]string{"alpha"}, "first command")
	builder.Declare([]string{"help"}, "help")
	encoded, err := clitree.Encode(builder.Tree("corvint-x help --json", "synthetic", []string{"cmd/corvint-x"}, nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	return map[string]string{
		"cmd/corvint-x/main.go":            dispatch,
		treesDir + "/corvint-x.json":       string(encoded),
		ratchetPath:                        "binary\treason\n",
		"cmd/corvint-x/testdata/README.md": "not source",
	}
}

func treeFindings(t *testing.T, files map[string]string, binaries []string) map[string][]string {
	t.Helper()
	root := t.TempDir()
	writeFiles(t, root, files)
	return kinds(CheckTrees(root, binaries))
}

// TestCAPV0979EventValuesAreNotDrift is the drift-gate fixture for V1-3679: a dispatcher that
// compares an argument with a hook event name and indexes an event table by it raises no
// subcommand-unlisted finding, while a new subcommand beside it still does (CAP-V0-979).
func TestCAPV0979EventValuesAreNotDrift(t *testing.T) {
	binaries := []string{"corvint-x"}
	events := strings.Replace(syntheticDispatch, `func main() {`, `var claudeContextEvents = map[string]string{"session-start": "SessionStart", "post-tool": "PostToolUse"}

func main() {`, 1)
	events = strings.Replace(events, `	flag.String(`, `	if len(args) > 1 && args[1] == "session-start" {
	}
	_ = claudeContextEvents[args[1]]
	flag.String(`, 1)
	if found := treeFindings(t, syntheticTreeRepository(t, events), binaries); len(found) != 0 {
		t.Fatalf("event values raised drift: %v", found)
	}
	withVerb := strings.Replace(events, `case "help":`, `case "help":
	case "beta":`, 1)
	if found := treeFindings(t, syntheticTreeRepository(t, withVerb), binaries); !slices.Contains(found["subcommand-unlisted"], "corvint-x beta") {
		t.Fatalf("a new subcommand beside the event table raised no drift: %v", found)
	}
}

// TestCAPV0047SyntheticDriftFails proves the gate fails on a new subcommand, a new flag and a new
// binary that lack help or a tree, and on a growing or stale ratchet (CAP-V0-047, CAP-V0-048).
func TestCAPV0047SyntheticDriftFails(t *testing.T) {
	binaries := []string{"corvint-x"}
	if found := treeFindings(t, syntheticTreeRepository(t, syntheticDispatch), binaries); len(found) != 0 {
		t.Fatalf("baseline synthetic tree: %v", found)
	}
	cases := []struct {
		name, kind, want string
		edit             func(files map[string]string) []string
	}{
		{"new subcommand", "subcommand-unlisted", "corvint-x beta", func(files map[string]string) []string {
			files["cmd/corvint-x/main.go"] = strings.Replace(syntheticDispatch, `case "help":`, `case "help":
	case "beta":`, 1)
			return binaries
		}},
		{"new flag", "flag-unlisted", "corvint-x --dry-run", func(files map[string]string) []string {
			files["cmd/corvint-x/main.go"] = strings.Replace(syntheticDispatch, `flag.String("out", "", "output file")`,
				`flag.String("out", "", "output file")
	flag.Bool("dry-run", false, "plan only")`, 1)
			return binaries
		}},
		{"new binary", "tree-missing", "corvint-y", func(files map[string]string) []string {
			files["cmd/corvint-y/main.go"] = "package main\n\nfunc main() {}\n"
			return append(binaries, "corvint-y")
		}},
		{"ratchet grew", "ratchet-grew", ratchetPath, func(files map[string]string) []string {
			var rows strings.Builder
			rows.WriteString("binary\treason\n")
			grown := append([]string{}, binaries...)
			for i := 0; i <= ratchetCeiling; i++ {
				name := "corvint-r" + strings.Repeat("x", i+1)
				rows.WriteString(name + "\tsynthetic\n")
				grown = append(grown, name)
			}
			files[ratchetPath] = rows.String()
			return grown
		}},
		{"ratchet trades a migrated binary for a new one", "ratchet-grew", "corvint-new", func(files map[string]string) []string {
			files["cmd/corvint-new/main.go"] = "package main\n\nfunc main() {}\n"
			files[ratchetPath] = "binary\treason\ncorvint-new\tsynthetic\n"
			return append(binaries, "corvint-new")
		}},
		{"ratchet names a tree binary", "ratchet-stale", "corvint-x", func(files map[string]string) []string {
			files[ratchetPath] = "binary\treason\ncorvint-x\tsynthetic\n"
			return binaries
		}},
		{"ratchet names a removed binary", "ratchet-stale", "corvint-gone", func(files map[string]string) []string {
			files[ratchetPath] = "binary\treason\ncorvint-gone\tsynthetic\n"
			return binaries
		}},
		{"node without example", "tree-problem", "corvint-x", func(files map[string]string) []string {
			files[treesDir+"/corvint-x.json"] = strings.Replace(files[treesDir+"/corvint-x.json"], `"example": "corvint-x alpha"`, `"example": ""`, 1)
			return binaries
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := syntheticTreeRepository(t, syntheticDispatch)
			found := treeFindings(t, files, tc.edit(files))
			if !contains(found[tc.kind], tc.want) {
				t.Fatalf("want %s %q, got %v", tc.kind, tc.want, found)
			}
		})
	}
}

// TestCAPV0047NewMCPServerToolNeedsRow proves a binary that answers tools/list is scanned for tool
// names whatever its name, so its new tool fails the gate until the catalog has its row.
func TestCAPV0047NewMCPServerToolNeedsRow(t *testing.T) {
	for _, method := range []string{`"tools/list"`, "`tools/list`"} {
		root := t.TempDir()
		files := syntheticRepository()
		files["cmd/corvint-serve/main.go"] = "package main\n\nconst method = " + method + "\nconst tool = \"corvint.brand_new\"\n"
		writeFiles(t, root, files)
		var stdout, stderr bytes.Buffer
		if code := run(root, &stdout, &stderr); code != 1 {
			t.Fatalf("%s: exit %d, stderr %q", method, code, stderr.String())
		}
		if !strings.Contains(stderr.String(), `missing mcp-tools "corvint.brand_new"`) {
			t.Fatalf("%s: stderr %q", method, stderr.String())
		}
	}
}

// TestCAPV0049LLMsEntryPointLinksEveryView proves the root llms.txt names the catalog page, the
// command catalog, the guide and every command tree, and that the check fails when it is missing
// or a new tree is not linked (CAP-V0-049).
func TestCAPV0049LLMsEntryPointLinksEveryView(t *testing.T) {
	if found := CheckEntryPoint("../.."); len(found) != 0 {
		t.Fatalf("llms.txt: %v", found)
	}
	files := syntheticTreeRepository(t, syntheticDispatch)
	files[llmsPath] = "docs/CAPABILITIES.md corvint help --json corvint help guide\n"
	root := t.TempDir()
	writeFiles(t, root, files)
	found := kinds(CheckEntryPoint(root))
	if !contains(found["entry-unlinked"], "corvint-x help --json") || !contains(found["entry-unlinked"], treesDir+"/corvint-x.json") {
		t.Fatalf("unlinked tree not reported: %v", found)
	}
	if found := kinds(CheckEntryPoint(t.TempDir())); len(found["entry-missing"]) != 1 {
		t.Fatalf("missing llms.txt not reported: %v", found)
	}
}

// TestCAPV0050AgentsRuleNamesHelpCatalogAndGate pins the AGENTS.md rule that new functionality
// ships with its help and catalog entry and that the capabilities check enforces it (CAP-V0-050).
func TestCAPV0050AgentsRuleNamesHelpCatalogAndGate(t *testing.T) {
	data, err := os.ReadFile("../../AGENTS.md")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(strings.Fields(string(data)), " ")
	for _, want := range []string{"New functionality ships with its help and its catalog entry", "tools/capabilities-check` enforces it", "llms.txt"} {
		if !strings.Contains(text, want) {
			t.Fatalf("AGENTS.md lacks %q", want)
		}
	}
}
