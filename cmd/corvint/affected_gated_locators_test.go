// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/plansnapshot"
)

// gatedLocatorRepository is a committed module with a root-locating package
// carrying the gate literal, a package importing it and an unrelated package.
func gatedLocatorRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":              "module example.test/m\n\ngo 1.27.0\n",
		"gated/gated.go":      "package gated\n\nimport \"os\"\n\nconst gate = \"/Library/Authority\"\n\nfunc Root() string { wd, _ := os.Getwd(); return wd + gate }\n",
		"gated/gated_test.go": "package gated\n\nimport \"testing\"\n\nfunc TestRoot(t *testing.T) { _ = Root() }\n",
		"dep/dep.go":          "package dep\n\nimport \"example.test/m/gated\"\n\nfunc Use() string { return gated.Root() }\n",
		"dep/dep_test.go":     "package dep\n\nimport \"testing\"\n\nfunc TestUse(t *testing.T) { _ = Use() }\n",
		"other/other.go":      "package other\n\nfunc Other() int { return 1 }\n",
		"other/other_test.go": "package other\n\nimport \"testing\"\n\nfunc TestOther(t *testing.T) { _ = Other() }\n",
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
		{"config", "maintenance.auto", "false"}, {"config", "gc.auto", "0"},
		{"add", "."}, {"commit", "-qm", "fixture"},
	} {
		affectedGit(t, root, arguments...)
	}
	return root
}

// TestAffectedSnapshotBoundsGatedLocatorChange_TSP_V0_609 drives the two
// planning tails over a change that adds the gating declaration. The
// --snapshot plan, pinned to the base before the declaration, selects the
// gated package (GATED_LOCATOR_CHANGE) and its dependent (DEPENDENCY_PATH),
// leaves the unrelated package out and names no go:gated-root-locators-changed
// entry; the --base plan over the same commits agrees. The --path plan, which
// measures from no commit, keeps the entry.
func TestAffectedSnapshotBoundsGatedLocatorChange_TSP_V0_609(t *testing.T) {
	root := gatedLocatorRepository(t)
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	declaration := filepath.Join(root, filepath.FromSlash(affected.GatedRootLocatorsPath))
	if err := os.MkdirAll(filepath.Dir(declaration), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"profile":"corvint-gated-root-locators/0","packages":{"gated":{"gate":"/Library/Authority","reason":"root locating runs only under the gate"}}}` + "\n"
	if err := os.WriteFile(declaration, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "declare")
	paths := []string{affected.GatedRootLocatorsPath}
	receipt := plansnapshot.Receipt{Schema: plansnapshot.Schema, Base: base, Commit: strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD")), Tree: strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD^{tree}")), Paths: paths, Digest: plansnapshot.PathDigest(paths)}
	raw, _ := json.Marshal(receipt)
	name := filepath.Join(t.TempDir(), "receipt.json")
	if err := os.WriteFile(name, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	kinds := func(plan affected.Plan) map[string]string {
		got := map[string]string{}
		for _, selection := range plan.Selected {
			got[selection.UnitID] = selection.Witness.Kind
		}
		return got
	}
	changedUnknown := func(plan affected.Plan) bool {
		for _, unknown := range plan.Unknown {
			if unknown.Detail == affected.FrontierGatedLocatorsChanged {
				return true
			}
		}
		return false
	}
	bounded := func(label string, plan affected.Plan) {
		t.Helper()
		got := kinds(plan)
		if changedUnknown(plan) {
			t.Errorf("%s: the bounded declaration edit still names %s: %+v", label, affected.FrontierGatedLocatorsChanged, plan.Unknown)
		}
		if got["go:example.test/m/gated"] != affected.WitnessGatedLocatorChange {
			t.Errorf("%s: gated selected as %q, want %s (%v)", label, got["go:example.test/m/gated"], affected.WitnessGatedLocatorChange, got)
		}
		if got["go:example.test/m/dep"] != affected.WitnessDependency {
			t.Errorf("%s: dep selected as %q, want %s", label, got["go:example.test/m/dep"], affected.WitnessDependency)
		}
		if kind, hit := got["go:example.test/m/other"]; hit {
			t.Errorf("%s: other selected as %s; its entry did not change", label, kind)
		}
	}
	snapshot, err := compileSnapshotAffected(context.Background(), affectedInvocation{Root: root, Snapshot: name})
	if err != nil {
		t.Fatal(err)
	}
	bounded("--snapshot", snapshot.(affectedReceipt).Plan)
	compact, _, stderrText, code := runAffectedCLI(t, root, "--snapshot", name)
	if code != 0 {
		t.Fatalf("--snapshot exited %d: %s", code, stderrText)
	}
	// The bare fixture declares no owner for the declaration path, so the CLI
	// plan may still name that path unowned; the gated-locator entry itself
	// must be gone.
	if unknowns, _ := compact["plan"].(map[string]any)["unknown"].([]any); true {
		for _, entry := range unknowns {
			if detail, _ := entry.(map[string]any)["detail"].(string); detail == affected.FrontierGatedLocatorsChanged {
				t.Errorf("--snapshot CLI plan still names %s: %v", detail, unknowns)
			}
		}
	}
	measured, err := compileAffected(context.Background(), affectedInvocation{Root: root, Base: base})
	if err != nil {
		t.Fatal(err)
	}
	bounded("--base", measured.Plan)
	byPath, err := compileAffected(context.Background(), affectedInvocation{Root: root, PathList: paths})
	if err != nil {
		t.Fatal(err)
	}
	if plan := byPath.Plan; !changedUnknown(plan) {
		t.Errorf("--path measures from no commit and must keep %s: %+v", affected.FrontierGatedLocatorsChanged, plan.Unknown)
	}
}
