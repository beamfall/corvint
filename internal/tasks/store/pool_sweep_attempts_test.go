//go:build darwin || linux

package store_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/store"
)

// TestPSRV0022_UnprovedCleanupAfterRetryableExitLaunchesNoRetry (V1-1426)
// counts reset invocations over a whole sweep with maxAttempts 2. A reset that
// exits nonzero, a retryable class, is retried once when its group cleanup is
// proved; when the first cleanup probe fails, that unproved cleanup makes the
// attempt terminal (PSR-V0-022, PSR-V0-004), so the sweep launches no second
// reset and the member stays quarantined as UNKNOWN.
func TestPSRV0022_UnprovedCleanupAfterRetryableExitLaunchesNoRetry(t *testing.T) {
	for _, c := range []struct {
		name     string
		unproved bool
		resets   string
	}{{"proved-cleanup-retries", false, "xx"}, {"unproved-cleanup-is-terminal", true, "x"}} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			count, child := filepath.Join(dir, "resets"), filepath.Join(dir, "child")
			reset := "printf x >> " + count + "; /bin/sleep 60 </dev/null >/dev/null 2>&1 & echo $! > " + child + "; exit 1"
			s, _ := psrFixture(t, reset, "printf verified", "verified", "2", false)
			var calls *int
			if c.unproved {
				calls = store.PSRTestFirstCleanupProbe(t, child)
			}
			out, e := psrRun(s, "retryable-exit")
			if e != nil || out.Pending {
				t.Fatalf("sweep %+v %v", out, e)
			}
			if c.unproved && *calls != 1 {
				t.Fatalf("cleanup probe injection reached %d times, want 1", *calls)
			}
			raw, e := os.ReadFile(count)
			if e != nil || string(raw) != c.resets {
				t.Fatalf("reset invocations %q (%v), want %q", raw, e, c.resets)
			}
			p := psrPools(t, s)
			if len(p.Entries) != 1 || p.Entries[0].State != "QUARANTINED" {
				t.Fatalf("pool %s", psrEntryStates(p))
			}
			if c.unproved && !strings.Contains(p.Entries[0].Reason, "UNKNOWN") {
				t.Fatalf("unproved cleanup reason %q, want UNKNOWN", p.Entries[0].Reason)
			}
		})
	}
}
