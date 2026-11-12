// SPDX-License-Identifier: Apache-2.0
package python

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// withImportRoots turns SLF-V1-001's gate on for one test (SLF-V1-006 holds it
// off in the product).
func withImportRoots(t *testing.T) {
	t.Helper()
	previous := importRootsNarrowing
	importRootsNarrowing = true
	t.Cleanup(func() { importRootsNarrowing = previous })
}

func absoluteEntries(frontier []string) []string {
	var entries []string
	for _, entry := range frontier {
		if strings.HasPrefix(entry, FrontierAbsoluteImport) {
			entries = append(entries, entry)
		}
	}
	return entries
}

func hasUnknown(plan affected.Plan, reason, detail string) bool {
	return slices.ContainsFunc(plan.Unknown, func(unknown affected.Unknown) bool {
		return unknown.Reason == reason && (detail == "" || unknown.Detail == detail)
	})
}

func uncovered(plan affected.Plan, id string) bool {
	return slices.ContainsFunc(plan.Uncovered, func(entry affected.Unknown) bool { return entry.Detail == id })
}

// assertSelectedOrUnknown is SLF-V1-002's plan rule: the importing test is
// selected or the plan is UNKNOWN, and the changed module is never Uncovered.
func assertSelectedOrUnknown(t *testing.T, plan affected.Plan, test, changed string) {
	t.Helper()
	if !slices.Contains(plan.SelectedTests(), test) && plan.Scope != affected.ScopeUnknown {
		t.Fatalf("%s neither selected nor UNKNOWN: scope=%s selected=%v unknown=%+v", test, plan.Scope, plan.SelectedTests(), plan.Unknown)
	}
	if uncovered(plan, changed) {
		t.Fatalf("%s listed as Uncovered while an absolute-import frontier is open: %+v", changed, plan.Uncovered)
	}
}

// SLF-V1-002: an absolute import whose first component names a directory of
// the Python tree, but which resolves to no unit, raises the frontier at the
// importing file; a third-party import raises nothing.
func TestSLFV1002UnresolvedInRepositoryAbsoluteImport(t *testing.T) {
	files := map[string]string{
		"services/core/__init__.py": "",
		"services/core/rates.py":    "RATE = 1\n",
		"tests/test_y.py":           "import os\nimport requests\nfrom core.rates import RATE\n",
	}
	selected, plan := selectedFor(t, files, "services/core/rates.py")
	root := t.TempDir()
	for relative, body := range files {
		write(t, root, relative, body)
	}
	result := pythonUnits(t, root)
	if got := absoluteEntries(result.Frontier); fmt.Sprint(got) != fmt.Sprint([]string{FrontierAbsoluteImport + ":tests/test_y.py"}) {
		t.Fatalf("absolute-import frontier = %v, want one site at tests/test_y.py", got)
	}
	if fmt.Sprint(result.FrontierSites[FrontierAbsoluteImport]) != "[tests/test_y.py]" {
		t.Fatalf("FrontierSites = %v", result.FrontierSites)
	}
	if !slices.Contains(selected, "tests/test_y.py") {
		t.Fatalf("OUTBOUND site closure did not select the importer: %v (unknown %+v)", selected, plan.Unknown)
	}
	assertSelectedOrUnknown(t, plan, "tests/test_y.py", "python:services.core.rates")
	if !hasUnknown(plan, affected.UnknownNoSelectableTest, "python:services.core.rates") {
		t.Fatalf("changed module with no modelled consumer is not NO_SELECTABLE_TEST: %+v", plan.Unknown)
	}

	// A third-party import names nothing in the tree and raises nothing.
	write(t, root, "tests/test_y.py", "import os\nimport requests\nfrom services.core.rates import RATE\n")
	if got := absoluteEntries(pythonUnits(t, root).Frontier); len(got) != 0 {
		t.Fatalf("resolved and third-party imports raised %v", got)
	}
}

