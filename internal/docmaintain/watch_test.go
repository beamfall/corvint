package docmaintain

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func watchOps() watchOperations { return watchOperations{readSourceIdentity, Preview, Apply} }
func watchFixture(t *testing.T) (string, Selector, Policy) {
	t.Helper()
	root := fixtureRepo(t, digestV1)
	writePage(t, root, "# Human\nPreserve me.\n")
	return root, Selector{"owner.md", "widget"}, Policy{Enabled: true, Apply: true, MaxWrites: 2, MaxWallClock: 5 * time.Second}
}

func TestWatchCommittedChangeAutomaticallyRefreshes(t *testing.T) {
	t.Run("SDD-V0-007 foreground refresh", func(t *testing.T) {
		root, selector, policy := watchFixture(t)
		ops := watchOps()
		calls := 0
		ops.apply = func(root string, preview *Result, policy Policy) (*Result, error) {
			result, err := Apply(root, preview, policy)
			calls++
			return result, err
		}
		// Change source at the next tick, after the prior cycle's post-write identity check.
		identities := 0
		ops.identity = func(ctx context.Context, root string) (sourceIdentity, error) {
			identities++
			if identities == 4 {
				if err := os.WriteFile(filepath.Join(root, "widget/w.go"), []byte("package widget\nfunc Changed() {}\n"), 0600); err != nil {
					t.Fatal(err)
				}
				commit(t, root, "changed")
			}
			return readSourceIdentity(ctx, root)
		}
		receipt, err := watch(context.Background(), root, "docs/page.md", selector, policy, time.Millisecond, ops)
		if err != nil || receipt.Writes != 2 || calls != 2 || receipt.StoppedReason != "max-writes" {
			t.Fatalf("%+v %v", receipt, err)
		}
		page := readPage(t, root)
		if !strings.Contains(page, "Changed") || !strings.Contains(page, "Preserve me.") || strings.Contains(page, "func Split") {
			t.Fatal("source replacement or human prose incorrect")
		}
	})
}

func TestWatchSourceDriftBeforeApplyRefuses(t *testing.T) {
	root, selector, policy := watchFixture(t)
	original := readPage(t, root)
	ops := watchOps()
	ops.preview = func(ctx context.Context, root, page string, selectors []Selector, policy Policy) (*Result, error) {
		result, err := Preview(ctx, root, page, selectors, policy)
		run(t, root, "-c", "user.name=t", "-c", "user.email=t@x", "commit", "--allow-empty", "-qm", "move HEAD")
		return result, err
	}
	receipt, err := watch(context.Background(), root, "docs/page.md", selector, policy, time.Millisecond, ops)
	if err == nil || receipt.StoppedReason != "source-drift" || receipt.Writes != 0 || readPage(t, root) != original {
		t.Fatalf("%+v %v", receipt, err)
	}
}

func TestWatchPostApplyDriftRetainsWrittenIdentity(t *testing.T) {
	root, selector, policy := watchFixture(t)
	ops := watchOps()
	written := ""
	ops.apply = func(root string, preview *Result, policy Policy) (*Result, error) {
		result, err := Apply(root, preview, policy)
		written = preview.Receipt.Blocks[0].Commit
		run(t, root, "-c", "user.name=t", "-c", "user.email=t@x", "commit", "--allow-empty", "-qm", "move HEAD")
		return result, err
	}
	receipt, err := watch(context.Background(), root, "docs/page.md", selector, policy, time.Millisecond, ops)
	if err == nil || receipt.StoppedReason != "source-superseded" || receipt.Complete || receipt.Writes != 1 || receipt.Summaries[0].Commit != written || receipt.Summaries[0].Status != "superseded" {
		t.Fatalf("%+v %v", receipt, err)
	}
	if !strings.Contains(readPage(t, root), written) {
		t.Fatal("written evidence was rolled back")
	}
}

func TestWatchIdleHumanEditAndDeletionStop(t *testing.T) {
	t.Run("SDD-V0-009 persistent human edit conflict", func(t *testing.T) {
		for _, remove := range []bool{false, true} {
			t.Run(map[bool]string{false: "edit", true: "delete"}[remove], func(t *testing.T) {
				root, selector, policy := watchFixture(t)
				ops := watchOps()
				calls := 0
				ops.identity = func(ctx context.Context, root string) (sourceIdentity, error) {
					calls++
					identity, err := readSourceIdentity(ctx, root)
					// End first cycle; second cycle has unchanged HEAD. Change just after its
					// opening check so the next idle cycle must catch it without Preview.
					if calls == 4 {
						if remove {
							if err := os.Remove(pagePath(root)); err != nil {
								t.Fatal(err)
							}
						} else {
							writePage(t, root, "human edited while idle\n")
						}
					}
					return identity, err
				}
				receipt, err := watch(context.Background(), root, "docs/page.md", selector, policy, time.Millisecond, ops)
				if err == nil || receipt.StoppedReason != "maintenance-conflict" || receipt.Writes != 1 {
					t.Fatalf("%+v %v", receipt, err)
				}
				if !remove && readPage(t, root) != "human edited while idle\n" {
					t.Fatal("human edit overwritten")
				}
			})
		}
	})
}

