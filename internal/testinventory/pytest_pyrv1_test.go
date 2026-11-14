// SPDX-License-Identifier: Apache-2.0
package testinventory

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// pytestCollect is an Exec answering `pytest --collect-only -q` with the
// node IDs out maps the instance directory (relative to root) to; any other
// argv or directory fails, so the test also proves the command and its
// working directory.
func pytestCollect(t *testing.T, root string, out map[string][]string, calls *[]string) Exec {
	t.Helper()
	return func(_ context.Context, dir string, argv []string) ([]byte, error) {
		rel, err := filepath.Rel(root, dir)
		if err != nil {
			return nil, err
		}
		rel = filepath.ToSlash(rel)
		*calls = append(*calls, rel+": "+strings.Join(argv, " "))
		ids, ok := out[rel]
		if !ok || len(argv) < 5 || argv[1] != "-m" || argv[2] != "pytest" || argv[3] != "--collect-only" {
			return nil, fmt.Errorf("unexpected lister call in %s: %v", rel, argv)
		}
		return []byte(strings.Join(ids, "\n") + "\n\n" + fmt.Sprint(len(ids)) + " tests collected in 0.01s\n"), nil
	}
}

// PYR-V1-001: a pytest configuration in pyproject.toml is one executing
// instance; every collected node ID is listed with its file and canonical
// test ID, through pytest's own collection from the rootdir.
func TestPYRV1001PyprojectRootIsListed(t *testing.T) {
	root, git := gitRepo(t, map[string]string{
		"pyproject.toml":    "[project]\nname = \"x\"\n\n[tool.pytest.ini_options]\naddopts = \"-q\"\n",
		"tests/conftest.py": "import pytest\n\n@pytest.fixture\ndef value():\n    return 1\n",
		"tests/test_a.py":   "def test_one(value):\n    assert value\n\nclass TestK:\n    def test_two(self):\n        pass\n",
	})
	var calls []string
	inventory := refreshTwice(t, git, pytestCollect(t, root, map[string][]string{
		".": {"tests/test_a.py::test_one", "tests/test_a.py::TestK::test_two"},
	}, &calls))
	pytest := instanceByKey(t, inventory, "pytest@.")
	if pytest.Config != "pyproject.toml" || pytest.Lister.Kind != KindExecuting || pytest.Lister.Name != "pytest-collect" {
		t.Fatalf("pytest instance %+v", pytest)
	}
	var ids, files []string
	for _, test := range pytest.Tests {
		ids, files = append(ids, test.ID), append(files, test.File)
	}
	if want := []string{"pytest:tests/test_a.py::TestK::test_two", "pytest:tests/test_a.py::test_one"}; !slices.Equal(ids, want) {
		t.Fatalf("pytest IDs %q, want %q", ids, want)
	}
	if !slices.Equal(files, []string{"tests/test_a.py", "tests/test_a.py"}) {
		t.Fatalf("pytest files %q", files)
	}
	if !slices.ContainsFunc(calls, func(call string) bool { return strings.HasSuffix(call, "-c pyproject.toml") }) {
		t.Fatalf("lister calls %q do not read pyproject.toml", calls)
	}
}

// PYR-V1-001: pytest configurations in services/a/pyproject.toml and
// services/b/pytest.ini are two instances, one per rootdir, each listed by
// its own collection run from its root.
func TestPYRV1001TwoRootdirsAreTwoInstances(t *testing.T) {
	root, git := gitRepo(t, map[string]string{
		"services/a/pyproject.toml":  "[tool.pytest.ini_options]\n",
		"services/a/tests/test_a.py": "def test_a():\n    pass\n",
		"services/b/pytest.ini":      "[pytest]\n",
		"services/b/test_b.py":       "def test_b():\n    pass\n",
	})
	var calls []string
	inventory := refreshTwice(t, git, pytestCollect(t, root, map[string][]string{
		"services/a": {"tests/test_a.py::test_a"},
		"services/b": {"test_b.py::test_b"},
	}, &calls))
	var keys []string
	for _, instance := range inventory.Instances {
		if instance.Runner == "pytest" {
			keys = append(keys, instance.Key)
		}
	}
	if !slices.Equal(keys, []string{"pytest@services/a", "pytest@services/b"}) {
		t.Fatalf("pytest instances %q (failures %+v)", keys, inventory.Failures)
	}
	a, b := instanceByKey(t, inventory, "pytest@services/a"), instanceByKey(t, inventory, "pytest@services/b")
	if got := testsOf(a); !reflect.DeepEqual(got, []Test{{ID: "pytest:services/a/tests/test_a.py::test_a", File: "services/a/tests/test_a.py", Kind: KindUnit, KindSource: KindSourceDefault}}) {
		t.Fatalf("instance a tests %+v", got)
	}
	if got := testsOf(b); !reflect.DeepEqual(got, []Test{{ID: "pytest:services/b/test_b.py::test_b", File: "services/b/test_b.py", Kind: KindUnit, KindSource: KindSourceDefault}}) {
		t.Fatalf("instance b tests %+v", got)
	}
	if !slices.ContainsFunc(calls, func(call string) bool {
		return strings.HasPrefix(call, "services/a: ") && strings.HasSuffix(call, "-c pyproject.toml")
	}) ||
		!slices.ContainsFunc(calls, func(call string) bool {
			return strings.HasPrefix(call, "services/b: ") && strings.HasSuffix(call, "-c pytest.ini")
		}) {
		t.Fatalf("lister calls %q", calls)
	}
}

