package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// pytestAdviceRepository commits a Python repository from files and then
// rewrites edited, so `corvint affected` plans the dirty source.
func pytestAdviceRepository(t *testing.T, files map[string]string, edited, body string) string {
	t.Helper()
	root := t.TempDir()
	writePytestFixture(t, root, files)
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"},
		{"config", "maintenance.auto", "false"}, {"config", "gc.auto", "0"},
		{"add", "."}, {"commit", "-qm", "fixture"},
	} {
		affectedGit(t, root, arguments...)
	}
	writePytestFixture(t, root, map[string]string{edited: body})
	return root
}

func writePytestFixture(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// pytestCheck runs `corvint affected` and returns its one advisory pytest
// check, failing when there is none or a pytest unknown line accompanies it.
func pytestCheck(t *testing.T, root string) map[string]any {
	t.Helper()
	receipt, _, stderr, code := runAffectedCLI(t, root)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	var found map[string]any
	for _, check := range affectedChecks(t, receipt) {
		if strings.Contains(check["command"].(string), adviceAdvisoryPytest) {
			if found != nil {
				t.Fatalf("more than one pytest check: %v", affectedChecks(t, receipt))
			}
			found = check
		}
	}
	if found == nil {
		t.Fatalf("no advisory pytest check: %v / %v", affectedChecks(t, receipt), affectedAdviceUnknown(t, receipt))
	}
	for _, line := range affectedAdviceUnknown(t, receipt) {
		if strings.HasPrefix(line, "NO_ADVISORY_PYTEST_COMMAND") {
			t.Fatalf("a runnable pytest check must not also report it is missing: %v", line)
		}
	}
	if found["kind"] != adviceKindAdvisory || found["source"] != adviceSourcePlan {
		t.Fatalf("pytest check kind/source: %v", found)
	}
	return found
}

// TestAFPV0472FlatLayoutPytestCommand: AFP-V0-472, AFP-V0-474. A flat-layout
// project configured in pyproject.toml gets `python -m pytest` over its
// selected test module, run from the repository root, without a PYTHONPATH
// prefix (python -m puts the root on sys.path), and conftest.py and helper
// modules under tests/ drop out by python_files.
func TestAFPV0472FlatLayoutPytestCommand(t *testing.T) {
	t.Parallel()
	root := pytestAdviceRepository(t, map[string]string{
		"pyproject.toml":     "[project]\nname = \"calc\"\n\n[tool.pytest.ini_options]\ntestpaths = [\"tests\"]\n",
		"calc/__init__.py":   "",
		"calc/core.py":       "def add(a, b):\n    return a + b\n",
		"tests/__init__.py":  "",
		"tests/conftest.py":  "import calc.core\n",
		"tests/helpers.py":   "from calc.core import add\n\n\ndef twice(x):\n    return add(x, x)\n",
		"tests/test_core.py": "from calc.core import add\nfrom tests.helpers import twice\n\n\ndef test_add():\n    assert add(1, 2) == 3\n    assert twice(2) == 4\n",
	}, "calc/core.py", "def add(a, b):\n    return b + a\n")
	check := pytestCheck(t, root)
	if check["command"] != "python -m pytest './tests/test_core.py'" {
		t.Fatalf("flat-layout command = %v", check["command"])
	}
	reason := check["reason"].(string)
	if !strings.Contains(reason, "run from the repository root") || !strings.Contains(reason, "pyproject.toml") || strings.Contains(reason, "PYTHONPATH") {
		t.Fatalf("reason must name the root, pyproject.toml and no PYTHONPATH: %q", reason)
	}
}

// TestAFPV0475SrcLayoutPytestCommand: AFP-V0-472, AFP-V0-474, AFP-V0-475. A
// src-layout project configured in setup.cfg with its own python_files gets
// both matching modules and a PYTHONPATH prefix naming src, because its tests
// import calc from under the src source root.
func TestAFPV0475SrcLayoutPytestCommand(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"setup.cfg":            "[metadata]\nname = calc\n\n[tool:pytest]\npython_files =\n    test_*.py\n    check_*.py\n",
		"src/calc/__init__.py": "",
		"src/calc/core.py":     "def add(a, b):\n    return a + b\n",
		"tests/helpers.py":     "from calc.core import add\n",
		"tests/test_core.py":   "from calc.core import add\n\n\ndef test_add():\n    assert add(1, 2) == 3\n",
		"tests/check_extra.py": "from calc import core\n\n\ndef test_extra():\n    assert core.add(2, 2) == 4\n",
	}
	root := pytestAdviceRepository(t, files, "src/calc/core.py", "def add(a, b):\n    return b + a\n")
	check := pytestCheck(t, root)
	if check["command"] != "PYTHONPATH='src' python -m pytest './tests/check_extra.py' './tests/test_core.py'" {
		t.Fatalf("src-layout command = %v", check["command"])
	}
	if reason := check["reason"].(string); !strings.Contains(reason, "setup.cfg") || !strings.Contains(reason, "source root(s) src") {
		t.Fatalf("reason must name setup.cfg and the src root: %q", reason)
	}

	// AFP-V0-475: a configuration that declares pythonpath already puts the
	// root on the module path, so the prefix is not added.
	files["setup.cfg"] = "[tool:pytest]\npythonpath = src\npython_files = test_*.py check_*.py\n"
	declared := pytestAdviceRepository(t, files, "src/calc/core.py", "def add(a, b):\n    return b + a\n")
	if command := pytestCheck(t, declared)["command"]; command != "python -m pytest './tests/check_extra.py' './tests/test_core.py'" {
		t.Fatalf("a declared pythonpath must suppress the prefix: %v", command)
	}
}

