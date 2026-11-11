// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"slices"
	"strings"
	"testing"
)

type planUnknown = struct {
	Reason string `json:"reason"`
	Detail string `json:"detail"`
}

func ncmFixture(t *testing.T, extra map[string]string) string {
	files := map[string]string{
		"go.mod":                    "module " + fixtureModule + "\n\ngo 1.27\n",
		"app/app.go":                "package app\n",
		"app/app_test.go":           "package app\n\nimport \"testing\"\n\nfunc TestLoad(t *testing.T) {}\n",
		"app/testdata/seed.txt":     "seed\n",
		"tests/tests_test.go":       "package tests\n\nvar users = \"fixtures/users.json\"\n",
		"tests/fixtures/users.json": "[]\n",
		"web/vitest.config.ts":      "export default {}\n",
	}
	for name, body := range extra {
		files[name] = body
	}
	return writeFixture(t, files)
}

func ncmPlan(dirty []string, unknown ...planUnknown) Receipt {
	var plan Receipt
	plan.Provider.Go.State = "RUNNABLE"
	plan.Plan.Scope = "BOUNDED"
	if len(unknown) != 0 {
		plan.Plan.Scope = "UNKNOWN"
	}
	plan.Plan.Dirty = dirty
	plan.Plan.Unknown = unknown
	return plan
}

// bothSelectors returns the Select and SelectStructural lines.
func bothSelectors(plan Receipt, root string) map[string][]string {
	return map[string][]string{
		"Select":           Select(plan, fixtureModule, root),
		"SelectStructural": SelectStructural(plan, fixtureModule, root),
	}
}

func requireVerdict(t *testing.T, plan Receipt, root, want string, audit ...string) {
	t.Helper()
	for name, lines := range bothSelectors(plan, root) {
		if got := lines[len(lines)-1]; got != want {
			t.Fatalf("%s verdict = %q, want %q:\n%s", name, got, want, strings.Join(lines, "\n"))
		}
		for _, line := range audit {
			if !slices.Contains(lines, line) {
				t.Fatalf("%s lacks %q:\n%s", name, line, strings.Join(lines, "\n"))
			}
		}
	}
}

// NCM-V1-001: an unowned non-code path runs everything, named
// UNOWNED_DIRTY_PATH when no runner's tree holds it and non-code-unmapped
// when a Go package directory encloses it; the scenario paths of the spec.
func TestNCMV1SelectorUnownedFallsBack(t *testing.T) {
	root := ncmFixture(t, nil)
	requireVerdict(t, ncmPlan([]string{"tools/notes.xyz"}, planUnknown{"UNOWNED_DIRTY_PATH", "tools/notes.xyz"}), root,
		"FALLBACK UNOWNED_DIRTY_PATH tools/notes.xyz", "noncode UNOWNED_DIRTY_PATH tools/notes.xyz")
	requireVerdict(t, ncmPlan([]string{"tests/fixtures/users.json"}, planUnknown{"UNOWNED_DIRTY_PATH", "tests/fixtures/users.json"}), root,
		"FALLBACK non-code-unmapped tests/fixtures/users.json", "noncode non-code-unmapped tests/fixtures/users.json")
	// A non-code path whose readers the plan proves (no UNOWNED entry) keeps
	// the AFP-V0-012 attribution.
	requireVerdict(t, ncmPlan([]string{"app/testdata/seed.txt"}), root, "run "+fixtureModule+"/app")
	// Declared documentation adds no test: the documentation line, no
	// noncode line, and the empty-selection rule of a documentation-only
	// change is the selector's own.
	doc := ncmPlan([]string{"docs/guide.md"})
	doc.Plan.NonSource = append(doc.Plan.NonSource, struct {
		Class string   `json:"class"`
		Paths []string `json:"paths"`
	}{"DECLARED_DOCUMENTATION", []string{"docs/guide.md"}})
	for name, lines := range bothSelectors(doc, root) {
		if slices.ContainsFunc(lines, func(line string) bool { return strings.HasPrefix(line, "noncode ") }) {
			t.Fatalf("%s printed a noncode line for declared documentation:\n%s", name, strings.Join(lines, "\n"))
		}
	}
}

