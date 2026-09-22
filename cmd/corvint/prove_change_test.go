package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/mutate"
)

// proveChangeRepository commits the mutate fixture, then a second commit that
// changes calc.go, so `prove --base FIRST` judges a real committed range.
func proveChangeRepository(t *testing.T) (root, base string) {
	t.Helper()
	root = proveMutateRepository(t, proveMutateKillingTest)
	base = strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	commitCalc(t, root, strings.Replace(proveMutateCalc, "return a + b", "return b + a", 1), "commute Add")
	return root, base
}

func commitCalc(t *testing.T, root, source, message string) {
	t.Helper()
	calc := filepath.Join(root, "pkg", "calc", "calc.go")
	if err := os.WriteFile(calc, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", message)
}

func affectedTestRow(t *testing.T, receipt map[string]any) map[string]any {
	t.Helper()
	for _, row := range proofRows(t, receipt) {
		if stringAt(row, "kind") == kindAffectedTest && stringAt(row, "result") == "pkg/calc/calc_test.go" {
			return row
		}
	}
	t.Fatalf("no affected-test row: %v", receipt["proof"])
	return nil
}

// TestProveChangeModeAddsAffectedTestRowsAndJudgesThemUnderMutate pins
// FPK-V0-017: the embedded packet is impact --base's, the selector's test
// becomes an affected-test row citing the test blob at HEAD, NOT_RUN without
// --mutate and PASS with it, and the repository is never written.
func TestProveChangeModeAddsAffectedTestRowsAndJudgesThemUnderMutate(t *testing.T) {
	t.Parallel()
	root, base := proveChangeRepository(t)
	before := treeDigest(t, root)
	receipt, first, stderr, code := runProveCLI(t, root, "--base", base)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	row := affectedTestRow(t, receipt)
	if row["falsifier"] != falsifierMutant || row["falsified"] != falsifiedNotRun || row["authority"] != authorityAffected || row["path"] != "pkg/calc/calc_test.go" || len(stringAt(row, "blob_hash")) != 40 {
		t.Fatalf("without --mutate: %v", row)
	}
	affected := receipt["proof"].(map[string]any)["affected"].(map[string]any)
	if affected["selected"] != float64(1) || affected["scope"] == "" || len(stringAt(affected, "graph_digest")) == 0 {
		t.Fatalf("affected summary: %v", affected)
	}
	impactStdout, _, impactCode := runProveArguments(t, "--root", root, "impact", "--base", base)
	var impactEnvelope map[string]any
	if impactCode != 0 || json.Unmarshal(impactStdout, &impactEnvelope) != nil {
		t.Fatalf("impact --base: exit %d %s", impactCode, impactStdout)
	}
	embedded, _ := json.Marshal(receipt["packet"])
	expected, _ := json.Marshal(impactEnvelope["context"])
	if !bytes.Equal(embedded, expected) {
		t.Fatalf("embedded packet differs from impact --base:\n%s\n%s", embedded, expected)
	}
	if _, second, _, _ := runProveCLI(t, root, "--base", base); !bytes.Equal(first, second) {
		t.Fatal("two runs over one range differ")
	}
	receipt, _, stderr, code = runProveCLI(t, root, "--base", base, "--mutate")
	if code != 0 {
		t.Fatalf("--mutate exit %d: %s", code, stderr)
	}
	row = affectedTestRow(t, receipt)
	if row["falsified"] != falsifiedPass || !strings.Contains(stringAt(row, "detail"), "killed") {
		t.Fatalf("with --mutate: %v", row)
	}
	if receipt["state"] != "READY" || proofField(t, receipt, "proven_results") < 1 || proofField(t, receipt, "failed_results") != 0 {
		t.Fatalf("state %v proof %v", receipt["state"], receipt["proof"])
	}
	if after := treeDigest(t, root); after != before {
		t.Fatal("prove --base changed the repository")
	}
	// A change the cited test never exercises is judged on that change alone:
	// the mutants inside the new lines survive, and the claim fails.
	uncovered := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	commitCalc(t, root, strings.Replace(proveMutateCalc, "return a + b", "return b + a", 1)+"\nfunc Sub(a, b int) int { return a - b }\n", "add uncovered Sub")
	receipt, _, stderr, code = runProveCLI(t, root, "--base", uncovered, "--mutate")
	if code != 0 {
		t.Fatalf("uncovered --mutate exit %d: %s", code, stderr)
	}
	row = affectedTestRow(t, receipt)
	if row["falsified"] != falsifiedFail || !strings.Contains(stringAt(row, "detail"), "survived") {
		t.Fatalf("uncovered change: %v", row)
	}
}

// TestProveChangeModeRefusesPathsAndUntrackedWithBase keeps change mode a
// committed-range contract.
func TestProveChangeModeRefusesPathsAndUntrackedWithBase(t *testing.T) {
	t.Parallel()
	root, base := proveChangeRepository(t)
	for _, arguments := range [][]string{
		{"--base", base, "pkg/calc/calc.go"},
		{"--base", base, "--working-tree-untracked"},
		{"--base", "notacommit"},
	} {
		_, stdout, stderr, code := runProveCLI(t, root, arguments...)
		if code != 2 || len(stdout) != 0 || stderr == "" {
			t.Fatalf("%v: exit %d stdout %q stderr %q", arguments, code, stdout, stderr)
		}
	}
}

// TestAffectedRowMakesNoMutationClaimForAChangedTest: mutating a test proves
// nothing about the code, so such a row keeps falsifier none.
func TestAffectedRowMakesNoMutationClaimForAChangedTest(t *testing.T) {
	t.Parallel()
	if row := affectedRow("pkg/calc/calc_test.go", "pkg/calc/calc.go"); row.Falsifier != falsifierMutant {
		t.Fatalf("source change: %+v", row)
	}
	if row := affectedRow("pkg/calc/calc_test.go", "pkg/calc/calc_test.go"); row.Falsifier != falsifierNone {
		t.Fatalf("changed test: %+v", row)
	}
	if row := affectedRow("tests/test_engine.py", "src/pkg/engine.py"); row.Falsifier != falsifierMutant {
		t.Fatalf("python test: %+v", row)
	}
	if changed, ok := testClaimTarget(affectedClaimPrefix + "a.go"); !ok || changed != "a.go" {
		t.Fatalf("claim target: %q %v", changed, ok)
	}
}

// TestCitedPathsBindsAnUncheckedAffectedTestRow: FPK-V0-017 gives every
// affected-test row the blob at the revision, so a falsifier-none row is still
// read, and citeAffectedBlobs can bind it, instead of emitting an empty blob_hash.
func TestCitedPathsBindsAnUncheckedAffectedTestRow(t *testing.T) {
	t.Parallel()
	row := affectedRow("pkg/calc/calc_test.go", "pkg/calc/calc_test.go")
	if paths := citedPaths([]proveRow{row}, false); len(paths) != 1 || paths[0] != row.Path {
		t.Fatalf("cited paths = %q", paths)
	}
}

// TestHunkSpansFollowTheNewSideOfTheDiff: change mode mutates inside the
// change, so the spans are the diff's new-side ranges, a pure deletion
// naming the line before it.
func TestHunkSpansFollowTheNewSideOfTheDiff(t *testing.T) {
	t.Parallel()
	diff := "diff --git a/pkg/a.go b/pkg/a.go\n--- a/pkg/a.go\n+++ b/pkg/a.go\n@@ -3,2 +3,4 @@ func A\n+x\n@@ -10 +12 @@\n+y\n@@ -20,3 +21,0 @@\n-z\ndiff --git a/pkg/b.go b/pkg/b.go\n--- a/pkg/b.go\n+++ b/pkg/b.go\n@@ -1,0 +2,3 @@\n+w\ndiff --git a/pkg/gone.go b/pkg/gone.go\ndeleted file mode 100644\n--- a/pkg/gone.go\n+++ /dev/null\n@@ -1,3 +0,0 @@\n-x\ndiff --git \"a/pkg/caf\\303\\251.go\" \"b/pkg/caf\\303\\251.go\"\n--- \"a/pkg/caf\\303\\251.go\"\n+++ \"b/pkg/caf\\303\\251.go\"\n@@ -7 +7,2 @@\n+v\n"
	spans := parseHunkSpans(diff)
	if len(spans) != 3 {
		t.Fatalf("spans for %d paths, want 3 (a deleted file names nothing): %v", len(spans), spans)
	}
	for path, expected := range map[string]string{"pkg/a.go": "3-6,12-12,21-21", "pkg/b.go": "2-4", "pkg/café.go": "7-8"} {
		parts := make([]string, 0, len(spans[path]))
		for _, span := range spans[path] {
			parts = append(parts, fmt.Sprintf("%d-%d", span.Start, span.End))
		}
		if got := strings.Join(parts, ","); got != expected {
			t.Fatalf("%s spans = %s, want %s", path, got, expected)
		}
	}
}

// TestConfinedSpansAdmitNothingForAPathWithoutHunks: impact mode has no span
// map and admits every line; in change mode a path the diff gave no hunk
// (a mode-only change, an unparsed header) admits no mutant at all.
func TestConfinedSpansAdmitNothingForAPathWithoutHunks(t *testing.T) {
	t.Parallel()
	if lines, ok := confinedSpans(nil, "pkg/a.go"); !ok || lines != nil {
		t.Fatalf("impact mode: %v %v", lines, ok)
	}
	spans := map[string][]mutate.LineSpan{"pkg/a.go": {{Start: 3, End: 4}}}
	if lines, ok := confinedSpans(spans, "pkg/a.go"); !ok || len(lines) != 1 {
		t.Fatalf("hunked path: %v %v", lines, ok)
	}
	if _, ok := confinedSpans(spans, "pkg/mode-only.go"); ok {
		t.Fatal("a path without hunks admitted mutants")
	}
}

// TestProveMutateInvocationBudgetLeavesLaterRowsNotRun: once the invocation's
// mutation budget is spent, remaining mutation rows are NOT_RUN with the
// budget detail instead of running late.
func TestProveMutateInvocationBudgetLeavesLaterRowsNotRun(t *testing.T) {
	t.Parallel()
	root, base := proveChangeRepository(t)
	ctx := proveBoundsContext(func(bounds *proveBounds) { bounds.mutationBudget = time.Nanosecond })
	receipt, _, stderr, code := runProveCLIContext(t, ctx, root, "--base", base, "--mutate")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	row := affectedTestRow(t, receipt)
	if row["falsified"] != falsifiedNotRun || stringAt(row, "detail") != budgetExhaustedDetail {
		t.Fatalf("budgeted row: %v", row)
	}
}

// proveOtherPackage is a dependent of calc that never calls Add: its test is
// reached from a change to calc.go but pins nothing inside the commuted line.
const proveOtherPackage = "package other\n\nimport \"example.test/prove/pkg/calc\"\n\nfunc Positive(n int) bool { return calc.IsPositive(n) }\n"

const proveOtherTest = "package other\n\nimport \"testing\"\n\nfunc TestPositive(t *testing.T) {\n\tif !Positive(1) || Positive(-1) {\n\t\tt.Fatal(\"Positive\")\n\t}\n}\n"

// proveChangeRepositoryWithDependent is proveChangeRepository plus a second
// package importing calc, committed before the base, so the selector reaches
// two tests from the one changed path.
func proveChangeRepositoryWithDependent(t *testing.T) (root, base string) {
	t.Helper()
	root = proveMutateRepository(t, proveMutateKillingTest)
	writeFixtureFiles(t, root, map[string]string{"pkg/other/other.go": proveOtherPackage, "pkg/other/other_test.go": proveOtherTest})
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "dependent package")
	base = strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	commitCalc(t, root, strings.Replace(proveMutateCalc, "return a + b", "return b + a", 1), "commute Add")
	return root, base
}

func writeFixtureFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for relative, content := range files {
		file := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func affectedPathOf(t *testing.T, receipt map[string]any) map[string]any {
	t.Helper()
	affected, _ := receipt["proof"].(map[string]any)["affected"].(map[string]any)
	paths := mapsFromAny(affected["paths"])
	if len(paths) != 1 || stringAt(paths[0], "path") != "pkg/calc/calc.go" {
		t.Fatalf("affected paths: %v", affected)
	}
	return paths[0]
}

func affectedRowVerdicts(t *testing.T, receipt map[string]any) map[string]string {
	t.Helper()
	verdicts := map[string]string{}
	for _, row := range proofRows(t, receipt) {
		if stringAt(row, "kind") == kindAffectedTest {
			verdicts[stringAt(row, "result")] = stringAt(row, "falsified")
		}
	}
	return verdicts
}

// TestProveChangeModeSummarizesCoveragePerChangedPath pins decision 0018: a
// change pinned by one reached test and merely reached by another is one
// proven result and no failed result, the rows keep their own verdicts, and
// proof.affected.paths names the covering test beside the reached count.
func TestProveChangeModeSummarizesCoveragePerChangedPath(t *testing.T) {
	t.Parallel()
	root, base := proveChangeRepositoryWithDependent(t)
	receipt, _, stderr, code := runProveCLI(t, root, "--base", base, "--mutate")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	verdicts := affectedRowVerdicts(t, receipt)
	if verdicts["pkg/calc/calc_test.go"] != falsifiedPass || verdicts["pkg/other/other_test.go"] != falsifiedFail {
		t.Fatalf("row verdicts: %v", verdicts)
	}
	path := affectedPathOf(t, receipt)
	covered, _ := json.Marshal(path["covered_by"])
	if string(covered) != `["pkg/calc/calc_test.go"]` || integerAt(path, "reached_not_covering") != 1 || integerAt(path, "not_run") != 0 {
		t.Fatalf("path summary: %v", path)
	}
	if proofField(t, receipt, "failed_results") != packetVerdicts(t, receipt)[falsifiedFail] || proofField(t, receipt, "proven_results") != 1+packetVerdicts(t, receipt)[falsifiedPass] || receipt["state"] != "READY" {
		t.Fatalf("summary: %v", receipt["proof"])
	}
	counts := receipt["proof"].(map[string]any)["counts"].(map[string]any)[falsifierMutant].(map[string]any)
	if integerAt(counts, falsifiedFail) != 1 {
		t.Fatalf("per-row counts no longer disclose the reached row: %v", counts)
	}
}

// TestProveChangeModeReportsAnUncoveredPathAsOneFailedResult: when every
// reached test lets the change's mutants survive, the path is one failed
// result, not one per reached test.
func TestProveChangeModeReportsAnUncoveredPathAsOneFailedResult(t *testing.T) {
	t.Parallel()
	root, _ := proveChangeRepositoryWithDependent(t)
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	commitCalc(t, root, strings.Replace(proveMutateCalc, "return a + b", "return b + a", 1)+"\nfunc Sub(a, b int) int { return a - b }\n", "add uncovered Sub")
	receipt, _, stderr, code := runProveCLI(t, root, "--base", base, "--mutate")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	verdicts := affectedRowVerdicts(t, receipt)
	if verdicts["pkg/calc/calc_test.go"] != falsifiedFail || verdicts["pkg/other/other_test.go"] != falsifiedFail {
		t.Fatalf("row verdicts: %v", verdicts)
	}
	path := affectedPathOf(t, receipt)
	if len(mapsFromAny(path["covered_by"])) != 0 || integerAt(path, "reached_not_covering") != 2 {
		t.Fatalf("path summary: %v", path)
	}
	if proofField(t, receipt, "failed_results") != 1+packetVerdicts(t, receipt)[falsifiedFail] {
		t.Fatalf("summary: %v", receipt["proof"])
	}
}

// packetVerdicts tallies the packet's own results, which keep per-row
// counting, so the affected-test contribution can be asserted on its own.
func packetVerdicts(t *testing.T, receipt map[string]any) map[string]int {
	t.Helper()
	rows := make([]proveRow, 0)
	for _, row := range proofRows(t, receipt) {
		rows = append(rows, proveRow{Kind: stringAt(row, "kind"), Result: stringAt(row, "result"), Falsifier: stringAt(row, "falsifier"), Falsified: stringAt(row, "falsified")})
	}
	tally := map[string]int{}
	for _, result := range mapsFromAny(receipt["packet"].(map[string]any)["results"]) {
		tally[resultVerdict(stringAt(result, "kind"), stringAt(result, "id"), rows)]++
	}
	return tally
}

// TestProveRefusesAGitStreamPastItsByteBoundBeforeBufferingIt: the three
// Git streams prove reads are refused at bound+1 bytes with the same code
// and message as before, without buffering the rest.
func TestProveRefusesAGitStreamPastItsByteBoundBeforeBufferingIt(t *testing.T) {
	t.Parallel()
	root, base := proveBoundRepository(t)
	for _, bound := range []struct {
		name    string
		lower   func(*proveBounds)
		message string
	}{
		{"changed paths", func(bounds *proveBounds) { bounds.rangeChangeBytes = 4 }, "the range's changed paths exceed the byte bound"},
		{"diff", func(bounds *proveBounds) { bounds.rangeDiffBytes = 16 }, "the range's diff exceeds the byte bound"},
		{"cited blobs", func(bounds *proveBounds) { bounds.blobBytes = 1 }, "cited blobs exceed the proof output bound"},
	} {
		_, stdout, stderr, code := runProveCLIContext(t, proveBoundsContext(bound.lower), root, "--base", base)
		if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, "unsupported-prove-history") || !strings.Contains(stderr, bound.message) {
			t.Fatalf("%s: exit %d stdout %q stderr %q", bound.name, code, stdout, stderr)
		}
	}
	if _, _, stderr, code := runProveCLI(t, root, "--base", base); code != 0 {
		t.Fatalf("bounds restored: exit %d: %s", code, stderr)
	}
}

