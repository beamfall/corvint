//go:build darwin || linux

package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-026: a moved read can retry; fatal audit/closure failures cannot.
func TestLeaseReadCanonical(t *testing.T) {
	proof, err := readLeaseProof(context.Background(), cachedLeaseRepo(t))
	if err != nil || proof == nil || proof.Mode != journal.ModeFull || proof.IntentError != nil {
		t.Fatalf("canonical read: proof=%+v err=%v", proof, err)
	}
}

func TestLeaseReadFatalAuditAndCleanup(t *testing.T) {
	for _, kind := range []string{"audit", "cleanup", "cleanup-with-movement", "snapshot-with-cleanup"} {
		t.Run(kind, func(t *testing.T) {
			repo := cachedLeaseRepo(t)
			// Prove the initialized fixture and canonical audit before injecting.
			g, obs := auditInventoryObservation(t, repo)
			if err := g.Close(); err != nil || obs.Cleanup != nil {
				t.Fatalf("fixture close: %v %+v", err, obs)
			}
			movement, err := authority.WatchChanges(repo)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { movement.Close() })
			injected := errors.New("injected " + kind + " failure")
			calls := 0
			proof, err := readLeaseProofWithAudit(context.Background(), repo, func(r journal.Reader) (*journal.Result, journal.PhysicalObservation, error) {
				calls++
				p, o, e := r.AuditForWriteObserved()
				if e != nil || o.Cleanup != nil || p == nil || p.IntentError != nil {
					t.Fatalf("canonical audit before injection: %v %+v", e, o)
				}
				if calls != 1 {
					return p, o, e
				}
				if kind != "cleanup" {
					// Same bytes, changed inode: guard movement must not be treated as corruption.
					path := filepath.Join(repo.StateDir, "head.json")
					raw, e := os.ReadFile(path)
					if e != nil {
						t.Fatal(e)
					}
					if e = os.WriteFile(path+".replacement", raw, 0600); e != nil {
						t.Fatal(e)
					}
					if e = os.Rename(path+".replacement", path); e != nil {
						t.Fatal(e)
					}
					if e = movement.Check(); wire.CodeOf(e) != wire.CodeSnapshotMoved {
						t.Fatalf("movement injection not observed: %v", e)
					}
				}
				if kind == "audit" {
					return p, o, injected
				}
				o.Cleanup = injected
				if kind == "snapshot-with-cleanup" {
					e = wire.Errorf(wire.CodeSnapshotMoved, "injection", "moved during close")
				}
				return p, o, e
			})
			if calls == 0 {
				t.Fatal("audit injection not reached")
			}
			if !errors.Is(err, injected) || calls != 1 || proof != nil {
				t.Fatalf("fatal error lost/retried: kind=%s calls=%d proofReturned=%v err=%v", kind, calls, proof != nil, err)
			}
		})
	}
}

func TestLeaseReadSnapshotRetry(t *testing.T) {
	repo := cachedLeaseRepo(t)
	calls := 0
	proof, err := readLeaseProofWithAudit(context.Background(), repo, func(r journal.Reader) (*journal.Result, journal.PhysicalObservation, error) {
		calls++
		if calls == 1 {
			return nil, journal.PhysicalObservation{}, wire.Errorf(wire.CodeSnapshotMoved, "injection", "retry")
		}
		return r.AuditForWriteObserved()
	})
	if err != nil || proof == nil || calls != 2 {
		t.Fatalf("snapshot retry: calls=%d proof=%v err=%v", calls, proof != nil, err)
	}
}
