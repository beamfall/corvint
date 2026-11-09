package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

const revertFiller = "// filler one\n// filler two\n// filler three\n// filler four\n// filler five\n// filler six\n// filler seven\n// filler eight\n"

// The base keeps Clamp and Sign unimplemented and lacks Abs; the target
// implements both and appends Abs, giving three hunks of pkg/calc/calc.go in
// line order: Clamp, Sign, Abs.
const revertBase = "package calc\n\n// Clamp bounds v to [lo, hi].\nfunc Clamp(v, lo, hi int) int {\n\treturn v\n}\n\n" +
	revertFiller + "\n// Sign reports -1, 0, or 1.\nfunc Sign(v int) int {\n\treturn 0\n}\n\n" + revertFiller

const revertTarget = "package calc\n\n// Clamp bounds v to [lo, hi].\nfunc Clamp(v, lo, hi int) int {\n" +
	"\tif v < lo {\n\t\treturn lo\n\t}\n\tif v > hi {\n\t\treturn hi\n\t}\n\treturn v\n}\n\n" +
	revertFiller + "\n// Sign reports -1, 0, or 1.\nfunc Sign(v int) int {\n\tif v < 0 {\n\t\treturn -1\n\t}\n\tif v > 0 {\n\t\treturn 1\n\t}\n\treturn 0\n}\n\n" +
	revertFiller + "\n// Abs returns the magnitude of v.\nfunc Abs(v int) int {\n\tif v < 0 {\n\t\treturn -v\n\t}\n\treturn v\n}\n"

// Each test file declares its one test on line 5.
const (
	revertAbsTest    = "package calc\n\nimport \"testing\"\n\nfunc TestAbs(t *testing.T) {\n\tif Abs(-2) != 2 {\n\t\tt.Fatal(\"Abs(-2) != 2\")\n\t}\n}\n"
	revertBrokenTest = "package calc\n\nimport \"testing\"\n\nfunc TestSignBroken(t *testing.T) {\n\t_ = Sign(1)\n\tt.Fatal(\"fails with or without any revert\")\n}\n"
)

// makeRevertRepo commits the base with every test file, so each can be cited
// as base evidence, and the target that changes only pkg/calc/calc.go.
func makeRevertRepo(t *testing.T) (root, base, target string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "init", "-q", "-b", "main")
	writeFile(t, root, "go.mod", "module example.com/m\n\ngo 1.27.0\n")
	writeFile(t, root, "pkg/calc/calc.go", revertBase)
	writeFile(t, root, "pkg/calc/calc_test.go", calcStrongTest)
	writeFile(t, root, "pkg/calc/weak_test.go", calcWeakTest)
	writeFile(t, root, "pkg/calc/abs_test.go", revertAbsTest)
	writeFile(t, root, "pkg/calc/broken_test.go", revertBrokenTest)
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "base")
	base = gitCmd(t, root, "rev-parse", "HEAD")
	writeFile(t, root, "pkg/calc/calc.go", revertTarget)
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "target")
	return root, base, gitCmd(t, root, "rev-parse", "HEAD")
}

// prepareRevert prepares base..target and cites hunk i with cites[i] as
// {path, lines, relation}.
func prepareRevert(t *testing.T, root, base, target string, cites [3][3]string) []wire.Hunk {
	t.Helper()
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	hunks := readMap(t, root).Hunks
	if len(hunks) != 3 {
		t.Fatalf("fixture has %d hunks, want 3", len(hunks))
	}
	for i, cite := range cites {
		if _, err := openSession(t, root).Cite(ctx(), CiteOptions{
			MapPath: wire.ExcludedCEMPath, Hunk: hunks[i].ID, EvidencePath: cite[0], Lines: cite[1], Relation: cite[2],
		}); err != nil {
			t.Fatal(err)
		}
	}
	return readMap(t, root).Hunks
}

