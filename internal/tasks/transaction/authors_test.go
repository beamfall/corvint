package transaction

import (
	"bytes"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-098: an absent mode keeps historical preimage bytes, and the mode
// (not a derived member set) binds the request digest.
func TestCALV0098_PreimageBindsMode(t *testing.T) {
	q, _ := wire.ParseQueueID("", fixture.QueueID)
	base := LeaseRequest{Verb: LeaseClaim, TicketID: "ticket:acme:main:AT-001", Holder: "builder", LeaseMinutes: "60", Pool: "db", Stage: "review"}
	absent, err := leaseValue(&base, q)
	if err != nil || strings.Contains(string(wire.Encode(absent)), "excludeAuthors") {
		t.Fatalf("absent mode encoded: %s %v", wire.Encode(absent), err)
	}
	enc := map[string][]byte{}
	for _, mode := range []string{ExcludeAuthorsLatest, ExcludeAuthorsAll} {
		l := base
		l.ExcludeAuthors = mode
		v, err := leaseValue(&l, q)
		if err != nil {
			t.Fatal(err)
		}
		enc[mode] = wire.Encode(v)
		if !strings.Contains(string(enc[mode]), `"excludeAuthors":"`+mode+`"`) {
			t.Fatalf("mode not bound: %s", enc[mode])
		}
	}
	if bytes.Equal(enc[ExcludeAuthorsLatest], enc[ExcludeAuthorsAll]) || bytes.Equal(enc[ExcludeAuthorsLatest], wire.Encode(absent)) {
		t.Fatal("modes do not bind the request digest")
	}
	next := LeaseRequest{Verb: LeaseClaimNext, Holder: "builder", LeaseMinutes: "60", Pool: "db", Stage: "integrate", ExcludeAuthors: ExcludeAuthorsAll}
	if _, err := leaseValue(&next, q); err != nil {
		t.Fatalf("claim-next mode refused: %v", err)
	}
}

// CAL-V0-098: the mode requires an explicit pool and a review or integrate
// stage, is closed, and is accepted only by CLAIM, CLAIM_NEXT and the health
// preparation of such a claim, which binds it together with the ticket.
func TestCALV0098_RequestShape(t *testing.T) {
	q, _ := wire.ParseQueueID("", fixture.QueueID)
	base := LeaseRequest{Verb: LeaseClaim, TicketID: "ticket:acme:main:AT-001", Holder: "builder", LeaseMinutes: "60", Pool: "db", Stage: "review", ExcludeAuthors: ExcludeAuthorsLatest}
	for name, mutate := range map[string]func(*LeaseRequest){
		"implement": func(l *LeaseRequest) { l.Stage = "implement" },
		"no stage":  func(l *LeaseRequest) { l.Stage = "" },
		"no pool":   func(l *LeaseRequest) { l.Pool = "" },
		"bad mode":  func(l *LeaseRequest) { l.ExcludeAuthors = "latest" },
		"renew": func(l *LeaseRequest) {
			*l = LeaseRequest{Verb: LeaseRenew, AttemptID: "attempt:acme:main:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Generation: "1", LeaseMinutes: "60", ExcludeAuthors: ExcludeAuthorsLatest}
		},
	} {
		l := base
		mutate(&l)
		if _, err := leaseValue(&l, q); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	l := base
	l.Stage = "integrate"
	if _, err := leaseValue(&l, q); err != nil {
		t.Fatalf("integrate refused: %v", err)
	}
	prep := LeaseRequest{Verb: LeasePoolPrepare, Pool: "db", Member: "a", Holder: "builder", Stage: "review", Evidence: string(wire.Sum([]byte("claim"))), TicketID: base.TicketID, ExcludeAuthors: ExcludeAuthorsLatest}
	if _, err := leaseValue(&prep, q); err != nil {
		t.Fatalf("bound preparation refused: %v", err)
	}
	for name, mutate := range map[string]func(*LeaseRequest){
		"no ticket": func(l *LeaseRequest) { l.TicketID = "" },
		"no mode":   func(l *LeaseRequest) { l.ExcludeAuthors = "" },
	} {
		l := prep
		mutate(&l)
		if _, err := leaseValue(&l, q); err == nil {
			t.Fatalf("preparation with %s accepted", name)
		}
	}
	prep.TicketID, prep.ExcludeAuthors = "", ""
	if _, err := leaseValue(&prep, q); err != nil {
		t.Fatalf("unbound preparation refused: %v", err)
	}
}

func authorHistory(stage, pool, member string) *snapshot.GenerationHistory {
	h := &snapshot.GenerationHistory{}
	if stage != "" {
		h.Stage = &stage
	}
	if pool != "" {
		h.PoolID, h.MemberID = &pool, &member
	}
	return h
}

// authorAttempt is an ended external-agent attempt whose generations are
// given oldest first; the last is its current generation.
func authorAttempt(id, ticketID string, first uint64, hs ...*snapshot.GenerationHistory) *snapshot.Attempt {
	a := &snapshot.Attempt{AttemptID: id, TicketID: wire.TicketID{Raw: ticketID}, RuntimeID: snapshot.RuntimeExternalAgent}
	for i, h := range hs {
		g := wire.Size(strconv.FormatUint(first+uint64(i), 10))
		if i < len(hs)-1 {
			a.PriorGenerations = append(a.PriorGenerations, snapshot.PriorGeneration{Generation: g, History: h})
			continue
		}
		a.Generation = g
		if h != nil && h.Stage != nil {
			a.Stage = *h.Stage
		}
		if h != nil && h.PoolID != nil {
			a.PoolAllocation = &snapshot.PoolAllocation{PoolID: *h.PoolID, MemberID: *h.MemberID}
		}
	}
	return a
}

func members(x *AuthorExclusion) []string {
	out := []string{}
	for _, a := range x.Authors {
		out = append(out, a.PoolID+"/"+a.MemberID+"@"+string(a.Generation))
	}
	return out
}

// CAL-V0-098: derivation walks the ticket's generations newest first, skips
// review and integrate generations, and refuses unverifiable history.
func TestCALV0098_DeriveAuthors(t *testing.T) {
	const tk = "ticket:acme:main:AT-001"
	impl := func(m string) *snapshot.GenerationHistory { return authorHistory("implement", "db", m) }
	rev := authorHistory("review", "db", "r")
	attempts := map[string]*snapshot.Attempt{
		"x": authorAttempt("x", tk, 1, impl("a"), rev, impl("b"), rev),
		"o": authorAttempt("o", "ticket:acme:main:AT-002", 9, impl("z")),
	}
	x, why := DeriveAuthors(attempts, tk, ExcludeAuthorsLatest, "db", nil)
	if x == nil || !reflect.DeepEqual(members(x), []string{"db/b@3"}) || !reflect.DeepEqual(x.Excluded, []string{"b"}) {
		t.Fatalf("latest: %+v %q", x, why)
	}
	if d := x.AuthorsDetail(); d != "excluded implement authors: member b of pool db (generation 3)" {
		t.Fatalf("detail %q", d)
	}
	x, why = DeriveAuthors(attempts, tk, ExcludeAuthorsAll, "db", []string{"c", "a"})
	if x == nil || !reflect.DeepEqual(members(x), []string{"db/b@3", "db/a@1"}) || !reflect.DeepEqual(x.Excluded, []string{"a", "b", "c"}) {
		t.Fatalf("all with explicit union: %+v %q", x, why)
	}
	x, _ = DeriveAuthors(attempts, tk, ExcludeAuthorsLatest, "other", nil)
	if x == nil || x.Excluded != nil || len(x.Authors) != 1 {
		t.Fatalf("foreign-pool author excluded: %+v", x)
	}
	for name, c := range map[string]struct {
		attempt *snapshot.Attempt
		mode    string
		why     string
	}{
		"not observed newest":          {authorAttempt("x", tk, 1, impl("a"), nil, rev), ExcludeAuthorsLatest, "NOT_OBSERVED"},
		"not observed older under all": {authorAttempt("x", tk, 1, nil, impl("a")), ExcludeAuthorsAll, "NOT_OBSERVED"},
		"null stage":                   {authorAttempt("x", tk, 1, impl("a"), authorHistory("", "db", "a")), ExcludeAuthorsLatest, "recorded no stage"},
		"implement without member":     {authorAttempt("x", tk, 1, authorHistory("implement", "", "")), ExcludeAuthorsLatest, "held no pool member"},
		"no implement":                 {authorAttempt("x", tk, 1, rev), ExcludeAuthorsLatest, "no implement generation"},
	} {
		x, why := DeriveAuthors(map[string]*snapshot.Attempt{"x": c.attempt}, tk, c.mode, "db", nil)
		if x != nil || !strings.Contains(why, c.why) {
			t.Fatalf("%s: %+v %q", name, x, why)
		}
	}
	// LATEST stops at the newest implement generation, so older unverifiable
	// history does not refuse it; ALL reads it and refuses.
	older := map[string]*snapshot.Attempt{"x": authorAttempt("x", tk, 1, nil, impl("a"))}
	if x, _ := DeriveAuthors(older, tk, ExcludeAuthorsAll, "db", nil); x != nil {
		t.Fatal("all ignored older NOT_OBSERVED history")
	}
	if x, why := DeriveAuthors(older, tk, ExcludeAuthorsLatest, "db", nil); x == nil {
		t.Fatalf("latest read older history: %q", why)
	}
	// A supervised attempt's current generation is NOT_OBSERVED.
	sup := authorAttempt("x", tk, 1, impl("a"))
	sup.RuntimeID = "supervisor"
	if x, why := DeriveAuthors(map[string]*snapshot.Attempt{"x": sup}, tk, ExcludeAuthorsLatest, "db", nil); x != nil || !strings.Contains(why, "NOT_OBSERVED") {
		t.Fatalf("supervised generation guessed: %+v %q", x, why)
	}
}
