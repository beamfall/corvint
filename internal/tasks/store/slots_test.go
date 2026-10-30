//go:build unix

package store_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// slotsStore is a lease store whose policy declares one slots resource gpu
// of the given capacity, with the given class reservations.
func slotsStore(t *testing.T, capacity string, reserved ...string) *leaseStore {
	t.Helper()
	s := newLeaseStore(t)
	s.slotsPolicy(t, now(t), "2", capacity, reserved...)
	return s
}

// slotsPolicy publishes, at at, the policy version after expected declaring
// gpu with capacity and the class reservations.
func (s *leaseStore) slotsPolicy(t *testing.T, at wire.Timestamp, expected, capacity string, reserved ...string) {
	t.Helper()
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", str(string(wire.SizeOf(wire.Size(expected).Uint64()+1))))
	v.Obj.Set("capacity", obj("maxActiveAttempts", str("4"), "maxWorkersTotal", str("4"), "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	gpu := obj("id", str("gpu"), "capacity", str(capacity))
	if len(reserved) > 0 {
		classes := wire.NewObject()
		for i := 0; i+1 < len(reserved); i += 2 {
			classes.Set(reserved[i], str(reserved[i+1]))
		}
		gpu.Obj.Set("reserved", wire.ObjectValue(classes))
	}
	v.Obj.Set("slots", wire.Array(gpu))
	rep, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("slots-"+expected, expected, wire.EncodeFile(v)), at)
	if e != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy %+v %v", rep, e)
	}
}

func slotAcquire(a *store.Report, count string) transaction.LeaseRequest {
	return transaction.LeaseRequest{Verb: transaction.LeaseSlotAcquire, AttemptID: a.AttemptID, Generation: a.Generation, Slots: "gpu", SlotCount: wire.Count(count)}
}

func slotExclusive(a *store.Report) transaction.LeaseRequest {
	return transaction.LeaseRequest{Verb: transaction.LeaseSlotAcquire, AttemptID: a.AttemptID, Generation: a.Generation, Slots: "gpu", SlotExclusive: true}
}

func slotClass(a *store.Report, count, class string) transaction.LeaseRequest {
	l := slotAcquire(a, count)
	l.SlotClass = class
	return l
}

func slotRelease(a *store.Report) transaction.LeaseRequest {
	return transaction.LeaseRequest{Verb: transaction.LeaseSlotRelease, AttemptID: a.AttemptID, Generation: a.Generation, Slots: "gpu"}
}

// slotIs asserts a slot command completed and left the attempt in state,
// with the 1-based waiter position when WAITING.
func slotIs(t *testing.T, r *store.Report, state string, position int) {
	t.Helper()
	if r.Outcome.Outcome != mutation.OutcomeCompleted || r.Slot == nil || r.Slot.SlotsID != "gpu" || r.Slot.State != state || r.Slot.Position != position {
		t.Fatalf("want slot %s at %d, got %+v slot %+v", state, position, r, r.Slot)
	}
}

