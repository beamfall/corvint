// SPDX-License-Identifier: Apache-2.0
package golang_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// gatedFixture is a module with a root-locating package that carries the
// declared gate, a package importing it, and an unrelated package.
func gatedFixture(t *testing.T, declaration string) string {
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
	if declaration != "" {
		files[affected.GatedRootLocatorsPath] = declaration
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
	return root
}

const gatedDeclaration = `{"profile":"corvint-gated-root-locators/0","packages":{"gated":{"gate":"/Library/Authority","reason":"root locating runs only under the gate"}}}` + "\n"

func gitIn(t *testing.T, root string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = root
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.test", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.test")
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", arguments, err, out)
	}
	return strings.TrimSpace(string(out))
}

// TestGatedLocatorChangesAt_TSP_V0_608 compares the declaration at the
// measured commit with the tree's: an added entry names its package, an
// unchanged declaration names nothing, an edited reason names the package,
// and an invalid side or an untouched path is unknown.
func TestGatedLocatorChangesAt_TSP_V0_608(t *testing.T) {
	root := gatedFixture(t, "")
	gitIn(t, root, "init", "-q")
	gitIn(t, root, "add", ".")
	gitIn(t, root, "commit", "-qm", "base")
	base := gitIn(t, root, "rev-parse", "HEAD")
	write := func(body string) {
		name := filepath.Join(root, filepath.FromSlash(affected.GatedRootLocatorsPath))
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	changed := []string{affected.GatedRootLocatorsPath}
	write(gatedDeclaration)
	if got := golang.GatedLocatorChangesAt(context.Background(), root, root, base, changed); !slices.Equal(got, []string{"gated"}) {
		t.Errorf("added entry: %v, want [gated]", got)
	}
	if got := golang.GatedLocatorChangesAt(context.Background(), root, root, base, []string{"other/other.go"}); got != nil {
		t.Errorf("untouched declaration: %v, want nil", got)
	}
	gitIn(t, root, "add", ".")
	gitIn(t, root, "commit", "-qm", "declare")
	declared := gitIn(t, root, "rev-parse", "HEAD")
	if got := golang.GatedLocatorChangesAt(context.Background(), root, root, declared, changed); got == nil || len(got) != 0 {
		t.Errorf("unchanged declaration: %v, want an empty, non-nil list", got)
	}
	write(strings.Replace(gatedDeclaration, "runs only under the gate", "runs only under the gate (reworded)", 1))
	if got := golang.GatedLocatorChangesAt(context.Background(), root, root, declared, changed); !slices.Equal(got, []string{"gated"}) {
		t.Errorf("edited reason: %v, want [gated]", got)
	}
	write(`{"profile":"corvint-gated-root-locators/0","packages":{"gated":{"gate":"relative","reason":"x"}}}`)
	if got := golang.GatedLocatorChangesAt(context.Background(), root, root, declared, changed); got != nil {
		t.Errorf("invalid head: %v, want nil (unknown)", got)
	}
	write(gatedDeclaration)
	if got := golang.GatedLocatorChangesAt(context.Background(), root, root, strings.Repeat("0", 40), changed); got != nil {
		t.Errorf("unreadable base: %v, want nil (unknown)", got)
	}
}

// TestGatedLocatorChangeSelection_TSP_V0_608 plans a dirty declaration over
// the planner: with the changed directories known, the gated package and its
// transitive importer are selected (GATED_LOCATOR_CHANGE, DEPENDENCY_PATH),
// the unrelated package is not, and the plan carries no
// go:gated-root-locators-changed entry. Without the comparison, with an
// invalid head declaration, or with a changed directory that holds no unit,
// the TSP-V0-003 unknown stays.
func TestGatedLocatorChangeSelection_TSP_V0_608(t *testing.T) {
	changedUnknown := func(plan affected.Plan) bool {
		return slices.ContainsFunc(plan.Unknown, func(unknown affected.Unknown) bool { return unknown.Detail == affected.FrontierGatedLocatorsChanged })
	}
	witness := func(plan affected.Plan) map[string]string {
		kinds := map[string]string{}
		for _, selection := range plan.Selected {
			kinds[selection.UnitID] = selection.Witness.Kind
		}
		return kinds
	}
	dirty := []string{affected.GatedRootLocatorsPath}
	graph, err := affected.Build(context.Background(), gatedFixture(t, gatedDeclaration), golang.New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph.WithGatedLocatorChanges([]string{"gated"}), dirty)
	if changedUnknown(plan) {
		t.Errorf("bounded edit keeps the unknown: %+v", plan.Unknown)
	}
	kinds := witness(plan)
	if kinds["go:example.test/m/gated"] != affected.WitnessGatedLocatorChange {
		t.Errorf("gated: %q, want %s (%v)", kinds["go:example.test/m/gated"], affected.WitnessGatedLocatorChange, kinds)
	}
	if kinds["go:example.test/m/dep"] != affected.WitnessDependency {
		t.Errorf("dep: %q, want %s", kinds["go:example.test/m/dep"], affected.WitnessDependency)
	}
	if _, selected := kinds["go:example.test/m/other"]; selected {
		t.Errorf("other is selected: %v", kinds)
	}
	if plan := affected.Select(graph, dirty); !changedUnknown(plan) {
		t.Errorf("without the comparison the edit must stay unknown: %+v", plan.Unknown)
	}
	// An edit that changed no entry is closed and seeds nothing: the gated
	// package keeps only its own unbounded-read selection (TSP-V0-003), and
	// its importer is not reached.
	if plan := affected.Select(graph.WithGatedLocatorChanges([]string{}), dirty); changedUnknown(plan) || witness(plan)["go:example.test/m/gated"] == affected.WitnessGatedLocatorChange || witness(plan)["go:example.test/m/dep"] != "" {
		t.Errorf("an edit that changed no entry seeds nothing and is closed: %+v %+v", plan.Unknown, witness(plan))
	}
	if plan := affected.Select(graph.WithGatedLocatorChanges([]string{"gated", "removed"}), dirty); !changedUnknown(plan) {
		t.Errorf("a changed directory without a unit must keep the unknown: %+v", plan.Unknown)
	}
	invalid, err := affected.Build(context.Background(), gatedFixture(t, `{"profile":"corvint-gated-root-locators/0","packages":{"other":{"gate":"/Library/Authority","reason":"not a locator"}}}`), golang.New())
	if err != nil {
		t.Fatal(err)
	}
	if plan := affected.Select(invalid.WithGatedLocatorChanges([]string{"other"}), dirty); !changedUnknown(plan) || !slices.Contains(invalid.Frontier(), golang.FrontierGatedLocatorsInvalid) {
		t.Errorf("an invalid head declaration must keep both unknowns: %+v frontier %v", plan.Unknown, invalid.Frontier())
	}
}
