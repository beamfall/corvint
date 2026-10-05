package cli_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-098: both parsers accept the bare flag and =all once, for a pooled
// review or integrate claim; preview renders each ticket's derivation only
// when asked, and stays a pure read; claim binds the mode to its request.
func TestCALV0098_CLIExcludeAuthors(t *testing.T) {
	r := exclusionCLIRepo(t)
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatal(x.res)
	}
	authored := planTicket(t, r.Root, "authored", "P1", `["authored"]`)
	planTicket(t, r.Root, "fresh", "P2", `["fresh"]`)
	for _, args := range [][]string{
		{"--exclude-authors", "--stage", "review"},
		{"--pool", "db", "--exclude-authors"},
		{"--pool", "db", "--stage", "implement", "--exclude-authors"},
		{"--pool", "db", "--stage", "review", "--exclude-authors=latest"},
		{"--pool", "db", "--stage", "review", "--exclude-authors=", "--exclude-authors"},
		{"--pool", "db", "--stage", "review", "--exclude-authors", "--exclude-authors=all"},
	} {
		if x := atm(t, r.Root, nil, append([]string{"plan", "preview"}, args...)...); x.res.Outcome == wire.OutcomeOK {
			t.Fatalf("malformed preview accepted: %v", args)
		}
		claim := append([]string{"claim", authored, "--holder", "reviewer", "--request-id", "bad"}, args...)
		if x := atm(t, r.Root, nil, claim...); x.res.Outcome == wire.OutcomeOK {
			t.Fatalf("malformed claim accepted: %v", args)
		}
	}
	plain := atm(t, r.Root, nil, "plan", "preview", "--pool", "db", "--stage", "review")
	for _, en := range field(plain.res.Items[0], "entries").Arr {
		if _, ok := en.Obj.Get("detail"); ok {
			t.Fatalf("flagless preview grew keys: %s", wire.Encode(en))
		}
		if _, ok := en.Obj.Get("excludedAuthors"); ok {
			t.Fatalf("flagless preview grew keys: %s", wire.Encode(en))
		}
	}

	impl := atm(t, r.Root, nil, "claim", authored, "--holder", "builder", "--request-id", "implement", "--pool", "db", "--stage", "implement", "--exclude-member", "a", "--exclude-member", "review")
	item := impl.res.Items[0]
	if impl.res.Outcome != wire.OutcomeOK || field(field(item, "poolAllocation"), "memberId").Str != "b" {
		t.Fatalf("implement %s", impl.stdout)
	}
	if x := atm(t, r.Root, nil, "release", "--attempt", field(item, "attemptId").Str, "--generation", field(item, "generation").Str, "--request-id", "implement-release"); x.res.Outcome != wire.OutcomeOK {
		t.Fatal(x.res)
	}
	if x := atm(t, r.Root, nil, "pool", "confirm-safe", "--member", "b", "--allocation", field(field(item, "poolAllocation"), "allocationId").Str, "--evidence", "local:fixture", "--reason", "fixture reset", "--request-id", "safe-b"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("confirm-safe %s", x.stdout)
	}

	state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	byTitle := func(args ...string) map[string]wire.Value {
		t.Helper()
		x := atm(t, r.Root, nil, append([]string{"plan", "preview", "--pool", "db", "--stage", "review", "--exclude-member", "review"}, args...)...)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("preview %v: %s", args, x.stdout)
		}
		out := map[string]wire.Value{}
		for _, en := range field(x.res.Items[0], "entries").Arr {
			out[field(en, "ticketId").Str] = en
		}
		return out
	}
	entries := byTitle("--exclude-authors")
	a := entries[authored]
	authors := field(a, "excludedAuthors")
	if field(a, "state").Str != "SELECTED" || field(a, "detail").Kind != wire.KindNull || len(authors.Arr) != 1 || field(authors.Arr[0], "memberId").Str != "b" || field(authors.Arr[0], "poolId").Str != "db" || field(authors.Arr[0], "generation").Str != field(item, "generation").Str {
		t.Fatalf("authored entry %s", wire.Encode(a))
	}
	for id, en := range entries {
		if id == authored {
			continue
		}
		if field(en, "reason").Str != wire.CodeIndependenceUnverified || field(en, "excludedAuthors").Kind != wire.KindNull || !strings.Contains(field(en, "detail").Str, "no implement generation") {
			t.Fatalf("unverified entry %s", wire.Encode(en))
		}
	}
	exhausted := byTitle("--exclude-authors=all", "--exclude-member", "a")[authored]
	if field(exhausted, "reason").Str != wire.CodeResourceCollision || !strings.Contains(field(exhausted, "detail").Str, "excluded implement authors: member b of pool db") {
		t.Fatalf("exhausted entry %s", wire.Encode(exhausted))
	}
	if !reflect.DeepEqual(state, fixture.TreeSnapshot(t, r.StateDir)) || !reflect.DeepEqual(intents, fixture.TreeSnapshot(t, r.IntentDir)) {
		t.Fatal("preview changed queue")
	}

	args := []string{"claim", authored, "--holder", "reviewer", "--request-id", "review", "--pool", "db", "--stage", "review", "--exclude-member", "review"}
	claim := atm(t, r.Root, nil, append(args, "--exclude-authors")...)
	if claim.res.Outcome != wire.OutcomeOK || field(field(claim.res.Items[0], "poolAllocation"), "memberId").Str != "a" {
		t.Fatalf("review claim %s", claim.stdout)
	}
	if x := atm(t, r.Root, nil, append(args, "--exclude-authors")...); x.res.Outcome != wire.OutcomeOK || !field(x.res.Items[0], "replayed").Bool {
		t.Fatalf("replay %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, append(args, "--exclude-authors=all")...); x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), wire.CodeRequestIDConflict) {
		t.Fatalf("changed mode %s", x.stdout)
	}
}