// SLF-V1-002: the direction is OUTBOUND, the reason is source-bound, and the
// other Python reasons stay INBOUND.
func TestSLFV1002DeclaresOutboundSourceBound(t *testing.T) {
	language := New()
	if got := language.FrontierDirection(FrontierAbsoluteImport); got != affected.FrontierOutbound {
		t.Fatalf("direction = %s", got)
	}
	for _, reason := range []string{FrontierDynamicImport, FrontierRelativeImport, FrontierUnreadableSource} {
		if got := language.FrontierDirection(reason); got != affected.FrontierInbound {
			t.Fatalf("%s direction = %s", reason, got)
		}
	}
	if !slices.Contains(language.SourceBoundFrontiers(), FrontierAbsoluteImport) {
		t.Fatal("absolute-import frontier is not source-bound")
	}
	if !language.CoverageUnproven("python:x", []string{FrontierAbsoluteImport + ":+3"}) || language.CoverageUnproven("python:x", []string{FrontierDynamicImport}) {
		t.Fatal("CoverageUnproven does not follow the absolute-import frontier")
	}
}

// SLF-V1-002: at most 16 sites are named and one `+N` entry counts the rest,
// while FrontierSites keeps every site, so the graph still closes them all.
func TestSLFV1002BoundsNamedSites(t *testing.T) {
	root := t.TempDir()
	write(t, root, "services/core/__init__.py", "")
	write(t, root, "services/core/rates.py", "RATE = 1\n")
	for index := range 20 {
		write(t, root, fmt.Sprintf("tests/test_%02d.py", index), "import core.rates\n")
	}
	result := pythonUnits(t, root)
	entries := absoluteEntries(result.Frontier)
	if len(entries) != absoluteImportSiteBound+1 || !slices.Contains(entries, FrontierAbsoluteImport+":+4") {
		t.Fatalf("entries = %v", entries)
	}
	if len(result.FrontierSites[FrontierAbsoluteImport]) != 20 {
		t.Fatalf("sites = %v", result.FrontierSites[FrontierAbsoluteImport])
	}
	graph, err := affected.Build(t.Context(), root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"services/core/rates.py"})
	if got := len(plan.SelectedTests()); got != 20 {
		t.Fatalf("selected %d of 20 importers: %v", got, plan.SelectedTests())
	}
}

// flatLayout is the SLF-V1-001 and task 3 fixture: pytest puts `app` on the
// path, which no fixed root names.
var flatLayout = map[string]string{
	"pyproject.toml":  "[tool.pytest.ini_options]\npythonpath = [\"app\"]\ntestpaths = [\"tests\"]\n",
	"app/billing.py":  "def total():\n    return 1\n",
	"tests/test_x.py": "from billing import total\n",
}

// Task 3 proof (SLF-V1-002): with SLF-V1-001 held, the flat layout's import
// raises the frontier, and a change to the module selects the importer or is
// UNKNOWN, never Uncovered.
func TestSLFV1002FlatLayoutHeldRuleNeverUncovered(t *testing.T) {
	selected, plan := selectedFor(t, flatLayout, "app/billing.py")
	if !slices.Contains(selected, "tests/test_x.py") && plan.Scope != affected.ScopeUnknown {
		t.Fatalf("plan = %+v", plan)
	}
	assertSelectedOrUnknown(t, plan, "tests/test_x.py", "python:app.billing")
	if !hasUnknown(plan, affected.UnknownNoSelectableTest, "python:app.billing") {
		t.Fatalf("app.billing not NO_SELECTABLE_TEST: %+v", plan.Unknown)
	}
}

// SLF-V1-001: with the gate on, pytest's pythonpath is an import root, so the
// import resolves to an edge, raises no frontier and the plan is BOUNDED.
func TestSLFV1001ConfiguredPythonpath(t *testing.T) {
	withImportRoots(t)
	root := t.TempDir()
	for relative, body := range flatLayout {
		write(t, root, relative, body)
	}
	result := pythonUnits(t, root)
	if got := absoluteEntries(result.Frontier); len(got) != 0 {
		t.Fatalf("frontier = %v", got)
	}
	if imports, _ := unitImports(result, "python:tests.test_x"); !slices.Contains(imports, "python:app.billing") {
		t.Fatalf("imports = %v", imports)
	}
	selected, plan := selectedFor(t, flatLayout, "app/billing.py")
	if !slices.Contains(selected, "tests/test_x.py") || plan.Scope != affected.ScopeBounded {
		t.Fatalf("scope=%s selected=%v unknown=%+v", plan.Scope, selected, plan.Unknown)
	}
	if !New().ResolutionInput("sub/pyproject.toml") || New().ResolutionInput("sub/README.md") {
		t.Fatal("pytest configuration is not a resolution input with the rule on")
	}
}

