// Package clitreetest checks a companion binary's command tree and help in its own tests
// (CAP-V0-972).
package clitreetest

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/clitree"
)

// CheckCompanion runs the binary's help entry point in process for every node with --help and -h:
// each exits 0 with stdout starting with that node's Render and nothing on stderr. The root help
// lists every node and names help --json, `help` with an unknown path exits 2, and the committed
// snapshot equals `help --json`.
func CheckCompanion(t *testing.T, run func(args []string, stdout, stderr io.Writer) int, snapshot string) {
	t.Helper()
	invoke := func(args ...string) (string, string, int) {
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr)
		return stdout.String(), stderr.String(), code
	}
	encoded, stderr, code := invoke("help", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("help --json: exit %d, %s", code, stderr)
	}
	want, err := os.ReadFile(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if encoded != string(want) {
		t.Fatalf("%s is stale; run `make capability-catalog`", snapshot)
	}
	var tree clitree.Tree
	if err := json.Unmarshal([]byte(encoded), &tree); err != nil {
		t.Fatal(err)
	}
	if problems := clitree.Problems(tree); len(problems) > 0 || len(tree.Nodes) == 0 {
		t.Fatalf("tree problems: %v (%d nodes)", problems, len(tree.Nodes))
	}
	for _, node := range tree.Nodes {
		for _, flag := range []string{"--help", "-h"} {
			stdout, stderr, code := invoke(append(slices.Clone(node.Path), flag)...)
			if code != 0 || stderr != "" || !strings.HasPrefix(stdout, clitree.Render(tree, node)) {
				t.Fatalf("%s %s: exit %d, stderr %q, stdout:\n%s", node.Command, flag, code, stderr, stdout)
			}
		}
	}
	for _, args := range [][]string{{"--help"}, {"-h"}, {"help"}} {
		stdout, stderr, code := invoke(args...)
		if code != 0 || stderr != "" || !strings.Contains(stdout, tree.HelpJSON) {
			t.Fatalf("%v: exit %d, stderr %q, stdout:\n%s", args, code, stderr, stdout)
		}
		for _, node := range tree.Nodes {
			if !strings.Contains(stdout, "  "+strings.Join(node.Path, " ")+": "+node.Summary+"\n") {
				t.Fatalf("%v root help lacks %v:\n%s", args, node.Path, stdout)
			}
		}
	}
	if _, stderr, code := invoke("help", "no-such-command"); code != 2 || !strings.Contains(stderr, "unknown command") {
		t.Fatalf("help no-such-command: exit %d, %q", code, stderr)
	}
}
