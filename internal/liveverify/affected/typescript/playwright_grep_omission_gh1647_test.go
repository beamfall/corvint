package typescript

import (
	json "encoding/json/v2"
	"slices"
	"strings"
	"testing"
)

func gh1647Omission(t *testing.T, root string, cases []grepListCase) ([]PlaywrightDiscoveryUnit, []PlaywrightUnknown) {
	t.Helper()
	raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, grepPlaywrightListing(t, root, "e2e", cases))
	if err != nil {
		t.Fatalf("listing refused: %v", err)
	}
	plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, raw)
	if err != nil || plan.Discovery.State != "MATCHED" {
		t.Fatalf("discovery=%+v err=%v", plan.Discovery, err)
	}
	omissions := []PlaywrightUnknown{}
	for _, unknown := range plan.Unknown {
		if unknown.Reason == PlaywrightUnknownDiscoveryOmission {
			omissions = append(omissions, unknown)
		}
	}
	return decodeGrepReceipt(t, raw).UnverifiedUnits, omissions
}

// AFP-V0-409, AFP-V0-410: a regex-literal grep omission is verified when every tag in the config,
// the test file and its static import closure is a literal `@\S+` string and the listing reports
// only those tags; a hidden or computed tag, an unresolved import or a reported tag the sources do
// not hold keeps it unverified, and the detail names the units (AFP-V0-411).
func TestPlaywrightGrepOmissionSourceTags_GH1647(t *testing.T) {
	plainUnverified := []PlaywrightDiscoveryUnit{{Project: "alt-a", Test: "e2e/plain.spec.ts"}, {Project: "alt-b", Test: "e2e/plain.spec.ts"}}
	smoke := func() []grepListCase {
		cases := grepReproCases()
		cases[0].tags = []string{"smoke"}
		return cases
	}
	for _, row := range []struct {
		label, plain string
		extra        map[string]string
		cases        func() []grepListCase
		verified     bool
	}{
		{"literal describe tag", "test.describe('d', { tag: '@smoke' }, () => { test('plain', async () => {}); });\n", nil, smoke, true},
		{"literal tag array from a relative import", "import { tags } from './tags';\ntest('plain', tags, async () => {});\n", map[string]string{"e2e/tags.ts": "export const tags = { tag: ['@smoke'] };\n"}, smoke, true},
		{"array literals and literal indexes stay literal", "const titles: string[] = ['plain'];\nfor (const title of [titles[0]]) test(title, { tag: ['@smoke'] }, async () => {});\n", nil, smoke, true},
		{"computed member access", "const d = { tag: ['@smoke'] };\nfor (const k in d) d[k].push('@x y');\ntest('plain', d, async () => {});\n", nil, smoke, false},
		{"tag with whitespace", "test.describe('d', { tag: '@smoke test' }, () => { test('plain', async () => {}); });\n", nil, smoke, false},
		{"tag alone", "test.describe('d', { tag: '@' }, () => { test('plain', async () => {}); });\n", nil, grepReproCases, false},
		{"computed tag", "const t = '@smoke';\ntest('plain', { tag: t }, async () => {});\n", nil, smoke, false},
		{"escaped tag", "test('plain', { tag: '@smo\\x6be' }, async () => {});\n", nil, smoke, false},
		{"computed key", "const k = 'ta' + 'g';\ntest('plain', { [k]: '@x y' }, async () => {});\n", nil, grepReproCases, false},
		{"property-building global", "test('plain', Object.fromEntries([['ta' + 'g', '@x y']]), async () => {});\n", nil, grepReproCases, false},
		{"bare package import", "import { tags } from 'shared-tags';\ntest('plain', tags, async () => {});\n", nil, grepReproCases, false},
		{"reported tag the sources do not hold", "test('plain', async () => {});\n", nil, smoke, false},
		{"escaped string key", "test('plain', { \"\\x74ag\": ['@x y'] }, async () => {});\n", nil, grepReproCases, false},
		{"escaped identifier key", "test('plain', { t\\u0061g: ['@x y'] }, async () => {});\n", nil, grepReproCases, false},
		{"constructor route", "const opts = { tag: ['@smoke'] };\n({}).constructor.values(opts)[0].push('@x y');\ntest('plain', opts, async () => {});\n", nil, smoke, false},
		{"ambiguous module extension", "import { tags } from './tags';\ntest('plain', tags, async () => {});\n", map[string]string{"e2e/tags.cjs": "exports.tags = { tag: ['@smoke'] };\n", "e2e/tags.js": "exports.tags = { tag: ['@smoke hidden'] };\n"}, smoke, false},
		{"dependency outside the bound sources", "import { tags } from '../dist/tags';\ntest('plain', tags, async () => {});\n", map[string]string{"dist/tags.ts": "export const tags = { tag: ['@smoke'] };\n"}, smoke, false},
	} {
		t.Run(row.label, func(t *testing.T) {
			root := grepFixture(t, grepRepro)
			write(t, root, "e2e/plain.spec.ts", row.plain)
			for file, content := range row.extra {
				write(t, root, file, content)
			}
			unverified, omissions := gh1647Omission(t, root, row.cases())
			if row.verified {
				if len(unverified) != 0 || len(omissions) != 0 {
					t.Fatalf("not verified: unverified=%+v omissions=%+v", unverified, omissions)
				}
				return
			}
			if !slices.Equal(unverified, plainUnverified) || len(omissions) != 1 ||
				!strings.Contains(omissions[0].Detail, `2 discovery unit(s)`) || !strings.Contains(omissions[0].Detail, `project "alt-a" test e2e/plain.spec.ts, project "alt-b" test e2e/plain.spec.ts`) {
				t.Fatalf("unverified=%+v omissions=%+v", unverified, omissions)
			}
		})
	}
	t.Run("config tag with whitespace", func(t *testing.T) {
		root := grepFixture(t, grepRepro)
		write(t, root, "playwright.config.ts", "export default defineConfig({ testDir: 'e2e', tag: '@nightly run', projects: ["+grepRepro+"] });\n")
		if unverified, _ := gh1647Omission(t, root, grepReproCases()); !slices.Equal(unverified, plainUnverified) {
			t.Fatalf("unverified=%+v", unverified)
		}
	})
}