func pythonPlan(tests ...string) affected.Plan {
	plan := affected.Plan{}
	for _, test := range tests {
		plan.Selected = append(plan.Selected, affected.Selection{UnitID: "python:" + strings.TrimSuffix(strings.ReplaceAll(test, "/", "."), ".py"), Tests: []string{test}})
	}
	return plan
}

// TestAFPV0473PytestNeedsADeclaration: AFP-V0-473, AFP-V0-476. Without a
// pytest configuration, conftest.py or a pytest dependency the selection
// yields one named unknown; any one of them is enough, an empty pytest.ini
// included.
func TestAFPV0473PytestNeedsADeclaration(t *testing.T) {
	t.Parallel()
	plan := pythonPlan("tests/test_a.py")
	bare := t.TempDir()
	if checks, unknown := advisoryPytestCheck(bare, nil, plan); len(checks) != 0 || !slices.Equal(unknown, []string{adviceNoPytestDeclared}) {
		t.Fatalf("undeclared pytest = %v / %v", checks, unknown)
	}
	for name, body := range map[string]string{
		"pytest.ini":           "",
		"tox.ini":              "[pytest]\naddopts = -q\n",
		"requirements-dev.txt": "pytest>=7\n",
		"setup.py":             "setup(tests_require=['pytest'])\n",
	} {
		root := t.TempDir()
		writePytestFixture(t, root, map[string]string{name: body})
		checks, unknown := advisoryPytestCheck(root, nil, plan)
		if len(unknown) != 0 || len(checks) != 1 || checks[0].Command != "python -m pytest './tests/test_a.py'" {
			t.Fatalf("%s must declare pytest: %v / %v", name, checks, unknown)
		}
	}
	// A word that merely contains pytest declares nothing.
	root := t.TempDir()
	writePytestFixture(t, root, map[string]string{"requirements.txt": "notpytest_x\n"})
	if _, unknown := advisoryPytestCheck(root, nil, plan); !slices.Equal(unknown, []string{adviceNoPytestDeclared}) {
		t.Fatalf("a non-pytest token must not declare pytest: %v", unknown)
	}
	// A selected conftest.py declares pytest even with no manifest.
	conftest := pythonPlan("tests/conftest.py", "tests/test_a.py")
	if checks, _ := advisoryPytestCheck(bare, nil, conftest); len(checks) != 1 {
		t.Fatalf("a conftest.py must declare pytest: %v", checks)
	}
}

