package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// AFP-V0-162: a change to one package puts it in tier 0, its direct importer
// and a same-parent sibling dependent in tier 1, and a transitive dependent
// in tier 2, without changing the selection; the --full document is unchanged.
func TestAFPV0162AffectedTiersRankTheFullSelection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":              "module example.com/tiers\n\ngo 1.27.0\n",
		"a/core/core.go":      "package core\n\nfunc Value() int { return 1 }\n",
		"a/core/core_test.go": "package core\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) { _ = Value() }\n",
		"b/mid/mid.go":        "package mid\n\nimport \"example.com/tiers/a/core\"\n\nfunc Value() int { return core.Value() }\n",
		"b/mid/mid_test.go":   "package mid\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) { _ = Value() }\n",
		"c/leaf/leaf.go":      "package leaf\n\nimport \"example.com/tiers/b/mid\"\n\nfunc Value() int { return mid.Value() }\n",
		"c/leaf/leaf_test.go": "package leaf\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) { _ = Value() }\n",
		"a/peer/peer.go":      "package peer\n\nimport \"example.com/tiers/b/mid\"\n\nfunc Value() int { return mid.Value() }\n",
		"a/peer/peer_test.go": "package peer\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) { _ = Value() }\n",
	}
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"},
		{"config", "maintenance.auto", "false"}, {"config", "gc.auto", "0"}, {"add", "."}, {"commit", "-qm", "fixture"},
	} {
		affectedGit(t, root, arguments...)
	}
	appendFile(t, filepath.Join(root, "a", "core", "core.go"), "\n// edited\n")

	receipt, _, stderr, code := runAffectedCLI(t, root)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	tiers, ok := receipt["tiers"].(map[string]any)
	if !ok {
		t.Fatalf("default document has no tiers member: %v", receipt)
	}
	want := map[string]any{
		"go:example.com/tiers/a/core": float64(0),
		"go:example.com/tiers/b/mid":  float64(1),
		"go:example.com/tiers/a/peer": float64(1),
		"go:example.com/tiers/c/leaf": float64(2),
	}
	if !reflect.DeepEqual(tiers["units"], want) {
		t.Fatalf("tiers.units = %v, want %v", tiers["units"], want)
	}
	if !reflect.DeepEqual(tiers["counts"], []any{float64(1), float64(2), float64(1)}) || tiers["summary"] != "tier0=1 tier1=2 tier2=1" {
		t.Fatalf("tier counts %v summary %v", tiers["counts"], tiers["summary"])
	}
	selected := receipt["plan"].(map[string]any)["selected"].([]any)
	if len(selected) != len(want) {
		t.Fatalf("tiers must rank the whole selection: %d selected, %d tiered", len(selected), len(want))
	}
	full, _, stderr, code := runAffectedCLI(t, root, "--full")
	if code != 0 {
		t.Fatalf("--full exit %d: %s", code, stderr)
	}
	if _, present := full["tiers"]; present {
		t.Fatal("the affected-plan/0 --full document must stay unchanged")
	}
}

func TestAFPV0162EmptySelectionOmitsTiers(t *testing.T) {
	if tiers := tierAffectedSelections(nil); tiers != nil {
		t.Fatalf("empty selection tiers = %v, want nil", tiers)
	}
}
