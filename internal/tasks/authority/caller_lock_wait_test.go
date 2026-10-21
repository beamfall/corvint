//go:build darwin || linux

package authority_test

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-111: a caller wait outlasts the 30-second default on both the
// preparation admission and the writer lock, while a default waiter beside it
// still refuses LOCK_TIMEOUT at the §1 bound.
func TestCALV0111_CallerWaitOutlastsDefault(t *testing.T) {
	// Parallel: the test mostly sleeps past the default, so it overlaps the
	// package's CPU-bound fixture tests instead of adding 35 s of wall time
	// (V1-0348). The hold margin below absorbs the scheduling delay that
	// overlap adds.
	t.Parallel()
	if testing.Short() {
		t.Skip("holds both locks past the 30-second default")
	}
	_, repo := resolved(t)
	ctx := context.Background()
	prep, err := authority.AcquirePreparation(ctx, repo, authority.LockOptions{})
	if err != nil {
		t.Fatal(err)
	}
	writer, err := authority.AcquireLock(ctx, repo, authority.LockOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// The default waiters must refuse between the default and the release;
	// five seconds of margin keeps that window load-robust while the caller
	// wait (40 s) still outlasts the hold.
	hold := authority.DefaultLockWait + 5*time.Second
	type result struct {
		name    string
		elapsed time.Duration
		err     error
	}
	results := make(chan result, 4)
	var wg sync.WaitGroup
	try := func(name string, acquire func() (io.Closer, error)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			c, err := acquire()
			elapsed := time.Since(start)
			if err == nil {
				err = c.Close()
			}
			results <- result{name, elapsed, err}
		}()
	}
	caller := authority.LockOptions{CallerWait: 40 * time.Second}
	try("caller-preparation", func() (io.Closer, error) { return authority.AcquirePreparation(ctx, repo, caller) })
	try("caller-writer", func() (io.Closer, error) { return authority.AcquireLock(ctx, repo, caller) })
	try("default-preparation", func() (io.Closer, error) { return authority.AcquirePreparation(ctx, repo, authority.LockOptions{}) })
	try("default-writer", func() (io.Closer, error) { return authority.AcquireLock(ctx, repo, authority.LockOptions{}) })
	time.Sleep(hold)
	if err := prep.Close(); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	close(results)
	for r := range results {
		switch r.name {
		case "caller-preparation", "caller-writer":
			if r.err != nil || r.elapsed < authority.DefaultLockWait {
				t.Errorf("%s: %v after %v, want acquisition after the default", r.name, r.err, r.elapsed)
			}
		default:
			if wire.CodeOf(r.err) != wire.CodeLockTimeout || r.elapsed < authority.DefaultLockWait || r.elapsed >= hold {
				t.Errorf("%s: %v after %v, want LOCK_TIMEOUT at the default", r.name, r.err, r.elapsed)
			}
		}
	}
}
