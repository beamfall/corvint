package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/sessiontest"
)

// STR-V0-069 (V1-2988): a nested module that requires the root becomes the
// synthetic AFP-V0-582 unit, which the root module's `go list ./...` never
// lists. The root Go universe excludes it with a stated reason, so it equals
// that listing and the runner's universe check passes, and the verdict names
// the exclusion.
func TestSTRV0069NestedModuleRequiringTheRootLeavesTheGoUniverse(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		"go.mod":            "module example.test/root\n\ngo 1.26\n",
		"a/a.go":            "package a\n",
		"a/a_test.go":       "package a\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\n",
		"tools/auth/go.mod": "module example.test/root/tools/auth\n\ngo 1.26\n\nrequire example.test/root v0.0.0\n\nreplace example.test/root => ../..\n",
		"tools/auth/m.go":   "package main\n\nimport _ \"example.test/root/a\"\n\nfunc main() {}\n",
	} {
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	graph, err := affected.Build(context.Background(), root, affectedLanguages()...)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(graph.UnitIDs(), "go:example.test/root/tools/auth") {
		t.Fatalf("the fixture has no AFP-V0-582 unit: %q", graph.UnitIDs())
	}
	universe, excluded := testGoUniverse(graph)
	if len(excluded) != 1 || !strings.HasPrefix(excluded[0], "go:example.test/root/tools/auth: a dependent nested module") {
		t.Fatalf("excluded %q", excluded)
	}
	list := exec.Command("go", "list", "./...")
	list.Dir, list.Env = root, append(os.Environ(), "GOTOOLCHAIN=local", "GOFLAGS=", "GOWORK=off")
	listed, err := list.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	if !reflect.DeepEqual(universe, strings.Fields(string(listed))) {
		t.Fatalf("universe %q, go list ./... %q", universe, listed)
	}
	var stderr bytes.Buffer
	testVerdict(&stderr, sessiontest.Manifest{Scope: sessiontest.ScopeNothing, Excluded: excluded})
	if !strings.Contains(stderr.String(), "corvint-test: EXCLUDED go:example.test/root/tools/auth: ") {
		t.Fatalf("verdict %q", stderr.String())
	}
}
