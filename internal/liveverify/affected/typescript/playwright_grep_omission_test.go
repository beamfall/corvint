package typescript

import (
	json "encoding/json/v2"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// grepListCase is one listed spec: the file, describe titles, test title and tags (without `@`), and
// the projects the listing places it in.
type grepListCase struct {
	file, title string
	describes   []string
	tags        []string
	projects    []string
}

// grepPlaywrightListing builds a `playwright test --list --reporter=json` report with real titles
// and tags, one file suite per distinct file and one nested describe suite per describe title.
func grepPlaywrightListing(t *testing.T, root, rootDir string, cases []grepListCase) []byte {
	t.Helper()
	files := []string{}
	byFile := map[string][]grepListCase{}
	for _, item := range cases {
		if _, ok := byFile[item.file]; !ok {
			files = append(files, item.file)
		}
		byFile[item.file] = append(byFile[item.file], item)
	}
	suites := []any{}
	for _, file := range files {
		top := map[string]any{"title": file, "file": file, "specs": []any{}, "suites": []any{}}
		for _, item := range byFile[file] {
			tests := []any{}
			for _, project := range item.projects {
				tests = append(tests, map[string]any{"projectName": project})
			}
			spec := map[string]any{"title": item.title, "file": file, "tags": toAny(item.tags), "tests": tests}
			suite := top
			for _, describe := range item.describes {
				child := map[string]any{"title": describe, "file": file, "specs": []any{}, "suites": []any{}}
				suite["suites"] = append(suite["suites"].([]any), child)
				suite = child
			}
			suite["specs"] = append(suite["specs"].([]any), spec)
		}
		suites = append(suites, top)
	}
	raw, err := json.Marshal(map[string]any{
		"config": map[string]any{
			"configFile": filepath.Join(root, "playwright.config.ts"), "rootDir": filepath.Join(root, rootDir), "shard": nil,
			"argv": []any{"/usr/local/bin/node", root + "/node_modules/.bin/playwright", "test", "--list", "--reporter=json"},
		},
		"suites": suites, "errors": []any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// grepFixture writes the GitHub #790 layout: three projects over one testDir, where main inverts
// the @tagged filter and alt-a/alt-b keep only @tagged tests (projects is the config body).
func grepFixture(t *testing.T, projects string) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "playwright.config.ts", "export default defineConfig({ testDir: 'e2e', projects: ["+projects+"] });\n")
	write(t, root, "e2e/plain.spec.ts", "test('plain', async () => {});\n")
	write(t, root, "e2e/tagged.spec.ts", "test('x @tagged', async () => {});\ntest('y', async () => {});\n")
	return root
}

const grepRepro = "{ name: 'main', grepInvert: /@tagged/ }, { name: 'alt-a', grep: /@tagged/ }, { name: 'alt-b', grep: /@tagged/ }"

func grepReproCases() []grepListCase {
	return []grepListCase{
		{file: "plain.spec.ts", title: "plain", projects: []string{"main"}},
		{file: "tagged.spec.ts", title: "x @tagged", tags: []string{"tagged"}, projects: []string{"alt-a", "alt-b"}},
		{file: "tagged.spec.ts", title: "y", projects: []string{"main"}},
	}
}

func decodeGrepReceipt(t *testing.T, raw []byte) PlaywrightDiscovery {
	t.Helper()
	var receipt PlaywrightDiscovery
	if err := json.Unmarshal(raw, &receipt); err != nil {
		t.Fatal(err)
	}
	return receipt
}

func TestPlaywrightDiscoveryGrepOmission_TJAA_V0_021(t *testing.T) {
	allUnits := []PlaywrightDiscoveryUnit{
		{Project: "alt-a", Test: "e2e/plain.spec.ts"}, {Project: "alt-a", Test: "e2e/tagged.spec.ts"},
		{Project: "alt-b", Test: "e2e/plain.spec.ts"}, {Project: "alt-b", Test: "e2e/tagged.spec.ts"},
		{Project: "main", Test: "e2e/plain.spec.ts"}, {Project: "main", Test: "e2e/tagged.spec.ts"},
	}
	t.Run("GitHub #790 three-project repro is explained by static grep", func(t *testing.T) {
		root := grepFixture(t, grepRepro)
		raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, grepPlaywrightListing(t, root, "e2e", grepReproCases()))
		if err != nil {
			t.Fatalf("grep-explained listing refused: %v", err)
		}
		receipt := decodeGrepReceipt(t, raw)
		if !slices.Equal(receipt.Units, allUnits) || receipt.UnverifiedUnits != nil || strings.Contains(string(raw), "unverifiedUnits") {
			t.Fatalf("receipt: %s", raw)
		}
		plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, raw)
		if err != nil || plan.Discovery.State != "MATCHED" {
			t.Fatalf("plan: %+v %v", plan, err)
		}
		for _, unknown := range plan.Unknown {
			if unknown.Reason == PlaywrightUnknownDiscoveryOmission {
				t.Fatalf("explained omission reported unverified: %+v", plan.Unknown)
			}
		}
	})
	t.Run("array grep and a describe title and tag in the title", func(t *testing.T) {
		root := grepFixture(t, "{ name: 'main', grepInvert: [/@tagged/, /@slow/] }, { name: 'alt-a', grep: [/@tagged/, /suite-x/i] }, { name: 'alt-b', grep: /@tagged/ }")
		cases := grepReproCases()
		cases[1] = grepListCase{file: "tagged.spec.ts", title: "x", describes: []string{"Suite-X"}, tags: []string{"tagged"}, projects: []string{"alt-a", "alt-b"}}
		cases = append(cases, grepListCase{file: "plain.spec.ts", title: "p", describes: []string{"suite-x"}, projects: []string{"alt-a", "main"}})
		raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, grepPlaywrightListing(t, root, "e2e", cases))
		if err != nil {
			t.Fatalf("array grep listing refused: %v", err)
		}
		if receipt := decodeGrepReceipt(t, raw); !slices.Equal(receipt.Units, allUnits) || receipt.UnverifiedUnits != nil {
			t.Fatalf("receipt: %s", raw)
		}
	})
	t.Run("grep that does not explain the omission is still refused", func(t *testing.T) {
		root := grepFixture(t, grepRepro)
		cases := grepReproCases()
		cases[1].projects = []string{"alt-b"} // alt-a's grep keeps "x @tagged", so its omission is stale
		raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, grepPlaywrightListing(t, root, "e2e", cases))
		if err == nil || raw != nil || !strings.Contains(err.Error(), `project "alt-a" test e2e/tagged.spec.ts`) || !strings.Contains(err.Error(), "grep/grepInvert does not exclude") {
			t.Fatalf("unexplained omission stamped: raw=%s err=%v", raw, err)
		}
	})
	t.Run("grep with a file that has no listed case is refused", func(t *testing.T) {
		root := grepFixture(t, grepRepro)
		write(t, root, "e2e/new.spec.ts", "test('n @tagged', async () => {});\n")
		raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, grepPlaywrightListing(t, root, "e2e", grepReproCases()))
		if err == nil || raw != nil || !strings.Contains(err.Error(), "e2e/new.spec.ts") {
			t.Fatalf("stale listing stamped: raw=%s err=%v", raw, err)
		}
	})
	t.Run("project without grep keeps the strict refusal", func(t *testing.T) {
		root := grepFixture(t, "{ name: 'main', grepInvert: /@tagged/ }, { name: 'alt-a', grep: /@tagged/ }, { name: 'alt-b' }")
		raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, grepPlaywrightListing(t, root, "e2e", grepReproCases()))
		if err == nil || raw != nil || !strings.Contains(err.Error(), `project "alt-b" test e2e/plain.spec.ts`) || strings.Contains(err.Error(), "grep/grepInvert") {
			t.Fatalf("no-grep omission stamped: raw=%s err=%v", raw, err)
		}
	})
	t.Run("dynamic grep records unverified units and stays unresolved", func(t *testing.T) {
		root := grepFixture(t, "{ name: 'main', grepInvert: /@tagged/ }, { name: 'alt-a', grep: tagPattern }, { name: 'alt-b', grep: new RegExp('@tagged') }")
		raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, grepPlaywrightListing(t, root, "e2e", grepReproCases()))
		if err != nil {
			t.Fatalf("dynamic grep listing refused: %v", err)
		}
		receipt := decodeGrepReceipt(t, raw)
		want := []PlaywrightDiscoveryUnit{{Project: "alt-a", Test: "e2e/plain.spec.ts"}, {Project: "alt-b", Test: "e2e/plain.spec.ts"}}
		if !slices.Equal(receipt.Units, allUnits) || !slices.Equal(receipt.UnverifiedUnits, want) {
			t.Fatalf("receipt: %s", raw)
		}
		plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, raw)
		if err != nil || plan.Discovery.State != "STATIC_UNRESOLVED" || plan.Scope == "BOUNDED" {
			t.Fatalf("dynamic grep plan claimed bounded: %+v %v", plan, err)
		}
	})
	for _, grep := range []string{`/^alt-a @tagged/`, `/x @tagged/`, `/[^ ]@tagged/`, `/\W@tagged/`, `/tagged$/`} {
		t.Run("static grep outside the partwise model is unverified "+grep, func(t *testing.T) {
			root := grepFixture(t, "{ name: 'main', grepInvert: /@tagged/ }, { name: 'alt-a', grep: "+grep+" }, { name: 'alt-b', grep: /@tagged/ }")
			raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, grepPlaywrightListing(t, root, "e2e", grepReproCases()))
			if err != nil {
				t.Fatalf("listing refused: %v", err)
			}
			receipt := decodeGrepReceipt(t, raw)
			if !slices.Equal(receipt.UnverifiedUnits, []PlaywrightDiscoveryUnit{{Project: "alt-a", Test: "e2e/plain.spec.ts"}}) {
				t.Fatalf("receipt: %s", raw)
			}
			plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, raw)
			if err != nil || plan.Discovery.State != "MATCHED" {
				t.Fatalf("plan: %+v %v", plan, err)
			}
			if !slices.ContainsFunc(plan.Unknown, func(unknown PlaywrightUnknown) bool {
				return unknown.Reason == PlaywrightUnknownDiscoveryOmission && strings.Contains(unknown.Detail, `"alt-a"`)
			}) {
				t.Fatalf("missing omission unknown: %+v", plan.Unknown)
			}
		})
	}
	t.Run("receipt unverifiedUnits must be non-empty, sorted and within units", func(t *testing.T) {
		root := grepFixture(t, grepRepro)
		altA, altB := PlaywrightDiscoveryUnit{Project: "alt-a", Test: "e2e/plain.spec.ts"}, PlaywrightDiscoveryUnit{Project: "alt-b", Test: "e2e/plain.spec.ts"}
		for _, row := range []struct {
			name       string
			unverified []PlaywrightDiscoveryUnit
			detail     string
		}{
			{"empty", []PlaywrightDiscoveryUnit{}, "unverifiedUnits: omit the member when empty"},
			{"outside", []PlaywrightDiscoveryUnit{{Project: "other", Test: "e2e/plain.spec.ts"}}, "unverifiedUnits[0]: not one of units"},
			{"unsorted", []PlaywrightDiscoveryUnit{altB, altA}, "unverifiedUnits[1]: not strictly after"},
			{"duplicate", []PlaywrightDiscoveryUnit{altA, altA}, "unverifiedUnits[1]: not strictly after"},
		} {
			var receipt PlaywrightDiscovery
			if err := json.Unmarshal(discoveryFixtureBytes(t, root, allUnits), &receipt); err != nil {
				t.Fatal(err)
			}
			receipt.UnverifiedUnits = row.unverified
			raw, err := json.Marshal(receipt, json.Deterministic(true))
			if err != nil {
				t.Fatal(err)
			}
			plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, raw)
			if err != nil || plan.Discovery.State != "MALFORMED" || !strings.Contains(plan.Discovery.Detail, row.detail) {
				t.Fatalf("%s: invalid unverifiedUnits accepted: %+v %v", row.name, plan.Discovery, err)
			}
		}
		var receipt PlaywrightDiscovery
		if err := json.Unmarshal(discoveryFixtureBytes(t, root, allUnits), &receipt); err != nil {
			t.Fatal(err)
		}
		receipt.UnverifiedUnits = []PlaywrightDiscoveryUnit{altA, altB}
		raw, err := json.Marshal(receipt, json.Deterministic(true))
		if err != nil {
			t.Fatal(err)
		}
		if plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, raw); err != nil || plan.Discovery.State != "MATCHED" {
			t.Fatalf("valid unverifiedUnits refused: %+v %v", plan.Discovery, err)
		}
	})
}

func TestPlaywrightRegexBodyPartwise_TJAA_V0_021(t *testing.T) {
	for body, want := range map[string]bool{
		`@tagged`: true, `@(slow|tagged)`: true, `\bsmoke\b`: true, `[a-z]+-\d{2,3}`: true, `[\w@]+`: true,
		`a.b`: false, `^main`: false, `tagged$`: false, `a b`: false, `a\Db`: false, `a\Wb`: false,
		`[^a]`: false, `[ a]`: false, `[\t-z]`: false, `[!-~]`: true,
	} {
		if got := playwrightRegexBodyPartwise(body); got != want {
			t.Errorf("playwrightRegexBodyPartwise(%q) = %v, want %v", body, got, want)
		}
	}
}
