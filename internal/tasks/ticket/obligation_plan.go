package ticket

import (
	"sort"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// MaxObligationPlanBytes bounds one plan document read by
// `ticket obligations plan` (TOL-V0-020).
const MaxObligationPlanBytes = 8 << 20

// Plan finding kinds (TOL-V0-020).
const (
	PlanUnassigned        = "UNASSIGNED"
	PlanSplit             = "SPLIT"
	PlanUnknownObligation = "UNKNOWN_OBLIGATION"
	PlanAlreadyClosed     = "ALREADY_CLOSED"
)

// ObligationPlanTest is one planned test and the obligations it is meant to
// witness.
type ObligationPlanTest struct {
	Test        string
	Project     string
	Obligations []string
}

// ObligationPlan is a decoded taskman-obligation-plan/0 document.
type ObligationPlan struct {
	TicketID wire.TicketID
	Tests    []ObligationPlanTest
}

// DecodeObligationPlan reads the closed plan document {profile, ticketId,
// tests}: at most MaxObligationPlanTests tests, each {test, project,
// obligations} with 1..256 sorted unique ids.
func DecodeObligationPlan(raw []byte) (*ObligationPlan, error) {
	if len(raw) > MaxObligationPlanBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "--plan", "the plan exceeds %d bytes", MaxObligationPlanBytes)
	}
	v, err := wire.ParseInput(raw)
	if err != nil {
		return nil, err
	}
	// A plan from a later build refuses UNSUPPORTED_VERSION (CAL-V0-131).
	if err := wire.ProfileVersion("/profile", v, ObligationPlanProfile); err != nil {
		return nil, err
	}
	r := wire.NewReader(v, "")
	r.Closed("profile", "ticketId", "tests")
	r.Field("profile").Exact(ObligationPlanProfile)
	p := &ObligationPlan{TicketID: r.Field("ticketId").TicketID()}
	for _, it := range r.Field("tests").Array(MaxObligationPlanTests, true) {
		it.Closed("test", "project", "obligations")
		t := ObligationPlanTest{Test: it.Field("test").Prose(1, MaxObligationTestIDBytes), Project: it.Field("project").Prose(1, MaxObligationTestIDBytes)}
		readIDKeyed(it.Field("obligations"), wire.ObligationsMaxEntries, "obligations", func(id *wire.Reader) string {
			s := ReadObligationID(id)
			t.Obligations = append(t.Obligations, s)
			return s
		})
		p.Tests = append(p.Tests, t)
	}
	if err := r.Err(); err != nil {
		return nil, err
	}
	return p, nil
}

// ObligationPlanFinding is one TOL-V0-020 finding. Tests lists the planned
// "project/test" names of a SPLIT, or the planning tests of an unknown or
// closed id.
type ObligationPlanFinding struct {
	Kind  string
	ID    string
	Tests []string
}

// CheckObligationPlan checks p against l. ok is true exactly when every
// OPEN, DEFECT or BLOCKED obligation is assigned to exactly one planned test
// and no finding is reported. Findings are sorted by id, then kind.
func CheckObligationPlan(l *ObligationLedger, p *ObligationPlan) (findings []ObligationPlanFinding, ok bool) {
	planned := map[string][]string{}
	for _, t := range p.Tests {
		for _, id := range t.Obligations {
			planned[id] = append(planned[id], t.Project+"/"+t.Test)
		}
	}
	for id, tests := range planned {
		sort.Strings(tests)
		e := l.Entry(id)
		switch {
		case e == nil:
			findings = append(findings, ObligationPlanFinding{Kind: PlanUnknownObligation, ID: id, Tests: tests})
		case e.State == ObligationWitnessed || e.State == ObligationDeferred:
			findings = append(findings, ObligationPlanFinding{Kind: PlanAlreadyClosed, ID: id, Tests: tests})
		}
	}
	if l != nil {
		for _, e := range l.Entries {
			if e.State != ObligationOpen && e.State != ObligationDefect && e.State != ObligationBlocked {
				continue
			}
			switch n := len(planned[e.ID]); {
			case n == 0:
				findings = append(findings, ObligationPlanFinding{Kind: PlanUnassigned, ID: e.ID, Tests: []string{}})
			case n > 1:
				findings = append(findings, ObligationPlanFinding{Kind: PlanSplit, ID: e.ID, Tests: planned[e.ID]})
			}
		}
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].ID != findings[j].ID {
			return findings[i].ID < findings[j].ID
		}
		return findings[i].Kind < findings[j].Kind
	})
	return findings, len(findings) == 0
}
