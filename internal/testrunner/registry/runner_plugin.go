// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Beamfall/corvint/internal/testrunner"
)

// RunnerPlugin is the compiled-in runner selection SPI (RSP-V0-025).
// Methods interpret core-captured artifacts or describe commands; they never
// execute tests, write the evidence store or decide exclusions/qualification.
// These in-process values do not modify the existing Build/Parse wire shapes.
type RunnerPlugin interface {
	Pieces() testrunner.RunnerPieces
	Inventory(context.Context, InventoryRequest) (testrunner.RunnerInventory, error)
	Command(context.Context, CommandRequest) (CommandResponse, error)
	Parse(context.Context, ParseRequest) ([]testrunner.RunnerResult, error)
	Evidence(context.Context, EvidenceRequest) ([]NativeEvidence, error)
}

type InventoryRequest struct {
	Key    testrunner.InventoryKey
	Output string // core-captured discovery output
}

type CommandRequest struct {
	Key     testrunner.InventoryKey
	Purpose string   // inventory, run, collect
	Tests   []string // nil for FULL; non-nil empty means no tests
	RunDir  string
}

type CommandResponse struct {
	Argv      []string
	Cwd       string
	Env       map[string]string
	Artifacts []string
	Superset  bool // argv may run more than Tests, never fewer
}

type ParseRequest struct{ Runner, Report string }
type EvidenceRequest struct {
	Runner    string
	Artifacts []string
}

// NativeEvidence carries only the three plugin-produced components. Their
// opaque payloads require the evidence record contract's validation before
// admission; this SPI neither decodes nor publishes them. Trace components and
// qualification cannot be supplied here. No current runner produces evidence.
type NativeEvidence struct {
	Test                      string
	Executed, Regions, Loaded json.RawMessage
}

type absentPlugin struct{ runner string }

func (p absentPlugin) Pieces() testrunner.RunnerPieces {
	return testrunner.AbsentRunnerPieces(p.runner)
}
func (p absentPlugin) unavailable() error { return fmt.Errorf("%s", p.Pieces().FullReason(false)) }
func (p absentPlugin) Inventory(context.Context, InventoryRequest) (testrunner.RunnerInventory, error) {
	return testrunner.RunnerInventory{Reason: p.unavailable().Error()}, p.unavailable()
}
func (p absentPlugin) Command(context.Context, CommandRequest) (CommandResponse, error) {
	return CommandResponse{}, p.unavailable()
}
func (p absentPlugin) Parse(context.Context, ParseRequest) ([]testrunner.RunnerResult, error) {
	return nil, p.unavailable()
}
func (p absentPlugin) Evidence(context.Context, EvidenceRequest) ([]NativeEvidence, error) {
	return nil, p.unavailable()
}

// SelectionPlugin exposes registered selection pieces. Bound execution inputs
// still come from core factories; lookup confers no qualification. The plugin
// comes from the registering family's table row, never from a runner name the
// registry names itself (AFP-V0-9985).
func SelectionPlugin(runner string) (RunnerPlugin, bool) {
	family, ok := Lookup(runner)
	if !ok {
		return nil, false
	}
	if family.Selection != nil && family.SelectionRunners != nil && has(family.SelectionRunners(), runner) {
		return family.Selection(runner), true
	}
	return absentPlugin{runner}, true
}

// SelectionSession retains divergence until the core supplies a newly qualified
// session. A plugin cannot restore its own qualification after a FULL retry.
type SelectionSession struct {
	plugin     RunnerPlugin
	key        testrunner.InventoryKey
	qualified  bool
	divergence string
}

// NewSelectionSession receives qualification from the core harness, never from
// Pieces. Existing runners all declare absence and cannot narrow this way.
func NewSelectionSession(plugin RunnerPlugin, key testrunner.InventoryKey, qualified bool) *SelectionSession {
	return &SelectionSession{plugin: plugin, key: key, qualified: qualified}
}

// Inventory consumes captured output and admits only a complete matching key.
// Even a plugin error alongside a partial inventory cannot justify selection.
func (s *SelectionSession) Inventory(ctx context.Context, output string) testrunner.RunnerInventory {
	if s.plugin == nil {
		return testrunner.RunnerInventory{Reason: "runner-plugin-absent"}
	}
	in, err := s.plugin.Inventory(ctx, InventoryRequest{Key: s.key, Output: output})
	if err == nil {
		in, err = testrunner.CanonicalInventory(in, s.key)
	}
	if err != nil {
		return testrunner.RunnerInventory{Reason: "inventory-failed:" + err.Error()}
	}
	return in
}

func (s *SelectionSession) Plan(in testrunner.RunnerInventory, exclusions []testrunner.Exclusion, mandated []string) testrunner.RunSet {
	plan := testrunner.PlanRunSet(in, s.key, exclusions, mandated)
	reason := s.divergence
	if reason == "" {
		if s.plugin == nil {
			reason = "runner-plugin-absent"
		} else {
			reason = s.plugin.Pieces().FullReason(s.qualified)
		}
	}
	if reason != "" {
		plan = testrunner.PlanRunSet(in, s.key, nil, mandated)
		plan.Full, plan.Reason = true, reason
	}
	return plan
}

// Run uses a core-owned executor, then reconciles the report. FULL omits Tests;
// a native superset still owes every planned identity a terminal result. The
// executor owns command/path/tool admission and process containment, as with the
// existing execution path. This helper itself starts no processes.
func (s *SelectionSession) Run(ctx context.Context, in testrunner.RunnerInventory, exclusions []testrunner.Exclusion, mandated []string, runDir string, allowRerun bool, execute func(context.Context, CommandResponse) (string, error)) testrunner.InventoryRun {
	plan := s.Plan(in, exclusions, mandated)
	out := testrunner.RunInventoried(in, s.key, plan, allowRerun, func(plan testrunner.RunSet) ([]testrunner.RunnerResult, bool, error) {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		if s.plugin == nil {
			return nil, false, fmt.Errorf("runner-plugin-absent")
		}
		if !plan.Full && len(plan.Tests) == 0 {
			return nil, false, nil
		}
		tests := plan.Tests
		if plan.Full {
			tests = nil
		}
		cmd, err := s.plugin.Command(ctx, CommandRequest{Key: s.key, Purpose: "run", Tests: tests, RunDir: runDir})
		if err != nil {
			return nil, false, err
		}
		if len(cmd.Argv) == 0 || cmd.Argv[0] == "" {
			return nil, cmd.Superset, fmt.Errorf("runner-command-empty")
		}
		// RSP-V0-003: captured results stand even when execution fails.
		report, executeErr := execute(ctx, cmd)
		results, parseErr := s.plugin.Parse(ctx, ParseRequest{Runner: s.key.Runner, Report: report})
		return results, cmd.Superset, errors.Join(executeErr, parseErr)
	})
	if out.NeedsRequalification {
		s.qualified = false
		s.divergence = "inventory-divergence:requalification-required"
	}
	return out
}
