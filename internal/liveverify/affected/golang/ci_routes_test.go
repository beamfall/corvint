// SPDX-License-Identifier: Apache-2.0
package golang_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// CIN-V0-7505/7506: real declaration + minimal source graph proves workflow and catalog routes.
func TestCINV07505RepositoryCIRoutes(t *testing.T) {
	repo := filepath.Join("..", "..", "..", "..")
	data, err := os.ReadFile(filepath.Join(repo, affected.ReadScopesPath))
	if err != nil {
		t.Fatal(err)
	}
	var declaration struct {
		Packages map[string][]string `json:"packages"`
	}
	if err := json.Unmarshal(data, &declaration); err != nil {
		t.Fatal(err)
	}
	packages := map[string][]string{}
	for _, pkg := range []string{"tools/ci-forward-row", "conformance/cli-errors-v0"} {
		packages[pkg] = declaration.Packages[pkg]
	}
	scoped, _ := json.Marshal(map[string]any{"profile": "corvint-test-read-scopes/0", "packages": packages})
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                                    "module example.test/ci\n\ngo 1.27.3\n",
		affected.ReadScopesPath:                     string(scoped),
		"tools/ci-forward-row/guard_test.go":        "package guard\nimport \"testing\"\nfunc TestGuard(t *testing.T) {}\n",
		"conformance/cli-errors-v0/catalog_test.go": "package catalog\nimport \"testing\"\nfunc TestCatalog(t *testing.T) {}\n",
		".github/workflows/ci.yml":                  "jobs: {}\n",
		"cmd/new-binary/main.go":                    "package main\nfunc main() {}\n",
		"internal/capcatalog/trees/new-binary.json": "{}\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ dirty, test string }{
		{".github/workflows/ci.yml", "tools/ci-forward-row/guard_test.go"},
		{"cmd/new-binary/main.go", "conformance/cli-errors-v0/catalog_test.go"},
		{"internal/capcatalog/trees/new-binary.json", "conformance/cli-errors-v0/catalog_test.go"},
	} {
		if p := affected.Select(graph, []string{tc.dirty}); !slices.Contains(p.SelectedTests(), tc.test) {
			t.Fatalf("%s: missing %s: %+v", tc.dirty, tc.test, p)
		}
	}
	batch, err := os.ReadFile(filepath.Join(repo, "script/batch-precheck.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []string{"./tools/ci-forward-row Test .github/", "./conformance/cli-errors-v0 TestCLIErrorContracts_ACXV0859 @own cmd/ internal/capcatalog/"} {
		if !strings.Contains(string(batch), row) {
			t.Fatalf("batch verification lacks %s", row)
		}
	}
	targets := regexp.MustCompile(`(?m)^EXTRA_TARGETS=\(([^\n]*)\)`).FindStringSubmatch(string(batch))
	if targets == nil || !slices.Contains(strings.Fields(targets[1]), "capability-catalog-test") {
		t.Fatal("catalog shell fixture absent from batch verification")
	}
}
