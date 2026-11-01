package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
)

// finishLaneIdentity makes every commit the finish-lane run writes match the
// cemGit commits of a manual run byte for byte, so the two seal trees compare.
var finishLaneIdentity = []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_SYSTEM=" + os.DevNull,
	"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid",
	"GIT_AUTHOR_DATE=2000-01-01T00:00:00+0000", "GIT_COMMITTER_DATE=2000-01-01T00:00:00+0000"}

// finishLaneInputs writes the plan and intents and returns the bind-loop
// environment of the manual recipe and the finish-lane arguments.
func finishLaneInputs(t *testing.T, plan, intents string) ([]string, []string) {
	t.Helper()
	dir := t.TempDir()
	citations, manifest := filepath.Join(dir, "citations.tsv"), filepath.Join(dir, "intents")
	if err := os.WriteFile(citations, []byte(plan), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte(intents), 0644); err != nil {
		t.Fatal(err)
	}
	env := []string{"DOGFOOD_TASK=Answer two.", "DOGFOOD_VERIFY=go test ./fixture", "DOGFOOD_OUTCOME=passed", "DOGFOOD_CITATIONS=" + citations, "DOGFOOD_INTENTS_FILE=" + manifest}
	return env, []string{"--citations", citations, "--intents", manifest, "--outcome", "passed"}
}

// dropFinishLaneTiming removes the DCW-V0-115 TIMING lines, whose seconds
// vary per run.
func dropFinishLaneTiming(stderr string) string {
	var kept strings.Builder
	review := false
	for _, line := range strings.SplitAfter(stderr, "\n") {
		// The RUNS line (ASD-V0-907) and the REVIEW line with its next-step
		// line (DCW-V0-908) name per-run paths and digests, as TIMING does.
		if review && strings.HasPrefix(line, "  next: ") {
			review = false
			continue
		}
		review = strings.HasPrefix(line, "dogfood-finish-lane: REVIEW ")
		if !review && !strings.HasPrefix(line, "dogfood-finish-lane: TIMING ") && !strings.HasPrefix(line, "dogfood-finish-lane: RUNS ") {
			kept.WriteString(line)
		}
	}
	return kept.String()
}

func finishLaneSubjects(t *testing.T, root, base string) string {
	t.Helper()
	return cemGit(t, root, "log", "--format=%s", base+"..HEAD")
}

