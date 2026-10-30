package gateselect

import (
	"slices"
	"strings"
	"testing"
)

// structuralVerdict is verdict for SelectStructural, the `corvint test` form.
func structuralVerdict(plan Receipt, root string) (string, []string) {
	lines := selectPackages(plan, fixtureModule, root, false)
	return lines[len(lines)-1], lines
}

func narrowingFixture(t *testing.T) string {
	return writeFixture(t, map[string]string{
		"core/core.go":                      "package core\n\nfunc Old() int { return 1 }\n",
		"core/fresh.go":                     "package core\n\nfunc Fresh() int { return 2 }\n",
		"ref/ref.go":                        "package ref\n\nimport \"example.com/fixture/core\"\n\nvar _ = core.Fresh()\n",
		"unref/unref.go":                    "package unref\n\nimport \"example.com/fixture/core\"\n\nvar _ = core.Old()\n",
		"leaf/leaf.go":                      "package leaf\n\nimport _ \"example.com/fixture/unref\"\n",
		"named/named_test.go":               "package named\n\nvar fixture = \"INDEX.json\"\n",
		"reader/reader_test.go":             "package reader\n\nvar index = \"../docs/specs/INDEX.json\"\n",
		"docs/specs/INDEX.json":             "{}\n",
		"docs/specs/unrelated.md":           "x\n",
		".corvint/documentation-paths.json": `{"profile":"corvint-documentation-paths/0","paths":["docs/"]}`,
	})
}

// TestSelectStructuralHonorsAdditiveNarrowing_ADNV0006 replays ADN-V0-006: a
// NARROWED record drops its unreferenced importers, and what only they reach,
// from the `corvint test` attribution, while Select, the CI PR tier's frozen
// selector, keeps every importer.
func TestSelectStructuralHonorsAdditiveNarrowing_ADNV0006(t *testing.T) {
	root := narrowingFixture(t)
	plan := func(state string) Receipt {
		var plan Receipt
		plan.Provider.Go.State = "RUNNABLE"
		plan.Provider.Go.Packages = []string{"example.com/fixture/core", "example.com/fixture/ref"}
		plan.Plan.Dirty = []string{"core/fresh.go"}
		plan.Plan.GoAdditive = append(plan.Plan.GoAdditive, struct {
			Unit         string   `json:"unit"`
			State        string   `json:"state"`
			Paths        []string `json:"paths"`
			Unreferenced []string `json:"unreferenced"`
		}{Unit: "go:example.com/fixture/core", State: state, Paths: []string{"core/fresh.go"}, Unreferenced: []string{"go:example.com/fixture/unref"}})
		return plan
	}
	whole := "run example.com/fixture/core example.com/fixture/leaf example.com/fixture/ref example.com/fixture/unref"
	if got, _ := structuralVerdict(plan("NARROWED"), root); got != "run example.com/fixture/core example.com/fixture/ref" {
		t.Fatalf("narrowed structural verdict = %q", got)
	}
	if got, _ := structuralVerdict(plan("FALLBACK"), root); got != whole {
		t.Fatalf("fallback structural verdict = %q, want %q", got, whole)
	}
	if got := verdict(plan("NARROWED"), root); got != whole {
		t.Fatalf("Select verdict = %q, want %q", got, whole)
	}
}

