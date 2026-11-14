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

// RNC-V1-019, RNC-V1-004, RNC-V1-014, RNC-V1-007: each
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
	// JST-V1-008: Jest narrows only in a setup it can read, here Jest 29
	// with no config file.
	root := t.TempDir()
	writeJestInstall(t, root, "29.7.0")
	planned := Invocations(m, input.Declared, 0, root, "")
	if len(planned) != 4 {
		t.Fatalf("invocations %#v", planned)
	}
	jest, pytest := planned[0], planned[2]
	if jest.Adapter != AdapterContract || jest.Dir != "" ||
		!slices.Equal(jest.Argv, []string{"npx", "jest", "--json", "--outputFile=" + ContractReportPlaceholder, "--runTestsByPath", "src/a.test.js"}) {
		t.Fatalf("jest invocation %#v", jest)
	}
	if pytest.Dir != "services/a" || !slices.Equal(pytest.Argv, append(append([]string{"python", "-m", "pytest"}, contract.PytestReportArgs(ContractReportPlaceholder)...), "tests/test_calc.py")) {
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

// RNC-V1-006: the manifest carries, per contract unit, the tests the
// inventory lists in its file, and only those: not the file-level ID, not
// another file's tests; the invocation must answer each one.
func TestRNCV1006ContractUnitsCarryListedTests(t *testing.T) {
	input := contractInput()
	input.ListedTests = map[string]map[string][]string{"pytest@services/a": {
		"services/a/tests/test_calc.py":  {"pytest:services/a/tests/test_calc.py::test_sub", "pytest:services/a/tests/test_calc.py", "pytest:services/a/tests/test_calc.py::test_add"},
		"services/a/tests/test_other.py": {"pytest:services/a/tests/test_other.py::test_x"},
	}}
	m, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Listed) != 1 || m.Listed[0].Slice != "pytest@services/a" || m.Listed[0].Unit != "pytest:services/a/tests/test_calc.py" ||
		!slices.Equal(m.Listed[0].Tests, []string{"pytest:services/a/tests/test_calc.py::test_add", "pytest:services/a/tests/test_calc.py::test_sub"}) {
		t.Fatalf("listed %+v", m.Listed)
	}
	planned := Invocations(m, input.Declared, 0, t.TempDir(), "")
	for _, invocation := range planned {
		if invocation.Units[0].Slice != "pytest@services/a" {
			continue
		}
		if !slices.Equal(invocation.Argv[len(invocation.Argv)-1:], []string{"tests/test_calc.py"}) ||
			!slices.Equal(invocation.contract.expected, []string{"pytest:services/a/tests/test_calc.py", "pytest:services/a/tests/test_calc.py::test_add", "pytest:services/a/tests/test_calc.py::test_sub"}) {
			t.Fatalf("argv %q expected %q", invocation.Argv, invocation.contract.expected)
		}
		return
	}
	t.Fatalf("no pytest invocation in %+v", planned)
}

// RNC-V1-019, TIV-V1-006, TIV-V1-007: an instance whose inventory record is
// absent or stale runs its full command with that reason even when the
// change selects no file of it; the instance whose files are selected
// stays narrowed, and a plan selecting nothing else is not NOTHING.
func TestRNCV1019InventoryFallbackRunsUnselectedInstances(t *testing.T) {
	input := contractInput()
	input.Selected = []string{"typescript:jest:packages/a/x.test.js"}
	input.Instances = []contract.Instance{
		{Runner: "jest", Root: "packages/a", Config: "packages/a/jest.config.js"},
		{Runner: "jest", Root: "packages/b", Config: "packages/b/jest.config.js"},
	}
	input.FullInstances = map[string]string{"jest@packages/b": "inventory-absent: the inventory has no record for this instance"}
	m, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"contract jest:packages/a/x.test.js [jest@packages/a]", "declared declared:jest@packages/b [whole]"}
	if m.Scope != ScopeNarrowed || !slices.Equal(contractUnitKeys(m), want) {
		t.Fatalf("scope %s units %q", m.Scope, contractUnitKeys(m))
	}
	if m.Units[1].Reason != "inventory-absent: the inventory has no record for this instance" {
		t.Fatalf("reason %q", m.Units[1].Reason)
	}
	input.Selected = nil
	if m, err = Build(input); err != nil || m.Scope != ScopeNarrowed || !slices.Equal(contractUnitKeys(m), want[1:]) {
		t.Fatalf("nothing selected: %v scope %s units %q", err, m.Scope, contractUnitKeys(m))
	}
	input.RunnerProfile = ProfileCI
	if m, err = Build(input); err != nil || m.Scope != ScopeNothing || len(m.Units) != 0 {
		t.Fatalf("ci profile: %v scope %s units %q", err, m.Scope, contractUnitKeys(m))
	}
}

// RNC-V1-008: the Go kill switch runs every Go instance, so an unchanged
// nested go.mod module runs its own go test ./... beside the root universe
// under `full:go` and under `full`; the ci profile names it as a check.
func TestRNCV1008GoKillSwitchRunsNestedModules(t *testing.T) {
	for _, value := range []string{"full:go", "full"} {
		t.Run(value, func(t *testing.T) {
			parsed, err := killswitch.Parse(value)
			if err != nil {
				t.Fatal(err)
			}
			input := Input{
				Declared: BuiltInDeclared(), ShardCount: 1, GoSelection: "run m/a", Universe: []string{"m/a", "m/b"}, KillSwitch: parsed,
				Instances: []contract.Instance{{Runner: "go", Root: ".", Config: "go.mod"}, {Runner: "go", Root: "tools/x", Config: "tools/x/go.mod"}},
			}
			m, err := Build(input)
			want := []string{"go go:m/a [whole]", "go go:m/b [whole]", "go go-module:tools/x [whole]"}
			if err != nil || !slices.Equal(contractUnitKeys(m), want) {
				t.Fatalf("%v units %q", err, contractUnitKeys(m))
			}
			if unit := m.Units[2]; unit.Dir != "tools/x" {
				t.Fatalf("module unit %+v", unit)
			}
			input.RunnerProfile = ProfileCI
			m, err = Build(input)
			if err != nil || slices.Contains(contractUnitKeys(m), "go go-module:tools/x [whole]") || !slices.Contains(m.Checks, ModuleCheck("tools/x")) {
				t.Fatalf("ci: %v units %q checks %q", err, contractUnitKeys(m), m.Checks)
			}
		})
	}
}
