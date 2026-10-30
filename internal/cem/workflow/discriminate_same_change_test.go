package workflow

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cemdiscriminate"
	"github.com/Beamfall/corvint/internal/liveverify/mutate"
)

// TestTCQV0067CandidateRuleAgreesWithTheRunnerDeclarations keeps the two
// sides of the runner contract together: a path is a candidate exactly when
// a compiled-in runner judges it, and its tests carry that runner's suffix.
func TestTCQV0067CandidateRuleAgreesWithTheRunnerDeclarations(t *testing.T) {
	t.Parallel()
	paths := []string{"pkg/calc.go", "pkg/calc_test.go", "lib/calc.rb", "spec/calc_spec.rb", "app/calc.py", "web/calc.ts", "README.md", "Makefile"}
	for _, path := range paths {
		runner, judged := cemdiscriminate.RunnerFor(path)
		suffix := runnerTestSuffix(path)
		if judged != (suffix != "") || (judged && runner.TestSuffix != suffix) {
			t.Errorf("%s: candidate suffix %q, runner %+v judged=%v", path, suffix, runner, judged)
		}
	}
}

func sameChangeMap() *wire.Map {
	return &wire.Map{
		Evidence: []wire.Evidence{{ID: "e1", Path: "pkg/calc/calc_test.go"}},
		Hunks: []wire.Hunk{
			{ID: "h1", Path: "pkg/calc/calc.go", NewRange: wire.Range{Start: 4, Count: 3}, Basis: []wire.Basis{{EvidenceID: "e1", Relation: "test-claim"}}},
			{ID: "h2", Path: "pkg/calc/calc.go", NewRange: wire.Range{Start: 20, Count: 2}},
			{ID: "h3", Path: "pkg/calc/new_test.go", NewRange: wire.Range{Start: 1, Count: 9}},
			{ID: "h4", Path: "pkg/calc/gone_test.go", NewRange: wire.Range{Start: 0, Count: 0}},
			{ID: "h5", Path: "spec/calc_spec.rb", NewRange: wire.Range{Start: 1, Count: 4}},
		},
	}
}

// TestTCQV0071SameChangeTestsJoinTheirHunkOnly checks that a named
// same-change test joins only its hunk's selection, merges with the cited
// tests, and is listed apart so the witness detail can name it.
func TestTCQV0071SameChangeTestsJoinTheirHunkOnly(t *testing.T) {
	t.Parallel()
	document := sameChangeMap()
	sameChange, err := parseSameChangeTests("h2=pkg/calc/new_test.go,h1=pkg/calc/new_test.go", document)
	if err != nil {
		t.Fatal(err)
	}
	candidates := discriminationCandidates(document, sameChange)
	want := []hunkCandidate{
		{index: 0, tests: []string{"pkg/calc/calc_test.go", "pkg/calc/new_test.go"}, sameChange: []string{"pkg/calc/new_test.go"}},
		{index: 1, tests: []string{"pkg/calc/new_test.go"}, sameChange: []string{"pkg/calc/new_test.go"}},
	}
	if !reflect.DeepEqual(candidates, want) {
		t.Fatalf("candidates = %+v, want %+v", candidates, want)
	}
	if got := selectionDigest(candidates); got != selectionOf("pkg/calc/calc_test.go", "pkg/calc/new_test.go") {
		t.Fatalf("selection digest %s does not cover the same-change test", got)
	}
	if got := sameChangeDetail([]string{"pkg/calc/new_test.go"}, "no mutant compiled"); got != "same-change test (execution only): pkg/calc/new_test.go; no mutant compiled" {
		t.Fatalf("detail = %q", got)
	}
	if got := sameChangeDetail(nil, ""); got != "" {
		t.Fatalf("detail without a same-change test = %q", got)
	}
}

// TestTCQV0071SameChangeTestRefusals refuses every name the contract does not
// admit before any run: an unknown hunk, a hunk no runner judges, a test of
// another language, and a test the change does not add lines to.
func TestTCQV0071SameChangeTestRefusals(t *testing.T) {
	t.Parallel()
	refusals := map[string]string{
		"missing separator": "h1",
		"unknown hunk":      "h9=pkg/calc/new_test.go",
		"empty hunk":        "=pkg/calc/new_test.go",
		"test hunk":         "h3=pkg/calc/new_test.go",
		"other language":    "h1=spec/calc_spec.rb",
		"not a test":        "h1=pkg/calc/calc.go",
		"unchanged test":    "h1=pkg/calc/calc_test.go",
		"deleted test":      "h1=pkg/calc/gone_test.go",
		"escaping path":     "h1=../calc/new_test.go",
		"empty entry":       "h1=pkg/calc/new_test.go,",
		"too many":          strings.Repeat("h1=pkg/calc/new_test.go,", maxSameChangeTests) + "h1=pkg/calc/new_test.go",
	}
	for name, text := range refusals {
		if _, err := parseSameChangeTests(text, sameChangeMap()); err == nil || cemcode.CodeOf(err) != cemcode.InvalidArguments {
			t.Errorf("%s: %q error = %v, want invalid-arguments", name, text, err)
		}
	}
}

