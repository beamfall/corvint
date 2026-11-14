// SPDX-License-Identifier: Apache-2.0
package sessiontest

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testrunner/contract"
	"github.com/Beamfall/corvint/internal/testrunner/killswitch"
)

// pytestInput is a narrowed plan with one Go package and two Python units,
// whose test files lie in two pytest rootdirs.
func pytestInput() Input {
	return Input{
		Declared: BuiltInDeclared(), ShardCount: 1, GoSelection: "run m/a", Universe: []string{"m/a"},
		Selected: []string{"go:m/a", "python:tests.test_a", "python:test_b"},
		SelectedTests: map[string][]string{
			"python:tests.test_a": {"services/a/tests/test_a.py"},
			"python:test_b":       {"services/b/test_b.py", "services/b/test_c.py"},
		},
		Instances: []contract.Instance{
			{Runner: "go", Root: ".", Config: "go.mod"},
			{Runner: "pytest", Root: "services/a", Config: "services/a/pyproject.toml"},
			{Runner: "pytest", Root: "services/b", Config: "services/b/pytest.ini"},
		},
	}
}

// PYR-V1-002, PYR-V1-003: the plan's pytest files run through the contract
// adapter, one invocation per rootdir from its root, at tier file; the argv
// is the instance's full command plus pytest's own JUnit report options and
// the selected files, and no pytest full command runs.
func TestPYRV1002PytestFilesRunAtTierFile(t *testing.T) {
	input := pytestInput()
	m, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"go go:m/a [whole]", "contract pytest:services/a/tests/test_a.py [pytest@services/a]", "contract pytest:services/b/test_b.py [pytest@services/b]", "contract pytest:services/b/test_c.py [pytest@services/b]"}
	if got := contractUnitKeys(m); m.Scope != ScopeNarrowed || !slices.Equal(got, want) {
		t.Fatalf("scope %s units %q, want %q", m.Scope, got, want)
	}
	planned := Invocations(m, input.Declared, 0, t.TempDir(), "")
	var runs []Planned
	for _, invocation := range planned {
		if invocation.Adapter == AdapterContract {
			runs = append(runs, invocation)
		}
	}
	if len(runs) != 2 {
		t.Fatalf("contract invocations %#v", planned)
	}
	report := contract.PytestReportArgs(ContractReportPlaceholder)
	for index, test := range []struct {
		dir, instance string
		files         []string
	}{{"services/a", "pytest@services/a", []string{"tests/test_a.py"}}, {"services/b", "pytest@services/b", []string{"test_b.py", "test_c.py"}}} {
		run := runs[index]
		wantArgv := append(append([]string{"python", "-m", "pytest"}, report...), test.files...)
		instance, command, refusal, ok := run.ContractSelection()
		if !ok || refusal != "" || instance != test.instance || command.Tier != contract.TierFile || run.Dir != test.dir || !slices.Equal(run.Argv, wantArgv) {
			t.Fatalf("invocation %d: %s %+v %q dir %q argv %q, want %q", index, instance, command, refusal, run.Dir, run.Argv, wantArgv)
		}
	}
	if runnerKey(runs[0]) == runnerKey(runs[1]) {
		t.Fatalf("two rootdirs share the runner key %q", runnerKey(runs[0]))
	}
	// A declared instance command is the full command the selection extends.
	input.Declared.Instances = []RunnerCommand{{Key: "pytest@services/a", Argv: []string{"uv", "run", "pytest"}}}
	if m, err = Build(input); err != nil {
		t.Fatal(err)
	}
	for _, invocation := range Invocations(m, input.Declared, 0, t.TempDir(), "") {
		if invocation.Dir == "services/a" && !slices.Equal(invocation.Argv[:3], []string{"uv", "run", "pytest"}) {
			t.Fatalf("declared instance argv %q", invocation.Argv)
		}
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
	if got := contractUnitKeys(m); m.Scope != ScopeNarrowed || !slices.Equal(got, []string{"contract pytest:services/a/tests/test_a.py [pytest@services/a]"}) {
		t.Fatalf("scope %s units %q", m.Scope, got)
	}
}

// PYR-V1-009: a selection that cannot go through the contract runs its
// language full command instead: a declared python full command, or the ci
// profile, which runs no contract unit.
func TestPYRV1009FallbacksNameTheirReason(t *testing.T) {
	input := pytestInput()
	input.Declared.Languages = []LanguageCommand{{Language: "python", Argv: []string{"make", "py"}}}
	m, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if got := contractUnitKeys(m); !slices.Contains(got, "declared declared:python [whole]") || slices.ContainsFunc(got, func(key string) bool { return strings.HasPrefix(key, "contract ") }) {
		t.Fatalf("declared python units %q", got)
	}
	input = pytestInput()
	input.RunnerProfile = ProfileCI
	if m, err = Build(input); err != nil {
		t.Fatal(err)
	}
	for _, unit := range m.Units {
		if unit.Adapter == AdapterContract {
			t.Fatalf("the ci profile ran a contract unit: %q", contractUnitKeys(m))
		}
	}
}

// PYR-V1-009: CORVINT_SELECTION=full:pytest runs every pytest instance's
// full command with reason kill-switch while Go stays narrowed; `full` runs
// every test; another runner's switch leaves pytest narrowed.
func TestPYRV1009KillSwitch(t *testing.T) {
	parse := func(value string) killswitch.Switch {
		parsed, err := killswitch.Parse(value)
		if err != nil {
			t.Fatal(err)
		}
		return parsed
	}
	input := pytestInput()
	input.KillSwitch = parse("full:pytest")
	m, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"declared declared:pytest@services/a [whole]", "declared declared:pytest@services/b [whole]", "go go:m/a [whole]"}
	got := contractUnitKeys(m)
	slices.Sort(got)
	if m.Scope != ScopeNarrowed || !slices.Equal(got, want) {
		t.Fatalf("full:pytest scope %s units %q, want %q", m.Scope, got, want)
	}
	for _, unit := range m.Units {
		if unit.Adapter == AdapterDeclared && !strings.HasPrefix(unit.Reason, "kill-switch: ") {
			t.Fatalf("forced unit %+v has no kill-switch reason", unit)
		}
		if unit.Adapter == AdapterGo && unit.Reason != "" {
			t.Fatalf("the Go unit took the pytest switch: %+v", unit)
		}
	}
	input.KillSwitch = parse("full")
	if m, err = Build(input); err != nil || m.Scope != ScopeFull || !strings.HasPrefix(m.Reason, "kill-switch: ") {
		t.Fatalf("full: %v scope %s reason %q", err, m.Scope, m.Reason)
	}
	input.KillSwitch = parse("full:jest")
	if m, err = Build(input); err != nil || m.Scope != ScopeNarrowed || !slices.Contains(contractUnitKeys(m), "contract pytest:services/b/test_b.py [pytest@services/b]") {
		t.Fatalf("full:jest: %v units %q", err, contractUnitKeys(m))
	}
}

// writeJestInstall writes root/node_modules/jest/package.json naming
// version, the install JST-V1-008 reads before a Jest selection narrows.
func writeJestInstall(t *testing.T, root, version string) {
	t.Helper()
	dir := filepath.Join(root, "node_modules", "jest")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"jest","version":"`+version+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
}
