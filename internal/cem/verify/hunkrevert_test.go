package verify

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/execfile"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/native"
)

func revertTestHunk(path string, oldStart, newStart int64, body ...string) *patch.Hunk {
	hunk := &patch.Hunk{NewPath: &path, OldPath: &path, ID: "h1"}
	for _, line := range body {
		hunk.Body = append(hunk.Body, patch.BodyLine{Prefix: line[0], Payload: []byte(line[1:] + "\n")})
		if line[0] != '+' {
			hunk.OldRange.Count++
		}
		if line[0] != '-' {
			hunk.NewRange.Count++
		}
	}
	hunk.OldRange.Start, hunk.NewRange.Start = oldStart, newStart
	return hunk
}

// TestCEMV1088ReverseHunkAppliesExactlyOneHunk pins reverse application:
// the new side at the new range is replaced by the old side, a pure insertion
// and a pure deletion both round-trip, and target bytes that are not the
// hunk's new side are refused rather than guessed.
func TestCEMV1088ReverseHunkAppliesExactlyOneHunk(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, target, want string
		hunk               *patch.Hunk
	}{
		{"modify", "a\nB\nc\n", "a\nb\nc\n", revertTestHunk("f.go", 1, 1, " a", "-b", "+B", " c")},
		{"insertion", "a\nx\ny\nb\n", "a\nb\n", revertTestHunk("f.go", 1, 1, " a", "+x", "+y", " b")},
		{"deletion at end", "a\n", "a\nz\n", revertTestHunk("f.go", 2, 1, "-z")},
	}
	for _, c := range cases {
		got, ok := reverseHunk([]byte(c.target), c.hunk)
		if !ok || string(got) != c.want {
			t.Fatalf("%s: reverseHunk = %q, %v; want %q", c.name, got, ok, c.want)
		}
	}
	if _, ok := reverseHunk([]byte("a\nB2\nc\n"), revertTestHunk("f.go", 1, 1, " a", "-b", "+B", " c")); ok {
		t.Fatal("reverseHunk accepted target bytes that are not the hunk's new side")
	}
	if _, ok := reverseHunk([]byte("a\n"), revertTestHunk("f.go", 1, 1, " a", "-b", "+B", " c")); ok {
		t.Fatal("reverseHunk accepted a range past the end of the file")
	}
}

// TestCEMV1088DeclaredTestsAreWhatGoTestRuns pins the named-test rule: only a
// top-level func TestX(t *testing.T) whose header lies inside the cited span is
// named; declaration-shaped text in a comment or a string never is
// (TCQ-V0-061, Codex review of V1-1266).
func TestCEMV1088DeclaredTestsAreWhatGoTestRuns(t *testing.T) {
	t.Parallel()
	file := "package p\n\nimport \"testing\"\n\n" +
		"func TestB(t *testing.T) {}\nfunc Test(t *testing.T) {}\nfunc Testing(t *testing.T) {}\n" +
		"type suite struct{}\n\nfunc (s suite) TestMethod(t *testing.T) {}\nfunc BenchmarkB(b *testing.B) {}\n  func TestIndented(t *testing.T) {}\n" +
		"func TestA( tt  *testing.T ) {}\nfunc TestHelper(t *testing.T, n int) {}\nfunc Test_x(t *testing.T) {}\n"
	whole := wire.Span{Start: 0, End: int64(len(file))}
	if got, want := declaredTests([]byte(file), whole), []string{"Test", "TestA", "TestB", "TestIndented", "Test_x"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("declaredTests = %q, want %q", got, want)
	}
	start := int64(strings.Index(file, "func TestA"))
	if got, want := declaredTests([]byte(file), wire.Span{Start: start, End: start + int64(len("func TestA( tt  *testing.T )"))}), []string{"TestA"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("exact header span = %q, want %q", got, want)
	}
	if got := declaredTests([]byte(file), wire.Span{Start: start, End: start + 12}); len(got) != 0 {
		t.Fatalf("a span ending inside the header named %q", got)
	}
	quoted := "package p\n\nimport \"testing\"\n\n// func TestGuard(t *testing.T) {}\nvar s = `\nfunc TestGuard(t *testing.T) {}\n`\n\nfunc TestGuard(t *testing.T) {}\n"
	end := int64(strings.LastIndex(quoted, "func TestGuard"))
	if got := declaredTests([]byte(quoted), wire.Span{Start: 0, End: end}); len(got) != 0 {
		t.Fatalf("a comment or string named %q", got)
	}
	if got := declaredTests([]byte("not go"), wire.Span{Start: 0, End: 6}); len(got) != 0 {
		t.Fatalf("an unparsable file named %q", got)
	}
}

