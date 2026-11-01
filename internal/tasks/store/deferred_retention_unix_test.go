//go:build darwin || linux

package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
)

// JSG-V0-891 (V1-2358): a Mutate on the writer route, a Mutate on the
// complete route and the scheduled refresh write no O(store) derived
// checkpoint under the writer lock: the read checkpoint and the writer
// checkpoint base are encoded and written outside it, and under it only the
// bounded writer delta is written, the same size at 40 and at 400 tickets.
// The checkpoints they publish still serve the next writer.
func TestJSGV0891_WritersRetainCheckpointsOutsideWriterLock(t *testing.T) {
	measure := func(tickets int) map[string]derivedBytes {
		repo, _ := completeRouteClaim(t, tickets)
		out := map[string]derivedBytes{}
		run := func(name string, f func()) {
			got := derivedBytes{locked: map[string]int{}, unlocked: map[string]int{}}
			derivedWritten = func(path string, n int) {
				if writerLockProbe(t, repo) {
					got.locked[filepath.Base(path)] += n
				} else {
					got.unlocked[filepath.Base(path)] += n
				}
			}
			f()
			derivedWritten = nil
			out[name] = got
			t.Logf("%d tickets %-15s locked %v unlocked %v", tickets, name, got.locked, got.unlocked)
		}
		run("mutate-writer", func() {
			if r := writerMutate(t, repo, fmt.Sprintf("deferred-w-%d", tickets), nil); !r.completed() || !r.fast() {
				t.Fatalf("writer-route mutate: %v %v", r.err, r.stages)
			}
		})
		run("refresh", func() {
			completeRefreshOnly(t, repo)
			refreshWriterCheckpoint(context.Background(), repo)
		})
		run("mutate-complete", func() {
			// The complete route still compares the retained checkpoint
			// with its audit under the lock (JSG-V0-003); its cost is the
			// in-memory comparison timed here, bounded by the checkpoint.
			if old, _ := boundWriterCheckpoint(repo); old != nil {
				head, err := readHead(repo)
				if err != nil {
					t.Fatal(err)
				}
				proof, obs, err := journalReader(repo, head).AuditForWriteObserved()
				if err != nil {
					t.Fatal(err)
				}
				start := time.Now()
				if ok, err := proof.VerifyWriterCheckpoint(old, obs.Files); !ok || err != nil {
					t.Fatalf("verify: %v %v", ok, err)
				}
				t.Logf("%d tickets: verifyWriterCheckpoint compare %v", tickets, time.Since(start))
			}
			for _, p := range []string{journal.WriterCheckpointPath(repo.StateDir), journal.WriterBasePath(repo.StateDir), journal.CheckpointPath(repo.StateDir)} {
				os.Remove(p)
			}
			forgetLeaseAudits()
			if r := writerMutate(t, repo, fmt.Sprintf("deferred-c-%d", tickets), nil); !r.completed() || r.fast() {
				t.Fatalf("complete-route mutate: %v %v", r.err, r.stages)
			}
		})
		// The complete route's checkpoints serve the next writer.
		run("next-writer", func() {
			if r := writerMutate(t, repo, fmt.Sprintf("deferred-n-%d", tickets), nil); !r.completed() || !r.fast() {
				t.Fatalf("writer after the complete route: %v %v", r.err, r.stages)
			}
		})
		mustCheckpointFiles(t, repo)
		return out
	}
	small, large := measure(40), measure(400)
	delta := filepath.Base(journal.WriterCheckpointPath("taskman"))
	for _, route := range []string{"mutate-writer", "refresh", "mutate-complete", "next-writer"} {
		for _, c := range []struct {
			tickets int
			got     derivedBytes
		}{{40, small[route]}, {400, large[route]}} {
			for name, n := range c.got.locked {
				if name != delta && n != 0 {
					t.Errorf("%s at %d tickets wrote %d bytes of %s under the writer lock", route, c.tickets, n, name)
				}
			}
		}
		if large[route].locked[delta] > small[route].locked[delta]+32 {
			t.Errorf("%s: locked writer delta grew with tickets: %d at 40, %d at 400", route, small[route].locked[delta], large[route].locked[delta])
		}
	}
	for _, route := range []string{"refresh", "mutate-complete"} {
		if large[route].unlocked[filepath.Base(journal.WriterBasePath("taskman"))] == 0 {
			t.Errorf("%s at 400 tickets staged no writer base outside the lock: %v", route, large[route].unlocked)
		}
	}
}

// mustCheckpointFiles requires a read checkpoint and a writer checkpoint.
func mustCheckpointFiles(t *testing.T, repo *intent.Repository) {
	t.Helper()
	for _, p := range []string{journal.CheckpointPath(repo.StateDir), journal.WriterCheckpointPath(repo.StateDir), journal.WriterBasePath(repo.StateDir)} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
	}
}
