package transaction

import (
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-102 loop signals.
const (
	LoopNoProgress         = "NO_PROGRESS"
	LoopAlternatingReturns = "ALTERNATING_RETURNS"
)

// loopGeneration is one ended generation of the ticket's current attempt.
// A nil evidence is UNKNOWN history: it never counts toward a signal and
// ends the run it interrupts.
type loopGeneration struct {
	generation       string
	stage, handoffTo string
	evidence         *snapshot.LoopEvidence
}

// LoopHoldOf derives the CAL-V0-102 LOOP_DETECTED hold of rec from the
// audited attempt records alone. It is nil unless the policy carries
// loopDetection and the ticket's newest attempt is ended, not COMPLETED and
// bound to the current acceptance revision, so a new acceptance revision
// (including an owner reopen, CAL-V0-103) clears it. It reads nothing else
// and writes nothing.
func LoopHoldOf(attempts map[string]*snapshot.Attempt, rec *ticket.Record, policy *intent.Policy) *ticket.LoopHold {
	if policy == nil || policy.LoopDetection == nil || rec == nil {
		return nil
	}
	last := lastAttemptOf(attempts, rec.TicketID.Raw)
	if last == nil || last.Live() || last.Phase == "COMPLETED" || last.TicketRevision != rec.AcceptanceRevision {
		return nil
	}
	gens := loopGenerations(last)
	// TOL-V0-018: the generation whose WORKER witness raised the obligation
	// high water at this acceptance revision is progress.
	raised := ""
	if o := rec.ObligationsRef; o != nil && o.LastRaise != nil && o.LastRaise.Attempt == last.AttemptID && o.LastRaise.AcceptanceRevision == rec.AcceptanceRevision {
		raised = string(o.LastRaise.Generation)
	}
	if counted := noProgressRun(gens, raised); int64(len(counted)) > policy.LoopDetection.MaxNoProgressGenerations.Int() {
		return &ticket.LoopHold{Signal: LoopNoProgress, AcceptanceRevision: rec.AcceptanceRevision, Generations: counted, Limit: policy.LoopDetection.MaxNoProgressGenerations}
	}
	if counted, returns := alternatingRun(gens); int64(returns) > policy.LoopDetection.MaxAlternatingReturns.Int() {
		return &ticket.LoopHold{Signal: LoopAlternatingReturns, AcceptanceRevision: rec.AcceptanceRevision, Generations: counted, Limit: policy.LoopDetection.MaxAlternatingReturns}
	}
	return nil
}

// loopGenerations lists the attempt's ended generations oldest first: its
// prior entries, whose evidence an opted-in claim recorded, then the ended
// current generation, whose evidence is the attempt itself.
func loopGenerations(a *snapshot.Attempt) []loopGeneration {
	out := make([]loopGeneration, 0, len(a.PriorGenerations)+1)
	for _, p := range a.PriorGenerations {
		g := loopGeneration{generation: string(p.Generation)}
		if h := p.History; h != nil {
			if h.Stage != nil {
				g.stage = *h.Stage
			}
			g.handoffTo, g.evidence = h.HandoffTo, h.Loop
		}
		out = append(out, g)
	}
	return append(out, loopGeneration{generation: string(a.Generation), stage: a.Stage, handoffTo: a.HandoffTo, evidence: snapshot.LoopEvidenceOf(a)})
}

// noProgressRun returns the newest consecutive generations, oldest first,
// that each ended in a clean HANDOFF with no gate result, no external
// review and no new candidate tree: none submitted, or the same tree as the
// latest earlier one. A generation whose tree cannot be compared because
// earlier history is UNKNOWN is itself UNKNOWN. The raised generation, when
// not empty, raised the obligation high water and is progress (TOL-V0-018).
func noProgressRun(gens []loopGeneration, raised string) []string {
	verdicts := make([]bool, len(gens))
	known := true // the attempt starts at this acceptance revision with no candidate
	var tree *string
	for i, g := range gens {
		e := g.evidence
		if e == nil {
			known = false
			continue
		}
		comparable := e.CandidateTreeOid == nil || (known && tree != nil && *tree == *e.CandidateTreeOid)
		if e.CandidateTreeOid != nil {
			tree, known = e.CandidateTreeOid, true
		}
		verdicts[i] = e.Disposition == wire.CodeHandoff && e.GateResults.Int() == 0 && e.Reviews.Int() == 0 && comparable && (raised == "" || g.generation != raised)
	}
	start := len(gens)
	for start > 0 && verdicts[start-1] {
		start--
	}
	return generationIDs(gens[start:])
}

// alternatingRun returns the newest consecutive implement HANDOFF and
// review REVIEW_RETURNED generations, oldest first, and the number of
// returns among them. A trailing implement hand-off belongs to the run.
func alternatingRun(gens []loopGeneration) ([]string, int) {
	implement := func(g loopGeneration) bool {
		return g.evidence != nil && g.evidence.Disposition == wire.CodeHandoff && g.stage == "implement" && (g.handoffTo == "" || g.handoffTo == "review")
	}
	returned := func(g loopGeneration) bool {
		return g.evidence != nil && g.evidence.Disposition == wire.CodeReviewReturned && g.stage == "review"
	}
	end, i := len(gens), len(gens)-1
	if i >= 0 && implement(gens[i]) {
		i--
	}
	returns := 0
	for i >= 1 && returned(gens[i]) && implement(gens[i-1]) {
		returns++
		i -= 2
	}
	if returns == 0 {
		return nil, 0
	}
	return generationIDs(gens[i+1 : end]), returns
}

func generationIDs(gens []loopGeneration) []string {
	out := make([]string, 0, len(gens))
	for _, g := range gens {
		out = append(out, g.generation)
	}
	return out
}
