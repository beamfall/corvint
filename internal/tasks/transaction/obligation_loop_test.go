package transaction

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTOLV0018_LastRaiseEndsNoProgressRun: a lastRaise that names the
// attempt, a counted generation and the record's acceptance revision makes
// that generation progress, so the trailing run ends and a claim at the
// loop threshold is admitted. A null or non-matching lastRaise keeps the
// CAL-V0-102 hold unchanged.
func TestTOLV0018_LastRaiseEndsNoProgressRun(t *testing.T) {
	t.Run("TOL-V0-018", func(t *testing.T) {
		rec := issue502Asking(t, "AT-01", 0)
		id := rec.TicketID.Raw
		looping := loopAttempt(rec, handoff("implement", ""), handoff("implement", ""), handoff("implement", ""))
		policy := loopPolicy("2", "2")
		attempts := map[string]*snapshot.Attempt{"a": looping}
		raise := func(attempt, gen string, ar wire.Count) {
			rec.ObligationsRef = &ticket.ObligationsReference{Prefix: "AT", LastRaise: &ticket.ObligationRaise{Attempt: attempt, Generation: wire.Size(gen), AcceptanceRevision: ar}}
		}
		for _, c := range []struct {
			name, attempt, gen string
			ar                 wire.Count
			gens               string
		}{
			{"another attempt", "attempt:acme:main:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "3", rec.AcceptanceRevision, "1,2,3"},
			{"another acceptance revision", looping.AttemptID, "3", "9", "1,2,3"},
			{"an uncounted generation", looping.AttemptID, "7", rec.AcceptanceRevision, "1,2,3"},
		} {
			raise(c.attempt, c.gen, c.ar)
			if h := LoopHoldOf(attempts, rec, policy); h == nil || strings.Join(h.Generations, ",") != c.gens {
				t.Fatalf("%s: hold %+v", c.name, h)
			}
		}
		rec.ObligationsRef = &ticket.ObligationsReference{Prefix: "AT"}
		if h := LoopHoldOf(attempts, rec, policy); h == nil {
			t.Fatal("a null lastRaise released the hold")
		}
		raise(looping.AttemptID, "1", rec.AcceptanceRevision)
		if h := LoopHoldOf(attempts, rec, policy); h != nil {
			t.Fatalf("a raise at generation 1 leaves a run of two: %+v", h)
		}
		raise(looping.AttemptID, "3", rec.AcceptanceRevision)
		if h := LoopHoldOf(attempts, rec, policy); h != nil {
			t.Fatalf("the raising generation is progress: %+v", h)
		}
		c := issue502ClaimContext(t, LeaseClaim, id, rec)
		c.st.policy.LoopDetection = &intent.LoopDetection{MaxNoProgressGenerations: "2", MaxAlternatingReturns: "2"}
		c.st.attempts["a"] = looping
		if out := c.eligibility(id); out != nil {
			t.Fatalf("claim at the loop threshold refused after a raise: %+v", out.result)
		}
		rec.ObligationsRef = nil
		if out := c.eligibility(id); out == nil {
			t.Fatal("without the raise the claim is admitted")
		} else if code, _ := issue502Refused(*out); code != wire.CodeLoopDetected {
			t.Fatalf("without the raise: %s", code)
		}
	})
}
