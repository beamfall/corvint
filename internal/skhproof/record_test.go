package skhproof

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appmap"
)

var (
	recHead = strings.Repeat("d", 40)
	recBase = strings.Repeat("e", 40)
)

// recordAnswer is the COMPLETE draft answer with a goal screen, one oracle and page-object steps.
func recordAnswer(t *testing.T) ([]byte, *Transform, []byte) {
	t.Helper()
	answer := answerFor(t, appmap.HowComplete, draftLines(), nil)
	var doc map[string]any
	if err := json.Unmarshal(answer, &doc); err != nil {
		t.Fatal(err)
	}
	doc["goal_screen"] = map[string]any{"id": "screen:shop:orders"}
	doc["oracle"] = []any{map[string]any{"locator": "text=Shipped", "matcher": "toBeVisible", "expected": ""}}
	doc["steps"] = []any{
		map[string]any{"index": 1, "action": "visit /orders/1"},
		map[string]any{"index": 2, "action": "click role=button name=Ship", "via": "page-object:e2e/pages/order.page.ts#OrderPage.ship"},
	}
	answer, _ = json.Marshal(doc)
	c, tr, err := Candidate(answer)
	if err != nil {
		t.Fatal(err)
	}
	return answer, tr, c
}

// recordProve is a hosted e2e-prove/0 report whose candidate scenario passed every head repeat.
func recordProve(path string, mutate func(map[string]any, map[string]any)) []byte {
	outcomes := []any{}
	for range 3 {
		outcomes = append(outcomes, map[string]any{"state": "passed"})
	}
	head := map[string]any{"application_revision": recHead, "test_revision": recHead, "qualified": true,
		"stability": "DETERMINISTIC", "outcomes": outcomes}
	scene := map[string]any{"placement": path, "hosted": true, "verdict": "DISCRIMINATES", "head": head,
		"base":     map[string]any{"application_revision": recBase, "test_revision": recHead, "qualified": true},
		"controls": []any{map[string]any{"id": "control:omitted-event:step-1", "status": "killed"}}}
	doc := map[string]any{"schema": ProveSchema, "head": recHead, "retries": 0, "scenarios": []any{scene}}
	if mutate != nil {
		mutate(doc, scene)
	}
	data, _ := json.Marshal(doc)
	return data
}

func recordValidityReport(path string, findings []any, strength bool) []byte {
	test := map[string]any{"test": path + ":2", "hygiene": map[string]any{"state": "ELIGIBLE"}}
	if strength {
		test["strength"] = map[string]any{"state": "NOT_MEASURED"}
	}
	data, _ := json.Marshal(map[string]any{"schema": ValiditySchema, "revision": recHead,
		"files": []any{map[string]any{"path": path, "uncertain": false, "findings": findings}}, "tests": []any{test}})
	return data
}

func readerOf(path string, data []byte) ReadFile {
	return func(rev, p string) ([]byte, error) {
		if rev == recHead && p == path {
			return data, nil
		}
		return nil, errors.New("no such blob")
	}
}

// SKH-V0-180: a passing hosted run records its runs, controls, base mutant and the committed
// candidate's digest, leaves every axis but execution unrecorded, and Assess then blocks on those
// axes rather than on infrastructure.
func TestSKHV0_180_RecordCopiesObservedFactsOnly(t *testing.T) {
	answer, tr, c := recordAnswer(t)
	e, err := Record(answer, recordProve(tr.ProposedPath, nil), recordValidityReport(tr.ProposedPath, nil, true), readerOf(tr.ProposedPath, c))
	if err != nil {
		t.Fatal(err)
	}
	if e.Infrastructure != "" || e.CandidateSHA256 != tr.CandidateSHA || e.AppRevision != recHead || len(e.Runs) != 3 {
		t.Fatalf("evidence = %+v", e)
	}
	if len(e.Assessment) != 2 || e.Assessment["execution"] != AxisObserved || e.Assessment["hygiene"] != AxisObserved {
		t.Errorf("assessment = %v, want execution and hygiene only (association has no observed source)", e.Assessment)
	}
	if !slices.Equal(e.Verdict.Mutants, []string{"base:" + recBase}) || len(e.Controls) != 1 || e.Controls[0].Result != "killed" {
		t.Errorf("verdict %+v controls %+v", e.Verdict, e.Controls)
	}
	round, err := DecodeEvidence(e.Encode())
	if err != nil {
		t.Fatalf("recorded evidence does not decode: %v", err)
	}
	r := Assess(tr, round, nil)
	wantBlocked(t, "recorded", r, CodeAcceptanceBlocked)
	if !strings.Contains(r.Detail, "association=UNKNOWN") || strings.Contains(r.Detail, "execution") {
		t.Errorf("detail = %q", r.Detail)
	}

	// A retry or a non-passing repeat leaves execution unrecorded too.
	e, _ = Record(answer, recordProve(tr.ProposedPath, func(doc, _ map[string]any) { doc["retries"] = 1 }),
		recordValidityReport(tr.ProposedPath, nil, true), readerOf(tr.ProposedPath, c))
	if _, ok := e.Assessment["execution"]; ok {
		t.Errorf("retries 1 recorded execution")
	}
}

