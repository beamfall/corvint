package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureRepo builds a tiny synthetic history: a base commit holding a source
// file, its spec, and its test, and a change commit that edits the source in
// three places and the test once. The test file is the change's gold.
func fixtureRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	run := func(arguments ...string) {
		t.Helper()
		if _, err := git(context.Background(), root, arguments...); err != nil {
			t.Fatalf("git %s: %v", strings.Join(arguments, " "), err)
		}
	}
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
	run("init", "-q", "-b", "main")
	write("internal/play/play.go", baseSource)
	write("internal/play/play_test.go", "package play\n\nimport \"testing\"\n\nfunc TestOne(t *testing.T) {}\n")
	write("docs/specs/play-v0.md", "# Play\n\nPLAY-V0-001: playback resumes at the stored offset.\n")
	write("README.md", "beamfall fixture\n")
	run("add", "-A")
	run("commit", "-q", "-m", "base")
	write("internal/play/play.go", changedSource)
	write("internal/play/play_test.go", "package play\n\nimport \"testing\"\n\nfunc TestOne(t *testing.T) {}\n\nfunc TestResume(t *testing.T) {}\n")
	write("README.md", "beamfall fixture, updated\n")
	run("add", "-A")
	run("commit", "-q", "-m", "change")
	return root
}

const baseSource = `package play

// Resume reports the resume offset used by the resume path.
// The comment is padded so each edit lands in its own hunk.
//
//
//
func Resume() int {
	return 0
}

// Offset reports the offset offset used by the resume path.
// The comment is padded so each edit lands in its own hunk.
//
//
//
func Offset() int {
	return 1
}

// Stop reports the stop offset used by the resume path.
// The comment is padded so each edit lands in its own hunk.
//
//
//
func Stop() int {
	return 2
}

// Start reports the start offset used by the resume path.
// The comment is padded so each edit lands in its own hunk.
//
//
//
func Start() int {
	return 3
}
`

const changedSource = `package play

// Resume reports the resume offset used by the resume path.
// The comment is padded so each edit lands in its own hunk.
//
//
//
func Resume() int {
	return 10
}

// Offset reports the offset offset used by the resume path.
// The comment is padded so each edit lands in its own hunk.
//
//
//
func Offset() int {
	return 11
}

// Stop reports the stop offset used by the resume path.
// The comment is padded so each edit lands in its own hunk.
//
//
//
func Stop() int {
	return 2
}

// Start reports the start offset used by the resume path.
// The comment is padded so each edit lands in its own hunk.
//
//
//
func Start() int {
	return 33
}
`

// buildCorvint builds the corvint the trial exercises, as the harness itself is
// given one on the command line.
func buildCorvint(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("builds corvint")
	}
	binary := filepath.Join(t.TempDir(), "corvint")
	command := exec.Command("go", "build", "-o", binary, "github.com/Beamfall/corvint/cmd/corvint")
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	if output, err := command.CombinedOutput(); err != nil {
		t.Skipf("cannot build corvint: %v: %s", err, output)
	}
	return binary
}

