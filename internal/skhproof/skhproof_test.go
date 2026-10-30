package skhproof

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appmap"
)

// draftLines is a COMPLETE stage 7 draft: the copied primary test with the guard after its opener.
func draftLines() []string {
	return []string{
		"import { test, expect } from '@playwright/test';",
		"test('ships an order', async ({ page }) => {",
		appmap.DraftGuardLine,
		"  await page.goto('/orders/1');",
		"  await page.getByRole('button', { name: 'Ship' }).click();",
		"  await expect(page.getByText('Shipped')).toBeVisible();",
		"});",
	}
}

func answerFor(t *testing.T, status string, lines []string, mutate func(*appmap.SKDraft)) []byte {
	t.Helper()
	d := appmap.SKDraft{Status: status, Label: "PROPOSAL", Gaps: []appmap.HowGap{}, Source: "test:e2e/orders.spec.ts#ships",
		ProposedPath: "e2e/how-0123456789ab.spec.ts", GuardLine: 3, Lines: lines, Evidence: []appmap.SKDraftLine{},
		Digest: digestOf([]byte(strings.Join(lines, "\n") + "\n"))}
	if mutate != nil {
		mutate(&d)
	}
	data, err := json.Marshal(map[string]any{"schema": appmap.HowSchema, "map_revision": strings.Repeat("a", 40), "draft": d})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func candidate(t *testing.T) ([]byte, *Transform) {
	t.Helper()
	c, tr, err := Candidate(answerFor(t, appmap.HowComplete, draftLines(), nil))
	if err != nil || c == nil {
		t.Fatalf("Candidate = %v, %v", c, err)
	}
	return c, tr
}

// passing is evidence on which every condition but the prover contract and the witness amendment
// holds: the starting point each negative case perturbs.
func passing(tr *Transform) *Evidence {
	return &Evidence{Schema: EvidenceSchema, CandidateSHA256: tr.CandidateSHA, AppRevision: strings.Repeat("b", 40),
		Runs:       []Run{{Status: "passed"}, {Status: "passed"}, {Status: "passed"}},
		Validity:   Validity{Examples: []Example{{Status: "LOCATED", Blob: tr.CandidateBlob, Line: 2, Hygiene: "ELIGIBLE"}}, Defects: []string{}},
		Assessment: map[string]string{"association": "OBSERVED", "hygiene": "OBSERVED", "freshness": "OBSERVED", "execution": "OBSERVED", "cleanup": "OBSERVED"},
		Bindings:   []Binding{{Element: "step:3", Kind: "step", Witness: "trace:3"}},
		Controls:   []Control{{ID: "ctl-1", Result: "killed"}},
		Verdict:    Verdict{Verdict: "DISCRIMINATES", Mutants: []string{"mut-1"}},
		Assertion:  &Oracle{Screen: "/orders/{id}", Locator: "text=Shipped", Matcher: "toBeVisible", Expected: ""},
	}
}

func accepted() *Oracle {
	return &Oracle{Schema: OracleSchema, Origin: "owner-request-set", Commit: strings.Repeat("c", 40),
		Screen: "/orders/{id}", Locator: "text=Shipped", Matcher: "toBeVisible", Expected: ""}
}

func withProver(t *testing.T) {
	t.Helper()
	proverAccepted = true
	t.Cleanup(func() { proverAccepted = false })
}

func wantBlocked(t *testing.T, name string, r Receipt, code string) {
	t.Helper()
	if r.Status != Blocked || r.Code != code {
		t.Errorf("%s: receipt %s %s (%s), want BLOCKED %s", name, r.Status, r.Code, r.Detail, code)
	}
}

// SKH-V0-164: the one transform removes exactly the guard line and records the draft digest,
// the removed line's digest and the candidate's SHA-256 and Git blob.
func TestSKHV0_164_TransformRemovesOnlyTheGuard(t *testing.T) {
	c, tr := candidate(t)
	lines := draftLines()
	want := strings.Join(append(lines[:2:2], lines[3:]...), "\n") + "\n"
	if string(c) != want {
		t.Fatalf("candidate =\n%s\nwant\n%s", c, want)
	}
	if tr.Kind != TransformRemoveGuard || tr.Line != 3 || tr.RemovedSHA256 != digestOf([]byte(appmap.DraftGuardLine)) ||
		tr.CandidateSHA != digestOf(c) || tr.CandidateBlob != gitBlob(c) || tr.DraftDigest == "" || tr.SourceTest == "" {
		t.Fatalf("transform = %+v", tr)
	}
	if bytes.Contains(c, []byte("test.fixme(")) {
		t.Fatal("candidate keeps a fixme")
	}
	// gitBlob is the Git object ID: the empty blob is e69de29.
	if gitBlob(nil) != "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391" {
		t.Fatalf("gitBlob(empty) = %s", gitBlob(nil))
	}
}

// SKH-V0-165: a draft that is not COMPLETE has no candidate and reads generate-incomplete; a
// draft whose digest, guard line or fixme count does not verify is refused.
func TestSKHV0_165_DraftMustVerify(t *testing.T) {
	c, tr, err := Candidate(answerFor(t, appmap.HowIncomplete, draftLines(), nil))
	if err != nil || c != nil {
		t.Fatalf("partial draft = %v, %v", c, err)
	}
	wantBlocked(t, "partial", Assess(tr, nil, nil), CodeGenerateIncomplete)
	extra := append(draftLines(), "  test.fixme(true, 'x');")
	for name, data := range map[string][]byte{
		"digest":       answerFor(t, appmap.HowComplete, draftLines(), func(d *appmap.SKDraft) { d.Digest = strings.Repeat("0", 64) }),
		"guard line":   answerFor(t, appmap.HowComplete, draftLines(), func(d *appmap.SKDraft) { d.GuardLine = 2 }),
		"no guard":     answerFor(t, appmap.HowComplete, draftLines(), func(d *appmap.SKDraft) { d.GuardLine = 0 }),
		"second fixme": answerFor(t, appmap.HowComplete, extra, nil),
		"no draft":     []byte(`{"schema":"site-knowledge-how/0"}`),
		"other schema": []byte(`{"schema":"x","draft":{}}`),
	} {
		if _, _, err := Candidate(data); err == nil || !strings.HasPrefix(err.Error(), "skh-answer-invalid") {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

// SKH-V0-166: with no recorded live run the receipt blocks as infrastructure, and evidence for
// other bytes than the candidate voids the receipt.
func TestSKHV0_166_EvidenceBindsTheCandidateBytes(t *testing.T) {
	_, tr := candidate(t)
	r := Assess(tr, nil, accepted())
	wantBlocked(t, "no evidence", r, CodeInfrastructure)
	if r.Evidence != "none" || !strings.HasPrefix(r.Detail, "live-run-not-recorded") {
		t.Fatalf("no evidence receipt = %+v", r)
	}
	e := passing(tr)
	e.CandidateSHA256 = strings.Repeat("9", 64)
	wantBlocked(t, "other bytes", Assess(tr, e, accepted()), CodeInfrastructure)
	e = passing(tr)
	e.Infrastructure = "browser-crashed"
	wantBlocked(t, "infrastructure", Assess(tr, e, accepted()), CodeInfrastructure)
}

// SKH-V0-167: condition 1 holds only for 2 to 100 repeats that all passed with retries zero; a
// fixme or skipped run of the guarded draft is run-failed.
func TestSKHV0_167_RunsMustPassEveryRepeat(t *testing.T) {
	_, tr := candidate(t)
	for name, rs := range map[string][]Run{
		"one repeat": {{Status: "passed"}},
		"retried":    {{Status: "passed"}, {Status: "passed", Retries: 1}},
		"fixme":      {{Status: "passed"}, {Status: "fixme"}},
		"skipped":    {{Status: "skipped"}, {Status: "skipped"}},
		"failed":     {{Status: "passed"}, {Status: "failed"}},
		"too many":   make([]Run, 101),
	} {
		e := passing(tr)
		e.Runs = rs
		wantBlocked(t, name, Assess(tr, e, accepted()), CodeRunFailed)
	}
	e := passing(tr)
	e.AppRevision = ""
	wantBlocked(t, "no revision", Assess(tr, e, accepted()), CodeRunFailed)
}

// SKH-V0-168: condition 2 needs one LOCATED, ELIGIBLE example in the candidate's blob and no
// defect; a defect reads validity-defect and anything short of that validity-unestablished.
func TestSKHV0_168_ValidityOfTheCandidateBlob(t *testing.T) {
	_, tr := candidate(t)
	cases := map[string]func(*Validity){
		"uncertain":  func(v *Validity) { v.Uncertain = true },
		"none":       func(v *Validity) { v.Examples = nil },
		"two":        func(v *Validity) { v.Examples = append(v.Examples, v.Examples[0]) },
		"other blob": func(v *Validity) { v.Examples[0].Blob = strings.Repeat("1", 40) },
		"not found":  func(v *Validity) { v.Examples[0].Status = "NOT_FOUND" },
		"no line":    func(v *Validity) { v.Examples[0].Line = 0 },
		"hygiene":    func(v *Validity) { v.Examples[0].Hygiene = "INELIGIBLE" },
	}
	for name, mutate := range cases {
		e := passing(tr)
		mutate(&e.Validity)
		wantBlocked(t, name, Assess(tr, e, accepted()), CodeValidityUnknown)
	}
	e := passing(tr)
	e.Validity.Defects = []string{"ASSERTION_FREE"}
	wantBlocked(t, "defect", Assess(tr, e, accepted()), CodeValidityDefect)
}

// SKH-V0-169: condition 3 needs every NEA assessment axis OBSERVED; a missing or other axis is
// acceptance-blocked and the receipt names each blocked axis.
func TestSKHV0_169_AcceptanceNeedsEveryAxis(t *testing.T) {
	_, tr := candidate(t)
	e := passing(tr)
	delete(e.Assessment, "cleanup")
	e.Assessment["freshness"] = "UNKNOWN"
	r := Assess(tr, e, accepted())
	wantBlocked(t, "axes", r, CodeAcceptanceBlocked)
	if strings.Join(r.BlockedAxes, ";") != "freshness=UNKNOWN;cleanup=UNKNOWN" {
		t.Fatalf("blocked axes = %v", r.BlockedAxes)
	}
}

// SKH-V0-170: condition 5 reads binding-unverified for every selector or method element until
// the PWP-V0, RVN-V0 and AMSP-V0 witness amendments land, and for a step with no witness.
func TestSKHV0_170_SelectorsAndMethodsStayUnverified(t *testing.T) {
	_, tr := candidate(t)
	for name, b := range map[string][]Binding{
		"selector":     {{Element: "role=button name=Ship", Kind: "selector", Witness: "dom:1"}},
		"method":       {{Element: "OrdersPage.ship", Kind: "method", Witness: "trace:2"}},
		"bare step":    {{Element: "step:3", Kind: "step"}},
		"unknown kind": {{Element: "x", Kind: "fixture", Witness: "w"}},
		"no bindings":  nil,
	} {
		e := passing(tr)
		e.Bindings = b
		r := Assess(tr, e, accepted())
		wantBlocked(t, name, r, CodeBindingUnverified)
		if len(r.Unverified) != 1 {
			t.Errorf("%s: unverified = %v", name, r.Unverified)
		}
	}
}

// SKH-V0-171: condition 4 needs at least one approved control and every control killed.
func TestSKHV0_171_EveryControlKilled(t *testing.T) {
	_, tr := candidate(t)
	for name, cs := range map[string][]Control{
		"none":     nil,
		"survived": {{ID: "ctl-1", Result: "killed"}, {ID: "ctl-2", Result: "survived"}},
		"unknown":  {{ID: "ctl-1"}},
	} {
		e := passing(tr)
		e.Controls = cs
		wantBlocked(t, name, Assess(tr, e, accepted()), CodeControlSurvived)
	}
}

// SKH-V0-172: the prover verdict maps by the SKH-V0-085 allowlist, a passing verdict must name
// its mutants, and until the V1-1733 prover contract is accepted nothing passes condition 4.
func TestSKHV0_172_VerdictLadder(t *testing.T) {
	_, tr := candidate(t)
	e := passing(tr)
	r := Assess(tr, e, accepted())
	wantBlocked(t, "prover unaccepted", r, CodeDiscriminationUnk)
	if r.ProverAccepted || !strings.Contains(r.Detail, "V1-1733") {
		t.Fatalf("unaccepted prover receipt = %+v", r)
	}
	withProver(t)
	for v, code := range map[string]string{
		"PRESERVES": CodeNotDiscriminating, "CONTROL_SURVIVED": CodeControlSurvived,
		"REGRESSION_DETECTED": CodeRunFailed, "SCENARIO_BROKEN": CodeRunFailed,
		"INCONCLUSIVE": CodeDiscriminationUnk, "UNSTABLE": CodeDiscriminationUnk, "": CodeDiscriminationUnk,
	} {
		e := passing(tr)
		e.Verdict.Verdict = v
		wantBlocked(t, "verdict "+v, Assess(tr, e, accepted()), code)
	}
	e = passing(tr)
	e.Verdict.Mutants = nil
	wantBlocked(t, "no mutants", Assess(tr, e, accepted()), CodeDiscriminationUnk)
	e = passing(tr)
	e.Verdict.Verdict = "NEW_REACH"
	if r := Assess(tr, e, accepted()); r.Status != Proven {
		t.Fatalf("NEW_REACH = %+v", r)
	}
}

// SKH-V0-173: condition 6 compares the outcome assertion with an accepted oracle on screen,
// locator, matcher and expected value; otherwise conditions 1-5 read EXECUTION_VERIFIED with the
// oracle code, and PROVEN needs all six.
func TestSKHV0_173_OracleSeparatesProvenFromExecution(t *testing.T) {
	withProver(t)
	_, tr := candidate(t)
	if r := Assess(tr, passing(tr), accepted()); r.Status != Proven || r.Code != "" || len(r.Conditions) != 6 {
		t.Fatalf("all six = %+v", r)
	}
	learned := accepted()
	learned.Origin = "learned"
	uncommitted := accepted()
	uncommitted.Commit = ""
	for name, o := range map[string]*Oracle{"none": nil, "learned": learned, "uncommitted": uncommitted} {
		if r := Assess(tr, passing(tr), o); r.Status != ExecutionVerified || r.OracleCode != CodeOracleUnaccepted {
			t.Errorf("%s oracle = %+v", name, r)
		}
	}
	for _, field := range []string{"screen", "locator", "matcher", "expected", "missing"} {
		e := passing(tr)
		switch field {
		case "screen":
			e.Assertion.Screen = "/orders"
		case "locator":
			e.Assertion.Locator = "text=Ship"
		case "matcher":
			e.Assertion.Matcher = "toHaveText"
		case "expected":
			e.Assertion.Expected = "x"
		case "missing":
			e.Assertion = nil
		}
		if r := Assess(tr, e, accepted()); r.Status != ExecutionVerified || r.OracleCode != CodeOracleMismatch {
			t.Errorf("%s mismatch = %+v", field, r)
		}
	}
}

// SKH-V0-175: recorded evidence and oracles decode strictly: a closed schema, no unknown field
// and bounded lists.
func TestSKHV0_175_StrictRecordedInputs(t *testing.T) {
	_, tr := candidate(t)
	good, _ := json.Marshal(passing(tr))
	if _, err := DecodeEvidence(good); err != nil {
		t.Fatalf("good evidence: %v", err)
	}
	long := passing(tr)
	long.Controls = make([]Control, maxRecordedListElements+1)
	big, _ := json.Marshal(long)
	for name, data := range map[string][]byte{
		"unknown field": bytes.Replace(good, []byte(`{"schema"`), []byte(`{"extra":1,"schema"`), 1),
		"schema":        bytes.Replace(good, []byte(EvidenceSchema), []byte("x/0"), 1),
		"long list":     big,
		"not json":      []byte("{"),
	} {
		if _, err := DecodeEvidence(data); err == nil {
			t.Errorf("%s evidence accepted", name)
		}
	}
	o, _ := json.Marshal(accepted())
	if _, err := DecodeOracle(o); err != nil {
		t.Fatalf("good oracle: %v", err)
	}
	if _, err := DecodeOracle(bytes.Replace(o, []byte(`{"schema"`), []byte(`{"weight":1,"schema"`), 1)); err == nil {
		t.Fatal("oracle with unknown field accepted")
	}
}

// SKH-V0-176: the receipt is canonical bytes naming the recorded evidence origin, and the same
// inputs give the same bytes.
func TestSKHV0_176_ReceiptIsStable(t *testing.T) {
	_, tr := candidate(t)
	a := Assess(tr, passing(tr), accepted()).Encode()
	b := Assess(tr, passing(tr), accepted()).Encode()
	if !bytes.Equal(a, b) || !bytes.HasSuffix(a, []byte("}\n")) {
		t.Fatalf("receipt not stable:\n%s\n%s", a, b)
	}
	var doc map[string]any
	if err := json.Unmarshal(a, &doc); err != nil || doc["schema"] != ReceiptSchema || doc["evidence"] != "recorded" {
		t.Fatalf("receipt = %s (%v)", a, err)
	}
	for _, key := range []string{"transform", "conditions", "blocked_axes", "unverified", "mutants", "controls"} {
		if doc[key] == nil {
			t.Errorf("receipt omits %s", key)
		}
	}
}
