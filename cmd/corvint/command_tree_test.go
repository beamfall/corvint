package main

import (
	"bytes"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/clitree"
)

// TestCAPV0042EveryTreeNodeAnswersHelp asks every corvint command-tree node for --help and -h:
// each exits 0, reads no stdin, and prints the help of exactly that node, with its usage, required
// inputs and an example (CAP-V0-042, CAP-V0-044).
func TestCAPV0042EveryTreeNodeAnswersHelp(t *testing.T) {
	t.Parallel()
	tree := commandTree()
	if problems := clitree.Problems(tree); len(problems) > 0 {
		t.Fatalf("tree problems: %v", problems)
	}
	if len(tree.Nodes) < 200 {
		t.Fatalf("tree has %d nodes", len(tree.Nodes))
	}
	for _, node := range tree.Nodes {
		for _, flag := range []string{"--help", "-h"} {
			args := append(slices.Clone(node.Path), flag)
			topic, ok, err := parseHelpInvocation(args)
			if err != nil || !ok {
				t.Fatalf("%q: no help topic (%v)", args, err)
			}
			own := topic == nodeHelpPrefix+strings.Join(node.Path, " ") || slices.Equal(helpTopicPath(topic), node.Path) ||
				onlyChildTopic(tree, node.Path, helpTopicPath(topic))
			if !own {
				t.Fatalf("%q resolves to topic %q, not its own node", args, topic)
			}
			reader := &forbiddenHelpReader{}
			var stdout, stderr bytes.Buffer
			if exit := run(args, reader, &stdout, &stderr); exit != 0 || stderr.Len() != 0 || reader.reads != 0 {
				t.Fatalf("%q: exit %d stderr %q reads %d", args, exit, &stderr, reader.reads)
			}
			text := stdout.String()
			if text != helpText(topic) || !strings.Contains(text, " "+strings.Join(node.Path, " ")) ||
				!strings.Contains(text, "Example") || node.Path[0] != "help" && !strings.Contains(text, "Required:") {
				t.Fatalf("%q: help lacks usage, required inputs or example:\n%s", args, text)
			}
		}
	}
}

// TestCAPV0045CatalogLinksCurrentTreeSnapshots pins `corvint help --tree` to the committed
// snapshot and requires `corvint help --json` to link every snapshot (CAP-V0-045, CAP-V0-046).
func TestCAPV0045CatalogLinksCurrentTreeSnapshots(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if exit := run([]string{"help", "--tree"}, &forbiddenHelpReader{}, &stdout, &stderr); exit != 0 || stdout.String() != commandTreeJSON() {
		t.Fatalf("help --tree: exit %d stderr %q", exit, &stderr)
	}
	snapshot, err := os.ReadFile("../../internal/capcatalog/trees/corvint.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stdout.Bytes(), snapshot) {
		t.Fatal("internal/capcatalog/trees/corvint.json is stale; run `make capability-catalog`")
	}
	var catalog struct {
		CommandTrees []struct{ Binary, Command, Snapshot string } `json:"commandTrees"`
	}
	if err := json.Unmarshal([]byte(capabilityCatalogJSON()), &catalog); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir("../../internal/capcatalog/trees")
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.CommandTrees) != len(entries) {
		t.Fatalf("catalog links %d command trees, %d snapshots exist", len(catalog.CommandTrees), len(entries))
	}
	for _, ref := range catalog.CommandTrees {
		data, err := os.ReadFile("../../" + ref.Snapshot)
		if err != nil {
			t.Fatal(err)
		}
		var tree clitree.Tree
		if err := json.Unmarshal(data, &tree); err != nil || tree.Binary != ref.Binary || tree.HelpJSON != ref.Command {
			t.Fatalf("%s: snapshot binary %q command %q (%v)", ref.Snapshot, tree.Binary, tree.HelpJSON, err)
		}
	}
}

// onlyChildTopic reports whether a group's help is the topic of its one subcommand, as `corvint
// harness --help` prints the `harness event` topic.
func onlyChildTopic(tree clitree.Tree, group, documented []string) bool {
	if len(documented) <= len(group) || !slices.Equal(documented[:len(group)], group) {
		return false
	}
	for _, other := range tree.Nodes {
		if len(other.Path) > len(group) && slices.Equal(other.Path[:len(group)], group) &&
			(len(other.Path) < len(documented) || !slices.Equal(other.Path[:len(documented)], documented)) {
			return false
		}
	}
	return true
}
