// SPDX-License-Identifier: Apache-2.0
package sessiontest

import (
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testrunner/contract"
	"github.com/Beamfall/corvint/internal/testrunner/killswitch"
)

// unitKeys lists a manifest's units as "<adapter> <unit> [<slice>]".
func contractUnitKeys(m Manifest) []string {
	keys := make([]string, 0, len(m.Units))
	for _, unit := range m.Units {
		keys = append(keys, unit.Adapter+" "+unit.Unit+" ["+unit.Slice+"]")
	}
	return keys
}

func contractInput() Input {
	return Input{
		Declared: BuiltInDeclared(), ShardCount: 1, GoSelection: "NOTHING no Go universe",
		Selected:      []string{"typescript:jest:src/a.test.js", "typescript:playwright:e2e/login.spec.ts", "python:app.calc", "ruby:rspec:spec/a_spec"},
		SelectedTests: map[string][]string{"python:app.calc": {"services/a/tests/test_calc.py"}},
		Instances: []contract.Instance{
			{Runner: "jest", Root: ".", Config: "jest.config.js"},
			{Runner: "playwright", Root: ".", Config: "playwright.config.ts"},
			{Runner: "pytest", Root: "services/a", Config: "services/a/pytest.ini"},
			{Runner: "rspec", Root: ".", Config: ".rspec"},
		},
	}
}

// RNC-V1-019, RNC-V1-004, RNC-V1-014, RNC-V1-007, PYR-V1-010: each
// selected Vitest, Jest, Playwright, pytest or RSpec file becomes one
// contract unit in its owning instance, a `python:<module>` unit through the
// plan's own test files; the contract derives each instance's argv from its
// full command, run from the instance root, with the report placeholder.
func TestRNCV1019ContractUnitsPlanTheSelectedFiles(t *testing.T) {
	input := contractInput()
	m, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"contract jest:src/a.test.js [jest@.]",
		"contract playwright:e2e/login.spec.ts [playwright@.]",
		"contract pytest:services/a/tests/test_calc.py [pytest@services/a]",
		"contract rspec:spec/a_spec.rb [rspec@.]",
	}
	if m.Scope != ScopeNarrowed || !slices.Equal(contractUnitKeys(m), want) || len(m.Incomplete) != 0 {
		t.Fatalf("scope %s units %q incomplete %q", m.Scope, contractUnitKeys(m), m.Incomplete)
	}
	for _, unit := range m.Units {
		wantKind := ""
		if strings.HasPrefix(unit.Unit, "playwright:") {
			wantKind = contract.KindE2E
		}
		if unit.Kind != wantKind || (wantKind != "") != (unit.KindSource == contract.KindSourceRunner) {
			t.Fatalf("RNC-V1-007 kind of %s: %q %q", unit.Unit, unit.Kind, unit.KindSource)
		}
	}
	planned := Invocations(m, input.Declared, 0, t.TempDir(), "")
	if len(planned) != 4 {
		t.Fatalf("invocations %#v", planned)
	}
	jest, pytest := planned[0], planned[2]
	if jest.Adapter != AdapterContract || jest.Dir != "" ||
		!slices.Equal(jest.Argv, []string{"npx", "jest", "--json", "--outputFile=" + ContractReportPlaceholder, "--runTestsByPath", "src/a.test.js"}) {
		t.Fatalf("jest invocation %#v", jest)
	}
	if pytest.Dir != "services/a" || !slices.Equal(pytest.Argv, []string{"python", "-m", "pytest", "--json-report", "--json-report-file=" + ContractReportPlaceholder, "tests/test_calc.py"}) {
		t.Fatalf("pytest invocation %#v", pytest)
	}
	if runnerKey(pytest) != "contract ./services/a python" || runnerKey(jest) != "contract npx" {
		t.Fatalf("runner keys %q %q", runnerKey(pytest), runnerKey(jest))
	}
	instance, command, refusal, ok := jest.ContractSelection()
	if !ok || instance != "jest@." || refusal != "" || command.Tier != contract.TierFile || !command.Exact {
		t.Fatalf("jest selection %q %+v %q %v", instance, command, refusal, ok)
	}
	if _, _, _, ok := (Planned{Adapter: AdapterGo}).ContractSelection(); ok {
		t.Fatal("a go invocation reports a contract selection")
	}
}