// SKH-V0-180: a run the prover did not qualify, or a candidate it cannot read back, is
// infrastructure; a non-COMPLETE draft or a foreign report is refused.
func TestSKHV0_180_UnqualifiedRunIsInfrastructure(t *testing.T) {
	answer, tr, c := recordAnswer(t)
	validity := recordValidityReport(tr.ProposedPath, nil, true)
	cases := map[string]struct {
		prove []byte
		read  ReadFile
		want  string
	}{
		"missing":  {recordProve("e2e/other.spec.ts", nil), readerOf(tr.ProposedPath, c), "prove-scenario-missing"},
		"unhosted": {recordProve(tr.ProposedPath, func(_, s map[string]any) { s["hosted"] = false }), readerOf(tr.ProposedPath, c), "prove-scenario-missing"},
		"unqualified": {recordProve(tr.ProposedPath, func(_, s map[string]any) {
			s["head"].(map[string]any)["qualified"] = false
			s["head"].(map[string]any)["reason"] = "app-start-failed"
		}), readerOf(tr.ProposedPath, c), "head-receipt-unqualified: app-start-failed"},
		"unpinned":   {recordProve(tr.ProposedPath, func(doc, _ map[string]any) { doc["head"] = recBase }), readerOf(tr.ProposedPath, c), "head-revision-unpinned"},
		"unreadable": {recordProve(tr.ProposedPath, nil), readerOf("e2e/other.spec.ts", c), "candidate-unreadable-at-head"},
	}
	for name, tc := range cases {
		e, err := Record(answer, tc.prove, validity, tc.read)
		if err != nil || !strings.HasPrefix(e.Infrastructure, tc.want) {
			t.Errorf("%s: Record = %+v, %v; want infrastructure %s", name, e, err, tc.want)
			continue
		}
		round, err := DecodeEvidence(e.Encode())
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		wantBlocked(t, name, Assess(tr, round, nil), CodeInfrastructure)
	}

	pending := answerFor(t, appmap.HowAbstained, draftLines(), nil)
	if _, err := Record(pending, recordProve(tr.ProposedPath, nil), validity, readerOf(tr.ProposedPath, c)); err == nil {
		t.Errorf("an abstained draft was recorded")
	}
	if _, err := Record(answer, []byte(`{"schema":"other/0"}`), validity, readerOf(tr.ProposedPath, c)); !isRefusal(err, "skh-evidence-invalid") {
		t.Errorf("foreign prove report: %v", err)
	}
	if _, err := Record(answer, recordProve(tr.ProposedPath, nil), []byte(`{}`), readerOf(tr.ProposedPath, c)); !isRefusal(err, "skh-evidence-invalid") {
		t.Errorf("foreign validity report: %v", err)
	}
}

// SKH-V0-181: the validity reading names the committed blob, LOCATED only with a strength axis,
// and every invalid finding as a defect.
func TestSKHV0_181_RecordValidity(t *testing.T) {
	answer, tr, c := recordAnswer(t)
	prove := recordProve(tr.ProposedPath, nil)
	e, _ := Record(answer, prove, recordValidityReport(tr.ProposedPath, nil, true), readerOf(tr.ProposedPath, c))
	want := Example{Status: "LOCATED", Blob: tr.CandidateBlob, Line: 2, Hygiene: "ELIGIBLE"}
	if len(e.Validity.Examples) != 1 || e.Validity.Examples[0] != want || len(e.Validity.Defects) != 0 {
		t.Fatalf("validity = %+v", e.Validity)
	}
	findings := []any{map[string]any{"rule": "JS-NO-ASSERTION", "severity": "invalid"}, map[string]any{"rule": "JS-ADVICE", "severity": "advisory"}}
	e, _ = Record(answer, prove, recordValidityReport(tr.ProposedPath, findings, true), readerOf(tr.ProposedPath, c))
	if !slices.Equal(e.Validity.Defects, []string{"JS-NO-ASSERTION"}) {
		t.Errorf("validity = %+v", e.Validity)
	}
	if r := Assess(tr, e, nil); r.Code != CodeValidityDefect {
		t.Errorf("defective candidate assessed %s", r.Code)
	}
	e, _ = Record(answer, prove, recordValidityReport(tr.ProposedPath, nil, false), readerOf(tr.ProposedPath, c))
	if e.Validity.Examples[0].Status != "ELIGIBLE" {
		t.Errorf("unlocated example recorded %+v", e.Validity.Examples[0])
	}
	if r := Assess(tr, e, nil); r.Code != CodeValidityUnknown {
		t.Errorf("unlocated candidate assessed %s", r.Code)
	}
}

