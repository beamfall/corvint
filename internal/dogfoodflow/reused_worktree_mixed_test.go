// SPDX-License-Identifier: Apache-2.0

package dogfoodflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/trace"
)

// TestFinishLaneDCWV0406LeavesAMixedTracePlanToTheOperator: a store holding a
// stranded commit trace, a reachable tree-keyed legacy trace and a canonical
// trace has one migration plan that would also rekey the legacy trace, so the
// run applies none of it, moves no trace, and refuses naming why.
func TestFinishLaneDCWV0406LeavesAMixedTracePlanToTheOperator(t *testing.T) {
	root, _, run := staleSidecarLane(t)
	exclude := filepath.Join(root, ".git", "info", "exclude")
	if err := os.WriteFile(exclude, []byte(".corvint/change.ocm*\n.context-corvint/\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	head := testGit(t, root, "rev-parse", "HEAD")
	tree := testGit(t, root, "rev-parse", "HEAD^{tree}")
	// A commit object HEAD does not reach, as an amend or rebase leaves.
	stranded := testGit(t, root, "commit-tree", "-m", "amended away", tree)
	traces := filepath.Join(root, ".context-corvint", "traces")
	if err := os.MkdirAll(traces, 0o700); err != nil {
		t.Fatal(err)
	}
	want := map[string][]byte{}
	for _, revision := range []string{stranded, tree, head} {
		record, err := trace.NewRecord(trace.Input{Producer: trace.ProducerCLI, Revision: revision, Task: "mixed plan", Outcome: "passed"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		row, err := trace.Encode(record)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(traces, revision+".jsonl"), row, 0o600); err != nil {
			t.Fatal(err)
		}
		want[revision] = row
	}
	code, calls, out := run()
	if code != 2 || calls != 0 || !strings.Contains(out, "REFUSE reused-worktree-state\n") ||
		!strings.Contains(out, "  stale trace: the worktree-local trace store names "+stranded+", ") ||
		!strings.Contains(out, "  the same migration plan also rekeys 1 reachable tree-keyed legacy trace(s), so it was not applied automatically (DCW-V0-406)\n") ||
		strings.Contains(out, "RETIRE stale trace") {
		t.Fatalf("exit=%d step calls=%d stderr=%q, want a refusal that applies no trace migration", code, calls, out)
	}
	for revision, row := range want {
		if data, err := os.ReadFile(filepath.Join(traces, revision+".jsonl")); err != nil || string(data) != string(row) {
			t.Fatalf("trace %s changed: %q, %v", revision, data, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(root, ".context-corvint", "legacy-traces")); err == nil {
		entries, _ := os.ReadDir(filepath.Join(root, ".context-corvint", "legacy-traces"))
		if len(entries) != 0 {
			t.Fatalf("a trace was quarantined: %v", entries)
		}
	}
}