// TestCEMV1089DecodeGoTestJSON pins the decoder over go test -json shapes:
// only a top-level test event sets a state, both build-failure shapes mark the
// build failed, the binary's timeout panic is a timeout, and an unknown action
// or undecodable stream is an error, never a verdict.
func TestCEMV1089DecodeGoTestJSON(t *testing.T) {
	t.Parallel()
	stream := `{"Action":"start","Package":"m/p"}
{"Action":"run","Package":"m/p","Test":"TestA"}
{"Action":"run","Package":"m/p","Test":"TestA/sub"}
{"Action":"fail","Package":"m/p","Test":"TestA/sub"}
{"Action":"pass","Package":"m/p","Test":"TestA"}
{"Action":"fail","Package":"m/p","Test":"TestB"}
{"Action":"skip","Package":"m/p","Test":"TestC"}
{"Action":"fail","Package":"m/p"}
`
	got := decodeGoTest([]byte(stream))
	if got.err != nil || got.buildFailed || !got.packageFailed || !reflect.DeepEqual(got.states, map[string]string{"TestA": "pass", "TestB": "fail", "TestC": "skip"}) {
		t.Fatalf("decodeGoTest = %+v", got)
	}
	buildFail := `{"ImportPath":"m/p [m/p.test]","Action":"build-output","Output":"undefined: Abs\n"}
{"ImportPath":"m/p [m/p.test]","Action":"build-fail"}
{"Action":"start","Package":"m/p"}
{"Action":"output","Package":"m/p","Output":"FAIL\tm/p [build failed]\n"}
{"Action":"fail","Package":"m/p","FailedBuild":"m/p [m/p.test]"}
`
	if got := decodeGoTest([]byte(buildFail)); got.err != nil || !got.buildFailed {
		t.Fatalf("build failure = %+v", got)
	}
	timeout := `{"Action":"output","Package":"m/p","Test":"TestA","Output":"panic: test timed out after 5m0s\n"}
{"Action":"fail","Package":"m/p","Test":"TestA"}
`
	if got := decodeGoTest([]byte(timeout)); !got.timedOut {
		t.Fatalf("timeout = %+v", got)
	}
	for _, bad := range []string{`{"Action":"explode","Package":"m/p"}`, `{"Action":"pass"`, "not json"} {
		if got := decodeGoTest([]byte(bad)); got.err == nil {
			t.Fatalf("decodeGoTest(%q) = %+v, want an error", bad, got)
		}
	}
}