// TestTCQV0071SameChangeTestWitnessesByExecution runs a test the change
// itself adds against its hunk: it is no evidence and cannot be cited, yet
// the run passes it at the target and a mutant of the hunk fails it, so the
// hunk records a discriminates witness naming the test. A named hunk the
// limit leaves unjudged stays not-run.
func TestTCQV0071SameChangeTestWitnessesByExecution(t *testing.T) {
	t.Parallel()
	if _, err := mutate.HostSandbox(); err != nil {
		t.Skipf("no sandbox on this host: %v", err)
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "init", "-q", "-b", "main")
	writeFile(t, root, "go.mod", "module example.com/m\n\ngo 1.27.0\n")
	writeFile(t, root, "pkg/calc/calc.go", calcBase)
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "base")
	base := gitCmd(t, root, "rev-parse", "HEAD")
	writeFile(t, root, "pkg/calc/calc.go", calcTarget)
	writeFile(t, root, "pkg/calc/calc_test.go", calcStrongTest)
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "target")
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: "HEAD"}); err != nil {
		t.Fatal(err)
	}
	prepared := readMapAt(t, root, wire.ExcludedCEMPath)
	if len(prepared.Hunks) != 3 || prepared.Hunks[2].Path != "pkg/calc/calc_test.go" {
		t.Fatalf("prepared hunks %+v", prepared.Hunks)
	}
	first, second := prepared.Hunks[0].ID, prepared.Hunks[1].ID
	started := time.Now()
	result, err := openSession(t, root).Discriminate(ctx(), DiscriminateOptions{
		MapPath: wire.ExcludedCEMPath, Target: "HEAD", MaxHunks: "1", MaxMutants: "6", WallTime: discriminateWallTime,
		SameChangeTests: first + "=pkg/calc/calc_test.go," + second + "=pkg/calc/calc_test.go", Output: witnessMap,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("same-change run: %s", time.Since(started).Round(time.Millisecond))
	document := readMapAt(t, root, witnessMap)
	judged, limited := document.Hunks[0].Discriminates, document.Hunks[1].Discriminates
	if result["selectionSha256"] != selectionOf("pkg/calc/calc_test.go") || judged == nil || limited == nil {
		t.Fatalf("envelope %v first %+v second %+v", result, judged, limited)
	}
	if limited.State != wire.DiscriminationNotRun || limited.Detail != "hunk limit 1 reached" {
		t.Fatalf("limited witness %+v", limited)
	}
	if judged.State == wire.DiscriminationNotRun && hostRefusedNestedSandbox(judged.Detail) {
		t.Skipf("host refused the nested sandbox: %s", judged.Detail)
	}
	if judged.State != wire.DiscriminationDiscriminates || judged.Killed < 1 ||
		!strings.HasPrefix(judged.Detail, "same-change test (execution only): pkg/calc/calc_test.go") {
		t.Fatalf("same-change witness %+v", judged)
	}
	if document.Hunks[0].Disposition != prepared.Hunks[0].Disposition || len(document.Hunks[0].Basis) != len(prepared.Hunks[0].Basis) {
		t.Fatalf("the witness changed the hunk's disposition or basis: %+v", document.Hunks[0])
	}
}

// TestTCQV0093SameChangeTestAcceptsTheCiteSelector resolves the hunk side of
// --same-change-test exactly as cite --hunk does: a canonical one-based
// ordinal selects the same hunk as its full ID, and a non-canonical or
// out-of-range ordinal is refused before any run.
func TestTCQV0093SameChangeTestAcceptsTheCiteSelector(t *testing.T) {
	t.Parallel()
	byOrdinal, err := parseSameChangeTests("2=pkg/calc/new_test.go,1=pkg/calc/new_test.go", sameChangeMap())
	if err != nil {
		t.Fatal(err)
	}
	byID, err := parseSameChangeTests("h2=pkg/calc/new_test.go,h1=pkg/calc/new_test.go", sameChangeMap())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(byOrdinal, byID) || len(byOrdinal) != 2 {
		t.Fatalf("ordinal selection %v differs from ID selection %v", byOrdinal, byID)
	}
	for _, text := range []string{"0=pkg/calc/new_test.go", "02=pkg/calc/new_test.go", "+1=pkg/calc/new_test.go", "6=pkg/calc/new_test.go", "3=pkg/calc/new_test.go"} {
		if _, err := parseSameChangeTests(text, sameChangeMap()); err == nil || cemcode.CodeOf(err) != cemcode.InvalidArguments {
			t.Errorf("%q error = %v, want invalid-arguments", text, err)
		}
	}
}
