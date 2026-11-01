package store

import (
	"context"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
)

// deferredRetention holds the derived checkpoints a Mutate or lease writer
// decided under the writer lock, so their O(store) encoding and writing run
// after the lock is released (V1-2358, JSG-V0-891). The read checkpoint is
// staged and renamed into place without the lock: it is bound to the head
// identity it was derived at, so one older than the store is only ignored
// or walked forward by its reader. The writer checkpoint's base is staged
// without the lock and renamed, with its bounded delta written, under a
// short re-acquisition of the lock that publishes nothing unless the
// invalidation token is still the one read before the audit (JSG-V0-015).
type deferredRetention struct {
	read   *journal.Result
	writer func() (*journal.WriterCheckpoint, error)
	token  writerToken
}

type deferredRetentionKey struct{}

// deferredRetentionLockWait bounds the re-acquisition; a busy lock skips the
// writer checkpoint, which only costs a later write its tail walk or one
// complete audit.
const deferredRetentionLockWait = 2 * time.Second

// withDeferredRetention gives the writers under ctx a collector.
func withDeferredRetention(ctx context.Context, d *deferredRetention) context.Context {
	return context.WithValue(ctx, deferredRetentionKey{}, d)
}

// keepReadCheckpoint is retainCheckpoint, deferred when ctx carries a
// collector. The caller holds the writer lock.
func keepReadCheckpoint(ctx context.Context, repo *intent.Repository, proof *journal.Result) {
	if d, ok := ctx.Value(deferredRetentionKey{}).(*deferredRetention); ok && d != nil {
		d.read = proof
		return
	}
	retainCheckpoint(repo, proof)
}

// keepWriterCheckpoint is retainWriterCheckpoint of the checkpoint derive
// returns, derived and retained after the lock is released when ctx carries
// a collector. derive reads only the audit the caller made under the lock.
func keepWriterCheckpoint(ctx context.Context, repo *intent.Repository, derive func() (*journal.WriterCheckpoint, error), since writerToken) {
	if d, ok := ctx.Value(deferredRetentionKey{}).(*deferredRetention); ok && d != nil {
		d.writer, d.token = derive, since
		return
	}
	if wc, err := derive(); err == nil {
		retainWriterCheckpoint(repo, wc, since)
	}
}

// publish retains what the writer deferred. The caller holds no lock.
func (d *deferredRetention) publish(ctx context.Context, repo *intent.Repository) {
	if d.read != nil {
		staged := stageReadCheckpoint(repo, d.read)
		staged.publish()
	}
	if d.writer == nil || !d.token.ok || writerInvalidation(repo) != d.token {
		return
	}
	wc, err := d.writer()
	if err != nil {
		return
	}
	staged := stageWriterCheckpoint(repo, wc, d.token)
	defer staged.discard()
	lock, err := authority.AcquireLock(ctx, repo, authority.LockOptions{CallerWait: deferredRetentionLockWait})
	if err != nil {
		return
	}
	defer lock.Close()
	retainStagedWriterCheckpoint(repo, wc, d.token, staged)
}
