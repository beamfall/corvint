package cli_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestERGV0023_ReasonsCanonicalizedNotRefused: repeated --reason flags in
// any order compose the request's canonical-byte sorted reasons set, with an
// exact repeat dropped (V1-1923). The retained request carries the sorted
// set, and a retry with the same --request-id and the flags in another order
// replays it instead of refusing REQUEST_ID_CONFLICT.
func TestERGV0023_ReasonsCanonicalizedNotRefused(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	root, tree := ergStore(t)
	runOK := func(args ...string) run {
		t.Helper()
		r := atm(t, root, nil, args...)
		if r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s", args, r.stdout)
		}
		return r
	}
	id := planTicket(t, root, "reason order", "P1", `["src/"]`)
	c := runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim")
	attempt, gen := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	runOK("submit", "--attempt", attempt, "--generation", gen, "--tree", tree, "--request-id", "submit")
	subject := field(runOK("receipt", "audit").res.Items[0], "headSeq").Str
	record := func(reasons ...string) []string {
		a := []string{"gate", "record", id, "--gate", ergGate, "--verdict", "RETURN", "--subject-receipt", subject,
			"--expected-generation", "0", "--expected-revision", "0", "--request-id", "ordered", "--issued-at", "2026-10-05T12:00:00Z"}
		for _, r := range reasons {
			a = append(a, "--reason", r)
		}
		return a
	}
	// "TESTS:x" sorts after "TEST:x" by element bytes ({"code":"TEST"... <
	// {"code":"TESTS"...) although "TESTS:x" < "TEST:x" as flag strings.
	first := runOK(record("TESTS:x", "DOCS:b", "TEST:x", "DOCS:b")...)
	want := `"reasons":[{"code":"DOCS","text":"b"},{"code":"TEST","text":"x"},{"code":"TESTS","text":"x"}]`
	if h := runOK("gate", "history", id, "--gate", ergGate); !strings.Contains(string(h.stdout), want) {
		t.Fatalf("history does not carry the sorted, deduplicated reasons %s:\n%s", want, h.stdout)
	}
	again := runOK(record("TEST:x", "TESTS:x", "DOCS:b")...)
	if field(again.res.Items[0], "resultingRevision").Str != field(first.res.Items[0], "resultingRevision").Str || !field(again.res.Items[0], "replayed").Bool {
		t.Fatalf("a reordered retry did not replay the retained request:\nfirst %s\nagain %s", first.stdout, again.stdout)
	}
}
