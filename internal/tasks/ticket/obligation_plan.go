package ticket

import (
	"slices"
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
	// PlanRerunNotWitnessed names an id a test lists under rerun that the
	// ledger does not hold as WITNESSED (TOL-V0-056).
	PlanRerunNotWitnessed = "RERUN_NOT_WITNESSED"
)

// ObligationPlanTest is one planned test, the obligations it is meant to
// witness and the optional WITNESSED obligations it re-runs as regression
// checks (TOL-V0-056); Rerun is nil when the member is absent.
type ObligationPlanTest struct {
	Test        string
	Project     string
	Obligations []string
	Rerun       []string
}

// ObligationPlan is a decoded taskman-obligation-plan/0 document.
type ObligationPlan struct {
	TicketID wire.TicketID
	Tests    []ObligationPlanTest
}

// DecodeObligationPlan reads the closed plan document {profile, ticketId,
// tests}: at most MaxObligationPlanTests tests, each {test, project,
// obligations, rerun?} with 1..256 sorted unique obligation ids, or 0..256
// when the optional rerun member lists 1..256 sorted unique ids
// (TOL-V0-056). One test may not name an id in both lists.
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
		it.Closed(wire.OptionalKeys(it.Value(), []string{"test", "project", "obligations"}, "rerun")...)
		t := ObligationPlanTest{Test: it.Field("test").Prose(1, MaxObligationTestIDBytes), Project: it.Field("project").Prose(1, MaxObligationTestIDBytes)}
		readID := func(list *[]string) func(*wire.Reader) string {
			return func(id *wire.Reader) string {
				s := ReadObligationID(id)
				*list = append(*list, s)
				return s
			}
		}
		if wire.Has(it.Value(), "rerun") {
			t.Rerun = []string{}
			readIDKeyed(it.Field("rerun"), wire.ObligationsMaxEntries, "rerun", readID(&t.Rerun))
			if obs := it.Field("obligations"); it.Err() == nil && !isEmptyArray(obs) {
				readIDKeyed(obs, wire.ObligationsMaxEntries, "obligations", readID(&t.Obligations))
			}
			for _, id := range t.Obligations {
				if it.Err() == nil && slices.Contains(t.Rerun, id) {
					it.Fail(wire.CodeMalformed, "obligation %s is listed under both obligations and rerun", id)
				}
			}
		} else {
			readIDKeyed(it.Field("obligations"), wire.ObligationsMaxEntries, "obligations", readID(&t.Obligations))
		}
		p.Tests = append(p.Tests, t)
	}
	if err := r.Err(); err != nil {
		return nil, err
	}
	return p, nil
}

// isEmptyArray reports whether r holds an empty array.
func isEmptyArray(r *wire.Reader) bool {
	v := r.Value()
	return v.Kind == wire.KindArray && len(v.Arr) == 0
}

// ObligationPlanFinding is one TOL-V0-020 finding. Tests lists the planned
// "project/test" names of a SPLIT, or the planning tests of an unknown or
// closed id.
type ObligationPlanFinding struct {
	Kind  string
	ID    string
	Tests []string
}

// CheckObligationPlan checks p against l (TOL-V0-020 as amended by
// TOL-V0-056/057). ok is true exactly when no finding is reported: every
// core OPEN or PARTIAL obligation is assigned to exactly one planned test,
// no listed pending obligation is split, no id is unknown or closed and
// every rerun id is WITNESSED. Non-core, DEFECT and BLOCKED obligations need
// no plan entry, and are checked for SPLIT only when listed. Findings are
// sorted by id, then kind.
func CheckObligationPlan(l *ObligationLedger, p *ObligationPlan) (findings []ObligationPlanFinding, ok bool) {
	planned, reruns := planListings(p)
	for id := range unionKeys(planned, reruns) {
		if l.Entry(id) == nil {
			tests := append(append([]string{}, planned[id]...), reruns[id]...)
			sort.Strings(tests)
			findings = append(findings, ObligationPlanFinding{Kind: PlanUnknownObligation, ID: id, Tests: tests})
		}
	}
	for id, tests := range planned {
		if e := l.Entry(id); e != nil && (e.State == ObligationWitnessed || e.State == ObligationDeferred) {
			findings = append(findings, ObligationPlanFinding{Kind: PlanAlreadyClosed, ID: id, Tests: tests})
		}
	}
	for id, tests := range reruns {
		if e := l.Entry(id); e != nil && e.State != ObligationWitnessed {
			findings = append(findings, ObligationPlanFinding{Kind: PlanRerunNotWitnessed, ID: id, Tests: tests})
		}
	}
	if l != nil {
		for _, e := range l.Entries {
			if !ObligationPending(e.State) {
				continue
			}
			required := e.Core && (e.State == ObligationOpen || e.State == ObligationPartial)
			switch n := len(planned[e.ID]); {
			case n == 0 && required:
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

// ObligationPlanReruns lists, sorted by id, the WITNESSED obligations p
// re-runs as regression checks, each with its planning tests (TOL-V0-056).
// It is empty when no test carries rerun.
func ObligationPlanReruns(l *ObligationLedger, p *ObligationPlan) []ObligationPlanFinding {
	_, reruns := planListings(p)
	var out []ObligationPlanFinding
	for id, tests := range reruns {
		if e := l.Entry(id); e != nil && e.State == ObligationWitnessed {
			out = append(out, ObligationPlanFinding{Kind: "RERUN", ID: id, Tests: tests})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// PlanHasRerun reports whether any test of p carries the rerun member.
func PlanHasRerun(p *ObligationPlan) bool {
	for _, t := range p.Tests {
		if t.Rerun != nil {
			return true
		}
	}
	return false
}

// planListings maps each id to its sorted "project/test" names, separately
// for the obligations and rerun lists.
func planListings(p *ObligationPlan) (planned, reruns map[string][]string) {
	planned, reruns = map[string][]string{}, map[string][]string{}
	for _, t := range p.Tests {
		for _, id := range t.Obligations {
			planned[id] = append(planned[id], t.Project+"/"+t.Test)
		}
		for _, id := range t.Rerun {
			reruns[id] = append(reruns[id], t.Project+"/"+t.Test)
		}
	}
	for _, m := range []map[string][]string{planned, reruns} {
		for _, tests := range m {
			sort.Strings(tests)
		}
	}
	return planned, reruns
}

func unionKeys(a, b map[string][]string) map[string]bool {
	out := map[string]bool{}
	for k := range a {
		out[k] = true
	}
	for k := range b {
		out[k] = true
	}
	return out
}