// TestAFPV0474PytestConfigurationDiscovery: AFP-V0-474, AFP-V0-476. The
// nearest configuration above the selected modules wins in pytest's order,
// python_files filters the candidates and is re-located until it settles,
// and a candidate configuration that cannot be read is an unknown.
func TestAFPV0474PytestConfigurationDiscovery(t *testing.T) {
	t.Parallel()
	// A tests/unit/pytest.ini nearer the filtered files replaces the root one.
	root := t.TempDir()
	writePytestFixture(t, root, map[string]string{
		"pyproject.toml":        "[tool.pytest.ini_options]\naddopts = \"-q\"\n",
		"tests/unit/pytest.ini": "",
	})
	checks, unknown := advisoryPytestCheck(root, nil, pythonPlan("tests/conftest.py", "tests/unit/test_a.py"))
	if len(unknown) != 0 || len(checks) != 1 || checks[0].Command != "python -m pytest './tests/unit/test_a.py'" ||
		!strings.Contains(checks[0].Reason, "tests/unit/pytest.ini") {
		t.Fatalf("nearest configuration = %v / %v", checks, unknown)
	}

	// pytest.ini outranks pyproject.toml in the same directory, and its
	// python_files replace the default.
	ordered := t.TempDir()
	writePytestFixture(t, ordered, map[string]string{
		"pyproject.toml": "[tool.pytest.ini_options]\npython_files = [\"test_*.py\"]\n",
		"pytest.ini":     "[pytest]\npython_files = check_*.py\n",
	})
	checks, unknown = advisoryPytestCheck(ordered, nil, pythonPlan("tests/check_a.py", "tests/test_b.py"))
	if len(unknown) != 0 || len(checks) != 1 || checks[0].Command != "python -m pytest './tests/check_a.py'" {
		t.Fatalf("pytest.ini must win over pyproject.toml: %v / %v", checks, unknown)
	}

	// A selection with no test module by python_files is a named unknown.
	if checks, unknown := advisoryPytestCheck(ordered, nil, pythonPlan("tests/conftest.py", "tests/helpers.py")); len(checks) != 0 ||
		!slices.Equal(unknown, []string{fmt.Sprintf(adviceNoPytestModule, 2, "check_*.py")}) {
		t.Fatalf("no matching module = %v / %v", checks, unknown)
	}

	// Two configurations that each move the files to the other never settle.
	oscillating := t.TempDir()
	writePytestFixture(t, oscillating, map[string]string{
		"pytest.ini":     "[pytest]\npython_files = test_*.py\n",
		"x/y/pytest.ini": "[pytest]\npython_files = check_*.py\n",
	})
	if checks, unknown := advisoryPytestCheck(oscillating, nil, pythonPlan("x/check_4.py", "x/y/test_2.py")); len(checks) != 0 ||
		!slices.Equal(unknown, []string{fmt.Sprintf(advicePytestUnsettled, "pytest.ini", "x/y/pytest.ini")}) {
		t.Fatalf("unsettled configuration = %v / %v", checks, unknown)
	}

	// An oversized pyproject.toml may be the configuration, so it is unknown.
	oversized := t.TempDir()
	writePytestFixture(t, oversized, map[string]string{"pyproject.toml": strings.Repeat("#\n", adviceMaxPytestConfigBytes)})
	if checks, unknown := advisoryPytestCheck(oversized, nil, pythonPlan("tests/test_a.py")); len(checks) != 0 ||
		!slices.Equal(unknown, []string{fmt.Sprintf(advicePytestConfigUnknown, "pyproject.toml")}) {
		t.Fatalf("unreadable configuration = %v / %v", checks, unknown)
	}
}

