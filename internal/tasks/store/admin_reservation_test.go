package store_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0023_AdministrationReservesOnlyExecutionEffects
// (AGENT-TASKS-ADMIN-RESERVATION-1): store administration (claim, submit,
// gate, renew, release) adds no resource beyond each attempt's declared
// scope, so disjoint private work on one store is admitted side by side. An
// effects change is refused ATTEMPT_LIVE without writing while the ticket's
// attempt is live, so no live reservation or acceptance-bound gate result is
// reused across it; once released, the change moves acceptanceRevision past
// the old attempt.
func TestCALV0023_AdministrationReservesOnlyExecutionEffects(t *testing.T) {
	t.Parallel()
	s := newGateStore(t)
	a, b, c := s.ticket(t, "a"), s.ticket(t, "b"), s.ticket(t, "c")
	claimA, _ := s.submitted(t, a, "src", 0)
	s.passes(t, "gate-a", gateOf(claimA, "verify"), 2)
	claimB := s.claim(t, "claim-b", b, 3, "docs/")
	for _, live := range []*store.Report{claimA, claimB} {
		if r := s.lease(t, "renew-"+live.AttemptID, renewOf(live), 4, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("renew %s: %+v", live.AttemptID, r)
		}
	}
	held := map[string]string{}
	for _, en := range s.entries(t) {
		if len(en.Resources) != 1 {
			t.Fatalf("entry holds more than its scope: %+v", en)
		}
		k := en.Resources[0].Key
		if strings.HasPrefix(k, ".git") || strings.HasPrefix(k, ".taskman") {
			t.Fatalf("administration reserved %q", k)
		}
		held[en.AttemptID] = k
	}
	if held[claimA.AttemptID] != "src/" || held[claimB.AttemptID] != "docs/" {
		t.Fatalf("reservations: %v", held)
	}

	reserved := s.entries(t)
	effects := func(requestID string, minutes int) (*store.Report, error) {
		rec := s.record(t, a)
		return store.Mutate(context.Background(), s.repo, operator(), envelope(requestID, mutation.OpSetEffects, a, string(rec.Revision), setEffectsPayload([]string{"lib/"}, nil)), s.at(t, minutes))
	}
	before := storeDigest(t, s.repo)
	if r, err := effects("effects-live", 5); err != nil || r.Outcome.Outcome != mutation.OutcomeBlocked || !has(r.Outcome.Codes, wire.CodeAttemptLive) {
		t.Fatalf("set-effects with a live attempt: %+v %v", r, err)
	}
	if storeDigest(t, s.repo) != before || !reflect.DeepEqual(s.entries(t), reserved) {
		t.Fatal("a refused effects change wrote")
	}

	for _, live := range []*store.Report{claimA, claimB} {
		if r := s.lease(t, "release-"+live.AttemptID, releaseOf(live), 6, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("release %s: %+v", live.AttemptID, r)
		}
	}
	prior := s.record(t, a).AcceptanceRevision
	if r, err := effects("effects-quiet", 7); err != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("set-effects after release: %+v %v", r, err)
	}
	if got := s.record(t, a).AcceptanceRevision; got == prior || got == s.attempt(t, claimA.AttemptID).TicketRevision {
		t.Fatalf("effects change left acceptanceRevision at %s", got)
	}
	s.claim(t, "claim-c", c, 8, "docs/")
	auditOK(t, s.repo)
}
