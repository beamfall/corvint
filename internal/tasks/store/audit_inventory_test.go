//go:build darwin || linux

package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Existing parent-I/O fault witnesses intentionally exercise the verified
// cache route. Prove that route's real prerequisite rather than bypassing the
// optimization whenever hooks are installed.
func primeAuditInventory(t *testing.T, repo *intent.Repository) {
	t.Helper()
	guard, err := authority.WatchChanges(repo)
	if err != nil {
		t.Fatal(err)
	}
	inv, err := inventory(repo)
	if err != nil {
		guard.Close()
		t.Fatal(err)
	}
	head, _, _, err := journalBytes(repo)
	if err != nil {
		guard.Close()
		t.Fatal(err)
	}
	proof, err := leaseAudit(repo, guard, inv, head)
	closeErr := guard.Close()
	if err != nil || closeErr != nil || proof == nil || proof.IntentError != nil || !possibleLeaseAudit(repo, head) {
		t.Fatalf("verified cache fixture: %+v %v close%v", proof, err, closeErr)
	}
}

func auditInventoryObservation(t *testing.T, repo *intent.Repository) (*authority.ChangeGuard, journal.PhysicalObservation) {
	t.Helper()
	g, err := authority.WatchChanges(repo)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { g.Close() })
	head, _, _, err := journalBytes(repo)
	if err != nil {
		t.Fatal(err)
	}
	h, err := snapshot.DecodeHead(head)
	if err != nil {
		t.Fatal(err)
	}
	proof, observed, err := journalReader(repo, h).AuditForWriteObserved()
	if err != nil || observed.Cleanup != nil || proof.IntentError != nil || proof.Mode != journal.ModeFull || len(observed.Files) == 0 {
		t.Fatalf("initialized observation %+v %v", observed, err)
	}
	return g, observed
}