// TestAFPV0476PytestBoundAndQuoting: AFP-V0-476. Past the file bound the
// command is replaced by one unknown line; operands are sorted, deduplicated
// and quoted with a leading ./.
func TestAFPV0476PytestBoundAndQuoting(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writePytestFixture(t, root, map[string]string{"pytest.ini": "[pytest]\n"})
	tests := []string{}
	for index := 0; index <= adviceMaxPytestFiles; index++ {
		tests = append(tests, fmt.Sprintf("tests/test_%04d.py", index))
	}
	if checks, unknown := advisoryPytestCheck(root, nil, pythonPlan(tests...)); len(checks) != 0 ||
		!slices.Equal(unknown, []string{fmt.Sprintf(advicePytestBoundUnknown, adviceMaxPytestFiles+1, adviceMaxPytestFiles)}) {
		t.Fatalf("bound = %d checks / %v", len(checks), unknown)
	}
	plan := pythonPlan("tests/test_z.py", "tests/test_it's.py")
	plan.Selected = append(plan.Selected, affected.Selection{UnitID: "python:tests.test_z", Tests: []string{"tests/test_z.py"}})
	checks, unknown := advisoryPytestCheck(root, nil, plan)
	if len(unknown) != 0 || len(checks) != 1 || checks[0].Command != `python -m pytest './tests/test_it'\''s.py' './tests/test_z.py'` {
		t.Fatalf("quoting = %v / %v", checks, unknown)
	}
}

// TestAFPV0477NonPythonPlanAdviceUnchanged: AFP-V0-477. A plan with no Python
// test file adds no pytest check or line, so Go and RSpec advice stay as they
// were, and the pytest check follows them when both apply.
func TestAFPV0477NonPythonPlanAdviceUnchanged(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writePytestFixture(t, root, map[string]string{"pytest.ini": "", "Gemfile": "gem \"rspec\"\n"})
	plan := affected.Plan{Selected: []affected.Selection{
		{UnitID: "go:example.com/fixture/core", Tests: []string{"core/core_test.go"}},
		{UnitID: "ruby:rspec:spec/a_spec", Tests: []string{"spec/a_spec.rb"}},
		{UnitID: "python:docs.conf", Tests: []string{"docs/conf.txt"}},
	}}
	provider := affectedGoProvider{Packages: []string{"example.com/fixture/core"}, State: providerStateRunnable}
	without := compileAffectedAdvice(root, nil, plan, provider)
	for _, line := range without.Unknown {
		if strings.HasPrefix(line, "NO_ADVISORY_PYTEST_COMMAND") {
			t.Fatalf("no Python test file may add a pytest line: %v", without.Unknown)
		}
	}
	if len(without.Checks) != 2 {
		t.Fatalf("Go and RSpec checks only: %v", without.Checks)
	}
	plan.Selected = append(plan.Selected, affected.Selection{UnitID: "python:tests.test_a", Tests: []string{"tests/test_a.py"}})
	with := compileAffectedAdvice(root, nil, plan, provider)
	// The Go reason counts every selected unit, so compare the commands.
	if len(with.Checks) != 3 || with.Checks[0].Command != without.Checks[0].Command || with.Checks[1] != without.Checks[1] ||
		with.Checks[2].Command != "python -m pytest './tests/test_a.py'" {
		t.Fatalf("pytest check must follow the unchanged Go and RSpec checks: %v", with.Checks)
	}
}

