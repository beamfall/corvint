package workflow

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

func verifyGuards(t *testing.T, root, base, target string) (map[string]any, *verify.GuardingTests, map[string]verify.GuardRow) {
	t.Helper()
	result, err := openSession(t, root).Read(ctx(), "verify", ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: target,
		ArtifactRoot: root, ArtifactsGiven: true, GuardingTests: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	report, _ := result["guardingTests"].(*verify.GuardingTests)
	if report == nil {
		encoded, _ := json.MarshalIndent(result, "", "  ")
		t.Fatalf("verify --guarding-tests reported no search block: %s", encoded)
	}
	rows := map[string]verify.GuardRow{}
	for _, row := range report.Hunks {
		rows[row.HunkID] = row
	}
	return result, report, rows
}

// TestTCQV0062GuardingTestsNameObservedGuards is the producer step end to end
// on the hunk-revert fixture, prepared with no citation at all. The search
// names the base tests that mention each changed declaration and keeps only
// the one observed to pass at the target and fail with the hunk reverted:
// TestClamp guards the Clamp hunk and TestClampWeak, which asserts nothing,
// does not. Reverting Abs breaks the build and the Sign hunk's only test fails
// without the revert, so both are not-run and suggest nothing. The plan row it
// prints is admitted by cem cite, after which the consumer's own falsifier
// holds the hunk and the search no longer selects it. The verifier result is
// byte-identical with and without the flag and the caller's repository is
// untouched.
func TestTCQV0062GuardingTestsNameObservedGuards(t *testing.T) {
	t.Parallel()
	root, base, target := makeRevertRepo(t)
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	hunks := readMap(t, root).Hunks
	if len(hunks) != 3 {
		t.Fatalf("fixture has %d hunks, want 3", len(hunks))
	}
	plain := verifyStable(t, root, base, target, false)
	if _, present := plain["guardingTests"]; present {
		t.Fatalf("verify without --guarding-tests added a search block: %v", plain)
	}
	before := treeDigest(t, root)
	result, report, rows := verifyGuards(t, root, base, target)
	if after := treeDigest(t, root); after != before {
		t.Fatal("verify --guarding-tests changed the caller's worktree, index, refs or objects")
	}
	plainBytes, _ := json.Marshal(plain["stableVerification"])
	guardBytes, _ := json.Marshal(result["stableVerification"])
	if string(plainBytes) != string(guardBytes) || stableOutcome(result) != "ACCEPT" {
		t.Fatalf("the flag changed the verifier result:\n%s\n%s", plainBytes, guardBytes)
	}
	citation := &verify.GuardCitation{Path: "pkg/calc/calc_test.go", Lines: "5:5", Test: "TestClamp"}
	want := map[string]verify.GuardRow{
		hunks[0].ID: {HunkID: hunks[0].ID, Path: "pkg/calc/calc.go", Candidates: []string{"pkg/calc::TestClamp", "pkg/calc::TestClampWeak"},
			Result: verify.GuardGuarded, Guards: []string{"TestClamp"}, Citation: citation},
		hunks[1].ID: {HunkID: hunks[1].ID, Path: "pkg/calc/calc.go", Candidates: []string{"pkg/calc::TestSignBroken"},
			Result: verify.GuardNotRun, Reason: verify.RevertFailsWithoutRevert, Guards: []string{}},
		hunks[2].ID: {HunkID: hunks[2].ID, Path: "pkg/calc/calc.go", Candidates: []string{"pkg/calc::TestAbs"},
			Result: verify.GuardNotRun, Reason: verify.RevertBreaksBuild, Guards: []string{}},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %+v\nwant %+v", rows, want)
	}
	plan := []string{hunks[0].ID + "\tpkg/calc/calc_test.go\t5:5\ttest-claim"}
	if report.Profile != verify.GuardingTestsProfile || report.Runner != "go-test" || report.Guarded != 1 ||
		report.Unguarded != 0 || report.NotRun != 2 || !reflect.DeepEqual(report.Plan, plan) {
		t.Fatalf("report = %+v", report)
	}
	t.Logf("guarding-test search elapsed %.3fs", report.ElapsedSeconds)

	// The printed row is a citation-plan row cem cite admits.
	fields := strings.Split(report.Plan[0], "\t")
	if _, err := openSession(t, root).Cite(ctx(), CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: fields[0], EvidencePath: fields[1], Lines: fields[2], Relation: fields[3],
	}); err != nil {
		t.Fatalf("cem cite refused the plan row %q: %v", report.Plan[0], err)
	}
	_, revertRowsByID := revertRows(t, verifyStable(t, root, base, target, true))
	if row := revertRowsByID[hunks[0].ID]; row.Result != verify.RevertHeld || !reflect.DeepEqual(row.Failing, []string{"TestClamp"}) {
		t.Fatalf("the consumer did not hold the cited hunk: %+v", row)
	}
	if _, _, again := verifyGuards(t, root, base, target); len(again) != 2 {
		t.Fatalf("a hunk citing a test-claim is still searched: %+v", again)
	} else if _, present := again[hunks[0].ID]; present {
		t.Fatalf("the cited hunk is still searched: %+v", again)
	}
}

// TestTCQV0062FlagIsVerifyOnly pins the opt-in boundary: status and report
// refuse --guarding-tests, and verify refuses it beside --hunk-revert, so one
// run never both suggests a test and credits it.
func TestTCQV0062FlagIsVerifyOnly(t *testing.T) {
	t.Parallel()
	root, base, target := makeRepo(t)
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"status", "report", "verify"} {
		_, err := openSession(t, root).Read(ctx(), action, ReadOptions{
			MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: target, ArtifactRoot: root, ArtifactsGiven: true,
			GuardingTests: true, HunkRevert: action == "verify",
		})
		if err == nil || !strings.Contains(err.Error(), "--guarding-tests") {
			t.Fatalf("%s --guarding-tests: %v", action, err)
		}
	}
	// A map with no Go source hunk has nothing to search: an empty block.
	if _, report, _ := verifyGuards(t, root, base, target); len(report.Hunks) != 0 || len(report.Plan) != 0 {
		t.Fatalf("report = %+v", report)
	}
}
