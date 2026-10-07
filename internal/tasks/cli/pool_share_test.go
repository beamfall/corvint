package cli_test

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// shareRepo is exclusionCLIRepo with room for five live attempts, so the
// share bound, not queue capacity, is what refuses the fifth.
func shareRepo(t *testing.T) *fixture.Repo {
	t.Helper()
	r := exclusionCLIRepo(t)
	v := fixture.PolicyValue()
	b, _ := v.Obj.Get("budgets")
	b.Obj.Set("requireEnforcedFields", wire.Array())
	v.Obj.Set("capacity", wire.ObjectValue(wire.NewObject().Set("maxActiveAttempts", wire.String("8")).Set("maxWorkersTotal", wire.String("8")).Set("classes", wire.Array())))
	v.Obj.Set("pools", wire.Array(wire.ObjectValue(wire.NewObject().Set("id", wire.String("db")).Set("members", wire.Strings([]string{"a", "b", "review"})).Set("reservedFor", wire.ObjectValue(wire.NewObject().Set("review", wire.String("review")))))))
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(v))
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatal(x.res)
	}
	return r
}

type sharedClaim struct{ attempt, generation, allocation string }

func shareClaim(t *testing.T, root, ticket, request string, extra ...string) run {
	t.Helper()
	return atm(t, root, nil, append([]string{"claim", ticket, "--holder", "builder", "--request-id", request, "--pool", "db", "--stage", "implement"}, extra...)...)
}

func okClaim(t *testing.T, x run) sharedClaim {
	t.Helper()
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("claim %s", x.stdout)
	}
	it := x.res.Items[0]
	return sharedClaim{field(it, "attemptId").Str, field(it, "generation").Str, field(field(it, "poolAllocation"), "allocationId").Str}
}

// boundOf renders a boundAttempts array as "attempt:ROLE" items.
func boundOf(v wire.Value) string {
	out := []string{}
	for _, x := range v.Arr {
		out = append(out, field(x, "attemptId").Str+":"+field(x, "role").Str)
	}
	return strings.Join(out, ",")
}

func memberA(t *testing.T, root string) wire.Value {
	t.Helper()
	return poolStatusMembers(t, root, "--member", "a")["db/a"]
}

func shareRelease(t *testing.T, root string, c sharedClaim, request string) {
	t.Helper()
	if x := atm(t, root, nil, "release", "--attempt", c.attempt, "--generation", c.generation, "--request-id", request); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("release %s", x.stdout)
	}
}

// PSR-V0-016, PSR-V0-020: a second attempt binds to the held allocation
// without taking a member; pool status lists both bound attempts and attempt
// show names each attempt's role, as pure reads.
func TestPSRV0016_ShareBindsAndReads(t *testing.T) {
	r := shareRepo(t)
	one, two := planTicket(t, r.Root, "one", "P1", `["one"]`), planTicket(t, r.Root, "two", "P1", `["two"]`)
	a := okClaim(t, shareClaim(t, r.Root, one, "claim-a", "--exclude-member", "b", "--exclude-member", "review"))
	x := shareClaim(t, r.Root, two, "claim-b", "--share-allocation", a.allocation)
	b := okClaim(t, x)
	if b.allocation != a.allocation || field(field(x.res.Items[0], "sharedAllocation"), "sourceAttemptId").Str != a.attempt {
		t.Fatalf("share result %s", x.stdout)
	}
	// Replaying the same request returns the same binding.
	if again := okClaim(t, shareClaim(t, r.Root, two, "claim-b", "--share-allocation", a.allocation)); again != b {
		t.Fatalf("replay %v %v", again, b)
	}
	m := memberA(t, r.Root)
	if field(m, "state").Str != "ALLOCATED" || field(m, "attemptId").Str != a.attempt || boundOf(field(m, "boundAttempts")) != a.attempt+":PRIMARY,"+b.attempt+":SHARED" {
		t.Fatalf("pool status %s", wire.Encode(m))
	}
	for id, m := range poolStatusMembers(t, r.Root) {
		if id != "db/a" && (field(m, "boundAttempts").Kind != wire.KindArray || len(field(m, "boundAttempts").Arr) != 0) {
			t.Fatalf("free member binds %s", wire.Encode(m))
		}
	}
	state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	for c, role := range map[sharedClaim]string{a: "PRIMARY", b: "SHARED"} {
		show := atm(t, r.Root, nil, "attempt", "show", c.attempt)
		binding := field(show.res.Items[0], "poolBinding")
		if show.res.Outcome != wire.OutcomeOK || field(binding, "role").Str != role || field(binding, "state").Str != "ALLOCATED" || boundOf(field(binding, "boundAttempts")) != a.attempt+":PRIMARY,"+b.attempt+":SHARED" {
			t.Fatalf("attempt show %s", show.stdout)
		}
		if shared := field(show.res.Items[0], "sharedAllocation"); (role == "SHARED") != (shared.Kind == wire.KindObject) {
			t.Fatalf("sharedAllocation record %s", show.stdout)
		}
	}
	if !reflect.DeepEqual(state, fixture.TreeSnapshot(t, r.StateDir)) || !reflect.DeepEqual(intents, fixture.TreeSnapshot(t, r.IntentDir)) {
		t.Fatal("pool status or attempt show wrote state")
	}
	if x := atm(t, r.Root, nil, "claim", "--help"); !strings.Contains(field(x.res.Items[0], "usage").Str, "[--share-allocation DIGEST]") {
		t.Fatalf("claim help %s", x.stdout)
	}
}

