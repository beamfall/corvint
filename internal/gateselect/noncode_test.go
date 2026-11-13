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

// NCM-V1-001 and the orchestrator ruling of 2026-11-11: an unowned non-code
// path no Go rule attributes runs everything, named unowned-path; one a Go
// package directory encloses (data beside a package, a package's fixtures)
// is owned and narrows to its attribution.
func TestNCMV1SelectorUnownedFallsBack(t *testing.T) {
	root := ncmFixture(t, map[string]string{"app/data.json": "{}\n", "user/user.go": "package user\n\nimport _ \"" + fixtureModule + "/app\"\n"})
	requireVerdict(t, ncmPlan([]string{"tools/notes.xyz"}, planUnknown{"UNOWNED_DIRTY_PATH", "tools/notes.xyz"}), root,
		"FALLBACK unowned-path tools/notes.xyz", "noncode unowned-path tools/notes.xyz")
	requireVerdict(t, ncmPlan([]string{"app/data.json"}, planUnknown{"UNOWNED_DIRTY_PATH", "app/data.json"}), root,
		"run "+fixtureModule+"/app "+fixtureModule+"/user")
	requireVerdict(t, ncmPlan([]string{"tests/fixtures/users.json"}, planUnknown{"UNOWNED_DIRTY_PATH", "tests/fixtures/users.json"}), root,
		"run "+fixtureModule+"/tests")
	// A declared Go reader the plan witnesses (AFP-V0-980) attributes a
	// deleted path outside every package; a witness of another kind does not.
	gone := ncmPlan([]string{"skills/x/SKILL.md"}, planUnknown{"UNOWNED_DIRTY_PATH", "skills/x/SKILL.md"}) //corvint:fixture-path
	gone.Provider.Go.Packages = []string{fixtureModule + "/app"}
	gone.Plan.Selected = append(gone.Plan.Selected, struct {
		UnitID  string `json:"unitId"`
		Witness struct {
			Kind      string `json:"kind"`
			DirtyPath string `json:"dirtyPath"`
		} `json:"witness"`
	}{UnitID: "go:" + fixtureModule + "/app"})
	gone.Plan.Selected[0].Witness.Kind, gone.Plan.Selected[0].Witness.DirtyPath = "PATH_LITERAL_READER", "skills/x/SKILL.md"
	requireVerdict(t, gone, root, "FALLBACK unowned-path skills/x/SKILL.md")
	gone.Plan.Selected[0].Witness.Kind = "DECLARED_EXISTENCE_READER"
	requireVerdict(t, gone, root, "run "+fixtureModule+"/app")
	for name, lines := range bothSelectors(ncmPlan([]string{"app/data.json"}, planUnknown{"UNOWNED_DIRTY_PATH", "app/data.json"}), root) {
		if slices.ContainsFunc(lines, func(line string) bool { return strings.HasPrefix(line, "noncode ") }) {
			t.Fatalf("%s printed a noncode line for an attributed path:\n%s", name, strings.Join(lines, "\n"))
		}
	}
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

// NCM-V1-001 under the orchestrator ruling of 2026-11-11: UNOWNED_DIRTY_PATH
// stays on the AFP-V0-982 allowlist that CI, `corvint test` and the
// line-evidence plan share, because the selector that holds the Go index runs
// everything for a dirty unowned path no Go rule attributes.
func TestNCMV1001UnownedAttributedOnAllowlist(t *testing.T) {
	if !AllowedUnknown("UNOWNED_DIRTY_PATH", "tools/notes.xyz") || !AllowedUnknown("NO_SELECTABLE_TEST", "go:x") || !AllowedUnknown("LANGUAGE_FRONTIER", "go:build-constraint-variants") {
		t.Fatal("AllowedUnknown dropped an entry NCM-V1 leaves admitted")
	}
	root := ncmFixture(t, nil)
	plan := ncmPlan([]string{"app/app.go", "outside.xyz"}, planUnknown{"UNOWNED_DIRTY_PATH", "outside.xyz"})
	if lines := Select(plan, fixtureModule, root); lines[len(lines)-1] != "FALLBACK unowned-path outside.xyz" {
		t.Fatalf("Select = %q", lines)
	}
}

// NCM-V1-005: a never-narrowed path runs everything whatever the map says.
func TestNCMV1SelectorNeverNarrowed(t *testing.T) {
	root := ncmFixture(t, map[string]string{
		".corvint/data-map.json": `{"profile":"corvint-data-map/0","entries":[{"path":"package-lock.json","tests":["go:app::TestLoad"],` +
			`"evidence":"e","accepted_by":"owner","accepted_at":"2026-11-11","pinned_commit":"0123456789abcdef0123456789abcdef01234567","proof":"FIH-NC-1"}]}`,
	})
	requireVerdict(t, ncmPlan([]string{"package-lock.json"}, planUnknown{"UNOWNED_DIRTY_PATH", "package-lock.json"}), root,
		"FALLBACK never-narrowed-path package-lock.json", "noncode never-narrowed-path package-lock.json")
	for _, p := range []string{".github/workflows/ci.yml", "app/testdata/out.golden", ".env.local", "rules.mk"} { //corvint:fixture-path
		requireVerdict(t, ncmPlan([]string{p}), root, "FALLBACK never-narrowed-path "+p)
	}
	// NCM-V1-010 (V1-5465): a Makefile edit still runs everything when the
	// plan names no base to attribute it against, and says so.
	requireVerdict(t, ncmPlan([]string{"Makefile"}), root, "FALLBACK never-narrowed-path Makefile: make edit: the plan's base commit is unknown")
}

// Review P2-1 under the orchestrator ruling of 2026-11-11: a non-Go file
// below an embedding package is attributed to the embedding package and its
// dependents, not a full run, and declared documentation there adds no full
// run either.
func TestNCMV1SelectorEmbeddedInputNarrowsToItsPackage(t *testing.T) {
	root := ncmFixture(t, map[string]string{
		"ui/ui.go":             "package ui\n\nimport _ \"embed\"\n\n//go:embed assets/app.css\nvar CSS string\n",
		"ui/assets/app.css":    "body{}\n",
		"ui/assets/notes.md":   "notes\n",
		"shell/shell.go":       "package shell\n\nimport _ \"" + fixtureModule + "/ui\"\n",
		documentationPathsPath: `{"profile":"corvint-documentation-paths/0","paths":["ui/assets/notes.md"]}` + "\n",
	})
	want := "run " + fixtureModule + "/shell " + fixtureModule + "/ui"
	requireVerdict(t, ncmPlan([]string{"ui/assets/app.css"}), root, want)
	requireVerdict(t, ncmPlan([]string{"ui/assets/app.css"}, planUnknown{"UNOWNED_DIRTY_PATH", "ui/assets/app.css"}), root, want)
	doc := ncmPlan([]string{"ui/assets/notes.md"})
	doc.Plan.NonSource = append(doc.Plan.NonSource, struct {
		Class string   `json:"class"`
		Paths []string `json:"paths"`
	}{"DECLARED_DOCUMENTATION", []string{"ui/assets/notes.md"}})
	for name, lines := range bothSelectors(doc, root) {
		if got := lines[len(lines)-1]; strings.HasPrefix(got, "FALLBACK ") || slices.ContainsFunc(lines, func(line string) bool { return strings.HasPrefix(line, "noncode ") }) {
			t.Fatalf("%s ran everything or printed a noncode line for declared documentation:\n%s", name, strings.Join(lines, "\n"))
		}
	}
}

// NCM-V1-006: Corvint configuration runs everything, the map file included;
// the orchestrator's call keeps change evidence configuration.
func TestNCMV1SelectorCorvintConfig(t *testing.T) {
	root := ncmFixture(t, map[string]string{".corvint/data-map.json": `{"profile":"corvint-data-map/0","entries":[]}`})
	for _, p := range []string{".corvint/data-map.json", ".corvint/test-read-scopes.json", ".corvint/runtime-readers.json", ".corvint/changes/x.cem.json", ".corvint/unplanned-reads.enabled"} { //corvint:fixture-path
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

	// Review P2-3: the deprecated flow file fails closed the same way.
	for _, p := range []string{".corvint/flows.json", ".corvint/e2e-flows.json"} { //corvint:fixture-path
		requireVerdict(t, ncmPlan([]string{p}), root, "FALLBACK flows-unreadable "+p)
	}
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
	stale := ncmFixture(t, map[string]string{"data/users.json": "[]\n", ".corvint/data-map.json": `{"profile":"corvint-data-map/0","entries":[{"path":"data/users.json",` +
		`"tests":["go:app::TestLoad","go:app::TestGone"],"evidence":"e","accepted_by":"owner","accepted_at":"2026-11-11",` +
		`"pinned_commit":"0123456789abcdef0123456789abcdef01234567","proof":"FIH-NC-1"}]}`})
	requireVerdict(t, ncmPlan([]string{"data/users.json"}, planUnknown{"UNOWNED_DIRTY_PATH", "data/users.json"}), stale,
		"FALLBACK unowned-path data/users.json",
		"data-map data/users.json stale: cited test go:app::TestGone is gone or cannot be confirmed")
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
// observation reports another language's source as UNOWNED_DIRTY_PATH, which
// the allowlist admits, so it runs nothing extra; an unowned path the receipt
// reports and no Go rule attributes runs everything; and one a Go package
// encloses narrows to it, with nothing set aside (review P3-2).
func TestNCMV1QualifiedKeepsThePlansOwnership(t *testing.T) {
	root := ncmFixture(t, map[string]string{"e2e/login.spec.ts": "test(\"login\", () => {})\n", "app/data.json": "{}\n"})
	plan := ncmPlan([]string{"app/app.go", "e2e/login.spec.ts"})
	plan.Provider.Go.Packages = []string{fixtureModule + "/app"}
	if lines := SelectQualified(plan, fixtureModule, root); lines[len(lines)-1] != "run "+fixtureModule+"/app" {
		t.Fatalf("a Go-only observation's unowned entry widened the run:\n%s", strings.Join(lines, "\n"))
	}
	owned := ncmPlan([]string{"app/app.go", "e2e/login.spec.ts"}, planUnknown{"UNOWNED_DIRTY_PATH", "e2e/login.spec.ts"})
	owned.Provider.Go.Packages = []string{fixtureModule + "/app"}
	if lines := SelectQualified(owned, fixtureModule, root); lines[len(lines)-1] != "FALLBACK unowned-path e2e/login.spec.ts" {
		t.Fatalf("the plan's unowned path did not run everything:\n%s", strings.Join(lines, "\n"))
	}
	enclosed := ncmPlan([]string{"app/data.json"}, planUnknown{"UNOWNED_DIRTY_PATH", "app/data.json"})
	enclosed.Provider.Go.Packages = []string{fixtureModule + "/app"}
	if lines := SelectQualified(enclosed, fixtureModule, root); lines[len(lines)-1] != "run "+fixtureModule+"/app" {
		t.Fatalf("an enclosed data file did not narrow to its package:\n%s", strings.Join(lines, "\n"))
	}
}

// Round-1 friction 6: SelectQualified keeps the NCM reason of a full run
// rather than replacing it with its generic input fallback; the run stays
// full either way.
func TestNCMV1QualifiedKeepsTheNamedReason(t *testing.T) {
	root := ncmFixture(t, nil)
	plan := ncmPlan([]string{".corvint/runtime-readers.json"}) //corvint:fixture-path
	plan.Plan.Scope = ""
	if lines := SelectQualified(plan, fixtureModule, root); lines[len(lines)-1] != "FALLBACK corvint-config .corvint/runtime-readers.json" {
		t.Fatalf("the NCM reason was replaced:\n%s", strings.Join(lines, "\n"))
	}
	plan.Plan.Dirty = []string{"app/app.go"}
	if lines := SelectQualified(plan, fixtureModule, root); lines[len(lines)-1] != "FALLBACK the selection input has no supported scope" {
		t.Fatalf("the generic input fallback is gone:\n%s", strings.Join(lines, "\n"))
	}
}