// TestAFPV0475TestsUnderASourceRootGetThePrefix: AFP-V0-475. A test module
// that lives under the src source root and imports calc from it still needs
// the root on the module path: pytest puts the test's own directory there,
// not src. Two reached roots are joined with the platform separator, and a
// TOML comment naming a root is not a pythonpath entry.
func TestAFPV0475TestsUnderASourceRootGetThePrefix(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"pytest.ini":             "[pytest]\n",
		"src/calc/__init__.py":   "",
		"src/calc/core.py":       "def add(a, b):\n    return a + b\n",
		"src/tests/test_core.py": "from calc.core import add\n\n\ndef test_add():\n    assert add(1, 2) == 3\n",
	}
	root := pytestAdviceRepository(t, files, "src/calc/core.py", "def add(a, b):\n    return b + a\n")
	if command := pytestCheck(t, root)["command"]; command != "PYTHONPATH='src' python -m pytest './src/tests/test_core.py'" {
		t.Fatalf("a test under the source root = %v", command)
	}

	if advicePathListSeparator != string(os.PathListSeparator) {
		t.Fatalf("PYTHONPATH separator = %q", advicePathListSeparator)
	}
	two := map[string]string{
		"pyproject.toml":       "[tool.pytest.ini_options]\npythonpath = [] # \"src\", \"lib\"\n",
		"src/calc/__init__.py": "",
		"src/calc/core.py":     "def add(a, b):\n    return a + b\n",
		"lib/util/__init__.py": "",
		"lib/util/text.py":     "def shout(s):\n    return s.upper()\n",
		"tests/test_both.py":   "from calc.core import add\nfrom util.text import shout\n\n\ndef test_both():\n    assert add(1, 2) == 3\n    assert shout('a') == 'A'\n",
	}
	both := pytestAdviceRepository(t, two, "src/calc/core.py", "def add(a, b):\n    return b + a\n")
	if command := pytestCheck(t, both)["command"]; command != "PYTHONPATH='src"+advicePathListSeparator+"lib' python -m pytest './tests/test_both.py'" {
		t.Fatalf("two source roots = %v", command)
	}
}

// TestAFPV0474PytestValueParsing: AFP-V0-474. Values are read as pytest reads
// them: a TOML comment outside a string is not a value, an ini string is
// split with shell rules (quotes removed), a value pytest cannot split is a
// named unknown, and an explicitly empty python_files collects nothing.
func TestAFPV0474PytestValueParsing(t *testing.T) {
	t.Parallel()
	plan := pythonPlan("tests/check_a.py", "tests/helpers.py", "tests/test_b.py")
	for _, entry := range []struct{ name, body, command string }{
		{"pyproject.toml", "[tool.pytest.ini_options]\npython_files = [\"test_*.py\"] # \"helpers.py\"\n", "python -m pytest './tests/test_b.py'"},
		{"pyproject.toml", "[tool.pytest.ini_options]\npython_files = [\n  \"check_*.py\", # \"helpers.py\"\n  'test_*.py',\n]\n", "python -m pytest './tests/check_a.py' './tests/test_b.py'"},
		{"pytest.ini", "[pytest]\npython_files = \"check_*.py\" 'helpers.py'\n", "python -m pytest './tests/check_a.py' './tests/helpers.py'"},
		{"tox.ini", "[pytest]\npython_files =\n    \"check_*.py\"\n    # test_*.py\n", "python -m pytest './tests/check_a.py'"},
	} {
		root := t.TempDir()
		writePytestFixture(t, root, map[string]string{entry.name: entry.body})
		checks, unknown := advisoryPytestCheck(root, nil, plan)
		if len(unknown) != 0 || len(checks) != 1 || checks[0].Command != entry.command {
			t.Fatalf("%s %q = %v / %v", entry.name, entry.body, checks, unknown)
		}
	}
	unsplittable := t.TempDir()
	writePytestFixture(t, unsplittable, map[string]string{"pytest.ini": "[pytest]\npython_files = \"check_*.py\n"})
	if checks, unknown := advisoryPytestCheck(unsplittable, nil, plan); len(checks) != 0 ||
		!slices.Equal(unknown, []string{fmt.Sprintf(advicePytestSplitUnknown, "pytest.ini", "python_files")}) {
		t.Fatalf("unsplittable value = %v / %v", checks, unknown)
	}
	for name, body := range map[string]string{
		"pytest.ini":     "[pytest]\npython_files =\n",
		"pyproject.toml": "[tool.pytest.ini_options]\npython_files = []\n",
	} {
		root := t.TempDir()
		writePytestFixture(t, root, map[string]string{name: body})
		if checks, unknown := advisoryPytestCheck(root, nil, plan); len(checks) != 0 ||
			!slices.Equal(unknown, []string{fmt.Sprintf(adviceNoPytestModule, 3, "(empty)")}) {
			t.Fatalf("%s empty python_files = %v / %v", name, checks, unknown)
		}
	}
}

