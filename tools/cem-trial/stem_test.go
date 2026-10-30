package main

import (
	"fmt"
	"strings"
	"testing"
)

// CRT-V0-017: the stem-worklist arm is the control prompt plus one declared
// block listing the recorded stem guesses; it names no Corvint artefact and
// runs without a Corvint binary.
func TestCRTV0017StemWorklistArmCarriesOnlyTheStemGuesses(t *testing.T) {
	item := task{ID: "c01", StemBaseline: []string{"a_test.go", "test_b.py"}}
	patch := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1,2 +1,2 @@\n-x\n+y\n"
	control := buildPrompt("control", item, patch, map[string]string{})
	stem := buildPrompt("stem", item, patch, map[string]string{})
	block := fmt.Sprintf(stemPrologue, "- a_test.go\n- test_b.py")
	if !strings.Contains(stem, block) || strings.Replace(stem, block, "", 1) != control {
		t.Fatal("the stem prompt differs from control beyond the declared stem worklist")
	}
	for _, word := range []string{"Corvint", "corvint", "CEM", "Change Evidence Map"} {
		if strings.Contains(stem, word) {
			t.Fatalf("the stem prompt names %q", word)
		}
	}
	if got := stemWorklist(nil); got != "(none)" {
		t.Fatalf("empty worklist = %q", got)
	}
	if _, err := parseRunOptions([]string{"--tasks", "t.json", "--history", "h", "--arms", "control,stem", "--agent", "script", "--agent-command", "true"}); err != nil {
		t.Fatalf("the stem arm needs no --corvint: %v", err)
	}
	if identity := prologueIdentity([]string{"stem"}); identity["stem"]["sha256"] != digestOf(stemPrologue) {
		t.Fatalf("stem prologue identity = %v", identity)
	}
}

// CRT-V0-017: every arm, control included, is paired with the mechanical stem
// worklist, and the report says in plain words when nothing beat it.
func TestCRTV0017ReportSaysPlainlyWhenTheStemBaselineIsNotBeaten(t *testing.T) {
	document := syntheticReport()
	finalize(document)
	block := document.Summary["stem_baseline"].(map[string]any)
	if block["statement"] != "the stem baseline is not beaten by any arm" || len(block["beaten_by"].([]string)) != 0 {
		t.Fatalf("stem baseline block = %v", block)
	}
	control := block["arms"].(map[string]any)["control"].(map[string]any)
	if !strings.HasPrefix(control["verdict"].(string), "not beaten") || control["pairs_scored"] != 2 {
		t.Fatalf("control against the stem baseline = %v", control)
	}

	// Six changes no stem guess reaches; the treatment cites all or half of
	// the gold, so it misses far less than the stem baseline.
	document = syntheticReport()
	document.Changes, document.Lanes = nil, nil
	for index := range 6 {
		id := fmt.Sprintf("c%02d", index)
		gold := []string{id + "_a_test.go", id + "_b_test.go"}
		document.Changes = append(document.Changes, changeMeta{ID: id, Gold: gold, StemBaseline: []string{}})
		cited := gold[0]
		document.Lanes = append(document.Lanes,
			&lane{ID: id, Arm: "control", Repeat: 1, ArmOrder: armNames, Reply: reply("src.go"), WallMs: 10, Tokens: notObserved, ExitCode: 0},
			&lane{ID: id, Arm: "treatment", Repeat: 1, ArmOrder: armNames, Reply: reply(cited), WallMs: 10, Tokens: notObserved, ExitCode: 0})
		if index%2 == 0 {
			document.Lanes[len(document.Lanes)-1].Reply = reply(gold...)
		}
	}
	finalize(document)
	block = document.Summary["stem_baseline"].(map[string]any)
	if beaten := block["beaten_by"].([]string); len(beaten) != 1 || beaten[0] != "treatment" {
		t.Fatalf("beaten_by = %v (%v)", beaten, block["arms"])
	}
	if !strings.Contains(block["statement"].(string), "false findings are NOT_OBSERVED") {
		t.Fatalf("a beaten baseline claims the reviewer bar: %v", block["statement"])
	}
}

// CRT-V0-018: evidence discovery and adjudicated defect detection are two
// outcomes; with no adjudication records the second is NOT_OBSERVED for every
// arm and the discovery counts come from the citations each lane used.
func TestCRTV0018DiscoveryAndDefectDetectionAreSeparateOutcomes(t *testing.T) {
	document := syntheticReport()
	finalize(document)
	outcomes := document.Summary["outcomes"].(map[string]any)
	if !strings.HasPrefix(outcomes["defect_detection"].(string), "NOT_OBSERVED") {
		t.Fatalf("defect detection = %v", outcomes["defect_detection"])
	}
	for _, arm := range armNames {
		block := outcomes["arms"].(map[string]any)[arm].(map[string]any)
		discovery := block["evidence_discovery"].(map[string]any)
		if discovery["lanes"] != 2 || discovery["citations_used"] != 2 || block["evidence_opened"] != notObserved {
			t.Fatalf("%s discovery = %v", arm, block)
		}
		defects := block["defect_detection"].(map[string]any)
		if defects["adjudicated_defects"] != notObserved || defects["adjudicated_false_findings"] != notObserved {
			t.Fatalf("%s defect detection = %v", arm, defects)
		}
	}
}
