// SPDX-License-Identifier: Apache-2.0
package python

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// slfv1006Fault is one pre-registered fault for a narrowing rule
// (SLF-V1-006): a project, the file whose line the fault changes and its
// faulted body, and the test that would kill the fault.
type slfv1006Fault struct {
	name   string
	files  map[string]string
	path   string
	mutant string
	killer string
}

// slfv1006ImportRootFaults are the SLF-V1-001 cases, one per import-root
// source the rule reads: pytest pythonpath in each configuration form, a
// namespace package below a pythonpath root, a layout root, a testpaths root
// and a transitive edge through two roots.
var slfv1006ImportRootFaults = []slfv1006Fault{
	{
		name: "pyproject pythonpath",
		files: map[string]string{
			"pyproject.toml":  "[tool.pytest.ini_options]\npythonpath = [\"app\"]\n",
			"app/billing.py":  "def total():\n    return 1\n",
			"tests/test_x.py": "from billing import total\n\ndef test_total():\n    assert total() == 1\n",
		},
		path: "app/billing.py", mutant: "def total():\n    return 2\n", killer: "tests/test_x.py",
	},
	{
		name: "namespace package under pytest.ini pythonpath",
		files: map[string]string{
			"pytest.ini":       "[pytest]\npythonpath =\n    app\n",
			"app/ns/mod.py":    "VALUE = 1\n",
			"tests/test_ns.py": "from ns.mod import VALUE\n\ndef test_value():\n    assert VALUE == 1\n",
		},
		path: "app/ns/mod.py", mutant: "VALUE = 0\n", killer: "tests/test_ns.py",
	},
	{
		name: "layout root above a package",
		files: map[string]string{
			"services/core/__init__.py": "",
			"services/core/rates.py":    "RATE = 1\n",
			"tests/test_rates.py":       "from core.rates import RATE\n\ndef test_rate():\n    assert RATE == 1\n",
		},
		path: "services/core/rates.py", mutant: "RATE = -1\n", killer: "tests/test_rates.py",
	},
	{
		name: "testpaths root",
		files: map[string]string{
			"pytest.ini":                 "[pytest]\ntestpaths = backend/tests\n",
			"backend/tests/__init__.py":  "",
			"backend/tests/helpers.py":   "HELP = 1\n",
			"backend/tests/test_help.py": "from tests.helpers import HELP\n\ndef test_help():\n    assert HELP == 1\n",
		},
		path: "backend/tests/helpers.py", mutant: "HELP = 2\n", killer: "backend/tests/test_help.py",
	},
	{
		name: "setup.cfg pythonpath",
		files: map[string]string{
			"setup.cfg":           "[tool:pytest]\npythonpath = lib2\n",
			"lib2/pricing.py":     "def price():\n    return 10\n",
			"tests/test_price.py": "import pricing\n\ndef test_price():\n    assert pricing.price() == 10\n",
		},
		path: "lib2/pricing.py", mutant: "def price():\n    return 11\n", killer: "tests/test_price.py",
	},
	{
		name: "transitive edge under tox.ini pythonpath",
		files: map[string]string{
			"tox.ini":         "[pytest]\npythonpath = app\n",
			"app/billing.py":  "from taxes import RATE\n\ndef total():\n    return 100 * (1 + RATE)\n",
			"app/taxes.py":    "RATE = 0.2\n",
			"tests/test_x.py": "from billing import total\n\ndef test_total():\n    assert total() == 120\n",
		},
		path: "app/taxes.py", mutant: "RATE = 0.3\n", killer: "tests/test_x.py",
	},
}

// slfv1006Plan writes the faulted tree, the head of a fault-carrying change,
// and returns the plan of the faulted path and the plugin's
// absolute-import frontier entries.
func slfv1006Plan(t *testing.T, fault slfv1006Fault) (affected.Plan, []string) {
	t.Helper()
	root := t.TempDir()
	for relative, body := range fault.files {
		write(t, root, relative, body)
	}
	write(t, root, fault.path, fault.mutant)
	graph, err := affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatal(err)
	}
	return affected.Select(graph, []string{fault.path}), absoluteEntries(pythonUnits(t, root).Frontier)
}

// SLF-V1-006 for SLF-V1-001, the selection side of each FIH-V1 case: with
// the rule's gate on, the plan of each fault-carrying change is BOUNDED (the
// rule discharged the frontier) and selects the test that kills the fault;
// with the gate held, the absolute-import frontier stays open and the plan
// still selects the killer or is UNKNOWN, never leaving the changed module
// Uncovered. That the killer fails on the faulted tree is the FIH-V1 kill,
// which needs the harness's pytest adapter and is not measured here.
func TestSLFV1006ImportRootFaultCases(t *testing.T) {
	if len(slfv1006ImportRootFaults) < 5 {
		t.Fatalf("SLF-V1-006 needs at least five faults for SLF-V1-001, have %d", len(slfv1006ImportRootFaults))
	}
	for _, fault := range slfv1006ImportRootFaults {
		t.Run(fault.name, func(t *testing.T) {
			held, frontier := slfv1006Plan(t, fault)
			if len(frontier) == 0 {
				t.Fatalf("held rule: no %s frontier, so the case does not exercise SLF-V1-001", FrontierAbsoluteImport)
			}
			assertSelectedOrUnknown(t, held, fault.killer, "python:"+strings.ReplaceAll(strings.TrimSuffix(fault.path, ".py"), "/", "."))
			withImportRoots(t)
			enabled, frontier := slfv1006Plan(t, fault)
			if len(frontier) != 0 || enabled.Scope != affected.ScopeBounded || !slices.Contains(enabled.SelectedTests(), fault.killer) {
				t.Fatalf("enabled rule: frontier=%v scope=%s selected=%v unknown=%+v, want BOUNDED with killer %s", frontier, enabled.Scope, enabled.SelectedTests(), enabled.Unknown, fault.killer)
			}
		})
	}
}
