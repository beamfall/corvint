package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cemdiscriminate"
	"github.com/Beamfall/corvint/internal/liveverify/mutate"
)

// The binary installs the mutation runner from cmd/corvint; these tests
// install the same one.
func init() { OpenHunkJudge = openTestHunkJudge }

func openTestHunkJudge(ctx context.Context, root, target string) (HunkJudge, error) {
	judge, err := cemdiscriminate.Open(ctx, root, target)
	if err != nil {
		return nil, err
	}
	return judge, nil
}

const calcFiller = "// filler one\n// filler two\n// filler three\n// filler four\n// filler five\n// filler six\n// filler seven\n// filler eight\n"

const calcBase = "package calc\n\n// Clamp bounds v to [lo, hi].\nfunc Clamp(v, lo, hi int) int {\n\treturn v\n}\n\n" +
	calcFiller + "\n// Sign reports -1, 0, or 1.\nfunc Sign(v int) int {\n\treturn 0\n}\n"

const calcTarget = "package calc\n\n// Clamp bounds v to [lo, hi].\nfunc Clamp(v, lo, hi int) int {\n" +
	"\tif v < lo {\n\t\treturn lo\n\t}\n\tif v > hi {\n\t\treturn hi\n\t}\n\treturn v\n}\n\n" +
	calcFiller + "\n// Sign reports -1, 0, or 1.\nfunc Sign(v int) int {\n\tif v < 0 {\n\t\treturn -1\n\t}\n\tif v > 0 {\n\t\treturn 1\n\t}\n\treturn 0\n}\n"

const calcStrongTest = "package calc\n\nimport \"testing\"\n\nfunc TestClamp(t *testing.T) {\n" +
	"\tfor _, c := range []struct{ v, lo, hi, want int }{{5, 0, 3, 3}, {-1, 0, 3, 0}, {2, 0, 3, 2}, {0, 0, 3, 0}, {3, 0, 3, 3}} {\n" +
	"\t\tif got := Clamp(c.v, c.lo, c.hi); got != c.want {\n\t\t\tt.Fatalf(\"Clamp(%d,%d,%d) = %d, want %d\", c.v, c.lo, c.hi, got, c.want)\n\t\t}\n\t}\n}\n"

const calcWeakTest = "package calc\n\nimport \"testing\"\n\nfunc TestClampWeak(t *testing.T) {\n\t_ = Clamp(2, 0, 3)\n}\n"

// makeCalcRepo builds a Go module whose target commit changes two hunks of
// pkg/calc/calc.go; the strong and weak test files exist at the base so they
// can be cited as test-claim evidence.
func makeCalcRepo(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "init", "-q", "-b", "main")
	writeFile(t, root, "go.mod", "module example.com/m\n\ngo 1.27.0\n")
	writeFile(t, root, "pkg/calc/calc.go", calcBase)
	writeFile(t, root, "pkg/calc/calc_test.go", calcStrongTest)
	writeFile(t, root, "pkg/calc/weak_test.go", calcWeakTest)
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "base")
	base := gitCmd(t, root, "rev-parse", "HEAD")
	writeFile(t, root, "pkg/calc/calc.go", calcTarget)
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "target")
	return root, base, gitCmd(t, root, "rev-parse", "HEAD")
}

func citeTest(t *testing.T, root, hunk, testPath string) {
	t.Helper()
	if _, err := openSession(t, root).Cite(ctx(), CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: hunk, EvidencePath: testPath, Lines: "1:3", Relation: "test-claim",
	}); err != nil {
		t.Fatal(err)
	}
}

func selectionOf(paths ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(paths, "\n") + "\n"))
	return hex.EncodeToString(digest[:])
}

// discriminateWallTime bounds each end-to-end discrimination run: a per-run
// limit of about 171 seconds against a cold build measured at 74.5 seconds
// under a load average near 60 on the 12-core reference host.
const discriminateWallTime = "20m"

