package main

import (
	"strings"
	"testing"
)

// DCW-V0-941 (V1-3036): a finish-lane run whose plan defers an intent scope
// commits the deferral record in its bind commit, settles the map that record
// adds a hunk to into that same commit, and completes rebind, check and seal
// in one invocation, leaving a clean worktree.
func TestDogfoodFinishLaneDCWV0941SettlesTheDeferralRecordHunk(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, base := portableDogfoodRepo(t)
	// BASE carries the deferred intent and the DCW-V0-372 bullet the rebind
	// cites the record hunk against; the change itself is unchanged.
	cemGit(t, root, "checkout", "-q", base)
	cemWrite(t, root, "later.md", "# Later\n\n## Requirements\n\n- `FIXTURE-LATER-001`: The later slice is assessed later.\n")
	cemWrite(t, root, "docs/specs/daily-change-evidence-workflow-v0.md", "# spec\n\n- `DCW-V0-371`: other\n- `DCW-V0-372`: cites the record\n  hunk automatically\n- `DCW-V0-373`: other\n")
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "base with a later intent")
	base = cemGit(t, root, "rev-parse", "HEAD")
	cemWrite(t, root, "fixture/fixture.go", "package fixture\nfunc Answer() int { return 2 }\n")
	cemGit(t, root, "commit", "-qam", "change")
	_, args := finishLaneInputs(t, "1\tintent.md\t1:5\tspecification\ndeferred\tlater.md\tnext slice\n", "intent.md\n")
	env := append([]string{"DOGFOOD_TASK=Answer two.", "DOGFOOD_VERIFY=go test ./fixture"}, finishLaneIdentity...)
	code, stdout, stderr := run.exec(t, root, env, append([]string{"dogfood", "finish-lane", "--base", base}, args...)...)
	if code != 0 || !strings.Contains(stdout, "dogfood-finish-lane: rebind PASS\n") || !strings.Contains(stdout, "dogfood-finish-lane: check PASS\n") ||
		!strings.Contains(stdout, "dogfood-finish-lane: seal PASS seal=") {
		t.Fatalf("finish-lane exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if status := cemGit(t, root, "status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Fatalf("status %q", status)
	}
	if subjects := finishLaneSubjects(t, root, base); subjects != "chore: seal change evidence\nchore: bind change evidence\nchange" {
		t.Fatalf("subjects %q", subjects)
	}
	bound := cemGit(t, root, "diff-tree", "-r", "--no-commit-id", "--name-only", "HEAD^^", "HEAD^")
	if !strings.Contains(bound, ".corvint/change.cem.json\n.corvint/deferred-intents/") {
		t.Fatalf("bind commit changed %q, want the sidecar and the deferral record", bound)
	}
}