// SKH-V0-182, SKH-V0-183: every step and page-object method is recorded without a witness, and
// the assertion is the goal screen with the one oracle.
func TestSKHV0_182_BindingsUnwitnessedAndAssertion(t *testing.T) {
	answer, tr, c := recordAnswer(t)
	e, _ := Record(answer, recordProve(tr.ProposedPath, nil), recordValidityReport(tr.ProposedPath, nil, true), readerOf(tr.ProposedPath, c))
	want := []Binding{{Element: "step:1 visit /orders/1", Kind: "step"}, {Element: "step:2 click role=button name=Ship", Kind: "step"},
		{Element: "e2e/pages/order.page.ts#OrderPage.ship", Kind: "method"}}
	if !slices.Equal(e.Bindings, want) {
		t.Errorf("bindings = %+v", e.Bindings)
	}
	if e.Assertion == nil || *e.Assertion != (Oracle{Screen: "screen:shop:orders", Locator: "text=Shipped", Matcher: "toBeVisible"}) {
		t.Errorf("assertion = %+v", e.Assertion)
	}
}

// SKH-V0-186, SKH-V0-187: hygiene reads OBSERVED only for the ELIGIBLE state of the one example in
// a certain file whose bytes the run executed; association, freshness and cleanup are never
// recorded, even for a hosted obligation at the candidate path with one outcome per repeat.
func TestSKHV0_186_HygieneSourceOnly(t *testing.T) {
	answer, tr, c := recordAnswer(t)
	bound := func(doc, s map[string]any) {
		doc["repeats"] = 3
		s["obligation"] = "hosted:" + tr.ProposedPath + "::ships the order"
	}
	validity := recordValidityReport(tr.ProposedPath, nil, true)
	e, err := Record(answer, recordProve(tr.ProposedPath, bound), validity, readerOf(tr.ProposedPath, c))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"execution": AxisObserved, "hygiene": AxisObserved}
	if len(e.Assessment) != len(want) || e.Assessment["execution"] != AxisObserved || e.Assessment["hygiene"] != AxisObserved {
		t.Fatalf("assessment = %v, want %v", e.Assessment, want)
	}
	r := Assess(tr, e, nil)
	wantBlocked(t, "bound", r, CodeAcceptanceBlocked)
	if r.Detail != "association=UNKNOWN, freshness=UNKNOWN, cleanup=UNKNOWN" {
		t.Errorf("detail = %q, want association, freshness and cleanup blocked", r.Detail)
	}

	twoExamples := func() []byte {
		var doc map[string]any
		_ = json.Unmarshal(validity, &doc)
		doc["tests"] = append(doc["tests"].([]any), map[string]any{"test": tr.ProposedPath + ":9", "hygiene": map[string]any{"state": "ELIGIBLE"}})
		data, _ := json.Marshal(doc)
		return data
	}
	withHygiene := func(state string, uncertain bool) []byte {
		var doc map[string]any
		_ = json.Unmarshal(validity, &doc)
		doc["tests"].([]any)[0].(map[string]any)["hygiene"] = map[string]any{"state": state}
		doc["files"].([]any)[0].(map[string]any)["uncertain"] = uncertain
		data, _ := json.Marshal(doc)
		return data
	}
	otherBytes := func(rev, p string) ([]byte, error) {
		if rev == recHead && p == tr.ProposedPath {
			return c, nil
		}
		return []byte("other bytes\n"), nil
	}
	cases := map[string]struct {
		prove, validity []byte
		read            ReadFile
		hygiene         bool
	}{
		"no obligation":  {recordProve(tr.ProposedPath, nil), validity, readerOf(tr.ProposedPath, c), true},
		"two examples":   {recordProve(tr.ProposedPath, bound), twoExamples(), readerOf(tr.ProposedPath, c), false},
		"ineligible":     {recordProve(tr.ProposedPath, bound), withHygiene("INELIGIBLE", false), readerOf(tr.ProposedPath, c), false},
		"uncertain file": {recordProve(tr.ProposedPath, bound), withHygiene("ELIGIBLE", true), readerOf(tr.ProposedPath, c), false},
	}
	for name, tc := range cases {
		e, err := Record(answer, tc.prove, tc.validity, tc.read)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, ok := e.Assessment["association"]; ok {
			t.Errorf("%s: association recorded", name)
		}
		if got := e.Assessment["hygiene"] == AxisObserved; got != tc.hygiene {
			t.Errorf("%s: hygiene observed = %v, want %v", name, got, tc.hygiene)
		}
		if _, ok := e.Assessment["freshness"]; ok {
			t.Errorf("%s: freshness recorded", name)
		}
		if _, ok := e.Assessment["cleanup"]; ok {
			t.Errorf("%s: cleanup recorded", name)
		}
	}

	// A validity report read at another revision whose bytes differ from the run's records neither.
	var doc map[string]any
	_ = json.Unmarshal(validity, &doc)
	doc["revision"] = recBase
	stale, _ := json.Marshal(doc)
	e, _ = Record(answer, recordProve(tr.ProposedPath, bound), stale, otherBytes)
	if e.Assessment["association"] != "" || e.Assessment["hygiene"] != "" {
		t.Errorf("stale validity reading recorded %v", e.Assessment)
	}
}

func isRefusal(err error, code string) bool {
	var r *Refusal
	return errors.As(err, &r) && r.Code == code
}
