// Package cemdiscriminate is the mutation runner behind `cem discriminate`
// (TCQ-V0-055..058). It lives outside internal/cem so the CEM seams' closure
// stays the standard library and internal/cem: the binary installs Open as
// workflow.OpenHunkJudge. It imports only the standard library,
// internal/liveverify/mutate, and internal/cem/wire.
package cemdiscriminate

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/liveverify/mutate"
)

// Judge runs hunk mutants on one exported copy of a target tree.
type Judge struct {
	exported *mutate.Export
}

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

// Judge runs every mutant of one hunk against each cited test file and folds
// the reports: a mutant survives only when every cited test lets it live, and
// any report the runner could not complete makes the hunk not-run. A not-run
// detail is the runner's text; the caller bounds it for the wire.
func (judge *Judge) Judge(ctx context.Context, hunk wire.Hunk, tests []string, maxMutants int, wallTime time.Duration) wire.DiscriminationWitness {
	if hunk.NewRange.Count == 0 {
		return notRun("hunk adds no lines")
	}
	span := mutate.LineSpan{Start: int(hunk.NewRange.Start), End: int(hunk.NewRange.Start + hunk.NewRange.Count - 1)}
	var reports []mutate.Report
	for _, test := range tests {
		report, err := judge.exported.Judge(ctx, mutate.Request{
			ChangedPath: hunk.Path, TestPath: test, Lines: []mutate.LineSpan{span},
			MaxMutants: maxMutants, Budget: wallTime, Complete: true,
		})
		if err != nil {
			return notRun("mutation runner failed for " + test + ": " + err.Error())
		}
		if report.Verdict != mutate.Killed && report.Verdict != mutate.Survived {
			return notRun(strings.ToLower(string(report.Verdict)) + ": " + report.Detail)
		}
		reports = append(reports, report)
	}
	return foldReports(hunk.Path, reports)
}

// foldReports intersects survivors across the cited tests of one hunk. The
// mutant plan is a function of the changed file and lines, so every report
// judged the same mutants and the first report's counts describe the set.
func foldReports(path string, reports []mutate.Report) wire.DiscriminationWitness {
	survivors := reports[0].Survivors
	for _, report := range reports[1:] {
		survivors = survivedBoth(survivors, report.Survivors)
	}
	witness := wire.DiscriminationWitness{
		Mutants: int64(reports[0].Mutants), Survived: int64(len(survivors)),
		Survivors: []wire.SurvivingMutant{}, State: wire.DiscriminationDiscriminates,
	}
	witness.Killed = witness.Mutants - int64(reports[0].Uncompilable) - witness.Survived
	for _, mutant := range survivors {
		witness.Survivors = append(witness.Survivors, wire.SurvivingMutant{
			Operator: mutant.Operator, Line: int64(mutant.Line),
			Description: fmt.Sprintf("%s at %s:%d (bytes %d..%d) passed every cited test", mutant.Operator, path, mutant.Line, mutant.Start, mutant.End),
		})
	}
	if witness.Survived > 0 {
		witness.State = wire.DiscriminationSurvived
	}
	if witness.Killed == 0 && witness.Survived == 0 {
		return notRun("no mutant compiled")
	}
	return witness
}

func survivedBoth(first, second []mutate.Survivor) []mutate.Survivor {
	keep := map[mutate.Survivor]bool{}
	for _, mutant := range second {
		keep[mutant] = true
	}
	var both []mutate.Survivor
	for _, mutant := range first {
		if keep[mutant] {
			both = append(both, mutant)
		}
	}
	return both
}

func notRun(detail string) wire.DiscriminationWitness {
	return wire.DiscriminationWitness{Survivors: []wire.SurvivingMutant{}, State: wire.DiscriminationNotRun, Detail: detail}
}
