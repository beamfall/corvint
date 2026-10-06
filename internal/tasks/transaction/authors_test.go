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
	v, err := leaseValue(&prep, q)
	if err != nil {
		t.Fatalf("unbound preparation refused: %v", err)
	}
	// Exact pre-change POOL_PREPARE preimage and digest, captured at base
	// 16d3a7d9: an unbound preparation must not gain an excludeAuthors member.
	want := `{"allocation":"","attemptId":null,"base":null,"branch":null,"evidence":"dd1b3c312cf7d816130354452e9629ce39355b0c534129dd26a08cd9a4502ede","generation":null,"holder":"builder","leaseMinutes":null,"member":"a","pool":"db","reason":null,"scope":null,"stage":"review","ticketId":null,"verb":"POOL_PREPARE","wholeRepository":false}`
	got := wire.Encode(v)
	if string(got) != want {
		t.Fatalf("legacy preparation preimage changed: %s", got)
	}
	if d := wire.Sum(got); d != "b2a21d07dd4714be4c88a5e047c6bfcc16e7cd6ef5b2fc1297f0fc7b6e47f9e0" {
		t.Fatalf("legacy preparation digest changed: %s", d)
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

// CAL-V0-104: explicit exclusions cover generations that record no pool
// member, and say so; without them the CAL-V0-098 refusal is unchanged; a
// ticket with no implement generation has nothing to exclude.
func TestCALV0104_ExplicitMembersCoverUnrecordedGenerations(t *testing.T) {
	const tk = "ticket:acme:main:AT-001"
	impl := func(m string) *snapshot.GenerationHistory { return authorHistory("implement", "db", m) }
	rev := authorHistory("review", "db", "r")
	noMember := authorHistory("implement", "", "")
	stageless := authorHistory("", "", "")
	for name, c := range map[string]struct {
		attempts map[string]*snapshot.Attempt
		mode     string
		why      string   // refusal without explicit members
		authors  []string // with explicit members
		covered  []string
		excluded []string
	}{
		"legacy newest, recorded older under all": {
			attempts: map[string]*snapshot.Attempt{"x": authorAttempt("x", tk, 1, impl("a"), nil, rev)},
			mode:     ExcludeAuthorsAll, why: "NOT_OBSERVED",
			authors: []string{"db/a@1"}, covered: []string{"generation 2 of x"}, excluded: []string{"a", "e"},
		},
		"legacy newest under latest walks to the recorded author": {
			attempts: map[string]*snapshot.Attempt{"x": authorAttempt("x", tk, 1, impl("a"), nil)},
			mode:     ExcludeAuthorsLatest, why: "recorded no stage",
			authors: []string{"db/a@1"}, covered: []string{"generation 2 of x"}, excluded: []string{"a", "e"},
		},
		"recorded newest, legacy older under all": {
			attempts: map[string]*snapshot.Attempt{"x": authorAttempt("x", tk, 1, nil, impl("b"))},
			mode:     ExcludeAuthorsAll, why: "NOT_OBSERVED",
			authors: []string{"db/b@2"}, covered: []string{"generation 1 of x"}, excluded: []string{"b", "e"},
		},
		"only legacy": {
			attempts: map[string]*snapshot.Attempt{"x": authorAttempt("x", tk, 1, nil, rev)},
			mode:     ExcludeAuthorsLatest, why: "NOT_OBSERVED",
			authors: []string{}, covered: []string{"generation 1 of x"}, excluded: []string{"e"},
		},
		"implement without member and stage-less without member": {
			attempts: map[string]*snapshot.Attempt{"x": authorAttempt("x", tk, 1, stageless, noMember, rev)},
			mode:     ExcludeAuthorsAll, why: "held no pool member",
			authors: []string{}, covered: []string{"generation 2 of x", "generation 1 of x"}, excluded: []string{"e"},
		},
	} {
		if x, why := DeriveAuthors(c.attempts, tk, c.mode, "db", nil); x != nil || !strings.Contains(why, c.why) {
			t.Fatalf("%s without explicit members: %+v %q", name, x, why)
		}
		x, why := DeriveAuthors(c.attempts, tk, c.mode, "db", []string{"e"})
		if x == nil || !reflect.DeepEqual(members(x), c.authors) || !reflect.DeepEqual(x.Covered, c.covered) || !reflect.DeepEqual(x.Excluded, c.excluded) {
			t.Fatalf("%s with explicit members: %+v %q", name, x, why)
		}
		notes := strings.Join(x.Notes(), "\n")
		if !strings.Contains(notes, "covered by the caller's explicit --exclude-member set, not by recorded members") {
			t.Fatalf("%s: cover not reported: %q", name, notes)
		}
	}
	// A stage-less generation that recorded a member is never covered.
	held := map[string]*snapshot.Attempt{"x": authorAttempt("x", tk, 1, impl("a"), authorHistory("", "db", "a"))}
	if x, why := DeriveAuthors(held, tk, ExcludeAuthorsLatest, "db", []string{"e"}); x != nil || !strings.Contains(why, "recorded no stage") {
		t.Fatalf("recorded stage-less member covered: %+v %q", x, why)
	}
	// A fully recorded derivation has no caveat.
	full := map[string]*snapshot.Attempt{"x": authorAttempt("x", tk, 1, impl("a"), rev)}
	if x, _ := DeriveAuthors(full, tk, ExcludeAuthorsAll, "db", []string{"e"}); x == nil || x.Covered != nil || x.Notes() != nil {
		t.Fatalf("recorded derivation reported a caveat: %+v", x)
	}
	// No implement generation (only review, or none): nothing to exclude,
	// with or without explicit members, and the result says so.
	for name, attempts := range map[string]map[string]*snapshot.Attempt{
		"review only":   {"x": authorAttempt("x", tk, 1, rev)},
		"never claimed": {"o": authorAttempt("o", "ticket:acme:main:AT-002", 1, impl("z"))},
	} {
		for _, explicit := range [][]string{nil, {"e"}} {
			x, why := DeriveAuthors(attempts, tk, ExcludeAuthorsLatest, "db", explicit)
			if x == nil || len(x.Authors) != 0 || x.Covered != nil || !reflect.DeepEqual(x.Excluded, []string(explicit)) {
				t.Fatalf("%s %v: %+v %q", name, explicit, x, why)
			}
			if n := x.Notes(); len(n) != 1 || !strings.Contains(n[0], "no implement generation of "+tk+" records an author") {
				t.Fatalf("%s: notes %q", name, n)
			}
		}
	}
	// The cover is the caller's explicit set, never implied: without it an
	// unrecorded generation refuses, with it the recorded author still counts.
	legacy := map[string]*snapshot.Attempt{"x": authorAttempt("x", tk, 1, impl("a"), nil)}
	if x, why := DeriveAuthors(legacy, tk, ExcludeAuthorsAll, "db", nil); x != nil || why == "" {
		t.Fatalf("implicit cover: %+v %q", x, why)
	}
	if x, why := DeriveAuthors(legacy, tk, ExcludeAuthorsAll, "db", []string{"e"}); x == nil || !reflect.DeepEqual(x.Excluded, []string{"a", "e"}) {
		t.Fatalf("explicit cover: %+v %q", x, why)
	}
}
