package cemdiscriminate

import (
	"errors"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
	"github.com/Beamfall/corvint/internal/liveverify/mutate"
	"github.com/Beamfall/corvint/internal/liveverify/rbmutate"
)

// The runner-contract conformance tests feed fixture runner reports through
// each arm's adapter and the one fold, so they need neither go test runs nor
// Ruby on the host (TCQ-V0-067..070).

func TestTCQV0067RunnersDeclareSourceAndTestRules(t *testing.T) {
	cases := map[string]string{
		"pkg/calc.go": "go", "lib/calc.rb": "ruby",
		"pkg/calc_test.go": "", "spec/calc_spec.rb": "", "app/calc.py": "", "README.md": "",
	}
	for path, want := range cases {
		runner, ok := RunnerFor(path)
		if got := runner.Language; got != want || ok != (want != "") {
			t.Errorf("RunnerFor(%q) = %q, %v; want %q", path, got, ok, want)
		}
	}
	seen := map[string]bool{}
	for _, runner := range Runners() {
		if runner.Language == "" || runner.SourceSuffix == "" || !strings.HasSuffix(runner.TestSuffix, runner.SourceSuffix) || seen[runner.Language] {
			t.Errorf("runner declaration %+v is incomplete or repeated", runner)
		}
		seen[runner.Language] = true
	}
	if len(Runners()) != 2 {
		t.Fatalf("Runners() = %+v, want exactly the Go and Ruby arms", Runners())
	}
}

var goRunner = Runner{Language: "go", SourceSuffix: ".go", TestSuffix: "_test.go"}
var rubyRunner = Runner{Language: "ruby", SourceSuffix: ".rb", TestSuffix: "_spec.rb"}

func goReport(outcomes ...mutate.Outcome) mutate.Report {
	report := mutate.Report{Mutants: len(outcomes)}
	for index, outcome := range outcomes {
		mutant := mutate.Survivor{Operator: "negate-condition", Line: 10 + index, Start: 100 + index, End: 101 + index}
		report.Outcomes = append(report.Outcomes, mutate.MutantOutcome{Mutant: mutant, Outcome: outcome})
		switch outcome {
		case mutate.OutcomeKilled:
			report.Killed++
		case mutate.OutcomeSurvived:
			report.Survived++
			report.Survivors = append(report.Survivors, mutant)
		default:
			report.Uncompilable++
		}
	}
	switch {
	case report.Killed > 0:
		report.Verdict = mutate.Killed
	case report.Survived > 0:
		report.Verdict = mutate.Survived
	default:
		report.Verdict = mutate.NoMutants
	}
	return report
}

func rubyReport(outcomes ...rbmutate.Outcome) rbmutate.Report {
	report := rbmutate.Report{Mutants: len(outcomes)}
	for index, outcome := range outcomes {
		report.Results = append(report.Results, rbmutate.Result{Operator: "negate-condition", Line: 10 + index, Start: 100 + index, End: 101 + index, Outcome: outcome})
		switch outcome {
		case rbmutate.OutcomeKilled:
			report.Killed++
		case rbmutate.OutcomeSurvived:
			report.Survived++
		}
	}
	switch {
	case report.Killed > 0:
		report.Verdict = mutate.Killed
	case report.Survived > 0:
		report.Verdict = mutate.Survived
	default:
		report.Verdict = mutate.NoMutants
	}
	return report
}

var hunkSpan = mutate.LineSpan{Start: 10, End: 14}