// PSR-V0-018: whichever order the bound attempts end in, the member stays
// ALLOCATED until the last one ends and is quarantined exactly once.
func TestPSRV0018_ShareReleaseOrders(t *testing.T) {
	for _, primaryFirst := range []bool{true, false} {
		r := shareRepo(t)
		one, two := planTicket(t, r.Root, "one", "P1", `["one"]`), planTicket(t, r.Root, "two", "P1", `["two"]`)
		a := okClaim(t, shareClaim(t, r.Root, one, "claim-a", "--exclude-member", "b", "--exclude-member", "review"))
		b := okClaim(t, shareClaim(t, r.Root, two, "claim-b", "--share-allocation", a.allocation))
		first, last := a, b
		if !primaryFirst {
			first, last = b, a
		}
		shareRelease(t, r.Root, first, "release-first")
		m := memberA(t, r.Root)
		if field(m, "state").Str != "ALLOCATED" || field(m, "attemptId").Str != last.attempt || boundOf(field(m, "boundAttempts")) != last.attempt+":PRIMARY" || field(m, "quarantine").Kind != wire.KindNull {
			t.Fatalf("primaryFirst=%v after first release %s", primaryFirst, wire.Encode(m))
		}
		show := atm(t, r.Root, nil, "attempt", "show", first.attempt)
		if binding := field(show.res.Items[0], "poolBinding"); field(binding, "role").Kind != wire.KindNull || field(binding, "state").Str != "ALLOCATED" {
			t.Fatalf("ended attempt still bound %s", show.stdout)
		}
		shareRelease(t, r.Root, last, "release-last")
		m = memberA(t, r.Root)
		if field(m, "state").Str != "QUARANTINED" || field(m, "attemptId").Str != last.attempt || field(field(m, "quarantine"), "changedSeq").Str != field(m, "changedSeq").Str || len(field(m, "boundAttempts").Arr) != 0 {
			t.Fatalf("primaryFirst=%v after last release %s", primaryFirst, wire.Encode(m))
		}
		// A quarantined allocation takes no further share.
		three := planTicket(t, r.Root, "three", "P1", `["three"]`)
		if x := shareClaim(t, r.Root, three, "claim-c", "--share-allocation", a.allocation); x.res.Outcome == wire.OutcomeOK || field(x.res.Items[0], "outcome").Str != mutation.OutcomeRevisionConflict || !hasCode(x.res, wire.CodeFenced) {
			t.Fatalf("share of quarantined allocation %s", x.stdout)
		}
	}
}

