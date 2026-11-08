// SPDX-License-Identifier: Apache-2.0

package registry_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

type fixturePlugin struct {
	inventory    tr.RunnerInventory
	inventoryErr error
	commands     []registry.CommandRequest
	reports      [][]tr.RunnerResult
	parseErrors  []error
	parsed       int
	superset     bool
}

func (p *fixturePlugin) Pieces() tr.RunnerPieces {
	return tr.RunnerPieces{Inventory: tr.RunnerPiece{Present: true}, TestEvidence: tr.RunnerPiece{Present: true}, Qualification: tr.RunnerPiece{Present: true}}
}
func (p *fixturePlugin) Inventory(_ context.Context, r registry.InventoryRequest) (tr.RunnerInventory, error) {
	return p.inventory, p.inventoryErr
}
func (p *fixturePlugin) Command(_ context.Context, r registry.CommandRequest) (registry.CommandResponse, error) {
	p.commands = append(p.commands, r)
	return registry.CommandResponse{Argv: []string{"fixture-runner"}, Cwd: r.RunDir, Superset: p.superset}, nil
}
func (p *fixturePlugin) Parse(_ context.Context, r registry.ParseRequest) ([]tr.RunnerResult, error) {
	if r.Runner != p.inventory.Key.Runner || r.Report != "captured-report" {
		return nil, errors.New("unexpected report")
	}
	results := p.reports[p.parsed]
	var err error
	if p.parsed < len(p.parseErrors) {
		err = p.parseErrors[p.parsed]
	}
	p.parsed++
	return results, err
}
func (p *fixturePlugin) Evidence(context.Context, registry.EvidenceRequest) ([]registry.NativeEvidence, error) {
	return nil, errors.New("fixture-evidence-absent")
}

var _ registry.RunnerPlugin = (*fixturePlugin)(nil)

func pluginFixture() *fixturePlugin {
	return &fixturePlugin{inventory: tr.RunnerInventory{Key: tr.InventoryKey{Checkout: "head", Runner: "fixture", ConfigDigest: strings.Repeat("a", 64), OS: "linux", Arch: "amd64"}, Complete: true,
		Tests: []tr.InventoryTest{{ID: "a", Path: "a_test.go"}, {ID: "b", Path: "b_test.go"}}}}
}
func captured(context.Context, registry.CommandResponse) (string, error) {
	return "captured-report", nil
}

// RSP-V0-025: a compiled-in plugin uses inventory, command and parse through
// the same SPI; evidence is available only as uninterpreted native components.
func TestRSPV0025CompiledInSession(t *testing.T) {
	p := pluginFixture()
	p.reports = [][]tr.RunnerResult{{{ID: "a", Outcome: "passed"}}}
	s := registry.NewSelectionSession(p, p.inventory.Key, true)
	in := s.Inventory(context.Background(), "captured-output")
	out := s.Run(context.Background(), in, []tr.Exclusion{{ID: "b", Fresh: true}}, nil, "run-dir", true, captured)
	if !out.Succeeded || out.NeedsRequalification || len(out.Attempts) != 1 {
		t.Fatalf("%+v", out)
	}
	if len(p.commands) != 1 || p.commands[0].Purpose != "run" || !reflect.DeepEqual(p.commands[0].Tests, []string{"a"}) || p.commands[0].Key != p.inventory.Key {
		t.Fatalf("%+v", p.commands)
	}
	if _, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: "fixture"}); err == nil {
		t.Fatal("fixture fabricated evidence")
	}
}

