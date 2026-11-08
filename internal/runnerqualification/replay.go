// SPDX-License-Identifier: Apache-2.0

package runnerqualification

import (
	"context"
	"fmt"
	"reflect"

	"github.com/Beamfall/corvint/internal/runnerselection"
)

// RetainedCapture is one invocation, including a repeated mutant draw. Replaying
// a single observation N times is not N observed experiments. No plugin or CLI
// may mark this input admitted; the core provenance verifier is a later gate.
type RetainedCapture struct {
	Experiment Experiment
	Pair       Pair
}
type ClassPopulation struct {
	Class      runnerselection.Class
	Population Population
}
type Replay struct {
	Request     Request
	Populations []ClassPopulation
	Captures    []RetainedCapture
}

type retainedDriver struct {
	replay Replay
	next   int
	verify func(context.Context, Declaration, string, []Merge) error
}

func (d *retainedDriver) Verify(ctx context.Context, decl Declaration, checkout string, merges []Merge) error {
	if d.verify == nil {
		return fmt.Errorf("pinned-corpus-verifier-absent")
	}
	return d.verify(ctx, decl, checkout, merges)
}
func (d *retainedDriver) Population(_ context.Context, _ Declaration, _ string, class runnerselection.Class) (Population, error) {
	var found *Population
	for i := range d.replay.Populations {
		p := &d.replay.Populations[i]
		if p.Class == class {
			if found != nil {
				return Population{}, fmt.Errorf("duplicate-class-population")
			}
			found = &p.Population
		}
	}
	if found == nil {
		return Population{}, fmt.Errorf("mutation-provider-absent:%s", class)
	}
	return *found, nil
}
func (d *retainedDriver) Capture(_ context.Context, _ Declaration, _ string, e Experiment) (Pair, error) {
	if d.next >= len(d.replay.Captures) {
		return Pair{}, fmt.Errorf("retained-capture-missing")
	}
	c := d.replay.Captures[d.next]
	d.next++
	if e.Base != "" {
		if c.Experiment.Base == "" {
			return Pair{}, fmt.Errorf("retained-replay-base-unpinned")
		}
		if !hexValue(c.Experiment.Base, 20) || c.Experiment.Base != e.Base {
			return Pair{}, fmt.Errorf("retained-replay-base-mismatch")
		}
	}
	if !reflect.DeepEqual(e, c.Experiment) {
		return Pair{}, fmt.Errorf("retained-experiment-mismatch")
	}
	return c.Pair, nil
}

// ReplayRetained reruns the harness's seeded draw, reconciliation, parity and
// miss checks over an exact capture sequence. This is evidence evaluation, not
// a claim that a live runner was exercised by the replay invocation.
func ReplayRetained(ctx context.Context, r Replay, verify func(context.Context, Declaration, string, []Merge) error) Record {
	if len(r.Populations) > 8 || len(r.Captures) > 2*MaxPopulation+MaxSamples+9 {
		return Record{Declaration: r.Request.Declaration, Reasons: []string{"replay-input-capped"}}
	}
	d := &retainedDriver{replay: r, verify: verify}
	out := Run(ctx, r.Request, d)
	// Run's clock times evaluation here, not execution of the retained captures.
	// Keep the original budget and measured execution-phase wall evidence; the
	// caller's context bounds evaluation independently without rewriting either.
	var reason string
	out.Wall, reason = retainedCost(out.Timings, r.Request.Budget)
	if reason != "" {
		out.Complete = false
		out.Reasons = append(out.Reasons, reason)
	}
	if d.next != len(r.Captures) {
		out.Complete = false
		out.Reasons = append(out.Reasons, "retained-captures-unused")
	}
	return out
}
