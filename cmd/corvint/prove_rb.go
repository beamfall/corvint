package main

import (
	"context"

	"github.com/Beamfall/corvint/internal/liveverify/mutate"
	"github.com/Beamfall/corvint/internal/liveverify/rbmutate"
)

// Witness reasons of the Ruby arm (FPK-V0-071). A Ruby claim never carries a
// PRODUCED witness: the replay contract is Go's, so a judged Ruby row says so
// by name, and a run that never reached a mutant says why.
const (
	rubyWitnessNotProduced = "ruby-mutation-witness-not-produced"
	rubyRunnerUnavailable  = "ruby-runner-unavailable"
	rubyBaselineUnusable   = "ruby-baseline-unusable"
)

// proveMutationRecord is the retained per-mutant record of a Ruby row
// (FPK-V0-070): every selected mutant with its verdict and its RSpec run's
// receipt, and the baseline receipt. It is deterministic: no timings, no
// temporary paths.
type proveMutationRecord struct {
	Engine     string `json:"engine"`
	Runner     string `json:"runner,omitempty"`
	Stage      string `json:"stage"`
	Planned    int    `json:"planned"`
	Selected   int    `json:"selected"`
	Killed     int    `json:"killed"`
	Survived   int    `json:"survived"`
	Unloadable int    `json:"unloadable"`
	// Unadmitted counts containerised runs whose report the runner did
	// not admit (FPK-V0-080); host rows omit it.
	Unadmitted int               `json:"unadmitted,omitempty"`
	TimedOut   int               `json:"timed_out"`
	Skipped    int               `json:"skipped"`
	Baseline   *rbmutate.Receipt `json:"baseline,omitempty"`
	Mutants    []rbmutate.Result `json:"mutants"`
	// Target and LineBasis are a named-source row's RSpec location selector
	// and how its mutated lines were chosen (FPK-V0-076); other rows omit
	// both.
	Target    string `json:"target,omitempty"`
	LineBasis string `json:"line_basis,omitempty"`
}

// judgeRubyMutation is judgeMutation's Ruby arm (FPK-V0-068 to FPK-V0-071):
// the same export, sandbox, budget, and confinement, with rbmutate writing
// the mutants and RSpec running the cited spec file against every one.
func judgeRubyMutation(ctx context.Context, exported *mutate.Export, request rbmutate.Request) (mutationVerdict, error) {
	report, err := rbmutate.Judge(ctx, exported, request)
	if err != nil {
		return mutationVerdict{}, err
	}
	return rubyVerdict(report), nil
}

// rubyVerdict maps one engine report onto the row: KILLED passes, SURVIVED
// fails, anything else stays NOT_RUN, and the witness names why no replayable
// witness exists.
func rubyVerdict(report rbmutate.Report) mutationVerdict {
	results := report.Results
	if results == nil {
		results = []rbmutate.Result{}
	}
	return mutationVerdict{
		falsified: mutationFalsified(report.Verdict), detail: report.Detail,
		witness: rubyWitness(report),
		mutation: &proveMutationRecord{
			Engine: "rbmutate", Runner: report.Runner, Stage: string(report.Stage),
			Planned: report.Planned, Selected: report.Mutants, Killed: report.Killed, Survived: report.Survived,
			Unloadable: report.Unloadable, Unadmitted: report.Unadmitted, TimedOut: report.TimedOut, Skipped: report.Skipped,
			Baseline: report.Baseline, Mutants: results,
			Target: namedRecordTarget(report), LineBasis: report.LineBasis,
		},
	}
}

// namedRecordTarget keeps the RSpec selector on a named-source row only, so
// every other Ruby row's record is unchanged.
func namedRecordTarget(report rbmutate.Report) string {
	if report.LineBasis == "" {
		return ""
	}
	return report.Target
}

func rubyWitness(report rbmutate.Report) *proveMutationWitness {
	switch {
	case report.Verdict == mutate.Unsupported && report.Stage == rbmutate.StageRunner:
		return &proveMutationWitness{Status: unsupportedWitnessStatus, Reason: rubyRunnerUnavailable}
	case report.Verdict == mutate.Unsupported && report.Stage == rbmutate.StageBaseline:
		return &proveMutationWitness{Status: unsupportedWitnessStatus, Reason: rubyBaselineUnusable}
	case report.Verdict == mutate.Unsupported:
		return mutationWitnessNotProduced("mutation-input-unavailable")
	case report.Verdict == mutate.BudgetExceeded:
		return mutationWitnessNotProduced("mutation-budget-exhausted")
	case report.Verdict == mutate.NoMutants && report.Mutants == 0:
		return mutationWitnessNotProduced("no-mutant-in-range")
	}
	return mutationWitnessNotProduced(rubyWitnessNotProduced)
}
