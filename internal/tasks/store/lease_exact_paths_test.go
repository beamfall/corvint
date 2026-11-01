package store_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestATRV0801_DistinctDeclaredFilesAdmitConcurrently (V1-3185): tickets that
// declare distinct exact files under one directory are all admitted, even
// when their claims race, and each holds only its file.
func TestATRV0801_DistinctDeclaredFilesAdmitConcurrently(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	ids := []string{}
	for i := range 4 {
		ids = append(ids, s.planned(t, fmt.Sprintf("log-%d", i), "P2", fmt.Sprintf("docs/build-log/2026-10-31-entry-%d.md", i)))
	}
	at := s.at(t, 0)
	var wg sync.WaitGroup
	outcomes := make([]string, len(ids))
	for i, id := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := s.try(fmt.Sprintf("claim-%d", i), claimOf(id), at)
			if err != nil {
				outcomes[i] = err.Error()
				return
			}
			outcomes[i] = r.Outcome.Outcome + " " + r.Detail
		}()
	}
	wg.Wait()
	for i, o := range outcomes {
		if !strings.HasPrefix(o, mutation.OutcomeCompleted+" ") {
			t.Fatalf("claim %d: %s", i, o)
		}
	}
	entries := s.entries(t)
	if len(entries) != len(ids) {
		t.Fatalf("entries: %+v", entries)
	}
	for _, en := range entries {
		if len(en.Resources) != 1 || !strings.HasPrefix(en.Resources[0].Key, "docs/build-log/2026-10-31-entry-") {
			t.Fatalf("entry holds more than its file: %+v", en)
		}
	}
	auditOK(t, s.repo)
}

// TestATRV0800_CollisionNamesTheNextAction (V1-3185): the same exact file
// still collides and two racing claims on it admit exactly one; a directory
// holder or claimant refuses with the set-effects hint; a declared directory
// claimed with a narrower --scope names that the declared scope is held
// (ATR-V0-215). Every refusal writes nothing.
func TestATRV0800_CollisionNamesTheNextAction(t *testing.T) {
	t.Parallel()
	const hint = "declare exact file touchPaths with ticket set-effects before claiming (ATR-V0-800)"
	const held = "this claim holds the ticket's declared scope, not --scope (ATR-V0-215)"
	s := newLeaseStore(t)
	file := "docs/build-log/2026-10-31-a.md"
	a, b := s.planned(t, "a", "P2", file), s.planned(t, "b", "P2", file)
	at := s.at(t, 0)
	var wg sync.WaitGroup
	won := make([]bool, 2)
	for i, id := range []string{a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := s.try(fmt.Sprintf("race-%d", i), claimOf(id), at)
			won[i] = err == nil && r.Outcome.Outcome == mutation.OutcomeCompleted
		}()
	}
	wg.Wait()
	if won[0] == won[1] {
		t.Fatalf("racing claims on one file: admitted %v", won)
	}
	loser := b
	if won[1] {
		loser = a
	}
	same := s.lease(t, "same-file", claimOf(loser), 0, nil)
	refusedWith(t, same, mutation.OutcomeBlocked, wire.CodeResourceCollision)
	if strings.Contains(same.Detail, hint) || strings.Contains(same.Detail, held) {
		t.Fatalf("exact-file collision carries a directory or scope hint: %q", same.Detail)
	}

	dir := s.planned(t, "dir", "P2", "docs/build-log/")
	before := storeDigest(t, s.repo)
	claimant := s.lease(t, "dir-claimant", claimOf(dir), 0, nil)
	refusedWith(t, claimant, mutation.OutcomeBlocked, wire.CodeResourceCollision)
	if !strings.Contains(claimant.Detail, hint) || strings.Contains(claimant.Detail, held) {
		t.Fatalf("directory claimant detail: %q", claimant.Detail)
	}

	narrowed := s.lease(t, "dir-narrowed", claimOf(dir, "docs/build-log/2026-10-31-z.md"), 0, nil)
	refusedWith(t, narrowed, mutation.OutcomeBlocked, wire.CodeResourceCollision)
	if !strings.Contains(narrowed.Detail, hint) || !strings.Contains(narrowed.Detail, held) {
		t.Fatalf("declared directory with --scope detail: %q", narrowed.Detail)
	}
	if storeDigest(t, s.repo) != before {
		t.Fatal("refused claim wrote")
	}

	// A live directory holder refuses an exact file beneath it the same way.
	t2 := newLeaseStore(t)
	t2.claim(t, "holder", t2.planned(t, "holder", "P2", "docs/build-log/"), 0)
	under := t2.lease(t, "under", claimOf(t2.planned(t, "under", "P2", file)), 0, nil)
	refusedWith(t, under, mutation.OutcomeBlocked, wire.CodeResourceCollision)
	if !strings.Contains(under.Detail, hint) {
		t.Fatalf("file under a directory holder detail: %q", under.Detail)
	}
	auditOK(t, s.repo)
}
