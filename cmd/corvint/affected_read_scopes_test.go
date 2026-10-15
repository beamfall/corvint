package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

func writeAffectedFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for relative, body := range files {
		target := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// affectedSelectionsByWitness runs `corvint affected --full` with options and
// returns, sorted, every selected unit and those a declaration selected.
func affectedSelectionsByWitness(t *testing.T, root string, options ...string) (selected, declared []string) {
	t.Helper()
	receipt, _, stderr, code := runAffectedCLI(t, root, append([]string{"--full"}, options...)...)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	for _, item := range receipt["plan"].(map[string]any)["selected"].([]any) {
		selection := item.(map[string]any)
		id := selection["unitId"].(string)
		selected = append(selected, id)
		if selection["witness"].(map[string]any)["kind"] == affected.WitnessDeclaredReadScope {
			declared = append(declared, id)
		}
	}
	slices.Sort(selected)
	slices.Sort(declared)
	return selected, declared
}

// AFP-V0-050 (V1-1298): a committed change that adds a new package and only
// adds its key to the read-scope declaration selects that package, not every
// declared unit; a changed key still selects its unit.
func TestAffectedAdditiveReadScopeEditSelectsOnlyTheNewPackage_AFPV0050(t *testing.T) {
	t.Parallel()
	root := affectedFixtureRepository(t)
	writeAffectedFiles(t, root, map[string]string{
		affected.ReadScopesPath: `{"profile":"corvint-test-read-scopes/0","packages":{"core":["docs/"],"solo":["docs/"]}}` + "\n",
	})
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "declare")
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	writeAffectedFiles(t, root, map[string]string{
		"fresh/fresh.go":        "package fresh\n\n// Fresh is new.\nconst Fresh = 1\n",
		"fresh/fresh_test.go":   "package fresh\n",
		affected.ReadScopesPath: `{"profile":"corvint-test-read-scopes/0","packages":{"core":["docs/"],"fresh":["docs/"],"solo":["docs/"]}}` + "\n",
	})
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "add fresh")

	selected, declared := affectedSelectionsByWitness(t, root, "--base", base)
	if want := []string{"go:example.com/fixture/fresh"}; !slices.Equal(selected, want) || len(declared) != 0 {
		t.Fatalf("additive edit selected %v (declared %v), want %v", selected, declared, want)
	}

	writeAffectedFiles(t, root, map[string]string{
		affected.ReadScopesPath: `{"profile":"corvint-test-read-scopes/0","packages":{"core":["docs/"],"fresh":["docs/"],"solo":["spec/"]}}` + "\n",
	})
	_, declared = affectedSelectionsByWitness(t, root, "--base", base)
	if want := []string{"go:example.com/fixture/solo"}; !slices.Equal(declared, want) {
		t.Fatalf("changed key selected %v by declaration, want %v", declared, want)
	}
}