// TestDiscriminateRecordsWitnessAndReportDowngrades runs the bounded mutation
// witness end to end (TCQ-V0-055..058): a strong cited test kills every
// mutant of the first hunk, the hunk limit leaves the second hunk explicitly
// not-run, and a weak cited test lets mutants survive, which the stable review
// projection shows as the survived state while status still accepts. The
// tcq/0 report downgrade is retired with the 0.x report (CEM-V1-217).
//
// The wall time is a hang bound, not a performance budget (V1-0547): each
// discrimination request starts from an empty build cache, so its first go test
// builds cold, and the per-run limit is the wall time over maxMutants+1, so
// five minutes left that cold build on a loaded host killed as a run without a
// verdict. A host whose enclosing sandbox refuses the nested one skips with the
// runner's own reason, but only after the witness identities, bounds and hunk
// limit that do not depend on the sandbox have been checked.
func TestDiscriminateRecordsWitnessAndReportDowngrades(t *testing.T) {
	t.Parallel()
	if _, err := mutate.HostSandbox(); err != nil {
		t.Skipf("no sandbox on this host: %v", err)
	}
	root, base, target := makeCalcRepo(t)
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	citeTest(t, root, "1", "pkg/calc/calc_test.go")
	citeTest(t, root, "2", "pkg/calc/calc_test.go")

	started := time.Now()
	result, err := openSession(t, root).Discriminate(ctx(), DiscriminateOptions{
		MapPath: wire.ExcludedCEMPath, Target: "HEAD", MaxHunks: "1", MaxMutants: "6", WallTime: discriminateWallTime, Output: witnessMap,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("strong run: %s", time.Since(started).Round(time.Millisecond))
	head := gitCmd(t, root, "rev-parse", "HEAD")
	selection := selectionOf("pkg/calc/calc_test.go")
	document := readMapAt(t, root, witnessMap)
	first, second := document.Hunks[0].Discriminates, document.Hunks[1].Discriminates
	if result["treeRevision"] != head || result["selectionSha256"] != selection || document.Spec != wire.StableSpec ||
		first == nil || first.TreeRevision != head || first.SelectionSha256 != selection ||
		first.Bounds != (wire.DiscriminationBounds{MaxHunks: 1, MaxMutants: 6, WallTimeSeconds: 1200}) {
		t.Fatalf("discriminate envelope = %v\nspec %s first witness %+v", result, document.Spec, first)
	}
	if second == nil || second.State != wire.DiscriminationNotRun || second.Mutants != 0 || second.Detail != "hunk limit 1 reached" {
		t.Fatalf("second witness %+v", second)
	}
	if first.State == wire.DiscriminationNotRun && strings.HasSuffix(first.Detail, ": "+mutate.ErrSandboxUnavailable.Error()) {
		t.Skipf("host refused the nested sandbox: %s", first.Detail)
	}
	if result["discriminates"] != 1 || result["notRun"] != 1 || result["survived"] != 0 {
		t.Fatalf("discriminate envelope = %v\nfirst witness %+v\nsecond witness %+v", result, first, second)
	}
	if first.State != wire.DiscriminationDiscriminates ||
		first.Killed < 1 || first.Survived != 0 || first.Killed+first.Survived > first.Mutants {
		t.Fatalf("first witness %+v", first)
	}
	if status := stableStatus(t, root, witnessMap, base, head, PolicyLimits{}); stableOutcome(status) != "ACCEPT" {
		t.Fatalf("status after discriminate: %v", status)
	}
	text := reportTextAt(t, root, base, head, witnessMap)
	if !strings.Contains(text, "discrimination `discriminates`") || !strings.Contains(text, "discrimination `not-run`") {
		t.Fatalf("report after a discriminating run:\n%s", text)
	}

	// A test that asserts nothing lets every mutant live: a visible downgrade.
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: "HEAD", Replace: true}); err != nil {
		t.Fatal(err)
	}
	citeTest(t, root, "1", "pkg/calc/weak_test.go")
	citeTest(t, root, "2", "pkg/calc/calc_test.go")
	started = time.Now()
	result, err = openSession(t, root).Discriminate(ctx(), DiscriminateOptions{
		MapPath: wire.ExcludedCEMPath, Target: "HEAD", MaxHunks: "1", MaxMutants: "6", WallTime: discriminateWallTime, Output: witnessMap,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("weak run: %s", time.Since(started).Round(time.Millisecond))
	first = readMapAt(t, root, witnessMap).Hunks[0].Discriminates
	if result["survived"] != 1 || result["notRun"] != 1 {
		t.Fatalf("weak envelope = %v\nfirst witness %+v", result, first)
	}
	if first == nil || first.State != wire.DiscriminationSurvived || first.Survived < 1 ||
		int64(len(first.Survivors)) != first.Survived || first.Survivors[0].Operator == "" ||
		!strings.Contains(first.Survivors[0].Description, "pkg/calc/calc.go:") {
		t.Fatalf("weak witness %+v", first)
	}
	if status := stableStatus(t, root, witnessMap, base, head, PolicyLimits{}); stableOutcome(status) != "ACCEPT" {
		t.Fatalf("status with survivors must still accept: %v", status)
	}
	text = reportTextAt(t, root, base, head, witnessMap)
	if !strings.Contains(text, "discrimination `survived`") {
		t.Fatalf("report with survivors:\n%s", text)
	}
}

// TestDiscriminateRefusesInvalidBoundsAndTarget pins the TCQ-V0-055 refusals:
// a non-positive bound, a sub-second wall time, a missing target, and a
// target whose canonical patch is not the map's; every refusal leaves the map
// untouched and runs no mutant.
func TestDiscriminateRefusesInvalidBoundsAndTarget(t *testing.T) {
	t.Parallel()
	root, base, target := makeCalcRepo(t)
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, wire.ExcludedCEMPath))
	if err != nil {
		t.Fatal(err)
	}
	refusals := map[string]struct {
		options DiscriminateOptions
		code    string
	}{
		"zero-hunks":      {DiscriminateOptions{MapPath: wire.ExcludedCEMPath, Target: "HEAD", MaxHunks: "0"}, cemcode.InvalidArguments},
		"negative-mutant": {DiscriminateOptions{MapPath: wire.ExcludedCEMPath, Target: "HEAD", MaxMutants: "-1"}, cemcode.InvalidArguments},
		"sub-second":      {DiscriminateOptions{MapPath: wire.ExcludedCEMPath, Target: "HEAD", WallTime: "500ms"}, cemcode.InvalidArguments},
		"no-target":       {DiscriminateOptions{MapPath: wire.ExcludedCEMPath}, cemcode.InvalidArguments},
		"base-as-target":  {DiscriminateOptions{MapPath: wire.ExcludedCEMPath, Target: base}, cemcode.PatchDigestMismatch},
	}
	for name, refusal := range refusals {
		_, err := openSession(t, root).Discriminate(ctx(), refusal.options)
		if err == nil || cemcode.CodeOf(err) != refusal.code {
			t.Errorf("%s: got %v, want %s", name, err, refusal.code)
		}
	}
	after, err := os.ReadFile(filepath.Join(root, wire.ExcludedCEMPath))
	if err != nil || string(after) != string(before) {
		t.Fatalf("a refusal changed the map: %v", err)
	}
}

