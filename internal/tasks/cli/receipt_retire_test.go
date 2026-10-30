//go:build darwin || linux

package cli_test

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestJRTV0012_RetireCLIAndDoctorJournal (JRT-V0-001, JRT-V0-012,
// JRT-V0-014): compact help says it reclaims superseded liveness bodies only
// and points to retire; doctor reports the journal size, growth rate and
// thresholds and raises JOURNAL_GROWTH past one; `receipt retire` needs an
// archive outside the store, its dry run writes nothing, and retire and
// unretire round-trip with doctor naming the retired prefix in between.
func TestJRTV0012_RetireCLIAndDoctorJournal(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	help := atm(t, root, nil, "receipt", "compact", "--help", "--verbose")
	if note := field(help.res.Items[0], "note").Str; !strings.Contains(note, "superseded liveness bodies only") || !strings.Contains(note, "receipt retire") {
		t.Fatalf("compact help: %s", help.stdout)
	}
	for _, verb := range []string{"retire", "unretire"} {
		if x := atm(t, root, nil, "receipt", verb, "--help"); x.res.Outcome != wire.OutcomeOK || !strings.Contains(field(x.res.Items[0], "usage").Str, "--archive") {
			t.Fatalf("%s help: %s", verb, x.stdout)
		}
	}

	_, item := doctorOK(t, root)
	j := field(item, "journal")
	receipts, err := strconv.ParseUint(field(j, "receipts").Str, 10, 64)
	if err != nil || receipts == 0 || field(j, "bytes").Str == "0" || field(j, "retiredThrough").Str != "0" || field(j, "thresholdReceipts").Str != "16384" {
		t.Fatalf("doctor journal: %s", wire.Encode(item))
	}
	if len(findingsOf(item, "JOURNAL_GROWTH")) != 0 {
		t.Fatalf("small journal raised JOURNAL_GROWTH: %s", wire.Encode(item))
	}
	restore := cli.SetDoctorJournalThresholds(0, 1<<40)
	_, item = doctorOK(t, root)
	restore()
	if f := findingsOf(item, "JOURNAL_GROWTH"); len(f) != 1 || !strings.Contains(field(f[0], "remedy").Str, "receipt retire") {
		t.Fatalf("JOURNAL_GROWTH past the receipt threshold: %s", wire.Encode(item))
	}

	if x := atm(t, root, nil, "receipt", "retire", "--dry-run"); x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeUnsupported) {
		t.Fatalf("retire of a one-receipt journal: %s", x.stdout)
	}
	planTicket(t, root, "first", "P1", `["src/"]`)
	planTicket(t, root, "second", "P2", `["doc/"]`)
	_, item = doctorOK(t, root)
	if receipts, err = strconv.ParseUint(field(field(item, "journal"), "receipts").Str, 10, 64); err != nil || receipts < 3 {
		t.Fatalf("doctor journal after two tickets: %s", wire.Encode(item))
	}
	if x := atm(t, root, nil, "receipt", "retire"); x.code == 0 || x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("retire without --archive: %s", x.stdout)
	}
	if x := atm(t, root, nil, "receipt", "unretire", "--dry-run", "--archive", "a.tar"); x.code == 0 || x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("unretire accepted --dry-run: %s", x.stdout)
	}
	dry := atm(t, root, nil, "receipt", "retire", "--dry-run")
	if dry.res.Outcome != wire.OutcomeOK || field(dry.res.Items[0], "dryRun").Kind != wire.KindBool || !field(dry.res.Items[0], "dryRun").Bool {
		t.Fatalf("retire dry run: %s", dry.stdout)
	}
	path := filepath.Join(t.TempDir(), "retired.tar")
	done := atm(t, root, nil, "receipt", "retire", "--archive", path)
	if done.code != 0 || done.res.Outcome != wire.OutcomeOK || field(done.res.Items[0], "anchorSeq").Str != strconv.FormatUint(receipts, 10) {
		t.Fatalf("retire: %s", done.stdout)
	}
	_, item = doctorOK(t, root)
	if j := field(item, "journal"); field(j, "receipts").Str != "1" || field(j, "retiredThrough").Str != strconv.FormatUint(receipts-1, 10) {
		t.Fatalf("doctor on a sealed store: %s", wire.Encode(item))
	}
	if a := atm(t, root, nil, "receipt", "audit"); a.code != 0 || a.res.Outcome != wire.OutcomeOK || field(a.res.Items[0], "anchorSeq").Str != strconv.FormatUint(receipts, 10) || field(a.res.Items[0], "retiredThrough").Str != strconv.FormatUint(receipts-1, 10) {
		t.Fatalf("audit of a sealed store does not report its anchor: %s", a.stdout)
	}
	un := atm(t, root, nil, "receipt", "unretire", "--archive", path)
	if un.code != 0 || un.res.Outcome != wire.OutcomeOK {
		t.Fatalf("unretire: %s", un.stdout)
	}
	_, item = doctorOK(t, root)
	if j := field(item, "journal"); field(j, "receipts").Str != strconv.FormatUint(receipts, 10) || field(j, "retiredThrough").Str != "0" {
		t.Fatalf("doctor after unretire: %s", wire.Encode(item))
	}
	if a := atm(t, root, nil, "receipt", "audit"); a.code != 0 || field(a.res.Items[0], "anchorSeq").Str != "0" || field(a.res.Items[0], "retiredThrough").Str != "0" {
		t.Fatalf("audit after unretire: %s", a.stdout)
	}
}

