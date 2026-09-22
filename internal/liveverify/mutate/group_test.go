package mutate

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// groupFixture is the cross-package fixture with a second, assertion-free
// test file in the changed package: three claims over one changed file, two
// of them in one package.
func groupFixture(t *testing.T, git string) (string, string) {
	t.Helper()
	return newFixture(t, git, map[string]string{
		"go.mod":                     fixtureGoMod,
		"pkg/calc/calc.go":           fixtureCalc,
		"pkg/calc/calc_test.go":      fixtureCalcTest,
		"pkg/calc/calc_more_test.go": strings.Replace(fixtureEmptyTest, "TestNothing", "TestNothingMore", 1),
		"pkg/app/app_test.go":        fixtureCrossPackageTest,
	})
}

var errExit = errors.New("exit status 1")

var groupTests = []string{"pkg/calc/calc_test.go", "pkg/calc/calc_more_test.go", "pkg/app/app_test.go"}

// EAF-V0-002: a panic leaves the second claim unattributed; its fallback
// must reject the same mode-only baseline as the grouped JSON invocation.
func TestGroupFallbackRejectsModeOnlyBaseline(t *testing.T) {
	requireSandbox(t)
	git := gitExecutable(t)
	root, revision := newFixture(t, git, map[string]string{
		"go.mod": fixtureGoMod, "pkg/calc/calc.go": fixtureCalc,
		"pkg/calc/calc_more_test.go": "package calc\nimport \"testing\"\nfunc TestPanic(t *testing.T) { panic(\"synthetic\") }\n",
		"pkg/calc/calc_test.go":      fixtureModeOnlyTest,
	})
	exported, err := Open(context.Background(), Request{Root: root, Git: git, Revision: revision})
	if err != nil {
		t.Fatal(err)
	}
	defer exported.Close()
	runs := 0
	observeRun = func(string) { runs++ }
	defer func() { observeRun = nil }()
	reports, err := exported.JudgeGroup(context.Background(), "pkg/calc/calc.go", nil, groupTests[:2])
	if err != nil {
		t.Fatal(err)
	}
	for _, report := range reports {
		if report.Verdict != Unsupported || report.Mutants != 0 || report.Killed != 0 {
			t.Fatal(report)
		}
	}
	if runs != 2 {
		t.Fatalf("runs=%d, want group baseline and isolated fallback", runs)
	}
}