func TestWatchUnrelatedAndDirtyChangesDoNotWrite(t *testing.T) {
	t.Run("SDD-V0-008 cited source eligibility", func(t *testing.T) {
		root, selector, policy := watchFixture(t)
		// Cycle 1 writes (identity calls 1-3), cycle 2 sees the unrelated commit
		// and the dirty file (4-6), and cycle 3 stops the session (7). The wall
		// clock is only a hang bound, so host load cannot end the session early.
		policy.MaxWallClock = time.Minute
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ops := watchOps()
		calls := 0
		ops.identity = func(ctx context.Context, root string) (sourceIdentity, error) {
			calls++
			if calls == 4 {
				run(t, root, "-c", "user.name=t", "-c", "user.email=t@x", "commit", "--allow-empty", "-qm", "unrelated")
				if err := os.WriteFile(filepath.Join(root, "widget/w.go"), []byte("package widget\nfunc Dirty() {}\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if calls == 7 {
				cancel()
				return readSourceIdentity(context.Background(), root)
			}
			return readSourceIdentity(ctx, root)
		}
		receipt, err := watch(ctx, root, "docs/page.md", selector, policy, 10*time.Millisecond, ops)
		if err != nil || receipt.StoppedReason != "interrupted" || receipt.Cycles != 3 || receipt.Writes != 1 || strings.Contains(readPage(t, root), "func Dirty") {
			t.Fatalf("%+v %v", receipt, err)
		}
	})
}

func TestWatchOwnerRemovalRefuses(t *testing.T) {
	root, selector, policy := watchFixture(t)
	ops := watchOps()
	calls := 0
	ops.identity = func(ctx context.Context, root string) (sourceIdentity, error) {
		calls++
		if calls == 4 {
			if err := os.Remove(filepath.Join(root, "owner.md")); err != nil {
				t.Fatal(err)
			}
			commit(t, root, "remove owner")
		}
		return readSourceIdentity(ctx, root)
	}
	receipt, err := watch(context.Background(), root, "docs/page.md", selector, policy, time.Millisecond, ops)
	if err == nil || receipt.Writes != 1 || receipt.Complete {
		t.Fatalf("owner removal should refuse: %+v %v", receipt, err)
	}
}

func TestWatchBoundsAndSummary(t *testing.T) {
	t.Run("SDD-V0-010 bounded session output", func(t *testing.T) {
		root, selector, policy := watchFixture(t)
		policy.MaxWrites = 1
		receipt, err := Watch(context.Background(), root, "docs/page.md", selector, policy)
		if err != nil || receipt.Writes != 1 || receipt.Cycles != 1 {
			t.Fatalf("%+v %v", receipt, err)
		}
		for i := 0; i < 100000; i++ {
			receipt.add(sourceIdentity{strings.Repeat("a", 64), strings.Repeat("b", 64)}, strings.Repeat("c", 64), strings.Repeat("d", 64), "unchanged")
		}
		raw, err := json.Marshal(receipt)
		if err != nil || len(raw) >= 65536 || len(receipt.Summaries) != 32 || receipt.OmittedSummaries != 99969 {
			t.Fatal("aggregate not bounded")
		}
		writePage(t, root, strings.Repeat("x", watchPageLimit+1))
		if _, err := Watch(context.Background(), root, "docs/page.md", selector, policy); err == nil {
			t.Fatal("oversize page admitted")
		}
	})
}

func TestWatchDeadlineAndCancellation(t *testing.T) {
	root, selector, policy := watchFixture(t)
	policy.MaxWallClock = time.Nanosecond
	receipt, err := Watch(context.Background(), root, "docs/page.md", selector, policy)
	if err != nil || receipt.StoppedReason != "max-wall-clock" || receipt.Writes != 0 {
		t.Fatalf("%+v %v", receipt, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	policy.MaxWallClock = time.Minute
	receipt, err = Watch(ctx, root, "docs/page.md", selector, policy)
	if err != nil || receipt.StoppedReason != "interrupted" || receipt.Writes != 0 {
		t.Fatalf("%+v %v", receipt, err)
	}
}

func TestWatchIdentityPackedRefsAndLinkedWorktree(t *testing.T) {
	root, _, _ := watchFixture(t)
	run(t, root, "pack-refs", "--all")
	initial, err := readSourceIdentity(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(t.TempDir(), "linked")
	run(t, root, "worktree", "add", "--detach", linked, "HEAD")
	actual, err := readSourceIdentity(context.Background(), linked)
	if err != nil || actual != initial {
		t.Fatalf("%+v %+v %v", initial, actual, err)
	}
}

func TestWatchManyLinePageAvoidsQuadraticPreviewDiff(t *testing.T) {
	root, selector, policy := watchFixture(t)
	policy.MaxWrites = 1
	prefix := strings.Repeat("human\n", 40000)
	writePage(t, root, prefix)
	receipt, err := Watch(context.Background(), root, "docs/page.md", selector, policy)
	if err != nil || receipt.Writes != 1 || !strings.HasPrefix(readPage(t, root), prefix) {
		t.Fatalf("%+v %v", receipt, err)
	}
}

func TestWatchPostApplyCancellationIsIncomplete(t *testing.T) {
	root, selector, policy := watchFixture(t)
	ops := watchOps()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ops.apply = func(root string, preview *Result, policy Policy) (*Result, error) {
		result, err := Apply(root, preview, policy)
		cancel()
		return result, err
	}
	receipt, err := watch(ctx, root, "docs/page.md", selector, policy, time.Millisecond, ops)
	if err == nil || receipt.StoppedReason != "source-incomplete" || receipt.Complete || receipt.Writes != 1 {
		t.Fatalf("%+v %v", receipt, err)
	}
}

func TestWatchProposedPageBound(t *testing.T) {
	root, selector, policy := watchFixture(t)
	writePage(t, root, strings.Repeat("x", watchPageLimit))
	receipt, err := Watch(context.Background(), root, "docs/page.md", selector, policy)
	if err == nil || receipt.StoppedReason != "page-too-large" || receipt.Writes != 0 {
		t.Fatalf("%+v %v", receipt, err)
	}
}
