package doccorpus

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
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

// TestDCIV1630BudgetRefusalNamesBoundInForce: an exhausted Git operation
// budget names the bound the run carried, not the 1,024 default (V1-3762):
// gitrun's own refusal from a 1-operation budget spent twice, and a pinned
// revision check under a bound too small to open the checkout.
func TestDCIV1630BudgetRefusalNamesBoundInForce(t *testing.T) {
	t.Parallel()
	budget := gitrun.NewBudget(1, gitrun.DefaultTotalBudget)
	if _, err := budget.ReserveOperation(time.Second); err != nil {
		t.Fatal(err)
	}
	_, spent := budget.ReserveOperation(time.Second)
	check := func(err error, want string) {
		t.Helper()
		var refused *Error
		if !errors.As(err, &refused) || refused.Code != CapacityCode {
			t.Fatalf("not a capacity refusal: %v", err)
		}
		if !strings.HasPrefix(refused.Message, want) || strings.Contains(refused.Message, "1024") {
			t.Fatalf("refusal %q does not start %q", refused.Message, want)
		}
	}
	check(gitCapacity(spent, ""), "Git operation budget of 1 operations exhausted (")
	check(gitCapacity(cemcode.New(cemcode.GitBudgetExceeded, "Git operation budget exhausted"), ""), "Git operation budget exhausted (")
	root, a, _ := builtFixture(t)
	git(t, root, "commit", "-q", "--allow-empty", "-m", "pin")
	c := *a
	input := a.Manifest.Inputs[0]
	input.Revision = git(t, root, "rev-parse", "HEAD")
	c.Manifest.Inputs = append(append([]Input{}, a.Manifest.Inputs...), input)
	check(verifyPinnedRevisions(boundCorpusGitOps(context.Background(), 0), root, &c), "Git operation budget of 0 operations exhausted while verifying pinned revisions (")
}
