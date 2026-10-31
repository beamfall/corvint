package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFinishLaneDCWV0806NamesTheOriginatingRefusal reproduces V1-3381 with the
// installed binary: an intent spec over one OCM map's 256-obligation bound
// makes ocm-prepare-001 refuse too-many-obligations, after which ocm-aggregate
// reports intent-scope-drift. The bind failure must name the prepare refusal
// as its CAUSE, give that row a fix, point the aggregate row back at it and
// name the sidecar recovery; a rerun of the same command must reach the same
// CAUSE instead of a dirty-worktree refusal. Fails on BASE, which printed no
// CAUSE line and blamed a drift.
func TestFinishLaneDCWV0806NamesTheOriginatingRefusal(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cemWrite(t, root, "a.txt", "base\n")
	cemWrite(t, root, ".gitignore", ".corvint/change.ocm.json\n.corvint/change.ocm-intents\n.corvint/change.ocm-status.json\n.corvint/change.ocm.[0-9][0-9][0-9].json\n.corvint/self-observations.jsonl\n.corvint/dogfood-report.json\n")
	cemGit(t, root, "init", "-q", "-b", "main")
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "base")
	base := cemGit(t, root, "rev-parse", "HEAD")
	var spec strings.Builder
	spec.WriteString("# Big v0\n\n## Requirements\n\n")
	for requirement := 1; requirement <= 257; requirement++ {
		fmt.Fprintf(&spec, "- `BIG-V0-%03d`: requirement %d holds.\n", requirement, requirement)
	}
	cemWrite(t, root, "docs/specs/big-v0.md", spec.String())
	cemWrite(t, root, "a.txt", "changed\n")
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "change")
	inputs := t.TempDir()
	write := func(name, text string) string {
		path := filepath.Join(inputs, name)
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	args := []string{"dogfood", "finish-lane", "--base", base,
		"--citations", write("cites", "1\tunknown\tinsufficient-evidence\tfixture\n2\tunknown\tinsufficient-evidence\tfixture\n"),
		"--intents", write("intents", "docs/specs/big-v0.md\n"),
		"--verify", write("verify", "go test ./...\n"), "--outcome", "passed"}
	for attempt := 1; attempt <= 2; attempt++ {
		code, stdout, stderr := run.exec(t, root, finishLaneIdentity, args...)
		for _, want := range []string{
			"dogfood-finish-lane: CAUSE bind ocm-prepare-001 too-many-obligations\n  recover: this pass left the uncommitted .corvint/change.cem.json",
			"dogfood-finish-lane: bind FAIL exit=1\n",
			"  ocm-prepare-001: too-many-obligations\n    fix: the intent on line N of DOGFOOD_INTENTS_FILE, ocm-prepare-001 being ocm-prepare-N, derives more obligations than one OCM map admits (256 requirements, OCM-V0-023)",
			"  ocm-aggregate: intent-scope-drift\n    fix: ocm-prepare-001 refused too-many-obligations first, so the aggregate did not run and no intent drifted",
		} {
			if code != 1 || !strings.Contains(stdout+stderr, want) {
				t.Fatalf("attempt %d exit=%d lacks %q\nstdout=%s\nstderr=%s", attempt, code, want, stdout, stderr)
			}
		}
		if strings.Contains(stdout+stderr, "dirty-worktree") {
			t.Fatalf("attempt %d refused the failed pass's sidecar:\n%s%s", attempt, stdout, stderr)
		}
	}
}