func selectPilot(t *testing.T, repo, output string) *manifest {
	t.Helper()
	corvintGo := buildCorvint(t)
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"select",
		"--repo", repo, "--name", "fixture", "--since", "1970-01-01", "--limit", "1",
		"--seed", "fixture", "--partition", "pilot", "--corvint", corvintGo, "--output", output,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("select failed: %s", stderr.String())
	}
	set, _, err := readManifest(filepath.Join(output, "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	return set
}

// CRT-V0-001, CRT-V0-004.
func TestSelectionIsDeterministicAndRefusesGoldLeak(t *testing.T) {
	repo := fixtureRepo(t)
	first := selectPilot(t, repo, t.TempDir())
	second := selectPilot(t, repo, t.TempDir())
	if len(first.Tasks) != 1 {
		t.Fatalf("expected one change, got %d", len(first.Tasks))
	}
	if first.Tasks[0].Commit != second.Tasks[0].Commit || first.Tasks[0].PatchSHA256 != second.Tasks[0].PatchSHA256 {
		t.Fatal("two selections over the same history disagreed")
	}
	if first.SelectionRule != selectionRule || first.Population < 1 {
		t.Fatal("the manifest did not freeze the rule and the population")
	}
	gold := []string{"internal/play/play_test.go"}
	leaking := "@@ -1,2 +1,3 @@\n+// see internal/play/play_test.go\n"
	if !leaksGold(leaking, gold) {
		t.Fatal("a hunk naming the gold path was not rejected")
	}
	if leaksGold("@@ -1,2 +1,3 @@\n+func Resume() int { return 10 }\n", gold) {
		t.Fatal("a hunk naming nothing was rejected")
	}
}

// CRT-V0-001: pairing, reuse, and the scorer key lanes by change id, so two
// changes sharing one id would pool into one pair.
func TestManifestRefusesDuplicateChangeIDs(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "tasks.json")
	change := `{"id":"c01","patch_path":"c.patch","patch_sha256":"` + digestOf("patch") + `"}`
	if err := os.WriteFile(filepath.Join(directory, "c.patch"), []byte("patch"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"partition":"pilot","tasks":[`+change+`,`+change+`]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readManifest(path); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("readManifest error = %v, want a duplicate id refusal", err)
	}
}

// CRT-V0-003.
func TestWithheldPatchExcludesEveryGoldPath(t *testing.T) {
	directory := t.TempDir()
	set := selectPilot(t, fixtureRepo(t), directory)
	item := set.Tasks[0]
	body, err := readPatch(directory, item)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range item.Gold.Paths {
		if strings.Contains(body, path) {
			t.Fatalf("the presented patch names the gold path %s", path)
		}
	}
	if len(item.Gold.Paths) == 0 || len(item.Gold.Spans) == 0 {
		t.Fatal("the change carries no gold paths or spans")
	}
	for _, file := range item.SourceFiles {
		if !strings.Contains(body, file) {
			t.Fatalf("the presented patch lost its source file %s", file)
		}
	}
}

// CRT-V0-005, CRT-V0-010.
func TestBothArmsShareTheSamePromptDigestApartFromThePrologue(t *testing.T) {
	item := task{ID: "c01"}
	patch := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1,2 +1,2 @@\n-x\n+y\n"
	artefacts := map[string]string{"map": "{}", "status": "{}", "report": "# report"}
	control := buildPrompt("control", item, patch, artefacts)
	treatment := buildPrompt("treatment", item, patch, artefacts)
	if control == treatment {
		t.Fatal("the arms received the same prompt")
	}
	block := fmt.Sprintf(treatmentPrologue, artefacts["map"], artefacts["status"], artefacts["report"])
	stripped := strings.Replace(treatment, block, "", 1)
	if stripped != control {
		t.Fatal("the treatment prompt differs from the control prompt beyond the declared prologue")
	}
	for _, word := range []string{"Corvint", "corvint", "CEM", "Change Evidence Map", "cem"} {
		if strings.Contains(control, word) {
			t.Fatalf("the control prompt names %q", word)
		}
	}
}

// CRT-V0-005: the map is built from the withheld bytes, never derived from
// the change, so `cem prepare` never appears and no `.corvint` state survives.
func TestTreatmentUsesCemBeginNotPrepare(t *testing.T) {
	corvintGo := buildCorvint(t)
	repo := fixtureRepo(t)
	directory := t.TempDir()
	set := selectPilot(t, repo, directory)
	item := set.Tasks[0]
	patch, err := readPatch(directory, item)
	if err != nil {
		t.Fatal(err)
	}
	root, err := cloneAtBase(context.Background(), repo, item.BaseCommit)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	artefacts, err := buildArtefacts(context.Background(), options{corvintGo: corvintGo}, root, patch)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Spec         string `json:"spec"`
		BaseRevision string `json:"baseRevision"`
		PatchSha256  string `json:"patchSha256"`
	}
	if err := json.Unmarshal([]byte(artefacts["map"]), &document); err != nil {
		t.Fatalf("the map is not JSON: %v", err)
	}
	if document.Spec != "cem/0.1" || document.BaseRevision != item.BaseCommit {
		t.Fatalf("the map is not a cem/0.1 map at the base revision: %+v", document)
	}
	if document.PatchSha256 != item.PatchSHA256 {
		t.Fatal("the map was not built over the withheld patch bytes")
	}
	if _, err := os.Stat(filepath.Join(root, mapRelative)); !os.IsNotExist(err) {
		t.Fatal("the map was left in the prep clone every lane copies")
	}
	if !strings.Contains(artefacts["status"], "\"worklist\"") || !strings.Contains(artefacts["report"], "Worklist") {
		t.Fatal("the treatment arm did not receive the status worklist and the report")
	}
}

