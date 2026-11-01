package typescript

import (
	"slices"
	"testing"
)

// TestPlaywrightNarrowingFrontierTaintsOnlyReachingTests_TJAA_V0_762 (V1-2060): a sited
// frontier other than a dynamic import no longer refuses the narrowing; it adds as candidates
// exactly the tests whose static import closure reaches its site, with the
// FRONTIER_SITE_REACH witness, and a test that reaches neither the site nor the dirty module
// stays out.
func TestPlaywrightNarrowingFrontierTaintsOnlyReachingTests_TJAA_V0_762(t *testing.T) {
	root := narrowingFixture(t, narrowingDynamicMobile)
	write(t, root, "tests/support/session.ts", `import { token } from "./missing"; export const session = token`)
	write(t, root, "tests/cart.spec.ts", `import { test } from "@playwright/test"; import { session } from "./support/session"; test("cart", () => session)`)
	write(t, root, "tests/search.spec.ts", `import { test } from "@playwright/test"; test("search", () => {})`)
	plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, []string{"tests/support/fixture.ts"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Narrowed == nil || plan.Narrowed.State != PlaywrightNarrowed {
		t.Fatalf("narrowing = %+v, want NARROWED", plan.Narrowed)
	}
	kinds := map[string]string{}
	for _, candidate := range plan.Narrowed.Candidates {
		kinds[candidate.Test] = candidate.Witness.Kind
	}
	if kinds["tests/cart.spec.ts"] != playwrightWitnessFrontierReach || kinds["tests/login.spec.ts"] != "DEPENDENCY_PATH" {
		t.Fatalf("candidate witnesses = %v", kinds)
	}
	if _, ok := kinds["tests/search.spec.ts"]; ok {
		t.Fatalf("a test reaching neither the frontier site nor the dirty module is a candidate: %v", narrowingCandidates(plan.Narrowed))
	}
}

// TestMultiBindingExportKeysEachName_TJAA_V0_761 (V1-2060): `export const a = 1, b = 2`
// yields one symbol per bound name, so changing one initializer modifies only that name and
// adding a binding adds only it.
func TestMultiBindingExportKeysEachName_TJAA_V0_761(t *testing.T) {
	before := []byte("export const a = 1, b = { x: [1, 2] }, c = \"p,q\"\n")
	modified := ChangedSymbols("m.ts", before, true, []byte("export const a = 1, b = { x: [1, 3] }, c = \"p,q\"\n"), true)
	if !slices.Equal(modified.Modified, []string{"b"}) || len(modified.Added) != 0 || len(modified.Removed) != 0 {
		t.Fatalf("modified one binding: %+v", modified)
	}
	added := ChangedSymbols("m.ts", before, true, []byte("export const a = 1, b = { x: [1, 2] }, c = \"p,q\", d = 4\n"), true)
	if !slices.Equal(added.Added, []string{"d"}) || len(added.Modified) != 0 {
		t.Fatalf("added one binding: %+v", added)
	}
	single := ChangedSymbols("m.ts", []byte("export let only = 1\n"), true, []byte("export let only = 2\n"), true)
	if !slices.Equal(single.Modified, []string{"only"}) {
		t.Fatalf("single binding: %+v", single)
	}
}

// TJAA-V0-761 (Codex review): a typed export whose annotation holds a generic comma binds one
// name, so changing its initializer modifies that name, never a fictitious type argument.
func TestTypedExportGenericCommaKeysTheBinding_TJAA_V0_761(t *testing.T) {
	for _, row := range []struct{ before, after, want string }{
		{"export const table: Record<string, number> = { x: 1 }\n", "export const table: Record<string, number> = { x: 2 }\n", "table"},
		{"export const pair: Map<string, Array<number>> = new Map(), other = 1\n", "export const pair: Map<string, Array<number>> = new Map(), other = 2\n", "other"},
		{"export const f = <T, U>(t: T, u: U) => [t, u], g = 1\n", "export const f = <T, U>(t: T, u: U) => [u, t], g = 1\n", "f"},
		// A comparison before a declarator comma is UNRESOLVED (TJAA-V0-761,
		// TestComparisonInMultiBindingExportIsUnresolved_TJAA_V0_761).
	} {
		change := ChangedSymbols("m.ts", []byte(row.before), true, []byte(row.after), true)
		if change.State != PlaywrightSymbolsResolved || !slices.Equal(change.Modified, []string{row.want}) || len(change.Added) != 0 || len(change.Removed) != 0 {
			t.Errorf("%q: %+v, want %s modified", row.before, change, row.want)
		}
	}
}