// TestSelectStructuralTakesDocumentationReadersFromThePlan_ADNV0010 replays
// ADN-V0-010: for a path the plan classifies DECLARED_DOCUMENTATION, `corvint
// test` adds no rule (c) reader of its own (a lone file-name token in a
// fixture no longer selects its holder), keeps the plan's own reader, and
// prints a documentation line; without the classification, with a
// path-token-bound frontier, or in Select, rule (c) stays whole.
func TestSelectStructuralTakesDocumentationReadersFromThePlan_ADNV0010(t *testing.T) {
	root := narrowingFixture(t)
	plan := func(classified bool, unknown string) Receipt {
		var plan Receipt
		plan.Provider.Go.State = "RUNNABLE"
		plan.Provider.Go.Packages = []string{"example.com/fixture/reader"}
		plan.Plan.Dirty = []string{"docs/specs/INDEX.json"}
		if classified {
			plan.Plan.NonSource = append(plan.Plan.NonSource, struct {
				Class string   `json:"class"`
				Paths []string `json:"paths"`
			}{Class: "DECLARED_DOCUMENTATION", Paths: []string{"docs/specs/INDEX.json"}})
		}
		if unknown != "" {
			plan.Plan.Unknown = append(plan.Plan.Unknown, struct {
				Reason string `json:"reason"`
				Detail string `json:"detail"`
			}{Reason: "LANGUAGE_FRONTIER", Detail: unknown})
		}
		return plan
	}
	whole := "run example.com/fixture/named example.com/fixture/reader"
	got, lines := structuralVerdict(plan(true, ""), root)
	if got != "run example.com/fixture/reader" || !slices.Contains(lines, "documentation docs/specs/INDEX.json") {
		t.Fatalf("documented structural selection = %q", strings.Join(lines, "\n"))
	}
	if got, _ := structuralVerdict(plan(false, ""), root); got != whole {
		t.Fatalf("unclassified structural verdict = %q, want %q", got, whole)
	}
	if got, _ := structuralVerdict(plan(true, "go:path-token-bound:go:example.com/fixture/named"), root); got != whole {
		t.Fatalf("token-bound structural verdict = %q, want %q", got, whole)
	}
	if got := verdict(plan(true, ""), root); got != whole {
		t.Fatalf("Select verdict = %q, want %q", got, whole)
	}
	stale := plan(true, "")
	stale.Provider.Go.Packages = []string{"example.com/fixture/named"}
	if got, lines := structuralVerdict(stale, root); got != whole || slices.Contains(lines, "documentation docs/specs/INDEX.json") {
		t.Fatalf("a plan without the resolving reader narrowed rule (c): %q", strings.Join(lines, "\n"))
	}
	forged := plan(true, "")
	forged.Plan.Dirty = []string{"docs/specs/unrelated.md"}
	forged.Plan.NonSource[0].Paths = []string{"docs/specs/unrelated.md", "named/named_test.go"}
	if _, lines := structuralVerdict(forged, root); slices.ContainsFunc(lines, func(line string) bool { return line == "documentation named/named_test.go" }) {
		t.Fatalf("an undeclared path took the plan's readers: %q", strings.Join(lines, "\n"))
	}
}