// RSP-V0-025: registered Ruby pieces stay unqualified; other legacy plugins
// remain absent. Neither replaces the existing family Build and Parse functions.
func TestRSPV0025LegacyAndUnknownAbsent(t *testing.T) {
	ctx := context.Background()
	for _, runner := range registry.Runners() {
		p, ok := registry.SelectionPlugin(runner)
		if !ok || p.Pieces().FullReason(true) == "" {
			t.Fatalf("%s admitted", runner)
		}
		want := tr.AbsentRunnerPieces(runner)
		switch runner {
		case "rspec", "minitest", "test-unit", "rails-test":
			want = tr.RunnerPieces{Inventory: tr.RunnerPiece{Present: true}, TestEvidence: tr.RunnerPiece{Present: true}, Qualification: tr.RunnerPiece{Reason: "unqualified"}}
		}
		if !reflect.DeepEqual(p.Pieces(), want) {
			t.Fatalf("%s pieces: %+v want %+v", runner, p.Pieces(), want)
		}
		if in, err := p.Inventory(ctx, registry.InventoryRequest{}); err == nil || in.Complete || in.Reason == "" {
			t.Fatalf("%s inventory", runner)
		}
		if _, err := p.Command(ctx, registry.CommandRequest{}); err == nil {
			t.Fatalf("%s command", runner)
		}
		if _, err := p.Parse(ctx, registry.ParseRequest{}); err == nil {
			t.Fatalf("%s parse", runner)
		}
		if _, err := p.Evidence(ctx, registry.EvidenceRequest{}); err == nil {
			t.Fatalf("%s evidence", runner)
		}
		f, ok := registry.Lookup(runner)
		if !ok || f.Build == nil || f.Parse == nil {
			t.Fatalf("%s legacy path lost", runner)
		}
	}
	if _, ok := registry.SelectionPlugin("unknown-runner"); ok {
		t.Fatal("unknown plugin resolved")
	}
	p := pluginFixture()
	for _, s := range []*registry.SelectionSession{registry.NewSelectionSession(p, p.inventory.Key, false), registry.NewSelectionSession(nil, p.inventory.Key, true)} {
		plan := s.Plan(p.inventory, []tr.Exclusion{{ID: "a", Fresh: true}}, nil)
		if !plan.Full || plan.Reason == "" || len(plan.Tests) != 2 {
			t.Fatalf("%+v", plan)
		}
	}
}

// RSP-V0-002: errors beside partial discovery, wrong targets and missing
// discovery cannot become narrower plans through the compiled-in boundary.
func TestRSPV0002PluginInventoryFailures(t *testing.T) {
	for _, mode := range []string{"error", "truncated", "target"} {
		p := pluginFixture()
		key := p.inventory.Key
		switch mode {
		case "error":
			p.inventoryErr = errors.New("unparseable")
		case "truncated":
			p.inventory.Complete = false
			p.inventory.Reason = "output-cap"
		case "target":
			p.inventory.Key.Arch = "arm64"
		}
		s := registry.NewSelectionSession(p, key, true)
		in := s.Inventory(context.Background(), "captured-output")
		plan := s.Plan(in, nil, nil)
		if in.Complete || !plan.Full || plan.Reason == "" {
			t.Fatalf("%s: %+v", mode, plan)
		}
	}
}

// RSP-V0-003: a dropped identity triggers one FULL command; the next plan
// remains FULL even when the retry succeeds, until core requalification.
func TestRSPV0003SessionDivergenceRetry(t *testing.T) {
	for _, allow := range []bool{true, false} {
		p := pluginFixture()
		p.reports = [][]tr.RunnerResult{{{ID: "a", Outcome: "passed"}}, {{ID: "a", Outcome: "passed"}, {ID: "b", Outcome: "skipped", Reason: "runner skip"}}}
		s := registry.NewSelectionSession(p, p.inventory.Key, true)
		out := s.Run(context.Background(), p.inventory, nil, nil, "run-dir", allow, captured)
		if out.Succeeded != allow || !out.NeedsRequalification || !reflect.DeepEqual(out.Attempts[0].Reconciliation.Missing, []string{"b"}) {
			t.Fatalf("%+v", out)
		}
		if allow && (len(p.commands) != 2 || p.commands[1].Tests != nil) {
			t.Fatalf("retry did not run FULL: %+v", p.commands)
		}
		plan := s.Plan(p.inventory, []tr.Exclusion{{ID: "b", Fresh: true}}, nil)
		if !plan.Full || !strings.Contains(plan.Reason, "requalification-required") || len(plan.Tests) != 2 {
			t.Fatalf("%+v", plan)
		}
	}
}

