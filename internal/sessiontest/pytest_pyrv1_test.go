// SPDX-License-Identifier: Apache-2.0
package sessiontest

import (
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/selectionswitch"
	"github.com/Beamfall/corvint/internal/testrunner/contract"
)

// pytestInput is a narrowed plan with one Go package and two Python test
// units, whose test files lie in two pytest rootdirs.
func pytestInput() Input {
	return Input{
		Declared: BuiltInDeclared(), ShardCount: 1, GoSelection: "run m/a", Universe: []string{"m/a"},
		Selected: []string{"go:m/a", "python:tests.test_a", "python:test_b"},
		ContractTests: map[string][]string{
			"python:tests.test_a": {"pytest:services/a/tests/test_a.py"},
			"python:test_b":       {"pytest:services/b/test_b.py", "pytest:services/b/test_c.py"},
		},
		Instances: []contract.Instance{
			{Runner: "go", Root: ".", Config: "go.mod"},
			{Runner: "pytest", Root: ".", Config: "pyproject.toml"},
			{Runner: "pytest", Root: "services/a", Config: "services/a/pyproject.toml"},
			{Runner: "pytest", Root: "services/b", Config: "services/b/pytest.ini"},
		},
	}
}

func unitsOf(m Manifest) []string {
	var units []string
	for _, unit := range m.Units {
		units = append(units, unit.Adapter+" "+unit.Unit)
	}
	return units
}

// PYR-V1-002, PYR-V1-003: the plan's pytest files run through the runner
// adapter at tier file, each unit labelled with the tier and its reason and
// owned by the deepest rootdir; one invocation per rootdir runs the
// instance's full command with the selected files from its root, and no
// pytest full command runs.
func TestPYRV1002PytestFilesRunAtTierFile(t *testing.T) {
	input := pytestInput()
	m, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"go go:m/a", "runner pytest:services/a/tests/test_a.py", "runner pytest:services/b/test_b.py", "runner pytest:services/b/test_c.py"}
	if got := unitsOf(m); m.Scope != ScopeNarrowed || !slices.Equal(got, want) {
		t.Fatalf("scope %s units %q, want %q", m.Scope, got, want)
	}
	for _, unit := range m.Units[1:] {
		if unit.Tier != contract.TierFile || unit.ReasonCode != ReasonTierFile || !strings.HasPrefix(unit.Reason, "tier file: ") || unit.Slice != SliceWhole {
			t.Fatalf("runner unit %+v carries no tier label", unit)
		}
	}
	if m.Units[1].Instance != "pytest@services/a" || m.Units[2].Instance != "pytest@services/b" {
		t.Fatalf("instances %+v", m.Units)
	}
	planned := Invocations(m, input.Declared, 0, t.TempDir(), "")
	if len(planned) != 3 {
		t.Fatalf("invocations %#v", planned)
	}
	report := []string{"--rootdir=.", "--junitxml=" + RunnerReportPlaceholder, "-o", "junit_family=xunit1", "-o", "junit_duration_report=call"}
	a, b := planned[1], planned[2]
	if a.Adapter != AdapterRunner || a.Dir != "services/a" || a.Instance != "pytest@services/a" || a.Tier != contract.TierFile ||
		!slices.Equal(a.Argv, append(append([]string{"python", "-m", "pytest"}, report...), "tests/test_a.py")) {
		t.Fatalf("instance a invocation %#v", a)
	}
	if b.Dir != "services/b" || !slices.Equal(b.Argv, append(append([]string{"python", "-m", "pytest"}, report...), "test_b.py", "test_c.py")) ||
		!slices.Equal(b.Full, []string{"python", "-m", "pytest"}) || len(b.Units) != 2 {
		t.Fatalf("instance b invocation %#v", b)
	}
	if runnerKey(a) == runnerKey(b) {
		t.Fatalf("two rootdirs share the runner key %q", runnerKey(a))
	}
	// A declared instance command is the full command the selection extends.
	input.Declared.Instances = []RunnerCommand{{Key: "pytest@services/a", Argv: []string{"uv", "run", "pytest"}}}
	if m, err = Build(input); err != nil {
		t.Fatal(err)
	}
	if planned := Invocations(m, input.Declared, 0, t.TempDir(), ""); !slices.Equal(planned[1].Argv[:3], []string{"uv", "run", "pytest"}) {
		t.Fatalf("declared instance argv %q", planned[1].Argv)
	}
}