func TestEventSinkPreservesFailureClassification(t *testing.T) {
	for _, test := range []struct {
		name, stream string
		want         runOutcome
	}{
		{"red test", `{"Action":"fail","Test":"TestAdd"}`, runFailed},
		{"build failed", `{"Action":"build-fail"}`, runUnbuildable},
		{"setup failed", `{"Action":"output","Output":"[setup failed]"}`, runUnbuildable},
		{"sandbox never launched", "sandbox-exec: profile parse error", runBroken},
		{"silent kill", "", runBroken},
	} {
		t.Run(test.name, func(t *testing.T) {
			sink := newEventSink(&boundedBuffer{limit: 256})
			_, _ = sink.Write([]byte(test.stream + "\n"))
			if got := sink.classify(errExit); got != test.want {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}

// TestJudgeGroupGivesEachClaimItsSingleJudgeVerdict: grouping changes how
// the runs are batched, never what a claim is told.
func TestJudgeGroupGivesEachClaimItsSingleJudgeVerdict(t *testing.T) {
	requireSandbox(t)
	git := gitExecutable(t)
	root, revision := groupFixture(t, git)
	exported, err := Open(context.Background(), Request{Root: root, Git: git, Revision: revision})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer exported.Close()
	alone := map[string]Report{}
	for _, test := range groupTests {
		report, err := exported.Judge(context.Background(), Request{ChangedPath: "pkg/calc/calc.go", TestPath: test})
		if err != nil {
			t.Fatalf("Judge %s: %v", test, err)
		}
		alone[test] = report
	}
	if alone[groupTests[0]].Verdict != Killed || alone[groupTests[1]].Verdict != Survived || alone[groupTests[2]].Verdict != Killed {
		t.Fatalf("single verdicts = %s %s %s", alone[groupTests[0]].Verdict, alone[groupTests[1]].Verdict, alone[groupTests[2]].Verdict)
	}
	grouped, err := exported.JudgeGroup(context.Background(), "pkg/calc/calc.go", nil, groupTests)
	if err != nil {
		t.Fatalf("JudgeGroup: %v", err)
	}
	for _, test := range groupTests {
		if grouped[test].Verdict != alone[test].Verdict || grouped[test].Detail != alone[test].Detail {
			t.Errorf("%s: group %s (%s), alone %s (%s)", test, grouped[test].Verdict, grouped[test].Detail, alone[test].Verdict, alone[test].Detail)
		}
	}
}

// TestJudgeGroupRunsOnePackagePerMutant counts go test invocations: one
// baseline per package, then one run per mutant per package while the package
// has an undecided claim, so the package a first mutant decides stops early.
func TestJudgeGroupRunsOnePackagePerMutant(t *testing.T) {
	requireSandbox(t)
	git := gitExecutable(t)
	root, revision := groupFixture(t, git)
	exported, err := Open(context.Background(), Request{Root: root, Git: git, Revision: revision})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer exported.Close()
	runs := map[string]int{}
	observeRun = func(packageDir string) { runs[packageDir]++ }
	defer func() { observeRun = nil }()
	grouped, err := exported.JudgeGroup(context.Background(), "pkg/calc/calc.go", nil, groupTests)
	if err != nil {
		t.Fatalf("JudgeGroup: %v", err)
	}
	survivor, killer := grouped[groupTests[1]], grouped[groupTests[2]]
	if survivor.Verdict != Survived || survivor.Survived+survivor.Uncompilable != survivor.Mutants {
		t.Fatalf("survivor saw %d of %d mutants: %+v", survivor.Survived+survivor.Uncompilable, survivor.Mutants, survivor)
	}
	if killer.Verdict != Killed || killer.Skipped != killer.Mutants-1 {
		t.Fatalf("cross-package claim did not stop at its first mutant: %+v", killer)
	}
	if runs["./pkg/calc"] != 1+survivor.Mutants || runs["./pkg/app"] != 2 || len(runs) != 2 {
		t.Fatalf("runs = %v, want ./pkg/calc %d and ./pkg/app 2", runs, 1+survivor.Mutants)
	}
}

// TestEventSinkLeavesAnUnfinishedFunctionUnattributed streams a -json run
// where one test passed, one panicked, and one never ran, in chunks that
// split lines and outgrow the raw buffer: the outcomes survive the cut, the
// run classifies as failed, and the claim holding the unrun function cannot
// be credited from the group.
func TestEventSinkLeavesAnUnfinishedFunctionUnattributed(t *testing.T) {
	stream := `{"Action":"run","Package":"p","Test":"TestPass"}
{"Action":"output","Package":"p","Test":"TestPass","Output":"` + strings.Repeat("x", 200) + `\n"}
{"Action":"pass","Package":"p","Test":"TestPass","Elapsed":0}
{"Action":"run","Package":"p","Test":"TestPanic"}
{"Action":"output","Package":"p","Test":"TestPanic","Output":"panic: boom\n"}
{"Action":"fail","Package":"p","Test":"TestPanic","Elapsed":0}
{"Action":"run","Package":"p","Test":"TestSub/case"}
{"Action":"fail","Package":"p","Test":"TestSub/case","Elapsed":0}
not an event
{"Action":"fail","Package":"p","Elapsed":0.2}
`
	sink := newEventSink(&boundedBuffer{limit: 64})
	for offset := 0; offset < len(stream); offset += 7 {
		end := min(offset+7, len(stream))
		if _, err := sink.Write([]byte(stream[offset:end])); err != nil {
			t.Fatal(err)
		}
	}
	outcomes := sink.outcomes
	if len(outcomes) != 2 || outcomes["TestPass"] != runPassed || outcomes["TestPanic"] != runFailed {
		t.Fatalf("outcomes = %v", outcomes)
	}
	if got := sink.classify(errExit); got != runFailed {
		t.Fatalf("classify = %d, want %d (raw buffer held %d bytes)", got, runFailed, len(sink.raw.String()))
	}
	passing := &claim{names: []string{"TestPass"}}
	panicking := &claim{names: []string{"TestPass", "TestPanic"}}
	unrun := &claim{names: []string{"TestAfter"}}
	if passing.outcome(outcomes) != runPassed || panicking.outcome(outcomes) != runFailed || unrun.outcome(outcomes) != runUnattributed {
		t.Fatalf("folded = %d %d %d", passing.outcome(outcomes), panicking.outcome(outcomes), unrun.outcome(outcomes))
	}
}