func TestCALV0026_AuditInventoryParityAndFallback(t *testing.T) {
	t.Run("populated-5000-and-unread-evidence", func(t *testing.T) {
		repo := cachedLeaseRepo(t)
		fr := &fixture.Repo{Root: repo.PrimaryWorktree, CommonDir: repo.CommonDir, StateDir: repo.StateDir, IntentDir: filepath.Join(repo.PrimaryWorktree, intent.Dir)}
		h, err := snapshot.DecodeHead(inventoryProbeRead(t, filepath.Join(repo.StateDir, "head.json")))
		if err != nil {
			t.Fatal(err)
		}
		for seq := h.LastSeq.Uint64() + 1; seq <= 5000; seq++ {
			id := fmt.Sprintf("audit-inventory-%d", seq)
			path, e := snapshot.RequestPath(id)
			if e != nil {
				t.Fatal(e)
			}
			inventoryProbeAppendReceipt(t, fr, "MUTATION", map[string][]byte{path: inventoryProbeRequestBytes(id, seq, wire.Sum([]byte(id)), false)}, id, true, true, seq == 5000)
		}
		unread := []byte("unreferenced retained evidence")
		unreadPath := "evidence/" + string(wire.Sum(unread))
		fixture.Write(t, filepath.Join(repo.StateDir, unreadPath), unread)
		baseline, err := inventory(repo)
		if err != nil {
			t.Fatal(err)
		}
		g, observed := auditInventoryObservation(t, repo)
		if _, ok := observed.Files[unreadPath]; ok {
			t.Fatal("unread evidence unexpectedly observed")
		}
		hooks := hooksForInventory(context.Background())
		reader := hooks.read
		fallback := map[string]int{}
		hooks.read = func(root *os.Root, path, name string, max int) ([]byte, error) {
			fallback[path]++
			return reader(root, path, name, max)
		}
		actual, err, cleanup := guardedLeaseInventory(repo, g, hooks, observed.Files)
		if err != nil || len(cleanup) != 0 || !reflect.DeepEqual(actual.Files(), baseline.Files()) || !reflect.DeepEqual(actual.Directories(), baseline.Directories()) {
			t.Fatalf("parity %v cleanup%v", err, cleanup)
		}
		if fallback[filepath.Join(repo.StateDir, unreadPath)] != 1 || len(observed.Files) < 10000 {
			t.Fatalf("paths unexercised observed%d fallback%v", len(observed.Files), fallback)
		}
		hits := 0
		for _, f := range baseline.Files() {
			if _, ok := observed.Files[f.Path]; ok {
				hits++
				full := filepath.Join(repo.StateDir, f.Path)
				if strings.HasPrefix(f.Path, "intent/") {
					full = filepath.Join(fr.IntentDir, strings.TrimPrefix(f.Path, "intent/"))
				}
				if fallback[full] != 0 {
					t.Fatalf("borrowed bytes reread %s", f.Path)
				}
			}
		}
		if hits < 10000 {
			t.Fatalf("insufficient exercised hits %d", hits)
		}
		t.Logf("physical metadata hits=%d fallback paths=%d complete inventory=%d", hits, len(fallback), len(actual.Files()))
	})
	t.Run("same-path-actual-byte-bound", func(t *testing.T) {
		// Existing inventory uses the archive path here; BoundFor accepts
		// intent-relative paths, so this exact path uses the evidence bound.
		// Borrowed metadata must preserve that effective bound too.
		bound := wire.MaxEvidenceBlobBytes
		reads := 0
		reader := func(string, int) ([]byte, error) { reads++; return nil, nil }
		_, _, err := fileEntryWithReader("unused", "intent/queue.json", reader, map[string]journal.PhysicalFile{"intent/queue.json": {Sha256: wire.Sum(nil), Bytes: bound + 1}})
		if wire.CodeOf(err) != wire.CodeLimitExceeded || reads != 0 {
			t.Fatalf("bound %v reads%d", err, reads)
		}
		file, present, err := fileEntryWithReader("unused", "evidence/empty", reader, map[string]journal.PhysicalFile{"evidence/empty": {Sha256: wire.Sum(nil), Bytes: 0}})
		if err != nil || !present || file.Bytes != "0" || reads != 0 {
			t.Fatalf("empty %+v %v", file, err)
		}
	})
	for _, kind := range []string{"same-bytes-change", "membership", "parent-replacement", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			repo := cachedLeaseRepo(t)
			g, obs := auditInventoryObservation(t, repo)
			switch kind {
			case "same-bytes-change":
				p := filepath.Join(repo.StateDir, "head.json")
				fixture.Write(t, p, inventoryProbeRead(t, p))
			case "membership":
				fixture.Write(t, filepath.Join(repo.StateDir, "evidence", string(wire.Sum([]byte("new")))), []byte("new"))
			case "parent-replacement":
				old := repo.StateDir + "-old"
				if err := os.Rename(repo.StateDir, old); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(repo.StateDir, 0700); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { os.RemoveAll(repo.StateDir); os.Rename(old, repo.StateDir) })
			case "symlink":
				p := filepath.Join(repo.StateDir, "VERSION")
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("head.json", p); err != nil {
					t.Fatal(err)
				}
			}
			if g.Check() == nil {
				t.Fatal("injected change not observed")
			}
			inv, err, cleanup := guardedLeaseInventory(repo, g, hooksForInventory(context.Background()), obs.Files)
			if inv != nil || wire.CodeOf(err) != wire.CodeSnapshotMoved || len(cleanup) != 0 {
				t.Fatalf("borrowed stale metadata %v %v", inv, err)
			}
		})
	}
	t.Run("divergent-intent-replay-and-fresh-refusal", func(t *testing.T) {
		repo, choice := inventoryClaimRepo(t)
		actor := mutation.Binding{ID: "tester", Role: "OWNER"}
		first, err := Lease(context.Background(), repo, actor, choice, fixture.Timestamp)
		if err != nil || first.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("fixture claim %+v %v", first, err)
		}
		// Canonical ticket exists and the physical empty edit is a supported intent divergence.
		tickets, err := filepath.Glob(filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets", "*.json"))
		if err != nil || len(tickets) != 1 {
			t.Fatalf("tickets %v %v", tickets, err)
		}
		fixture.Write(t, tickets[0], nil)
		headBefore := inventoryProbeRead(t, filepath.Join(repo.StateDir, "head.json"))
		hooks := hooksForInventory(context.Background())
		audit := hooks.audit
		divergent := 0
		hooks.audit = func(r journal.Reader) (*journal.Result, journal.PhysicalObservation, error) {
			p, o, e := audit(r)
			if p != nil && p.IntentError != nil {
				divergent++
				if o.Files != nil {
					t.Fatal("divergent metadata published")
				}
			}
			return p, o, e
		}
		ctx := context.WithValue(context.Background(), inventoryHooksKey{}, hooks)
		replay, err := Lease(ctx, repo, actor, choice, fixture.Timestamp)
		if err != nil || replay.Kind != "Replay" || replay.AttemptID != first.AttemptID || divergent < 2 {
			t.Fatalf("fallback replay %+v %v audits%d", replay, err, divergent)
		}
		choice.RequestID = "fresh-divergent"
		fresh, err := Lease(ctx, repo, actor, choice, fixture.Timestamp)
		if wire.CodeOf(err) != wire.CodeIntentDiverged || fresh.Receipt != "" {
			t.Fatalf("fresh refusal %+v %v", fresh, err)
		}
		if string(headBefore) != string(inventoryProbeRead(t, filepath.Join(repo.StateDir, "head.json"))) {
			t.Fatal("replay/refusal moved head")
		}
	})
}

