package cli_test

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/clitree"
	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCAPV0042EveryTreeNodeAnswersHelp asks every corvint-tasks command-tree node for --help and
// -h: each exits 0 with its own usage, flags, required inputs and example (CAP-V0-042, CAP-V0-044).
func TestCAPV0042EveryTreeNodeAnswersHelp(t *testing.T) {
	tree := cli.CommandTree()
	if problems := clitree.Problems(tree); len(problems) > 0 {
		t.Fatalf("tree problems: %v", problems)
	}
	if len(tree.Nodes) < 150 {
		t.Fatalf("tree has %d nodes", len(tree.Nodes))
	}
	for _, node := range tree.Nodes {
		if slices.Equal(node.Path, []string{"help"}) {
			continue // `help --help` is the root help, pinned by CAL-V0-047.
		}
		for _, flag := range []string{"--help", "-h"} {
			args := append(slices.Clone(node.Path), flag)
			var out, errb bytes.Buffer
			code := cli.Run(cli.Env{Cwd: t.TempDir(), Args: args, Stdin: unreadHelpInput{}, Stdout: &out, Stderr: &errb})
			r, err := wire.DecodeResult(out.Bytes())
			if err != nil || code != 0 || r.Outcome != wire.OutcomeOK || errb.Len() != 0 {
				t.Fatalf("%v: exit %d %s %s (%v)", args, code, out.Bytes(), errb.Bytes(), err)
			}
			item := r.Items[0]
			if field(item, "usage").Str == "" || field(item, "example").Str != node.Example {
				t.Fatalf("%v: usage %q, example %q want %q", args, field(item, "usage").Str, field(item, "example").Str, node.Example)
			}
			var required []string
			for _, value := range field(item, "required").Arr {
				required = append(required, value.Str)
			}
			if _, ok := item.Obj.Get("required"); !ok || strings.Join(required, " ") != strings.Join(node.Required, " ") {
				t.Fatalf("%v: required %v want %v", args, required, node.Required)
			}
		}
	}
}

// TestCAPV0046HelpJSONIsTheTree pins `corvint-tasks help --json` to the tree document and keeps the
// committed snapshot the capabilities check reads current (CAP-V0-045, CAP-V0-046).
func TestCAPV0046HelpJSONIsTheTree(t *testing.T) {
	var out bytes.Buffer
	if code := cli.Run(cli.Env{Cwd: t.TempDir(), Args: []string{"help", "--json"}, Stdin: unreadHelpInput{}, Stdout: &out}); code != 0 {
		t.Fatalf("help --json: exit %d", code)
	}
	if !bytes.Equal(out.Bytes(), cli.CommandTreeJSON()) {
		t.Fatal("help --json is not the command tree document")
	}
	// CAP-V0-950: `help --tree`, corvint's spelling, prints the same tree.
	var tree bytes.Buffer
	if code := cli.Run(cli.Env{Cwd: t.TempDir(), Args: []string{"help", "--tree"}, Stdin: unreadHelpInput{}, Stdout: &tree}); code != 0 || !bytes.Equal(tree.Bytes(), out.Bytes()) {
		t.Fatalf("help --tree: exit %d, equal to help --json %v", code, bytes.Equal(tree.Bytes(), out.Bytes()))
	}
	want, err := os.ReadFile("../../capcatalog/trees/corvint-tasks.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), want) {
		t.Fatal("internal/capcatalog/trees/corvint-tasks.json is stale; run `make capability-catalog`")
	}
}
