package store_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCBV0021_CompletePinsTheSealCommitOfAMergedForwardLane: the lane
// submits and gates its seal commit's tree while main moves on; after a
// post-seal merge of main into the lane and a merge-commit integration, the
// merge and the post-seal commit refuse STALE_TREE alone, the seal commit
// refuses STALE_TREE plus COMMIT_NOT_INTEGRATED until main reaches it, and then
// completes through the merge's second parent with no Tasks rule beyond
// CAL-V0-017 (TCB-V0-021).
func TestTCBV0021_CompletePinsTheSealCommitOfAMergedForwardLane(t *testing.T) {
	t.Parallel()
	s := newGateStore(t)
	id := s.ticket(t, "one")
	claim := s.claim(t, "claim-"+id, id, 0, "src/")
	gitRun(t, s.root, "checkout", "-q", "-b", "lane")
	seal, sealTree := s.commit(t, "src/a.go")
	if r := s.lease(t, "submit-seal", submitOf(claim, sealTree), 1, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("submit: %+v", r)
	}
	s.passes(t, "gate-seal", gateOf(claim, "verify"), 2)
	gitRun(t, s.root, "checkout", "-q", "main")
	s.commit(t, "other/z.go")
	identity := []string{"-c", "user.name=t", "-c", "user.email=t@example.com"}
	gitRun(t, s.root, "checkout", "-q", "lane")
	gitRun(t, s.root, append(identity, "merge", "-q", "--no-edit", "main")...)
	forward := gitOut(t, s.root, "rev-parse", "HEAD")
	gitRun(t, s.root, "checkout", "-q", "main")
	before := storeDigest(t, s.repo)
	notIntegrated(t, s.lease(t, "complete-early", completeOf(claim, seal), 3, nil), "refs/heads/main")
	gitRun(t, s.root, append(identity, "merge", "-q", "--no-ff", "--no-edit", "lane")...)
	merge := gitOut(t, s.root, "rev-parse", "HEAD")
	if gitOut(t, s.root, "rev-parse", merge+"^2") != forward || gitOut(t, s.root, "rev-parse", forward+"^1") != seal {
		t.Fatal("fixture: main must reach the seal only through the merge's second parent")
	}
	for _, oid := range []string{merge, forward} {
		report := s.lease(t, "complete-"+oid[:12], completeOf(claim, oid), 3, nil)
		if report.Outcome.Outcome != mutation.OutcomeBlocked || len(report.Outcome.Codes) != 1 || report.Outcome.Codes[0] != wire.CodeStaleTree {
			t.Fatalf("a merged-forward commit's tree is not the candidate: %+v", report)
		}
	}
	if storeDigest(t, s.repo) != before {
		t.Fatal("refused completion wrote")
	}
	if report := s.lease(t, "complete-seal", completeOf(claim, seal), 4, nil); report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("complete at the seal commit: %+v", report)
	}
	if a := s.attempt(t, claim.AttemptID); a.Phase != "COMPLETED" || *a.CandidateTreeOid != sealTree {
		t.Fatalf("attempt: %+v", a)
	}
	auditOK(t, s.repo)
}
