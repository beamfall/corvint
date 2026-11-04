// SPDX-License-Identifier: Apache-2.0

package dogfoodflow

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/trace"
)

// TestDCWV0844BindResetRerunNamesAndRetiresTheResetCase (V1-3318, V1-4318): a
// lane binds (its bind commit gains a worktree-local trace), resets that
// unpushed commit away and reruns. The check-only pre-flight notes the stranded
// trace, moves nothing and goes on (DWC-V0-003: a plan of stranded traces alone
// is one finish-lane applies itself); the real run retires the stranded trace
// and goes on to its steps, so the reset and rerun need no manual step.
func TestDCWV0844BindResetRerunNamesAndRetiresTheResetCase(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "a.txt")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("b\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "commit", "-q", "-am", "change")
	// The pre-flight now goes on to its steps, which write the report the
	// repository ignores.
	if err := os.WriteFile(filepath.Join(root, ".git", "info", "exclude"), []byte(".context-corvint/\n.corvint/dogfood-report.json\n.corvint/.dogfood-report.*.tmp\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	// Bind: an early bind/seal commit whose trace the worktree-local store keeps.
	if err := os.WriteFile(filepath.Join(root, "bound.txt"), []byte("cem\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "bound.txt")
	testGit(t, root, "commit", "-q", "-m", "bind")
	bound := testGit(t, root, "rev-parse", "HEAD")
	traces := filepath.Join(root, ".context-corvint", "traces")
	if err := os.MkdirAll(traces, 0o700); err != nil {
		t.Fatal(err)
	}
	record, err := trace.NewRecord(trace.Input{Producer: trace.ProducerCLI, Revision: bound, Task: "bind", Outcome: "passed"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	row, err := trace.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(traces, bound+".jsonl"), row, 0o600); err != nil {
		t.Fatal(err)
	}
	// Reset: the unpushed bind commit is dropped.
	testGit(t, root, "reset", "-q", "--hard", "HEAD~1")
	scratch := t.TempDir()
	plan := filepath.Join(scratch, "cites.tsv")
	verify := filepath.Join(scratch, "verify.txt")
	intents := filepath.Join(scratch, "intents.txt")
	for path, data := range map[string]string{plan: "1\ta.txt\t1:1\tspecification\n", verify: "go test -count=1 -run TestA ./internal/a\n", intents: "#no-intent-declared\n"} {
		if err := os.WriteFile(path, []byte(data), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	rerun := func(preflight bool) (int, int, string) {
		calls := 0
		steps := Runner{Path: "corvint", Run: func(context.Context, string, []string, io.Writer, io.Writer) int {
			calls++
			return 1
		}}
		var stdout, stderr bytes.Buffer
		code, err := FinishLane(context.Background(), FinishLaneOptions{Preflight: preflight, Change: ChangeOptions{Root: root, Base: base, Steps: steps, Citations: plan, IntentsFile: intents, VerifyFile: verify, Outcome: "passed"}}, &stdout, &stderr)
		if err != nil {
			t.Fatal(err)
		}
		return code, calls, stderr.String()
	}
	_, calls, out := rerun(true)
	if calls == 0 || strings.Contains(out, "REFUSE reused-worktree-state") ||
		!strings.Contains(out, "dogfood-inputs-check: NOTE stranded trace "+bound+": the worktree-local trace store names a commit HEAD does not reach") ||
		!strings.Contains(out, "dogfood finish-lane quarantines it byte-preserved with migrate-traces --apply --plan-digest ") {
		t.Fatalf("step calls=%d stderr=%q, want a note and the pre-flight steps run", calls, out)
	}
	if _, err := os.Lstat(filepath.Join(traces, bound+".jsonl")); err != nil {
		t.Fatalf("the check-only pre-flight moved the stranded trace: %v", err)
	}
	_, calls, out = rerun(false)
	if calls == 0 || strings.Contains(out, "REFUSE reused-worktree-state") ||
		!strings.Contains(out, "RETIRE stale trace "+bound+" quarantined byte-preserved") {
		t.Fatalf("step calls=%d stderr=%q, want the stranded trace retired and the steps run", calls, out)
	}
	if _, err := os.Lstat(filepath.Join(traces, bound+".jsonl")); !os.IsNotExist(err) {
		t.Fatalf("the stranded trace is still in the live store: %v", err)
	}
}