// NCM-V1-001 (R32): an unavailable runner and a deleted runner configuration
// run everything, named runner-removed.
func TestNCMV1SelectorUnavailableRunner(t *testing.T) {
	root := ncmFixture(t, nil)
	plan := ncmPlan([]string{"app/app.go"})
	plan.Provider.Go.State = "UNAVAILABLE"
	requireVerdict(t, plan, root, "FALLBACK runner-removed go: provider.go.state is UNAVAILABLE")
	removed := ncmPlan([]string{"web/jest.config.js"})
	removed.Plan.Deleted = []string{"web/jest.config.js"}
	requireVerdict(t, removed, root, "FALLBACK runner-removed web/jest.config.js")
	// Present runner configuration is a never-narrowed path instead.
	requireVerdict(t, ncmPlan([]string{"web/vitest.config.ts"}), root, "FALLBACK never-narrowed-path web/vitest.config.ts")
}

// NCM-V1-001 (R32): UNOWNED_DIRTY_PATH is off the AFP-V0-982 allowlist that
// CI, `corvint test` and the line-evidence plan share.
func TestNCMV1001UnownedOffAllowlist(t *testing.T) {
	if AllowedUnknown("UNOWNED_DIRTY_PATH", "tools/notes.xyz") {
		t.Fatal("AllowedUnknown admits UNOWNED_DIRTY_PATH")
	}
	if !AllowedUnknown("NO_SELECTABLE_TEST", "go:x") || !AllowedUnknown("LANGUAGE_FRONTIER", "go:build-constraint-variants") {
		t.Fatal("AllowedUnknown dropped an entry NCM-V1 leaves admitted")
	}
	// An unowned entry whose path is not dirty still falls back, named.
	root := ncmFixture(t, nil)
	plan := ncmPlan([]string{"app/app.go"}, planUnknown{"UNOWNED_DIRTY_PATH", "tools/elsewhere.xyz"})
	if lines := Select(plan, fixtureModule, root); lines[len(lines)-1] != "FALLBACK UNOWNED_DIRTY_PATH tools/elsewhere.xyz" {
		t.Fatalf("Select = %q", lines)
	}
}

// NCM-V1-005: a never-narrowed path runs everything whatever the map says,
// and a go:embed input of an embedding package is one.
func TestNCMV1SelectorNeverNarrowed(t *testing.T) {
	root := ncmFixture(t, map[string]string{
		"ui/ui.go":          "package ui\n\nimport _ \"embed\"\n\n//go:embed assets/app.css\nvar CSS string\n",
		"ui/assets/app.css": "body{}\n",
		".corvint/data-map.json": `{"profile":"corvint-data-map/0","entries":[{"path":"package-lock.json","tests":["go:app::TestLoad"],` +
			`"evidence":"e","accepted_by":"owner","accepted_at":"2026-11-11","pinned_commit":"0123456789abcdef0123456789abcdef01234567","proof":"FIH-NC-1"}]}`,
	})
	requireVerdict(t, ncmPlan([]string{"package-lock.json"}, planUnknown{"UNOWNED_DIRTY_PATH", "package-lock.json"}), root,
		"FALLBACK never-narrowed-path package-lock.json", "noncode never-narrowed-path package-lock.json")
	for _, p := range []string{".github/workflows/ci.yml", "Makefile", "app/testdata/out.golden", ".env.local", "ui/assets/app.css"} {
		requireVerdict(t, ncmPlan([]string{p}), root, "FALLBACK never-narrowed-path "+p)
	}
}

// NCM-V1-006: Corvint configuration runs everything, the map file included;
// the orchestrator's call keeps change evidence configuration.
func TestNCMV1SelectorCorvintConfig(t *testing.T) {
	root := ncmFixture(t, map[string]string{".corvint/data-map.json": `{"profile":"corvint-data-map/0","entries":[]}`})
	for _, p := range []string{".corvint/data-map.json", ".corvint/test-read-scopes.json", ".corvint/runtime-readers.json", ".corvint/changes/x.cem.json", ".corvint/unplanned-reads.enabled"} {
		requireVerdict(t, ncmPlan([]string{p}), root, "FALLBACK corvint-config "+p, "noncode corvint-config "+p)
	}
}

// NCM-V1-006: a ledger append adds nothing, a flow file edit adds no full
// run while the plan's flow join can compute the changed flows, and runs
// everything, named, when it cannot.
func TestNCMV1SelectorLedgerAndFlow(t *testing.T) {
	root := ncmFixture(t, nil)
	ledger := ncmPlan([]string{".corvint/self-observations.jsonl", "app/app.go"})
	requireVerdict(t, ledger, root, "run "+fixtureModule+"/app", "noncode private-ledger .corvint/self-observations.jsonl")

	flows := ncmPlan([]string{".corvint/flows.json", "app/app.go"})
	flows.Plan.E2E = &struct {
		Flows   string `json:"flows"`
		Unknown []struct {
			Reason string `json:"reason"`
			Detail string `json:"detail"`
		} `json:"unknown"`
	}{Flows: "flows"}
	requireVerdict(t, flows, root, "run "+fixtureModule+"/app", "noncode flow-linked .corvint/flows.json")

	flows.Plan.E2E.Unknown = append(flows.Plan.E2E.Unknown, planUnknown{"FLOWS_DECLARATION_INVALID", ".corvint/flows.json"})
	requireVerdict(t, flows, root, "FALLBACK flows-unreadable .corvint/flows.json")

	unjoined := ncmPlan([]string{".corvint/flows.json"})
	requireVerdict(t, unjoined, root, "FALLBACK flows-unreadable .corvint/flows.json")
}

