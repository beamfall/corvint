package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureRepoWithAddedTest is fixtureRepo plus a change that creates a new
// test file: gold the change itself adds, absent at the base revision.
func fixtureRepoWithAddedTest(t *testing.T) string {
	t.Helper()
	root := fixtureRepo(t)
	write := func(path, content string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run := func(arguments ...string) {
		t.Helper()
		if _, err := git(context.Background(), root, arguments...); err != nil {
			t.Fatalf("git %s: %v", strings.Join(arguments, " "), err)
		}
	}
	edited := strings.NewReplacer("return 10", "return 100", "return 11", "return 110", "return 33", "return 44").Replace(changedSource)
	write("internal/play/play.go", edited)
	write("internal/play/play_test.go", "package play\n\nimport \"testing\"\n\nfunc TestOne(t *testing.T) {}\n\nfunc TestResume(t *testing.T) {}\n\nfunc TestStart(t *testing.T) {}\n")
	write("internal/play/offset_test.go", "package play\n\nimport \"testing\"\n\nfunc TestOffset(t *testing.T) {}\n")
	run("add", "-A")
	run("commit", "-q", "-m", "change with a new test")
	return root
}

// CRT-V0-004 (amended 2026-10-01): a test-or-spec file the change adds does
// not exist at the base revision, so no arm can cite it; it is recorded as
// gold_unreachable and never scored.
func TestSelectionRecordsUnreachableGoldSeparately(t *testing.T) {
	repo := fixtureRepoWithAddedTest(t)
	output := t.TempDir()
	corvintGo := buildCorvint(t)
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"select",
		"--repo", repo, "--name", "fixture", "--since", "1970-01-01", "--limit", "2",
		"--seed", "fixture", "--partition", "pilot", "--corvint", corvintGo, "--output", output,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("select failed: %s", stderr.String())
	}
	set, _, err := readManifest(filepath.Join(output, "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var added *task
	for index := range set.Tasks {
		if len(set.Tasks[index].GoldUnreachable) != 0 {
			added = &set.Tasks[index]
		}
	}
	if added == nil {
		t.Fatalf("no change recorded unreachable gold: %s", stdout.String())
	}
	if len(added.GoldUnreachable) != 1 || added.GoldUnreachable[0] != "internal/play/offset_test.go" {
		t.Fatalf("gold_unreachable = %v, want the added test alone", added.GoldUnreachable)
	}
	for _, path := range added.Gold.Paths {
		if path == "internal/play/offset_test.go" {
			t.Fatal("the added test is scored gold")
		}
	}
	if len(added.Gold.Paths) != 1 || added.Gold.Paths[0] != "internal/play/play_test.go" {
		t.Fatalf("gold = %v, want the modified test alone", added.Gold.Paths)
	}
	if !strings.Contains(stdout.String(), "unreachable (added by the change, not scored): internal/play/offset_test.go") {
		t.Fatalf("selection summary does not name the unreachable gold: %s", stdout.String())
	}
	if !strings.Contains(set.SelectionRule, "gold_unreachable") {
		t.Fatalf("the manifest's frozen rule does not state the reachability clause: %s", set.SelectionRule)
	}
}

// CRT-V0-005, CRT-V0-012: the seeded prompt is the control prompt plus its
// declared prologue, and that prologue carries the suggestions beside the
// three CEM artefacts.
func TestSeededArmSharesThePromptApartFromItsPrologue(t *testing.T) {
	item := task{ID: "c01", SourceFiles: []string{"internal/play/play.go"}}
	patch := "--- a/internal/play/play.go\n+++ b/internal/play/play.go\n@@ -1,1 +1,1 @@\n-a\n+b\n"
	suggestions := `[{"subject":"internal/play/play.go","path":"internal/play/play_test.go","relation":"pair","reason":"test counterpart of internal/play/play.go"}]`
	artefacts := map[string]string{"map": "{}", "status": "{}", "report": "report", "suggestions": suggestions}
	control := buildPrompt("control", item, patch, artefacts)
	seeded := buildPrompt("seeded", item, patch, artefacts)
	prologue := strings.TrimPrefix(seeded, control[:strings.Index(control, "## Reply")])
	prologue = prologue[:strings.Index(prologue, "## Reply")]
	if strings.Replace(seeded, prologue, "", 1) != control {
		t.Fatal("removing the seeded prologue does not reproduce the control prompt")
	}
	for _, needle := range []string{"## Change Evidence Map", "### Suggested evidence", "Nothing here is cited", suggestions} {
		if !strings.Contains(prologue, needle) {
			t.Fatalf("seeded prologue lacks %q", needle)
		}
	}
	if strings.Contains(control, "Corvint") || strings.Contains(control, "suggest") {
		t.Fatal("the control prompt names Corvint or the suggestions")
	}
	identity := prologueIdentity([]string{"control", "treatment", "seeded"})
	if identity["seeded"]["sha256"] == identity["treatment"]["sha256"] || identity["seeded"]["text"] != seededPrologue {
		t.Fatalf("seeded prologue identity is not its own: %v", identity["seeded"])
	}
	if got := suggestedPaths(suggestions); len(got) != 1 || got[0] != "internal/play/play_test.go" {
		t.Fatalf("suggestedPaths = %v", got)
	}
}

func TestRunRefusesSeededArmWithoutCorvint(t *testing.T) {
	_, err := parseRunOptions([]string{"--tasks", "t.json", "--history", "h", "--arms", "control,seeded", "--agent", "script", "--agent-command", "x"})
	if err == nil || !strings.Contains(err.Error(), "--corvint is required") {
		t.Fatalf("seeded arm without --corvint was accepted: %v", err)
	}
	_, err = parseRunOptions([]string{"--tasks", "t.json", "--history", "h", "--arms", "control,other", "--agent", "script", "--agent-command", "x"})
	if err == nil || !strings.Contains(err.Error(), "unknown arm") {
		t.Fatalf("unknown arm was accepted: %v", err)
	}
}

// threeArmReport is two changes under control, treatment and seeded, each arm
// twice: control cites nothing of the gold, treatment cites the first gold
// item on the first repeat only, seeded was shown the first gold item and
// cites it on both repeats.
func threeArmReport() *report {
	arms := []string{"control", "treatment", "seeded"}
	changes := []changeMeta{
		{ID: "c01", Gold: []string{"a_test.go", "b_test.go"}, StemBaseline: []string{"a_test.go"}},
		{ID: "c02", Gold: []string{"c_test.go", "d_test.go"}, StemBaseline: []string{}},
	}
	lanes := []*lane{}
	for _, change := range changes {
		for repeat := 1; repeat <= 2; repeat++ {
			treatmentReply := reply("src.go")
			if repeat == 1 {
				treatmentReply = reply(change.Gold[0])
			}
			lanes = append(lanes,
				&lane{ID: change.ID, Arm: "control", Repeat: repeat, ArmOrder: arms, Reply: reply("src.go"), WallMs: int64(10), Tokens: notObserved, ExitCode: 0},
				&lane{ID: change.ID, Arm: "treatment", Repeat: repeat, ArmOrder: arms, Reply: treatmentReply, WallMs: int64(11), Tokens: notObserved, ExitCode: 0,
					CEM: map[string]any{"counts": map[string]any{"total": float64(4), "supported": float64(3), "mechanical": float64(0), "unknown": float64(1)}, "state": "incomplete", "verify_ok": true, "replay_ok": true}},
				&lane{ID: change.ID, Arm: "seeded", Repeat: repeat, ArmOrder: arms, Reply: reply(change.Gold[0]), WallMs: int64(12), Tokens: notObserved, ExitCode: 0,
					Suggested: []string{change.Gold[0], "src.go"},
					CEM:       map[string]any{"counts": map[string]any{"total": float64(4), "supported": float64(4), "mechanical": float64(0), "unknown": float64(0)}, "state": "ready", "verify_ok": true, "replay_ok": true}},
			)
		}
	}
	return &report{
		Profile: profile, Partition: "pilot", Pilot: true, Repository: "fixture",
		SelectionRule: selectionRule, Population: 2, Pairs: 2, Seed: "fixture",
		Model: notObserved, Effort: notObserved, ManifestSHA256: digestOf("manifest"),
		SkeletonSHA256: digestOf(skeleton), Arms: arms, Repeats: 2,
		Prologues: prologueIdentity(arms), Agent: map[string]any{"kind": "script"},
		CorvintGo: map[string]any{"version": notObserved, "sha256": notObserved},
		Changes:   changes, Lanes: lanes,
	}
}

// CRT-V0-007, CRT-V0-012: each compared arm is estimated against control, the
// lane-level McNemar keeps the discordance the change-level average hides, and
// the seeded arm is decomposed into retrieval and adoption.
func TestThreeArmSummaryKeepsPilotKeysAndDecomposesTheSeededArm(t *testing.T) {
	document := threeArmReport()
	finalize(document)
	summary := document.Summary
	// The top level is still the pilot's treatment-against-control block.
	if summary["pairs_scored"] != 2 {
		t.Fatalf("pairs_scored = %v", summary["pairs_scored"])
	}
	meanMiss := summary["mean_miss"].(map[string]any)
	if meanMiss["control"] != 1.0 || meanMiss["treatment"] != 0.75 || meanMiss["stem_baseline"] != 0.75 {
		t.Fatalf("mean_miss = %v", meanMiss)
	}
	arms := summary["arms"].(map[string]any)
	seeded := arms["seeded"].(map[string]any)
	if seeded["mean_miss"].(map[string]any)["seeded"] != 0.5 || seeded["delta"] != 0.5 || seeded["relative_reduction"] != 0.5 {
		t.Fatalf("seeded block = %v", seeded)
	}
	if seeded["citable"].(map[string]any)["fraction"] != 1.0 {
		t.Fatalf("seeded citable reads the seeded lanes' own maps: %v", seeded["citable"])
	}
	treatmentLanes := arms["treatment"].(map[string]any)["mcnemar_lanes"].(map[string]any)
	if treatmentLanes["pairs"] != 4 || treatmentLanes["reference_only_missed"] != 0 || treatmentLanes["compared_only_missed"] != 0 {
		t.Fatalf("treatment lane McNemar = %v (every lane still missed something)", treatmentLanes)
	}
	coverage := summary["seed_coverage"].(map[string]any)
	if coverage["lanes"] != 4 || coverage["gold_items"] != 8 || coverage["suggested_items"] != 4 || coverage["fraction"] != 0.5 {
		t.Fatalf("seed_coverage = %v", coverage)
	}
	if coverage["miss_given_suggested"] != 0.0 || coverage["miss_given_unsuggested"] != 1.0 {
		t.Fatalf("seed_coverage decomposition = %v", coverage)
	}
	contrast := summary["contrasts"].(map[string]any)["seeded_vs_treatment"].(map[string]any)
	if contrast["delta"] != 0.25 || contrast["pairs_scored"] != 2 {
		t.Fatalf("seeded_vs_treatment = %v", contrast)
	}
	for _, item := range document.Lanes {
		if item.Arm == "seeded" && (len(item.SeedHits) != 1 || item.SeedHits[0] != item.GoldHits[0]) {
			t.Fatalf("seed hits not scored on lane %s/%d: %v", item.ID, item.Repeat, item.SeedHits)
		}
		if item.Arm != "seeded" && item.SeedHits != nil {
			t.Fatalf("seed hits recorded on a lane without suggestions: %s", item.Arm)
		}
	}
	// `score --report` rebuilds the three-arm report byte for byte (CRT-V0-010).
	path := filepath.Join(t.TempDir(), "report.json")
	var stdout, stderr bytes.Buffer
	if code := write(document, path, &stdout, &stderr); code != 0 {
		t.Fatalf("write failed: %s", stderr.String())
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rebuilt, output, err := rescore([]string{"--report", path})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if code := write(rebuilt, output, &out, &stderr); code != 0 {
		t.Fatalf("rescore write failed: %s", stderr.String())
	}
	if !bytes.Equal(original, out.Bytes()) {
		t.Fatal("score did not reproduce the three-arm report's bytes")
	}
	var decoded map[string]any
	if err := json.Unmarshal(original, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded["summary"].(map[string]any)["seed_coverage"]; !ok {
		t.Fatal("the written report lacks seed_coverage")
	}
}

// A two-arm report summarises exactly as the pilot did: no `arms`, no
// `seed_coverage`, no `contrasts`.
func TestTwoArmSummaryIsUnchanged(t *testing.T) {
	document := syntheticReport()
	finalize(document)
	for _, key := range []string{"arms", "seed_coverage", "contrasts"} {
		if _, present := document.Summary[key]; present {
			t.Fatalf("two-arm summary carries %s", key)
		}
	}
}

// The lane-level McNemar sees discordance the averaged change-level test
// collapses: the pilot reported 0/0 discordant pairs over fifty lanes.
func TestLaneLevelMcNemarKeepsDiscordance(t *testing.T) {
	arms := []string{"control", "treatment"}
	document := &report{Arms: arms, Changes: []changeMeta{{ID: "c01", Gold: []string{"a_test.go"}}}, Seed: "s"}
	for repeat := 1; repeat <= 4; repeat++ {
		treatmentReply := reply("a_test.go")
		if repeat == 4 {
			treatmentReply = reply("src.go")
		}
		document.Lanes = append(document.Lanes,
			&lane{ID: "c01", Arm: "control", Repeat: repeat, ArmOrder: arms, Reply: reply("src.go"), WallMs: int64(1), Tokens: notObserved, ExitCode: 0},
			&lane{ID: "c01", Arm: "treatment", Repeat: repeat, ArmOrder: arms, Reply: treatmentReply, WallMs: int64(1), Tokens: notObserved, ExitCode: 0},
		)
	}
	meta := map[string]changeMeta{"c01": document.Changes[0]}
	for _, item := range document.Lanes {
		scoreLane(item, meta["c01"])
	}
	lanes := mcnemarLanes(document, "control", "treatment")
	if lanes["pairs"] != 4 || lanes["reference_only_missed"] != 3 || lanes["compared_only_missed"] != 0 {
		t.Fatalf("lane McNemar = %v", lanes)
	}
	byChange, errored, order := laneMisses(document)
	changeLevel := mcnemar(armPairs(byChange, errored, order, meta, "control", "treatment"))
	if changeLevel["control_only_missed"] != 0 {
		t.Fatalf("change-level test should average the repeats into a tie: %v", changeLevel)
	}
}