// requireWireShape checks the rules the frozen witness wire enforces: counts
// within mutants, each state agreeing with its counts, survivors inside the
// hunk, and a not-run witness that names why.
func requireWireShape(t *testing.T, witness wire.DiscriminationWitness) {
	t.Helper()
	if witness.Killed+witness.Survived > witness.Mutants || int64(len(witness.Survivors)) != witness.Survived {
		t.Fatalf("witness counts are inconsistent: %+v", witness)
	}
	switch witness.State {
	case wire.DiscriminationDiscriminates:
		if witness.Killed < 1 || witness.Survived != 0 {
			t.Fatalf("discriminates witness disagrees with its counts: %+v", witness)
		}
	case wire.DiscriminationSurvived:
		if witness.Survived < 1 {
			t.Fatalf("survived witness has no survivor: %+v", witness)
		}
	case wire.DiscriminationNotRun:
		if witness.Mutants != 0 || witness.Detail == "" {
			t.Fatalf("not-run witness carries counts or no reason: %+v", witness)
		}
	default:
		t.Fatalf("unknown state %q", witness.State)
	}
	for _, survivor := range witness.Survivors {
		if survivor.Line < int64(hunkSpan.Start) || survivor.Line > int64(hunkSpan.End) {
			t.Fatalf("survivor line %d lies outside the hunk", survivor.Line)
		}
	}
}

// TestTCQV0069FoldKillsOnAnySurvivesOnAll is the one fold for both arms: a
// mutant killed by either test is killed, one every test lets live survives,
// and one that never built or loaded in a run and lived in the other is
// neither - never a kill.
func TestTCQV0069FoldKillsOnAnySurvivesOnAll(t *testing.T) {
	goRuns := []armRun{
		goRun("a_test.go", goReport(mutate.OutcomeKilled, mutate.OutcomeSurvived, mutate.OutcomeUncompilable), nil),
		goRun("b_test.go", goReport(mutate.OutcomeSurvived, mutate.OutcomeSurvived, mutate.OutcomeSurvived), nil),
	}
	rubyRuns := []armRun{
		rubyRun("a_spec.rb", rubyReport(rbmutate.OutcomeKilled, rbmutate.OutcomeSurvived, rbmutate.OutcomeUnloadable), nil),
		rubyRun("b_spec.rb", rubyReport(rbmutate.OutcomeSurvived, rbmutate.OutcomeSurvived, rbmutate.OutcomeSurvived), nil),
	}
	for name, arm := range map[string]struct {
		runner Runner
		runs   []armRun
	}{"go": {goRunner, goRuns}, "ruby": {rubyRunner, rubyRuns}} {
		witness := foldRuns(arm.runner, "lib/calc", hunkSpan, arm.runs)
		requireWireShape(t, witness)
		if witness.State != wire.DiscriminationSurvived || witness.Mutants != 3 || witness.Killed != 1 || witness.Survived != 1 || witness.Survivors[0].Line != 11 {
			t.Fatalf("%s fold = %+v, want 3 mutants, 1 killed, 1 survivor at line 11, 1 neither", name, witness)
		}
	}
	for name, run := range map[string]armRun{
		"go":   goRun("a_test.go", goReport(mutate.OutcomeKilled, mutate.OutcomeUncompilable), nil),
		"ruby": rubyRun("a_spec.rb", rubyReport(rbmutate.OutcomeKilled, rbmutate.OutcomeTimedOut), nil),
	} {
		runner := goRunner
		if name == "ruby" {
			runner = rubyRunner
		}
		witness := foldRuns(runner, "lib/calc", hunkSpan, []armRun{run})
		requireWireShape(t, witness)
		if witness.State != wire.DiscriminationDiscriminates || witness.Killed != 1 || witness.Mutants != 2 {
			t.Fatalf("%s fold = %+v, want discriminates with one kill of two", name, witness)
		}
	}
}

// TestTCQV0068OnlyAParsedKillCounts keeps the contract's kill rule at the
// adapters: an unadmitted or timed-out Ruby run and an uncompilable Go
// mutant are neither, so a run with nothing else is not-run.
func TestTCQV0068OnlyAParsedKillCounts(t *testing.T) {
	ruby := rubyRun("a_spec.rb", rubyReport(rbmutate.OutcomeUnadmitted, rbmutate.OutcomeTimedOut, rbmutate.OutcomeUnloadable), nil)
	if witness := foldRuns(rubyRunner, "lib/calc.rb", hunkSpan, []armRun{ruby}); witness.State != wire.DiscriminationNotRun || witness.Detail != "no mutant loaded" {
		t.Fatalf("ruby fold = %+v, want not-run: no mutant loaded", witness)
	}
	golang := goRun("a_test.go", goReport(mutate.OutcomeUncompilable, mutate.OutcomeUncompilable), nil)
	if witness := foldRuns(goRunner, "pkg/calc.go", hunkSpan, []armRun{golang}); witness.State != wire.DiscriminationNotRun || witness.Detail != "no mutant compiled" {
		t.Fatalf("go fold = %+v, want not-run: no mutant compiled", witness)
	}
}

