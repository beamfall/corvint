package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// DCW-V0-964 (V1-3303; the note is DCW-V0-810): a sealed finish leaves its ignored OCM sidecars, and a
// stacked follow-up in the same worktree, bound against that seal, needs no
// manual cleanup: inputs-check notes the sidecars and passes, and finish-lane
// moves them aside byte-preserved and seals.
func TestDogfoodFinishLaneDCWV0964StackedFinishNeedsNoCleanup(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, base := portableDogfoodRepo(t)
	_, args := finishLaneInputs(t, "1\tintent.md\t1:5\tspecification\n", "intent.md\n")
	env := append([]string{"DOGFOOD_TASK=Answer two.", "DOGFOOD_VERIFY=go test ./fixture"}, finishLaneIdentity...)
	if code, stdout, stderr := run.exec(t, root, env, append([]string{"dogfood", "finish-lane", "--base", base}, args...)...); code != 0 || !strings.Contains(stdout, "dogfood-finish-lane: seal PASS seal=") {
		t.Fatalf("first finish exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	seal := cemGit(t, root, "rev-parse", "HEAD")
	sidecars, err := filepath.Glob(filepath.Join(root, ".corvint", "change.ocm*"))
	if err != nil || len(sidecars) == 0 {
		t.Fatalf("the first finish left no OCM sidecar to reuse: %v", err)
	}
	// The fixture's commits are dated 2000-01-01; the sidecars were written
	// before the seal commit, as on a real lane.
	before := time.Date(1999, 12, 31, 0, 0, 0, 0, time.UTC)
	for _, sidecar := range sidecars {
		if err := os.Chtimes(sidecar, before, before); err != nil {
			t.Fatal(err)
		}
	}
	cemWrite(t, root, "fixture/fixture.go", "package fixture\n\n// Answer is two.\nfunc Answer() int { return 2 }\n")
	cemGit(t, root, "commit", "-qam", "stacked follow-up")
	code, stdout, stderr := run.exec(t, root, env, append([]string{"dogfood", "inputs-check", "--base", seal}, args...)...)
	if code != 0 || !strings.Contains(stderr, "dogfood-inputs-check: NOTE stale sidecar .corvint/change.ocm") || strings.Contains(stderr, "REFUSE") {
		t.Fatalf("inputs-check exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	code, stdout, stderr = run.exec(t, root, env, append([]string{"dogfood", "finish-lane", "--base", seal}, args...)...)
	if code != 0 || !strings.Contains(stderr, "dogfood-finish-lane: RETIRE stale sidecar .corvint/change.ocm") || !strings.Contains(stdout, "dogfood-finish-lane: seal PASS seal=") {
		t.Fatalf("stacked finish exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if status := cemGit(t, root, "status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Fatalf("status %q", status)
	}
}