func TestCALV0026_AuditInventoryFatalCleanup(t *testing.T) {
	t.Run("fallback-close-after-metadata-tail", func(t *testing.T) {
		repo := cachedLeaseRepo(t)
		unread := []byte("fallback")
		fixture.Write(t, filepath.Join(repo.StateDir, "evidence", string(wire.Sum(unread))), unread)
		g, obs := auditInventoryObservation(t, repo)
		hooks := hooksForInventory(context.Background())
		closeRoot := hooks.close
		injected := false
		hooks.close = func(root *os.Root) error {
			err := closeRoot(root)
			if err != nil {
				return err
			}
			if !injected {
				injected = true
				return errors.New("injected fallback close")
			}
			return nil
		}
		inv, err, cleanup := guardedLeaseInventory(repo, g, hooks, obs.Files)
		if !injected {
			t.Fatal("fallback close not reached")
		}
		if inv != nil || len(cleanup) == 0 {
			t.Fatalf("close ignored %v %v %v", inv, err, cleanup)
		}
	})
	for _, dirty := range []bool{false, true} {
		t.Run(fmt.Sprintf("journal-close-dirty-%t", dirty), func(t *testing.T) {
			repo, choice := inventoryClaimRepo(t)
			actor := mutation.Binding{ID: "tester", Role: "OWNER"}
			headBefore := inventoryProbeRead(t, filepath.Join(repo.StateDir, "head.json"))
			hooks := hooksForInventory(context.Background())
			audit := hooks.audit
			calls, guardCloses, prepCloses, acquires := 0, 0, 0, 0
			hooks.audit = func(r journal.Reader) (*journal.Result, journal.PhysicalObservation, error) {
				calls++
				proof, obs, err := audit(r)
				if err != nil || obs.Cleanup != nil || len(obs.Files) == 0 || proof.IntentError != nil {
					t.Fatalf("fault initialization %+v %v", obs, err)
				}
				if dirty {
					p := filepath.Join(repo.StateDir, "head.json")
					fixture.Write(t, p, headBefore)
				}
				// Journal-package lifetime tests inject actual native file/root closure.
				// Here inject that separate return channel at the store boundary.
				return nil, journal.PhysicalObservation{Cleanup: errors.New("injected journal close")}, wire.Errorf(wire.CodeUnsupportedFilesystem, "read lifetime", "injected journal close")
			}
			hooks.closeGuard = func(g *authority.ChangeGuard) error {
				guardCloses++
				if dirty && g.Check() == nil {
					t.Fatal("dirty-guard injection not reached")
				}
				return g.Close()
			}
			hooks.closePreparation = func(p *authority.PreparationLock) error { prepCloses++; return p.Close() }
			ctx := context.WithValue(context.Background(), inventoryHooksKey{}, hooks)
			ctx = authority.WithLockObserver(ctx, func(time.Duration) { acquires++ })
			report, err := Lease(ctx, repo, actor, choice, fixture.Timestamp)
			if calls != 1 {
				t.Fatalf("audit hook reached %d times; expected one terminal attempt", calls)
			}
			if wire.CodeOf(err) != wire.CodeUnsupportedFilesystem || report.Receipt != "" || acquires != 0 || guardCloses != 1 || prepCloses != 1 {
				t.Fatalf("fatal precedence %+v %v acquire%d guard%d prep%d", report, err, acquires, guardCloses, prepCloses)
			}
			if possibleLeaseAudit(repo, headBefore) {
				t.Fatal("failed audit filled cache")
			}
			if string(headBefore) != string(inventoryProbeRead(t, filepath.Join(repo.StateDir, "head.json"))) {
				t.Fatal("fatal failure committed")
			}
		})
	}
}
