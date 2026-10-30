// Package cemdiscriminate is the mutation runner behind `cem discriminate`
// (TCQ-V0-055..058) and its language-neutral runner contract
// (TCQ-V0-067..070). It lives outside internal/cem so the CEM seams' closure
// stays the standard library and internal/cem: the binary installs Open as
// workflow.OpenHunkJudge. It imports only the standard library,
// internal/liveverify/mutate, its RSpec arm internal/liveverify/rbmutate
// (TCQ-V0-066), and internal/cem/wire.
package cemdiscriminate

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/liveverify/mutate"
	"github.com/Beamfall/corvint/internal/liveverify/rbmutate"
)

// Judge runs hunk mutants on one exported copy of a target tree.
type Judge struct {
	exported *mutate.Export
	// RSpec, when set, is the argv prefix the Ruby arm runs RSpec with
	// (rbmutate.Request.RSpec); nil keeps `bundle exec rspec` beside a
	// Gemfile, else rspec on PATH.
	RSpec []string
	// unavailable is the runner-stage cause the last Judge call's Ruby arm
	// reported (rbmutate.Report.Cause), or "" (TCQ-V0-094).
	unavailable string
}

// RunnerUnavailable names why the last Judge call's runner could not start:
// rbmutate's runner-not-found or bundle-unavailable, or "" when it started
// or the call stopped for any other reason (TCQ-V0-094).
func (judge *Judge) RunnerUnavailable() string { return judge.unavailable }

// Open exports the target revision of the repository at root for the prove
// --mutate runner (mutate.Open).
func Open(ctx context.Context, root, target string) (*Judge, error) {
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		return nil, err
	}
	exported, err := mutate.Open(ctx, mutate.Request{Root: root, Git: gitExecutable, Revision: target})
	if err != nil {
		return nil, err
	}
	return &Judge{exported: exported}, nil
}

// Close removes the exported tree.
func (judge *Judge) Close() { judge.exported.Close() }

// Runner declares one compiled-in execution runner arm of the
// language-neutral runner contract (TCQ-V0-067): the source files it mutates
// and the test files it runs. The repository cannot declare a runner.
type Runner struct {
	Language     string
	SourceSuffix string
	TestSuffix   string
}

// Runners lists the compiled-in arms in dispatch order: Go under go test and
// Ruby under RSpec (TCQ-V0-066). workflow's candidate rule must agree.
func Runners() []Runner {
	return []Runner{
		{Language: "go", SourceSuffix: ".go", TestSuffix: "_test.go"},
		{Language: "ruby", SourceSuffix: ".rb", TestSuffix: "_spec.rb"},
	}
}

// RunnerFor returns the arm that judges a source path, or false for a test
// file or a language with no arm.
func RunnerFor(path string) (Runner, bool) {
	for _, runner := range Runners() {
		if strings.HasSuffix(path, runner.TestSuffix) {
			return Runner{}, false
		}
	}
	for _, runner := range Runners() {
		if strings.HasSuffix(path, runner.SourceSuffix) {
			return runner, true
		}
	}
	return Runner{}, false
}

// Judge runs every mutant of one hunk against each cited test file through
// the hunk's runner arm and folds the per-mutant outcomes (foldRuns). Any
// run the arm could not complete makes the hunk not-run. A not-run detail is
// the runner's text; the caller bounds it for the wire.
func (judge *Judge) Judge(ctx context.Context, hunk wire.Hunk, tests []string, maxMutants int, wallTime time.Duration) wire.DiscriminationWitness {
	judge.unavailable = ""
	if hunk.NewRange.Count == 0 {
		return notRun("hunk adds no lines")
	}
	runner, ok := RunnerFor(hunk.Path)
	if !ok {
		return notRun("no runner judges " + hunk.Path)
	}
	span := mutate.LineSpan{Start: int(hunk.NewRange.Start), End: int(hunk.NewRange.Start + hunk.NewRange.Count - 1)}
	runs := make([]armRun, 0, len(tests))
	for _, test := range tests {
		var run armRun
		switch runner.Language {
		case "go":
			report, err := judge.exported.Judge(ctx, mutate.Request{
				ChangedPath: hunk.Path, TestPath: test, Lines: []mutate.LineSpan{span},
				MaxMutants: maxMutants, Budget: wallTime, Complete: true,
			})
			run = goRun(test, report, err)
		case "ruby":
			report, err := rbmutate.Judge(ctx, judge.exported, rbmutate.Request{
				ChangedPath: hunk.Path, TestPath: test, Lines: []mutate.LineSpan{span},
				MaxMutants: maxMutants, Budget: wallTime, RSpec: judge.RSpec,
			})
			run = rubyRun(test, report, err)
			if err == nil && report.Stage == rbmutate.StageRunner && report.Verdict == mutate.Unsupported {
				judge.unavailable = report.Cause
			}
		}
		if run.notRun != "" {
			return notRun(run.notRun)
		}
		runs = append(runs, run)
	}
	return foldRuns(runner, hunk.Path, span, runs)
}

// verdict is what one arm run made of one mutant under the runner contract
// (TCQ-V0-069): a kill only from the runner's own parsed report, a survival
// only from a pass, and neither for anything else.
type verdict int

const (
	verdictNeither verdict = iota
	verdictKilled
	verdictSurvived
)

// plannedMutant identifies one mutant across the runs of one hunk.
type plannedMutant struct {
	operator         string
	line, start, end int
}

// judgedMutant is one mutant and one run's verdict on it.
type judgedMutant struct {
	mutant  plannedMutant
	verdict verdict
}

