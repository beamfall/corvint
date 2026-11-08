// SPDX-License-Identifier: Apache-2.0
package runnerprotocol

import (
	"context"
	"fmt"
	"reflect"

	"github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

// Corpus describes core-captured artifacts. The kit never launches runner commands.
type Corpus struct {
	Key                       testrunner.InventoryKey
	Inventory, Report, RunDir string
	Artifacts                 []string
}
type ConformanceResult struct {
	Inventory testrunner.RunnerInventory
	Commands  []registry.CommandResponse
	Results   []testrunner.RunnerResult
	Evidence  []registry.NativeEvidence
}

// Conform exercises the same SPI for compiled-in and subprocess forms. finalize
// commits a subprocess session only after all requests have been validated.
func Conform(ctx context.Context, plugin registry.RunnerPlugin, corpus Corpus, finalize func(context.Context) error) (ConformanceResult, error) {
	out := ConformanceResult{}
	var err error
	out.Inventory, err = plugin.Inventory(ctx, registry.InventoryRequest{Key: corpus.Key, Output: corpus.Inventory})
	if err != nil {
		return out, err
	}
	out.Inventory, err = testrunner.CanonicalInventory(out.Inventory, corpus.Key)
	if err != nil {
		return out, err
	}
	for _, purpose := range []string{"inventory", "run", "collect"} {
		cmd, e := plugin.Command(ctx, registry.CommandRequest{Key: corpus.Key, Purpose: purpose, RunDir: corpus.RunDir})
		if e != nil {
			return out, e
		}
		out.Commands = append(out.Commands, cmd)
	}
	out.Results, err = plugin.Parse(ctx, registry.ParseRequest{Runner: corpus.Key.Runner, Report: corpus.Report})
	if err != nil {
		return out, err
	}
	request := registry.EvidenceRequest{Runner: corpus.Key.Runner, Artifacts: corpus.Artifacts}
	out.Evidence, err = plugin.Evidence(ctx, request)
	if finalize != nil {
		if err == nil || err.Error() != "evidence-pending-session-close" {
			return ConformanceResult{}, fmt.Errorf("conformance-evidence-not-staged:%v", err)
		}
		if err = finalize(ctx); err != nil {
			return ConformanceResult{}, err
		}
		out.Evidence, err = plugin.Evidence(ctx, request)
	}
	if err != nil {
		return ConformanceResult{}, err
	}
	return out, nil
}
func CompareConformance(a, b ConformanceResult) error {
	if !reflect.DeepEqual(a, b) {
		return fmt.Errorf("conformance-parity-mismatch")
	}
	return nil
}

// ResolvePlan retains every failed runner claim alongside admitted descriptors.
// FullAll must dominate all per-runner choices at the core planning boundary.
func ResolvePlan(plan DiscoveryPlan, store, release Discovery) (map[string]Resolved, DiscoveryPlan) {
	admitted := map[string]Resolved{}
	if plan.Full == nil {
		plan.Full = map[string]string{}
	}
	if plan.FullAll != "" {
		return admitted, plan
	}
	for _, pin := range plan.Pins {
		resolved, err := Resolve(pin, store, release)
		for _, runner := range pin.Runners {
			if err != nil {
				plan.Full[runner] = err.Error()
			}
			if plan.Full[runner] == "" {
				admitted[runner] = resolved
			}
		}
	}
	return admitted, plan
}
func (s *Session) FullPlans() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]string{}
	for _, r := range s.pin.Runners {
		if s.failure != "" {
			out[r] = s.failure
		} else {
			out[r] = "runner-not-qualified"
		}
	}
	return out
}
