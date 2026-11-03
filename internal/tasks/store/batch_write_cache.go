// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

type batchWriteKey struct{}
type batchWriteCache struct {
	mu        sync.Mutex
	tree      intent.TreeCapture
	inventory transaction.SummarizedInventoryCache
	indexRead bool
	indexRoot string
	index     *intent.ClosedIndex
	summaries map[wire.Digest]*intent.ClosedEntry
	queue     string
	policy    wire.Digest
}

// WithBatchWrites scopes bounded derived reuse to one sequential batch-close
// command. It grants no authority; planning may reuse bytes, but binding
// always reads and hashes the live content (TCX-V0-977..979).
func WithBatchWrites(ctx context.Context) context.Context {
	return context.WithValue(ctx, batchWriteKey{}, &batchWriteCache{})
}

func batchWrites(ctx context.Context) *batchWriteCache {
	c, _ := ctx.Value(batchWriteKey{}).(*batchWriteCache)
	return c
}

func captureWriteTree(ctx context.Context, repo *intent.Repository) (intent.Tree, error) {
	if c := batchWrites(ctx); c != nil {
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.tree.Capture(repo.IntentRoot())
	}
	return intent.TreeDigest(repo.IntentRoot())
}

func bindWriterObservation(ctx context.Context, repo *intent.Repository, identity journal.Identity, operation string) error {
	raw, err := intent.ReadFile(filepath.Join(repo.StateDir, "head.json"), wire.MaxJournalHeadBytes)
	if err != nil {
		return err
	}
	// Stat keys cannot detect every content change (notably MAP_SHARED writes).
	// CAL-V0-070 requires live bytes here, independently of the modeling cache.
	tree, err := intent.TreeDigest(repo.IntentRoot())
	if err != nil {
		return err
	}
	if !sameSnapshot(journal.Identity{HeadSha256: wire.Sum(raw), IntentTreeSha256: tree.Sha256}, identity) {
		return wire.Errorf(wire.CodeSnapshotMoved, "head.json", "validated head or intent changed")
	}
	_, err = writerGuards(repo, operation)
	return err
}

// LoadBatchIntent preserves the mutation CLI's validation/diagnostics while
// sharing the batch's independently validated native content capture.
func LoadBatchIntent(ctx context.Context, repo *intent.Repository, useIndex bool) (*intent.Store, error) {
	var index *intent.ClosedIndex
	if useIndex {
		if c := batchWrites(ctx); c != nil {
			c.mu.Lock()
			c.readIndex(repo)
			index = c.index
			c.mu.Unlock()
		} else {
			index = intent.ReadClosedIndex(repo.StateDir)
		}
	}
	if batchWrites(ctx) == nil {
		return intent.LoadSummarized(repo.IntentRoot(), index)
	}
	tree, err := captureWriteTree(ctx, repo)
	if err != nil {
		return nil, err
	}
	return intent.LoadSummarizedTree(repo.IntentRoot(), tree, index)
}

func writerSummaries(ctx context.Context, repo *intent.Repository, queue string, policy wire.Digest) map[wire.Digest]*intent.ClosedEntry {
	c := batchWrites(ctx)
	if c == nil {
		return intent.ReadClosedIndex(repo.StateDir).WriterSummaries(queue, policy)
	}
	// An admitted historical index remains useful only for canonical bytes
	// whose audited digest and length equal its entries. A derived index
	// rewrite cannot invalidate those immutable content bindings.
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readIndex(repo)
	if c.queue != queue || c.policy != policy {
		c.summaries = c.index.WriterSummaries(queue, policy)
		c.queue, c.policy = queue, policy
	}
	return c.summaries
}

func (c *batchWriteCache) readIndex(repo *intent.Repository) {
	if !c.indexRead || c.indexRoot != repo.StateDir {
		c.indexRead, c.indexRoot, c.index = true, repo.StateDir, intent.ReadClosedIndex(repo.StateDir)
		c.queue = ""
	}
}
