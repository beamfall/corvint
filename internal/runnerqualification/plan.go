// SPDX-License-Identifier: Apache-2.0

package runnerqualification

import (
	"reflect"
	"time"

	"github.com/Beamfall/corvint/internal/runnerselection"
)

// PlanFor is a pure scheduler: its jobs are consumed by the operator's CI job,
// never started by read commands. Each identity change produces a job keyed by
// the entire new identity. The host deduplicates that key while work is pending.
// The cost ceiling must come from trusted configuration; there is no invented
// default. Records are ordered by admission sequence, not a plugin timestamp.
func PlanFor(d Declaration, records []Admission, ceiling time.Duration, samplingEnabled bool) Plan {
	out := Plan{Mode: "full", Status: "requalifying"}
	pending := func(reason string) Plan {
		out.Reasons = append(out.Reasons, reason)
		if ceiling > 0 {
			out.Jobs = []Job{{Identity: d.Identity, Declaration: d, Budget: ceiling, Reason: reason}}
		} else {
			out.Reasons = append(out.Reasons, "ci-cost-ceiling-unset")
		}
		return out
	}
	if err := validate(d); err != nil {
		return pending(err.Error())
	}
	if ceiling <= 0 {
		out.Reasons = []string{"ci-cost-ceiling-unset"}
		return out
	}
	if len(records) > MaxPopulation {
		return pending("qualification-records-capped")
	}
	var latest *Admission
	for i := range records {
		r := &records[i]
		if !r.Admitted || !sameScope(r.Record.Declaration, d) {
			continue
		}
		if latest == nil || r.Sequence > latest.Sequence {
			latest = r
		} else if r.Sequence == latest.Sequence {
			return pending("qualification-order-ambiguous")
		}
	}
	if latest == nil {
		return pending("qualification-not-admitted")
	}
	r := latest.Record
	if !reflect.DeepEqual(r.Declaration, d) {
		return pending("qualification-identity-changed")
	}
	if !r.Complete || len(r.Reasons) != 0 || len(r.Classes) != len(d.Classes) || len(r.Timings) == 0 || len(r.Merges) == 0 {
		return pending("qualification-incomplete")
	}
	if r.Budget <= 0 || r.Wall < 0 {
		return pending("qualification-cost-invalid")
	}
	if r.Budget > ceiling {
		return pending("qualification-cost-ceiling-exceeded")
	}
	if r.Wall > r.Budget {
		return pending("qualification-budget-exceeded")
	}
	classes := map[runnerselection.Class]bool{}
	for _, c := range r.Classes {
		if !hexValue(c.PopulationDigest, 32) {
			return pending("qualification-population-digest-invalid")
		}
		if !knownClass(c.Class) || classes[c.Class] || !c.Passed || !c.ControlPassed || !c.ReplayPassed || len(c.Reasons) != 0 || len(c.Misses) != 0 || c.PopulationSize <= 0 || c.PopulationSize > MaxPopulation || c.Trials < TrialFloor || c.Sampled > MaxSamples || c.Sampled != c.Trials+c.Unkilled || c.Unkilled < 0 || len(c.Draws) != c.Sampled || len(c.SelectedSizes) != c.Sampled || c.Upper95 != Upper95(0, c.Trials) {
			return pending("qualification-class-not-passing")
		}
		classes[c.Class] = true
	}
	for _, class := range d.Classes {
		if !classes[class] {
			return pending("qualification-class-missing")
		}
	}
	if _, reason := retainedCost(r.Timings, r.Budget); reason != "" {
		return pending(reason)
	}
	out.Status = "qualified"
	out.Qualification = runnerselection.Qualification{Admitted: true, Passed: true, SamplingEnabled: samplingEnabled, Model: d.Model, Classes: append([]runnerselection.Class{}, d.Classes...), Runner: d.Identity.Runner, Execution: d.Identity.Execution}
	if samplingEnabled {
		out.Mode = "selected"
	} else {
		out.Reasons = []string{"sampling-disabled"}
	}
	return out
}

// Scope separates independently qualified targets/configurations. Versions,
// verifier revisions and corpus commits remain invalidators WITHIN a scope:
// a newer failure or identity change must never revive an older passing record.
func sameScope(a, b Declaration) bool {
	return a.Runner == b.Runner && a.Corpus.Repository == b.Corpus.Repository &&
		a.Model == b.Model &&
		a.Identity.Execution.OS == b.Identity.Execution.OS &&
		a.Identity.Execution.Architecture == b.Identity.Execution.Architecture &&
		a.Identity.Runner.ConfigurationDigest == b.Identity.Runner.ConfigurationDigest &&
		a.Identity.Execution.EnvironmentDigest == b.Identity.Execution.EnvironmentDigest
}

// retainedCost accounts for both measured phases of every execution, including
// repeated draws and both parity models. Only the two harness models are known;
// unknown cost forms and overflow fail closed. It does not measure replay CPU
// time or invent unobserved setup/teardown time.
func retainedCost(timings []Timing, budget time.Duration) (time.Duration, string) {
	var total time.Duration
	const maxDuration = time.Duration(1<<63 - 1)
	if len(timings) == 0 || budget <= 0 {
		return 0, "qualification-cost-invalid"
	}
	for _, t := range timings {
		if t.Model != "traced" && t.Model != "untraced" {
			return total, "qualification-cost-invalid"
		}
		for _, wall := range []time.Duration{t.InventoryWall, t.CollectionWall} {
			if wall < 0 || wall > maxDuration-total {
				return total, "qualification-cost-invalid"
			}
			total += wall
		}
	}
	if total > budget {
		return total, "qualification-budget-exceeded"
	}
	return total, ""
}