// slotEntries reads the pools.json slot entries keyed by attempt.
func (s *leaseStore) slotEntries(t *testing.T) map[string]snapshot.SlotEntry {
	t.Helper()
	out := map[string]snapshot.SlotEntry{}
	raw, err := os.ReadFile(filepath.Join(s.repo.StateDir, "pools.json"))
	if os.IsNotExist(err) {
		return out
	}
	if err != nil {
		t.Fatal(err)
	}
	state, err := snapshot.DecodePools(raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, en := range state.Slots {
		out[en.AttemptID] = en
	}
	return out
}

// TestPSRV0032_SlotsGrantWaitAndFreeWithoutOperator: a request that fits is
// HELD, one that does not is WAITING; an ended attempt's slots and a crashed
// holder's slots (its lease expired, reaped by the next slot write) free
// without operator action, and the next poll grants the waiter.
func TestPSRV0032_SlotsGrantWaitAndFreeWithoutOperator(t *testing.T) {
	t.Parallel()
	s := slotsStore(t, "2")
	one, two, three := s.ticket(t, "one"), s.ticket(t, "two"), s.ticket(t, "three")
	a := s.claim(t, "claim-a", one, 0, "a/")
	b := s.claim(t, "claim-b", two, 0, "b/")
	slotIs(t, s.lease(t, "a-1", slotAcquire(a, "2"), 1, nil), snapshot.SlotHeld, 0)
	slotIs(t, s.lease(t, "b-1", slotAcquire(b, ""), 2, nil), snapshot.SlotWaiting, 1)
	if en := s.slotEntries(t); len(en) != 2 || en[a.AttemptID].Count != "2" || en[b.AttemptID].Count != "1" || en[b.AttemptID].Holder != "agent-1" {
		t.Fatalf("entries %+v", en)
	}
	// An ended attempt frees its slots: the next poll grants the waiter and
	// prunes the dead entry.
	if r := s.lease(t, "release-a", releaseOf(a), 3, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("release %+v", r)
	}
	slotIs(t, s.lease(t, "b-2", slotAcquire(b, ""), 4, nil), snapshot.SlotHeld, 0)
	if en := s.slotEntries(t); len(en) != 1 || en[b.AttemptID].State != snapshot.SlotHeld {
		t.Fatalf("after release %+v", en)
	}
	// A crashed holder: b's lease expires at minute 60; c waits, and its
	// poll after the expiry reaps b and takes the freed slots.
	c := s.claim(t, "claim-c", three, 30, "c/")
	slotIs(t, s.lease(t, "c-1", slotAcquire(c, "2"), 31, nil), snapshot.SlotWaiting, 1)
	got := s.lease(t, "c-2", slotAcquire(c, "2"), 70, nil)
	slotIs(t, got, snapshot.SlotHeld, 0)
	if len(got.Reaped) != 1 || got.Reaped[0].AttemptID != b.AttemptID || s.attempt(t, b.AttemptID).Live() {
		t.Fatalf("crashed holder not reaped %+v", got)
	}
	if en := s.slotEntries(t); len(en) != 1 || en[c.AttemptID].State != snapshot.SlotHeld {
		t.Fatalf("after reap %+v", en)
	}
	auditOK(t, s.repo)
}

// TestPSRV0033_ExclusiveEventuallyGrantedUnderSharedLoad: an exclusive
// request waits for every holder to leave; while it waits no new shared
// request is granted, even with slots free, so it is granted once the
// current holders release, and shared waiters resume after it.
func TestPSRV0033_ExclusiveEventuallyGrantedUnderSharedLoad(t *testing.T) {
	t.Parallel()
	s := slotsStore(t, "2")
	one, two, three := s.ticket(t, "one"), s.ticket(t, "two"), s.ticket(t, "three")
	a := s.claim(t, "claim-a", one, 0, "a/")
	b := s.claim(t, "claim-b", two, 0, "b/")
	x := s.claim(t, "claim-x", three, 0, "x/")
	slotIs(t, s.lease(t, "a-1", slotAcquire(a, "1"), 1, nil), snapshot.SlotHeld, 0)
	slotIs(t, s.lease(t, "x-1", slotExclusive(x), 2, nil), snapshot.SlotWaiting, 1)
	// One slot is free, but a shared request behind the exclusive waits.
	slotIs(t, s.lease(t, "b-1", slotAcquire(b, "1"), 3, nil), snapshot.SlotWaiting, 2)
	// Steady shared load: a releases and asks again; it queues behind x.
	slotIs(t, s.lease(t, "a-2", slotRelease(a), 4, nil), "NONE", 0)
	if en := s.slotEntries(t)[x.AttemptID]; en.State != snapshot.SlotHeld || !en.Exclusive || en.Count != "2" {
		t.Fatalf("exclusive not granted %+v", en)
	}
	slotIs(t, s.lease(t, "a-3", slotAcquire(a, "1"), 5, nil), snapshot.SlotWaiting, 2)
	slotIs(t, s.lease(t, "b-2", slotAcquire(b, "1"), 6, nil), snapshot.SlotWaiting, 1)
	slotIs(t, s.lease(t, "x-2", slotRelease(x), 7, nil), "NONE", 0)
	slotIs(t, s.lease(t, "b-3", slotAcquire(b, "1"), 8, nil), snapshot.SlotHeld, 0)
	slotIs(t, s.lease(t, "a-4", slotAcquire(a, "1"), 9, nil), snapshot.SlotHeld, 0)
	auditOK(t, s.repo)
}

// TestPSRV0034_WaitersGrantedByPriorityThenRequestOrder: a later P1 waiter
// is granted before an earlier P3 waiter.
func TestPSRV0034_WaitersGrantedByPriorityThenRequestOrder(t *testing.T) {
	t.Parallel()
	s := slotsStore(t, "1")
	holder := s.planned(t, "holder", "P2", "h/")
	low := s.planned(t, "low", "P3", "l/")
	high := s.planned(t, "high", "P1", "x/")
	h := s.claim(t, "claim-h", holder, 0, "h/")
	l := s.claim(t, "claim-l", low, 0, "l/")
	x := s.claim(t, "claim-x", high, 0, "x/")
	slotIs(t, s.lease(t, "h-1", slotAcquire(h, ""), 1, nil), snapshot.SlotHeld, 0)
	slotIs(t, s.lease(t, "l-1", slotAcquire(l, ""), 2, nil), snapshot.SlotWaiting, 1)
	slotIs(t, s.lease(t, "x-1", slotAcquire(x, ""), 3, nil), snapshot.SlotWaiting, 1)
	slotIs(t, s.lease(t, "l-2", slotAcquire(l, ""), 4, nil), snapshot.SlotWaiting, 2)
	if en := s.slotEntries(t); en[l.AttemptID].Priority != "P3" || en[x.AttemptID].Priority != "P1" {
		t.Fatalf("priorities %+v", en)
	}
	slotIs(t, s.lease(t, "h-2", slotRelease(h), 5, nil), "NONE", 0)
	slotIs(t, s.lease(t, "x-2", slotAcquire(x, ""), 6, nil), snapshot.SlotHeld, 0)
	slotIs(t, s.lease(t, "l-3", slotAcquire(l, ""), 7, nil), snapshot.SlotWaiting, 1)
	auditOK(t, s.repo)
}

// TestPSRV0035_ReservedClassNeverStarved: unreserved requests never take a
// class's unused reservation, even from a higher-priority waiter ahead of
// it, and a request beyond what the class could ever get is refused.
func TestPSRV0035_ReservedClassNeverStarved(t *testing.T) {
	t.Parallel()
	s := slotsStore(t, "3", "ci", "1")
	bulk := s.planned(t, "bulk", "P2", "a/")
	urgent := s.planned(t, "urgent", "P0", "b/")
	nightly := s.planned(t, "nightly", "P3", "c/")
	a := s.claim(t, "claim-a", bulk, 0, "a/")
	b := s.claim(t, "claim-b", urgent, 0, "b/")
	c := s.claim(t, "claim-c", nightly, 0, "c/")
	before := storeDigest(t, s.repo)
	refusedWith(t, s.lease(t, "a-too-many", slotAcquire(a, "3"), 1, nil), mutation.OutcomeValidationFailed, wire.CodeLimitExceeded)
	refusedWith(t, s.lease(t, "a-bad-class", slotClass(a, "1", "nightly"), 1, nil), mutation.OutcomeValidationFailed, wire.CodeMalformed)
	if storeDigest(t, s.repo) != before {
		t.Fatal("refused acquire wrote")
	}
	slotIs(t, s.lease(t, "a-1", slotAcquire(a, "2"), 2, nil), snapshot.SlotHeld, 0)
	slotIs(t, s.lease(t, "b-1", slotAcquire(b, "1"), 3, nil), snapshot.SlotWaiting, 1)
	slotIs(t, s.lease(t, "c-1", slotClass(c, "1", "ci"), 4, nil), snapshot.SlotHeld, 0)
	slotIs(t, s.lease(t, "b-2", slotAcquire(b, "1"), 5, nil), snapshot.SlotWaiting, 1)
	slotIs(t, s.lease(t, "a-2", slotRelease(a), 6, nil), "NONE", 0)
	slotIs(t, s.lease(t, "b-3", slotAcquire(b, "1"), 7, nil), snapshot.SlotHeld, 0)
	auditOK(t, s.repo)
}

// TestPSRV0036_ReleaseAndRefusals: release drops a waiting entry, releasing
// nothing changes no entry (its receipt is PSR-V0-042's), a barrier pauses
// acquire but not release, and a fenced, unknown or re-parameterized request
// refuses without a write.
func TestPSRV0036_ReleaseAndRefusals(t *testing.T) {
	t.Parallel()
	s := slotsStore(t, "1")
	one, two := s.ticket(t, "one"), s.ticket(t, "two")
	a := s.claim(t, "claim-a", one, 0, "a/")
	b := s.claim(t, "claim-b", two, 0, "b/")
	if r := s.lease(t, "none", slotRelease(a), 1, nil); r.Outcome.Outcome != mutation.OutcomeCompleted || r.Slot == nil || r.Slot.State != "NONE" || len(s.slotEntries(t)) != 0 {
		t.Fatalf("empty release %+v", r)
	}
	slotIs(t, s.lease(t, "a-1", slotAcquire(a, "1"), 1, nil), snapshot.SlotHeld, 0)
	slotIs(t, s.lease(t, "b-1", slotAcquire(b, "1"), 2, nil), snapshot.SlotWaiting, 1)
	stale := slotAcquire(b, "1")
	stale.Generation = wire.SizeOf(b.Generation.Uint64() + 1)
	refusedWith(t, s.lease(t, "b-stale", stale, 3, nil), mutation.OutcomeRevisionConflict, wire.CodeFenced)
	unknown := slotAcquire(b, "1")
	unknown.Slots = "tpu"
	refusedWith(t, s.lease(t, "b-unknown", unknown, 3, nil), mutation.OutcomeValidationFailed, wire.CodeMalformed)
	refusedWith(t, s.lease(t, "b-other", slotExclusive(b), 3, nil), mutation.OutcomeBlocked, wire.CodeResourceCollision)
	if en := s.slotEntries(t); len(en) != 2 || en[b.AttemptID].Exclusive {
		t.Fatalf("refusals changed entries %+v", en)
	}
	for _, bad := range []transaction.LeaseRequest{
		{Verb: transaction.LeaseSlotAcquire, AttemptID: b.AttemptID, Generation: b.Generation, Slots: "gpu", SlotExclusive: true, SlotCount: "1"},
		{Verb: transaction.LeaseSlotAcquire, AttemptID: b.AttemptID, Generation: b.Generation, Slots: "gpu", SlotCount: "0"},
		{Verb: transaction.LeaseSlotAcquire, AttemptID: b.AttemptID, Generation: b.Generation},
		{Verb: transaction.LeaseSlotRelease, AttemptID: b.AttemptID, Generation: b.Generation, Slots: "gpu", SlotCount: "1"},
	} {
		r, err := s.try("bad", bad, s.at(t, 3))
		if err == nil && (r.Outcome.Outcome == mutation.OutcomeCompleted || len(r.Outcome.Codes) == 0) {
			t.Fatalf("malformed %+v accepted: %+v", bad, r)
		}
	}
	if r, err := store.Barrier(context.Background(), s.repo, operator(), barrierRequest(transaction.Pause, "pause"), s.at(t, 4)); err != nil || r.Receipt == "" {
		t.Fatalf("pause %+v %v", r, err)
	}
	refusedWith(t, s.lease(t, "a-paused", slotAcquire(a, "1"), 5, nil), mutation.OutcomeBlocked, wire.CodePaused)
	slotIs(t, s.lease(t, "b-2", slotRelease(b), 5, nil), "NONE", 0)
	if en := s.slotEntries(t); len(en) != 1 || en[a.AttemptID].State != snapshot.SlotHeld {
		t.Fatalf("after release %+v", en)
	}
	auditOK(t, s.repo)
}

// TestPSRV0037_AcquireIdempotentByRequestID: retrying a request ID replays
// its committed receipt without a write (with no slot member, PSR-V0-043); a
// poll with a fresh ID and the same parameters leaves pools.json unchanged
// while nothing changes.
func TestPSRV0037_AcquireIdempotentByRequestID(t *testing.T) {
	t.Parallel()
	s := slotsStore(t, "1")
	one, two := s.ticket(t, "one"), s.ticket(t, "two")
	a := s.claim(t, "claim-a", one, 0, "a/")
	b := s.claim(t, "claim-b", two, 0, "b/")
	first := s.lease(t, "a-1", slotAcquire(a, "1"), 1, nil)
	slotIs(t, first, snapshot.SlotHeld, 0)
	before := storeDigest(t, s.repo)
	again := s.lease(t, "a-1", slotAcquire(a, "1"), 2, nil)
	if again.Kind != "Replay" || !again.Outcome.Replayed || again.Outcome.ReceiptSeq == nil || first.Outcome.ReceiptSeq == nil || *again.Outcome.ReceiptSeq != *first.Outcome.ReceiptSeq || storeDigest(t, s.repo) != before || again.Slot != nil {
		t.Fatalf("replay %+v", again)
	}
	pools := s.poolsBytes(t)
	poll := s.lease(t, "a-2", slotAcquire(a, "1"), 2, nil)
	slotIs(t, poll, snapshot.SlotHeld, 0)
	if string(s.poolsBytes(t)) != string(pools) {
		t.Fatalf("poll changed pools.json %+v", poll)
	}
	slotIs(t, s.lease(t, "b-1", slotAcquire(b, "1"), 3, nil), snapshot.SlotWaiting, 1)
	before = storeDigest(t, s.repo)
	if r := s.lease(t, "b-1", slotAcquire(b, "1"), 4, nil); r.Kind != "Replay" || storeDigest(t, s.repo) != before {
		t.Fatalf("waiting replay %+v", r)
	}
	auditOK(t, s.repo)
}

func (s *leaseStore) poolsBytes(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(s.repo.StateDir, "pools.json"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// TestPSRV0039_SupervisorAttachRefusedWithSlots is in the transaction
// package (planSupervisor).

// TestPSRV0040_GrantsFollowCurrentPolicy: after the capacity drops, a queued
// exclusive request is granted the current capacity, not its enqueue-time
// count, and a waiter the new policy can never grant neither is granted nor
// blocks a later waiter; its own poll refuses as a fresh acquire would.
func TestPSRV0040_GrantsFollowCurrentPolicy(t *testing.T) {
	t.Parallel()
	s := slotsStore(t, "2")
	one, two, three := s.ticket(t, "one"), s.ticket(t, "two"), s.ticket(t, "three")
	a := s.claim(t, "claim-a", one, 0, "a/")
	x := s.claim(t, "claim-x", two, 0, "x/")
	b := s.claim(t, "claim-b", three, 0, "b/")
	slotIs(t, s.lease(t, "a-1", slotAcquire(a, "1"), 1, nil), snapshot.SlotHeld, 0)
	slotIs(t, s.lease(t, "x-1", slotExclusive(x), 2, nil), snapshot.SlotWaiting, 1)
	s.slotsPolicy(t, s.at(t, 3), "3", "1")
	slotIs(t, s.lease(t, "a-2", slotRelease(a), 4, nil), "NONE", 0)
	if en := s.slotEntries(t)[x.AttemptID]; en.State != snapshot.SlotHeld || en.Count != "1" {
		t.Fatalf("exclusive granted above the current capacity %+v", en)
	}
	slotIs(t, s.lease(t, "x-2", slotExclusive(x), 5, nil), snapshot.SlotHeld, 0)
	slotIs(t, s.lease(t, "x-3", slotRelease(x), 6, nil), "NONE", 0)
	s.slotsPolicy(t, s.at(t, 7), "4", "3")
	slotIs(t, s.lease(t, "a-3", slotAcquire(a, "1"), 8, nil), snapshot.SlotHeld, 0)
	slotIs(t, s.lease(t, "b-1", slotAcquire(b, "3"), 9, nil), snapshot.SlotWaiting, 1)
	slotIs(t, s.lease(t, "x-4", slotAcquire(x, "1"), 10, nil), snapshot.SlotWaiting, 2)
	s.slotsPolicy(t, s.at(t, 11), "5", "2")
	slotIs(t, s.lease(t, "x-5", slotAcquire(x, "1"), 12, nil), snapshot.SlotHeld, 0)
	refusedWith(t, s.lease(t, "b-2", slotAcquire(b, "3"), 13, nil), mutation.OutcomeValidationFailed, wire.CodeLimitExceeded)
	if en := s.slotEntries(t); en[b.AttemptID].State != snapshot.SlotWaiting || en[a.AttemptID].State != snapshot.SlotHeld {
		t.Fatalf("entries %+v", en)
	}
	slotIs(t, s.lease(t, "b-3", slotRelease(b), 14, nil), "NONE", 0)
	auditOK(t, s.repo)
}

// TestPSRV0041_ExclusiveDemandCannotStarveReservedClass: two P0 exclusive
// requesters alternating cannot pass a P3 reserved-class request enqueued
// before the next exclusive one, and a reserved-class request enqueued after
// an exclusive one still waits for it (PSR-V0-033).
func TestPSRV0041_ExclusiveDemandCannotStarveReservedClass(t *testing.T) {
	t.Parallel()
	s := slotsStore(t, "2", "ci", "1")
	x1t, x2t := s.planned(t, "x1", "P0", "x/"), s.planned(t, "x2", "P0", "y/")
	wt, w2t := s.planned(t, "w", "P3", "w/"), s.planned(t, "w2", "P3", "v/")
	x1 := s.claim(t, "claim-x1", x1t, 0, "x/")
	x2 := s.claim(t, "claim-x2", x2t, 0, "y/")
	w := s.claim(t, "claim-w", wt, 0, "w/")
	w2 := s.claim(t, "claim-w2", w2t, 0, "v/")
	slotIs(t, s.lease(t, "x1-1", slotExclusive(x1), 1, nil), snapshot.SlotHeld, 0)
	slotIs(t, s.lease(t, "w-1", slotClass(w, "1", "ci"), 2, nil), snapshot.SlotWaiting, 1)
	slotIs(t, s.lease(t, "x2-1", slotExclusive(x2), 3, nil), snapshot.SlotWaiting, 1)
	slotIs(t, s.lease(t, "x1-2", slotRelease(x1), 4, nil), "NONE", 0)
	if en := s.slotEntries(t); en[w.AttemptID].State != snapshot.SlotHeld || en[x2.AttemptID].State != snapshot.SlotWaiting {
		t.Fatalf("a later exclusive passed the reserved class %+v", en)
	}
	slotIs(t, s.lease(t, "x1-3", slotExclusive(x1), 5, nil), snapshot.SlotWaiting, 2)
	slotIs(t, s.lease(t, "w2-1", slotClass(w2, "1", "ci"), 6, nil), snapshot.SlotWaiting, 3)
	slotIs(t, s.lease(t, "w-2", slotRelease(w), 7, nil), "NONE", 0)
	if en := s.slotEntries(t); en[x2.AttemptID].State != snapshot.SlotHeld || en[w2.AttemptID].State != snapshot.SlotWaiting {
		t.Fatalf("exclusive not granted after the earlier reserved request %+v", en)
	}
	slotIs(t, s.lease(t, "x2-2", slotRelease(x2), 8, nil), "NONE", 0)
	if en := s.slotEntries(t); en[x1.AttemptID].State != snapshot.SlotHeld || en[w2.AttemptID].State != snapshot.SlotWaiting {
		t.Fatalf("earlier exclusive not granted before the later reserved request %+v", en)
	}
	slotIs(t, s.lease(t, "x1-4", slotRelease(x1), 9, nil), "NONE", 0)
	slotIs(t, s.lease(t, "w2-2", slotClass(w2, "1", "ci"), 10, nil), snapshot.SlotHeld, 0)
	auditOK(t, s.repo)
}

// TestPSRV0042_NoChangeRequestsReplay: a release with no entry and a poll
// that grants nothing each record a receipt, so retrying either request id
// replays it instead of releasing a later grant or re-enqueueing.
func TestPSRV0042_NoChangeRequestsReplay(t *testing.T) {
	t.Parallel()
	s := slotsStore(t, "1")
	a := s.claim(t, "claim-a", s.ticket(t, "one"), 0, "a/")
	if r := s.lease(t, "none", slotRelease(a), 1, nil); r.Kind != "Transaction" || r.Outcome.Outcome != mutation.OutcomeCompleted || r.Receipt == "" {
		t.Fatalf("empty release not recorded %+v", r)
	}
	slotIs(t, s.lease(t, "a-1", slotAcquire(a, "1"), 2, nil), snapshot.SlotHeld, 0)
	if r := s.lease(t, "none", slotRelease(a), 3, nil); !r.Outcome.Replayed || s.slotEntries(t)[a.AttemptID].State != snapshot.SlotHeld {
		t.Fatalf("retried empty release released a later grant %+v", r)
	}
	if r := s.lease(t, "poll", slotAcquire(a, "1"), 4, nil); r.Kind != "Transaction" || r.Receipt == "" {
		t.Fatalf("no-change poll not recorded %+v", r)
	}
	slotIs(t, s.lease(t, "a-2", slotRelease(a), 5, nil), "NONE", 0)
	if r := s.lease(t, "poll", slotAcquire(a, "1"), 6, nil); !r.Outcome.Replayed || len(s.slotEntries(t)) != 0 {
		t.Fatalf("retried poll re-enqueued %+v", r)
	}
	auditOK(t, s.repo)
}

// TestPSRV0043_ReplayCarriesNoSlot: a replay does not report a later
// request's grant as its own.
func TestPSRV0043_ReplayCarriesNoSlot(t *testing.T) {
	t.Parallel()
	s := slotsStore(t, "2")
	a := s.claim(t, "claim-a", s.ticket(t, "one"), 0, "a/")
	slotIs(t, s.lease(t, "r", slotAcquire(a, "1"), 1, nil), snapshot.SlotHeld, 0)
	slotIs(t, s.lease(t, "release", slotRelease(a), 2, nil), "NONE", 0)
	slotIs(t, s.lease(t, "s", slotAcquire(a, "2"), 3, nil), snapshot.SlotHeld, 0)
	if r := s.lease(t, "r", slotAcquire(a, "1"), 4, nil); !r.Outcome.Replayed || r.Slot != nil {
		t.Fatalf("replay reported %+v slot %+v", r, r.Slot)
	}
	auditOK(t, s.repo)
}