// armRun is one completed run of one test file in the contract's shape:
// every planned mutant in plan order with its verdict. notRun, when set,
// is why the run did not complete and the hunk cannot be judged.
type armRun struct {
	notRun  string
	mutants []judgedMutant
}

// goRun adapts a go test run (internal/liveverify/mutate). A run whose every
// mutant failed to build completes with no verdicts; a plan with no mutant,
// a spent budget, or an unsupported claim does not complete.
func goRun(test string, report mutate.Report, err error) armRun {
	if err != nil {
		return armRun{notRun: "mutation runner failed for " + test + ": " + err.Error()}
	}
	allUncompilable := report.Verdict == mutate.NoMutants && report.Mutants > 0 && report.Uncompilable == report.Mutants
	if report.Verdict != mutate.Killed && report.Verdict != mutate.Survived && !allUncompilable {
		return armRun{notRun: strings.ToLower(string(report.Verdict)) + ": " + report.Detail}
	}
	if len(report.Outcomes) != report.Mutants {
		return armRun{notRun: "incomplete mutation report for " + test}
	}
	run := armRun{mutants: make([]judgedMutant, 0, len(report.Outcomes))}
	for _, outcome := range report.Outcomes {
		judged := judgedMutant{mutant: plannedMutant{operator: outcome.Mutant.Operator, line: outcome.Mutant.Line, start: outcome.Mutant.Start, end: outcome.Mutant.End}}
		switch outcome.Outcome {
		case mutate.OutcomeKilled:
			judged.verdict = verdictKilled
		case mutate.OutcomeSurvived:
			judged.verdict = verdictSurvived
		}
		run.mutants = append(run.mutants, judged)
	}
	return run
}

// rubyRun adapts an RSpec run (internal/liveverify/rbmutate). A mutant that
// did not load, timed out, or whose report was not admitted is neither.
func rubyRun(test string, report rbmutate.Report, err error) armRun {
	if err != nil {
		return armRun{notRun: "mutation runner failed for " + test + ": " + err.Error()}
	}
	asked := report.Verdict == mutate.NoMutants && report.Mutants > 0
	if report.Verdict != mutate.Killed && report.Verdict != mutate.Survived && !asked {
		return armRun{notRun: strings.ToLower(string(report.Verdict)) + ": " + report.Detail}
	}
	if len(report.Results) != report.Mutants {
		return armRun{notRun: "incomplete mutation report for " + test}
	}
	run := armRun{mutants: make([]judgedMutant, 0, len(report.Results))}
	for _, result := range report.Results {
		judged := judgedMutant{mutant: plannedMutant{operator: result.Operator, line: result.Line, start: result.Start, end: result.End}}
		switch result.Outcome {
		case rbmutate.OutcomeKilled:
			judged.verdict = verdictKilled
		case rbmutate.OutcomeSurvived:
			judged.verdict = verdictSurvived
		}
		run.mutants = append(run.mutants, judged)
	}
	return run
}

// foldRuns is the contract's one fold over the cited tests of one hunk
// (TCQ-V0-069): a mutant is killed when any test kills it, survives only
// when every test lets it live, and is neither otherwise. Every run must
// name the same plan, since the plan is a function of the changed file and
// lines; a disagreement, or a survivor outside the hunk, is not-run. No
// killed and no survived mutant is not-run.
func foldRuns(runner Runner, path string, span mutate.LineSpan, runs []armRun) wire.DiscriminationWitness {
	if len(runs) == 0 {
		return notRun("no test ran")
	}
	plan := runs[0].mutants
	killed := make([]bool, len(plan))
	survivedAll := make([]bool, len(plan))
	for index := range plan {
		survivedAll[index] = true
	}
	for _, run := range runs {
		if len(run.mutants) != len(plan) {
			return notRun("runs disagree on the mutant plan")
		}
		for index, judged := range run.mutants {
			if judged.mutant != plan[index].mutant {
				return notRun("runs disagree on the mutant plan")
			}
			killed[index] = killed[index] || judged.verdict == verdictKilled
			survivedAll[index] = survivedAll[index] && judged.verdict == verdictSurvived
		}
	}
	witness := wire.DiscriminationWitness{Mutants: int64(len(plan)), Survivors: []wire.SurvivingMutant{}, State: wire.DiscriminationDiscriminates}
	for index, judged := range plan {
		mutant := judged.mutant
		switch {
		case killed[index]:
			witness.Killed++
		case survivedAll[index]:
			if mutant.line < span.Start || mutant.line > span.End {
				return notRun(fmt.Sprintf("survivor at line %d lies outside the hunk", mutant.line))
			}
			witness.Survived++
			witness.Survivors = append(witness.Survivors, wire.SurvivingMutant{
				Operator: mutant.operator, Line: int64(mutant.line),
				Description: fmt.Sprintf("%s at %s:%d (bytes %d..%d) passed every cited test", mutant.operator, path, mutant.line, mutant.start, mutant.end),
			})
		}
	}
	if witness.Survived > 0 {
		witness.State = wire.DiscriminationSurvived
	}
	if witness.Killed == 0 && witness.Survived == 0 {
		if runner.Language == "ruby" {
			return notRun("no mutant loaded")
		}
		return notRun("no mutant compiled")
	}
	return witness
}

func notRun(detail string) wire.DiscriminationWitness {
	return wire.DiscriminationWitness{Survivors: []wire.SurvivingMutant{}, State: wire.DiscriminationNotRun, Detail: detail}
}
