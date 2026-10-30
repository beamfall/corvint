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
	plainUnverified := []PlaywrightDiscoveryUnit{{Project: "alt-a", Test: "e2e/plain.spec.ts"}, {Project: "alt-b", Test: "e2e/plain.spec.ts"}}
	// AFP-V0-409/410 (GitHub #1647): with no tag in the config or test sources, no hidden describe
	// or config tag can reverse grep's negative decision, so the GitHub #790 pairs are verified.
	t.Run("GitHub #790 three-project repro is verified when the sources hold no hidden tag", func(t *testing.T) {
		root := grepFixture(t, grepRepro)
		raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, grepPlaywrightListing(t, root, "e2e", grepReproCases()))
		if err != nil {
			t.Fatalf("grep-filtered listing refused: %v", err)
		}
		receipt := decodeGrepReceipt(t, raw)
		if !slices.Equal(receipt.Units, allUnits) || len(receipt.UnverifiedUnits) != 0 {
			t.Fatalf("receipt: %s", raw)
		}
		plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, raw)
		if err != nil || plan.Discovery.State != "MATCHED" || slices.ContainsFunc(plan.Unknown, func(unknown PlaywrightUnknown) bool { return unknown.Reason == PlaywrightUnknownDiscoveryOmission }) {
			t.Fatalf("plan: %+v %v", plan, err)
		}
	})
	// A grep that matches no listed title is a negative decision a hidden describe or config tag
	// (one Playwright reports only in part, or not at all) could reverse, so with a computed tag
	// in the sources the GitHub #790 pairs are kept but unverified, and the plan names them
	// (Codex review round 2; AFP-V0-410, AFP-V0-411).
	t.Run("GitHub #790 repro with a computed describe tag keeps unverified grep omissions", func(t *testing.T) {
		root := grepFixture(t, grepRepro)
		write(t, root, "e2e/plain.spec.ts", "const extra = ' slow';\ntest.describe('d', { tag: '@a' + extra }, () => {\n  test('plain', async () => {});\n});\n")
		raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, grepPlaywrightListing(t, root, "e2e", grepReproCases()))
		if err != nil {
			t.Fatalf("grep-filtered listing refused: %v", err)
		}
		receipt := decodeGrepReceipt(t, raw)
		if !slices.Equal(receipt.Units, allUnits) || !slices.Equal(receipt.UnverifiedUnits, plainUnverified) {
			t.Fatalf("receipt: %s", raw)
		}
		plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, raw)
		if err != nil || plan.Discovery.State != "MATCHED" || !slices.ContainsFunc(plan.Unknown, func(unknown PlaywrightUnknown) bool {
			return unknown.Axis == PlaywrightAxisExecution && unknown.Reason == PlaywrightUnknownDiscoveryOmission && strings.Contains(unknown.Detail, `2 discovery unit(s)`) &&
				strings.Contains(unknown.Detail, `project "alt-a" test e2e/plain.spec.ts, project "alt-b" test e2e/plain.spec.ts`)
		}) {
			t.Fatalf("plan: %+v %v", plan, err)
		}
	})
	t.Run("grepInvert matching every listed title explains the omission", func(t *testing.T) {
		root := grepFixture(t, "{ name: 'main', grepInvert: /@tagged/ }, { name: 'other' }")
		cases := []grepListCase{
			{file: "plain.spec.ts", title: "plain", projects: []string{"main", "other"}},
			{file: "tagged.spec.ts", title: "x @tagged", tags: []string{"tagged"}, projects: []string{"other"}},
			{file: "tagged.spec.ts", title: "y", describes: []string{"@tagged suite"}, tags: []string{"tagged"}, projects: []string{"other"}},
		}
		raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, grepPlaywrightListing(t, root, "e2e", cases))
		if err != nil {
			t.Fatalf("grepInvert-explained listing refused: %v", err)
		}
		want := []PlaywrightDiscoveryUnit{{Project: "main", Test: "e2e/plain.spec.ts"}, {Project: "main", Test: "e2e/tagged.spec.ts"}, {Project: "other", Test: "e2e/plain.spec.ts"}, {Project: "other", Test: "e2e/tagged.spec.ts"}}
		if receipt := decodeGrepReceipt(t, raw); !slices.Equal(receipt.Units, want) || strings.Contains(string(raw), "unverifiedUnits") {
			t.Fatalf("receipt: %s", raw)
		}
		plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, raw)
		if err != nil || plan.Discovery.State != "MATCHED" || slices.ContainsFunc(plan.Unknown, func(unknown PlaywrightUnknown) bool { return unknown.Reason == PlaywrightUnknownDiscoveryOmission }) {
			t.Fatalf("plan: %+v %v", plan, err)
		}
	})
	t.Run("array grep and a describe title and tag in the title", func(t *testing.T) {
		root := grepFixture(t, "{ name: 'main', grepInvert: [/@tagged/, /@slow/] }, { name: 'alt-a', grep: [/@tagged/, /suite-x/i] }, { name: 'alt-b', grep: /@tagged/ }")
		cases := grepReproCases()
		cases[1] = grepListCase{file: "tagged.spec.ts", title: "x @tagged", describes: []string{"Suite-X"}, tags: []string{"tagged"}, projects: []string{"alt-a", "alt-b"}}
		cases = append(cases, grepListCase{file: "plain.spec.ts", title: "p", describes: []string{"suite-x"}, projects: []string{"alt-a", "main"}})
		raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, grepPlaywrightListing(t, root, "e2e", cases))
		if err != nil {
			t.Fatalf("array grep listing refused: %v", err)
		}
		if receipt := decodeGrepReceipt(t, raw); !slices.Equal(receipt.Units, allUnits) || len(receipt.UnverifiedUnits) != 0 {
			t.Fatalf("receipt: %s", raw)
		}
	})
	t.Run("array grep element matching a listed title refuses the omission", func(t *testing.T) {
		root := grepFixture(t, "{ name: 'main', grepInvert: /@tagged/ }, { name: 'alt-a', grep: [/@tagged/, /plain/] }, { name: 'alt-b', grep: /@tagged/ }")
		raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, grepPlaywrightListing(t, root, "e2e", grepReproCases()))
		if err == nil || raw != nil || !strings.Contains(err.Error(), `project "alt-a" test e2e/plain.spec.ts`) {
			t.Fatalf("stale listing stamped: raw=%s err=%v", raw, err)
		}
	})
	// Codex review round 1: Playwright appends only explicit tags to the grep title; a title tag
	// stays inside its title, so `a@tagged` gives no standalone `@tagged` part and /\B@tagged/ does
	// not match it.
	t.Run("title-derived tag is not a separate grep-title part", func(t *testing.T) {
		root := grepFixture(t, "{ name: 'main', grepInvert: /\\B@tagged/ }, { name: 'alt-a', grep: /@tagged/ }, { name: 'alt-b', grep: /@tagged/ }")
		cases := grepReproCases()
		cases[2] = grepListCase{file: "tagged.spec.ts", title: "a@tagged", tags: []string{"tagged"}, projects: []string{"alt-a", "alt-b"}}
		raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, grepPlaywrightListing(t, root, "e2e", cases))
		if err == nil || raw != nil || !strings.Contains(err.Error(), `project "main" test e2e/tagged.spec.ts`) {
			t.Fatalf("omission explained by a reconstructed tag part: raw=%s err=%v", raw, err)
		}
	})
	// Codex review round 1: an explicit describe tag such as '@foo bar' is reported only as `foo`,
	// so an exclusion that rests on explicit tags cannot be verified from the listing.
	t.Run("exclusion resting on explicit tags is unverified", func(t *testing.T) {
		root := grepFixture(t, grepRepro)
		cases := []grepListCase{
			{file: "plain.spec.ts", title: "plain", tags: []string{"foo"}, projects: []string{"main"}},
			{file: "tagged.spec.ts", title: "x", tags: []string{"tagged"}, projects: []string{"alt-a", "alt-b"}},
		}
		raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, grepPlaywrightListing(t, root, "e2e", cases))
		if err != nil {
			t.Fatalf("listing refused: %v", err)
		}
		want := []PlaywrightDiscoveryUnit{{Project: "alt-a", Test: "e2e/plain.spec.ts"}, {Project: "alt-b", Test: "e2e/plain.spec.ts"}, {Project: "main", Test: "e2e/tagged.spec.ts"}}
		receipt := decodeGrepReceipt(t, raw)
		if !slices.Equal(receipt.Units, allUnits) || !slices.Equal(receipt.UnverifiedUnits, want) {
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
	// The same omission the "grepInvert matching every listed title" case explains becomes
	// unverified when the pattern is outside the partwise model.
	for _, invert := range []string{`/^main @tagged/`, `/x @tagged/`, `/[^ ]?@tagged/`, `/\W@tagged/`, `/tagged$/`} {
		t.Run("static grepInvert outside the partwise model is unverified "+invert, func(t *testing.T) {
			root := grepFixture(t, "{ name: 'main', grepInvert: "+invert+" }, { name: 'other' }")
			cases := []grepListCase{
				{file: "plain.spec.ts", title: "plain", projects: []string{"main", "other"}},
				{file: "tagged.spec.ts", title: "x @tagged", tags: []string{"tagged"}, projects: []string{"other"}},
			}
			raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, grepPlaywrightListing(t, root, "e2e", cases))
			if err != nil {
				t.Fatalf("listing refused: %v", err)
			}
			receipt := decodeGrepReceipt(t, raw)
			if !slices.Equal(receipt.UnverifiedUnits, []PlaywrightDiscoveryUnit{{Project: "main", Test: "e2e/tagged.spec.ts"}}) {
				t.Fatalf("receipt: %s", raw)
			}
			plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, raw)
			if err != nil || plan.Discovery.State != "MATCHED" {
				t.Fatalf("plan: %+v %v", plan, err)
			}
			if !slices.ContainsFunc(plan.Unknown, func(unknown PlaywrightUnknown) bool {
				return unknown.Reason == PlaywrightUnknownDiscoveryOmission && strings.Contains(unknown.Detail, `"main"`)
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

func TestPlaywrightTitleTags_TJAA_V0_021(t *testing.T) {
	for title, want := range map[string][]string{
		"x @tagged":        {"@tagged"},
		"a@b@c d @ e @f g": {"@b@c", "@f"},
		"@x @y @":          {"@x", "@y"},
		"none":             {},
	} {
		if got := playwrightTitleTags(title); !slices.Equal(got, want) {
			t.Errorf("playwrightTitleTags(%q) = %q, want %q", title, got, want)
		}
	}
}

// locatedGrepListing reshapes a grepPlaywrightListing report as Playwright reports projects: one
// spec per project, each with its own test ID and the location PlaywrightListedTests requires
// (line is the spec's 1-based order within its file).
func locatedGrepListing(t *testing.T, raw []byte) []byte {
	t.Helper()
	var report map[string]any
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	var walk func(suites []any, line *int)
	walk = func(suites []any, line *int) {
		for _, item := range suites {
			suite := item.(map[string]any)
			specs := []any{}
			for _, item := range suite["specs"].([]any) {
				spec := item.(map[string]any)
				*line++
				for _, test := range spec["tests"].([]any) {
					project := test.(map[string]any)["projectName"].(string)
					specs = append(specs, map[string]any{
						"id": spec["file"].(string) + "#" + spec["title"].(string) + "#" + project, "title": spec["title"], "file": spec["file"],
						"tags": spec["tags"], "line": *line, "column": 1, "tests": []any{test},
					})
				}
			}
			suite["specs"] = specs
			walk(suite["suites"].([]any), line)
		}
	}
	for _, suite := range report["suites"].([]any) {
		line := 0
		walk([]any{suite}, &line)
	}
	located, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	return located
}

// GitHub #800: the listed-tests entry point behind `docs corpus discover-playwright` applies the
// same grep/grepInvert omission rules as discovery, so a grep-split listing yields its listed
// executions instead of a missing-pair refusal, and an unexplained omission still refuses.
func TestPlaywrightListedTestsGrepOmission_TJAA_V0_021(t *testing.T) {
	root := grepFixture(t, grepRepro)
	listing := locatedGrepListing(t, grepPlaywrightListing(t, root, "e2e", grepReproCases()))
	// Corpus discovery always passes the pinned revision's paths; its revision check must accept
	// the same grep omissions (Codex review round 1).
	revision := []string{"e2e/plain.spec.ts", "e2e/tagged.spec.ts", "playwright.config.ts"}
	tests, err := PlaywrightListedTests(root, "playwright.config.ts", listing, revision)
	if err != nil {
		t.Fatalf("grep-split listing refused: %v", err)
	}
	got := []string{}
	for _, test := range tests {
		got = append(got, test.Project+" "+test.Path+" "+test.Title)
	}
	want := []string{"main e2e/plain.spec.ts plain", "alt-a e2e/tagged.spec.ts x @tagged", "alt-b e2e/tagged.spec.ts x @tagged", "main e2e/tagged.spec.ts y"}
	if !slices.Equal(got, want) {
		t.Fatalf("executions = %q, want %q", got, want)
	}
	if _, err := PlaywrightListedTests(root, "playwright.config.ts", listing, append(revision, "e2e/gone.spec.ts")); err == nil || !strings.Contains(err.Error(), "at the source revision") {
		t.Fatalf("revision-only test file accepted: %v", err)
	}
	cases := grepReproCases()
	cases[0].projects = []string{"alt-a", "alt-b"} // main's grepInvert keeps `plain`, so its omission is unexplained
	if _, err := PlaywrightListedTests(root, "playwright.config.ts", locatedGrepListing(t, grepPlaywrightListing(t, root, "e2e", cases)), nil); err == nil || !strings.Contains(err.Error(), `omits project "main"`) {
		t.Fatalf("unexplained omission accepted: %v", err)
	}
}

// Codex review round 2 (GitHub #800): membership is checked against exactly the files and cases
// PlaywrightListedTests returns, so a nested suite file the execution walker does not use cannot
// stand in for the returned one, and a spec with no execution cannot explain an omission.
// The corpus entry point and affected discovery attribute files and cases from one listing walker
// (V1-1234), so each case runs through both and must be refused by both.
func TestPlaywrightListedTestsCasesMatchExecutions_TJAA_V0_021(t *testing.T) {
	listing := func(root string, suites string) []byte {
		return []byte(`{"config":{"configFile":` + jsonString(filepath.Join(root, "playwright.config.ts")) + `,"rootDir":` + jsonString(filepath.Join(root, "e2e")) +
			`,"shard":null,"argv":["/usr/local/bin/node","` + root + `/node_modules/.bin/playwright","test","--list","--reporter=json"]},"suites":` + suites + `,"errors":[]}`)
	}
	revision := []string{"e2e/plain.spec.ts", "e2e/tagged.spec.ts", "playwright.config.ts"}
	entries := map[string]func(root string, raw []byte) (any, error){
		"PlaywrightListedTests": func(root string, raw []byte) (any, error) {
			return PlaywrightListedTests(root, "playwright.config.ts", raw, revision)
		},
		"PlaywrightDiscoveryFromList": func(root string, raw []byte) (any, error) {
			receipt, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, raw)
			return string(receipt), err
		},
	}
	for _, row := range []struct {
		name, projects, suites, refusal string
	}{
		{
			name: "a nested suite file is not the returned execution's file", projects: "{ name: 'main', testIgnore: '**/plain.spec.ts' }",
			suites: `[{"title":"plain.spec.ts","file":"plain.spec.ts","specs":[],"suites":[{"title":"tagged.spec.ts","file":"tagged.spec.ts","specs":[` +
				`{"id":"tagged-main","title":"x @tagged","tags":["tagged"],"line":1,"column":1,"tests":[{"projectName":"main"}]}],"suites":[]}]}]`,
			refusal: `omits project "main" test e2e/tagged.spec.ts`,
		},
		{
			name: "a spec with no execution does not explain an omission", projects: "{ name: 'main', grepInvert: /@tagged/ }",
			suites: `[{"title":"plain.spec.ts","file":"plain.spec.ts","specs":[{"id":"plain-main","title":"plain","tags":[],"line":1,"column":1,"tests":[{"projectName":"main"}]}],"suites":[]},` +
				`{"title":"tagged.spec.ts","file":"tagged.spec.ts","specs":[{"id":"tagged-none","title":"x @tagged","tags":["tagged"],"line":1,"column":1,"tests":[]}],"suites":[]}]`,
			refusal: `omits project "main" test e2e/tagged.spec.ts`,
		},
		{
			name: "case titles follow every enclosing suite, not file inheritance", projects: "{ name: 'main', grep: /plain/ }, { name: 'other' }",
			suites: `[{"title":"@file.spec.ts","specs":[{"id":"plain-other","file":"plain.spec.ts","title":"plain","tags":[],"line":1,"column":1,"tests":[{"projectName":"other"}]}],"suites":[]},` +
				`{"title":"tagged.spec.ts","file":"tagged.spec.ts","specs":[{"id":"tagged-main","title":"plain x","tags":[],"line":1,"column":1,"tests":[{"projectName":"main"}]},` +
				`{"id":"tagged-other","title":"plain x","tags":[],"line":2,"column":1,"tests":[{"projectName":"other"}]}],"suites":[]}]`,
			refusal: `omits project "main" test e2e/plain.spec.ts`,
		},
	} {
		for entry, run := range entries {
			t.Run(entry+"/"+row.name, func(t *testing.T) {
				root := grepFixture(t, row.projects)
				if got, err := run(root, listing(root, row.suites)); err == nil || !strings.Contains(err.Error(), row.refusal) {
					t.Fatalf("listing accepted or refused for another reason: %v %v", got, err)
				}
			})
		}
	}
}

func jsonString(value string) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}
