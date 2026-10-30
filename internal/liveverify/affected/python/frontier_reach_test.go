// SPDX-License-Identifier: Apache-2.0
package python

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AFP-V0-511: every Python reason the plugin raises for a module it can read binds another
// module, so a dirty path no plugin owns raises none of them, and the plugin reads no manifest
// or configuration file that could steer its resolution.
func TestFrontierReachHoldsForUnownedPaths_AFPV0511(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/fixture\n\ngo 1.22\n")
	write(t, root, "pkg/pkg.go", "package pkg\n")
	write(t, root, "pkg/pkg_test.go", "package pkg\n")
	write(t, root, "app/__init__.py", "")
	write(t, root, "app/loader.py", "import importlib\n")
	write(t, root, "tests/test_loader.py", "import app.loader\n")
	before, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(before.Frontier) != fmt.Sprint([]string{FrontierDynamicImport}) {
		t.Fatalf("plugin frontier = %v", before.Frontier)
	}
	write(t, root, "Makefile", "all:\n\tpytest\n")
	write(t, root, "pyproject.toml", "[tool.pytest.ini_options]\npythonpath = [\"src\"]\n")
	write(t, root, "pkg/testdata/cases.json", `[{"name": "a"}]`)
	after, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("unowned paths changed the observation:\n%+v\n%+v", before, after)
	}
	for _, input := range []string{"Makefile", "pyproject.toml", "pkg/testdata/cases.json"} {
		if New().ResolutionInput(input) {
			t.Errorf("%s reported as a resolution input", input)
		}
	}
	graph, err := affected.Build(context.Background(), root, golang.New(), New())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		dirty string
		want  []string
	}{
		{"Makefile", []string{}},
		{"pyproject.toml", []string{}},
		{"pkg/testdata/cases.json", []string{}},
		{"app/loader.py", []string{FrontierDynamicImport}},
	} {
		plan := affected.Select(graph, []string{tc.dirty})
		if got := languageFrontier(plan); fmt.Sprint(got) != fmt.Sprint(tc.want) {
			t.Errorf("dirty %s: frontier %v, want %v", tc.dirty, got, tc.want)
		}
	}
}

func languageFrontier(plan affected.Plan) []string {
	details := []string{}
	for _, unknown := range plan.Unknown {
		if unknown.Reason == affected.UnknownLanguageFrontier {
			details = append(details, unknown.Detail)
		}
	}
	return details
}
