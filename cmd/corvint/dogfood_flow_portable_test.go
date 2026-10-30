package main

import (
	"bytes"
	"encoding/json"
	"github.com/Beamfall/corvint/internal/groupreap"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/localcompletion"
)

// portableDogfoodRepo is a Git repository that is not Corvint: no script/, no
// VERSION and no Corvint source, only a small Go module with one intent.

// absentAgentReceipts is what dogfood change notes, without blocking, when the
// agent wrote no pre-change receipts (V1-0316).
const absentAgentReceipts = "dogfood-change: NOTE prechange-query NOT_OBSERVED agent-receipt-absent\n" +
	"dogfood-change: NOTE prechange-impact NOT_OBSERVED agent-receipt-absent\n"

// dropGuardingNote removes the advisory guarding-tests NOTE (DCW-V0-069) that
// dogfood change prints when the fixture's map holds a hunk that search would
// select; these tests assert the other notes exactly.
func dropGuardingNote(stderr string) string {
	var kept strings.Builder
	for _, line := range strings.SplitAfter(stderr, "\n") {
		// DCW-V0-112 tier warnings and the DCW-V0-165 and DCW-V0-388 notes are asserted
		// where they are the subject.
		// The DCW-V0-342 closing PASS line is asserted in
		// TestDogfoodDailyPathRunsFromBinaryInForeignRepository.
		if !strings.HasPrefix(line, "dogfood-change: NOTE guarding-tests ") && !strings.HasPrefix(line, "dogfood-check: WARN tier-below-anchored ") &&
			!strings.HasPrefix(line, "dogfood-check: NOTE tier-below-anchored-unanchorable ") &&
			!strings.HasPrefix(line, "dogfood-check: NOTE tier-below-anchored-same-path ") && !strings.HasPrefix(line, "dogfood-change: PASS complete ") {
			kept.WriteString(line)
		}
	}
	return kept.String()
}

func portableDogfoodRepo(t *testing.T) (string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cemWrite(t, root, "go.mod", "module example.com/portable\n\ngo 1.22\n")
	cemWrite(t, root, "AGENTS.md", "# Fixture authority\nUse intent.md to govern the fixture.\n")
	cemWrite(t, root, "intent.md", "# Intent\n\n## Requirements\n\n- `FIXTURE-LCP-001`: The answer is two.\n- `FIXTURE-LCP-002`: The existing companion behavior remains unchanged.\n")
	cemWrite(t, root, "fixture/fixture.go", "package fixture\nfunc Answer() int { return 1 }\n")
	cemWrite(t, root, "fixture/fixture_test.go", "package fixture\nimport \"testing\"\nfunc TestAnswer(t *testing.T) {\n t.Run(\"FIXTURE-LCP-001\",func(t *testing.T){ if Answer()!=2 { t.Fatal(\"answer\") } })\n}\n")
	cemGit(t, root, "init", "-q", "-b", "main")
	// The private outputs and local trace store the daily path writes in the
	// worktree; the recorder and the clean check require them ignored.
	exclude := ".corvint/dogfood-report.json\n.corvint/change.ocm-intents\n.corvint/change.ocm-status.json\n.corvint/change.ocm.*.json\n.corvint/self-observations.jsonl\n.context-corvint/\n"
	if err = os.WriteFile(filepath.Join(root, ".git/info/exclude"), []byte(exclude), 0644); err != nil {
		t.Fatal(err)
	}
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "base")
	base := cemGit(t, root, "rev-parse", "HEAD")
	cemWrite(t, root, "fixture/fixture.go", "package fixture\nfunc Answer() int { return 2 }\n")
	cemGit(t, root, "commit", "-qam", "change")
	for _, absent := range []string{"script", "VERSION", "cmd/corvint"} {
		if _, err = os.Lstat(filepath.Join(root, absent)); !os.IsNotExist(err) {
			t.Fatalf("fixture carries %s", absent)
		}
	}
	return root, base
}

type portableRun struct {
	binary string
	env    []string
}

// portableDogfoodRunner builds this binary and an environment whose PATH
// resolves `corvint` to a failing impostor and whose CORVINT_BIN names a
// missing file, so every step must run as the executable itself.
// portableDogfoodBuild links the VCS-free corvint binary once per package run.
// Each runner copies it to its own path, so no test shares or can alter another
// test's executable; relinking it for every runner overran the macOS package
// timeout (V1-3060).
var portableDogfoodBuild struct {
	once sync.Once
	root string
	path string
	err  error
}

func portableDogfoodRunner(t *testing.T) portableRun {
	t.Helper()
	name := "corvint"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	portableDogfoodBuild.once.Do(func() {
		portableDogfoodBuild.root, portableDogfoodBuild.err = os.MkdirTemp("", "corvint-portable-dogfood-")
		if portableDogfoodBuild.err != nil {
			return
		}
		portableDogfoodBuild.path = filepath.Join(portableDogfoodBuild.root, name)
		portableDogfoodBuild.err = workBuildCorvintWithoutVCS(portableDogfoodBuild.path)
	})
	if portableDogfoodBuild.err != nil {
		t.Fatal(portableDogfoodBuild.err)
	}
	binary := filepath.Join(t.TempDir(), "bin", name)
	workCopyExecutable(t, portableDogfoodBuild.path, binary)
	impostor := t.TempDir()
	if err := os.WriteFile(filepath.Join(impostor, "corvint"), []byte("#!/bin/sh\nexit 99\n"), 0755); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "PATH="+impostor+string(os.PathListSeparator)+os.Getenv("PATH"), "CORVINT_BIN="+filepath.Join(impostor, "missing"))
	return portableRun{binary: binary, env: env}
}

func (run portableRun) exec(t *testing.T, root string, extra []string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr strings.Builder
	command := exec.Command(run.binary, args...)
	command.Dir = root
	command.Env = append(append([]string{}, run.env...), extra...)
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return code, stdout.String(), stderr.String()
}

func (run portableRun) ok(t *testing.T, root string, args ...string) string {
	t.Helper()
	code, stdout, stderr := run.exec(t, root, nil, args...)
	if code != 0 {
		t.Fatalf("%v exit=%d stdout=%s stderr=%s", args, code, stdout, stderr)
	}
	return stdout
}

