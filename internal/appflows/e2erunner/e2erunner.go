// SPDX-License-Identifier: Apache-2.0

// Package e2erunner binds the shared RSP Playwright inventory and runner
// selection model to the E2E-safe selector (RSP-V0-052). Discovery is
// exclusively the runner's native list report, read by the one Playwright
// inventory in browsercov. It sits in the affected area, so appflows reaches
// the runner packages only through the callback this package supplies.
package e2erunner

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/browsercov"
	"github.com/Beamfall/corvint/internal/jslinecov"
	"github.com/Beamfall/corvint/internal/runnerselection"
)

// Input carries the native list report and the selector's admitted T1
// records and core qualification.
type Input struct {
	Discovery jslinecov.Discovery
	Report    string
	Selector  runnerselection.Input
}

// Bind returns the appflows binding that evaluates in.
func Bind(in Input) *appflows.E2ERunnerInput {
	return &appflows.E2ERunnerInput{Evaluate: func(context.Context) appflows.E2ERunnerResult { return Evaluate(in) }}
}

// Evaluate reads the inventory, checks that it is keyed to the selector's
// checkout, configuration, runner version and platform, and runs the shared
// selector. Any malformed inventory or key mismatch fails the whole binding.
func Evaluate(in Input) appflows.E2ERunnerResult {
	fail := func(reason string) appflows.E2ERunnerResult {
		return appflows.E2ERunnerResult{Failed: true, Failure: reason}
	}
	inv, err := browsercov.PlaywrightInventory(in.Discovery, in.Report)
	if err != nil {
		return fail(inv.Reason)
	}
	var out appflows.E2ERunnerResult
	for _, t := range inv.Tests {
		var tuple []string
		if json.Unmarshal([]byte(strings.TrimPrefix(t.ID, "playwright:")), &tuple) != nil || len(tuple) != 3 {
			return fail("inventory-unavailable")
		}
		out.Tests = append(out.Tests, appflows.E2ERunnerTest{TestKey: t.ID, Path: t.Path, Project: tuple[1]})
	}
	input := in.Selector
	input.Runners = append([]runnerselection.Runner{}, input.Runners...)
	if len(input.Runners) != 1 || input.Runners[0].Name != "playwright" || input.Head != inv.Key.Checkout || input.Runners[0].Identity.ConfigurationDigest != inv.Key.ConfigDigest || input.Runners[0].Identity.RunnerVersion != in.Discovery.Version || input.Runners[0].Execution.OS != inv.Key.OS || input.Runners[0].Execution.Architecture != inv.Key.Arch {
		return fail("inventory-key-mismatch")
	}
	input.Runners[0].Inventory = inv
	for _, p := range runnerselection.Select(input).Runners {
		out.Reasons = append(out.Reasons, p.Reasons...)
	}
	return out
}