// TestCEMV1089ClassifyRevert pins that held needs a consumer-decoded test
// failure of a test that passed at the target; every other shape is survived
// or not-run with its reason.
func TestCEMV1089ClassifyRevert(t *testing.T) {
	t.Parallel()
	eligible := []string{"TestA", "TestB"}
	states := func(pairs ...string) map[string]string {
		out := map[string]string{}
		for i := 0; i < len(pairs); i += 2 {
			out[pairs[i]] = pairs[i+1]
		}
		return out
	}
	cases := []struct {
		outcome runOutcome
		want    unitVerdict
	}{
		{runOutcome{states: states("TestA", "pass", "TestB", "fail"), packageFailed: true}, unitVerdict{result: RevertHeld, failing: []string{"TestB"}}},
		// A test-level fail in a package that passed was printed, not failed.
		{runOutcome{states: states("TestA", "pass", "TestB", "fail")}, unitVerdict{result: RevertNotRun, reason: RevertInfrastructure}},
		{runOutcome{states: states("TestA", "pass", "TestB", "pass")}, unitVerdict{result: RevertSurvived}},
		// CEM-V1-693: survived needs a terminal pass for every eligible test.
		{runOutcome{states: states("TestA", "pass"), packageFailed: true}, unitVerdict{result: RevertNotRun, reason: RevertNotObserved}},
		{runOutcome{states: states("TestA", "pass")}, unitVerdict{result: RevertNotRun, reason: RevertNotObserved}},
		{runOutcome{states: states("TestA", "pass", "TestB", "skip")}, unitVerdict{result: RevertNotRun, reason: RevertSkipped}},
		{runOutcome{states: states(), buildFailed: true}, unitVerdict{result: RevertNotRun, reason: RevertBreaksBuild}},
		{runOutcome{states: states("TestA", "fail"), buildFailed: true}, unitVerdict{result: RevertNotRun, reason: RevertBreaksBuild}},
		{runOutcome{states: states(), packageFailed: true}, unitVerdict{result: RevertNotRun, reason: RevertBreaksBuild}},
		{runOutcome{states: states("TestA", "fail"), timedOut: true}, unitVerdict{result: RevertNotRun, reason: RevertTimeout}},
		{runOutcome{states: states("TestA", "fail"), err: errors.New("x")}, unitVerdict{result: RevertNotRun, reason: RevertInfrastructure}},
		{runOutcome{states: states("TestA", "skip")}, unitVerdict{result: RevertNotRun, reason: RevertSkipped}},
		{runOutcome{states: states("TestOther", "fail")}, unitVerdict{result: RevertNotRun, reason: RevertNotObserved}},
	}
	for i, c := range cases {
		if got := classifyRevert(c.outcome, eligible); !reflect.DeepEqual(got, c.want) {
			t.Fatalf("case %d: classifyRevert = %+v, want %+v", i, got, c.want)
		}
	}
	at := []struct {
		outcome runOutcome
		want    string
	}{
		{runOutcome{states: states("TestA", "fail")}, RevertFailsWithoutRevert},
		{runOutcome{states: states(), buildFailed: true}, RevertFailsWithoutRevert},
		{runOutcome{states: states(), packageFailed: true}, RevertFailsWithoutRevert},
		{runOutcome{states: states("TestA", "skip")}, RevertSkipped},
		{runOutcome{states: states()}, RevertNotObserved},
		{runOutcome{timedOut: true}, RevertTimeout},
		{runOutcome{err: errors.New("x")}, RevertInfrastructure},
	}
	for i, c := range at {
		if got := c.outcome.notRunReason(eligible); got != c.want {
			t.Fatalf("target case %d: notRunReason = %q, want %q", i, got, c.want)
		}
	}
	// The stream go test -json produced (go1.27.2) for a TestMain that printed
	// "=== RUN", "--- FAIL: TestGuard" and "PASS" framing lines and exited 0:
	// the forged test-level fail never makes a row held.
	forged := `{"Action":"start","Package":"m/f"}
{"Action":"run","Package":"m/f","Test":"TestGuard"}
{"Action":"output","Package":"m/f","Test":"TestGuard","Output":"--- FAIL: TestGuard (0.00s)\n","OutputType":"frame"}
{"Action":"fail","Package":"m/f","Test":"TestGuard","Elapsed":0}
{"Action":"output","Package":"m/f","Output":"PASS\n","OutputType":"frame"}
{"Action":"pass","Package":"m/f","Elapsed":0.226}
`
	if got := classifyRevert(decodeGoTest([]byte(forged)), []string{"TestGuard"}); got.result == RevertHeld {
		t.Fatalf("a printed failure in a passing package held: %+v", got)
	}
}

