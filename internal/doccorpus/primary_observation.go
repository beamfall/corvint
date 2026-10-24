package doccorpus

import (
	"context"
	"sync"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// primaryObservation is the live observation of a read's primary checkout, started when the
// read starts rather than when its artifact header arrives (DCR-V1-465). The primary checkout
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

// observePrimaryTree observes a read's primary checkout; tests replace it to hold the
// observation open.
var observePrimaryTree = contextindex.ObserveTree

// SetPrimaryObservationForTest replaces how a read's primary checkout is observed and returns
// the restore. It is for tests that must hold the started observation open; it is not safe to
// call while a read is running.
func SetPrimaryObservationForTest(observe func(context.Context, string) (*contextindex.TreeObservation, error)) (restore func()) {
	saved := observePrimaryTree
	observePrimaryTree = observe
	return func() { observePrimaryTree = saved }
}

// WithPrimaryObservation starts observing root's tree on its own goroutine and returns a
// context carrying it. The first live freshness of root under the returned context uses that
// observation, at most once; any later one observes the checkout again (DCR-V1-466). The
// returned join cancels nothing: it waits for the observation, so the caller cancels ctx first
// and then joins, and no observation outlives the read (DCR-V1-467).
func WithPrimaryObservation(ctx context.Context, root string) (context.Context, func()) {
	p := &primaryObservation{root: root, done: make(chan struct{})}
	observe := observePrimaryTree
	go func() {
		defer close(p.done)
		p.live, p.err = observe(ctx, root)
	}()
	return context.WithValue(ctx, primaryObservationKey{}, p), func() { <-p.done }
}

// observeTree is dir's live observation: the read's started primary observation when it is of
// dir and not yet used, otherwise a new one. The started observation shares the read's
// context, so a cancelled read ends it as it ends a new one. A caller whose own context ends
// first, such as one deriving a shorter deadline, stops waiting and gets its context's error as
// a new observation would; the read's cancel and join still end the started one (DCR-V1-467).
func observeTree(ctx context.Context, dir string) (*contextindex.TreeObservation, error) {
	if p, ok := ctx.Value(primaryObservationKey{}).(*primaryObservation); ok && p.root == dir && p.take() {
		select {
		case <-p.done:
			return p.live, p.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
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