// proveBoundRepository is a committed Go range that needs no sandbox: two
// commits over one file, so `prove --base FIRST` reads all three Git streams.
func proveBoundRepository(t *testing.T) (root, base string) {
	t.Helper()
	root = cliRepository(t)
	writeFixtureFiles(t, root, map[string]string{"go.mod": "module example.test/prove\n\ngo 1.27.0\n", "pkg/calc/calc.go": proveMutateCalc, "pkg/calc/calc_test.go": proveMutateKillingTest, ".gitignore": ".corvint/\n"})
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "fixture")
	base = strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	commitCalc(t, root, strings.Replace(proveMutateCalc, "return a + b", "return b + a", 1), "commute Add")
	return root, base
}

// TestMutationGroupsShareOneRunnerCallPerChangedPath: Go rows claiming the
// same changed path form one group under their first row's index, in
// document order; a row judgeMutation decides before any run stays out.
func TestMutationGroupsShareOneRunnerCallPerChangedPath(t *testing.T) {
	t.Parallel()
	blob := citedBlob{oid: "a1", objectType: "blob"}
	cited := map[string]citedBlob{"pkg/a/a_test.go": blob, "pkg/b/b_test.go": blob, "pkg/c/c_test.go": blob}
	rows := []proveRow{
		{Falsifier: falsifierMutant, Path: "pkg/a/a_test.go", BlobHash: "a1", reason: affectedClaimPrefix + "pkg/a/a.go"},
		{Falsifier: falsifierNone, Path: "pkg/a/a.go"},
		{Falsifier: falsifierMutant, Path: "pkg/b/b_test.go", BlobHash: "a1", reason: affectedClaimPrefix + "pkg/x/x.go"},
		{Falsifier: falsifierMutant, Path: "pkg/c/c_test.go", BlobHash: "stale", reason: affectedClaimPrefix + "pkg/a/a.go"},
		{Falsifier: falsifierMutant, Path: "pkg/c/c_test.go", BlobHash: "a1", reason: samePackageClaim + "pkg/a/a.go"},
	}
	spans := map[string][]mutate.LineSpan{"pkg/a/a.go": {{Start: 2, End: 3}}}
	groups := mutationGroups(rows, cited, nil, spans)
	if len(groups) != 2 || groups[0] == nil || groups[4] == nil || groups[0] != groups[4] {
		t.Fatalf("groups = %v", groups)
	}
	if got := fmt.Sprint(groups[0].rows, groups[0].changed, groups[0].lines); got != "[0 4]pkg/a/a.go[{2 3}]" {
		t.Fatalf("group of pkg/a/a.go = %s", got)
	}
	if _, grouped := groups[2]; grouped {
		t.Fatal("a path the range gave no hunk joined a group")
	}
	if _, grouped := groups[3]; grouped {
		t.Fatal("a misplaced test blob joined a group")
	}
}
