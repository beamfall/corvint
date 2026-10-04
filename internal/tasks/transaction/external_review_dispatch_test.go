//go:build darwin || linux

package transaction

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// issue504GateView is this test's mapping of an ERG-V0-009 adapter view onto
// the dispatcher's typed gate state. The native observation that will do this
// in production is not delivered yet.
func issue504GateView(v ExternalReviewView, head wire.Digest) dispatch.GateView {
	g := dispatch.GateView{Status: v.Status, Generation: string(v.Generation), Revision: string(v.Revision), Resubmitted: v.Resubmitted, Head: string(head)}
	if v.Verdict != nil {
		g.Verdict = *v.Verdict
	}
	return g
}

// TestIssue504DispatchMisroutes replays the three misroutes from #504 through
// the native gate record, the adapter and the dispatcher's typed predicates:
// a RETURN whose prose says "No G1 PASS is claimed", a resubmission whose
// author section is titled "G1 RETURN repair dispositions", and a second
// RETURN after a resubmission. Each routes by the typed verdict alone.
func TestIssue504DispatchMisroutes(t *testing.T) {
	_, blobs, _, binding, heads := issue504Chain(t)
	lookup := func(d wire.Digest) ([]byte, bool) { b, ok := blobs[d]; return b, ok }
	c := &dispatch.Config{Roles: []dispatch.Role{
		{Name: "author", Cap: 1, Match: &dispatch.Match{Gates: []dispatch.GateMatch{{Gate: "G1", States: []string{dispatch.GateReturn}}}}},
		{Name: "reviewer", Cap: 1, Match: &dispatch.Match{Gates: []dispatch.GateMatch{{Gate: "G1", States: []string{dispatch.GateNone, dispatch.GateResubmitted}}}}},
	}, GlobalCap: 2}
	route := func(gates map[string]dispatch.GateView) []string {
		t.Helper()
		obs := &dispatch.Observation{Tickets: []dispatch.Ticket{{ID: issue504Ticket, Local: "AT-0001", Status: "OPEN", Priority: "P1", Kind: "TASK", Revision: "1", State: dispatch.StateNone, Gates: gates, GatesObserved: true}}}
		var roles []string
		for _, a := range dispatch.Roster(c, obs, nil, nil) {
			roles = append(roles, a.Role)
		}
		return roles
	}
	if got := route(nil); len(got) != 1 || got[0] != "reviewer" {
		t.Fatalf("no record routes %v", got)
	}
	want := []string{"", "author", "reviewer", "author"} // PASS, RETURN, RESUBMIT, second RETURN
	var prints []string
	for i, ref := range heads {
		views, err := ExternalReviewGates(issue504Ticket, map[string]snapshot.ExternalReviewRef{"G1": ref}, lookup, map[string]*ExternalReviewBinding{"G1": &binding})
		if err != nil {
			t.Fatal(err)
		}
		gates := map[string]dispatch.GateView{"G1": issue504GateView(views["G1"], ref.Head)}
		got := route(gates)
		if (want[i] == "" && len(got) != 0) || (want[i] != "" && (len(got) != 1 || got[0] != want[i])) {
			t.Fatalf("step %d (%+v) routes %v, want %q", i, gates["G1"], got, want[i])
		}
		obs := &dispatch.Observation{Tickets: []dispatch.Ticket{{ID: "k", Status: "OPEN", Revision: "1", State: dispatch.StateNone, Gates: gates}}}
		prints = append(prints, dispatch.Fingerprint(obs, "k"))
	}
	if prints[1] == prints[3] {
		t.Fatal("a second RETURN after a resubmission is not progress")
	}
	stale := binding
	stale.AcceptanceRevision = "2"
	views, _ := ExternalReviewGates(issue504Ticket, map[string]snapshot.ExternalReviewRef{"G1": heads[1]}, lookup, map[string]*ExternalReviewBinding{"G1": &stale})
	if got := route(map[string]dispatch.GateView{"G1": issue504GateView(views["G1"], heads[1].Head)}); len(got) != 0 {
		t.Fatalf("stale RETURN routes %v", got)
	}
	views, _ = ExternalReviewGates(issue504Ticket, map[string]snapshot.ExternalReviewRef{"G1": heads[3]}, func(wire.Digest) ([]byte, bool) { return nil, false }, map[string]*ExternalReviewBinding{"G1": &binding})
	if got := route(map[string]dispatch.GateView{"G1": issue504GateView(views["G1"], heads[3].Head)}); len(got) != 0 {
		t.Fatalf("unknown RETURN routes %v", got)
	}
}
