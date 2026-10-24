package transaction

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// postMergeContext is a complete whose facts are f, at commit "c1", for a
// queue whose intent branch is main.
func postMergeContext(f LeaseFacts) leaseContext {
	c := leaseContext{l: &LeaseRequest{Commit: "c1"}}
	c.in.LeaseFacts = f
	c.st.queue = &intent.Queue{IntentBranch: "main"}
	return c
}

// wantPostMergeSequence fails unless detail names, in order, the submit of
// the merge commit's tree, the gate run and the completion at the merge.
func wantPostMergeSequence(t *testing.T, name, detail string) {
	t.Helper()
	at := 0
	for _, step := range []string{"merge commit", "submit --tree $(git rev-parse <merge>^{tree})", "gate run", "complete --commit $(git rev-parse <merge>)"} {
		i := strings.Index(detail[at:], step)
		if i < 0 {
			t.Fatalf("%s: detail %q lacks %q after offset %d", name, detail, step, at)
		}
		at += i + len(step)
	}
}

// TestTCBV0047_TreeMismatchNamesThePostMergeSequence (V1-1895): a reachable
// commit whose tree is not the candidate refuses STALE_TREE and names the
// sequence to follow after a merge queue's merge commit.
func TestTCBV0047_TreeMismatchNamesThePostMergeSequence(t *testing.T) {
	tree := "t-submitted"
	c := postMergeContext(CompleteFacts("t-merge", true, "refs/remotes/origin/main", "", nil))
	code, detail := c.completionBlocker(&snapshot.Attempt{CandidateTreeOid: &tree}, &ticket.Record{})
	if code != wire.CodeStaleTree || !strings.Contains(detail, "commit tree t-merge is not the candidate t-submitted") {
		t.Fatalf("tree mismatch: %s %q", code, detail)
	}
	wantPostMergeSequence(t, "tree mismatch", detail)
}

// TestTCBV0047_NotIntegratedNamesThePostMergeSequence (V1-1895): an intent
// commit neither the branch nor its upstream contains refuses STALE_TREE,
// keeps its fetch or integrate route, and names the post-merge sequence with
// or without an upstream; the extra-repository refusal is unchanged.
func TestTCBV0047_NotIntegratedNamesThePostMergeSequence(t *testing.T) {
	tree := "t-submitted"
	a := &snapshot.Attempt{CandidateTreeOid: &tree}
	for _, tc := range []struct {
		name, upstream, route string
	}{
		{"upstream behind", "refs/remotes/origin/main", "fetch the upstream (git fetch)"},
		{"no upstream", "", "integrate it into refs/heads/main"},
	} {
		c := postMergeContext(CompleteFacts(tree, false, tc.upstream, "", nil))
		code, detail := c.completionBlocker(a, &ticket.Record{})
		if code != wire.CodeStaleTree || !c.in.LeaseFacts.commitNotIntegrated() {
			t.Fatalf("%s: %s %q, want STALE_TREE with COMMIT_NOT_INTEGRATED", tc.name, code, detail)
		}
		route := strings.Index(detail, tc.route)
		if route < 0 || route > strings.Index(detail, "merge commit") {
			t.Fatalf("%s: detail %q must keep %q before the post-merge sequence", tc.name, detail, tc.route)
		}
		wantPostMergeSequence(t, tc.name, detail)
	}
	c := postMergeContext(CompleteFacts(tree, false, "", "docs", nil))
	if code, detail := c.completionBlocker(a, &ticket.Record{}); code != wire.CodeStaleTree || detail != "repository docs candidate is not integrated in its designated integration branch" {
		t.Fatalf("extra repository: %s %q", code, detail)
	}
}