// SLF-V1-001: the layout roots. services/ holds no `__init__.py`, so it is the
// nearest non-package ancestor of services/core/rates.py; a namespace package
// below a pythonpath root resolves through that root; a testpaths entry's
// nearest non-package ancestor is a root.
func TestSLFV1001LayoutAndNamespaceRoots(t *testing.T) {
	withImportRoots(t)
	root := t.TempDir()
	write(t, root, "pytest.ini", "[pytest]\npythonpath =\n    app\ntestpaths = backend/tests\n")
	write(t, root, "services/core/__init__.py", "")
	write(t, root, "services/core/rates.py", "RATE = 1\n")
	write(t, root, "app/ns/mod.py", "VALUE = 1\n")
	write(t, root, "backend/tests/__init__.py", "")
	write(t, root, "backend/tests/helpers.py", "HELP = 1\n")
	write(t, root, "tests/test_y.py", "from core.rates import RATE\nfrom ns.mod import VALUE\nfrom tests.helpers import HELP\n")
	result := pythonUnits(t, root)
	if got := absoluteEntries(result.Frontier); len(got) != 0 {
		t.Fatalf("frontier = %v", got)
	}
	imports, _ := unitImports(result, "python:tests.test_y")
	for _, want := range []string{"python:services.core.rates", "python:app.ns.mod", "python:backend.tests.helpers"} {
		if !slices.Contains(imports, want) {
			t.Fatalf("imports = %v, want %s", imports, want)
		}
	}
	// SLF-V1-006: the same tree with the gate off keeps the frontier.
	importRootsNarrowing = false
	if got := absoluteEntries(pythonUnits(t, root).Frontier); fmt.Sprint(got) != fmt.Sprint([]string{FrontierAbsoluteImport + ":tests/test_y.py"}) {
		t.Fatalf("held rule frontier = %v", got)
	}
}

// SLF-V1-001: a root only adds aliases. A src-layout edge the fixed roots make
// survives with the rule on, and two files sharing a dotted name both stay
// edges.
func TestSLFV1001RootsOnlyAddAliases(t *testing.T) {
	root := t.TempDir()
	write(t, root, "src/pkg/__init__.py", "")
	write(t, root, "src/pkg/util.py", "U = 1\n")
	write(t, root, "lib/util.py", "U = 2\n")
	write(t, root, "tests/test_u.py", "import pkg.util\nimport util\n")
	held, _ := unitImports(pythonUnits(t, root), "python:tests.test_u")
	withImportRoots(t)
	enabled, _ := unitImports(pythonUnits(t, root), "python:tests.test_u")
	for _, edge := range held {
		if !slices.Contains(enabled, edge) {
			t.Fatalf("enabling roots removed edge %s: held=%v enabled=%v", edge, held, enabled)
		}
	}
	for _, want := range []string{"python:src.pkg.util", "python:lib.util"} {
		if !slices.Contains(enabled, want) {
			t.Fatalf("enabled = %v, want %s", enabled, want)
		}
	}
}

// pytestOptions reads each configuration form's pytest section only.
func TestSLFV1001ReadsPytestSections(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		declared   bool
		pythonpath []string
	}{
		{"pyproject.toml", "[tool.pytest.ini_options]\npythonpath = [\n  \"src\", # comment\n  'app',\n]\n", true, []string{"src", "app"}},
		{"pyproject.toml", "[tool.poetry]\nname = \"x\"\n", false, nil},
		{"setup.cfg", "[tool:pytest]\npythonpath = a b\n    c\n", true, []string{"a", "b", "c"}},
		{"tox.ini", "[tox]\npythonpath = wrong\n[pytest]\npythonpath = right\n", true, []string{"right"}},
		{"pytest.ini", "", true, nil},
	} {
		values, declared := pytestOptions(tc.name, tc.body)
		if declared != tc.declared || fmt.Sprint(values["pythonpath"]) != fmt.Sprint(tc.pythonpath) {
			t.Errorf("%s %q: declared=%v pythonpath=%v", tc.name, tc.body, declared, values["pythonpath"])
		}
	}
}