// TestAFPV0474PytestPerArgumentDiscovery: AFP-V0-474. When nothing is found
// from the files' common directory, pytest stops at a setup.py between it and
// the root, takes a pyproject.toml without a pytest table as its
// configuration file, and otherwise searches from each file's own directory.
func TestAFPV0474PytestPerArgumentDiscovery(t *testing.T) {
	t.Parallel()
	plan := pythonPlan("tests/a/check_1.py", "tests/b/test_2.py")
	for _, entry := range []struct {
		files   map[string]string
		command string
		reason  string
	}{
		{map[string]string{"tests/a/pytest.ini": "[pytest]\npython_files = check_*.py\n"},
			"python -m pytest './tests/a/check_1.py'", "tests/a/pytest.ini"},
		{map[string]string{"tests/a/pytest.ini": "[pytest]\npython_files = check_*.py\n", "setup.py": "setup(tests_require=['pytest'])\n"},
			"python -m pytest './tests/b/test_2.py'", "pytest's defaults apply"},
		{map[string]string{"tests/a/pytest.ini": "[pytest]\npython_files = check_*.py\n", "pyproject.toml": "[project]\nname = \"x\"\n", "requirements.txt": "pytest\n"},
			"python -m pytest './tests/b/test_2.py'", "pyproject.toml"},
	} {
		root := t.TempDir()
		writePytestFixture(t, root, entry.files)
		checks, unknown := advisoryPytestCheck(root, nil, plan)
		if len(unknown) != 0 || len(checks) != 1 || checks[0].Command != entry.command || !strings.Contains(checks[0].Reason, entry.reason) {
			t.Fatalf("%v = %v / %v", entry.files, checks, unknown)
		}
	}
	// A pyproject.toml without a pytest table is the configuration file but
	// declares nothing on its own.
	root := t.TempDir()
	writePytestFixture(t, root, map[string]string{"pyproject.toml": "[project]\nname = \"x\"\n"})
	if _, unknown := advisoryPytestCheck(root, nil, plan); !slices.Equal(unknown, []string{adviceNoPytestDeclared}) {
		t.Fatalf("a table-less pyproject.toml must not declare pytest: %v", unknown)
	}
}

// TestAFPV0473PytestDependencyDiscoveryBound: AFP-V0-473. The dependency
// manifest search is bounded in files and bytes; reaching a bound before
// pytest is found is a named unknown.
func TestAFPV0473PytestDependencyDiscoveryBound(t *testing.T) {
	t.Parallel()
	plan := pythonPlan("tests/test_a.py")
	many := map[string]string{}
	for index := 0; index <= adviceMaxPytestRequirements; index++ {
		many[fmt.Sprintf("requirements-%02d.txt", index)] = "requests\n"
	}
	large := map[string]string{}
	for _, name := range advicePytestDependencyManifests {
		large[name] = strings.Repeat("#\n", 125<<10)
	}
	for bound, files := range map[string]map[string]string{
		fmt.Sprintf("more than %d requirements*.txt files", adviceMaxPytestRequirements): many,
		fmt.Sprintf("more than %d bytes of manifests", adviceMaxPytestManifestBytes):     large,
	} {
		root := t.TempDir()
		writePytestFixture(t, root, files)
		if checks, unknown := advisoryPytestCheck(root, nil, plan); len(checks) != 0 ||
			!slices.Equal(unknown, []string{fmt.Sprintf(advicePytestDependencyUnbounded, bound)}) {
			t.Fatalf("%s = %v / %v", bound, checks, unknown)
		}
	}
}
