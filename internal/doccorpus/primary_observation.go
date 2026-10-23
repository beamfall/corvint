package doccorpus

import (
	"context"
	"sync"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// primaryObservation is the live observation of a read's primary checkout, started when the
// read starts rather than when its artifact header arrives (DCP-V1-465). The primary checkout
// is the read's own root, so observing it needs nothing the artifact records.
type primaryObservation struct {
	root string
	done chan struct{}
	live *contextindex.TreeObservation
	err  error

	mu    sync.Mutex
	taken bool
}

type primaryObservationKey struct{}

// WithPrimaryObservation starts observing root's tree on its own goroutine and returns a
// context carrying it. The first live freshness of root under the returned context uses that
// observation, at most once; any later one observes the checkout again (DCP-V1-466). The
// returned join cancels nothing: it waits for the observation, so the caller cancels ctx first
// and then joins, and no observation outlives the read (DCP-V1-467).
func WithPrimaryObservation(ctx context.Context, root string) (context.Context, func()) {
	p := &primaryObservation{root: root, done: make(chan struct{})}
	go func() {
		defer close(p.done)
		p.live, p.err = contextindex.ObserveTree(ctx, root)
	}()
	return context.WithValue(ctx, primaryObservationKey{}, p), func() { <-p.done }
}

// observeTree is dir's live observation: the read's started primary observation when it is of
// dir and not yet used, otherwise a new one. The started observation shares the read's
// context, so a cancelled read ends it as it ends a new one.
func observeTree(ctx context.Context, dir string) (*contextindex.TreeObservation, error) {
	if p, ok := ctx.Value(primaryObservationKey{}).(*primaryObservation); ok && p.root == dir && p.take() {
		<-p.done
		return p.live, p.err
	}
	return contextindex.ObserveTree(ctx, dir)
}

// take claims the observation for its one use.
func (p *primaryObservation) take() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.taken {
		return false
	}
	p.taken = true
	return true
}