// AFP-V0-409: every file of the tag proof is in the receipt's source binding, so changing an
// imported tag module after the receipt is written makes it BINDING_MISMATCH instead of keeping a
// stale verified omission.
func TestPlaywrightGrepOmissionDependencyInvalidates_GH1647(t *testing.T) {
	root := grepFixture(t, grepRepro)
	write(t, root, "e2e/plain.spec.ts", "import { tags } from './tags';\ntest('plain', tags, async () => {});\n")
	write(t, root, "e2e/tags.ts", "export const tags = { tag: ['@smoke'] };\n")
	cases := grepReproCases()
	cases[0].tags = []string{"smoke"}
	raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, grepPlaywrightListing(t, root, "e2e", cases))
	if err != nil {
		t.Fatalf("listing refused: %v", err)
	}
	if unverified := decodeGrepReceipt(t, raw).UnverifiedUnits; len(unverified) != 0 {
		t.Fatalf("unverified=%+v", unverified)
	}
	write(t, root, "e2e/tags.ts", "export const tags = { tag: ['@smoke hidden'] };\n")
	plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, raw)
	if err != nil {
		t.Fatal(err)
	}
	assertDiscoveryFallback(t, plan, "BINDING_MISMATCH")
	if !slices.ContainsFunc(plan.Unknown, func(unknown PlaywrightUnknown) bool { return strings.Contains(unknown.Detail, "source digest") }) {
		t.Fatalf("unknown=%+v", plan.Unknown)
	}
}