// TestCEMV1088JudgeRevertsOnlyInsideTheCopy drives judgeCandidates with a
// recording runner: the reverted bytes exist only during the reverted run,
// the copy is restored afterwards, a per-run timeout is not-run, and only the
// runner's own observation makes a row held.
func TestCEMV1088JudgeRevertsOnlyInsideTheCopy(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tree, "p"), 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(tree, "p", "f.go")
	target := "a\nB\nc\n"
	if err := execfile.WriteFile(file, []byte(target), 0o755); err != nil {
		t.Fatal(err)
	}
	hunk := revertTestHunk("p/f.go", 1, 1, " a", "-b", "+B", " c")
	candidate := revertCandidate{hunk: hunk, kind: patch.KindModify, units: []testUnit{{dir: "p", names: []string{"TestA", "TestB"}}}, tests: []string{"p::TestA", "p::TestB"}}
	var seen []string
	runner := func(ctx context.Context, root, dir string, names []string) runOutcome {
		data, _ := os.ReadFile(filepath.Join(root, dir, "f.go"))
		seen = append(seen, string(data)+"|"+strings.Join(names, ","))
		if string(data) == target {
			return runOutcome{states: map[string]string{"TestA": "pass", "TestB": "fail"}}
		}
		return runOutcome{states: map[string]string{"TestA": "fail"}, packageFailed: true}
	}
	rows := []HunkRevertRow{{HunkID: "h1", Result: RevertNotRun, Failing: []string{}}}
	judgeCandidates(context.Background(), tree, []revertCandidate{candidate}, rows, DefaultHunkRevertLimits, runner)
	if want := []string{target + "|TestA,TestB", "a\nb\nc\n|TestA"}; !reflect.DeepEqual(seen, want) {
		t.Fatalf("runner saw %q, want %q", seen, want)
	}
	if rows[0].Result != RevertHeld || !reflect.DeepEqual(rows[0].Failing, []string{"TestA"}) {
		t.Fatalf("row = %+v", rows[0])
	}
	if data, _ := os.ReadFile(file); string(data) != target {
		t.Fatalf("copy not restored: %q", data)
	}
	if info, _ := os.Stat(file); info.Mode().Perm() != 0o755 {
		t.Fatalf("copy mode changed: %v", info.Mode())
	}
	blocking := func(ctx context.Context, root, dir string, names []string) runOutcome {
		<-ctx.Done()
		return runOutcome{states: map[string]string{"TestA": "fail"}}
	}
	rows = []HunkRevertRow{{HunkID: "h1", Result: RevertNotRun, Failing: []string{}}}
	judgeCandidates(context.Background(), tree, []revertCandidate{candidate}, rows, HunkRevertLimits{PerRun: 20 * time.Millisecond, Total: time.Minute}, blocking)
	if rows[0].Result != RevertNotRun || rows[0].Reason != RevertTimeout {
		t.Fatalf("timed-out row = %+v", rows[0])
	}
	expired, cancel := context.WithCancel(context.Background())
	cancel()
	rows = []HunkRevertRow{{HunkID: "h1", Result: RevertNotRun, Failing: []string{}}}
	judgeCandidates(expired, tree, []revertCandidate{candidate}, rows, DefaultHunkRevertLimits, runner)
	if rows[0].Reason != RevertTotalLimit {
		t.Fatalf("total-limit row = %+v", rows[0])
	}
	mismatch := candidate
	mismatch.hunk = revertTestHunk("p/f.go", 1, 1, " a", "-b", "+X", " c")
	rows = []HunkRevertRow{{HunkID: "h1", Result: RevertNotRun, Failing: []string{}}}
	judgeCandidates(context.Background(), tree, []revertCandidate{mismatch}, rows, DefaultHunkRevertLimits, runner)
	if rows[0].Reason != RevertMismatch {
		t.Fatalf("mismatch row = %+v", rows[0])
	}
}