// PYR-V1-005: each pytest test carries kind and kindSource: e2e from a
// pytest-playwright fixture (a parameter or usefixtures) when the plugin is
// in use, integration from the `integration` marker on the function, its
// class or the module, unit otherwise, and UNKNOWN on a conflict or when
// its function is not in the file. The plugin is in use when a dependency
// manifest names it or the test file mentions playwright; without either a
// `page` fixture is the project's own and the test stays unit.
func TestPYRV1005KindLabels(t *testing.T) {
	source := `import pytest
from playwright.sync_api import Page, expect

def test_unit():
    assert True

def test_home(page: Page) -> None:
    expect(page).to_have_title("x")

@pytest.mark.parametrize("n", [1, 2])
@pytest.mark.integration
def test_db(n):
    pass

@pytest.mark.usefixtures("browser")
def test_used():
    pass

class TestApi:
    pytestmark = [pytest.mark.integration]

    def test_call(self):
        pass

    def test_both(
        self,
        context,
    ):
        pass
`
	root, git := gitRepo(t, map[string]string{
		"pytest.ini":         "[pytest]\n",
		"requirements.txt":   "pytest\n",
		"tests/test_mix.py":  source,
		"other/pytest.ini":   "[pytest]\n",
		"other/test_page.py": "def test_page(page):\n    pass\n",
		"other/test_gen.py":  "pytestmark = pytest.mark.integration\n\ndef test_gen():\n    pass\n",
		"other/conftest.py":  "import pytest\n\n@pytest.fixture\ndef page():\n    return 1\n",
	})
	var calls []string
	inventory := refreshTwice(t, git, pytestCollect(t, root, map[string][]string{
		".": {"tests/test_mix.py::test_unit", "tests/test_mix.py::test_home", "tests/test_mix.py::test_db[1]", "tests/test_mix.py::test_used",
			"tests/test_mix.py::TestApi::test_call", "tests/test_mix.py::TestApi::test_both", "tests/test_mix.py::test_generated[a]"},
		"other": {"test_page.py::test_page", "test_gen.py::test_gen"},
	}, &calls))
	kinds := map[string]string{}
	for _, instance := range inventory.Instances {
		for _, test := range instance.Tests {
			kinds[test.ID] = test.Kind + "/" + test.KindSource
		}
	}
	want := map[string]string{
		"pytest:tests/test_mix.py::test_unit":          "unit/default",
		"pytest:tests/test_mix.py::test_home":          "e2e/runner",
		"pytest:tests/test_mix.py::test_db[1]":         "integration/runner",
		"pytest:tests/test_mix.py::test_used":          "e2e/runner",
		"pytest:tests/test_mix.py::TestApi::test_call": "integration/runner",
		"pytest:tests/test_mix.py::TestApi::test_both": "UNKNOWN/conflict",
		"pytest:tests/test_mix.py::test_generated[a]":  "UNKNOWN/unresolved",
		"pytest:other/test_page.py::test_page":         "unit/default",
		"pytest:other/test_gen.py::test_gen":           "integration/runner",
	}
	if !reflect.DeepEqual(kinds, want) {
		t.Fatalf("kinds\n got %v\nwant %v", kinds, want)
	}

	// A dependency manifest naming the plugin is enough.
	root, git = gitRepo(t, map[string]string{
		"pyproject.toml":     "[project]\ndependencies = [\"pytest-playwright>=0.5\"]\n\n[tool.pytest.ini_options]\n",
		"tests/test_page.py": "def test_page(page):\n    pass\n",
	})
	inventory = refreshTwice(t, git, pytestCollect(t, root, map[string][]string{".": {"tests/test_page.py::test_page"}}, &calls))
	if got := testsOf(instanceByKey(t, inventory, "pytest@.")); len(got) != 1 || got[0].Kind != KindE2E || got[0].KindSource != KindSourceRunner {
		t.Fatalf("manifest-declared plugin: %+v", got)
	}
}
