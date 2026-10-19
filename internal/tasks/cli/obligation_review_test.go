package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// reviewFile writes a --review input citing e2e/login.spec.ts.
func reviewFile(t *testing.T, k, maxStub string, verdicts ...string) string {
	t.Helper()
	body := `{"policy":{"reviewers":"` + k + `","maxStubRisk":"` + maxStub + `"},"subject":{"path":"e2e/login.spec.ts","startLine":"1","endLine":"3"},"verdicts":[` +
		strings.Join(verdicts, ",") + `]}`
	p := filepath.Join(t.TempDir(), "review.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func verdict(reviewer, v, stub, reason string) string {
	return `{"reviewer":"` + reviewer + `","verdict":"` + v + `","stubRisk":"` + stub + `","reason":"` + reason + `"}`
}

func reviewWitness(t *testing.T, root, id, req, head, ac, file string, extra ...string) run {
	t.Helper()
	return atm(t, root, nil, witnessArgs(id, req, obligationRevision(t, root, id), head, append([]string{"--reviewed", ac, "--review", file}, extra...)...)...)
}

func witnessReview(t *testing.T, v wire.Value, ac string) wire.Value {
	t.Helper()
	for _, e := range field(field(v, "witnessReview"), "entries").Arr {
		if field(e, "id").Str == ac {
			return e
		}
	}
	t.Fatalf("no witnessReview entry %s: %s", ac, wire.Encode(v))
	return wire.Value{}
}

// TestTOLV0073_ReviewedWitnessCreditsOnAgreement: two agreeing reviewers
// with no on-path stub risk credit the obligation with REVIEWED evidence;
// a non-OWNER REVIEWED witness is refused (TOL-V0-073, 075).
func TestTOLV0073_ReviewedWitnessCreditsOnAgreement(t *testing.T) {
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	before := obligationsShow(t, r.Root, id)
	if wire.Has(before, "witnessReview") {
		t.Fatalf("witnessReview before any review: %s", wire.Encode(before))
	}
	file := reviewFile(t, "2", "boundary", verdict("rev-a", "correct", "none", "asserts the login redirect"), verdict("rev-b", "correct", "boundary", "fixture stub at the boundary only"))
	if x := reviewWitness(t, r.Root, id, "rv-op", head, "AC-1", file, "--role", "OPERATOR"); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("OPERATOR reviewed witness: %s", x.stdout)
	}
	x := reviewWitness(t, r.Root, id, "rv-1", head, "AC-1", file)
	if x.res.Outcome != wire.OutcomeOK || !field(x.res.Items[0], "written").Bool || field(x.res.Items[0], "reviewOutcome").Str != "CREDITED" || strList(x.res.Items[0], "credited") != "AC-1" {
		t.Fatalf("reviewed witness: %s", x.stdout)
	}
	v := obligationsShow(t, r.Root, id)
	if entryStates(v)["AC-1"] != "WITNESSED" {
		t.Fatalf("AC-1 not witnessed: %s", wire.Encode(v))
	}
	for _, e := range field(v, "entries").Arr {
		if field(e, "id").Str != "AC-1" {
			continue
		}
		ev := field(e, "evidence")
		if field(ev, "source").Str != "REVIEWED" || field(ev, "commit").Str != head || field(ev, "declaration").Kind != wire.KindNull || field(ev, "reportSha256").Kind != wire.KindNull {
			t.Fatalf("reviewed evidence: %s", wire.Encode(ev))
		}
		if field(field(e, "review"), "outcome").Str != "CREDITED" || len(field(field(field(e, "review"), "review"), "verdicts").Arr) != 2 {
			t.Fatalf("entry review: %s", wire.Encode(e))
		}
	}
	wr := witnessReview(t, v, "AC-1")
	if !field(wr, "credited").Bool || field(wr, "stale").Bool || field(field(v, "witnessReview"), "effectiveWitnessed").Str != "1" {
		t.Fatalf("witnessReview: %s", wire.Encode(field(v, "witnessReview")))
	}
	// One reviewer short of K=3 is HELD on the reviewers axis.
	short := reviewFile(t, "3", "none", verdict("rev-a", "correct", "none", "ok"), verdict("rev-b", "correct", "none", "ok"))
	if x := reviewWitness(t, r.Root, id, "rv-2", head, "AC-2", short); x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "reviewOutcome").Str != "HELD" ||
		strList(x.res.Items[0], "reviewAxes") != "reviewers" {
		t.Fatalf("short review: %s", x.stdout)
	}
}

