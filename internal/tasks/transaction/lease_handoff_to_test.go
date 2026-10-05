package transaction

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-082: the hand-off keys enter the RELEASE preimage only when
// supplied, so every historical preimage and replay is unchanged; the
// request-decidable combinations refuse MALFORMED.
func TestCALV0082_HandoffTargetPreimage(t *testing.T) {
	t.Run("CAL-V0-082 HandoffTargetPreimage", func(t *testing.T) {
		q, _ := wire.ParseQueueID("", fixture.QueueID)
		attempt := "attempt:acme:main:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		l := LeaseRequest{Verb: LeaseRelease, AttemptID: attempt, Generation: "1", Reason: wire.CodeHandoff}
		v, e := leaseValue(&l, q)
		if e != nil {
			t.Fatal(e)
		}
		want := `{"attemptId":"attempt:acme:main:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","base":null,"branch":null,"generation":"1","holder":null,"leaseMinutes":null,"reason":"HANDOFF","scope":null,"ticketId":null,"verb":"RELEASE","wholeRepository":false}`
		if got := string(wire.Encode(v)); got != want {
			t.Fatalf("legacy preimage changed: %s", got)
		}
		l.HandoffTo, l.HandoffReason = "review", "STAGE_COMPLETE"
		v, e = leaseValue(&l, q)
		if e != nil {
			t.Fatal(e)
		}
		if got := string(wire.Encode(v)); !strings.Contains(got, `"handoffReason":"STAGE_COMPLETE","handoffTo":"review"`) {
			t.Fatalf("target not bound: %s", got)
		}
		for name, bad := range map[string]LeaseRequest{
			"no-reason":        {Verb: LeaseRelease, AttemptID: attempt, Generation: "1", HandoffTo: "review"},
			"dirty-reason":     {Verb: LeaseRelease, AttemptID: attempt, Generation: "1", Reason: wire.CodeContaminated, HandoffTo: "review"},
			"reason-only":      {Verb: LeaseRelease, AttemptID: attempt, Generation: "1", Reason: wire.CodeHandoff, HandoffReason: "STAGE_COMPLETE"},
			"dirty-reason-why": {Verb: LeaseRelease, AttemptID: attempt, Generation: "1", Reason: wire.CodeContaminated, HandoffReason: "STAGE_COMPLETE"},
			"unknown-reason":   {Verb: LeaseRelease, AttemptID: attempt, Generation: "1", Reason: wire.CodeHandoff, HandoffTo: "review", HandoffReason: "DONE"},
			"unknown-stage":    {Verb: LeaseRelease, AttemptID: attempt, Generation: "1", Reason: wire.CodeHandoff, HandoffTo: "deploy"},
			"review-returned":  {Verb: LeaseRelease, AttemptID: attempt, Generation: "1", Reason: wire.CodeReviewReturned, HandoffTo: "review"},
			"renew":            {Verb: LeaseRenew, AttemptID: attempt, Generation: "1", HandoffTo: "review"},
		} {
			if _, e := leaseValue(&bad, q); wire.CodeOf(e) != wire.CodeMalformed {
				t.Fatalf("%s: accepted or wrong refusal: %v", name, e)
			}
		}
	})
}

// CAL-V0-084: nextStage is the latest generation's recorded or default
// hand-off at the current acceptance revision, STALE after it changed, and
// null when nothing is observed.
func TestCALV0084_NextStageDerivation(t *testing.T) {
	t.Run("CAL-V0-084 NextStageDerivation", func(t *testing.T) {
		rec := fixture.Ticket("AT-01")
		ended := func(gen wire.Size, disposition, to string) *snapshot.Attempt {
			return &snapshot.Attempt{AttemptID: "attempt:acme:main:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", TicketID: rec.TicketID, TicketRevision: rec.AcceptanceRevision, Generation: gen, Phase: "CANCELLED", RetryAccounting: &snapshot.RetryAccounting{Disposition: disposition}, HandoffTo: to}
		}
		live := ended("3", "NONE", "")
		live.Phase = "RUNNING"
		stale := ended("2", wire.CodeHandoff, "review")
		stale.TicketRevision = wire.CountOf(rec.AcceptanceRevision.Int() + 1)
		for name, c := range map[string]struct {
			attempts []*snapshot.Attempt
			want     string
		}{
			"none":            {nil, ""},
			"recorded":        {[]*snapshot.Attempt{ended("1", wire.CodeHandoff, "review")}, "review"},
			"untargeted":      {[]*snapshot.Attempt{ended("1", wire.CodeHandoff, "")}, ""},
			"review-returned": {[]*snapshot.Attempt{ended("1", wire.CodeReviewReturned, "")}, "implement"},
			"not-clean":       {[]*snapshot.Attempt{ended("1", "NONE", "")}, ""},
			"latest-live":     {[]*snapshot.Attempt{ended("2", wire.CodeHandoff, "review"), live}, ""},
			"stale":           {[]*snapshot.Attempt{stale}, NextStageStale},
		} {
			attempts := map[string]*snapshot.Attempt{}
			for i, a := range c.attempts {
				attempts[string(rune('a'+i))] = a
			}
			got := NextStage(attempts, rec)
			if c.want == "" && got.Kind != wire.KindNull || c.want != "" && got.Str != c.want {
				t.Fatalf("%s: got %s want %q", name, wire.Encode(got), c.want)
			}
		}
	})
}
