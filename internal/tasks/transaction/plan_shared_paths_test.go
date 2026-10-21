package transaction

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestNTPV0026_PlanSelectsAcrossSharedPaths: the plan selects an entry whose
// PATH scope overlaps a live reservation only on declared shared paths, and
// defers it RESOURCE_COLLISION without the declaration or on a hand-authored
// overlap (V1-2119).
func TestNTPV0026_PlanSelectsAcrossSharedPaths(t *testing.T) {
	p := func(keys ...string) []ticket.Resource {
		rs := make([]ticket.Resource, 0, len(keys))
		for _, k := range keys {
			rs = append(rs, ticket.Resource{Class: "PATH", Key: k})
		}
		return rs
	}
	generated := []string{"docs/specs/INDEX.json", "docs/specs/README.md", "docs/specs/REQUIREMENTS.tsv"}
	live := &snapshot.ReservationSet{Entries: []snapshot.ReservationEntry{{TicketID: wire.TicketID{Raw: "V1-0812"}, Resources: p(append(generated, "docs/specs/a-v0.md")...)}}}
	for _, c := range []struct {
		name   string
		policy *intent.Policy
		scope  []string
		want   string
	}{
		{"undeclared", &intent.Policy{MaxActiveAttempts: "4"}, append(generated, "docs/specs/b-v0.md"), PlanDeferred},
		{"declared", &intent.Policy{MaxActiveAttempts: "4", Reservations: &intent.ReservationPolicy{SharedPaths: generated}}, append(generated, "docs/specs/b-v0.md"), PlanSelected},
		{"hand-authored", &intent.Policy{MaxActiveAttempts: "4", Reservations: &intent.ReservationPolicy{SharedPaths: generated}}, []string{"docs/specs/a-v0.md"}, PlanDeferred},
	} {
		got := choose(PlanInput{Policy: c.policy, Reservations: live}, PlanEntry{Resources: p(c.scope...)}, nil, nil)
		if got.State != c.want || (c.want == PlanDeferred && got.Reason != wire.CodeResourceCollision) {
			t.Errorf("%s: %s %s %v", c.name, got.State, got.Reason, got.Blockers)
		}
	}
}
