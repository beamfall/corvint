package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// planWitnessView is the TOL-V0-104..106 view of one report: each ledger
// witness, and each plan finding with its considered witness.
func planWitnessView(t *testing.T, item wire.Value, planOnly string) (witnesses, findings string) {
	t.Helper()
	witnesses = string(wire.Encode(field(item, "ledgerWitnesses")))
	var out []string
	for _, f := range field(item, "findings").Arr {
		if planOnly != "" && field(f, "path").Str != planOnly {
			continue
		}
		c := "-"
		if wire.Has(f, "considered") {
			c = string(wire.Encode(field(f, "considered")))
		}
		out = append(out, field(f, "kind").Str+":"+field(f, "id").Str+":"+c)
	}
	return witnesses, strings.Join(out, ",")
}

// TestTOLV0940_PreflightPlanWitnessesMatchesPlan: `preflight --plan PATH
// --witnesses` names the same ledger witnesses, the same considered witness
// on an UNASSIGNED finding, and the same STALE_REVIEW finding as `ticket
// obligations plan --witnesses` on that plan; without --witnesses the
// preflight report is unchanged (TOL-V0-940, V1-1961).
func TestTOLV0940_PreflightPlanWitnessesMatchesPlan(t *testing.T) {
	r, id, _ := obligationRepo(t)
	seedObligations(t, r.Root, id)
	const tests = `[{"test":"a","project":"api","obligations":["AC-1"]}]`
	fixture.Write(t, filepath.Join(r.Root, "e2e", "plan.json"), []byte(`{"profile":"taskman-obligation-plan/0","ticketId":"`+id+`","tests":`+tests+`}`))
	git(t, r.Root, "add", "e2e")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "plan")
	head := headOID(t, r.Root)
	credit := reviewFile(t, "1", "none", verdict("rev-a", "correct", "none", "asserts the login redirect"))
	if x := reviewWitness(t, r.Root, id, "rv-1", head, "AC-1", credit); x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "reviewOutcome").Str != "CREDITED" {
		t.Fatalf("review: %s", x.stdout)
	}
	held := reviewFile(t, "1", "none", verdict("rev-a", "partial", "none", "misses the error branch"))
	if x := reviewWitness(t, r.Root, id, "rv-2", head, "AC-2", held); x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "reviewOutcome").Str != "HELD" {
		t.Fatalf("held review: %s", x.stdout)
	}
	plan := planFile(t, id, tests)

	compare := func(wantStale bool) {
		t.Helper()
		p := planRun(t, r.Root, id, plan, "--witnesses")
		x := atm(t, r.Root, nil, "preflight", id, "--plan", "e2e/plan.json", "--witnesses")
		if len(p.res.Items) != 1 || len(x.res.Items) != 1 || x.res.Outcome == wire.OutcomeOK {
			t.Fatalf("plan %s\npreflight %s", p.stdout, x.stdout)
		}
		pw, pf := planWitnessView(t, p.res.Items[0], "")
		xw, xf := planWitnessView(t, x.res.Items[0], "e2e/plan.json")
		if pw != xw || pf != xf {
			t.Fatalf("preflight witnesses differ from the plan:\nplan      %s %s\npreflight %s %s", pw, pf, xw, xf)
		}
		if !strings.Contains(pf, "UNASSIGNED:AC-2:") || !strings.Contains(pf, `"REVIEW_HELD"`) {
			t.Fatalf("no considered held witness: %s", pf)
		}
		stale := strings.Contains(pf, "STALE_REVIEW:AC-1:")
		warned := strings.Contains(string(x.stdout), "stale or unresolved at HEAD (STALE_REVIEW)")
		if stale != wantStale || warned != wantStale {
			t.Fatalf("stale %v warned %v, want %v: %s", stale, warned, wantStale, x.stdout)
		}
		for _, f := range field(x.res.Items[0], "findings").Arr {
			if field(f, "kind").Str == "STALE_REVIEW" && (field(f, "remedy").Str == "" || !strings.Contains(field(f, "detail").Str, "REVIEW_STALE_AT_HEAD")) {
				t.Fatalf("stale finding: %s", wire.Encode(f))
			}
		}
	}
	compare(false)

	plain := atm(t, r.Root, nil, "preflight", id, "--plan", "e2e/plan.json")
	if len(plain.res.Items) != 1 || wire.Has(plain.res.Items[0], "ledgerWitnesses") {
		t.Fatalf("plain preflight names ledger witnesses: %s", plain.stdout)
	}
	for _, f := range field(plain.res.Items[0], "findings").Arr {
		if wire.Has(f, "considered") || field(f, "kind").Str == "STALE_REVIEW" {
			t.Fatalf("plain preflight judged witnesses: %s", plain.stdout)
		}
	}

	fixture.Write(t, filepath.Join(r.Root, "e2e", "login.spec.ts"), []byte(obligationSpec+"// changed\n"))
	git(t, r.Root, "add", "e2e")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "change spec")
	compare(true)
}

