package doccorpus

import (
	"context"
	"strings"
	"testing"
)

// TestDCRV1975PinnedRevisionsOneBatchPerCheckout: a checkout's pinned
// revisions resolve through one cat-file batch, so six revisions verify under
// the Git operation bound one revision needs, which six one-revision batches
// exceed; a missing revision still refuses, and a primary revision sorted
// before it still refuses as a recorded tree difference first.
func TestDCRV1975PinnedRevisionsOneBatchPerCheckout(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root, a, _ := builtFixture(t)
	verify := func(ops int, artifact *Artifact) error {
		return verifyPinnedRevisions(boundCorpusGitOps(ctx, ops), root, artifact)
	}
	// bound is the fewest operations one pinned revision needs: opening the
	// checkout plus one batch.
	bound := 0
	for ops := 1; ops <= 32 && bound == 0; ops++ {
		if verify(ops, a) == nil {
			bound = ops
		}
	}
	if bound == 0 {
		t.Fatal("one pinned revision did not verify within 32 Git operations")
	}
	pinned := func(revisions ...string) *Artifact {
		c := *a
		c.Manifest.Inputs = append([]Input{}, a.Manifest.Inputs...)
		for _, revision := range revisions {
			input := a.Manifest.Inputs[0]
			input.Revision = revision
			c.Manifest.Inputs = append(c.Manifest.Inputs, input)
		}
		return &c
	}
	extra := make([]string, 0, 5)
	for i := 0; i < 5; i++ {
		git(t, root, "commit", "-q", "--allow-empty", "-m", "pin")
		extra = append(extra, git(t, root, "rev-parse", "HEAD"))
	}
	six := pinned(extra...)
	if err := verify(bound, six); err != nil {
		t.Fatalf("six pinned revisions under the one-revision bound of %d operations: %v", bound, err)
	}
	if err := verify(bound-1, six); err == nil {
		t.Fatalf("six pinned revisions verified under %d operations, below the measured bound", bound-1)
	}
	missing := strings.Repeat("f", 40)
	if err := verify(bound, pinned(append(extra, missing)...)); err == nil || !strings.Contains(err.Error(), "invalid tree identity") {
		t.Fatalf("missing revision: %v", err)
	}
	if err := verify(bound, pinned("not-a-revision")); err == nil || !strings.Contains(err.Error(), "tree requires a pinned commit") {
		t.Fatalf("malformed revision: %v", err)
	}
	wrongTree := pinned(missing)
	wrongTree.Tree = strings.Repeat("1", 40)
	if a.Manifest.Repository.Revision > missing {
		t.Fatal("fixture precondition: the primary revision must sort before the missing one")
	}
	if err := verify(bound, wrongTree); err == nil || !strings.Contains(err.Error(), "recorded tree differs") {
		t.Fatalf("primary tree mismatch before a missing revision: %v", err)
	}
}