// TestTOLV0075_HeldVerdictsKeptWithReasons: one partial verdict and one
// on-path stub risk give HELD with both reasons kept; the entry stays OPEN
// (TOL-V0-075).
func TestTOLV0075_HeldVerdictsKeptWithReasons(t *testing.T) {
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	file := reviewFile(t, "2", "boundary", verdict("rev-a", "partial", "none", "misses the error branch"), verdict("rev-b", "correct", "on-path", "the session store is stubbed on the path"))
	x := reviewWitness(t, r.Root, id, "rv-h", head, "AC-1", file)
	if x.res.Outcome != wire.OutcomeOK || !field(x.res.Items[0], "written").Bool || field(x.res.Items[0], "reviewOutcome").Str != "HELD" ||
		strList(x.res.Items[0], "reviewAxes") != "stubRisk,verdict" || strList(x.res.Items[0], "credited") != "" {
		t.Fatalf("held review: %s", x.stdout)
	}
	v := obligationsShow(t, r.Root, id)
	if entryStates(v)["AC-1"] != "OPEN" {
		t.Fatalf("held entry state: %v", entryStates(v))
	}
	wr := witnessReview(t, v, "AC-1")
	var reasons []string
	for _, vd := range field(wr, "verdicts").Arr {
		reasons = append(reasons, field(vd, "reason").Str)
	}
	if field(wr, "outcome").Str != "HELD" || field(wr, "credited").Bool || strings.Join(reasons, "|") != "misses the error branch|the session store is stubbed on the path" {
		t.Fatalf("held witnessReview: %s", wire.Encode(wr))
	}
}

// TestTOLV0077_EditedExampleMakesReviewStale: editing the cited example
// marks the review stale and un-credits it in show without writing state;
// a re-review of the same blob is refused, and a new review of the edited
// example credits again (TOL-V0-076, 077).
func TestTOLV0077_EditedExampleMakesReviewStale(t *testing.T) {
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	file := reviewFile(t, "2", "none", verdict("rev-a", "correct", "none", "ok"), verdict("rev-b", "correct", "none", "ok"))
	if x := reviewWitness(t, r.Root, id, "rv-1", head, "AC-1", file); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("review: %s", x.stdout)
	}
	if x := reviewWitness(t, r.Root, id, "rv-same", head, "AC-1", file); x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), "OBLIGATION_CREDIT_MISMATCH:") {
		t.Fatalf("same-blob re-review admitted: %s", x.stdout)
	}
	spec := filepath.Join(r.Root, "e2e", "login.spec.ts")
	raw, err := os.ReadFile(spec)
	if err != nil {
		t.Fatal(err)
	}
	fixture.Write(t, spec, append(raw, []byte("\n// edited after review\n")...))
	git(t, r.Root, "add", "e2e")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-m", "edit")
	stateBefore := fixture.TreeSnapshot(t, r.StateDir)
	v := obligationsShow(t, r.Root, id)
	wr := witnessReview(t, v, "AC-1")
	if !field(wr, "stale").Bool || field(wr, "credited").Bool || field(field(v, "witnessReview"), "effectiveWitnessed").Str != "0" {
		t.Fatalf("stale witnessReview: %s", wire.Encode(field(v, "witnessReview")))
	}
	if !fixture.SameTree(stateBefore, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Fatal("show wrote state")
	}
	newHead := headOID(t, r.Root)
	if x := reviewWitness(t, r.Root, id, "rv-2", newHead, "AC-1", file); x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "reviewOutcome").Str != "CREDITED" {
		t.Fatalf("re-review of the edited example: %s", x.stdout)
	}
	wr = witnessReview(t, obligationsShow(t, r.Root, id), "AC-1")
	if field(wr, "stale").Bool || !field(wr, "credited").Bool {
		t.Fatalf("re-reviewed witnessReview: %s", wire.Encode(wr))
	}
}

// TestTOLV0074_AuthorReviewerRefused: a reviewer named as an author is
// refused OBLIGATION_REVIEWER_NOT_INDEPENDENT and nothing is written; a
// secret-shaped reason refuses SECRET_DETECTED (TOL-V0-074, 078).
func TestTOLV0074_AuthorReviewerRefused(t *testing.T) {
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	before := fixture.TreeSnapshot(t, r.StateDir)
	file := reviewFile(t, "1", "none", verdict("alice", "correct", "none", "self review"))
	x := reviewWitness(t, r.Root, id, "rv-a", head, "AC-1", file, "--exclude-author", "alice")
	if x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), "OBLIGATION_REVIEWER_NOT_INDEPENDENT:") {
		t.Fatalf("author reviewer admitted: %s", x.stdout)
	}
	secret := reviewFile(t, "1", "none", verdict("rev-a", "correct", "none", "token ghp_"+strings.Repeat("A", 36)))
	if x := reviewWitness(t, r.Root, id, "rv-s", head, "AC-1", secret); x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), "SECRET_DETECTED") {
		t.Fatalf("secret reason admitted: %s", x.stdout)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Fatal("a refused review wrote state")
	}
}
