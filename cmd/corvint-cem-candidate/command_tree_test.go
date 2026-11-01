package main

import (
	"bytes"
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/clitree"
)

// TestCAPV0932EveryNodeAnswersHelp runs the command for every command-tree node with --help
// and -h: each exits 0 with that node's own help on stdout and nothing on stderr, and the root
// help lists every node and names help --json.
func TestCAPV0932EveryNodeAnswersHelp(t *testing.T) {
	tree := commandTree()
	if problems := clitree.Problems(tree); len(problems) > 0 || len(tree.Nodes) == 0 {
		t.Fatalf("tree problems: %v (%d nodes)", problems, len(tree.Nodes))
	}
	for _, node := range tree.Nodes {
		for _, flag := range []string{"--help", "-h"} {
			stdout, stderr, code := run(t, append(slices.Clone(node.Path), flag)...)
			if code != 0 || stderr != "" || stdout != clitree.Render(tree, node) {
				t.Fatalf("%s %s: exit %d, stderr %q, stdout:\n%s", node.Command, flag, code, stderr, stdout)
			}
		}
	}
	stdout, _, code := run(t, "--help")
	if code != 0 || !strings.Contains(stdout, "corvint-cem-candidate help --json") {
		t.Fatalf("--help: exit %d, %q", code, stdout)
	}
	for _, node := range tree.Nodes {
		if !strings.Contains(stdout, "  "+strings.Join(node.Path, " ")+": "+node.Summary+"\n") {
			t.Fatalf("root help lacks %v:\n%s", node.Path, stdout)
		}
	}
	if _, stderr, code := run(t, "help", "nope"); code != 2 || !strings.Contains(stderr, "unknown command") {
		t.Fatalf("help nope: exit %d, %q", code, stderr)
	}
	if _, _, code := run(t); code != 2 {
		t.Fatalf("no arguments: exit %d, want the usage refusal", code)
	}
}

// TestCAPV0930TreeSnapshotIsCurrent keeps the committed snapshot the capabilities check reads equal
// to `corvint-cem-candidate help --json` (regenerate with `make capability-catalog`).
func TestCAPV0930TreeSnapshotIsCurrent(t *testing.T) {
	var got, errout bytes.Buffer
	if code := command(context.Background(), []string{"help", "--json"}, &got, &errout); code != 0 {
		t.Fatalf("help --json: exit %d, %s", code, errout.String())
	}
	want, err := os.ReadFile("../../internal/capcatalog/trees/corvint-cem-candidate.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), want) {
		t.Fatal("internal/capcatalog/trees/corvint-cem-candidate.json is stale; run `make capability-catalog`")
	}
}

func run(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := command(context.Background(), args, &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}
