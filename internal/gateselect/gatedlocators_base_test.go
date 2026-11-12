// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGatedLocatorChangeNarrowsTheProof_TSP_V0_608 commits a fixture without
// the gated root-locator declaration, adds it, and selects a plan the planner
// bounded (no go:gated-root-locators-changed entry, measured from the base).
// The selector's own Go proof mirrors the planner's immutable-base comparison,
// so the verdict runs the gated package and its dependent instead of falling
// back on the frontier its graph would otherwise re-raise. A plan that still
// names the frontier, as the planner emits for an unbounded edit, falls back.
// Since NCM-V1-006 (V1-5424) the declaration is Corvint configuration, so the
// selector runs everything for its edit before the proof narrows; the
// planner-named frontier keeps its module-level fallback, which comes first.
func TestGatedLocatorChangeNarrowsTheProof_TSP_V0_608(t *testing.T) {
	test := func(name string) string {
		return "package " + name + "\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n"
	}
	files := map[string]string{
		"go.mod":              "module " + fixtureModule + "\n\ngo 1.27.0\n",
		"loc/loc.go":          "package loc\n\nimport \"os\"\n\nconst Gate = \"/gate/root\"\n\nfunc Bind() string { dir, _ := os.Getwd(); return dir + Gate }\n",
		"loc/loc_test.go":     test("loc"),
		"dep/dep.go":          "package dep\n\nimport \"" + fixtureModule + "/loc\"\n\nfunc Use() string { return loc.Bind() }\n",
		"dep/dep_test.go":     test("dep"),
		"other/other.go":      "package other\n\nfunc Other() int { return 1 }\n",
		"other/other_test.go": test("other"),
	}
	root := writeFixture(t, files)
	fixtureGit(t, root, "init", "-q")
	fixtureGit(t, root, "add", "-A")
	fixtureGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "base")
	base := strings.TrimSpace(fixtureGit(t, root, "rev-parse", "HEAD"))
	declaration := filepath.Join(root, ".corvint", "gated-root-locators.json") //corvint:fixture-path
	if err := os.MkdirAll(filepath.Dir(declaration), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(declaration, []byte(`{"profile":"corvint-gated-root-locators/0","packages":{"loc":{"gate":"/gate/root","reason":"bound under the gate"}}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, root, "add", "-A")
	fixtureGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "declare")
	var plan Receipt
	plan.Provider.Go.State = "RUNNABLE"
	plan.Provider.Go.Packages = []string{fixtureModule + "/loc", fixtureModule + "/dep"}
	plan.Plan.Scope = "BOUNDED"
	plan.Plan.Dirty = []string{".corvint/gated-root-locators.json"} //corvint:fixture-path
	plan.Range.Base = base
	plan.Revision = strings.TrimSpace(fixtureGit(t, root, "rev-parse", "HEAD"))
	// The qualified proof is the tools/gate-affected-select path; verdict
	// alone runs the plain Select, which holds no proof.
	qualified := func() string {
		lines := SelectQualified(plan, fixtureModule, root)
		return lines[len(lines)-1]
	}
	// NCM-V1-006 (V1-5424): the declaration is Corvint configuration, so
	// the selector runs everything for its edit whatever the base, before
	// the proof could narrow it.
	const config = "FALLBACK corvint-config .corvint/gated-root-locators.json" //corvint:fixture-path
	if got := qualified(); got != config {
		t.Errorf("bounded declaration edit: verdict %q, want %q", got, config)
	}
	plan.Range.Base = ""
	if got := qualified(); got != config {
		t.Errorf("measured from the revision: verdict %q, want %q", got, config)
	}
	plan.Revision = "HEAD"
	if got := qualified(); got != config {
		t.Errorf("no measurable commit: verdict %q, want %q", got, config)
	}
	plan.Revision = strings.TrimSpace(fixtureGit(t, root, "rev-parse", "HEAD"))
	plan.Range.Base = base
	plan.Plan.Scope = "UNKNOWN"
	plan.Plan.Unknown = append(plan.Plan.Unknown, struct {
		Reason string `json:"reason"`
		Detail string `json:"detail"`
	}{"LANGUAGE_FRONTIER", "go:gated-root-locators-changed"})
	if got := qualified(); got != "FALLBACK Go frontier is unknown at module level: go:gated-root-locators-changed" {
		t.Errorf("planner-named frontier: verdict %q, want the module-level fallback", got)
	}
}
