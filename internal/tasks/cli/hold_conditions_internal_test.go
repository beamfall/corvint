package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
)

// sccBoundsRepo is a one-commit repository on refs/heads/main holding a/b.
func sccBoundsRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a", "b"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "a/b"}, {"commit", "-q", "-m", "seed"}} {
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

// TestSCCV0013_EvaluationBound: one read evaluates at most
// holdConditionEvaluationBound typed conditions; the rest are UNKNOWN
// "not evaluated", never UNBLOCKED or BLOCKED.
func TestSCCV0013_EvaluationBound(t *testing.T) {
	dir := sccBoundsRepo(t)
	var holds []heldHold
	for i := 0; i <= holdConditionEvaluationBound; i++ {
		holds = append(holds, heldHold{ticketID: fmt.Sprintf("T-%03d", i), hold: ticket.Hold{HoldID: "h",
			Condition: &ticket.HoldCondition{Kind: ticket.HoldConditionPathExists, Ref: "refs/heads/main", Path: "a/b"}}})
	}
	holds = append(holds, heldHold{ticketID: "T-free", hold: ticket.Hold{HoldID: "h", Reason: "free text"}})
	got := evaluateHeldConditions(dir, holds)
	if len(got) != holdConditionEvaluationBound+1 {
		t.Fatalf("%d results; want one per typed condition", len(got))
	}
	for i, r := range got[:holdConditionEvaluationBound] {
		if r.res.State != store.HoldConditionUnblocked {
			t.Fatalf("result %d: %+v; want UNBLOCKED", i, r.res)
		}
	}
	last := got[holdConditionEvaluationBound].res
	if last.State != store.HoldConditionUnknown || !strings.HasPrefix(last.Detail, "not evaluated") {
		t.Fatalf("over the bound: %+v; want UNKNOWN not evaluated", last)
	}
}

// TestSCCV0014_FreeTextCandidates: commit ids need a digit and 7..64 lowercase
// hex characters; paths are slash-separated and relative; URLs, refnames and
// parent-relative paths are skipped; a hold yields at most
// holdFreeTextPerHold candidates.
func TestSCCV0014_FreeTextCandidates(t *testing.T) {
	shas, paths := holdFreeTextCandidates("wait for (a1b2c3d), see https://x.example/a/b, refs/heads/main, ../up/x, deadbeef, `internal/tasks/cli/` and docs/a.md.")
	if !slices.Equal(shas, []string{"a1b2c3d"}) || !slices.Equal(paths, []string{"internal/tasks/cli/", "docs/a.md"}) {
		t.Fatalf("candidates %q %q", shas, paths)
	}
	var words []string
	for i := 0; i < 2*holdFreeTextPerHold; i++ {
		words = append(words, fmt.Sprintf("d/p%d", i))
	}
	if _, paths := holdFreeTextCandidates(strings.Join(words, " ")); len(paths) != holdFreeTextPerHold {
		t.Fatalf("%d candidates; want the per-hold bound %d", len(paths), holdFreeTextPerHold)
	}
}

// TestSCCV0014_FreeTextCandidateBound: past holdFreeTextCandidateBound
// candidates the doctor stops evaluating and warns how many it left.
func TestSCCV0014_FreeTextCandidateBound(t *testing.T) {
	dir := sccBoundsRepo(t)
	var holds []heldHold
	extra := 3
	n := holdFreeTextCandidateBound/holdFreeTextPerHold + 1
	for i := 0; i < n; i++ {
		var words []string
		for j := 0; j < holdFreeTextPerHold; j++ {
			words = append(words, fmt.Sprintf("missing/p%d-%d", i, j))
		}
		if i == n-1 {
			words = words[:extra]
		}
		holds = append(holds, heldHold{ticketID: fmt.Sprintf("T-%03d", i), hold: ticket.Hold{HoldID: "h", Reason: strings.Join(words, " ")}})
	}
	scan := &doctorScan{}
	if got := doctorFreeTextHoldFindings(dir, "refs/heads/main", holds, scan); len(got) != 0 {
		t.Fatalf("findings for missing paths: %+v", got)
	}
	want := fmt.Sprintf("free-text hold scan evaluated %d candidates and left %d unevaluated", holdFreeTextCandidateBound, extra)
	if !slices.Contains(scan.warnings, want) {
		t.Fatalf("warnings %q; want %q", scan.warnings, want)
	}
}
