// SPDX-License-Identifier: Apache-2.0
package runnerpolicy

import (
	"context"
	"fmt"
	"slices"
	"time"

	q "github.com/Beamfall/corvint/internal/runnerqualification"
	s "github.com/Beamfall/corvint/internal/runnerselection"
	"github.com/Beamfall/corvint/internal/testrunner"
)

// Retained pins the actual run, not a later recomputation of selection. Its
// private slices are copied on capture/access so replay cannot change exclusions.
type Retained struct {
	base, head, runner, repository string
	inventory                      testrunner.RunnerInventory
	excluded                       []string
	classes                        []s.Class
	valid                          bool
}

func (p Retained) Excluded() []string { return slices.Clone(p.excluded) }
func (p Retained) Base() string       { return p.base }
func (p Retained) Head() string       { return p.head }

// Retain is called after the host records the actual native run set. A native
// superset is allowed; a subset, unknown inventory, or unbound comparison is not.
func Retain(d Decision, runner string, actual testrunner.RunSet) (Retained, error) {
	fail := func() (Retained, error) { return Retained{}, fmt.Errorf("retained-plan-unverifiable") }
	planned, ok := d.Runs[runner]
	if !ok || !commit(d.input.Base) || !commit(d.input.Head) || (!actual.Full && (planned.Full || !containsAll(actual.Tests, planned.Tests))) {
		return fail()
	}
	var candidate *s.Runner
	for i := range d.input.Runners {
		if d.input.Runners[i].Name == runner {
			if candidate != nil {
				return fail()
			}
			candidate = &d.input.Runners[i]
		}
	}
	if candidate == nil {
		return fail()
	}
	key := testrunner.InventoryKey{Checkout: d.input.Head, Runner: runner, ConfigDigest: candidate.Identity.ConfigurationDigest, OS: candidate.Execution.OS, Arch: candidate.Execution.Architecture}
	inv, err := testrunner.CanonicalInventory(candidate.Inventory, key)
	if err != nil || len(actual.Tests) > s.MaxInputs {
		return fail()
	}
	known, selected := map[string]bool{}, map[string]bool{}
	for _, t := range inv.Tests {
		known[t.ID] = true
	}
	for _, t := range actual.Tests {
		if !known[t] || selected[t] {
			return fail()
		}
		selected[t] = true
	}
	p := Retained{base: d.input.Base, head: d.input.Head, runner: runner, repository: d.input.Repository, inventory: inv, valid: true}
	if !actual.Full {
		for _, t := range inv.Tests {
			if !selected[t.ID] {
				p.excluded = append(p.excluded, t.ID)
			}
		}
	}
	for _, rp := range d.Shadow.Runners {
		if rp.Runner == runner {
			for _, c := range rp.Changes {
				if !knownClass(c.Class) {
					return fail()
				}
				p.classes = append(p.classes, c.Class)
			}
		}
	}
	slices.Sort(p.classes)
	p.classes = slices.Compact(p.classes)
	if len(p.excluded) > 0 && len(p.classes) == 0 {
		return fail()
	}
	return p, nil
}

// Finding is retained under protected authority with its original plan. It is
// never inferred from a plugin report or a newly computed excluded set.
type Finding struct {
	Runner, Test, Kind, Reason string // miss, flake, unknown
	Base, Head                 string
	Classes                    []s.Class
	Sequence                   uint64 // protected verifier order, not wall time or plugin input
	Plan                       Retained
}
type Detection struct {
	Sampled bool
	// Complete is false when the request or capture could not be evaluated.
	// Hosts must retain incomplete history as incomplete, even with no findings.
	Complete bool
	Reason   string
	Findings []Finding
	Plan     Retained
}

// Capture runs the requested checkout in isolation and reports native outcomes.
// The core host owns containment and protected retention. No read command calls it.
type Capture func(context.Context, string, string, testrunner.RunSet) (q.Observation, error)

