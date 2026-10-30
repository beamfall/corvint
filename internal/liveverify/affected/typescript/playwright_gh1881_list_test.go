package typescript

import (
	"slices"
	"strings"
	"testing"
)

// gh1881Ungrepped returns the GitHub #790 cases as an ungrepped listing places them: every case in
// every project, as Playwright lists them with the project grep and grepInvert removed.
func gh1881Ungrepped(projects ...string) []grepListCase {
	cases := grepReproCases()
	for index := range cases {
		cases[index].projects = projects
	}
	return cases
}

// gh1881HiddenTagFixture is the GitHub #790 layout whose plain.spec.ts carries a computed describe
// tag, so the static tag closure stops and the grep omissions of plain.spec.ts are unverified.
func gh1881HiddenTagFixture(t *testing.T, projects string) string {
	t.Helper()
	root := grepFixture(t, projects)
	write(t, root, "e2e/plain.spec.ts", "const extra = ' slow';\ntest.describe('d', { tag: '@a' + extra }, () => {\n  test('plain', async () => {});\n});\n")
	return root
}

func TestPlaywrightDiscoveryUngreppedListing_GH1881(t *testing.T) {
	allUnits := []PlaywrightDiscoveryUnit{
		{Project: "alt-a", Test: "e2e/plain.spec.ts"}, {Project: "alt-a", Test: "e2e/tagged.spec.ts"},
		{Project: "alt-b", Test: "e2e/plain.spec.ts"}, {Project: "alt-b", Test: "e2e/tagged.spec.ts"},
		{Project: "main", Test: "e2e/plain.spec.ts"}, {Project: "main", Test: "e2e/tagged.spec.ts"},
	}
	plainUnverified := []PlaywrightDiscoveryUnit{{Project: "alt-a", Test: "e2e/plain.spec.ts"}, {Project: "alt-b", Test: "e2e/plain.spec.ts"}}
	produce := func(t *testing.T, root string, listing, ungrepped []grepListCase) (PlaywrightDiscovery, []byte, error) {
		t.Helper()
		var other []byte
		if ungrepped != nil {
			other = grepPlaywrightListing(t, root, "e2e", ungrepped)
		}
		raw, err := PlaywrightDiscoveryFromLists(root, "playwright.config.ts", discoveryFixtureRevision, grepPlaywrightListing(t, root, "e2e", listing), other)
		if err != nil {
			return PlaywrightDiscovery{}, nil, err
		}
		return decodeGrepReceipt(t, raw), raw, nil
	}
	for _, row := range []struct {
		name, projects   string
		listing, ungrepp []grepListCase
		unverified       []PlaywrightDiscoveryUnit
	}{
		// TJAA-V0-119: without the second listing the closure stop leaves both pairs unverified.
		{name: "TJAA-V0-118 no ungrepped listing keeps the closure-stop unknowns", projects: grepRepro, listing: grepReproCases(), unverified: plainUnverified},
		{name: "TJAA-V0-119 an agreeing ungrepped listing verifies every omission", projects: grepRepro, listing: grepReproCases(), ungrepp: gh1881Ungrepped("main", "alt-a", "alt-b")},
		// The configured grep would keep a case the ungrepped listing reports with @tagged in alt-a,
		// but the discovery listing does not list it there: alt-a stays unverified, alt-b is verified.
		{name: "TJAA-V0-119 a prediction mismatch leaves only that project unverified", projects: grepRepro, listing: grepReproCases(), ungrepp: func() []grepListCase {
			cases := gh1881Ungrepped("main", "alt-a", "alt-b")
			cases[0].projects = []string{"main", "alt-b"}
			return append(cases, grepListCase{file: "plain.spec.ts", title: "plain", tags: []string{"tagged"}, projects: []string{"alt-a"}})
		}(), unverified: plainUnverified[:1]},
		{name: "TJAA-V0-119 an ungrepped listing that lacks the pair does not verify it", projects: grepRepro, listing: grepReproCases(), ungrepp: func() []grepListCase {
			cases := gh1881Ungrepped("main", "alt-a", "alt-b")
			cases[0].projects = []string{"main", "alt-b"}
			return cases
		}(), unverified: plainUnverified[:1]},
		{name: "TJAA-V0-119 a discovery case missing from the ungrepped listing verifies nothing", projects: grepRepro, listing: grepReproCases(), ungrepp: gh1881Ungrepped("main", "alt-a", "alt-b")[:2], unverified: plainUnverified},
		{name: "TJAA-V0-119 a repeated case key verifies nothing", projects: grepRepro, listing: grepReproCases(), ungrepp: append(gh1881Ungrepped("main", "alt-a", "alt-b"), grepListCase{file: "plain.spec.ts", title: "plain", projects: []string{"alt-a"}}), unverified: plainUnverified},
		{name: "TJAA-V0-119 a dynamic grep is not verified", projects: "{ name: 'main', grepInvert: /@tagged/ }, { name: 'alt-a', grep: process.env.G }, { name: 'alt-b', grep: /@tagged/ }", listing: grepReproCases(), ungrepp: gh1881Ungrepped("main", "alt-a", "alt-b"), unverified: plainUnverified[:1]},
		{name: "TJAA-V0-119 a project without grep listed differently verifies nothing", projects: grepRepro + ", { name: 'all' }", listing: append(grepReproCases(), grepListCase{file: "plain.spec.ts", title: "plain", projects: []string{"all"}}, grepListCase{file: "tagged.spec.ts", title: "y", projects: []string{"all"}}), ungrepp: gh1881Ungrepped("main", "alt-a", "alt-b", "all"), unverified: plainUnverified},
	} {
		t.Run(row.name, func(t *testing.T) {
			root := gh1881HiddenTagFixture(t, row.projects)
			receipt, raw, err := produce(t, root, row.listing, row.ungrepp)
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			if !slices.Equal(receipt.UnverifiedUnits, row.unverified) || !slices.Equal(receipt.Units[:6], allUnits) && !strings.Contains(row.projects, "'all'") {
				t.Fatalf("receipt: %s", raw)
			}
			plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, raw)
			omission := slices.ContainsFunc(plan.Unknown, func(unknown PlaywrightUnknown) bool { return unknown.Reason == PlaywrightUnknownDiscoveryOmission })
			if strings.Contains(row.projects, "process.env") {
				return // a dynamic grep leaves discovery unproven; the receipt above is the evidence
			}
			if err != nil || plan.Discovery.State != "MATCHED" || omission != (len(row.unverified) != 0) {
				t.Fatalf("plan: %+v %v", plan, err)
			}
		})
	}
	// TJAA-V0-120: an omission the listing alone cannot explain at all (the file has no listed case
	// in any project) is refused without the second listing and verified with an agreeing one.
	t.Run("TJAA-V0-120 an otherwise unexplained omission is verified", func(t *testing.T) {
		projects := "{ name: 'alt-a', grep: /@tagged/ }, { name: 'alt-b', grep: /@tagged/ }"
		root := grepFixture(t, projects)
		listing := []grepListCase{{file: "tagged.spec.ts", title: "x @tagged", tags: []string{"tagged"}, projects: []string{"alt-a", "alt-b"}}}
		if _, _, err := produce(t, root, listing, nil); err == nil || !strings.Contains(err.Error(), "omits project") {
			t.Fatalf("listing alone accepted: %v", err)
		}
		receipt, raw, err := produce(t, root, listing, gh1881Ungrepped("alt-a", "alt-b"))
		if err != nil || len(receipt.UnverifiedUnits) != 0 || len(receipt.Units) != 4 {
			t.Fatalf("receipt: %s %v", raw, err)
		}
	})
	t.Run("TJAA-V0-118 an invalid ungrepped listing is refused", func(t *testing.T) {
		root := gh1881HiddenTagFixture(t, grepRepro)
		listing := grepPlaywrightListing(t, root, "e2e", grepReproCases())
		ghost := grepPlaywrightListing(t, root, "e2e", append(gh1881Ungrepped("main", "alt-a", "alt-b"), grepListCase{file: "plain.spec.ts", title: "plain", projects: []string{"ghost"}}))
		filtered := []byte(strings.Replace(string(grepPlaywrightListing(t, root, "e2e", gh1881Ungrepped("main", "alt-a", "alt-b"))), `"--reporter=json"`, `"--reporter=json","--grep","x"`, 1))
		for name, ungrepped := range map[string][]byte{"foreign pair": ghost, "filtered argv": filtered, "not JSON": []byte("{")} {
			if _, err := PlaywrightDiscoveryFromLists(root, "playwright.config.ts", discoveryFixtureRevision, listing, ungrepped); err == nil || !strings.Contains(err.Error(), "ungrepped") {
				t.Fatalf("%s accepted: %v", name, err)
			}
		}
	})
}
