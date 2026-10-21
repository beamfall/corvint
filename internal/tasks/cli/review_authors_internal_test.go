package cli

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// reviewGen is one recorded generation; an empty stage records none and an
// empty pool records no member.
func reviewGen(stage, pool, member string) *snapshot.GenerationHistory {
	h := &snapshot.GenerationHistory{}
	if stage != "" {
		h.Stage = &stage
	}
	if pool != "" {
		h.PoolID, h.MemberID = &pool, &member
	}
	return h
}

// reviewAttempt is an ended external-agent attempt whose generations are
// given oldest first from generation 1.
func reviewAttempt(id, ticketID string, hs ...*snapshot.GenerationHistory) *snapshot.Attempt {
	a := &snapshot.Attempt{AttemptID: id, TicketID: wire.TicketID{Raw: ticketID}, RuntimeID: snapshot.RuntimeExternalAgent}
	for i, h := range hs {
		g := wire.Size(strconv.Itoa(i + 1))
		if i < len(hs)-1 {
			a.PriorGenerations = append(a.PriorGenerations, snapshot.PriorGeneration{Generation: g, History: h})
			continue
		}
		a.Generation = g
		if h.Stage != nil {
			a.Stage = *h.Stage
		}
		if h.PoolID != nil {
			a.PoolAllocation = &snapshot.PoolAllocation{PoolID: *h.PoolID, MemberID: *h.MemberID}
		}
	}
	return a
}

// TestTOLV0075_ReviewAuthorSet pins the reviewed-witness author set
// (TOL-V0-074/075, CAL-V0-107, V1-1924): every implement member of the
// ticket under mode ALL across pools, unioned with the explicit exclusions
// by member label, so an alias equal to a derived member or one member in
// two pools appears once; review and integrate members and other tickets'
// authors are not authors; an unrecorded implement generation refuses
// INDEPENDENCE_UNVERIFIED unless an explicit exclusion covers it.
func TestTOLV0075_ReviewAuthorSet(t *testing.T) {
	const tk = "ticket:acme:main:AT-001"
	attempts := map[string]*snapshot.Attempt{
		"x": reviewAttempt("x", tk, reviewGen("implement", "db", "a"), reviewGen("review", "db", "r"), reviewGen("implement", "gpu", "a"), reviewGen("integrate", "db", "v")),
		"y": reviewAttempt("y", tk, reviewGen("implement", "db", "b")),
		"o": reviewAttempt("o", "ticket:acme:main:AT-002", reviewGen("implement", "db", "z")),
	}
	for name, c := range map[string]struct {
		explicit []string
		want     []string
	}{
		"derived only":            {nil, []string{"a", "b"}},
		"alias equals derived":    {[]string{"b", "a"}, []string{"a", "b"}},
		"explicit adds an author": {[]string{"c", "a"}, []string{"a", "b", "c"}},
	} {
		got, err := reviewAuthorSet(attempts, tk, c.explicit)
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Fatalf("%s: %v %v; want %v", name, got, err, c.want)
		}
	}
	if got, err := reviewAuthorSet(attempts, "ticket:acme:main:AT-003", nil); err != nil || len(got) != 0 {
		t.Fatalf("ticket with no generations: %v %v", got, err)
	}

	unrecorded := map[string]*snapshot.Attempt{"x": reviewAttempt("x", tk, reviewGen("implement", "", ""), reviewGen("implement", "db", "a"))}
	_, err := reviewAuthorSet(unrecorded, tk, nil)
	if err == nil || wire.CodeOf(err) != wire.CodeIndependenceUnverified || !strings.Contains(err.Error(), "--exclude-author") {
		t.Fatalf("unrecorded implement member: %v", err)
	}
	if got, err := reviewAuthorSet(unrecorded, tk, []string{"e"}); err != nil || !reflect.DeepEqual(got, []string{"a", "e"}) {
		t.Fatalf("explicit cover: %v %v", got, err)
	}
}
