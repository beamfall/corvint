// SPDX-License-Identifier: Apache-2.0
package watch

import (
	"bufio"
	"context"
	"io"
	"sync"
)

// Source delivers save notifications on C. C closes when the source stops;
// Err then reports why, nil for a stop the caller asked for (context end,
// end of input) and the failure otherwise, so the loop can refuse to keep a
// stale answer (LPCV-V0-1087).
type Source struct {
	C    <-chan struct{}
	mu   sync.Mutex
	err  error
	done bool
}

// Err is the reason C closed; it is nil while C is open.
func (s *Source) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.done {
		return nil
	}
	return s.err
}

// stop records why the source ended; the owner closes C afterwards.
func (s *Source) stop(err error) {
	s.mu.Lock()
	s.done, s.err = true, err
	s.mu.Unlock()
}

// Once is a source of exactly one notification, for a one-shot replan.
func Once() *Source {
	ch := make(chan struct{}, 1)
	ch <- struct{}{}
	close(ch)
	s := &Source{C: ch}
	s.stop(nil)
	return s
}

// Lines notifies once per line an editor writes to r, for hosts without a
// native event source; end of input or ctx stops it deliberately, a read
// error stops it with that error.
func Lines(ctx context.Context, r io.Reader) *Source {
	ch := make(chan struct{})
	s := &Source{C: ch}
	go func() {
		defer close(ch)
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			select {
			case ch <- struct{}{}:
			case <-ctx.Done():
				s.stop(nil)
				return
			}
		}
		s.stop(sc.Err())
	}()
	return s
}

// Native blocks on the operating system's save notifications for root:
// inotify on Linux, kqueue on macOS (LPCV-V0-1086). Other systems refuse
// with an explicit message naming the alternatives; no polling process or
// daemon is started anywhere.
func Native(ctx context.Context, root string) (*Source, error) {
	return native(ctx, root)
}
