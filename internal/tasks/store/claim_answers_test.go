package store_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
)

// deliveredAnswers returns the request IDs a claim delivered, failing on
// unresolved material.
func deliveredAnswers(t *testing.T, r *store.Report) []string {
	t.Helper()
	if r.Delivery == nil || r.Delivery.EscalationAnswers.Err != nil {
		t.Fatalf("claim %s answers unresolved: %+v", r.AttemptID, r.Delivery)
	}
	ids := []string{}
	for _, v := range r.Delivery.EscalationAnswers.Value.Arr {
		id, _ := v.Obj.Get("requestId")
		answer, _ := v.Obj.Get("answer")
		actor, _ := v.Obj.Get("actor")
		source, _ := v.Obj.Get("source")
		if answer.Str != "take a" || actor.Str != operator().ID || source.Obj == nil {
			t.Fatalf("delivered answer %+v", v)
		}
		ids = append(ids, id.Str)
	}
	return ids
}

// TestESCV0005_ClaimDeliversTheAnswersPinnedByItsAdmission: a claim delivers
// the same-acceptance answers of its own admitted ticket snapshot, separate
// from the note. A later answer moves neither the live response nor its
// exact replay; a new generation and claim-next take their own snapshot; an
// open question is never delivered; a no-answer claim omits the attempt pin.
func TestESCV0005_ClaimDeliversTheAnswersPinnedByItsAdmission(t *testing.T) {
	s, id, src := escalationClaim(t)
	// escalationClaim admitted claim-1 before any question; replay reports it.
	first := s.lease(t, "claim-1", claimOf(id), 1, nil)
	if got := deliveredAnswers(t, first); len(got) != 0 {
		t.Fatalf("no-answer claim delivered %v", got)
	}
	raw, err := os.ReadFile(filepath.Join(s.repo.StateDir, "attempts", first.AttemptID+".json"))
	if err != nil || strings.Contains(string(raw), "escalationAnswers") {
		t.Fatalf("no-answer attempt carries an answer pin (%v)", err)
	}

	committed(t, escalate(t, s, holder, openRequest(t, "q-1", src, "", ""), 2), "OPEN")
	// An open infrastructure question holds no claim (ESC-V0-006), so the
	// next claim is admitted while it is open and must not deliver it.
	infra := bytes.Replace(openRequest(t, "q-2", src, "", ""), []byte(`"kind":"decision"`), []byte(`"kind":"infrastructure"`), 1)
	committed(t, escalate(t, s, holder, infra, 3), "OPEN")
	committed(t, answer(t, s, operator(), answerRequest(t, "a-1", id, operator(), "q-1", "1"), 4), "ANSWER")
	if r := s.lease(t, "release-1", releaseOf(first), 5, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("release = %+v %s", r.Outcome, r.Detail)
	}

	second := s.claim(t, "claim-2", id, 6)
	if got := deliveredAnswers(t, second); !slices.Equal(got, []string{"q-1"}) {
		t.Fatalf("claim delivered %v, want only the answered question", got)
	}
	a := s.attempt(t, second.AttemptID)
	if len(a.EscalationAnswers) != 1 || a.EscalationAnswers[0].RequestID != "q-1" || second.Delivery.TicketRecordSha256 != a.TicketRecordSha256 {
		t.Fatalf("attempt pin = %+v", a.EscalationAnswers)
	}

	// A post-commit answer changes neither the live claim nor its replay.
	committed(t, answer(t, s, operator(), answerRequest(t, "a-2", id, operator(), "q-2", "1"), 7), "ANSWER")
	replay := s.lease(t, "claim-2", claimOf(id), 8, nil)
	if !replay.Outcome.Replayed || replay.AttemptID != second.AttemptID || !slices.Equal(deliveredAnswers(t, replay), []string{"q-1"}) {
		t.Fatalf("replay = %+v delivered %v", replay.Outcome, deliveredAnswers(t, replay))
	}

	// claim-next re-snapshots from its own admission; delivery acknowledged nothing.
	if r := s.lease(t, "release-2", releaseOf(second), 9, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("release = %+v %s", r.Outcome, r.Detail)
	}
	next := s.lease(t, "next-1", claimNext, 10, nil)
	if next.Outcome.Outcome != mutation.OutcomeCompleted || next.Ticket != id || !slices.Equal(deliveredAnswers(t, next), []string{"q-1", "q-2"}) {
		t.Fatalf("claim-next = %+v ticket %s", next.Outcome, next.Ticket)
	}
}
