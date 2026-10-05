package cli_test

import (
	"bytes"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestERGV0011_CompletionOfferThroughTheCLI drives the read-only completion
// offer against a native store: a RETURN, a resubmission, a PASS while the
// subject attempt is live, a STALE PASS and an undeclared gate (UNKNOWN) give
// no offer; a CURRENT PASS with no other blocker makes `ticket show`,
// `ticket blockers` and `plan preview` report nextAction complete-manual with
// the review head as suggested evidence. Reading writes nothing, the ticket
// stays OPEN, and a ticket without review references renders byte-identically
// before and after the offer exists.
func TestERGV0011_CompletionOfferThroughTheCLI(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	root, tree := ergStore(t)
	id := planTicket(t, root, "reviewed", "P1", `["src/"]`)
	other := planTicket(t, root, "unreviewed", "P2", `["lib/"]`)
	runOK := func(args ...string) run {
		t.Helper()
		r := atm(t, root, nil, args...)
		if r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s", args, r.stdout)
		}
		return r
	}
	headSeq := func() string {
		t.Helper()
		return field(runOK("receipt", "audit").res.Items[0], "headSeq").Str
	}
	show := func(tid string) wire.Value {
		t.Helper()
		return runOK("ticket", "show", tid).res.Items[0]
	}
	entry := func(tid string) wire.Value {
		t.Helper()
		for _, e := range field(runOK("plan", "preview").res.Items[0], "entries").Arr {
			if field(e, "ticketId").Str == tid {
				return e
			}
		}
		t.Fatalf("plan preview has no entry for %s", tid)
		return wire.Value{}
	}
	noOffer := func(when, want string) {
		t.Helper()
		v := show(id)
		if got := field(v, "nextAction").Str; got != want {
			t.Fatalf("%s: nextAction %q, want %q: %s", when, got, want, wire.Encode(v))
		}
		if _, ok := v.Obj.Get("suggestedEvidence"); ok {
			t.Fatalf("%s: suggestedEvidence without an offer: %s", when, wire.Encode(v))
		}
		e := entry(id)
		for _, k := range []string{"nextAction", "suggestedEvidence"} {
			if _, ok := e.Obj.Get(k); ok {
				t.Fatalf("%s: plan entry carries %s without an offer: %s", when, k, wire.Encode(e))
			}
		}
	}
	baseShow, baseEntry := wire.Encode(show(other)), wire.Encode(entry(other))
	noOffer("before any review", "admit")

	c := runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim-a")
	attempt, generation := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	runOK("submit", "--attempt", attempt, "--generation", generation, "--tree", tree, "--request-id", "submit-a")
	subject := headSeq()
	record := func(req, verdict, gen, rev string) {
		t.Helper()
		runOK("gate", "record", id, "--gate", ergGate, "--verdict", verdict, "--subject-receipt", subject,
			"--expected-generation", gen, "--expected-revision", rev, "--request-id", req, "--reason", "TESTS:missing case")
	}
	record("review-1", "RETURN", "0", "0")
	noOffer("RETURN with a live attempt", "wait-attempt")
	runOK("gate", "resubmit", id, "--gate", ergGate, "--author-attempt", attempt, "--subject-receipt", subject,
		"--expected-generation", "1", "--expected-revision", "1", "--reason", "FIXED:added the case", "--request-id", "resubmit-1")
	noOffer("RESUBMITTED", "wait-attempt")
	record("review-2", "PASS", "2", "2")
	noOffer("PASS while the attempt is live", "wait-attempt")

	runOK("release", "--attempt", attempt, "--generation", generation, "--reason", wire.CodeHandoff, "--request-id", "release-a")
	head := string(show(id).Obj.Vals["record"].Obj.Vals["externalReviews"].Obj.Vals[ergGate].Obj.Vals["head"].Str)
	if head == "" {
		t.Fatal("no review head on the ticket record")
	}
	before := headSeq()
	for _, verb := range []string{"show", "blockers"} {
		v := runOK("ticket", verb, id).res.Items[0]
		ev := field(v, "suggestedEvidence")
		if field(v, "nextAction").Str != "complete-manual" || ev.Kind != wire.KindArray || len(ev.Arr) != 1 || ev.Arr[0].Str != head {
			t.Fatalf("ticket %s: no completion offer for a CURRENT PASS: %s", verb, wire.Encode(v))
		}
		if field(v, "status").Str != "OPEN" || field(v, "completion").Kind != wire.KindNull {
			t.Fatalf("ticket %s: the offer completed the ticket: %s", verb, wire.Encode(v))
		}
	}
	e := entry(id)
	if ev := field(e, "suggestedEvidence"); field(e, "nextAction").Str != "complete-manual" || ev.Kind != wire.KindArray || len(ev.Arr) != 1 || ev.Arr[0].Str != head {
		t.Fatalf("plan preview: no completion offer for a CURRENT PASS: %s", wire.Encode(e))
	}
	if headSeq() != before {
		t.Fatal("reading the offer advanced the journal")
	}
	if got := wire.Encode(show(other)); !bytes.Equal(got, baseShow) {
		t.Fatalf("a ticket without reviews changed:\n%s\n%s", baseShow, got)
	}
	if got := wire.Encode(entry(other)); !bytes.Equal(got, baseEntry) {
		t.Fatalf("a plan entry without reviews changed:\n%s\n%s", baseEntry, got)
	}

	// A queue pause is a planner blocker the ticket view cannot see: the
	// plan entry is BLOCKED (PAUSED), show reports the pause through its
	// claimability, and neither offers. Unpausing restores the offer.
	runOK("pause", "--request-id", "pause-1")
	noOffer("queue paused", "admit")
	if e := entry(id); field(e, "state").Str != "BLOCKED" || field(e, "reason").Str != wire.CodePaused {
		t.Fatalf("paused plan entry is not BLOCKED/PAUSED: %s", wire.Encode(e))
	}
	if v := show(id); field(v, "claimabilityReason").Str != wire.CodePaused {
		t.Fatalf("paused show does not report PAUSED: %s", wire.Encode(v))
	}
	runOK("unpause", "--request-id", "unpause-1")
	if field(show(id), "nextAction").Str != "complete-manual" || field(entry(id), "nextAction").Str != "complete-manual" {
		t.Fatal("unpausing did not restore the offer")
	}

	// A gate the policy no longer declares reads UNKNOWN: no offer.
	ergPolicyUpdateStages(t, root, "3", nil)
	noOffer("undeclared gate", "admit")
	ergPolicyUpdate(t, root, "4")
	if field(show(id), "nextAction").Str != "complete-manual" {
		t.Fatal("redeclaring the same gate did not restore the offer")
	}

	// A newer submission stales the PASS: no offer, also once released.
	c = runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim-b")
	runOK("submit", "--attempt", field(c.res.Items[0], "attemptId").Str, "--generation", field(c.res.Items[0], "generation").Str, "--tree", tree, "--request-id", "submit-b")
	runOK("release", "--attempt", field(c.res.Items[0], "attemptId").Str, "--generation", field(c.res.Items[0], "generation").Str,
		"--reason", wire.CodeHandoff, "--request-id", "release-b")
	noOffer("STALE PASS", "admit")
	runOK("receipt", "audit")
}