// CRT-V0-005.
func TestCloneCannotResolveTheChangeCommit(t *testing.T) {
	repo := fixtureRepo(t)
	head, err := git(context.Background(), repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	base, err := git(context.Background(), repo, "rev-parse", "HEAD^")
	if err != nil {
		t.Fatal(err)
	}
	root, err := cloneAtBase(context.Background(), repo, base)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	if resolvable(context.Background(), root, head) {
		t.Fatal("the clone resolves the change commit")
	}
	if !resolvable(context.Background(), root, base) {
		t.Fatal("the clone does not resolve its own base commit")
	}
	lane, err := laneClone(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(lane)
	if resolvable(context.Background(), lane, head) {
		t.Fatal("the lane clone resolves the change commit")
	}
	if _, err := os.Stat(filepath.Join(lane, "internal", "play", "play.go")); err != nil {
		t.Fatalf("the lane clone has no working tree: %v", err)
	}
}

// CRT-V0-007, CRT-V0-008.
func TestMissRateAndPairedBootstrapAreDeterministicUnderASeed(t *testing.T) {
	document := syntheticReport()
	finalize(document)
	first := document.Summary
	second := syntheticReport()
	finalize(second)
	if !equalJSON(t, first, second.Summary) {
		t.Fatal("two scorings of the same raw report disagreed")
	}
	if got := first["delta"]; got != 0.5 {
		t.Fatalf("delta = %v, want 0.5", got)
	}
	interval := first["delta_ci95"].(map[string]any)
	if interval["low"].(float64) > interval["high"].(float64) {
		t.Fatal("the bootstrap interval is inverted")
	}
	verdict := first["verdict"].(map[string]any)["missed_evidence"].(string)
	if !strings.HasPrefix(verdict, "met:") && !strings.HasPrefix(verdict, "consistent") {
		t.Fatalf("unexpected verdict %q", verdict)
	}
}

// CRT-V0-007: the replay checker is written against git alone, so it can
// disagree with `cem verify` on a citation `cem verify` rejects.
func TestReplayCheckerDisagreesWithVerifyOnAKnownGoodCitation(t *testing.T) {
	repo := fixtureRepo(t)
	base, err := git(context.Background(), repo, "rev-parse", "HEAD^")
	if err != nil {
		t.Fatal(err)
	}
	root, err := cloneAtBase(context.Background(), repo, base)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	good := citation{Hunk: "1", Path: "docs/specs/play-v0.md", Lines: "1:3", Relation: "specification"}
	if !replayCitations(context.Background(), root, base, []citation{good}) {
		t.Fatal("the replay rejected a citation that resolves at the base revision")
	}
	for name, bad := range map[string]citation{
		"absent path":    {Hunk: "1", Path: "docs/specs/missing.md", Lines: "1:2", Relation: "specification"},
		"range past end": {Hunk: "1", Path: "docs/specs/play-v0.md", Lines: "1:900", Relation: "specification"},
		"blank span":     {Hunk: "1", Path: "docs/specs/play-v0.md", Lines: "2:2", Relation: "specification"},
		"bad relation":   {Hunk: "1", Path: "docs/specs/play-v0.md", Lines: "1:3", Relation: "vibes"},
	} {
		if replayCitations(context.Background(), root, base, []citation{bad}) {
			t.Fatalf("the replay accepted a %s citation", name)
		}
	}
}

// CRT-V0-010: an older report recorded a non-zero exit without an error, so
// reuse must read the exit code and re-run that lane.
func TestReuseRerunsALaneThatExitedNonZeroWithoutAnError(t *testing.T) {
	prior := &lane{ID: "c01", Arm: "control", Repeat: 1, PromptSHA256: "p", WallMs: float64(10), ExitCode: float64(1)}
	record := &lane{ID: "c01", Arm: "control", Repeat: 1, PromptSHA256: "p", WallMs: notObserved, ExitCode: notObserved}
	if newReuseSource(&report{Lanes: []*lane{prior}}).apply(record) {
		t.Fatalf("a lane that exited 1 was reused: %+v", record)
	}
}

// CRT-V0-010.
func TestScoreRebuildsTheReportByteIdentically(t *testing.T) {
	document := syntheticReport()
	finalize(document)
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
		t.Fatal("score did not reproduce the report's bytes")
	}
}

// CRT-V0-007: an errored lane drops its pair whole, so the arms never cover
// different changes.
func TestErroredLaneDropsItsPair(t *testing.T) {
	document := syntheticReport()
	for _, item := range document.Lanes {
		if item.ID == "c02" && item.Arm == "treatment" {
			item.Error = "codex: context deadline exceeded after 5m0s"
		}
	}
	finalize(document)
	if got := document.Summary["pairs_scored"]; got != 1 {
		t.Fatalf("pairs_scored = %v, want 1", got)
	}
	if got := document.Summary["pairs_dropped"]; got != 1 {
		t.Fatalf("pairs_dropped = %v, want 1", got)
	}
	for _, item := range document.Lanes {
		if item.ID == "c02" && item.Arm == "control" && item.Miss == 0 {
			t.Fatal("the surviving arm of a dropped pair was not scored at all")
		}
	}
}

// A lane whose agent process exited non-zero produced no observation. Scoring
// it as "cited nothing" put a miss of 1.0 into the arm mean and reported the
// pair as scored, which is how the 2026-09-03 pilot published a delta over 31
// failed invocations.
func TestNonZeroAgentExitErrorsTheLaneAndInvalidatesTheRun(t *testing.T) {
	document := syntheticReport()
	for _, item := range document.Lanes {
		if item.ID == "c02" && item.Arm == "treatment" {
			item.Reply, item.ExitCode = "", float64(1)
		}
	}
	finalize(document)
	if got := document.Summary["errored_lanes"]; got != 1 {
		t.Fatalf("errored_lanes = %v, want 1", got)
	}
	if got := document.Summary["pairs_scored"]; got != 1 {
		t.Fatalf("pairs_scored = %v, want 1", got)
	}
	if document.Invalid != "" {
		t.Fatalf("one errored lane invalidated the run: %q", document.Invalid)
	}
	for _, item := range document.Lanes {
		item.Reply, item.ExitCode = "", float64(1)
	}
	finalize(document)
	if got := document.Summary["pairs_scored"]; got != 0 {
		t.Fatalf("pairs_scored = %v over failed invocations, want 0", got)
	}
	if document.Invalid == "" {
		t.Fatal("four errored lanes did not invalidate the run (CRT-V0-010)")
	}
}

// CRT-V0-006: a reply cut at the 1 MiB bound lost its tail, so a missing
// citations block is the harness's loss, not a total miss.
func TestTruncatedReplyWithoutCitationsErrorsTheLane(t *testing.T) {
	document := syntheticReport()
	for _, item := range document.Lanes {
		if item.ID == "c02" && item.Arm == "treatment" {
			item.Reply, item.ReplyTruncated = strings.Repeat("x", maxReplyBytes)+truncatedMarker, true
		}
	}
	finalize(document)
	if got := document.Summary["pairs_scored"]; got != 1 {
		t.Fatalf("pairs_scored = %v, want the truncated lane's pair dropped", got)
	}
}

func reply(paths ...string) string {
	block := `{"citations":[`
	for index, path := range paths {
		if index > 0 {
			block += ","
		}
		block += `{"hunk":"1","path":"` + path + `","lines":"1:2","relation":"test-claim","confidence":"likely"}`
	}
	return "done\n\n```json\n" + block + `],"unknown":[]}` + "\n```\n"
}

// syntheticReport is two changes, each with a control lane that cites nothing
// of the gold and a treatment lane that cites half of it.
func syntheticReport() *report {
	changes := []changeMeta{
		{ID: "c01", Gold: []string{"a_test.go", "b_test.go"}, StemBaseline: []string{"a_test.go"}},
		{ID: "c02", Gold: []string{"c_test.go", "d_test.go"}, StemBaseline: []string{}},
	}
	lanes := []*lane{}
	for _, change := range changes {
		lanes = append(lanes,
			&lane{ID: change.ID, Arm: "control", Repeat: 1, ArmOrder: armNames, Reply: reply("src.go"), WallMs: int64(10), Tokens: notObserved, ExitCode: 0},
			&lane{ID: change.ID, Arm: "treatment", Repeat: 1, ArmOrder: armNames, Reply: reply(change.Gold[0]), WallMs: int64(11), Tokens: notObserved, ExitCode: 0,
				CEM: map[string]any{"counts": map[string]any{"total": float64(4), "supported": float64(3), "mechanical": float64(0), "unknown": float64(1)}, "state": "incomplete", "verify_ok": true, "replay_ok": true}},
		)
	}
	return &report{
		Profile: profile, Partition: "pilot", Pilot: true, Repository: "fixture",
		SelectionRule: selectionRule, Population: 2, Pairs: 2, Seed: "fixture",
		Model: notObserved, Effort: notObserved, ManifestSHA256: digestOf("manifest"),
		SkeletonSHA256: digestOf(skeleton), Arms: armNames, Repeats: 1,
		Prologues: prologueIdentity(armNames), Agent: map[string]any{"kind": "script"},
		CorvintGo: map[string]any{"version": notObserved, "sha256": notObserved},
		Changes:   changes, Lanes: lanes,
	}
}

func equalJSON(t *testing.T, left, right any) bool {
	t.Helper()
	first, err := json.Marshal(left)
	if err != nil {
		t.Fatal(err)
	}
	second, err := json.Marshal(right)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Equal(first, second)
}