// TestTOLV0940_PreflightWitnessesUsage: --witnesses needs --plan and does
// not combine with --cache (TOL-V0-940).
func TestTOLV0940_PreflightWitnessesUsage(t *testing.T) {
	r, id, _ := obligationRepo(t)
	seedObligations(t, r.Root, id)
	if x := atm(t, r.Root, nil, "preflight", id, "--witnesses"); x.res.Outcome != wire.OutcomeError || !warned(x, "--witnesses judges the plan's ledger witnesses") {
		t.Fatalf("--witnesses without --plan: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, "preflight", id, "--plan", "e2e/plan.json", "--witnesses", "--cache"); x.res.Outcome != wire.OutcomeError || !warned(x, "--witnesses judges reviewed witnesses at HEAD") {
		t.Fatalf("--witnesses with --cache: %s", x.stdout)
	}
}

// TestTOLV0940_PreflightWitnessSignatureKeysCandidate: a --witnesses
// failure signature covers the candidate branch's commit, so after
// --refuse-repeat is exhausted on a stale witness, repairing the candidate
// admits the next run, which judges the repaired witness (TOL-V0-940).
func TestTOLV0940_PreflightWitnessSignatureKeysCandidate(t *testing.T) {
	r, id, _ := obligationRepo(t)
	seedObligations(t, r.Root, id)
	const tests = `[{"test":"a","project":"api","obligations":["AC-1"]}]`
	fixture.Write(t, filepath.Join(r.Root, "e2e", "plan.json"), []byte(`{"profile":"taskman-obligation-plan/0","ticketId":"`+id+`","tests":`+tests+`}`))
	git(t, r.Root, "add", "e2e")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "plan")
	head := headOID(t, r.Root)
	credit := reviewFile(t, "1", "none", verdict("rev-a", "correct", "none", "asserts the login redirect"))
	if x := reviewWitness(t, r.Root, id, "rv-1", head, "AC-1", credit); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("review: %s", x.stdout)
	}
	short := id[strings.LastIndex(id, ":")+1:]
	main := strings.TrimSpace(gitOut(t, r.Root, "rev-parse", "--abbrev-ref", "HEAD"))
	git(t, r.Root, "branch", short, head)
	c := atm(t, r.Root, nil, "claim", id, "--holder", "owner", "--request-id", "claim-c")
	if c.res.Outcome != wire.OutcomeOK {
		t.Fatalf("claim: %s", c.stdout)
	}
	attempt, generation := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	if s := atm(t, r.Root, nil, "submit", "--attempt", attempt, "--generation", generation, "--tree", strings.TrimSpace(gitOut(t, r.Root, "rev-parse", head+"^{tree}")), "--request-id", "submit-c"); s.res.Outcome != wire.OutcomeOK {
		t.Fatalf("submit: %s", s.stdout)
	}
	spec := filepath.Join(r.Root, "e2e", "login.spec.ts")
	original, err := os.ReadFile(spec)
	if err != nil {
		t.Fatal(err)
	}
	onCandidate := func(content []byte, msg string) {
		t.Helper()
		git(t, r.Root, "checkout", "-q", short)
		fixture.Write(t, spec, content)
		git(t, r.Root, "add", "e2e")
		git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", msg)
		git(t, r.Root, "checkout", "-q", main)
	}
	onCandidate(append(append([]byte{}, original...), "\n// stale\n"...), "stale the witness")
	preflight := func() run {
		return atm(t, r.Root, nil, "preflight", id, "--plan", "e2e/plan.json", "--witnesses", "--refuse-repeat", "1")
	}
	if x := preflight(); !strings.Contains(string(x.stdout), "STALE_REVIEW") || field(signatureMember(t, x), "status").Str != "RECORDED" {
		t.Fatalf("stale witness: %s", x.stdout)
	}
	if x := preflight(); !hasCode(x.res, wire.CodeLoopDetected) {
		t.Fatalf("repeat not refused: %s", x.stdout)
	}
	onCandidate(original, "repair the witness")
	x := preflight()
	if hasCode(x.res, wire.CodeLoopDetected) || len(x.res.Items) != 1 {
		t.Fatalf("repaired candidate refused before it was checked: %s", x.stdout)
	}
	for _, f := range field(x.res.Items[0], "findings").Arr {
		if field(f, "kind").Str == "STALE_REVIEW" {
			t.Fatalf("repaired witness still stale: %s", x.stdout)
		}
	}
}