// DCW-V0-060 and DCW-V0-062: one command runs prepare, bind, the bind commit,
// rebind, check and seal, and its seal tree is the manual recipe's.
func TestDogfoodFinishLaneDCWV0060MatchesManualRecipe(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	manual, base := portableDogfoodRepo(t)
	root, finishBase := portableDogfoodRepo(t)
	if base != finishBase {
		t.Fatalf("fixtures differ: %s %s", base, finishBase)
	}
	env, args := finishLaneInputs(t, "1\tintent.md\t1:5\tspecification\n", "intent.md\n")
	// The manual recipe of docs/DOGFOOD.md §4 "Repair".
	if code, _, stderr := run.exec(t, manual, env[:3], "dogfood", "change", base); code != 1 || !strings.Contains(stderr, "  cem-cite: citation-plan-not-provided\n") {
		t.Fatalf("pass 0 exit=%d stderr=%s", code, stderr)
	}
	if err := os.Remove(filepath.Join(manual, ".corvint/change.cem.json")); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := run.exec(t, manual, env, "dogfood", "change", base); code != 1 || !strings.HasSuffix(stderr, "  local-outcome: record-index-failed\n    fix: the worktree has uncommitted changes (often the prepared sidecar); commit them, then rerun corvint dogfood change "+base+"\n  full report: "+manual+"/.corvint/dogfood-report.json\n") {
		t.Fatalf("pass 1 exit=%d stderr=%s", code, stderr)
	}
	cemGit(t, manual, "add", ".corvint/change.cem.json")
	cemGit(t, manual, "commit", "-qm", "chore: bind change evidence")
	if code, _, stderr := run.exec(t, manual, env, "dogfood", "change", base); code != 0 || dropGuardingNote(stderr) != absentAgentReceipts {
		t.Fatalf("pass 2 exit=%d stderr=%s", code, stderr)
	}
	if code, stdout, stderr := run.exec(t, manual, append(append([]string{}, env...), finishLaneIdentity...), "dogfood", "seal", base); code != 0 {
		t.Fatalf("manual seal exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}

	code, stdout, stderr := run.exec(t, root, append(env[:3:3], finishLaneIdentity...), append([]string{"dogfood", "finish-lane", "--base", base}, args...)...)
	if code != 0 || dropGuardingNote(dropFinishLaneTiming(stderr)) != absentAgentReceipts+"dogfood-check: NOTE unbound-commits NOT_OBSERVED previous-cem-absent\n" {
		t.Fatalf("finish-lane exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	// ASD-V0-907 and DCW-V0-908: a sealed run names its dogfood-change count
	// and the exact report still to be reviewed.
	if !strings.Contains(stderr, "\ndogfood-finish-lane: RUNS dogfood-change=") || !strings.Contains(stderr, "\ndogfood-finish-lane: REVIEW report="+filepath.Join(root, ".corvint", "dogfood-report.json")+" sha256=") {
		t.Fatalf("finish-lane printed no RUNS or REVIEW line: %s", stderr)
	}
	bind := cemGit(t, root, "rev-parse", "HEAD^")
	seal := cemGit(t, root, "rev-parse", "HEAD")
	detail := cemGit(t, root, "rev-parse", "--absolute-git-dir") + "/corvint/dogfood-check.stdout"
	want := "dogfood-finish-lane: prepare PASS hunks=1\n" +
		"dogfood-finish-lane: bind PASS\n" +
		"dogfood-finish-lane: commit PASS bind=" + bind + "\n" +
		"dogfood-finish-lane: rebind PASS\n" +
		"dogfood-finish-lane: check PASS\n" +
		"dogfood-finish-lane: seal PASS seal=" + seal + "\n" +
		// DCW-V0-062: every row a pass did not produce stays listed, even
		// the ones a later pass produced.
		// DCW-V0-069: the advisory guarding-tests row is listed by each
		// pass that prints it, though it never blocks completion.
		// DCW-V0-115: the prepare pass stops after the CEM prepare and the
		// bind pass leaves the recorder to the rebind, so neither lists the
		// rows the rebind decides.
		"dogfood-finish-lane: NOT_PRODUCED rows=2\n" +
		"  bind guarding-tests NOT_PRODUCED guarding-tests-not-run\n" +
		"  rebind guarding-tests NOT_PRODUCED guarding-tests-not-run\n" +
		"dogfood-check: SUMMARY cem=accepted hunks=1 supported=1 unknown=0 mechanical=0 unknownShare=0/1 reasonedUnknown=0 ocm=ready-for-review change-requirements=1 change-traced=1 change-untraced=0 report=.corvint/dogfood-report.json detail=" + detail + "\n" +
		"dogfood-check: PASS\n" +
		"dogfood-seal: PASS sealed=.corvint/changes/" + bind + ".cem.json\n"
	if stdout != want {
		t.Fatalf("finish-lane stdout:\n%s\nwant:\n%s", stdout, want)
	}
	// DCW-V0-166 (V1-2404): with no configured identity, the bind and seal
	// commits take the GIT_AUTHOR_* and GIT_COMMITTER_* environment.
	if got := cemGit(t, root, "log", "-2", "--format=%s|%an|%ae|%cn|%ce"); got != "chore: seal change evidence|t|t@example.invalid|t|t@example.invalid\nchore: bind change evidence|t|t@example.invalid|t|t@example.invalid" {
		t.Fatalf("finish-lane commit identities %q", got)
	}
	if got, manualTree := cemGit(t, root, "rev-parse", "HEAD^{tree}"), cemGit(t, manual, "rev-parse", "HEAD^{tree}"); got != manualTree || seal != cemGit(t, manual, "rev-parse", "HEAD") {
		t.Fatalf("seal tree %s, manual %s", got, manualTree)
	}
	// DCW-V0-115: each step's TIMING line and the per-command table, in
	// which only the rebind runs the coordination query, impact and recorder.
	for _, step := range []string{"prepare", "bind", "commit", "rebind", "check-seal"} {
		if !strings.Contains(stderr, "dogfood-finish-lane: TIMING "+step+" seconds=") {
			t.Fatalf("stderr lacks the %s TIMING line:\n%s", step, stderr)
		}
	}
	timings := cemGit(t, root, "rev-parse", "--absolute-git-dir") + "/corvint/dogfood-finish-lane.timings.tsv"
	if !strings.Contains(stderr, " detail="+timings+"\n") {
		t.Fatalf("stderr lacks the total TIMING line naming %s:\n%s", timings, stderr)
	}
	table, err := os.ReadFile(timings)
	if err != nil {
		t.Fatal(err)
	}
	ran := map[string]bool{}
	for _, row := range strings.Split(strings.TrimSuffix(string(table), "\n"), "\n")[1:] {
		fields := strings.Split(row, "\t")
		ran[fields[0]+" "+fields[1]] = true
	}
	for _, want := range []string{"prepare cem prepare", "bind cem cite", "rebind query", "rebind impact", "rebind dogfood-record", "check-seal cem status"} {
		if !ran[want] {
			t.Errorf("timings lack %q:\n%s", want, table)
		}
	}
	for _, unwanted := range []string{"prepare query", "prepare impact", "prepare ocm prepare", "prepare dogfood-record", "bind query", "bind impact", "bind dogfood-record"} {
		if ran[unwanted] {
			t.Errorf("timings keep %q:\n%s", unwanted, table)
		}
	}
	if subjects := finishLaneSubjects(t, root, base); subjects != "chore: seal change evidence\nchore: bind change evidence\nchange" {
		t.Fatalf("subjects %q", subjects)
	}
	if status := cemGit(t, root, "status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Fatalf("status %q", status)
	}
	// A rerun on the seal commit has nothing left to do.
	if code, stdout, _ = run.exec(t, root, finishLaneIdentity, append([]string{"dogfood", "finish-lane", "--base", base}, args...)...); code != 0 || stdout != "dogfood-finish-lane: seal SKIP already-sealed sealed=.corvint/changes/"+bind+".cem.json\n" {
		t.Fatalf("sealed rerun exit=%d stdout=%s", code, stdout)
	}
	// ASD-V0-842 (V1-3960): inputs-check on the seal head checks nothing, so
	// it reports NOT_CHECKED and exits 3 rather than a passing SKIP; the
	// wrappers' --sealed skip keeps the SKIP, and any other value is refused.
	sealedPath := ".corvint/changes/" + bind + ".cem.json"
	code, stdout, stderr = run.exec(t, root, finishLaneIdentity, append([]string{"dogfood", "inputs-check", "--base", base}, args...)...)
	if code != 3 || stdout != "dogfood-inputs-check: inputs-check NOT_CHECKED already-sealed sealed="+sealedPath+"\n" || !strings.Contains(stderr, "dogfood-inputs-check: NOT_CHECKED already-sealed head="+seal+"\n") {
		t.Fatalf("inputs-check on a seal head exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if code, stdout, _ = run.exec(t, root, finishLaneIdentity, append([]string{"dogfood", "inputs-check", "--base", base, "--sealed", "skip"}, args...)...); code != 0 || stdout != "dogfood-inputs-check: seal SKIP already-sealed sealed="+sealedPath+"\n" {
		t.Fatalf("inputs-check --sealed skip exit=%d stdout=%s", code, stdout)
	}
	if code, _, stderr = run.exec(t, root, finishLaneIdentity, append([]string{"dogfood", "inputs-check", "--base", base, "--sealed", "pass"}, args...)...); code != 2 || !strings.Contains(stderr, "invalid-dogfood-sealed-value") {
		t.Fatalf("inputs-check --sealed pass exit=%d stderr=%s", code, stderr)
	}
}

// DCW-V0-061, DCW-V0-379: a uniform plan stops at prepare, before any bind,
// with the cite step's reason, the plan's shape and its fix line, commits
// nothing, and a rerun with the corrected plan continues.
func TestDogfoodFinishLaneDCWV0061StopsOnUniformPlanAndResumes(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, base := portableDogfoodRepo(t)
	cemWrite(t, root, "fixture/fixture_test.go", "package fixture\nimport \"testing\"\nfunc TestAnswer(t *testing.T) {\n t.Run(\"FIXTURE-LCP-001\",func(t *testing.T){ if Answer()!=2 { t.Fatal(\"two\") } })\n}\n")
	cemGit(t, root, "commit", "-qam", "test")
	head := cemGit(t, root, "rev-parse", "HEAD")
	env, args := finishLaneInputs(t, "1\tintent.md\t1:5\tspecification\n2\tintent.md\t1:5\tspecification\n", "intent.md\n")
	code, stdout, stderr := run.exec(t, root, append(env[:3:3], finishLaneIdentity...), append([]string{"dogfood", "finish-lane", "--base", base}, args...)...)
	if code != 1 || !strings.Contains(stderr, "FAIL dogfood-inputs-precheck cem-cite citation-plan-uniform input=--citations "+args[1]+" rows=all hunks=2 span=intent.md:1:5 ") ||
		!strings.Contains(stderr, "\n  fix: the plan cites one span for every hunk: hunks=2 span=intent.md:1:5; ") ||
		!strings.Contains(stdout, "dogfood-finish-lane: prepare FAIL exit=1\ndogfood-finish-lane: NOT_PRODUCED rows=1\n  prepare dogfood-inputs-precheck NOT_PRODUCED citation-plan-uniform\n") {
		t.Fatalf("uniform exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if now := cemGit(t, root, "rev-parse", "HEAD"); now != head {
		t.Fatalf("a stopped run committed: %s", now)
	}
	if status := cemGit(t, root, "status", "--porcelain", "--untracked-files=all"); status != "?? .corvint/change.cem.json" {
		t.Fatalf("stopped status %q", status)
	}
	// The rerun removes the stopped run's sidecar and prepares again.
	if err := os.WriteFile(args[1], []byte("1\tintent.md\t1:5\tspecification\n2\tintent.md\t5:5\tspecification\n"), 0644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = run.exec(t, root, append(env[:3:3], finishLaneIdentity...), append([]string{"dogfood", "finish-lane", "--base", base}, args...)...)
	if code != 0 || !strings.HasSuffix(stdout, "\ndogfood-check: PASS\ndogfood-seal: PASS sealed=.corvint/changes/"+cemGit(t, root, "rev-parse", "HEAD^")+".cem.json\n") {
		t.Fatalf("corrected rerun exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if subjects := finishLaneSubjects(t, root, base); subjects != "chore: seal change evidence\nchore: bind change evidence\ntest\nchange" {
		t.Fatalf("subjects %q", subjects)
	}
}

// DCW-V0-380 (V1-2970): dogfood inputs-check refuses a uniform plan before
// any bind, passes the corrected plan, and commits nothing either way.
func TestDogfoodInputsCheckDCWV0380RefusesThenPassesWithoutCommitting(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, base := portableDogfoodRepo(t)
	cemWrite(t, root, "fixture/fixture_test.go", "package fixture\nimport \"testing\"\nfunc TestAnswer(t *testing.T) {\n t.Run(\"FIXTURE-LCP-001\",func(t *testing.T){ if Answer()!=2 { t.Fatal(\"two\") } })\n}\n")
	cemGit(t, root, "commit", "-qam", "test")
	head := cemGit(t, root, "rev-parse", "HEAD")
	env, args := finishLaneInputs(t, "1\tintent.md\t1:5\tspecification\n2\tintent.md\t1:5\tspecification\n", "intent.md\n")
	code, stdout, stderr := run.exec(t, root, env[:3], append([]string{"dogfood", "inputs-check", "--base", base}, args...)...)
	if code != 1 || !strings.Contains(stdout, "dogfood-inputs-check: prepare FAIL exit=1\n") || !strings.Contains(stderr, " citation-plan-uniform input=--citations ") {
		t.Fatalf("uniform exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if err := os.WriteFile(args[1], []byte("1\tintent.md\t1:5\tspecification\n2\tintent.md\t5:5\tspecification\n"), 0644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = run.exec(t, root, env[:3], append([]string{"dogfood", "inputs-check", "--base", base}, args...)...)
	if code != 0 || !strings.Contains(stdout, "dogfood-inputs-check: prepare PASS hunks=2\ndogfood-inputs-check: inputs-check PASS nothing was bound or committed\n") {
		t.Fatalf("corrected exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if now := cemGit(t, root, "rev-parse", "HEAD"); now != head {
		t.Fatalf("the pre-flight committed: %s", now)
	}
	if status := cemGit(t, root, "status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Fatalf("the pre-flight left %q", status)
	}
}

// DCW-V0-061, DCW-V0-945: an intent without one requirements section, which
// ocm-prepare would refuse after the bind commit, is refused with the same
// code before any step runs.
func TestDogfoodFinishLaneDCWV0061StopsOnInvalidRequirementsSection(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, base := portableDogfoodRepo(t)
	head := cemGit(t, root, "rev-parse", "HEAD")
	env, args := finishLaneInputs(t, "1\tintent.md\t1:5\tspecification\n", "AGENTS.md\n")
	code, stdout, stderr := run.exec(t, root, env[:3], append([]string{"dogfood", "finish-lane", "--base", base}, args...)...)
	if code != 2 || !strings.Contains(stderr, "dogfood-change: REFUSE intent-invalid\n  intent=AGENTS.md code=invalid-requirements-section: ") ||
		!strings.Contains(stdout, "dogfood-finish-lane: prepare FAIL exit=2\n") {
		t.Fatalf("invalid intent exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if now := cemGit(t, root, "rev-parse", "HEAD"); now != head {
		t.Fatalf("a stopped run committed: %s", now)
	}
}

// DCW-V0-061: a rerun from the bind commit continues at rebind and never
// commits the sidecar twice.
func TestDogfoodFinishLaneDCWV0061ContinuesFromBindCommit(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, base := portableDogfoodRepo(t)
	env, args := finishLaneInputs(t, "1\tintent.md\t1:5\tspecification\n", "intent.md\n")
	if code, _, stderr := run.exec(t, root, env, "dogfood", "change", base); code != 1 {
		t.Fatalf("bind exit=%d stderr=%s", code, stderr)
	}
	cemGit(t, root, "add", ".corvint/change.cem.json")
	cemGit(t, root, "commit", "-qm", "chore: bind change evidence")
	bind := cemGit(t, root, "rev-parse", "HEAD")
	code, stdout, stderr := run.exec(t, root, append(env[:3:3], finishLaneIdentity...), append([]string{"dogfood", "finish-lane", "--base", base}, args...)...)
	if code != 0 || !strings.HasPrefix(stdout, "dogfood-finish-lane: prepare SKIP bind-commit="+bind+"\ndogfood-finish-lane: bind SKIP bind-commit="+bind+"\ndogfood-finish-lane: commit SKIP bind-commit="+bind+"\ndogfood-finish-lane: rebind PASS\ndogfood-finish-lane: check PASS\n") ||
		!strings.Contains(stdout, "\ndogfood-finish-lane: NOT_PRODUCED rows=1\n  rebind guarding-tests NOT_PRODUCED guarding-tests-not-run\n") {
		t.Fatalf("resume exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if subjects := finishLaneSubjects(t, root, base); subjects != "chore: seal change evidence\nchore: bind change evidence\nchange" {
		t.Fatalf("subjects %q", subjects)
	}
}

// DCW-V0-060: uncommitted work, a HEAD at BASE and a missing plan are refused
// before any step runs.
func TestDogfoodFinishLaneDCWV0060RefusesBeforeAnyStep(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, base := portableDogfoodRepo(t)
	_, args := finishLaneInputs(t, "1\tintent.md\t1:5\tspecification\n", "intent.md\n")
	cemWrite(t, root, "notes.txt", "draft\n")
	code, stdout, stderr := run.exec(t, root, nil, append([]string{"dogfood", "finish-lane", "--base", base}, args...)...)
	if code != 2 || stdout != "" || stderr != "dogfood-finish-lane: REFUSE dirty-worktree\n  uncommitted: \"notes.txt\"\n  fix: commit the change (finish-lane binds only committed work) and remove every other uncommitted path, then rerun corvint dogfood finish-lane --base "+base+"\n" {
		t.Fatalf("dirty exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, ".corvint/dogfood-report.json")); !os.IsNotExist(err) {
		t.Fatalf("a refused run wrote a report: %v", err)
	}
	// DCW-V0-397: the refusal names at most five sorted paths, quoted, and counts the rest.
	for _, name := range []string{"a\x1b[31m.txt", "b.txt", "c.txt", "d.txt", "e.txt", "f.txt"} {
		cemWrite(t, root, name, "draft\n")
	}
	if code, _, stderr = run.exec(t, root, nil, append([]string{"dogfood", "finish-lane", "--base", base}, args...)...); code != 2 || !strings.HasPrefix(stderr, "dogfood-finish-lane: REFUSE dirty-worktree\n  uncommitted: \"a\\x1b[31m.txt\" \"b.txt\" \"c.txt\" \"d.txt\" \"e.txt\" and 2 more\n  fix: ") {
		t.Fatalf("bounded dirty sample exit=%d stderr=%q", code, stderr)
	}
	for _, name := range []string{"a\x1b[31m.txt", "b.txt", "c.txt", "d.txt", "e.txt", "f.txt", "notes.txt"} {
		if err := os.Remove(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	head := cemGit(t, root, "rev-parse", "HEAD")
	if code, _, stderr = run.exec(t, root, nil, append([]string{"dogfood", "finish-lane", "--base", head}, args...)...); code != 2 || !strings.HasPrefix(stderr, "dogfood-finish-lane: REFUSE head-not-ahead-of-base\n") {
		t.Fatalf("at base exit=%d stderr=%s", code, stderr)
	}
	if code, _, stderr = run.exec(t, root, nil, "dogfood", "finish-lane", "--base", base); code != 2 || !strings.HasPrefix(stderr, "dogfood-finish-lane: REFUSE citation-plan-not-provided\n") {
		t.Fatalf("no plan exit=%d stderr=%s", code, stderr)
	}
	if code, _, stderr = run.exec(t, root, nil, "dogfood", "finish-lane", base); code == 0 || !strings.Contains(stderr, "dogfood-base-required") {
		t.Fatalf("positional base exit=%d stderr=%s", code, stderr)
	}
}

// DCW-V0-061: a seal whose commit a hook rejects stops at seal; the rerun is
// refused with the restore command until the staged rename is undone, then
// seals, and a sealed HEAD is still admitted like its bind commit.
func TestDogfoodFinishLaneDCWV0061ResumesAfterRejectedSeal(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, base := portableDogfoodRepo(t)
	env, args := finishLaneInputs(t, "1\tintent.md\t1:5\tspecification\n", "intent.md\n")
	hook := filepath.Join(cemGit(t, root, "rev-parse", "--absolute-git-dir"), "hooks", "commit-msg")
	if err := os.MkdirAll(filepath.Dir(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := execfile.WriteFile(hook, []byte("#!/bin/sh\n! grep -q 'chore: seal change evidence' \"$1\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	finish := append([]string{"dogfood", "finish-lane", "--base", base}, args...)
	code, stdout, stderr := run.exec(t, root, append(env[:3:3], finishLaneIdentity...), finish...)
	bind := cemGit(t, root, "rev-parse", "HEAD")
	if code != 2 || !strings.Contains(stdout, "dogfood-finish-lane: commit PASS bind="+bind+"\ndogfood-finish-lane: rebind PASS\ndogfood-finish-lane: check PASS\ndogfood-finish-lane: seal FAIL exit=2\n") ||
		!strings.Contains(stdout, "\ndogfood-check: PASS\n") {
		t.Fatalf("rejected seal exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	restore := "git restore --source=HEAD --staged --worktree -- .corvint/change.cem.json .corvint/changes/" + bind + ".cem.json"
	if code, _, stderr = run.exec(t, root, finishLaneIdentity, finish...); code != 2 || !strings.HasPrefix(stderr, "dogfood-finish-lane: REFUSE dirty-worktree\n  uncommitted: \".corvint/change.cem.json\" \".corvint/changes/"+bind+".cem.json\"\n  fix: "+restore+", ") {
		t.Fatalf("staged rename rerun exit=%d stderr=%s", code, stderr)
	}
	cemGit(t, root, strings.Fields(restore)[1:]...)
	if err := os.Remove(hook); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = run.exec(t, root, append(env[:3:3], finishLaneIdentity...), finish...)
	if code != 0 || !strings.HasPrefix(stdout, "dogfood-finish-lane: prepare SKIP bind-commit="+bind+"\n") || !strings.HasSuffix(stdout, "dogfood-seal: PASS sealed=.corvint/changes/"+bind+".cem.json\n") {
		t.Fatalf("resumed seal exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	cemWrite(t, root, "notes.txt", "draft\n")
	if code, stdout, stderr = run.exec(t, root, nil, finish...); code != 2 || stdout != "" || stderr != "dogfood-finish-lane: REFUSE dirty-worktree\n  uncommitted: \"notes.txt\"\n  fix: HEAD is sealed; commit or remove every uncommitted path, then rerun corvint dogfood finish-lane --base "+base+"\n" {
		t.Fatalf("dirty sealed rerun exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}

// ICL-V0-010 (V1-1629): a second inputs-check over the inputs a passing one
// prepared replays its prepare line and keeps the receipt, so the closeout's
// own pre-flight and dogfood-finish's prepare once and the finish-lane run
// still reuses it; an input changed in between prepares afresh.
func TestDogfoodInputsCheckICLV0010ReusesAMatchingPreflight(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	for _, changed := range []bool{false, true} {
		root, base := portableDogfoodRepo(t)
		env, args := finishLaneInputs(t, "1\tintent.md\t1:5\tspecification\n", "intent.md\n")
		check := append([]string{"dogfood", "inputs-check", "--base", base}, args...)
		if code, stdout, stderr := run.exec(t, root, env[:3], check...); code != 0 || strings.Contains(stdout, "reused=") {
			t.Fatalf("first inputs-check exit=%d stdout=%s stderr=%s", code, stdout, stderr)
		}
		if changed {
			if err := os.WriteFile(args[1], []byte("1\tintent.md\t1:4\tspecification\n"), 0644); err != nil {
				t.Fatal(err)
			}
		}
		code, stdout, stderr := run.exec(t, root, env[:3], check...)
		if code != 0 || strings.Contains(stdout, "prepare PASS hunks=1 reused=inputs-check\n") == changed {
			t.Fatalf("changed=%t second inputs-check exit=%d stdout=%s stderr=%s", changed, code, stdout, stderr)
		}
		gitDir := cemGit(t, root, "rev-parse", "--absolute-git-dir")
		if _, err := os.Stat(gitDir + "/corvint/inputs-check-pass"); err != nil {
			t.Fatalf("changed=%t: the second inputs-check left no receipt: %v", changed, err)
		}
		if _, err := os.Stat(root + "/.corvint/change.cem.json"); !os.IsNotExist(err) {
			t.Fatalf("changed=%t: the second inputs-check left a map: %v", changed, err)
		}
		code, stdout, stderr = run.exec(t, root, append(env[:3:3], finishLaneIdentity...), append([]string{"dogfood", "finish-lane", "--base", base}, args...)...)
		if code != 0 || !strings.HasPrefix(stdout, "dogfood-finish-lane: prepare PASS hunks=1 reused=inputs-check\n") {
			t.Fatalf("changed=%t finish-lane exit=%d stdout=%s stderr=%s", changed, code, stdout, stderr)
		}
	}
}

// DCW-V0-887 (V1-2978): a finish-lane run after a passing inputs-check with
// the same base, HEAD, inputs and binary replays the pre-flight's prepare line
// and runs no cem prepare of its own; an input changed in between forces a
// fresh prepare.
func TestDogfoodFinishLaneDCWV0887ReusesThePreflightPrepare(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	for _, changed := range []bool{false, true} {
		root, base := portableDogfoodRepo(t)
		env, args := finishLaneInputs(t, "1\tintent.md\t1:5\tspecification\n", "intent.md\n")
		if code, stdout, stderr := run.exec(t, root, env[:3], append([]string{"dogfood", "inputs-check", "--base", base}, args...)...); code != 0 {
			t.Fatalf("inputs-check exit=%d stdout=%s stderr=%s", code, stdout, stderr)
		}
		if changed {
			if err := os.WriteFile(args[1], []byte("1\tintent.md\t1:4\tspecification\n"), 0644); err != nil {
				t.Fatal(err)
			}
		}
		code, stdout, stderr := run.exec(t, root, append(env[:3:3], finishLaneIdentity...), append([]string{"dogfood", "finish-lane", "--base", base}, args...)...)
		if code != 0 {
			t.Fatalf("changed=%t finish-lane exit=%d stdout=%s stderr=%s", changed, code, stdout, stderr)
		}
		table, err := os.ReadFile(cemGit(t, root, "rev-parse", "--absolute-git-dir") + "/corvint/dogfood-finish-lane.timings.tsv")
		if err != nil {
			t.Fatal(err)
		}
		prepared := strings.Contains(string(table), "\nprepare\tcem prepare\t")
		reused := strings.HasPrefix(stdout, "dogfood-finish-lane: prepare PASS hunks=1 reused=inputs-check\n")
		if prepared == !changed || reused == changed {
			t.Errorf("changed=%t: prepare ran=%t reused=%t\nstdout=%s\ntimings=%s", changed, prepared, reused, stdout, table)
		}
		if _, err := os.Stat(cemGit(t, root, "rev-parse", "--absolute-git-dir") + "/corvint/inputs-check-pass"); !os.IsNotExist(err) {
			t.Errorf("changed=%t: the pre-flight receipt was not consumed: %v", changed, err)
		}
	}
}
