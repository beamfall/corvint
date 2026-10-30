package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestKHNV0008_ProvenanceThroughTheCLI: `ticket know-how add --attempt
// --generation` commits only provenance the audited attempt inventory
// proves: the home ticket's claimed attempt at its generation. An unknown
// attempt, an unrecorded generation, another ticket's attempt and a
// generation without an attempt are refused PROVENANCE_UNVERIFIED with
// nothing written and the asserted value never echoed.
func TestKHNV0008_ProvenanceThroughTheCLI(t *testing.T) {
	r := knowHowCLIRepo(t)
	home := planTicket(t, r.Root, "home", "P2", `["src/a.go"]`)
	other := planTicket(t, r.Root, "other", "P2", `["docs/"]`)
	claim := func(id, req string) (string, string) {
		t.Helper()
		c := atm(t, r.Root, nil, "claim", id, "--holder", "agent", "--request-id", req)
		if c.res.Outcome != wire.OutcomeOK {
			t.Fatalf("claim %s: %s", id, c.stdout)
		}
		return field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	}
	attempt, gen := claim(home, "claim-home")
	otherAttempt, otherGen := claim(other, "claim-other")
	add := func(req string, extra ...string) run {
		t.Helper()
		return atm(t, r.Root, nil, append([]string{"ticket", "know-how", "add", home, "--request-id", req, "--expected-revision", "1",
			"--issued-at", "2026-10-07T12:00:00Z", "--text", "verified note", "--anchor", "src/a.go"}, extra...)...)
	}
	for name, extra := range map[string][]string{
		"unknown attempt":            {"--attempt", "att-unknown-khn", "--generation", gen},
		"unrecorded generation":      {"--attempt", attempt, "--generation", "99"},
		"other ticket's attempt":     {"--attempt", otherAttempt, "--generation", otherGen},
		"generation without attempt": {"--generation", gen},
	} {
		state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
		x := add("kh-bad-"+strings.ReplaceAll(name, " ", "-"), extra...)
		if x.res.Outcome == wire.OutcomeOK && field(x.res.Items[0], "outcome").Str == mutation.OutcomeCompleted {
			t.Fatalf("%s accepted: %s", name, x.stdout)
		}
		if !strings.Contains(string(x.stdout), `"`+wire.CodeProvenanceUnverified+`"`) || strings.Contains(string(x.stdout), "att-unknown-khn") {
			t.Fatalf("%s: want PROVENANCE_UNVERIFIED without the echo: %s", name, x.stdout)
		}
		if !fixture.SameTree(state, fixture.TreeSnapshot(t, r.StateDir)) || !fixture.SameTree(intents, fixture.TreeSnapshot(t, r.IntentDir)) {
			t.Fatalf("%s: a refused write changed the store", name)
		}
	}
	x := add("kh-good", "--attempt", attempt, "--generation", gen)
	if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "outcome").Str != mutation.OutcomeCompleted {
		t.Fatalf("verified provenance refused: %s", x.stdout)
	}
	show := atm(t, r.Root, nil, "ticket", "show", home)
	ledger := field(show.res.Items[0], "knowHow").Arr
	if len(ledger) != 1 || field(ledger[0], "attempt").Str != attempt || field(ledger[0], "generation").Str != gen {
		t.Fatalf("ticket show knowHow: %s", show.stdout)
	}
	if again := add("kh-good", "--attempt", attempt, "--generation", gen); again.res.Outcome != wire.OutcomeOK || !field(again.res.Items[0], "replayed").Bool {
		t.Fatalf("identical retry did not replay: %s", again.stdout)
	}
}

// TestKHNV0014_UnreadableInventoryDeliversUnavailable: a claim whose
// inventory cannot be loaded when the response is built still commits and
// delivers knowHow state UNAVAILABLE with the load's code and a warning,
// never an empty list (KHN-V0-006 failure mode).
func TestKHNV0014_UnreadableInventoryDeliversUnavailable(t *testing.T) {
	r := knowHowCLIRepo(t)
	home := planTicket(t, r.Root, "home", "P2", `["src/a.go"]`)
	if x := atm(t, r.Root, nil, "ticket", "know-how", "add", home, "--request-id", "kh-1", "--expected-revision", "1",
		"--issued-at", "2026-10-07T12:00:00Z", "--text", "note", "--anchor", "src/a.go"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("add: %s", x.stdout)
	}
	work := planTicket(t, r.Root, "work", "P2", `["src/"]`)
	claim := []string{"claim", work, "--holder", "agent", "--request-id", "claim-work"}
	first := atm(t, r.Root, nil, claim...)
	if first.res.Outcome != wire.OutcomeOK || field(field(first.res.Items[0], "knowHow"), "state").Str != "DELIVERED" {
		t.Fatalf("claim: %s", first.stdout)
	}
	garbage := filepath.Join(r.IntentDir, "tickets", "ZZ-garbage.json")
	fixtureWrite(t, garbage, []byte("{not json\n"))
	c := atm(t, r.Root, nil, claim...)
	item := c.res.Items[0]
	kh := field(item, "knowHow")
	if c.res.Outcome != wire.OutcomeOK || field(item, "outcome").Str != mutation.OutcomeCompleted || !field(item, "replayed").Bool ||
		field(item, "attemptId").Str != field(first.res.Items[0], "attemptId").Str {
		t.Fatalf("the committed claim did not replay: %s", c.stdout)
	}
	if field(kh, "state").Str != "UNAVAILABLE" || field(kh, "code").Str != wire.CodeMalformed || field(kh, "notes").Kind != wire.KindNull ||
		field(kh, "trust").Str != "UNTRUSTED_AGENT_AUTHORED_DATA" {
		t.Fatalf("unreadable inventory delivery: %s", wire.Encode(kh))
	}
	warned := false
	for _, w := range c.res.Warnings {
		warned = warned || strings.HasPrefix(w, "know-how notes unavailable (MALFORMED)")
	}
	if !warned {
		t.Fatalf("no UNAVAILABLE warning: %v", c.res.Warnings)
	}
	// The member is computed per response: once the inventory reads again the
	// same replay delivers the note.
	if err := os.Remove(garbage); err != nil {
		t.Fatal(err)
	}
	if again := atm(t, r.Root, nil, claim...); field(field(again.res.Items[0], "knowHow"), "state").Str != "DELIVERED" ||
		len(field(field(again.res.Items[0], "knowHow"), "notes").Arr) != 1 {
		t.Fatalf("recovered delivery: %s", again.stdout)
	}
}

func fixtureWrite(t *testing.T, p string, b []byte) {
	t.Helper()
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
}