// AFP-V0-413: a pattern using a JavaScript-only regex feature (lookaround, a backreference, the v or
// y flag) stays unverified even when the sources hold no hidden tag, and the plan is not bounded.
func TestPlaywrightGrepOmissionJavaScriptOnlyRegex_GH1647(t *testing.T) {
	for _, pattern := range []string{`/(?<=\s)@tagged/`, `/(?=@)@tagged/`, `/(@)tagged\1?/`, `/@tagged/v`, `/@tagged/y`} {
		t.Run(pattern, func(t *testing.T) {
			root := grepFixture(t, "{ name: 'main', grepInvert: /@tagged/ }, { name: 'alt-a', grep: "+pattern+" }, { name: 'alt-b', grep: /@tagged/ }")
			raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, grepPlaywrightListing(t, root, "e2e", grepReproCases()))
			if err != nil {
				t.Fatalf("listing refused: %v", err)
			}
			if unverified := decodeGrepReceipt(t, raw).UnverifiedUnits; !slices.Equal(unverified, []PlaywrightDiscoveryUnit{{Project: "alt-a", Test: "e2e/plain.spec.ts"}}) {
				t.Fatalf("unverified=%+v", unverified)
			}
			plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, raw)
			if err != nil || plan.Scope == "BOUNDED" || plan.Discovery.State == "MATCHED" {
				t.Fatalf("scope=%s discovery=%+v err=%v", plan.Scope, plan.Discovery, err)
			}
		})
	}
}

// AFP-V0-411: the unverified-unit detail lists at most ten units and counts the rest.
func TestPlaywrightUnitListBound_GH1647(t *testing.T) {
	units := []PlaywrightDiscoveryUnit{}
	for index := range 12 {
		units = append(units, PlaywrightDiscoveryUnit{Project: "p", Test: "t/" + string(rune('a'+index)) + ".spec.ts"})
	}
	if got := playwrightUnitList(units[:1]); got != `project "p" test t/a.spec.ts` {
		t.Fatal(got)
	}
	got := playwrightUnitList(units)
	if !strings.HasSuffix(got, `project "p" test t/j.spec.ts, and 2 more`) || strings.Contains(got, "t/k.spec.ts") {
		t.Fatal(got)
	}
}

// AFP-V0-412: a BINDING_MISMATCH receipt names each differing binding with both values and says
// how to regenerate it.
func TestPlaywrightBindingMismatchNamesBinding_GH1647(t *testing.T) {
	root := playwrightFixture(t)
	valid := discoveryFixtureBytes(t, root, smallDiscoveryUnits())
	for _, row := range []struct {
		label  string
		mutate func(*PlaywrightDiscovery)
		names  []string
	}{
		{"revision", func(r *PlaywrightDiscovery) { r.Revision = strings.Repeat("2", 40) }, []string{"revision (receipt 222222222222, plan "}},
		{"config digest", func(r *PlaywrightDiscovery) { r.Config.SHA256 = strings.Repeat("0", 64) }, []string{"config digest (receipt 000000000000, plan "}},
		{"source digest", func(r *PlaywrightDiscovery) { r.SourceDigest = "playwright-sources:sha256:" + strings.Repeat("0", 64) }, []string{"source digest"}},
		{"two bindings", func(r *PlaywrightDiscovery) {
			r.Revision = strings.Repeat("2", 40)
			r.Config.SHA256 = strings.Repeat("0", 64)
		}, []string{"revision", "config digest"}},
	} {
		t.Run(row.label, func(t *testing.T) {
			var receipt PlaywrightDiscovery
			if err := json.Unmarshal(valid, &receipt); err != nil {
				t.Fatal(err)
			}
			row.mutate(&receipt)
			raw, err := json.Marshal(receipt, json.Deterministic(true))
			if err != nil {
				t.Fatal(err)
			}
			plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, raw)
			if err != nil {
				t.Fatal(err)
			}
			assertDiscoveryFallback(t, plan, "BINDING_MISMATCH")
			index := slices.IndexFunc(plan.Unknown, func(unknown PlaywrightUnknown) bool { return unknown.Reason == "playwright:discovery-unproven" })
			if index < 0 {
				t.Fatalf("unknown=%+v", plan.Unknown)
			}
			detail := plan.Unknown[index].Detail
			for _, want := range append(row.names, "BINDING_MISMATCH: ", "regenerate it at this revision", "corvint affected discovery --playwright-config playwright.config.ts") {
				if !strings.Contains(detail, want) {
					t.Fatalf("detail %q lacks %q", detail, want)
				}
			}
		})
	}
}
