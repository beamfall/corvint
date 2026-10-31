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

// TestDCWV0844BindResetRerunNamesAndRetiresTheResetCase (V1-3318): a lane binds
// (its bind commit gains a worktree-local trace), resets that unpushed commit
// away and reruns. The check-only pre-flight refuses naming the reset-after-bind
// case and its one-step migrate-traces recovery; the real run retires the
// stranded trace itself and goes on to its steps.
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
	if err := os.WriteFile(filepath.Join(root, ".git", "info", "exclude"), []byte(".context-corvint/\n"), 0o666); err != nil {
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
	code, calls, out := rerun(true)
	if code != 2 || calls != 0 || !strings.HasPrefix(out, "dogfood-inputs-check: REFUSE reused-worktree-state\n") ||
		!strings.Contains(out, "or this lane's own bind/seal commits reset away before push") ||
		!strings.Contains(out, "  stale trace: the worktree-local trace store names "+bound+", which HEAD does not reach") ||
		!strings.Contains(out, "  fix: corvint migrate-traces --apply --plan-digest ") ||
		!strings.Contains(out, "one-step recovery after a reset of this lane's own unpushed bind/seal commits (DCW-V0-844)") {
		t.Fatalf("exit=%d step calls=%d stderr=%q, want the reset case and its recovery named", code, calls, out)
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