// PSR-V0-017: every refusal is stable and writes no binding.
func TestPSRV0017_ShareRefusals(t *testing.T) {
	r := shareRepo(t)
	tickets := []string{}
	for _, name := range []string{"one", "two", "three", "four", "five", "six"} {
		tickets = append(tickets, planTicket(t, r.Root, name, "P1", `["`+name+`"]`))
	}
	a := okClaim(t, shareClaim(t, r.Root, tickets[0], "claim-a", "--exclude-member", "b", "--exclude-member", "review"))
	// Malformed requests refuse before any state is read.
	for i, args := range [][]string{
		{"claim", tickets[1], "--holder", "builder", "--request-id", "m0", "--share-allocation", a.allocation},
		{"claim", tickets[1], "--holder", "builder", "--request-id", "m1", "--pool", "db", "--share-allocation", a.allocation, "--exclude-member", "b"},
		{"claim", tickets[1], "--holder", "builder", "--request-id", "m2", "--pool", "db", "--share-allocation", "not-a-digest"},
		{"claim", "--next", "--holder", "builder", "--request-id", "m3", "--pool", "db", "--share-allocation", a.allocation},
		{"claim", tickets[1], "--holder", "builder", "--request-id", "m4", "--pool", "db", "--share-allocation", a.allocation, "--share-allocation", a.allocation + "0"},
		{"claim", tickets[1], "--holder", "builder", "--request-id", "m5", "--pool", "db", "--stage", "implement", "--share-allocation", ""},
	} {
		if x := atm(t, r.Root, nil, args...); x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeMalformed) {
			t.Fatalf("malformed %d accepted: %s", i, x.stdout)
		}
	}
	refusals := []struct {
		name, outcome, code string
		args                []string
	}{
		{"holder", mutation.OutcomeBlocked, wire.CodeResourceCollision, []string{"claim", tickets[1], "--holder", "other", "--request-id", "r0", "--pool", "db", "--stage", "implement", "--share-allocation", a.allocation}},
		{"stage", mutation.OutcomeValidationFailed, wire.CodeMalformed, []string{"claim", tickets[1], "--holder", "builder", "--request-id", "r1", "--pool", "db", "--stage", "integrate", "--share-allocation", a.allocation}},
		{"unknown", mutation.OutcomeRevisionConflict, wire.CodeFenced, []string{"claim", tickets[1], "--holder", "builder", "--request-id", "r2", "--pool", "db", "--stage", "implement", "--share-allocation", strings.Repeat("0", 64)}},
		{"live ticket", mutation.OutcomeBlocked, wire.CodeAttemptLive, []string{"claim", tickets[0], "--holder", "builder", "--request-id", "r3", "--pool", "db", "--stage", "implement", "--share-allocation", a.allocation}},
	}
	for _, c := range refusals {
		x := atm(t, r.Root, nil, c.args...)
		if x.res.Outcome == wire.OutcomeOK || len(x.res.Items) == 0 || field(x.res.Items[0], "outcome").Str != c.outcome || !hasCode(x.res, c.code) {
			t.Fatalf("%s refusal %s", c.name, x.stdout)
		}
	}
	if got := boundOf(field(memberA(t, r.Root), "boundAttempts")); got != a.attempt+":PRIMARY" {
		t.Fatalf("refusal bound an attempt: %s", got)
	}
	for i := 1; i < 4; i++ {
		okClaim(t, shareClaim(t, r.Root, tickets[i], "share-"+tickets[i], "--share-allocation", a.allocation))
	}
	x := shareClaim(t, r.Root, tickets[4], "share-over", "--share-allocation", a.allocation)
	if x.res.Outcome == wire.OutcomeOK || field(x.res.Items[0], "outcome").Str != mutation.OutcomeCapacityExhausted || !hasCode(x.res, wire.CodeLimitExceeded) {
		t.Fatalf("fifth bound attempt %s", x.stdout)
	}
	if n := len(field(memberA(t, r.Root), "boundAttempts").Arr); n != 4 {
		t.Fatalf("bound %d attempts", n)
	}
}