// TestJRTV0008_ReviewAcrossRetirement (JRT-V0-007, JRT-V0-008): after
// retire, external review verdicts and a resubmission under the review
// policy posted before the anchor commit, and receipt audit, whose binding
// folds start from the anchor's afterimages, stays consistent. After
// unretire, the reviewed ticket, released but not terminal, carries review
// heads, so retire refuses TICKET_STATE.
func TestJRTV0008_ReviewAcrossRetirement(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	root, tree := ergStore(t)
	id := planTicket(t, root, "reviewed", "P1", `["src/"]`)
	runOK := func(args ...string) run {
		t.Helper()
		x := atm(t, root, nil, args...)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s", args, x.stdout)
		}
		return x
	}
	path := filepath.Join(t.TempDir(), "retired.tar")
	anchor := field(runOK("receipt", "retire", "--archive", path).res.Items[0], "anchorSeq").Str
	c := runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim-a")
	attempt, generation := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	runOK("submit", "--attempt", attempt, "--generation", generation, "--tree", tree, "--request-id", "submit-a")
	subject := field(runOK("receipt", "audit").res.Items[0], "headSeq").Str
	record := func(req, verdict, gen, rev string) {
		t.Helper()
		runOK("gate", "record", id, "--gate", ergGate, "--verdict", verdict, "--subject-receipt", subject,
			"--expected-generation", gen, "--expected-revision", rev, "--request-id", req, "--reason", "TESTS:missing case")
	}
	record("review-1", "RETURN", "0", "0")
	runOK("gate", "resubmit", id, "--gate", ergGate, "--author-attempt", attempt, "--subject-receipt", subject,
		"--expected-generation", "1", "--expected-revision", "1", "--reason", "FIXED:added the case", "--request-id", "resubmit-1")
	record("review-2", "PASS", "2", "2")
	if _, state := ergGateView(t, root, id); state != dispatch.GatePass {
		t.Fatalf("review after the anchor: %s", state)
	}
	a := runOK("receipt", "audit")
	if !auditConsistent(t, root) || field(a.res.Items[0], "anchorSeq").Str != anchor {
		t.Fatalf("audit of review transitions after the anchor: %s", a.stdout)
	}
	runOK("release", "--attempt", attempt, "--generation", generation, "--request-id", "release-a")
	runOK("receipt", "unretire", "--archive", path)
	if !auditConsistent(t, root) {
		t.Fatal("audit after unretire")
	}
	if x := atm(t, root, nil, "receipt", "retire", "--dry-run"); x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeTicketState) {
		t.Fatalf("retire with review heads on an open ticket: %s", x.stdout)
	}
}

// TestJRTV0008_ObligationAcrossRetirement (JRT-V0-008): an obligation ledger
// change after the anchor on a ticket seeded before it commits, and receipt
// audit, whose obligation fold starts from the anchor's ticket afterimage,
// stays consistent.
func TestJRTV0008_ObligationAcrossRetirement(t *testing.T) {
	r, ledger, _ := obligationRepo(t)
	seedObligations(t, r.Root, ledger)
	sealedAt := filepath.Join(t.TempDir(), "ledger.tar")
	if x := atm(t, r.Root, nil, "receipt", "retire", "--archive", sealedAt); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("retire a store with an obligation ledger: %s", x.stdout)
	}
	set := atm(t, r.Root, nil, "ticket", "obligations", "set", "--target", ledger, "--expected-revision", obligationRevision(t, r.Root, ledger),
		"--request-id", "set-after", "--payload", `{"changes":[{"id":"AC-3","state":"BLOCKED","core":null,"reason":"waits on fixture"}]}`)
	if set.res.Outcome != wire.OutcomeOK || entryStates(obligationsShow(t, r.Root, ledger))["AC-3"] != "BLOCKED" {
		t.Fatalf("obligation change after the anchor: %s", set.stdout)
	}
	if !auditConsistent(t, r.Root) {
		t.Fatal("audit of an obligation change after the anchor")
	}
}