// TestSelectStructuralTakesSealedEvidenceReadersFromThePlan_ADNV0011 replays
// ADN-V0-011 (V1-3138): for a sealed CEM directly under .corvint/changes/,
// `corvint test` adds no rule (c) reader of its own, keeps the plan's reader
// that enumerates the directory, and prints an evidence line; the CEM
// sidecar, a path-token-bound frontier and Select keep rule (c) whole. Since
// AFP-V0-931 (V1-3146) that whole rule (c) no longer composes a `.corvint`
// token with a bare "/" (loose), only with a parent-only token (climb).
func TestSelectStructuralTakesSealedEvidenceReadersFromThePlan_ADNV0011(t *testing.T) {
	const sealed = ".corvint/changes/0123456789abcdef0123456789abcdef01234567.cem.json"
	root := writeFixture(t, map[string]string{
		"enum/enum_test.go":        "package enum\n\nvar sealed = \"../.corvint/changes\"\n",
		"loose/loose.go":           "package loose\n\nvar sep, base = \"/\", \".corvint\"\n",
		"loose/loose_test.go":      "package loose\n",
		"climb/climb.go":           "package climb\n\nvar up, base = \"..\", \".corvint\"\n",
		"climb/climb_test.go":      "package climb\n",
		sealed:                     "{}\n",
		".corvint/change.cem.json": "{}\n",
	})
	plan := func(dirty, unknown string) Receipt {
		var plan Receipt
		plan.Provider.Go.State = "RUNNABLE"
		plan.Provider.Go.Packages = []string{"example.com/fixture/enum"}
		plan.Plan.Dirty = []string{dirty}
		if unknown != "" {
			plan.Plan.Unknown = append(plan.Plan.Unknown, struct {
				Reason string `json:"reason"`
				Detail string `json:"detail"`
			}{Reason: "LANGUAGE_FRONTIER", Detail: unknown})
		}
		return plan
	}
	whole := "run example.com/fixture/climb example.com/fixture/enum"
	got, lines := structuralVerdict(plan(sealed, ""), root)
	if got != "run example.com/fixture/enum" || !slices.Contains(lines, "evidence "+sealed) {
		t.Fatalf("sealed structural selection = %q", strings.Join(lines, "\n"))
	}
	if got, _ := structuralVerdict(plan(sealed, "go:path-token-bound:go:example.com/fixture/loose"), root); got != whole {
		t.Fatalf("token-bound structural verdict = %q, want %q", got, whole)
	}
	if got := verdict(plan(sealed, ""), root); got != whole {
		t.Fatalf("Select verdict = %q, want %q", got, whole)
	}
	for _, forged := range []string{".corvint/changes/forged.cem.json", ".corvint/changes/0123456789ABCDEF0123456789abcdef01234567.cem.json", ".corvint/changes/x/0123456789abcdef0123456789abcdef01234567.cem.json"} {
		if _, lines := structuralVerdict(plan(forged, ""), root); slices.ContainsFunc(lines, func(line string) bool { return strings.HasPrefix(line, "evidence ") }) {
			t.Fatalf("a forged CEM name %s took the plan's readers: %q", forged, strings.Join(lines, "\n"))
		}
	}
	stale := plan(sealed, "")
	stale.Provider.Go.Packages = []string{"example.com/fixture/loose"}
	if got, lines := structuralVerdict(stale, root); got != "run example.com/fixture/climb example.com/fixture/enum example.com/fixture/loose" {
		t.Fatalf("a plan without the enumerating reader narrowed rule (c): %q", strings.Join(lines, "\n"))
	}
	if _, lines := structuralVerdict(plan(".corvint/change.cem.json", ""), root); slices.ContainsFunc(lines, func(line string) bool { return strings.HasPrefix(line, "evidence ") }) {
		t.Fatalf("sidecar took the plan's readers: %q", strings.Join(lines, "\n"))
	}

	// A holder whose declared read scope excludes the CEM is no reader the
	// plan must keep (AFP-V0-161); a dirty declaration is not trusted.
	scopedRoot := writeFixture(t, map[string]string{
		"enum/enum_test.go":              "package enum\n\nvar sealed = \"../.corvint/changes\"\n",
		"scoped/scoped_test.go":          "package scoped\n\nvar fixture = \"../.corvint/changes\"\n",
		sealed:                           "{}\n",
		".corvint/test-read-scopes.json": `{"profile":"corvint-test-read-scopes/0","packages":{"scoped":[]}}` + "\n",
	})
	if got, lines := structuralVerdict(plan(sealed, ""), scopedRoot); got != "run example.com/fixture/enum" || !slices.Contains(lines, "evidence "+sealed) {
		t.Fatalf("a read-scoped holder blocked the plan's readers: %q", strings.Join(lines, "\n"))
	}
	dirtyScopes := plan(sealed, "")
	dirtyScopes.Plan.Dirty = append(dirtyScopes.Plan.Dirty, ".corvint/test-read-scopes.json")
	if _, lines := structuralVerdict(dirtyScopes, scopedRoot); slices.ContainsFunc(lines, func(line string) bool { return strings.HasPrefix(line, "evidence ") }) {
		t.Fatalf("a dirty read-scope declaration took the plan's readers: %q", strings.Join(lines, "\n"))
	}
}
