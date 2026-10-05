package transaction

import (
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// loopStep is one ended generation: its stage, disposition, recorded target,
// candidate tree ("" none), gate result and review counts. unknown marks a
// legacy prior entry that recorded no history.
type loopStep struct {
	stage, disposition, to, tree string
	gates, reviews               int64
	unknown                      bool
}

func handoff(stage, tree string) loopStep {
	return loopStep{stage: stage, disposition: wire.CodeHandoff, tree: tree}
}
func returned(tree string) loopStep {
	return loopStep{stage: "review", disposition: wire.CodeReviewReturned, tree: tree}
}

// loopAttempt is the ended attempt whose generations are steps, oldest
// first: every step but the last is a prior generation an opted-in claim
// recorded.
func loopAttempt(rec *ticket.Record, steps ...loopStep) *snapshot.Attempt {
	tree := func(s string) *string {
		if s == "" {
			return nil
		}
		t := strings.Repeat(s, 40)
		return &t
	}
	a := &snapshot.Attempt{AttemptID: "attempt:acme:main:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", TicketID: rec.TicketID, TicketRevision: rec.AcceptanceRevision, RuntimeID: snapshot.RuntimeExternalAgent, Phase: "CANCELLED"}
	for i, s := range steps {
		gen := wire.Size(wire.CountOf(int64(i + 1)))
		if i == len(steps)-1 {
			a.Generation, a.Stage, a.HandoffTo, a.CandidateTreeOid = gen, s.stage, s.to, tree(s.tree)
			a.RetryAccounting = &snapshot.RetryAccounting{Disposition: s.disposition}
			for g := int64(0); g < s.gates; g++ {
				a.GateResults = append(a.GateResults, "g")
			}
			for r := int64(0); r < s.reviews; r++ {
				a.Reviews = append(a.Reviews, "r")
			}
			break
		}
		p := snapshot.PriorGeneration{Generation: gen, Quiescence: "FENCED", ProvedSeq: "1"}
		if !s.unknown {
			stage := s.stage
			p.History = &snapshot.GenerationHistory{Stage: &stage, HandoffTo: s.to, Loop: &snapshot.LoopEvidence{Disposition: s.disposition, CandidateTreeOid: tree(s.tree), GateResults: wire.CountOf(s.gates), Reviews: wire.CountOf(s.reviews)}}
		}
		a.PriorGenerations = append(a.PriorGenerations, p)
	}
	return a
}

func loopPolicy(noProgress, returns string) *intent.Policy {
	return &intent.Policy{LoopDetection: &intent.LoopDetection{MaxNoProgressGenerations: wire.Count(noProgress), MaxAlternatingReturns: wire.Count(returns)}}
}

// TestCALV0102_LoopHoldSignals proves the derivation: more consecutive
// no-progress hand-offs, or more alternating implement/REVIEW_RETURNED pairs,
// than the policy bound hold the ticket with the counted generations as
// evidence; UNKNOWN history never counts; a changed tree, a gate result or a
// review is progress; and no policy, a live, completed or stale attempt
// derive nothing.
func TestCALV0102_LoopHoldSignals(t *testing.T) {
	t.Run("CAL-V0-102 LoopHoldSignals", func(t *testing.T) {
		rec := issue502Asking(t, "AT-01", 0)
		policy := loopPolicy("2", "2")
		impl := func(tree string) loopStep { return handoff("implement", tree) }
		cases := []struct {
			name   string
			steps  []loopStep
			signal string
			gens   string
		}{
			{"two below bound", []loopStep{impl(""), impl("")}, "", ""},
			{"three no-tree hand-offs", []loopStep{impl(""), impl(""), impl("")}, LoopNoProgress, "1,2,3"},
			{"run after progress", []loopStep{impl("a"), impl("b"), impl(""), impl(""), impl("")}, LoopNoProgress, "3,4,5"},
			{"unchanged tree", []loopStep{impl("a"), impl("a"), impl("a"), impl("a")}, LoopNoProgress, "2,3,4"},
			{"changed tree is progress", []loopStep{impl("a"), impl("b"), impl("c"), impl("d")}, "", ""},
			{"unknown breaks the run", []loopStep{impl(""), {unknown: true}, impl(""), impl("")}, "", ""},
			{"unknown makes a tree incomparable", []loopStep{impl("a"), {unknown: true}, impl("a"), impl("a"), impl("a"), impl("a")}, LoopNoProgress, "4,5,6"},
			{"gate result is progress", []loopStep{impl(""), {stage: "implement", disposition: wire.CodeHandoff, gates: 1}, impl(""), impl("")}, "", ""},
			{"review is progress", []loopStep{impl(""), impl(""), {stage: "implement", disposition: wire.CodeHandoff, reviews: 1}, impl("")}, "", ""},
			{"charged end is not a hand-off", []loopStep{impl(""), impl(""), {stage: "implement", disposition: "NONE"}}, "", ""},
			{"two returns", []loopStep{impl("a"), returned("a"), impl("b"), returned("b")}, "", ""},
			{"three returns", []loopStep{impl("a"), returned("a"), impl("b"), returned("b"), impl("c"), returned("c")}, LoopAlternatingReturns, "1,2,3,4,5,6"},
			{"three returns and a pending hand-off", []loopStep{impl("z"), impl("a"), returned("a"), impl("b"), returned("b"), impl("c"), returned("c"), impl("d")}, LoopAlternatingReturns, "2,3,4,5,6,7,8"},
			{"unknown breaks alternation", []loopStep{impl("a"), returned("a"), {unknown: true}, returned("b"), impl("c"), returned("c")}, "", ""},
			{"integrate target is not alternation", []loopStep{impl("a"), returned("a"), {stage: "implement", disposition: wire.CodeHandoff, to: "integrate", tree: "b"}, returned("b"), impl("c"), returned("c")}, "", ""},
		}
		for _, c := range cases {
			got := LoopHoldOf(map[string]*snapshot.Attempt{"a": loopAttempt(rec, c.steps...)}, rec, policy)
			if c.signal == "" {
				if got != nil {
					t.Fatalf("%s: held %+v", c.name, got)
				}
				continue
			}
			if got == nil || got.Signal != c.signal || strings.Join(got.Generations, ",") != c.gens || got.AcceptanceRevision != rec.AcceptanceRevision {
				t.Fatalf("%s: got %+v want %s %s", c.name, got, c.signal, c.gens)
			}
		}

		looping := loopAttempt(rec, impl(""), impl(""), impl(""))
		attempts := map[string]*snapshot.Attempt{"a": looping}
		if LoopHoldOf(attempts, rec, nil) != nil || LoopHoldOf(attempts, rec, &intent.Policy{}) != nil || LoopHoldOf(attempts, nil, policy) != nil || LoopHoldOf(nil, rec, policy) != nil {
			t.Fatal("absent policy, record or history derived a hold")
		}
		if LoopHoldOf(attempts, rec, loopPolicy("3", "2")) != nil {
			t.Fatal("a run equal to the bound held")
		}
		for name, change := range map[string]func(a *snapshot.Attempt){
			"live":      func(a *snapshot.Attempt) { a.Phase = "RUNNING" },
			"completed": func(a *snapshot.Attempt) { a.Phase = "COMPLETED" },
			"stale":     func(a *snapshot.Attempt) { a.TicketRevision = "1" },
		} {
			a := loopAttempt(rec, impl(""), impl(""), impl(""))
			change(a)
			if got := LoopHoldOf(map[string]*snapshot.Attempt{"a": a}, rec, policy); got != nil {
				t.Fatalf("%s attempt held %+v", name, got)
			}
		}
		older := loopAttempt(rec, impl(""), impl(""), impl(""))
		older.AttemptID, older.Generation = "attempt:acme:main:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "9"
		older.TicketRevision = "1"
		if LoopHoldOf(map[string]*snapshot.Attempt{"a": looping, "b": older}, rec, policy) != nil {
			t.Fatal("the newest attempt (a new acceptance revision) must clear the hold")
		}
	})
}

// TestCALV0102_ClaimHonoursLoopHold proves the hold on every native
// admission path: eligibility, direct claim, claim-next, recorded
// claimability and the plan refuse LOOP_DETECTED with the counted
// generations, ticket show's View names reopen, and the derivation wrote
// nothing. Without the policy the same history admits.
func TestCALV0102_ClaimHonoursLoopHold(t *testing.T) {
	t.Run("CAL-V0-102 ClaimHonoursLoopHold", func(t *testing.T) {
		rec := issue502Asking(t, "AT-01", 0)
		id := rec.TicketID.Raw
		looping := loopAttempt(rec, handoff("implement", ""), handoff("implement", ""), handoff("implement", ""))
		want := "no-progress loop NO_PROGRESS at acceptanceRevision 2: generations 1,2,3 exceed the policy bound 2"
		held := func(verb, ticketID string) leaseContext {
			c := issue502ClaimContext(t, verb, ticketID, rec)
			c.st.policy.LoopDetection = &intent.LoopDetection{MaxNoProgressGenerations: "2", MaxAlternatingReturns: "2"}
			c.st.attempts["a"] = looping
			return c
		}
		c := held(LeaseClaim, id)
		if out := c.eligibility(id); out == nil {
			t.Fatal("eligibility admitted a held ticket")
		} else if code, detail := issue502Refused(*out); code != wire.CodeLoopDetected || detail != want {
			t.Fatalf("eligibility %s %q", code, detail)
		}
		if code, detail := issue502Refused(planClaim(c)); code != wire.CodeLoopDetected || detail != want {
			t.Fatalf("claim %s %q", code, detail)
		}
		code, detail := issue502Refused(planClaimNext(held(LeaseClaimNext, "")))
		if code != wire.CodeLoopDetected || !strings.HasSuffix(detail, "is BLOCKED LOOP_DETECTED") {
			t.Fatalf("claim-next %s %q", code, detail)
		}
		in := PlanInput{Queue: c.st.queue, Policy: c.st.policy, Tickets: c.st.tickets, Reservations: c.st.reservations, Attempts: c.st.attempts}
		if got, reason := RecordedClaimability(in, rec); got.Kind != wire.KindBool || got.Bool || reason != wire.CodeLoopDetected {
			t.Fatalf("claimability %s %s", wire.Encode(got), reason)
		}
		if e := PriorityFirst(in).Entries[0]; e.State != PlanBlocked || e.Reason != wire.CodeLoopDetected {
			t.Fatalf("plan %s %s", e.State, e.Reason)
		}
		v, _ := c.st.tickets.View(id, ticket.Context{CanonicalWriter: c.st.queue.CanonicalWriter, Loop: LoopHoldOf(c.st.attempts, rec, c.st.policy)})
		if v.Eligibility != "BLOCKED" || v.NextAction != "reopen" || !slices.ContainsFunc(v.Blockers, func(b ticket.Blocker) bool { return b.Code == wire.CodeLoopDetected && b.Detail == want }) {
			t.Fatalf("view %+v", v)
		}
		if stale, _ := c.st.tickets.View(id, ticket.Context{CanonicalWriter: c.st.queue.CanonicalWriter, Loop: &ticket.LoopHold{Signal: LoopNoProgress, AcceptanceRevision: "1", Generations: []string{"1"}, Limit: "0"}}); slices.ContainsFunc(stale.Blockers, func(b ticket.Blocker) bool { return b.Code == wire.CodeLoopDetected }) {
			t.Fatal("a hold from another acceptance revision blocked")
		}
		if len(c.st.reservations.Entries) != 0 || rec.Status != ticket.StatusOpen || len(rec.Holds) != 0 || looping.Phase != "CANCELLED" {
			t.Fatal("the derived hold wrote state")
		}

		// D8: the same history under a policy without loopDetection admits
		// and plans exactly as before.
		plain := issue502ClaimContext(t, LeaseClaim, id, rec)
		plain.st.attempts["a"] = looping
		if out := plain.eligibility(id); out != nil {
			t.Fatalf("policy-absent eligibility refused %+v", out.result)
		}
		pin := PlanInput{Queue: plain.st.queue, Policy: plain.st.policy, Tickets: plain.st.tickets, Reservations: plain.st.reservations, Attempts: plain.st.attempts}
		if e := PriorityFirst(pin).Entries[0]; e.Reason == wire.CodeLoopDetected {
			t.Fatalf("policy-absent plan %s %s", e.State, e.Reason)
		}
	})
}

// The N-1 scenario exercises the opt-in: the same history under
// loopDetection records loopEvidence and holds the last claim.
func TestCALV0102_NMinusOneScenarioOptsIn(t *testing.T) {
	t.Run("CAL-V0-102 NMinusOneScenarioOptsIn", func(t *testing.T) {
		got := loopN1Transcript(t, func(f *priorityFixture) {
			p := *f.st.policy
			p.LoopDetection = &intent.LoopDetection{MaxNoProgressGenerations: "2", MaxAlternatingReturns: "2"}
			f.st.policy = &p
		})
		if !strings.Contains(got, `"loopEvidence":{`) || !strings.Contains(got, wire.CodeLoopDetected) {
			t.Fatalf("opted-in scenario did not record evidence and hold:\n%s", got)
		}
		if got == loopN1Transcript(t, nil) {
			t.Fatal("opt-in changed nothing")
		}
	})
}
