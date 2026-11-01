package store_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestATRV0532_StageWatcherToleratesRetryableHeartbeatFailure (V1-1493): the
// stage watcher's heartbeat renews a 60-minute lease, so a renewal that wrote
// nothing and may succeed on retry (LOCK_TIMEOUT on a loaded host) is retried
// on the next tick, not taken as a reason to cancel the stage; a renewal that
// keeps failing for the tolerance, or any non-retryable refusal, still stops
// it. The fake host waits until the watcher has renewed (or for the stop), so
// no case depends on how long a stage takes.
func TestATRV0532_StageWatcherToleratesRetryableHeartbeatFailure(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name      string
		code      string
		failFirst int // failing renewals before one succeeds; -1 fails every one
		tolerance time.Duration
		built     bool
		minHits   int
		maxHits   int
	}{
		// A stage expected to survive gets a tolerance no starved tick
		// reaches; one expected to stop gets a short one it must outlast.
		{"retryable-then-renewed", wire.CodeLockTimeout, 1, time.Hour, true, 2, 1 << 30},
		{"retryable-past-tolerance", wire.CodeLockTimeout, -1, 2 * time.Second, false, 2, 1 << 30},
		{"non-retryable", wire.CodeMalformed, -1, time.Hour, false, 1, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := buildProgramFixture(t, true, false, nil)
			beat := filepath.Join(f.scripts, "beat")
			if err := os.WriteFile(filepath.Join(f.scripts, "await-beat"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
			defer store.SetWatchTimingForTest(f.s.repo.StateDir, 0, c.tolerance)()
			var mu sync.Mutex
			hits := 0
			defer store.SetRunFaultForTest(f.s.repo.StateDir, func(point string) error {
				if point != "heartbeat" {
					return nil
				}
				mu.Lock()
				defer mu.Unlock()
				hits++
				if c.failFirst < 0 || hits <= c.failFirst {
					return wire.Errorf(c.code, "lock", "injected heartbeat failure %d", hits)
				}
				if err := os.WriteFile(beat, nil, 0o644); err != nil {
					t.Error(err)
				}
				return nil
			})()
			self, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			w, err := store.OpenWorkflow(context.Background(), f.s.repo, operator(), "program", self, f.config, f.ticketID)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			a, err := w.RunRole(context.Background(), "implementer", "")
			mu.Lock()
			n := hits
			mu.Unlock()
			if n < c.minHits || n > c.maxHits {
				t.Fatalf("heartbeat attempts %d, want %d..%d (err %v)", n, c.minHits, c.maxHits, err)
			}
			if got := err == nil && a != nil && a.Phase == "BUILT"; got != c.built {
				t.Fatalf("implement built=%v, want %v: %+v %v", got, c.built, a, err)
			}
		})
	}
}