// TestCEMV1692EachRunGetsAFreshSnapshot is CEM-V1-692: a test that creates a
// sentinel file and fails when one already exists would make an unrelated
// hunk held if the reverted run saw the baseline run's write. Every run gets
// a fresh snapshot of the pristine export, so the row survives, the export
// never sees the sentinel, and no run directory is left behind.
func TestCEMV1692EachRunGetsAFreshSnapshot(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	tree := filepath.Join(workspace, "tree")
	if err := os.MkdirAll(filepath.Join(tree, "p"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tree, "p", "f.go"), []byte("a\nB\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hunk := revertTestHunk("p/f.go", 1, 1, " a", "-b", "+B", " c")
	candidate := revertCandidate{hunk: hunk, kind: patch.KindModify, units: []testUnit{{dir: "p", names: []string{"TestSentinel"}}}, tests: []string{"p::TestSentinel"}}
	var roots []string
	runner := func(ctx context.Context, root, dir string, names []string) runOutcome {
		roots = append(roots, root)
		sentinel := filepath.Join(root, dir, "sentinel")
		if _, err := os.Stat(sentinel); err == nil {
			return runOutcome{states: map[string]string{"TestSentinel": "fail"}, packageFailed: true}
		}
		if err := os.WriteFile(sentinel, nil, 0o644); err != nil {
			return runOutcome{err: err}
		}
		if info, err := os.Stat(filepath.Join(filepath.Dir(root), "tmp")); err != nil || !info.IsDir() {
			return runOutcome{err: errors.New("run has no private tmp")}
		}
		return runOutcome{states: map[string]string{"TestSentinel": "pass"}}
	}
	rows := []HunkRevertRow{{HunkID: "h1", Result: RevertNotRun, Failing: []string{}}}
	judgeCandidates(context.Background(), tree, []revertCandidate{candidate}, rows, DefaultHunkRevertLimits, runner)
	if rows[0].Result != RevertSurvived {
		t.Fatalf("sentinel row = %+v, want survived (no cross-run state)", rows[0])
	}
	if len(roots) != 2 || roots[0] == roots[1] || roots[0] == tree || roots[1] == tree {
		t.Fatalf("runs used roots %q; want two distinct snapshots, never the export %q", roots, tree)
	}
	if _, err := os.Stat(filepath.Join(tree, "p", "sentinel")); !os.IsNotExist(err) {
		t.Fatalf("a run wrote into the pristine export: %v", err)
	}
	if entries, _ := os.ReadDir(workspace); len(entries) != 1 {
		t.Fatalf("workspace holds %d entries after judging, want only the export", len(entries))
	}
}

// TestCEMV1693SurvivedNeedsEveryEligiblePass is CEM-V1-693 at the row level:
// a package whose eligible test was skipped or never reported keeps the row
// from surviving even when another package survived, while a package with no
// eligible test does not.
func TestCEMV1693SurvivedNeedsEveryEligiblePass(t *testing.T) {
	t.Parallel()
	survived := unitVerdict{result: RevertSurvived}
	cases := []struct {
		verdicts    []unitVerdict
		result, why string
	}{
		{[]unitVerdict{survived, {result: RevertNotRun, reason: RevertSkipped}}, RevertNotRun, RevertSkipped},
		{[]unitVerdict{survived, {result: RevertNotRun, reason: RevertNotObserved}}, RevertNotRun, RevertNotObserved},
		{[]unitVerdict{{result: RevertNotRun, reason: RevertFailsWithoutRevert, noEligible: true}, survived}, RevertSurvived, ""},
		{[]unitVerdict{{result: RevertNotRun, reason: RevertFailsWithoutRevert, noEligible: true}}, RevertNotRun, RevertFailsWithoutRevert},
		{[]unitVerdict{survived, survived}, RevertSurvived, ""},
	}
	for i, c := range cases {
		row := HunkRevertRow{Result: RevertNotRun, Failing: []string{}}
		combine(&row, c.verdicts)
		if row.Result != c.result || row.Reason != c.why {
			t.Fatalf("case %d: row = %+v, want %s/%s", i, row, c.result, c.why)
		}
	}
}

// TestCEMV1088CopyWritesNeverFollowSymlinks pins the copy's file writes: a
// test run that replaced the reverted file, or a directory above it, with a
// symlink out of the copy makes the write fail, and the outside file is never
// touched (Codex review of V1-1266).
func TestCEMV1088CopyWritesNeverFollowSymlinks(t *testing.T) {
	t.Parallel()
	outside := filepath.Join(t.TempDir(), "caller.go")
	if err := os.WriteFile(outside, []byte("caller\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tree := t.TempDir()
	file := filepath.Join(tree, "p", "f.go")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := replaceCopyFile(tree, file, []byte("b\n"), 0o644); err != nil {
		t.Fatalf("regular replace: %v", err)
	}
	if data, _, err := readCopyFile(tree, file); err != nil || string(data) != "b\n" {
		t.Fatalf("read back %q, %v", data, err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, file); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readCopyFile(tree, file); err == nil {
		t.Fatal("read followed a file symlink")
	}
	if err := replaceCopyFile(tree, file, []byte("reverted\n"), 0o644); err == nil {
		t.Fatal("write followed a file symlink")
	}
	if err := os.RemoveAll(filepath.Join(tree, "p")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(outside), filepath.Join(tree, "p")); err != nil {
		t.Fatal(err)
	}
	if err := replaceCopyFile(tree, filepath.Join(tree, "p", "caller.go"), []byte("reverted\n"), 0o644); err == nil {
		t.Fatal("write followed a directory symlink")
	}
	if data, _ := os.ReadFile(outside); string(data) != "caller\n" {
		t.Fatalf("outside file changed: %q", data)
	}
	if err := replaceCopyFile(tree, outside, []byte("x\n"), 0o644); err == nil {
		t.Fatal("a path outside the copy was written")
	}
}

// TestCEMV1088GoTestProfileMatchesNative pins the falsifier's fixed go test
// profile to the native go-test runner so the two cannot drift.
func TestCEMV1088GoTestProfileMatchesNative(t *testing.T) {
	names := []string{"TestA", "TestB.x"}
	invocation, err := native.Build(tr.Request{Runner: "go-test", Executable: "go", ReportDir: "r", Project: ".", Selectors: []string{".::TestA", ".::TestB.x"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := goTestProfileArgv(".", names); !reflect.DeepEqual(got, invocation.Argv) {
		t.Fatalf("argv = %q, native %q", got, invocation.Argv)
	}
	if !reflect.DeepEqual(goTestProfileEnvironment, invocation.Environment) {
		t.Fatalf("environment = %v, native %v", goTestProfileEnvironment, invocation.Environment)
	}
	repository := t.TempDir()
	t.Setenv("GOCACHE", filepath.Join(repository, ".cache", "go-build"))
	t.Setenv("GOMODCACHE", "relative/mod")
	t.Setenv("GOPATH", filepath.Join(filepath.Dir(repository), "elsewhere"))
	environment := strings.Join(goTestEnvironment("/w", repository), "\n") + "\n"
	for _, pin := range []string{"GOTOOLCHAIN=local\n", "GOPROXY=off\n", "GOSUMDB=off\n", "GOWORK=off\n", "GOENV=off\n", "TMPDIR=/w/tmp\n",
		"GOCACHE=" + filepath.Join("/w", "gocache") + "\n", "GOMODCACHE=" + filepath.Join("/w", "gomodcache") + "\n",
		"GOPATH=" + filepath.Join(filepath.Dir(repository), "elsewhere") + "\n"} {
		if !strings.Contains(environment, pin) {
			t.Fatalf("run environment lacks %q:\n%s", pin, environment)
		}
	}
	if strings.Contains(environment, "GOFLAGS=") {
		t.Fatalf("run environment passes GOFLAGS through:\n%s", environment)
	}
}

// TestCEMV1088ExportTreeReadsOnlyObjects pins the disposable copy: blobs and
// modes from the target tree, and a symlink refused rather than dropped.
func TestCEMV1088ExportTreeReadsOnlyObjects(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", repository, "-c", "user.name=t", "-c", "user.email=t@e", "-c", "commit.gpgsign=false"}, args...)...)
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	if err := os.MkdirAll(filepath.Join(repository, "d"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := execfile.WriteFile(filepath.Join(repository, "d", "run.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-qm", "one")
	first := git("rev-parse", "HEAD")
	directory := filepath.Join(t.TempDir(), "tree")
	if err := exportTree(context.Background(), repository, first, directory, filepath.Join(filepath.Dir(directory), "tmp")); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(directory, "a.go")); string(data) != "package a\n" {
		t.Fatalf("a.go = %q", data)
	}
	if info, err := os.Stat(filepath.Join(directory, "d", "run.sh")); err != nil || info.Mode().Perm()&0o100 == 0 {
		t.Fatalf("run.sh mode = %v, %v", info, err)
	}
	if err := os.Symlink("a.go", filepath.Join(repository, "link")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	git("add", ".")
	git("commit", "-qm", "two")
	second := filepath.Join(t.TempDir(), "tree")
	if err := exportTree(context.Background(), repository, git("rev-parse", "HEAD"), second, filepath.Join(filepath.Dir(second), "tmp")); err == nil {
		t.Fatal("exportTree copied a tree with a symlink")
	}
}

// TestCEMV1088CountMovesOnlyAcceptedTiers pins the beside-result tally: a held
// row moves its hunk from anchored or cited to held, never from unknown.
func TestCEMV1088CountMovesOnlyAcceptedTiers(t *testing.T) {
	t.Parallel()
	report := &HunkRevert{HunksByClass: CitationClasses{Anchored: 1, Cited: 1, Unknown: 1}}
	report.count(HunkRevertRow{Result: RevertHeld}, TierAnchored)
	report.count(HunkRevertRow{Result: RevertHeld}, TierUnknown)
	report.count(HunkRevertRow{Result: RevertSurvived}, TierCited)
	report.count(HunkRevertRow{Result: RevertNotRun}, TierCited)
	want := CitationClasses{Held: 1, Cited: 1, Unknown: 1}
	if report.HunksByClass != want || report.Held != 2 || report.Survived != 1 || report.NotRun != 1 {
		t.Fatalf("report = %+v", report)
	}
}

// TestCEMV1320NotAttemptedHunksCarryAClosedReason pins CEM-V1-320 and
// CEM-V1-321: every mapped hunk the falsifier does not attempt is listed with
// one closed reason, in hunk ID order, and only a test-cited Go source hunk is
// a candidate. A map whose only hunk is a Ruby spec reports why, not nothing.
func TestCEMV1320NotAttemptedHunksCarryAClosedReason(t *testing.T) {
	t.Parallel()
	testFile := "package p\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\n"
	evidence := wire.Evidence{ID: "e1", BlobOid: "blob", Path: "p/a_test.go", Span: wire.Span{Start: 0, End: int64(len(testFile))}}
	claim := []wire.Basis{{EvidenceID: "e1", Relation: "test-claim"}}
	mapped := []wire.Hunk{
		{ID: "h6", Path: "p/cited.go", Disposition: "supported", Basis: claim},
		{ID: "h1", Path: "spec/models/user_spec.rb", Disposition: "supported", Basis: claim},
		{ID: "h2", Path: "p/a_test.go", Disposition: "supported", Basis: claim},
		{ID: "h3", Path: "p/unknown.go", Disposition: "unknown"},
		{ID: "h4", Path: "p/uncited.go", Disposition: "supported", Basis: []wire.Basis{{EvidenceID: "e1", Relation: "implementation"}}},
		{ID: "h5", Path: "p/missing.go", Disposition: "supported", Basis: claim},
	}
	group := &patch.Group{Kind: patch.KindModify}
	for _, hunk := range mapped {
		if hunk.ID != "h5" {
			group.Hunks = append(group.Hunks, revertTestHunk(hunk.Path, 1, 1, "+x"))
			group.Hunks[len(group.Hunks)-1].ID = hunk.ID
		}
	}
	capture := &stableCapture{change: &wire.Map{Hunks: mapped, Evidence: []wire.Evidence{evidence}}, parsed: &patch.Patch{Groups: []*patch.Group{group}}}
	candidates, notAttempted := revertCandidates(capture, func(string) ([]byte, error) { return []byte(testFile), nil })
	if len(candidates) != 1 || candidates[0].hunk.ID != "h6" || !reflect.DeepEqual(candidates[0].tests, []string{"p::TestA"}) {
		t.Fatalf("candidates = %+v, want only h6 naming p::TestA", candidates)
	}
	want := []HunkNotAttempted{
		{HunkID: "h1", Path: "spec/models/user_spec.rb", Reason: NotAttemptedLanguage},
		{HunkID: "h2", Path: "p/a_test.go", Reason: NotAttemptedTestOnly},
		{HunkID: "h3", Path: "p/unknown.go", Reason: NotAttemptedDisposition},
		{HunkID: "h4", Path: "p/uncited.go", Reason: NotAttemptedNoTestClaim},
		{HunkID: "h5", Path: "p/missing.go", Reason: NotAttemptedNotInPatch},
	}
	if !reflect.DeepEqual(notAttempted, want) {
		t.Fatalf("notAttempted = %+v, want %+v", notAttempted, want)
	}

	// A Ruby-only map: no candidate runs, no copy is exported, and the report
	// names the hunk instead of an empty list alone.
	ruby := &stableCapture{change: &wire.Map{Hunks: mapped[1:2]}, parsed: &patch.Patch{Groups: []*patch.Group{{Kind: patch.KindModify, Hunks: []*patch.Hunk{group.Hunks[1]}}}}}
	report := runHunkRevert(context.Background(), t.TempDir(), ruby, &Citation{}, DefaultHunkRevertLimits, func(context.Context, string, string, []string) runOutcome {
		t.Fatal("a Ruby-only map ran go test")
		return runOutcome{}
	})
	if len(report.Hunks) != 0 || report.NotRun != 0 || !reflect.DeepEqual(report.NotAttempted, want[:1]) {
		t.Fatalf("Ruby-only report = %+v", report)
	}
}