// RNC-V1-019: whatever the contract cannot narrow keeps its full command:
// Minitest (RNC-V1-004, runner-not-implemented), a language the declaration
// covers with one command (RNC-V1-013), the ci profile (STR-V0-012), an
// instance whose inventory verdict is not current (RNC-V1-002), a python
// module the plan lists no test file for, and a file no instance owns. A
// file two instances own runs in both (RNC-V1-014).
func TestRNCV1019WhatTheContractDoesNotNarrow(t *testing.T) {
	build := func(t *testing.T, input Input) Manifest {
		t.Helper()
		m, err := Build(input)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	t.Run("minitest runs its full command", func(t *testing.T) {
		input := contractInput()
		input.Selected = []string{"ruby:minitest:test/a_test"}
		input.Instances = []contract.Instance{{Runner: "minitest", Root: ".", Config: "Rakefile"}}
		if got := contractUnitKeys(build(t, input)); !slices.Equal(got, []string{"declared declared:minitest@. [whole]"}) {
			t.Fatalf("units %q", got)
		}
	})
	t.Run("a declared language command covers its runners", func(t *testing.T) {
		input := contractInput()
		input.Declared.Languages = []LanguageCommand{{Language: "python", Argv: []string{"make", "py"}}}
		got := contractUnitKeys(build(t, input))
		if !slices.Contains(got, "declared declared:python [whole]") || slices.ContainsFunc(got, func(key string) bool { return strings.Contains(key, "pytest:") }) {
			t.Fatalf("units %q", got)
		}
	})
	t.Run("the ci profile runs no contract unit", func(t *testing.T) {
		input := contractInput()
		input.RunnerProfile = ProfileCI
		m := build(t, input)
		if slices.ContainsFunc(m.Units, func(unit Unit) bool { return unit.Adapter == AdapterContract }) || !slices.Contains(m.Checks, "jest@. full command") {
			t.Fatalf("units %q checks %q", contractUnitKeys(m), m.Checks)
		}
	})
	t.Run("an instance without a current inventory runs its full command", func(t *testing.T) {
		input := contractInput()
		input.FullInstances = map[string]string{"jest@.": "inventory-absent"}
		m := build(t, input)
		got := contractUnitKeys(m)
		if !slices.Contains(got, "declared declared:jest@. [whole]") || slices.Contains(got, "contract jest:src/a.test.js [jest@.]") || !slices.Contains(got, "contract rspec:spec/a_spec.rb [rspec@.]") {
			t.Fatalf("units %q", got)
		}
		for _, unit := range m.Units {
			if unit.Unit == "declared:jest@." && unit.Reason != "inventory-absent" {
				t.Fatalf("reason %q", unit.Reason)
			}
		}
	})
	t.Run("a module with no listed test file runs the language", func(t *testing.T) {
		input := contractInput()
		input.SelectedTests = nil
		got := contractUnitKeys(build(t, input))
		if !slices.Contains(got, "declared declared:pytest@services/a [whole]") || slices.ContainsFunc(got, func(key string) bool { return strings.HasPrefix(key, "contract pytest:") }) {
			t.Fatalf("units %q", got)
		}
	})
	t.Run("a file no instance owns runs the language", func(t *testing.T) {
		input := contractInput()
		input.Selected = []string{"typescript:jest:lib/x.test.js"}
		input.Instances = []contract.Instance{{Runner: "jest", Root: "packages/web", Config: "packages/web/jest.config.js"}}
		if got := contractUnitKeys(build(t, input)); !slices.Equal(got, []string{"declared declared:jest@packages/web [whole]"}) {
			t.Fatalf("units %q", got)
		}
	})
	t.Run("a file two instances own runs in both", func(t *testing.T) {
		input := contractInput()
		input.Selected = []string{"typescript:jest:packages/web/x.test.js"}
		input.Instances = []contract.Instance{{Runner: "jest", Root: ".", Config: "jest.config.js"}, {Runner: "jest", Root: "packages/web", Config: "packages/web/jest.config.js"}}
		want := []string{"contract jest:packages/web/x.test.js [jest@.]", "contract jest:packages/web/x.test.js [jest@packages/web]"}
		if got := contractUnitKeys(build(t, input)); !slices.Equal(got, want) {
			t.Fatalf("units %q", got)
		}
	})
	t.Run("FULL runs each instance's full command, not its files", func(t *testing.T) {
		input := contractInput()
		input.GoSelection = "FALLBACK forced"
		m := build(t, input)
		if m.Scope != ScopeFull || slices.ContainsFunc(m.Units, func(unit Unit) bool { return unit.Adapter == AdapterContract }) {
			t.Fatalf("scope %s units %q", m.Scope, contractUnitKeys(m))
		}
	})
}

// RNC-V1-008: the kill switch's `full` makes the plan FULL with its reason;
// `full:<runner>` runs every instance of each named runner in full, selected
// or not, and leaves the other runners narrowed; Go switched runs its whole
// universe. The manifest states the switch with its raw value.
func TestRNCV1008KillSwitchPlans(t *testing.T) {
	parse := func(t *testing.T, value string) killswitch.Switch {
		t.Helper()
		parsed, err := killswitch.Parse(value)
		if err != nil {
			t.Fatal(err)
		}
		return parsed
	}
	t.Run("full", func(t *testing.T) {
		input := contractInput()
		input.KillSwitch = parse(t, "full")
		m, err := Build(input)
		if err != nil || m.Scope != ScopeFull || m.Reason != "kill-switch: CORVINT_SELECTION=full makes every runner run its full command" || m.KillSwitch != m.Reason {
			t.Fatalf("%v scope %s reason %q switch %q", err, m.Scope, m.Reason, m.KillSwitch)
		}
	})
	t.Run("full:jest", func(t *testing.T) {
		input := contractInput()
		input.Selected = []string{"python:app.calc"}
		input.Instances = append(input.Instances, contract.Instance{Runner: "jest", Root: "packages/web", Config: "packages/web/jest.config.js"})
		input.KillSwitch = parse(t, "full:jest")
		m, err := Build(input)
		want := []string{"contract pytest:services/a/tests/test_calc.py [pytest@services/a]", "declared declared:jest@. [whole]", "declared declared:jest@packages/web [whole]"}
		got := contractUnitKeys(m)
		slices.Sort(got)
		if err != nil || m.Scope != ScopeNarrowed || !slices.Equal(got, want) || !strings.Contains(m.KillSwitch, "CORVINT_SELECTION=full:jest") {
			t.Fatalf("%v scope %s units %q switch %q", err, m.Scope, got, m.KillSwitch)
		}
		for _, unit := range m.Units {
			if unit.Adapter == AdapterDeclared && !strings.HasPrefix(unit.Reason, "kill-switch: ") {
				t.Fatalf("unit %+v has no kill-switch reason", unit)
			}
		}
	})
	t.Run("full:go", func(t *testing.T) {
		input := Input{Declared: BuiltInDeclared(), ShardCount: 1, GoSelection: "run m/a", Universe: []string{"m/a", "m/b"}}
		m, err := Build(input)
		if err != nil || !slices.Equal(contractUnitKeys(m), []string{"go go:m/a [whole]"}) {
			t.Fatalf("narrowed: %v %q", err, contractUnitKeys(m))
		}
		input.KillSwitch = parse(t, "full:go")
		m, err = Build(input)
		if err != nil || m.Scope != ScopeNarrowed || !slices.Equal(contractUnitKeys(m), []string{"go go:m/a [whole]", "go go:m/b [whole]"}) {
			t.Fatalf("switched: %v %s %q", err, m.Scope, contractUnitKeys(m))
		}
	})
	t.Run("a switched runner runs even when nothing is selected", func(t *testing.T) {
		input := contractInput()
		input.Selected = nil
		input.KillSwitch = parse(t, "full:rspec")
		m, err := Build(input)
		if err != nil || m.Scope != ScopeNarrowed || !slices.Equal(contractUnitKeys(m), []string{"declared declared:rspec@. [whole]"}) {
			t.Fatalf("%v %s %q", err, m.Scope, contractUnitKeys(m))
		}
	})
}