// NCM-V1-007: an invalid map is refused, never read as empty; NCM-V1-008:
// a stale entry is reported and narrows nothing.
func TestNCMV1SelectorInvalidMap(t *testing.T) {
	root := ncmFixture(t, map[string]string{".corvint/data-map.json": `{"profile":"corvint-data-map/1","entries":[]}`})
	for name, lines := range bothSelectors(ncmPlan([]string{"app/app.go"}), root) {
		if got := lines[len(lines)-1]; !strings.HasPrefix(got, "FALLBACK data-map-invalid: .corvint/data-map.json profile \"corvint-data-map/1\"") {
			t.Fatalf("%s verdict = %q, want the data-map-invalid fallback", name, got)
		}
	}
	stale := ncmFixture(t, map[string]string{".corvint/data-map.json": `{"profile":"corvint-data-map/0","entries":[{"path":"tests/fixtures/users.json",` +
		`"tests":["go:app::TestLoad","go:app::TestGone"],"evidence":"e","accepted_by":"owner","accepted_at":"2026-11-11",` +
		`"pinned_commit":"0123456789abcdef0123456789abcdef01234567","proof":"FIH-NC-1"}]}`})
	requireVerdict(t, ncmPlan([]string{"tests/fixtures/users.json"}, planUnknown{"UNOWNED_DIRTY_PATH", "tests/fixtures/users.json"}), stale,
		"FALLBACK non-code-unmapped tests/fixtures/users.json",
		"data-map tests/fixtures/users.json stale: cited test go:app::TestGone is gone or cannot be confirmed")
}

// NCM-V1-008: the selector confirms a cited Go test by its package and its
// declaration, and another runner's test by its file.
func TestNCMV1SelectorConfirmsCitedTests(t *testing.T) {
	root := ncmFixture(t, nil)
	index, err := indexRepository(root, fixtureModule)
	if err != nil {
		t.Fatal(err)
	}
	for test, want := range map[string]bool{
		"go:app":                      true,
		"go:app::TestLoad":            true,
		"go:app::TestLoad::sub":       true,
		"go:app::TestGone":            false,
		"go:missing::TestLoad":        false,
		"vitest:web/vitest.config.ts": true,
		"vitest:web/gone.test.ts":     false,
		"vitest:../escape.ts":         false,
		"no-runner":                   false,
	} {
		if got := index.testExists(root, test); got != want {
			t.Errorf("testExists(%q) = %v, want %v", test, got, want)
		}
	}
}

// NCM-V1-001: ownership is the full plan's. The qualified selector's Go-only
// observation reports another language's source as UNOWNED_DIRTY_PATH; that
// alone runs nothing extra, while an unowned path the receipt reports runs
// everything.
func TestNCMV1QualifiedKeepsThePlansOwnership(t *testing.T) {
	root := ncmFixture(t, map[string]string{"e2e/login.spec.ts": "test(\"login\", () => {})\n"})
	plan := ncmPlan([]string{"app/app.go", "e2e/login.spec.ts"})
	plan.Provider.Go.Packages = []string{fixtureModule + "/app"}
	if lines := SelectQualified(plan, fixtureModule, root); lines[len(lines)-1] != "run "+fixtureModule+"/app" {
		t.Fatalf("a Go-only observation's unowned entry widened the run:\n%s", strings.Join(lines, "\n"))
	}
	owned := ncmPlan([]string{"app/app.go", "e2e/login.spec.ts"}, planUnknown{"UNOWNED_DIRTY_PATH", "e2e/login.spec.ts"})
	owned.Provider.Go.Packages = []string{fixtureModule + "/app"}
	if lines := SelectQualified(owned, fixtureModule, root); lines[len(lines)-1] != "FALLBACK UNOWNED_DIRTY_PATH e2e/login.spec.ts" {
		t.Fatalf("the plan's unowned path did not run everything:\n%s", strings.Join(lines, "\n"))
	}
}
