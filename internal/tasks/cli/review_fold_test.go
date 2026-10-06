package cli_test

import (
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-138: the dispatcher's carried review fold answers exactly as the
// whole-history fold after every real review transition (submission, RETURN,
// resubmit, PASS), each a tail append to the carried prefix.
func TestCALV0138_CarriedReviewFoldMatchesWholeHistory(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	root, tree := ergStore(t)
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	id := planTicket(t, root, "reviewed", "P1", `["src/"]`)
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
	var carried store.ReviewFold
	check := func(step string) {
		t.Helper()
		last, err := wire.ParseCount("headSeq", headSeq())
		if err != nil {
			t.Fatal(err)
		}
		got, err := carried.Fold(repo, uint64(last.Int()))
		if err != nil {
			t.Fatalf("%s: carried fold: %v", step, err)
		}
		want, err := store.FoldExternalReviews(repo, uint64(last.Int()), nil)
		if err != nil {
			t.Fatalf("%s: whole fold: %v", step, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: carried fold differs from the whole-history fold", step)
		}
	}
	check("plan")

	c := runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim-a")
	attempt, generation := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	runOK("submit", "--attempt", attempt, "--generation", generation, "--tree", tree, "--request-id", "submit-a")
	subject := headSeq()
	check("submit")
	record := func(req, verdict, gen, rev string) {
		t.Helper()
		runOK("gate", "record", id, "--gate", ergGate, "--verdict", verdict, "--subject-receipt", subject,
			"--expected-generation", gen, "--expected-revision", rev, "--request-id", req, "--reason", "TESTS:missing case")
	}
	record("review-1", "RETURN", "0", "0")
	check("RETURN")
	runOK("gate", "resubmit", id, "--gate", ergGate, "--author-attempt", attempt, "--subject-receipt", subject,
		"--expected-generation", "1", "--expected-revision", "1", "--reason", "FIXED:added the case", "--request-id", "resubmit-1")
	check("resubmit")
	record("review-2", "PASS", "2", "2")
	check("PASS")
}
