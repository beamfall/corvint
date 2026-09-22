package evalrepo

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCaseSplitIsAHashOfTheRowIDNotItsPosition(t *testing.T) {
	for id, want := range map[string]string{"flask-ensure-sync": SplitHeldout, "flask-load-dotenv": SplitDev, "zod-treeify-errors": SplitHeldout, "exact-feature-pairing": SplitDev} {
		if got := CaseSplit(id); got != want {
			t.Fatalf("%s split=%s want=%s", id, got, want)
		}
	}
	forward, err := SplitRows([]byte(`{"cases":[{"id":"flask-ensure-sync"},{"id":"flask-load-dotenv"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	reversed, err := SplitRows([]byte(`{"cases":[{"id":"flask-load-dotenv"},{"id":"flask-ensure-sync"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if forward[0] != reversed[1] || forward[1] != reversed[0] {
		t.Fatalf("forward=%v reversed=%v", forward, reversed)
	}
}

func TestSplitManifestRefusesDriftedCorpus(t *testing.T) {
	corpus := `{"cases":[{"id":"flask-ensure-sync","query":"a"},{"id":"flask-load-dotenv","query":"b"}]}`
	manifest, err := NewSplitManifest(map[string][]byte{"cases.json": []byte(corpus)})
	if err != nil {
		t.Fatal(err)
	}
	if registered, err := manifest.VerifyCorpus("cases.json", []byte(corpus)); !registered || err != nil {
		t.Fatalf("unchanged registered=%v err=%v", registered, err)
	}
	for name, drifted := range map[string]string{
		"edited row":  strings.Replace(corpus, `"query":"b"`, `"query":"c"`, 1),
		"renamed id":  strings.Replace(corpus, "flask-load-dotenv", "flask-load-env", 1),
		"missing row": `{"cases":[{"id":"flask-ensure-sync","query":"a"}]}`,
	} {
		if _, err := manifest.VerifyCorpus("cases.json", []byte(drifted)); err == nil {
			t.Fatalf("%s was accepted", name)
		}
	}
	if registered, err := manifest.VerifyCorpus("other.json", []byte(corpus)); registered || err != nil {
		t.Fatalf("unregistered registered=%v err=%v", registered, err)
	}
}

func TestTuningPurposeRefusesHeldOutRows(t *testing.T) {
	cases := []goldenCase{{ID: "flask-load-dotenv", Split: SplitDev}, {ID: "flask-ensure-sync", Split: SplitHeldout}}
	readable, withheld, err := casesForPurpose(cases, PurposeTune)
	if err != nil || withheld != 1 || len(readable) != 1 || readable[0].ID != "flask-load-dotenv" {
		t.Fatalf("readable=%v withheld=%d err=%v", readable, withheld, err)
	}
	_, err = evaluateCases(context.Background(), t.TempDir(), nil, cases, "", "", nil, PurposeTune)
	var refusal ErrHeldoutRowInTuning
	if !errors.As(err, &refusal) || refusal.ID != "flask-ensure-sync" {
		t.Fatalf("tuning over a held-out row err=%v", err)
	}
	if _, _, err := casesForPurpose(cases, Purpose("calibrate")); err == nil {
		t.Fatal("unknown purpose was accepted")
	}
}

func TestBudgetYieldCurveAreaAndMerge(t *testing.T) {
	scores := []caseScore{
		{split: SplitDev, mustHit: 2, mustTotal: 3, mustHitBytes: []int{300, 3000}},
		{split: SplitHeldout, mustHit: 1, mustTotal: 1, mustHitBytes: []int{600}},
	}
	dev := splitMetrics(scores, SplitDev)
	yield := dev["budget_yield"].(map[string]any)
	wantHits := []int{1, 1, 1, 2, 2, 2}
	for position, want := range wantHits {
		if yield["hits_at_budget"].([]any)[position] != want {
			t.Fatalf("hits=%v want=%v", yield["hits_at_budget"], wantHits)
		}
	}
	if yield["recall_at_budget"].([]any)[5] != dev["recall"] || yield["area"] != 0.5 {
		t.Fatalf("dev metrics=%v", dev)
	}
	if empty := splitMetrics(nil, SplitHeldout); empty["recall"] != nil || empty["budget_yield"].(map[string]any)["area"] != nil {
		t.Fatalf("empty split=%v", empty)
	}
	full := splitMetrics(scores, "")
	merged := MergeSplitMetrics([]map[string]any{dev, splitMetrics(scores, SplitHeldout)})
	if full["recall"] != 0.75 || merged["recall"] != full["recall"] || merged["budget_yield"].(map[string]any)["area"] != full["budget_yield"].(map[string]any)["area"] {
		t.Fatalf("full=%v merged=%v", full, merged)
	}
}

func TestDownstreamOutcomeSlotValidation(t *testing.T) {
	for _, valid := range []any{NotRecordedOutcome(), map[string]any{"state": "recorded", "outcome": "failed", "evidence": "ci run 12"}} {
		if err := ValidateDownstreamOutcome(valid); err != nil {
			t.Fatalf("%v: %v", valid, err)
		}
	}
	for _, invalid := range []any{nil, map[string]any{}, map[string]any{"state": "unknown"},
		map[string]any{"state": OutcomeNotRecorded, "outcome": "passed"},
		map[string]any{"state": "recorded", "outcome": "passed"},
		map[string]any{"state": "recorded", "outcome": "maybe", "evidence": "x"}} {
		if err := ValidateDownstreamOutcome(invalid); err == nil {
			t.Fatalf("%v was accepted", invalid)
		}
	}
}