// TestDiscriminateWithoutRunnerIsNotRun pins the uninstalled-hook seam: with
// no mutation runner installed every selected hunk carries the
// runner-unavailable not-run witness, and nothing panics.
func TestDiscriminateWithoutRunnerIsNotRun(t *testing.T) {
	installed := OpenHunkJudge
	OpenHunkJudge = nil
	defer func() { OpenHunkJudge = installed }()
	root, base, target := makeCalcRepo(t)
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	citeTest(t, root, "1", "pkg/calc/calc_test.go")
	gitCmd(t, root, "add", wire.ExcludedCEMPath)
	gitCmd(t, root, "commit", "-qm", "candidate")
	result, err := openSession(t, root).Discriminate(ctx(), DiscriminateOptions{MapPath: wire.ExcludedCEMPath, Target: "HEAD", Output: witnessMap})
	if err != nil || result["notRun"] != 2 || result["discriminates"] != 0 {
		t.Fatalf("uninstalled runner: %v %v", err, result)
	}
	first := readMapAt(t, root, witnessMap).Hunks[0].Discriminates
	if first == nil || first.State != wire.DiscriminationNotRun ||
		first.Detail != "mutation runner unavailable: no mutation runner is installed" {
		t.Fatalf("uninstalled runner witness %+v", first)
	}
}