// RSP-V0-003: execution and parse errors cannot erase captured results. Only
// passed, failed and reasoned skipped outcomes are terminal; unknowns widen.
func TestRSPV0003SessionExecutionErrorRetainsResults(t *testing.T) {
	for _, tc := range []struct {
		name     string
		outcome  string
		parseErr error
	}{
		{name: "failed", outcome: "failed"},
		{name: "partial-parse", outcome: "failed", parseErr: errors.New("incomplete report")},
		{name: "unknown-outcome", outcome: "future-outcome"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := pluginFixture()
			p.reports = [][]tr.RunnerResult{{{ID: "a", Outcome: tc.outcome}}, {{ID: "a", Outcome: "passed"}, {ID: "b", Outcome: "passed"}}}
			p.parseErrors = []error{tc.parseErr}
			s := registry.NewSelectionSession(p, p.inventory.Key, true)
			calls := 0
			out := s.Run(context.Background(), p.inventory, nil, nil, "run-dir", true, func(context.Context, registry.CommandResponse) (string, error) {
				calls++
				if calls == 1 {
					return "captured-report", errors.New("exit status 1")
				}
				return "captured-report", nil
			})
			if len(out.Attempts) != 2 || p.parsed != 2 || calls != 2 || !out.NeedsRequalification {
				t.Fatalf("attempts or requalification lost: %+v; parses=%d calls=%d", out, p.parsed, calls)
			}
			first, retry := out.Attempts[0], out.Attempts[1]
			if !reflect.DeepEqual(first.Results, p.reports[0]) || first.Reconciliation.Succeeded {
				t.Fatalf("captured results lost or failed execution accepted: %+v", first)
			}
			reasons := strings.Join(first.Reconciliation.Reasons, "\n")
			if !strings.Contains(reasons, "exit status 1") || (tc.parseErr != nil && !strings.Contains(reasons, tc.parseErr.Error())) {
				t.Fatalf("execution or parse error lost: %+v", first)
			}
			missing := []string{"b"}
			if tc.outcome == "future-outcome" {
				missing = []string{"a", "b"}
			}
			if !reflect.DeepEqual(first.Reconciliation.Missing, missing) || !retry.Plan.Full || p.commands[1].Tests != nil || !retry.Reconciliation.Succeeded {
				t.Fatalf("missing results did not receive a passing FULL retry: %+v", out)
			}
			if out.Succeeded != (tc.outcome == "future-outcome") {
				t.Fatalf("reported failure did not stand across retry: %+v", out)
			}
			if plan := s.Plan(p.inventory, []tr.Exclusion{{ID: "b", Fresh: true}}, nil); !plan.Full {
				t.Fatalf("divergence allowed subsequent narrowing: %+v", plan)
			}
		})
	}
}

// RSP-V0-003: a result absent from inventory disables selection without
// discarding that result or silently declaring a successful invocation.
func TestRSPV0003SessionExtraResult(t *testing.T) {
	p := pluginFixture()
	p.superset = true
	p.reports = [][]tr.RunnerResult{{{ID: "a", Outcome: "passed"}, {ID: "b", Outcome: "passed"}, {ID: "extra", Outcome: "passed"}}}
	s := registry.NewSelectionSession(p, p.inventory.Key, true)
	out := s.Run(context.Background(), p.inventory, nil, nil, "run-dir", true, captured)
	if out.Succeeded || !out.NeedsRequalification || len(out.Attempts[0].Results) != 3 || !reflect.DeepEqual(out.Attempts[0].Reconciliation.Unexpected, []string{"extra"}) {
		t.Fatalf("%+v", out)
	}
	if plan := s.Plan(p.inventory, []tr.Exclusion{{ID: "b", Fresh: true}}, nil); !plan.Full {
		t.Fatal("divergent runner narrowed")
	}
}

// RSP-V0-004: a native filter unable to express the exact set reports its
// superset, which is still checked for every required test identity.
func TestRSPV0004SessionSuperset(t *testing.T) {
	p := pluginFixture()
	p.superset = true
	p.reports = [][]tr.RunnerResult{{{ID: "a", Outcome: "passed"}, {ID: "b", Outcome: "passed"}}}
	s := registry.NewSelectionSession(p, p.inventory.Key, true)
	out := s.Run(context.Background(), p.inventory, []tr.Exclusion{{ID: "b", Fresh: true}}, nil, "run-dir", false, captured)
	if !out.Succeeded || !out.Attempts[0].Superset || !reflect.DeepEqual(p.commands[0].Tests, []string{"a"}) {
		t.Fatalf("%+v", out)
	}
	p = pluginFixture()
	p.superset = true
	p.reports = [][]tr.RunnerResult{{{ID: "b", Outcome: "passed"}}}
	s = registry.NewSelectionSession(p, p.inventory.Key, true)
	out = s.Run(context.Background(), p.inventory, []tr.Exclusion{{ID: "b", Fresh: true}}, nil, "run-dir", false, captured)
	if out.Succeeded || !reflect.DeepEqual(out.Attempts[0].Reconciliation.Missing, []string{"a"}) {
		t.Fatalf("subset passed: %+v", out)
	}
}
