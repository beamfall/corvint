package store_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// notIntegrated asserts a BLOCKED refusal carrying exactly STALE_TREE and
// COMMIT_NOT_INTEGRATED, whose detail names every want fragment. The plan's
// code order is not a contract: the command-result envelope, rendered here
// as the CLI's mutateResult does, serializes them in canonical sorted order.
func notIntegrated(t *testing.T, report *store.Report, want ...string) {
	t.Helper()
	codes := report.Outcome.Codes
	if report.Outcome.Outcome != mutation.OutcomeBlocked || len(codes) != 2 || !has(codes, wire.CodeStaleTree) || !has(codes, wire.CodeCommitNotIntegrated) {
		t.Fatalf("want BLOCKED {STALE_TREE, COMMIT_NOT_INTEGRATED}, got %+v", report)
	}
	envelope := (&wire.Result{Command: []string{"complete"}, Outcome: wire.OutcomeRefused, Codes: codes}).Value()
	rendered, _ := envelope.Obj.Get("codes")
	if len(rendered.Arr) != 2 || rendered.Arr[0].Str != wire.CodeCommitNotIntegrated || rendered.Arr[1].Str != wire.CodeStaleTree {
		t.Fatalf("serialized codes %+v, want [COMMIT_NOT_INTEGRATED STALE_TREE]", rendered.Arr)
	}
	for _, w := range want {
		if !strings.Contains(report.Detail, w) {
			t.Fatalf("detail %q lacks %q", report.Detail, w)
		}
	}
}

// TestCALV0017_CompleteAcceptsAnUpstreamIntegratedCommit (V1-1081): a commit
// the intent branch lacks completes only when the branch's configured
// upstream is an existing remote-tracking ref that contains it. No upstream,
// an upstream that lacks it, an absent remote-tracking ref and a local
// upstream each refuse STALE_TREE plus COMMIT_NOT_INTEGRATED, write nothing,
// and leave every ref where it was; a tree mismatch keeps STALE_TREE alone.
func TestCALV0017_CompleteAcceptsAnUpstreamIntegratedCommit(t *testing.T) {
	t.Parallel()
	s := newGateStore(t)
	id := s.ticket(t, "one")
	claim, commit := s.submitted(t, id, "src", 0)
	s.passes(t, "gate-1", gateOf(claim, "verify"), 2)
	tree := gitOut(t, s.root, "rev-parse", commit+"^{tree}")
	// merged is the candidate tree merged on the remote: main lacks it.
	merged := gitOut(t, s.root, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "user.useConfigOnly=true", "commit-tree", tree, "-p", commit, "-m", "remote merge")
	complete := func(requestID, oid string) *store.Report {
		t.Helper()
		return s.lease(t, requestID, completeOf(claim, oid), 3, nil)
	}
	before := storeDigest(t, s.repo)

	notIntegrated(t, complete("complete-no-upstream", merged), "refs/heads/main", "no remote-tracking upstream configured")

	gitRun(t, s.root, "config", "remote.origin.url", "/nonexistent")
	gitRun(t, s.root, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	gitRun(t, s.root, "config", "branch.main.remote", "origin")
	gitRun(t, s.root, "config", "branch.main.merge", "refs/heads/main")
	notIntegrated(t, complete("complete-upstream-absent", merged), "no remote-tracking upstream configured")

	gitRun(t, s.root, "update-ref", "refs/remotes/origin/main", commit+"~1")
	notIntegrated(t, complete("complete-upstream-behind", merged), "refs/heads/main", "refs/remotes/origin/main", "git fetch")

	gitRun(t, s.root, "update-ref", "refs/heads/side", merged)
	gitRun(t, s.root, "config", "branch.main.remote", ".")
	gitRun(t, s.root, "config", "branch.main.merge", "refs/heads/side")
	notIntegrated(t, complete("complete-local-upstream", merged), "no remote-tracking upstream configured")

	base := complete("complete-base", gitOut(t, s.root, "rev-parse", commit+"~1"))
	if base.Outcome.Outcome != mutation.OutcomeBlocked || len(base.Outcome.Codes) != 1 || base.Outcome.Codes[0] != wire.CodeStaleTree {
		t.Fatalf("tree mismatch: %+v", base)
	}
	if storeDigest(t, s.repo) != before {
		t.Fatal("refused completion wrote")
	}

	gitRun(t, s.root, "config", "branch.main.remote", "origin")
	gitRun(t, s.root, "config", "branch.main.merge", "refs/heads/main")
	gitRun(t, s.root, "update-ref", "refs/remotes/origin/main", merged)
	mainTip := gitOut(t, s.root, "rev-parse", "refs/heads/main")
	if report := complete("complete-upstream", merged); report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("complete: %+v", report)
	}
	if got := gitOut(t, s.root, "rev-parse", "refs/heads/main"); got != mainTip || mainTip != commit {
		t.Fatalf("complete moved main: %s, was %s", got, mainTip)
	}
	if got := gitOut(t, s.root, "rev-parse", "refs/remotes/origin/main"); got != merged {
		t.Fatalf("complete moved the upstream: %s", got)
	}
	auditOK(t, s.repo)
}
