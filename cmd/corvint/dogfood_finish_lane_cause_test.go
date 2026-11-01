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
// used to make ocm-prepare-001 refuse too-many-obligations at the bind, after
// which ocm-aggregate reported intent-scope-drift (DCW-V0-806 named the
// prepare refusal as the CAUSE). Since DCW-V0-961 the prepare pass refuses
// that intent before any step, naming it, so no bind runs, no sidecar is left
// and no drift is blamed; a rerun reaches the same refusal instead of a
// dirty-worktree refusal.
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
			"REFUSE too-many-obligations\n",
			"  intent docs/specs/big-v0.md in DOGFOOD_INTENTS_FILE derives more obligations at HEAD than one OCM map admits (256); no step ran",
		} {
			if code != 2 || !strings.Contains(stdout+stderr, want) || strings.Contains(stdout+stderr, "intent-scope-drift") || strings.Contains(stdout, "bind FAIL") {
				t.Fatalf("attempt %d exit=%d lacks %q\nstdout=%s\nstderr=%s", attempt, code, want, stdout, stderr)
			}
		}
		if _, err := os.Lstat(filepath.Join(root, ".corvint", "change.cem.json")); !os.IsNotExist(err) {
			t.Fatalf("attempt %d left a sidecar: %v", attempt, err)
		}
		if strings.Contains(stdout+stderr, "dirty-worktree") {
			t.Fatalf("attempt %d refused the failed pass's sidecar:\n%s%s", attempt, stdout, stderr)
		}
	}
}

// TestFinishLaneDCWV0806CauseNamesABindTimeRefusal (V1-3495): a bind-time
// step refusal that no pre-flight catches, followed through the installed
// binary. The fixture first used an intent with two `## Requirements`
// headings, refused by ocm-prepare-001 only in the bind; since DCW-V0-945 the
// prepare pass derives every intent as ocm-prepare does and refuses that one
// before any step, so the intent here is valid. Instead the DOGFOOD_OCM_LINKS
// plan, which only the bind pass reads, links obligation 2 of a one-requirement
// map: the row is well formed, so ocm-links is PRODUCED, and only `ocm link`
// against the prepared map refuses obligation-selector-out-of-range.
// finish-lane then names that originating row as the CAUSE before its FAIL
// line; without the CAUSE line this fails.
func TestFinishLaneDCWV0806CauseNamesABindTimeRefusal(t *testing.T) {
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
	cemWrite(t, root, "docs/specs/once-v0.md", "# Once v0\n\n## Requirements\n\n- `ONE-V0-001`: one holds.\n")
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
	links := write("links", "docs/specs/once-v0.md\t2\t1\ta_test.go\ttest:TestOne\n")
	args := []string{"dogfood", "finish-lane", "--base", base,
		"--citations", write("cites", "1\tunknown\tinsufficient-evidence\tfixture\n2\tunknown\tinsufficient-evidence\tfixture\n"),
		"--intents", write("intents", "docs/specs/once-v0.md\n"),
		"--verify", write("verify", "go test ./...\n"), "--outcome", "passed"}
	code, stdout, stderr := run.exec(t, root, append(finishLaneIdentity[:len(finishLaneIdentity):len(finishLaneIdentity)], "DOGFOOD_OCM_LINKS="+links), args...)
	if code == 0 || !strings.Contains(stdout, "dogfood-finish-lane: prepare PASS ") || strings.Contains(stdout+stderr, "REFUSE ") {
		t.Fatalf("exit=%d, want the prepare pass to admit the inputs and the bind to refuse\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	// The link row is the bind pass's first failing row; the CAUSE line names
	// it, ahead of the bind FAIL line.
	cause := strings.Index(stdout, "dogfood-finish-lane: CAUSE bind ocm-link-001 obligation-selector-out-of-range\n")
	fail := strings.Index(stdout, "dogfood-finish-lane: bind FAIL exit=")
	if cause < 0 || fail < cause || !strings.Contains(stdout, "  bind ocm-link-001 NOT_PRODUCED obligation-selector-out-of-range\n") {
		t.Fatalf("no CAUSE line naming the ocm-link refusal before the bind FAIL line\nstdout=%s\nstderr=%s", stdout, stderr)
	}
}