// TestTCQV0070IncompleteRunsAreNotRun covers every run the contract does not
// let complete: a runner error, a spent budget, an unsupported claim (a
// failed baseline), an empty plan, a short report, runs that disagree on the
// plan, and a survivor outside the hunk.
func TestTCQV0070IncompleteRunsAreNotRun(t *testing.T) {
	short := goReport(mutate.OutcomeKilled, mutate.OutcomeSurvived)
	short.Outcomes = short.Outcomes[:1]
	shifted := goReport(mutate.OutcomeKilled)
	shifted.Outcomes[0].Mutant.Line = 12
	outside := goReport(mutate.OutcomeSurvived)
	outside.Outcomes[0].Mutant.Line = 40
	cases := map[string][]armRun{
		"go error":      {goRun("a_test.go", mutate.Report{}, errors.New("export vanished"))},
		"ruby error":    {rubyRun("a_spec.rb", rbmutate.Report{}, errors.New("run ended without a verdict"))},
		"go budget":     {goRun("a_test.go", mutate.Report{Verdict: mutate.BudgetExceeded, Mutants: 2, Detail: "budget exhausted"}, nil)},
		"ruby baseline": {rubyRun("a_spec.rb", rbmutate.Report{Verdict: mutate.Unsupported, Detail: "baseline failed"}, nil)},
		"go baseline":   {goRun("a_test.go", mutate.Report{Verdict: mutate.Unsupported, Detail: "baseline failed"}, nil)},
		"go empty plan": {goRun("a_test.go", mutate.Report{Verdict: mutate.NoMutants, Detail: "no mutable declarations"}, nil)},
		"ruby empty":    {rubyRun("a_spec.rb", rbmutate.Report{Verdict: mutate.NoMutants, Detail: "no mutable sites"}, nil)},
		"go short":      {goRun("a_test.go", short, nil)},
		"plan disagree": {goRun("a_test.go", goReport(mutate.OutcomeKilled), nil), goRun("b_test.go", shifted, nil)},
		"plan length":   {goRun("a_test.go", goReport(mutate.OutcomeKilled), nil), goRun("b_test.go", goReport(mutate.OutcomeKilled, mutate.OutcomeKilled), nil)},
		"outside hunk":  {goRun("a_test.go", outside, nil)},
		"no runs":       {},
	}
	for name, runs := range cases {
		var witness wire.DiscriminationWitness
		for _, run := range runs {
			if run.notRun != "" {
				witness = notRun(run.notRun)
				break
			}
		}
		if witness.State == "" {
			witness = foldRuns(goRunner, "pkg/calc.go", hunkSpan, runs)
		}
		requireWireShape(t, witness)
		if witness.State != wire.DiscriminationNotRun {
			t.Errorf("%s: state = %s, want not-run", name, witness.State)
		}
	}
}

// TestTCQV0092SymlinkRefusalIsTheWorkflowReason pins that the runner's
// symlink refusal text is the reason cem discriminate classifies; the
// workflow cannot import the runner.
func TestTCQV0092SymlinkRefusalIsTheWorkflowReason(t *testing.T) {
	if mutate.SymlinkRefusal != workflow.SandboxUnpackSymlink {
		t.Fatalf("mutate.SymlinkRefusal %q != workflow.SandboxUnpackSymlink %q", mutate.SymlinkRefusal, workflow.SandboxUnpackSymlink)
	}
}
