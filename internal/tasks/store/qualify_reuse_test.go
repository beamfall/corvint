//go:build darwin || linux

package store_test

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/authority"
)

// LGL-V0-890 (V1-2203): with the common dir's qualification reused, a
// heartbeat leaves no probe file and makes every directory sync it made with
// a fresh probe except the probe's own sync of the common dir: each artifact
// it publishes keeps its file sync and destination directory sync
// (LGL-V0-008), so an acknowledged heartbeat is as durable as before.
func TestLGLV0890_HeartbeatKeepsEveryArtifactSyncWithAReusedQualification(t *testing.T) {
	s := newLeaseStore(t)
	id := s.ticket(t, "one")
	claim := s.claim(t, "claim-1", id, 0, "src/")
	common, err := os.Stat(s.repo.CommonDir)
	if err != nil {
		t.Fatal(err)
	}
	type syncs struct{ total, common int }
	heartbeat := func(request string, at int) syncs {
		t.Helper()
		// Overlapped publish syncs (LGL-V0-016) call the hook from several
		// goroutines at once, so the counts are atomic.
		var total, onCommon atomic.Int64
		restore := authority.SetDirectorySyncTestHook(func(dir os.FileInfo, next func() error) error {
			total.Add(1)
			if os.SameFile(dir, common) {
				onCommon.Add(1)
			}
			return next()
		})
		defer restore()
		s.lease(t, request, heartbeatOf(claim), at, nil)
		return syncs{total: int(total.Load()), common: int(onCommon.Load())}
	}
	reused := heartbeat("heartbeat-reused", 1)
	entries, err := os.ReadDir(s.repo.CommonDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".taskman-qualify-") {
			t.Fatalf("probe file %s left in the common dir", filepath.Join(s.repo.CommonDir, e.Name()))
		}
	}
	if reused.common != 0 {
		t.Fatalf("a heartbeat with a reused qualification synced the common dir %d times", reused.common)
	}
	if reused.total < 4 {
		t.Fatalf("heartbeat made %d directory syncs, want its receipt, posts and head synced", reused.total)
	}
	if got := heartbeat("heartbeat-again", 2); got.total != reused.total || got.common != 0 {
		t.Fatalf("second heartbeat made %+v syncs, want %d and none of the common dir", got, reused.total)
	}
}