// DCW-V0-020, DCW-V0-021 and DCW-V0-023: the installed binary alone runs change, check and
// seal in a repository with no Corvint scripts, VERSION or source.
func TestDogfoodDailyPathRunsFromBinaryInForeignRepository(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, base := portableDogfoodRepo(t)
	inputsDir := t.TempDir()
	citations := filepath.Join(inputsDir, "citations.tsv")
	intents := filepath.Join(inputsDir, "intents")
	if err := os.WriteFile(citations, []byte("1\tintent.md\t1:5\tspecification\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(intents, []byte("intent.md\n"), 0644); err != nil {
		t.Fatal(err)
	}
	inputs := []string{"DOGFOOD_TASK=Answer two.", "DOGFOOD_VERIFY=go test ./fixture", "DOGFOOD_OUTCOME=passed", "DOGFOOD_CITATIONS=" + citations, "DOGFOOD_INTENTS_FILE=" + intents}
	// DCW-V0-015: the first pass prepares and cites an untracked sidecar with
	// every input supplied, and the real recorder, which runs last, refuses it.
	code, stdout, stderr := run.exec(t, root, inputs, "dogfood", "change", base)
	report, _ := os.ReadFile(filepath.Join(root, ".corvint/dogfood-report.json"))
	if code != 1 || !strings.Contains(stderr, "\n  local-outcome: record-index-failed\n") || !strings.Contains(string(report), `"complete": false`) {
		t.Fatalf("untracked sidecar exit=%d stderr=%s report=%s", code, stderr, report)
	}
	if status := cemGit(t, root, "status", "--porcelain", "--untracked-files=all"); status != "?? .corvint/change.cem.json" {
		t.Fatalf("first pass status %q", status)
	}
	// V1-0261: the check's fix lines name the subverb an adopter runs.
	if code, _, stderr = run.exec(t, root, nil, "dogfood", "check", base); code != 2 || !strings.HasSuffix(stderr, "\n  required order: commit the change; corvint dogfood change <sha>; commit .corvint/change.cem.json; corvint dogfood change <sha>; corvint dogfood check <sha>\n") {
		t.Fatalf("dirty check exit=%d stderr=%s", code, stderr)
	}
	cemGit(t, root, "add", ".corvint/change.cem.json")
	cemGit(t, root, "commit", "-qm", "chore: bind change evidence")
	bind := cemGit(t, root, "rev-parse", "HEAD")
	code, stdout, stderr = run.exec(t, root, inputs, "dogfood", "change", base)
	if code != 0 || stdout != "" || dropGuardingNote(stderr) != absentAgentReceipts {
		t.Fatalf("change exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	// DCW-V0-342: a complete pass closes with one PASS line naming the range and report.
	if want := "dogfood-change: PASS complete base=" + base + " target=" + bind + " report=" + root + "/.corvint/dogfood-report.json\n"; !strings.HasSuffix(stderr, want) {
		t.Fatalf("change stderr=%q, want suffix %q", stderr, want)
	}
	code, stdout, stderr = run.exec(t, root, nil, "dogfood", "check", base)
	// DCW-V0-033: PASS prints one summary line; the full status JSON is in the detail file.
	// OCM-V0-019: the plan cites FIXTURE-LCP-001 only, and fixture_test.go names it,
	// so the change references one requirement of the intent's two and links it.
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	detail := strings.TrimSpace(cemGit(t, root, "rev-parse", "--absolute-git-dir")) + "/corvint/dogfood-check.stdout"
	summary := "dogfood-check: SUMMARY cem=accepted hunks=1 supported=1 unknown=0 mechanical=0 unknownShare=0/1 reasonedUnknown=0 ocm=ready-for-review requirements=1 linked=1 unlinked=0 report=.corvint/dogfood-report.json detail=" + detail
	if code != 0 || len(lines) != 2 || lines[0] != summary || lines[1] != "dogfood-check: PASS" {
		t.Fatalf("check exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	// DCW-V0-112 and DCW-V0-165: the plan's cited hunk sits below anchored,
	// but no BASE line names fixture/fixture.go or fixture.go, so check
	// counts it in one stderr NOTE instead of warning.
	if strings.Contains(stderr, "dogfood-check: WARN tier-below-anchored ") || !strings.Contains(stderr, "dogfood-check: NOTE tier-below-anchored-unanchorable hunks=1 rule=CEM-V1-081 reason=") {
		t.Fatalf("check stderr lacks the unanchorable note: %s", stderr)
	}
	full, err := os.ReadFile(detail)
	if err != nil || strings.Count(string(full), "\n") != 2 || !strings.Contains(string(full), `"tool":"cem-status"`) || !strings.Contains(string(full), `"tool":"dogfood-ocm-status"`) {
		t.Fatalf("detail file %s err=%v", full, err)
	}
	// DOGFOOD_VERBOSE=1 restores the former full output: the same JSON lines, then PASS.
	if code, verbose, stderr := run.exec(t, root, []string{"DOGFOOD_VERBOSE=1"}, "dogfood", "check", base); code != 0 || verbose != string(full)+"dogfood-check: PASS\n" {
		t.Fatalf("verbose check exit=%d stdout=%s stderr=%s", code, verbose, stderr)
	}
	nested := filepath.Join(root, "fixture")
	if code, _, stderr = run.exec(t, nested, nil, "dogfood", "check", base); code != 2 || stderr != "dogfood-check: REFUSE not-repository-root\n" {
		t.Fatalf("nested root exit=%d stderr=%s", code, stderr)
	}
	code, stdout, stderr = run.exec(t, root, nil, "dogfood", "seal", base)
	if code != 0 || stdout != summary+"\ndogfood-check: PASS\ndogfood-seal: PASS sealed=.corvint/changes/"+bind+".cem.json\n" {
		t.Fatalf("seal exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	cemGit(t, root, "cat-file", "-e", "HEAD:.corvint/changes/"+bind+".cem.json")
	if subject := cemGit(t, root, "log", "-1", "--format=%s"); subject != "chore: seal change evidence" {
		t.Fatalf("seal commit %q", subject)
	}
}

// DCW-V0-046 (V1-1258): the installed binary binds a plan that keeps the only
// hunk unknown with a recorded reason, the check passes and reports the unknown
// share instead of refusing, and the seal keeps the reason in the sealed map.
func TestDogfoodDCWV0046ReasonedUnknownChecksAndSeals(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, base := portableDogfoodRepo(t)
	inputsDir := t.TempDir()
	citations := filepath.Join(inputsDir, "citations.tsv")
	intents := filepath.Join(inputsDir, "intents")
	for path, content := range map[string]string{citations: "1\tunknown\tinsufficient-evidence\tno governing line states the new answer\n", intents: "intent.md\n"} {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	inputs := []string{"DOGFOOD_TASK=Answer two.", "DOGFOOD_VERIFY=go test ./fixture", "DOGFOOD_OUTCOME=passed", "DOGFOOD_CITATIONS=" + citations, "DOGFOOD_INTENTS_FILE=" + intents}
	if code, _, stderr := run.exec(t, root, inputs, "dogfood", "change", base); code != 1 || !strings.Contains(stderr, "\n  local-outcome: record-index-failed\n") || strings.Contains(stderr, "cem-cite") || strings.Contains(stderr, "cem-status") {
		t.Fatalf("untracked sidecar exit=%d stderr=%s", code, stderr)
	}
	cemGit(t, root, "add", ".corvint/change.cem.json")
	cemGit(t, root, "commit", "-qm", "chore: bind change evidence")
	bind := cemGit(t, root, "rev-parse", "HEAD")
	if code, stdout, stderr := run.exec(t, root, inputs, "dogfood", "change", base); code != 0 || stdout != "" || dropGuardingNote(stderr) != absentAgentReceipts {
		t.Fatalf("change exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	code, stdout, stderr := run.exec(t, root, nil, "dogfood", "check", base)
	if code != 0 || !strings.HasPrefix(stdout, "dogfood-check: SUMMARY cem=accepted hunks=1 supported=0 unknown=1 mechanical=0 unknownShare=1/1 reasonedUnknown=1 ocm=") || !strings.HasSuffix(stdout, "\ndogfood-check: PASS\n") {
		t.Fatalf("check exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if code, stdout, stderr = run.exec(t, root, nil, "dogfood", "seal", base); code != 0 || !strings.HasSuffix(stdout, "dogfood-seal: PASS sealed=.corvint/changes/"+bind+".cem.json\n") {
		t.Fatalf("seal exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	var sealed struct {
		Hunks []struct {
			Disposition string `json:"disposition"`
			Reason      string `json:"reason"`
		} `json:"hunks"`
	}
	if err := json.Unmarshal([]byte(cemGit(t, root, "show", "HEAD:.corvint/changes/"+bind+".cem.json")), &sealed); err != nil || len(sealed.Hunks) != 1 ||
		sealed.Hunks[0].Disposition != "unknown" || sealed.Hunks[0].Reason != "insufficient-evidence" {
		t.Fatalf("sealed map hunks %+v err=%v", sealed.Hunks, err)
	}
	// The real verifier admits the sealed reasoned unknown hunk in batch-check.
	if code, stdout, stderr = run.exec(t, root, nil, "dogfood", "batch-check", base); code != 0 ||
		!strings.Contains(stdout, " reasonedUnknown=1 ") || !strings.Contains(stdout, "state=accepted") {
		t.Fatalf("batch-check exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}

// DCW-V0-024: a change no requirements spec governs completes, checks and
// seals only under the explicit #no-intent-declared manifest, and every OCM row
// and the check say intent linkage was not assessed. An unset variable or an
// empty file still refuses, and a link plan cannot join the declaration.
func TestDogfoodDailyPathCompletesWithDeclaredNoIntent(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, base := portableDogfoodRepo(t)
	inputsDir := t.TempDir()
	citations := filepath.Join(inputsDir, "citations.tsv")
	intents := filepath.Join(inputsDir, "intents")
	links := filepath.Join(inputsDir, "links.tsv")
	for path, content := range map[string]string{citations: "1\tAGENTS.md\t1:2\tspecification\n", intents: "", links: "#no-intent-declared\tFIXTURE-LCP-001\t1\tfixture/fixture_test.go\ttest:TestAnswer\n"} {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	inputs := []string{"DOGFOOD_TASK=Answer two.", "DOGFOOD_VERIFY=go test ./fixture", "DOGFOOD_OUTCOME=passed", "DOGFOOD_CITATIONS=" + citations}
	for _, accidental := range [][]string{inputs, append(inputs[:len(inputs):len(inputs)], "DOGFOOD_INTENTS_FILE="+intents)} {
		if code, _, stderr := run.exec(t, root, accidental, "dogfood", "change", base); code != 1 || !strings.Contains(stderr, "\n  ocm-aggregate: missing-intent-scope\n") {
			t.Fatalf("accidental absence exit=%d stderr=%s", code, stderr)
		}
	}
	if err := os.WriteFile(intents, []byte("#no-intent-declared\n"), 0644); err != nil {
		t.Fatal(err)
	}
	inputs = append(inputs, "DOGFOOD_INTENTS_FILE="+intents)
	if code, _, stderr := run.exec(t, root, append(inputs[:len(inputs):len(inputs)], "DOGFOOD_OCM_LINKS="+links), "dogfood", "change", base); code != 1 || !strings.Contains(stderr, "\n  ocm-links: invalid-ocm-link-plan\n") {
		t.Fatalf("link plan exit=%d stderr=%s", code, stderr)
	}
	cemGit(t, root, "add", ".corvint/change.cem.json")
	cemGit(t, root, "commit", "-qm", "chore: bind change evidence")
	bind := cemGit(t, root, "rev-parse", "HEAD")
	code, stdout, stderr := run.exec(t, root, inputs, "dogfood", "change", base)
	report, _ := os.ReadFile(filepath.Join(root, ".corvint/dogfood-report.json"))
	for _, want := range []string{
		`"complete": true`,
		`{"name": "ocm-prepare", "status": "NOT_PRODUCED", "reason": "no-intent-declared"}`,
		`{"name": "ocm-status", "status": "NOT_PRODUCED", "reason": "no-intent-declared"}`,
		`{"name": "ocm-aggregate", "status": "NOT_PRODUCED", "reason": "no-intent-declared"}`,
		"\n  \"ocmStatus\": {\"state\": \"NOT_ASSESSED\", \"reason\": \"no-intent-declared\"}\n",
		`"bootstrapUnknown": 0,`,
	} {
		if code != 0 || stdout != "" || dropGuardingNote(stderr) != absentAgentReceipts || !strings.Contains(string(report), want) {
			t.Fatalf("change exit=%d stderr=%s want %s in report=%s", code, stderr, want, report)
		}
	}
	snapshot := filepath.Join(root, ".corvint/change.ocm-intents")
	if err := os.WriteFile(snapshot, []byte("intent.md\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr = run.exec(t, root, nil, "dogfood", "check", base); code != 1 || !strings.HasSuffix(stderr, "\ndogfood-check: FAIL dogfood-report-drift\n") {
		t.Fatalf("swapped declaration exit=%d stderr=%s", code, stderr)
	}
	if err := os.WriteFile(snapshot, []byte("#no-intent-declared\n"), 0600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = run.exec(t, root, nil, "dogfood", "check", base)
	if code != 0 || !strings.HasPrefix(stdout, "dogfood-check: SUMMARY cem=accepted hunks=1 supported=1 unknown=0 mechanical=0 unknownShare=0/1 reasonedUnknown=0 ocm=NOT_ASSESSED report=.corvint/dogfood-report.json detail=") || !strings.HasSuffix(stdout, "\ndogfood-check: NOTE intent-linkage NOT_ASSESSED no-intent-declared\ndogfood-check: PASS\n") || strings.Count(stdout, "\n") != 3 || strings.Contains(stdout, "dogfood-ocm-status") {
		t.Fatalf("check exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	code, stdout, stderr = run.exec(t, root, nil, "dogfood", "seal", base)
	if code != 0 || !strings.HasSuffix(stdout, "dogfood-seal: PASS sealed=.corvint/changes/"+bind+".cem.json\n") {
		t.Fatalf("seal exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}

// Decision 0398: a later fix commit leaves the prepared map outdated, and the
// change pass regenerates it on the coded outdated-map refusal (CCF-V1-004).
func TestDogfoodChangeRegeneratesAMapTheNextFixCommitOutdates(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, base := portableDogfoodRepo(t)
	inputsDir := t.TempDir()
	citations := filepath.Join(inputsDir, "citations.tsv")
	intents := filepath.Join(inputsDir, "intents")
	for path, content := range map[string]string{citations: "1\tintent.md\t1:5\tspecification\n", intents: "intent.md\n"} {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	inputs := []string{"DOGFOOD_TASK=Answer two.", "DOGFOOD_VERIFY=go test ./fixture", "DOGFOOD_OUTCOME=passed", "DOGFOOD_CITATIONS=" + citations, "DOGFOOD_INTENTS_FILE=" + intents}
	if code, _, stderr := run.exec(t, root, inputs, "dogfood", "change", base); code != 1 {
		t.Fatalf("first pass exit=%d stderr=%s", code, stderr)
	}
	cemWrite(t, root, "fixture/fixture.go", "package fixture\nfunc Answer() int { return 1 + 1 }\n")
	cemGit(t, root, "commit", "-qam", "second fix")
	run.exec(t, root, inputs, "dogfood", "change", base)
	report, _ := os.ReadFile(filepath.Join(root, ".corvint/dogfood-report.json"))
	if !strings.Contains(string(report), `{"name": "cem-prepare", "status": "PRODUCED", "reason": "none"}`) {
		t.Fatalf("outdated map was not regenerated: %s", report)
	}
}

// DCW-V0-019, DCW-V0-117: cem cite only adds evidence, so a corrected plan that
// would join the resumed map's earlier citation is refused before any cite and
// names the delete-and-rerun step, which leaves only the corrected citation.
func TestDogfoodChangeNamesDeleteWhenACorrectedPlanJoinsEarlierCitations(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, base := portableDogfoodRepo(t)
	inputsDir := t.TempDir()
	citations := filepath.Join(inputsDir, "citations.tsv")
	intents := filepath.Join(inputsDir, "intents")
	if err := os.WriteFile(intents, []byte("intent.md\n"), 0644); err != nil {
		t.Fatal(err)
	}
	inputs := []string{"DOGFOOD_TASK=Answer two.", "DOGFOOD_VERIFY=go test ./fixture", "DOGFOOD_OUTCOME=passed", "DOGFOOD_CITATIONS=" + citations, "DOGFOOD_INTENTS_FILE=" + intents}
	// V1-0261: an adopter's fix line names the subverb, not the Corvint make target.
	note := "\n  cem-cite: the plan was added to citations the map already carried and never replaces them; to correct an earlier plan, delete .corvint/change.cem.json and rerun corvint dogfood change " + base + " (docs/DOGFOOD.md step 4)\n"
	pass := func(plan string) (string, int) {
		t.Helper()
		if err := os.WriteFile(citations, []byte(plan), 0644); err != nil {
			t.Fatal(err)
		}
		code, _, stderr := run.exec(t, root, inputs, "dogfood", "change", base)
		var cem struct{ Evidence []any }
		data, _ := os.ReadFile(filepath.Join(root, ".corvint/change.cem.json"))
		if err := json.Unmarshal(data, &cem); err != nil || code != 1 {
			t.Fatalf("pass exit=%d stderr=%s %v", code, stderr, err)
		}
		return stderr, len(cem.Evidence)
	}
	if stderr, evidence := pass("1\tintent.md\t1:5\tspecification\n"); strings.Contains(stderr, note) || evidence != 1 {
		t.Fatalf("first plan evidence=%d stderr=%s", evidence, stderr)
	}
	// DCW-V0-117 (V1-2267): the corrected plan is refused before any cite, so
	// the uncommitted map keeps only the first plan's citation.
	if stderr, evidence := pass("1\tintent.md\t1:3\tspecification\n"); strings.Contains(stderr, note) || !strings.Contains(stderr, "\n  cem-cite: citation-plan-recites-cited-hunk\n    fix: plan row 1 would change hunk ") || evidence != 1 {
		t.Fatalf("corrected plan evidence=%d stderr=%s", evidence, stderr)
	}
	if err := os.Remove(filepath.Join(root, ".corvint/change.cem.json")); err != nil {
		t.Fatal(err)
	}
	if stderr, evidence := pass("1\tintent.md\t1:3\tspecification\n"); strings.Contains(stderr, note) || evidence != 1 {
		t.Fatalf("after delete evidence=%d stderr=%s", evidence, stderr)
	}
}

// DCW-V0-019, V1-0386: the CEM contract fixes strict JSON, not a byte layout,
// so a rerun whose plan a committed compact map already carries keeps its bytes
// and leaves the worktree clean for check.
func TestDogfoodChangeKeepsTheEncodingOfACommittedMapItCitesNothingInto(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, base := portableDogfoodRepo(t)
	inputsDir := t.TempDir()
	citations := filepath.Join(inputsDir, "citations.tsv")
	intents := filepath.Join(inputsDir, "intents")
	for path, content := range map[string]string{citations: "1\tAGENTS.md\t1:2\tspecification\n", intents: "#no-intent-declared\n"} {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	inputs := []string{"DOGFOOD_TASK=Answer two.", "DOGFOOD_VERIFY=go test ./fixture", "DOGFOOD_OUTCOME=passed", "DOGFOOD_CITATIONS=" + citations, "DOGFOOD_INTENTS_FILE=" + intents}
	if code, _, stderr := run.exec(t, root, inputs, "dogfood", "change", base); code != 1 {
		t.Fatalf("first pass exit=%d stderr=%s", code, stderr)
	}
	cemGit(t, root, "add", ".corvint/change.cem.json")
	cemGit(t, root, "commit", "-qm", "chore: bind change evidence")
	mapPath := filepath.Join(root, ".corvint/change.cem.json")
	indented, _ := os.ReadFile(mapPath)
	var compact bytes.Buffer
	if err := json.Compact(&compact, indented); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mapPath, compact.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	cemGit(t, root, "commit", "-qam", "re-encode change evidence")
	code, _, stderr := run.exec(t, root, inputs, "dogfood", "change", base)
	kept, _ := os.ReadFile(mapPath)
	if status := cemGit(t, root, "status", "--porcelain"); code != 0 || dropGuardingNote(stderr) != absentAgentReceipts || !bytes.Equal(kept, compact.Bytes()) || status != "" {
		t.Fatalf("rerun exit=%d stderr=%s status=%q map=%s", code, stderr, status, kept)
	}
	if code, stdout, stderr := run.exec(t, root, nil, "dogfood", "check", base); code != 0 || !strings.HasSuffix(stdout, "dogfood-check: PASS\n") {
		t.Fatalf("check exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}

// V1-0272: a clone that borrows objects through alternates is refused, and the
// fix line names the repack remediation and the subverb an adopter reruns.
func TestDogfoodChangeNamesAlternatesRemediation(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, base := portableDogfoodRepo(t)
	if err := os.WriteFile(filepath.Join(root, ".git/objects/info/alternates"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	fix := "run git repack -a -d, delete .git/objects/info/alternates and .git/objects/info/commit-graphs, run git commit-graph write --reachable, then rerun corvint dogfood change " + base + "\n"
	code, _, stderr := run.exec(t, root, nil, "dogfood", "change", base)
	if code == 0 || !strings.Contains(stderr, "unsupported-object-alternates") || !strings.Contains(stderr, fix) {
		t.Fatalf("alternates exit=%d stderr=%s", code, stderr)
	}
}

// V1-0354, DCW-V0-014, DCW-V0-065 (V1-1341): a verification line with shell
// anchors or a quoted, bracketed -run group is refused unsupported-verify-syntax,
// and its fix line names the admitted syntax and the committed-script escape.
func TestDogfoodChangeNamesVerifySyntaxRemediation(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, base := portableDogfoodRepo(t)
	// The escape's script is committed with the change; the recorder checks
	// only the listed command's syntax, so committing it is the caller's part.
	cemWrite(t, root, "fixture/verify-split.sh", "go test ./fixture -run '^Test[A-C]'\n")
	cemGit(t, root, "add", "fixture/verify-split.sh")
	cemGit(t, root, "commit", "-qm", "verify script")
	verify := filepath.Join(t.TempDir(), "verify")
	if err := os.WriteFile(verify, []byte("go test ./fixture -run ^TestAnswer$\ngo test ./fixture -run '^Test[A-C]'\n"), 0644); err != nil {
		t.Fatal(err)
	}
	fix := "\n  local-outcome: unsupported-verify-syntax\n    fix: each DOGFOOD_VERIFY_FILE line is one command of ASCII letters, digits and _./:@=+, - only, with no quotes, ^, $, |, parentheses or other shell syntax, except one quoted -run alternation of test names such as -run 'TestA|TestB', -run '^(TestA|TestB)$' or -run '^TestA$|^TestB$' (DCW-V0-384, DCW-V0-801); write -run TestName instead of -run '^TestName$'; a command that needs quotes or regular-expression syntax, such as a split -run '^Test[A-C]' group, goes in a script committed with the change and is listed as sh path/to/script.sh (DCW-V0-065)\n"
	inputs := []string{"DOGFOOD_TASK=Answer two.", "DOGFOOD_OUTCOME=passed", "DOGFOOD_VERIFY_FILE=" + verify}
	// Bind the sidecar to verify the post-commit refusal as well.
	run.exec(t, root, inputs, "dogfood", "change", base)
	cemGit(t, root, "add", ".corvint/change.cem.json")
	cemGit(t, root, "commit", "-qm", "chore: bind change evidence")
	code, _, stderr := run.exec(t, root, inputs, "dogfood", "change", base)
	if code != 1 || !strings.Contains(stderr, fix) {
		t.Fatalf("verify syntax exit=%d stderr=%s", code, stderr)
	}
	// The escape the fix line names passes the same grammar.
	if err := os.WriteFile(verify, []byte("go test ./fixture -run TestAnswer\nsh fixture/verify-split.sh\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, _, stderr = run.exec(t, root, inputs, "dogfood", "change", base)
	report, err := os.ReadFile(filepath.Join(root, ".corvint", "dogfood-report.json"))
	if err != nil || strings.Contains(stderr, "unsupported-verify-syntax") || !strings.Contains(string(report), `{"name": "local-outcome", "status": "PRODUCED", "reason": "none"}`) {
		t.Fatalf("escape refused: err=%v report=%s stderr=%s", err, report, stderr)
	}
	// DCW-V0-384 (V1-3131): one quoted -run alternation of test names is
	// recorded as a typed argv entry whose quoted word is one argument.
	if err := os.WriteFile(verify, []byte("go test ./fixture -run 'TestAnswer|TestOther'\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, _, stderr = run.exec(t, root, inputs, "dogfood", "change", base)
	report, err = os.ReadFile(filepath.Join(root, ".corvint", "dogfood-report.json"))
	if err != nil || strings.Contains(stderr, "unsupported-verify") || !strings.Contains(string(report), `{"name": "local-outcome", "status": "PRODUCED", "reason": "none"}`) {
		t.Fatalf("quoted -run alternation refused: err=%v report=%s stderr=%s", err, report, stderr)
	}
}

// DCW-V0-034, V1-1123: syntax is refused on the first pass while the
// prepared sidecar is still uncommitted, then with the same reason after binding.
func TestDogfoodChangeDCWV0034VerifySyntaxBeforeBinding(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, base := portableDogfoodRepo(t)
	verify := filepath.Join(t.TempDir(), "verify")
	command := "go test ./fixture -run '^(A|[B])$'"
	if err := os.WriteFile(verify, []byte(command+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	inputs := []string{"DOGFOOD_TASK=Answer two.", "DOGFOOD_OUTCOME=passed", "DOGFOOD_VERIFY_FILE=" + verify}
	head := cemGit(t, root, "rev-parse", "HEAD")
	assertRefusal := func() {
		t.Helper()
		code, _, stderr := run.exec(t, root, inputs, "dogfood", "change", base)
		for _, want := range []string{"local-outcome: unsupported-verify-syntax", command, "ASCII letters, digits", "_./:@=+, - only"} {
			if code != 1 || !strings.Contains(stderr, want) {
				t.Fatalf("exit=%d missing %q: %s", code, want, stderr)
			}
		}
	}
	assertRefusal()
	if got := cemGit(t, root, "rev-parse", "HEAD"); got != head {
		t.Fatal("pre-commit pass changed HEAD")
	}
	if status := cemGit(t, root, "status", "--porcelain", "--untracked-files=all"); !strings.Contains(status, "?? .corvint/change.cem.json") {
		t.Fatalf("sidecar was not uncommitted: %s", status)
	}
	cemGit(t, root, "add", ".corvint/change.cem.json")
	cemGit(t, root, "commit", "-qm", "chore: bind change evidence")
	assertRefusal()
}

// DCW-V0-034: a clean file reaches the recorder on both passes.
func TestDogfoodChangeDCWV0034CleanVerifyFile(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, base := portableDogfoodRepo(t)
	verify := filepath.Join(t.TempDir(), "verify")
	if err := os.WriteFile(verify, []byte("go test ./fixture -run TestAnswer\n\n go vet ./fixture \n"), 0644); err != nil {
		t.Fatal(err)
	}
	inputs := []string{"DOGFOOD_TASK=Answer two.", "DOGFOOD_OUTCOME=passed", "DOGFOOD_VERIFY_FILE=" + verify}
	code, _, stderr := run.exec(t, root, inputs, "dogfood", "change", base)
	if code != 1 || !strings.Contains(stderr, "local-outcome: record-index-failed") {
		t.Fatalf("first pass exit=%d stderr=%s", code, stderr)
	}
	cemGit(t, root, "add", ".corvint/change.cem.json")
	cemGit(t, root, "commit", "-qm", "chore: bind change evidence")
	_, _, stderr = run.exec(t, root, inputs, "dogfood", "change", base)
	if strings.Contains(stderr, "local-outcome:") {
		t.Fatalf("clean file refused: %s", stderr)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".git/corvint/local-outcome.json"))
	if err != nil || !strings.Contains(string(raw), `"state":"recorded"`) {
		t.Fatalf("outcome=%s err=%v", raw, err)
	}
}

// V1-0435, DCW-V0-014: a verification line over the recorder's 512-character
// bound is refused record-failed, its fix line names the stderr file and the
// bounds, and that file names the cause.
func TestDogfoodChangeNamesVerifyLengthRemediation(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, base := portableDogfoodRepo(t)
	verify := filepath.Join(t.TempDir(), "verify")
	if err := os.WriteFile(verify, []byte("go test ./fixture -run "+strings.Repeat("A", 500)+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	inputs := []string{"DOGFOOD_TASK=Answer two.", "DOGFOOD_OUTCOME=passed", "DOGFOOD_VERIFY_FILE=" + verify}
	// Bind the sidecar to verify the post-commit refusal as well.
	run.exec(t, root, inputs, "dogfood", "change", base)
	cemGit(t, root, "add", ".corvint/change.cem.json")
	cemGit(t, root, "commit", "-qm", "chore: bind change evidence")
	code, _, stderr := run.exec(t, root, inputs, "dogfood", "change", base)
	fix := "\n  local-outcome: record-failed\n    fix: read "
	bounds := "/local-outcome.stderr for the cause; DOGFOOD_VERIFY_FILE holds at most 50 commands of at most 512 characters each, so split a longer command into several lines\n"
	if code != 1 || !strings.Contains(stderr, fix) || !strings.Contains(stderr, bounds) {
		t.Fatalf("verify length exit=%d stderr=%s", code, stderr)
	}
	cause, err := os.ReadFile(filepath.Join(root, ".git/corvint/local-outcome.stderr"))
	if err != nil || !strings.Contains(string(cause), "verification command exceeds 512 characters") {
		t.Fatalf("local-outcome.stderr=%q err=%v", cause, err)
	}
}

// impactAbstentionRepo is a repository whose one change native Go impact
// refuses: a text file with no Go module, or a Go file at the module root.
func impactAbstentionRepo(t *testing.T, module bool) (string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cemWrite(t, root, "AGENTS.md", "# Fixture authority\nUse intent.md to govern the fixture.\n")
	cemWrite(t, root, "intent.md", "# Intent\n\n## Requirements\n\n- `FIXTURE-LCP-001`: The answer is two.\n")
	file, before, after := "answer.txt", "answer 1\n", "answer 2\n"
	if module {
		cemWrite(t, root, "go.mod", "module example.com/portable\n\ngo 1.22\n")
		file, before, after = "answer.go", "package portable\nfunc Answer() int { return 1 }\n", "package portable\nfunc Answer() int { return 2 }\n"
	}
	cemWrite(t, root, file, before)
	cemGit(t, root, "init", "-q", "-b", "main")
	exclude := ".corvint/dogfood-report.json\n.corvint/change.ocm-intents\n.corvint/change.ocm-status.json\n.corvint/change.ocm.*.json\n.corvint/self-observations.jsonl\n.context-corvint/\n"
	if err = os.WriteFile(filepath.Join(root, ".git/info/exclude"), []byte(exclude), 0644); err != nil {
		t.Fatal(err)
	}
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "base")
	base := cemGit(t, root, "rev-parse", "HEAD")
	cemWrite(t, root, file, after)
	cemGit(t, root, "commit", "-qam", "change")
	return root, base
}

// DCW-V0-025 (proposed): impact's no-module refusal is a typed, visible
// abstention that keeps its own code, so the daily path completes, checks and
// seals in a repository native Go impact cannot analyse. Since ERI-V0-305 a
// module-root path is refused per path inside a produced range receipt, so
// that case completes with coordination-time-impact PRODUCED.
func TestDogfoodDailyPathCompletesWhenImpactRefusesTheRepositoryOrModuleRoot(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	for _, tc := range []struct {
		name   string
		module bool
		reason string
	}{{"no-module", false, "unsupported-impact-repository"}, {"module-root", true, ""}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, base := impactAbstentionRepo(t, tc.module)
			inputsDir := t.TempDir()
			citations := filepath.Join(inputsDir, "citations.tsv")
			intents := filepath.Join(inputsDir, "intents")
			if err := os.WriteFile(citations, []byte("1\tintent.md\t1:5\tspecification\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(intents, []byte("intent.md\n"), 0644); err != nil {
				t.Fatal(err)
			}
			inputs := []string{"DOGFOOD_TASK=Answer two.", "DOGFOOD_VERIFY=true", "DOGFOOD_OUTCOME=passed", "DOGFOOD_CITATIONS=" + citations, "DOGFOOD_INTENTS_FILE=" + intents}
			produced := tc.reason == ""
			note, row := "dogfood-change: NOTE coordination-time-impact NOT_PRODUCED "+tc.reason+"\n", `{"name": "coordination-time-impact", "status": "NOT_PRODUCED", "reason": "`+tc.reason+`"}`
			if produced {
				note, row = "", `{"name": "coordination-time-impact", "status": "PRODUCED", "reason": "none"}`
			}
			if code, _, stderr := run.exec(t, root, inputs, "dogfood", "change", base); code != 1 || !strings.HasPrefix(stderr, note) || strings.Contains(stderr, "coordination-time-impact") != !produced {
				t.Fatalf("first pass exit=%d stderr=%s", code, stderr)
			}
			cemGit(t, root, "add", ".corvint/change.cem.json")
			cemGit(t, root, "commit", "-qm", "chore: bind change evidence")
			code, stdout, stderr := run.exec(t, root, inputs, "dogfood", "change", base)
			report, _ := os.ReadFile(filepath.Join(root, ".corvint/dogfood-report.json"))
			for _, want := range []string{`"complete": true`, row} {
				if code != 0 || stdout != "" || dropGuardingNote(stderr) != note+absentAgentReceipts || !strings.Contains(string(report), want) {
					t.Fatalf("change exit=%d stderr=%s want %s in report=%s", code, stderr, want, report)
				}
			}
			artifact, _ := os.ReadFile(filepath.Join(cemGit(t, root, "rev-parse", "--absolute-git-dir"), "corvint/coordination-time-impact-abstention.json"))
			if produced != (len(artifact) == 0) || !produced && !strings.Contains(string(artifact), `"reason":"`+tc.reason+`"`) {
				t.Fatalf("abstention artifact %s", artifact)
			}
			if code, stdout, stderr = run.exec(t, root, nil, "dogfood", "check", base); code != 0 || !strings.HasSuffix(stdout, "dogfood-check: PASS\n") || strings.Contains(stderr, "dogfood-check: NOTE coordination-time-impact NOT_PRODUCED "+tc.reason+"\n") == produced {
				t.Fatalf("check exit=%d stdout=%s stderr=%s", code, stdout, stderr)
			}
			if code, stdout, stderr = run.exec(t, root, nil, "dogfood", "seal", base); code != 0 || !strings.Contains(stdout, "dogfood-seal: PASS") {
				t.Fatalf("seal exit=%d stdout=%s stderr=%s", code, stdout, stderr)
			}
		})
	}
}

// LCP-V0-014: finish runs the change and the final check in-process from the
// installed binary, ignoring CORVINT_BIN and the DOGFOOD_* inputs.
func TestDogfoodFinishRunsFromBinaryInForeignRepository(t *testing.T) {
	t.Parallel()
	root, base := portableDogfoodRepo(t)
	testDogfoodFinishFromBinary(t, root, base, false)
}

func TestTraceGitignoreFinishThenChangeCheckSeal(t *testing.T) {
	t.Parallel()
	t.Run("GPK-V0-050 emitted ignore rules survive the next clean workflow", func(t *testing.T) {
		root, base := portableDogfoodRepo(t)
		cemWrite(t, root, ".gitignore", "*.tmp\n")
		cemGit(t, root, "add", ".gitignore")
		cemGit(t, root, "commit", "-qm", "ignore temporary files")
		testDogfoodFinishFromBinary(t, root, base, true)
	})
}

func testDogfoodFinishFromBinary(t *testing.T, root, base string, ignoreRules bool) {
	t.Helper()
	run := portableDogfoodRunner(t)
	run.env = append(run.env, "DOGFOOD_OUTCOME=failed", "DOGFOOD_VERIFY=false", "DOGFOOD_CITATIONS="+filepath.Join(root, "missing.tsv"), "DOGFOOD_INTENTS_FILE="+filepath.Join(root, "missing"))
	key := localcompletion.HashSession(t.Name())
	plan, _ := json.Marshal(localcompletion.Plan{Base: base, Intents: []string{"intent.md"}, Checks: []localcompletion.Check{{ID: "actual-test", Argv: []string{"go", "test", "./fixture"}, TimeoutSeconds: 60}}})
	planPath := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(planPath, plan, 0600); err != nil {
		t.Fatal(err)
	}
	session := []string{"--session-key", key}
	run.ok(t, root, append([]string{"dogfood", "begin", "--plan", planPath}, session...)...)
	run.ok(t, root, "cem", "prepare", "--base", base, "--target", "HEAD")
	// CEM-V1-151: finish reads the default cem/1.0 change map end to end.
	if prepared, err := os.ReadFile(filepath.Join(root, ".corvint/change.cem.json")); err != nil || !strings.Contains(string(prepared), `"spec":"cem/1.0"`) {
		t.Fatalf("default prepare did not write cem/1.0: %v %.80s", err, prepared)
	}
	run.ok(t, root, "cem", "cite", "--map", ".corvint/change.cem.json", "--hunk", "1", "--evidence-path", "intent.md", "--lines", "1:5", "--relation", "specification")
	if ignoreRules {
		// Each hunk cites its own span, so the next change's plan is not uniform (DCW-V0-048).
		run.ok(t, root, "cem", "cite", "--map", ".corvint/change.cem.json", "--hunk", "2", "--evidence-path", "AGENTS.md", "--lines", "1:2", "--relation", "specification")
	}
	cemGit(t, root, "add", ".corvint/change.cem.json")
	cemGit(t, root, "commit", "-qm", "bind evidence")
	target := cemGit(t, root, "rev-parse", "HEAD")
	run.ok(t, root, "ocm", "prepare", "--map", ".corvint/change.ocm.001.json", "--cem", ".corvint/change.cem.json", "--intent", "intent.md", "--expected-base", base, "--target", target)
	run.ok(t, root, append([]string{"dogfood", "verify", "--check", "actual-test"}, session...)...)
	run.ok(t, root, "ocm", "link", "--map", ".corvint/change.ocm.001.json", "--cem", ".corvint/change.cem.json", "--obligation", "FIXTURE-LCP-001", "--hunk", "1", "--test-path", "fixture/fixture_test.go", "--claim", "test:TestAnswer/case:fixture-lcp", "--expected-base", base, "--target", target)
	var inspected, finished struct {
		OK     bool `json:"ok"`
		Policy struct {
			Lifecycle       string `json:"lifecycle"`
			Satisfied       bool   `json:"satisfied"`
			ReportSetDigest string `json:"reportSetDigest"`
		} `json:"policy"`
	}
	// Before review, finish runs the coordinator and the final check, then
	// exits 1 naming the report set to review; ok mirrors that exit (LCP-V0-017).
	code, output, stderr := run.exec(t, root, nil, append([]string{"dogfood", "finish"}, session...)...)
	if err := json.Unmarshal([]byte(output), &inspected); err != nil || code != 1 || inspected.OK || inspected.Policy.ReportSetDigest == "" {
		t.Fatalf("finish before review exit=%d stdout=%s stderr=%s %v", code, output, stderr, err)
	}
	run.ok(t, root, append([]string{"dogfood", "review", "--report-set", inspected.Policy.ReportSetDigest}, session...)...)
	output = run.ok(t, root, append([]string{"dogfood", "finish"}, session...)...)
	if err := json.Unmarshal([]byte(output), &finished); err != nil || !finished.OK || !finished.Policy.Satisfied || finished.Policy.Lifecycle != "satisfied" {
		t.Fatalf("finish: %s %v", output, err)
	}
	if ignoreRules {
		query := run.ok(t, root, "query", "--task", "ignore rules", "--limit", "1")
		var packet struct {
			Context struct {
				Learning struct {
					State string `json:"local_trace_state"`
					Count int    `json:"local_trace_count"`
				} `json:"learning"`
			} `json:"context"`
		}
		if err := json.Unmarshal([]byte(query), &packet); err != nil || packet.Context.Learning.State != "ready" || packet.Context.Learning.Count == 0 {
			t.Fatalf("query did not consume recorded trace: %v %s", err, query)
		}
		traces, err := os.ReadFile(filepath.Join(root, ".context-corvint/traces", target+".jsonl"))
		if err != nil || !bytes.Contains(traces, []byte(`".gitignore"`)) || !bytes.Contains(traces, []byte(`".corvint/change.cem.json"`)) {
			t.Fatalf("finish did not retain ignore rules and CEM: %v %s", err, traces)
		}
		inputsDir := t.TempDir()
		intents := filepath.Join(inputsDir, "intents")
		if err := os.WriteFile(intents, []byte("intent.md\n"), 0600); err != nil {
			t.Fatal(err)
		}
		citations := filepath.Join(inputsDir, "citations.tsv")
		if err := os.WriteFile(citations, []byte("1\tintent.md\t1:5\tspecification\n2\tAGENTS.md\t1:2\tspecification\n"), 0600); err != nil {
			t.Fatal(err)
		}
		inputs := []string{"DOGFOOD_TASK=ignore rules", "DOGFOOD_VERIFY=go test ./fixture", "DOGFOOD_OUTCOME=passed", "DOGFOOD_INTENTS_FILE=" + intents, "DOGFOOD_CITATIONS=" + citations}
		code, stdout, stderr := run.exec(t, root, inputs, "dogfood", "change", base)
		if code != 0 {
			t.Fatalf("next change: %d %s %s", code, stdout, stderr)
		}
		run.ok(t, root, "dogfood", "check", base)
		run.ok(t, root, "dogfood", "seal", base)
		if status := cemGit(t, root, "status", "--porcelain", "--untracked-files=all"); status != "" {
			t.Fatalf("workflow left dirty paths: %s", status)
		}
	}
}

func TestAggregateQualificationPublicWriterRaces(t *testing.T) {
	binary := os.Getenv("CORVINT_QUAL_CURRENT_BINARY")
	if binary == "" || !groupreap.OwnerAvailable() {
		t.Skip("requires pinned native binary")
	}
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	for _, second := range []string{"public", "enrolled"} {
		t.Run(second, func(t *testing.T) {
			root, base := portableDogfoodRepo(t)
			run := portableRun{binary: binary, env: os.Environ()}
			key := localcompletion.HashSession(t.Name())
			plan, _ := json.Marshal(localcompletion.Plan{Base: base, Intents: []string{"intent.md"}, Checks: []localcompletion.Check{{ID: "fixture", Argv: []string{"go", "test", "./fixture"}, TimeoutSeconds: 60}}})
			planPath := filepath.Join(t.TempDir(), "plan.json")
			if err := os.WriteFile(planPath, plan, 0600); err != nil {
				t.Fatal(err)
			}
			run.ok(t, root, "dogfood", "begin", "--plan", planPath, "--session-key", key)
			bin := t.TempDir()
			ready := filepath.Join(bin, "ready")
			release := filepath.Join(bin, "release")
			script := "#!/bin/sh\nif [ -d \"$QUAL_LOCK\" ] && [ ! -f \"$QUAL_READY\" ]; then printf '%s' \"$$\" > \"$QUAL_READY\"; while [ ! -f \"$QUAL_RELEASE\" ]; do /bin/sleep 0.02; done; fi\nexec \"$QUAL_REAL_GIT\" \"$@\"\n"
			quote := func(v string) string { return "'" + strings.ReplaceAll(v, "'", "'\\''") + "'" }
			script = strings.NewReplacer("\"$QUAL_LOCK\"", quote(filepath.Join(root, ".git/corvint/local-completion/operation.lock")), "\"$QUAL_READY\"", quote(ready), "\"$QUAL_RELEASE\"", quote(release), "\"$QUAL_REAL_GIT\"", quote(realGit)).Replace(script)
			if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0755); err != nil {
				t.Fatal(err)
			}
			first := exec.Command(binary, "dogfood", "change", base)
			first.Dir = root
			first.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "QUAL_LOCK="+filepath.Join(root, ".git/corvint/local-completion/operation.lock"), "QUAL_READY="+ready, "QUAL_RELEASE="+release, "QUAL_REAL_GIT="+realGit)
			owner, err := groupreap.Start(first)
			if err != nil {
				t.Fatal(err)
			}
			retired := false
			defer func() {
				if retired {
					return
				}
				owner.Stop()
				bound := time.Now().Add(20 * time.Second)
				result := owner.FinishBounded(groupreap.RetirementBound{Expired: func() bool { return !time.Now().Before(bound) }})
				if result.State != groupreap.Released {
					t.Errorf("race cleanup: %+v", result)
				}
			}()
			deadline := time.Now().Add(5 * time.Second)
			for {
				if _, err := os.Stat(ready); err == nil {
					break
				}
				if !time.Now().Before(deadline) {
					t.Fatal("first writer did not reach locked barrier")
				}
				time.Sleep(10 * time.Millisecond)
			}
			snapshot := func() map[string]string {
				result := map[string]string{}
				for _, prefix := range []string{".git/corvint", ".corvint"} {
					_ = filepath.WalkDir(filepath.Join(root, prefix), func(path string, e os.DirEntry, err error) error {
						if os.IsNotExist(err) {
							return nil
						}
						if err != nil {
							t.Fatal(err)
						}
						if !e.IsDir() {
							raw, err := os.ReadFile(path)
							if err != nil {
								t.Fatal(err)
							}
							result[path] = string(raw)
						}
						return nil
					})
				}
				return result
			}
			before := snapshot()
			args := []string{"dogfood", "change", base}
			if second == "enrolled" {
				args = []string{"dogfood", "finish", "--session-key", key}
			}
			code, out, stderr := run.exec(t, root, nil, args...)
			if code != 2 || !strings.Contains(stderr, "operation-in-progress") {
				t.Fatalf("second %s admitted: %d %s %s", second, code, out, stderr)
			}
			if after := snapshot(); !reflect.DeepEqual(before, after) {
				t.Fatal("contending process modified artifacts")
			}
			if err := os.WriteFile(release, []byte("release"), 0600); err != nil {
				t.Fatal(err)
			}
			// Cancel the first real public CLI after the contention observation.
			if err := first.Process.Signal(os.Interrupt); err != nil {
				t.Fatal(err)
			}
			select {
			case <-owner.Exited():
			case <-time.After(10 * time.Second):
				t.Fatal("first writer did not stop")
			}
			bound := time.Now().Add(20 * time.Second)
			result := owner.FinishBounded(groupreap.RetirementBound{Expired: func() bool { return !time.Now().Before(bound) }})
			if result.State != groupreap.Released {
				t.Fatalf("owned group not retired: %+v", result)
			}
			retired = true
			if _, err := os.Stat(filepath.Join(root, ".git/corvint/local-completion/operation.lock")); !os.IsNotExist(err) {
				t.Fatalf("lock not released after observed retirement: %v", err)
			}
			t.Logf("firstGroup=%d second=%s observed RELEASED", first.Process.Pid, second)
		})
	}
}