func verifyStable(t *testing.T, root, base, target string, revert bool) map[string]any {
	t.Helper()
	result, err := openSession(t, root).Read(ctx(), "verify", ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: target,
		ArtifactRoot: root, ArtifactsGiven: true, HunkRevert: revert,
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func revertRows(t *testing.T, result map[string]any) (*verify.HunkRevert, map[string]verify.HunkRevertRow) {
	t.Helper()
	report, _ := result["hunkRevert"].(*verify.HunkRevert)
	if report == nil {
		encoded, _ := json.MarshalIndent(result, "", "  ")
		t.Fatalf("verify --hunk-revert reported no falsifier block: %s", encoded)
	}
	rows := map[string]verify.HunkRevertRow{}
	for _, row := range report.Hunks {
		rows[row.HunkID] = row
	}
	return report, rows
}

// treeDigest hashes every path, mode and byte under root, .git included, so
// any write to the caller's worktree, index, refs or object store shows.
func treeDigest(t *testing.T, root string) string {
	t.Helper()
	digest := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(root, path)
		digest.Write([]byte(relative + "\x00" + info.Mode().String() + "\x00"))
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			digest.Write(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(digest.Sum(nil))
}

// TestCEMV1088HeldOnlyFromConsumerReexecution is the AC 1 fixture set: with
// --hunk-revert the consumer itself reverts each candidate hunk in a
// disposable copy and runs only its named tests. The Clamp hunk's test passes
// at the target and fails reverted, so it is held; reverting Abs breaks the
// build and the Sign hunk's test fails without the revert, so both are not-run
// (CEM-V1-089). The verifier result is byte-identical with and without the
// flag, held stays 0 inside it, and the caller's repository is untouched.
func TestCEMV1088HeldOnlyFromConsumerReexecution(t *testing.T) {
	t.Parallel()
	root, base, target := makeRevertRepo(t)
	hunks := prepareRevert(t, root, base, target, [3][3]string{
		{"pkg/calc/calc_test.go", "5:5", "test-claim"},
		{"pkg/calc/broken_test.go", "5:5", "test-claim"},
		{"pkg/calc/abs_test.go", "5:5", "test-claim"},
	})
	plain := verifyStable(t, root, base, target, false)
	if _, present := plain["hunkRevert"]; present {
		t.Fatalf("verify without --hunk-revert added a falsifier block: %v", plain)
	}
	before := treeDigest(t, root)
	result := verifyStable(t, root, base, target, true)
	if after := treeDigest(t, root); after != before {
		t.Fatal("verify --hunk-revert changed the caller's worktree, index, refs or objects")
	}
	plainBytes, _ := json.Marshal(plain["stableVerification"])
	revertBytes, _ := json.Marshal(result["stableVerification"])
	if string(plainBytes) != string(revertBytes) || stableOutcome(result) != "ACCEPT" {
		t.Fatalf("the flag changed the verifier result:\n%s\n%s", plainBytes, revertBytes)
	}
	if verification := result["stableVerification"].(StableVerification); verification.Result.Citation.HunksByClass.Held != 0 {
		t.Fatalf("the in-result held count moved: %+v", verification.Result.Citation.HunksByClass)
	}
	report, rows := revertRows(t, result)
	want := map[string]verify.HunkRevertRow{
		hunks[0].ID: {HunkID: hunks[0].ID, Tests: []string{"pkg/calc::TestClamp"}, Result: verify.RevertHeld, Failing: []string{"TestClamp"}},
		hunks[1].ID: {HunkID: hunks[1].ID, Tests: []string{"pkg/calc::TestSignBroken"}, Result: verify.RevertNotRun, Reason: verify.RevertFailsWithoutRevert, Failing: []string{}},
		hunks[2].ID: {HunkID: hunks[2].ID, Tests: []string{"pkg/calc::TestAbs"}, Result: verify.RevertNotRun, Reason: verify.RevertBreaksBuild, Failing: []string{}},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %+v\nwant %+v", rows, want)
	}
	if report.Profile != verify.HunkRevertProfile || report.Runner != "go-test" || report.Held != 1 || report.NotRun != 2 ||
		report.HunksByClass.Held != 1 || report.HunksByClass.Held+report.HunksByClass.Anchored+report.HunksByClass.Cited != 3 {
		t.Fatalf("report = %+v", report)
	}
	t.Logf("falsifier elapsed %.3fs", report.ElapsedSeconds)
}

// TestCEMV1088ForgedWitnessNeverCounts pins that a producer-written witness
// never counts: a map that carries a forged discriminates witness on a hunk
// guarded only by a test that asserts nothing is not held, because the revert
// the consumer runs survives; a test-claim span that declares no test is
// not-run; a hunk with no test-claim is not a candidate.
func TestCEMV1088ForgedWitnessNeverCounts(t *testing.T) {
	t.Parallel()
	root, base, target := makeRevertRepo(t)
	hunks := prepareRevert(t, root, base, target, [3][3]string{
		{"pkg/calc/weak_test.go", "5:5", "test-claim"},
		{"pkg/calc/calc_test.go", "1:3", "test-claim"},
		{"pkg/calc/abs_test.go", "5:5", "specification"},
	})
	sidecar := filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath))
	raw, err := os.ReadFile(sidecar)
	if err != nil {
		t.Fatal(err)
	}
	document, err := wire.ParseStable(raw)
	if err != nil {
		t.Fatal(err)
	}
	document.Change.Hunks[0].Discriminates = &wire.DiscriminationWitness{
		TreeRevision: target, SelectionSha256: selectionOf("pkg/calc/weak_test.go"),
		Mutants: 6, Killed: 6, Survivors: []wire.SurvivingMutant{},
		Bounds: wire.DiscriminationBounds{MaxHunks: 1, MaxMutants: 6, WallTimeSeconds: 300},
		State:  wire.DiscriminationDiscriminates,
	}
	forged, err := wire.EncodeStable(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sidecar, forged, 0o644); err != nil {
		t.Fatal(err)
	}
	result := verifyStable(t, root, base, target, true)
	if stableOutcome(result) != "ACCEPT" {
		encoded, _ := json.MarshalIndent(result, "", "  ")
		t.Fatalf("forged-witness map did not accept: %s", encoded)
	}
	report, rows := revertRows(t, result)
	want := map[string]verify.HunkRevertRow{
		hunks[0].ID: {HunkID: hunks[0].ID, Tests: []string{"pkg/calc::TestClampWeak"}, Result: verify.RevertSurvived, Failing: []string{}},
		hunks[1].ID: {HunkID: hunks[1].ID, Tests: []string{}, Result: verify.RevertNotRun, Reason: verify.RevertNoNamedTest, Failing: []string{}},
	}
	if !reflect.DeepEqual(rows, want) || report.Held != 0 || report.HunksByClass.Held != 0 {
		t.Fatalf("rows = %+v\nwant %+v\nreport %+v", rows, want, report)
	}
}

// TestCEMV1088FlagIsVerifyOnly pins the opt-in boundary: status and report
// refuse --hunk-revert rather than silently ignoring it.
func TestCEMV1088FlagIsVerifyOnly(t *testing.T) {
	t.Parallel()
	root, base, target := makeRepo(t)
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"status", "report"} {
		_, err := openSession(t, root).Read(ctx(), action, ReadOptions{
			MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: target, ArtifactRoot: root, ArtifactsGiven: true, HunkRevert: true,
		})
		if err == nil || !strings.Contains(err.Error(), "--hunk-revert") {
			t.Fatalf("%s --hunk-revert: %v", action, err)
		}
	}
	// A map with no test-claim has no candidate: an empty block, no export.
	result := verifyStable(t, root, base, target, true)
	if report, _ := revertRows(t, result); len(report.Hunks) != 0 || report.Held != 0 {
		t.Fatalf("report = %+v", report)
	}
}
