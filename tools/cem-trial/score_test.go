package main

import (
	"strconv"
	"strings"
	"testing"
)

func TestMissedEvidenceVerdictIntervalCompatibility(t *testing.T) {
	tests := []struct {
		name                       string
		delta, relative, low, high float64
		controlMiss                float64
		want                       string
	}{
		{"met", 0.1, 0.2, 0.01, 0.15, 0.5, "met: at least 20% fewer missed-evidence findings"},
		{"treatment missed more", -0.05, -0.1, -0.08, -0.01, 0.5, "not met: treatment missed more evidence than control"},
		{"below target excludes zero", 0.04, 0.05, 0.03, 0.06, 0.8, "below target but interval excludes 0"},
		{"covers zero and absolute target", 0.05, 0.1, -0.01, 0.12, 0.5, "consistent with 20% and with 0"},
		{"covers zero excludes target", 0.018, 0.02, 0, 0.06, 0.9, "consistent with 0 but not with 20%"},
		{"includes both endpoints", 0.05, 0.1, 0, 0.1, 0.5, "consistent with 20% and with 0"},
		{"zero upper bound", -0.01, -0.02, -0.03, 0, 0.5, "consistent with 0 but not with 20%"},
	}
	for _, tt := range tests {
		t.Run("CRT-V0-008/"+tt.name, func(t *testing.T) {
			summary := map[string]any{"citable": map[string]any{}, "verifier": map[string]any{}}
			got := verdicts(tt.delta, tt.relative, tt.low, tt.high, tt.controlMiss, summary)["missed_evidence"]
			if got != tt.want {
				t.Fatalf("missed_evidence = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCitableAndVerifierSkipNonZeroExitLanes(t *testing.T) {
	t.Run("CRT-V0-007/non-zero exit without error text", func(t *testing.T) {
		counts := map[string]any{"total": 4.0, "mechanical": 0.0, "supported": 4.0}
		document := &report{Lanes: []*lane{
			{Arm: "treatment", ExitCode: 0, Citations: []citation{{}}, CEM: map[string]any{"counts": counts, "verify_ok": true}},
			{Arm: "treatment", ExitCode: 1, Citations: []citation{{}}, CEM: map[string]any{"counts": map[string]any{"total": 4.0, "mechanical": 0.0, "supported": 0.0}, "verify_ok": false, "replay_ok": true}},
		}}
		if got := citable(document); got["lanes"] != 1 || got["fraction"] != 1.0 {
			t.Fatalf("citable = %v, want the exit-1 lane dropped", got)
		}
		if got := verifierRates(document); got["citing_lanes"] != 1 || got["incorrect_hard_failures"] != 0 {
			t.Fatalf("verifier = %v, want the exit-1 lane dropped", got)
		}
	})
}

func TestSinglePairLeavesTheIntervalUnestimated(t *testing.T) {
	t.Run("CRT-V0-008/one scored pair", func(t *testing.T) {
		document := syntheticReport()
		document.Lanes = document.Lanes[:2]
		document.Changes = document.Changes[:1]
		finalize(document)
		if got := document.Summary["delta_ci95"]; got != notObserved {
			t.Fatalf("delta_ci95 = %v, want %s", got, notObserved)
		}
		if got := document.Summary["verdict"].(map[string]any)["missed_evidence"]; got != notEstimable {
			t.Fatalf("missed_evidence = %q, want %q", got, notEstimable)
		}
	})
}

// citeEverySuggestion is the scripted agent decision 0523 P4 names: it cites
// every path the seeded prologue suggested, each over a wide range, and
// declares nothing unknown.
func citeEverySuggestion(suggested []string) string {
	block := `{"citations":[`
	for index, path := range suggested {
		if index > 0 {
			block += ","
		}
		block += `{"hunk":"1","path":"` + path + `","lines":"1:40","relation":"test-claim","confidence":"certain"}`
	}
	return "done\n\n```json\n" + block + `],"unknown":[]}` + "\n```\n"
}

// CRT-V0-013: an arm that cites every one of up to eight suggestions per file
// reaches miss 0 at no cost under miss alone; the cited-not-gold rate at path
// and span level is where it loses to an arm that cites only what it can
// justify.
func TestCiteEverySuggestionAgentLosesOnCitedNotGold(t *testing.T) {
	arms := []string{"control", "seeded"}
	changes := []changeMeta{}
	lanes := []*lane{}
	for _, id := range []string{"c01", "c02"} {
		goldPaths := []string{id + "/a_test.go", id + "/b_test.go"}
		suggested := append([]string{}, goldPaths...)
		for index := 0; index < 6; index++ {
			suggested = append(suggested, id+"/other"+strconv.Itoa(index)+".go")
		}
		changes = append(changes, changeMeta{ID: id, Gold: goldPaths, StemBaseline: []string{},
			GoldSpans: []span{{Path: goldPaths[0], Lines: "10:20"}, {Path: goldPaths[1], Lines: "5:9"}}})
		careful := strings.Replace(reply(goldPaths[0]), `"lines":"1:2"`, `"lines":"10:20"`, 1)
		lanes = append(lanes,
			&lane{ID: id, Arm: "control", Repeat: 1, ArmOrder: arms, Reply: careful, WallMs: int64(1), Tokens: notObserved, ExitCode: 0},
			&lane{ID: id, Arm: "seeded", Repeat: 1, ArmOrder: arms, Reply: citeEverySuggestion(suggested), Suggested: suggested, WallMs: int64(1), Tokens: notObserved, ExitCode: 0},
		)
	}
	document := &report{Profile: profile, Partition: "development", Seed: "fixture", Arms: arms, Repeats: 1, Pairs: 2, Changes: changes, Lanes: lanes}
	finalize(document)
	for _, item := range document.Lanes {
		want := citedNotGold{Paths: 0.0, Spans: 0.0}
		if item.Arm == "seeded" {
			// 6 of 8 paths are not gold; 320 cited lines, of which 11 + 5 are gold.
			want = citedNotGold{Paths: 0.75, Spans: round(304.0 / 320.0)}
		}
		if item.CitedNotGold != want {
			t.Fatalf("%s/%s cited_not_gold = %+v, want %+v", item.ID, item.Arm, item.CitedNotGold, want)
		}
	}
	block := document.Summary["arms"].(map[string]any)["seeded"].(map[string]any)
	if block["delta"] != 0.5 {
		t.Fatalf("the cite-everything arm should win on miss alone: %v", block)
	}
	rates := block["cited_not_gold"].(map[string]any)
	seeded, control := rates["seeded"].(map[string]any), rates["control"].(map[string]any)
	if !(seeded["paths"].(float64) > control["paths"].(float64)) || !(seeded["spans"].(float64) > control["spans"].(float64)) {
		t.Fatalf("the cite-everything arm did not lose on cited-not-gold: %v", rates)
	}
	if seeded["lanes"] != 2 || control["span_lanes"] != 2 {
		t.Fatalf("cited_not_gold lane counts = %v", rates)
	}
}

// CRT-V0-013: the lane rate counts distinct paths and distinct lines, scores
// a range it cannot place as outside the gold, and reports NOT_OBSERVED for a
// lane that cited nothing or a report that recorded no span gold.
func TestCitedNotGoldCountsDistinctPathsAndLines(t *testing.T) {
	meta := changeMeta{Gold: []string{"a_test.go"}, GoldSpans: []span{{Path: "a_test.go", Lines: "1:10"}, {Path: "other.go", Lines: "1:10"}}}
	citations := []citation{
		{Path: "./a_test.go", Lines: "1:5"}, {Path: "a_test.go", Lines: "3:8"}, // lines 1..8, all gold
		{Path: "src.go", Lines: "1:1"},    // one line, not gold
		{Path: "src.go", Lines: "bad"},    // unplaceable: one line outside
		{Path: "src.go", Lines: "0:1000"}, // starts before line 1: one line outside
	}
	got := scoreCitedNotGold(citations, meta)
	if got.Paths != 0.5 || got.Spans != round(3.0/11.0) {
		t.Fatalf("cited_not_gold = %+v", got)
	}
	if got := scoreCitedNotGold(nil, meta); got.Paths != notObserved || got.Spans != notObserved {
		t.Fatalf("no citation = %+v", got)
	}
	meta.GoldSpans = nil
	if got := scoreCitedNotGold(citations, meta); got.Paths != 0.5 || got.Spans != notObserved {
		t.Fatalf("no recorded span gold = %+v", got)
	}
}