func Detect(ctx context.Context, sampling Sampling, defaultBranch bool, p Retained, sequence uint64, capture Capture) Detection {
	out := Detection{Plan: p}
	finding := func(test, kind, reason string) {
		out.Findings = append(out.Findings, Finding{Runner: p.runner, Test: test, Kind: kind, Reason: reason, Base: p.base, Head: p.head, Classes: slices.Clone(p.classes), Sequence: sequence, Plan: p})
	}
	if !p.valid {
		out.Reason = "retained-plan-unverifiable"
		return out
	}
	if sampling.status != "owner-set" {
		out.Reason = sampling.Status()
		return out
	}
	if !defaultBranch {
		out.Complete = true
		out.Reason = "not-default-branch"
		return out
	}
	if !sampling.Sample(p.base, p.head, true) {
		out.Complete = true
		out.Reason = "not-sampled"
		return out
	}
	if len(p.excluded) == 0 {
		out.Complete = true
		out.Reason = "no-exclusions"
		return out
	}
	if capture == nil || sequence == 0 {
		out.Reason = "capture-or-order-unavailable"
		finding("", "unknown", out.Reason)
		return out
	}
	out.Sampled = true
	inventories := map[string][]testrunner.InventoryTest{}
	observe := func(commit string, run testrunner.RunSet) (map[string]string, error) {
		obs, err := capture(ctx, commit, p.runner, run)
		if err != nil {
			return nil, err
		}
		key := p.inventory.Key
		key.Checkout = commit
		inv, err := testrunner.CanonicalInventory(obs.Inventory, key)
		if err != nil {
			return nil, err
		}
		if commit == p.head && !slices.Equal(inv.Tests, p.inventory.Tests) {
			return nil, fmt.Errorf("head-inventory-divergent")
		}
		if prior, ok := inventories[commit]; ok && !slices.Equal(prior, inv.Tests) {
			return nil, fmt.Errorf("rerun-inventory-divergent")
		}
		inventories[commit] = inv.Tests
		// FULL evidence must really describe a FULL capture, not just a subset
		// whose results happen to fit a caller-supplied inventory.
		if run.Full && !obs.Plan.Full {
			return nil, fmt.Errorf("sample-not-full")
		}
		if !run.Full && !obs.Plan.Full && !containsAll(obs.Plan.Tests, run.Tests) {
			return nil, fmt.Errorf("rerun-plan-incomplete")
		}
		// RSP-V0-057: certify only an inventoried FULL or explicit covering
		// set, with exactly one terminal result for every captured identity.
		// Native widening is valid; undeclared or unknown results are not.
		rec := testrunner.ReconcileInventory(obs.Inventory, key, obs.Plan, obs.Results, false)
		if rec.Full || rec.NeedsRequalification {
			return nil, fmt.Errorf("sample-results-incomplete")
		}
		results := map[string]string{}
		for _, r := range obs.Results {
			results[r.ID] = r.Outcome
		}
		for _, id := range p.excluded {
			if run.Full && results[id] == "" {
				return nil, fmt.Errorf("excluded-result-missing")
			}
		}
		return results, nil
	}
	base, err := observe(p.base, testrunner.RunSet{Full: true})
	if err != nil {
		out.Reason = "base-sample-unverifiable"
		finding("", "unknown", out.Reason)
		return out
	}
	head, err := observe(p.head, testrunner.RunSet{Full: true})
	if err != nil {
		out.Reason = "head-sample-unverifiable"
		finding("", "unknown", out.Reason)
		return out
	}
	for _, test := range p.excluded {
		if head[test] != "failed" || base[test] == "failed" {
			continue
		}
		if base[test] != "passed" {
			finding(test, "unknown", "base-outcome-not-passed")
			continue
		}
		run := testrunner.RunSet{Tests: []string{test}}
		b, e1 := observe(p.base, run)
		h, e2 := observe(p.head, run)
		switch {
		case e1 != nil || e2 != nil:
			finding(test, "unknown", "confirmation-unverifiable")
		case h[test] == "passed":
			finding(test, "flake", "head-failure-did-not-reproduce")
		case h[test] != "failed" || b[test] != "passed":
			finding(test, "unknown", "confirmation-outcomes-inconclusive")
		default:
			finding(test, "miss", "confirmed-base-pass-head-fail")
		}
	}
	out.Complete = true
	for _, f := range out.Findings {
		if f.Kind == "unknown" {
			out.Complete = false
		}
	}
	out.Reason = "sample-complete"
	return out
}

func validFinding(f Finding) bool {
	if !f.Plan.valid || f.Runner != f.Plan.runner || f.Base != f.Plan.base || f.Head != f.Plan.head || f.Sequence == 0 || !slices.Equal(f.Classes, f.Plan.classes) {
		return false
	}
	if f.Kind == "unknown" {
		return f.Reason != ""
	}
	return (f.Kind == "miss" || f.Kind == "flake") && f.Reason != "" && slices.Contains(f.Plan.excluded, f.Test) && len(f.Classes) > 0
}

// Requalify clears only this runner's blocked classes after T6 validates a newer
// admitted qualification. A failed, forged, stale or partial record clears none.
// The returned slice is the active policy view, not a replacement event log.
// The host retains the original findings and requalification evidence immutably
// under protected authority, including after a block is cleared.
func Requalify(findings []Finding, d q.Declaration, records []q.Admission, ceiling time.Duration) ([]Finding, error) {
	if len(findings) > s.MaxInputs {
		return nil, fmt.Errorf("miss-history-capped")
	}
	plan := q.PlanFor(d, records, ceiling, true)
	if plan.Status != "qualified" {
		return nil, fmt.Errorf("requalification-not-admitted")
	}
	var sequence uint64
	for _, r := range records {
		if r.Admitted && r.Record.Declaration.Runner == d.Runner && r.Sequence > sequence {
			sequence = r.Sequence
		}
	}
	out := make([]Finding, 0, len(findings))
	for _, f := range findings {
		if !validFinding(f) {
			return nil, fmt.Errorf("miss-history-unverifiable")
		}
		if f.Runner == d.Runner && f.Kind != "flake" {
			if d.Corpus.Repository != f.Plan.repository || d.Identity.Execution.OS != f.Plan.inventory.Key.OS || d.Identity.Execution.Architecture != f.Plan.inventory.Key.Arch {
				out = append(out, f)
				continue
			}
			if sequence <= f.Sequence {
				return nil, fmt.Errorf("requalification-not-newer")
			}
			if !containsClasses(plan.Qualification.Classes, f.Classes) {
				out = append(out, f)
				continue
			}
			continue
		}
		out = append(out, f)
	}
	return out, nil
}
func containsClasses(have, want []s.Class) bool {
	for _, c := range want {
		if !slices.Contains(have, c) {
			return false
		}
	}
	return true
}