// PYR-V1-009: a selection that cannot go through the runner runs its
// language full command, and that unit names why: a declared python full
// command, or a file no pytest instance owns. The ci profile, which runs
// only Go, keeps the declared command as a check, as before.
func TestPYRV1009FallbacksNameTheirReason(t *testing.T) {
	input := pytestInput()
	input.Declared.Languages = []LanguageCommand{{Language: "python", Argv: []string{"make", "py"}}}
	m, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if got := unitsOf(m); !slices.Equal(got, []string{"go go:m/a", "declared declared:python"}) {
		t.Fatalf("declared python units %q", got)
	}
	if unit := m.Units[1]; unit.ReasonCode != ReasonRunnerFallback || !strings.Contains(unit.Reason, "declares a python full command") {
		t.Fatalf("declared python unit %+v", unit)
	}

	// Only services/a is a pytest rootdir, so no instance owns
	// services/b/test_b.py; the selection's language full command runs,
	// which also covers services/a.
	input = pytestInput()
	input.Instances = []contract.Instance{{Runner: "go", Root: ".", Config: "go.mod"}, {Runner: "pytest", Root: "services/a", Config: "services/a/pyproject.toml"}}
	if m, err = Build(input); err != nil {
		t.Fatal(err)
	}
	if got := unitsOf(m); !slices.Equal(got, []string{"go go:m/a", "declared declared:pytest@services/a"}) {
		t.Fatalf("unowned file units %q", got)
	}
	if unit := m.Units[1]; unit.ReasonCode != ReasonRunnerFallback || !strings.Contains(unit.Reason, "no declared pytest instance owns services/b/test_b.py") {
		t.Fatalf("unowned file unit %+v", unit)
	}

	input = pytestInput()
	input.RunnerProfile = ProfileCI
	if m, err = Build(input); err != nil {
		t.Fatal(err)
	}
	for _, unit := range m.Units {
		if unit.Adapter == AdapterRunner {
			t.Fatalf("the ci profile ran a runner unit: %q", unitsOf(m))
		}
	}
}

// PYR-V1-009: CORVINT_SELECTION=full:pytest runs every pytest instance's
// full suite with reason kill-switch while Go stays narrowed; `full` runs
// every test.
func TestPYRV1009KillSwitch(t *testing.T) {
	input := pytestInput()
	selection, err := selectionswitch.Parse("full:pytest")
	if err != nil {
		t.Fatal(err)
	}
	input.Selection = selection
	m, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"go go:m/a", "declared declared:pytest@.", "declared declared:pytest@services/a", "declared declared:pytest@services/b"}
	if got := unitsOf(m); m.Scope != ScopeNarrowed || !slices.Equal(got, want) {
		t.Fatalf("full:pytest scope %s units %q, want %q", m.Scope, got, want)
	}
	for _, unit := range m.Units[1:] {
		if unit.ReasonCode != ReasonKillSwitch || unit.Reason != "kill-switch: CORVINT_SELECTION=full:pytest forces every pytest instance to run its full suite" {
			t.Fatalf("forced unit %+v", unit)
		}
	}
	if m.Units[0].Reason != "" {
		t.Fatalf("the Go unit took the pytest switch: %+v", m.Units[0])
	}

	input.Selection, _ = selectionswitch.Parse("full")
	if m, err = Build(input); err != nil {
		t.Fatal(err)
	}
	if m.Scope != ScopeFull || m.Reason != "kill-switch: CORVINT_SELECTION=full forces every runner's full suite" {
		t.Fatalf("full: scope %s reason %q", m.Scope, m.Reason)
	}
	for _, unit := range m.Units {
		if unit.Adapter == AdapterRunner {
			t.Fatalf("a FULL plan kept a runner unit: %q", unitsOf(m))
		}
	}

	// A forced Go runs its whole universe with the reason; pytest stays
	// narrowed.
	input.Universe = []string{"m/a", "m/b"}
	input.Selection, _ = selectionswitch.Parse("full:go")
	if m, err = Build(input); err != nil {
		t.Fatal(err)
	}
	var goUnits []string
	for _, unit := range m.Units {
		if unit.Adapter == AdapterGo {
			goUnits = append(goUnits, unit.Unit)
			if unit.ReasonCode != ReasonKillSwitch || unit.Slice != SliceWhole {
				t.Fatalf("forced Go unit %+v", unit)
			}
		}
	}
	if !slices.Equal(goUnits, []string{"go:m/a", "go:m/b"}) || !slices.Contains(unitsOf(m), "runner pytest:services/b/test_b.py") {
		t.Fatalf("full:go units %q", unitsOf(m))
	}

	// Another runner's switch leaves pytest narrowed.
	input.Selection, _ = selectionswitch.Parse("full:jest")
	if m, err = Build(input); err != nil {
		t.Fatal(err)
	}
	if got := unitsOf(m); m.Scope != ScopeNarrowed || !slices.Contains(got, "runner pytest:services/b/test_b.py") {
		t.Fatalf("full:jest units %q", got)
	}
}

// PYR-V1-003: a narrowed plan that selects only pytest files is narrowed,
// not NOTHING.
func TestPYRV1003PytestOnlyPlanIsNarrowed(t *testing.T) {
	input := pytestInput()
	input.GoSelection, input.Selected = "NOTHING no Go change", []string{"python:tests.test_a"}
	m, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if got := unitsOf(m); m.Scope != ScopeNarrowed || !slices.Equal(got, []string{"runner pytest:services/a/tests/test_a.py"}) {
		t.Fatalf("scope %s units %q", m.Scope, got)
	}
}
