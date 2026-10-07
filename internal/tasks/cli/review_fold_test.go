package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
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

// CAL-V0-138: an in-place rewrite of an earlier receipt's content is not
// re-hashed by the carried fold; receipt audit detects it.
func TestCALV0138_RewrittenEarlierReceiptIsDetectedByReceiptAudit(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	root, _ := ergStore(t)
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	planTicket(t, root, "reviewed", "P1", `["src/"]`)
	audit := atm(t, root, nil, "receipt", "audit")
	if audit.res.Outcome != wire.OutcomeOK {
		t.Fatalf("receipt audit: %s", audit.stdout)
	}
	last, err := strconv.ParseUint(field(audit.res.Items[0], "headSeq").Str, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if last < 2 {
		t.Fatalf("head %d leaves no earlier receipt", last)
	}
	var carried store.ReviewFold
	if _, err := carried.Fold(repo, last); err != nil {
		t.Fatal(err)
	}
	name, err := snapshot.ReceiptName(1)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo.StateDir, "receipts", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	i := bytes.Index(raw, []byte(`"sha256":"`))
	if i < 0 {
		t.Fatal("receipt 1 has no digest to rewrite")
	}
	j := i + len(`"sha256":"`)
	if raw[j] == '0' {
		raw[j] = '1'
	} else {
		raw[j] = '0'
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := carried.Fold(repo, last); err != nil {
		t.Fatalf("the carried fold re-hashed an earlier receipt: %v", err)
	}
	if r := atm(t, root, nil, "receipt", "audit"); r.res.Outcome == wire.OutcomeOK || !slices.Contains(r.res.Codes, wire.CodeJournalForked) {
		t.Fatalf("receipt audit of a rewritten earlier receipt: %s, want JOURNAL_FORKED", r.stdout)
	}
}
