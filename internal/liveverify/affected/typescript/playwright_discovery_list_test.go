package typescript

import (
	"bytes"
	json "encoding/json/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// multiProjectListFixture writes a repository whose config has a setup project, two device
// projects that depend on it and a project with its own testDir, and returns it with the real
// `playwright test --list --reporter=json` report (Playwright 1.61.1) rebased onto it.
func multiProjectListFixture(t *testing.T) (string, []byte) {
	t.Helper()
	root := t.TempDir()
	write(t, root, "playwright.config.ts", `import { defineConfig, devices } from '@playwright/test';

const authFile = 'playwright/.auth/user.json';

export default defineConfig({
  testDir: './e2e',
  use: { baseURL: process.env.BASE_URL ?? 'http://localhost:3000', trace: 'on-first-retry' },
  projects: [
    { name: 'setup', testMatch: /.*\.setup\.ts/ },
    { name: 'chromium', use: { ...devices['Desktop Chrome'], storageState: authFile }, dependencies: ['setup'] },
    { name: 'firefox', use: { ...devices['Desktop Firefox'], storageState: authFile }, dependencies: ['setup'] },
    { name: 'admin', testDir: './e2e/admin', use: { ...devices['Desktop Chrome'] } },
  ],
});
`)
	write(t, root, "e2e/auth.setup.ts", "import { test as setup } from '@playwright/test';\nsetup('authenticate', async () => {});\n")
	write(t, root, "e2e/home.spec.ts", "import { test, expect } from '@playwright/test';\nimport { title } from './shared/page';\ntest.describe('home', () => {\n  test('has title', async () => { expect(title).toBe('home'); });\n  test('second', async () => {});\n});\n")
	write(t, root, "e2e/search.spec.ts", "import { test } from '@playwright/test';\ntest('search', async () => {});\n")
	write(t, root, "e2e/admin/users.spec.ts", "import { test } from '@playwright/test';\ntest('users', async () => {});\n")
	write(t, root, "e2e/shared/page.ts", "export const title = 'home';\n")
	template, err := os.ReadFile(filepath.Join("testdata", "playwright-list", "multi-project.json"))
	if err != nil {
		t.Fatal(err)
	}
	return root, bytes.ReplaceAll(template, []byte("@ROOT@"), []byte(root))
}

func multiProjectListUnits() []PlaywrightDiscoveryUnit {
	return []PlaywrightDiscoveryUnit{
		{Project: "admin", Test: "e2e/admin/users.spec.ts"},
		{Project: "chromium", Test: "e2e/admin/users.spec.ts"}, {Project: "chromium", Test: "e2e/home.spec.ts"}, {Project: "chromium", Test: "e2e/search.spec.ts"},
		{Project: "firefox", Test: "e2e/admin/users.spec.ts"}, {Project: "firefox", Test: "e2e/home.spec.ts"}, {Project: "firefox", Test: "e2e/search.spec.ts"},
		{Project: "setup", Test: "e2e/auth.setup.ts"},
	}
}

// GitHub #709 part 2 (V1-1066): a real multi-project listing produces the canonical receipt
// and that receipt reconciles to MATCHED (TJAA-V0-019).
func TestPlaywrightDiscoveryFromListMultiProject_V1_1066(t *testing.T) {
	root, listing := multiProjectListFixture(t)
	raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, listing)
	if err != nil {
		t.Fatal(err)
	}
	if want := discoveryFixtureBytes(t, root, multiProjectListUnits()); !bytes.Equal(raw, want) {
		t.Fatalf("receipt\n%s\nwant\n%s", raw, want)
	}
	again, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, listing)
	if err != nil || !bytes.Equal(raw, again) {
		t.Fatalf("repeated bytes differ: %v", err)
	}
	plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, raw)
	if err != nil {
		t.Fatal(err)
	}
	// MATCHED proves the universe; the fixture's unresolved bare imports still widen the
	// selection to the whole matched universe (TJAA-V0-015), which is not a discovery failure.
	if plan.Discovery.State != "MATCHED" || len(plan.Discovery.OnlyInReceipt) != 0 || len(plan.Discovery.OnlyInStatic) != 0 || len(plan.Selected) != len(multiProjectListUnits()) {
		t.Fatalf("discovery=%+v selected=%d unknown=%+v", plan.Discovery, len(plan.Selected), plan.Unknown)
	}
	if units, state := VerifyPlaywrightDiscovery(root, "playwright.config.ts", discoveryFixtureRevision, raw); state != "MATCHED" || !slices.Equal(units, multiProjectListUnits()) {
		t.Fatal(state, units)
	}
	// Playwright reports `file` relative to rootDir (here e2e/), not to the repository. A receipt
	// that copies those names verbatim cannot reconcile: every unit is on both sides.
	naive := make([]PlaywrightDiscoveryUnit, 0, len(multiProjectListUnits()))
	for _, unit := range multiProjectListUnits() {
		naive = append(naive, PlaywrightDiscoveryUnit{Project: unit.Project, Test: strings.TrimPrefix(unit.Test, "e2e/")})
	}
	slices.SortFunc(naive, func(a, b PlaywrightDiscoveryUnit) int {
		if discoveryUnitLess(a, b) {
			return -1
		}
		return 1
	})
	plan, err = SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, discoveryFixtureBytes(t, root, naive))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Discovery.State != "UNIVERSE_MISMATCH" || len(plan.Discovery.OnlyInReceipt) != len(naive) || len(plan.Discovery.OnlyInStatic) != len(naive) {
		t.Fatalf("naive receipt: %+v", plan.Discovery)
	}
}

func TestPlaywrightDiscoveryFromListRefusals_V1_1066(t *testing.T) {
	root, listing := multiProjectListFixture(t)
	edit := func(change func(report, config map[string]any)) []byte {
		var copied map[string]any
		if err := json.Unmarshal(listing, &copied); err != nil {
			t.Fatal(err)
		}
		change(copied, copied["config"].(map[string]any))
		raw, err := json.Marshal(copied)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	argv := func(extra ...string) func(map[string]any, map[string]any) {
		return func(_, config map[string]any) {
			config["argv"] = append([]any{"/usr/local/bin/node", root + "/node_modules/.bin/playwright", "test", "--list", "--reporter=json"}, toAny(extra)...)
		}
	}
	firstSpec := func(report map[string]any) map[string]any {
		return report["suites"].([]any)[0].(map[string]any)["specs"].([]any)[0].(map[string]any)
	}
	outside := t.TempDir()
	write(t, outside, "playwright.config.ts", "export default {}\n")
	for _, row := range []struct {
		name, want string
		listing    []byte
		revision   string
	}{
		{"not-json", "not a JSON report", []byte("Listing tests:\n"), ""},
		{"bound", "bound", bytes.Repeat([]byte(" "), PlaywrightListMaxBytes+1), ""},
		{"no-config", "lacks config", edit(func(report, _ map[string]any) { delete(report, "config") }), ""},
		{"errors", "1 error", edit(func(report, _ map[string]any) { report["errors"] = []any{map[string]any{"message": "SyntaxError"}} }), ""},
		{"project-filter", `"--project"`, edit(argv("--project", "chromium")), ""},
		{"file-filter", `"home"`, edit(argv("home")), ""},
		{"grep", `"--grep=search"`, edit(argv("--grep=search")), ""},
		{"only-changed", `"--only-changed"`, edit(argv("--only-changed")), ""},
		{"other-reporter", `"line"`, edit(argv("--reporter", "line")), ""},
		{"no-argv", "argv", edit(func(_, config map[string]any) { delete(config, "argv") }), ""},
		// An execution report (test.only applies; --list disables it) is not a discovery listing.
		{"execution-report", "--list", edit(func(_, config map[string]any) {
			config["argv"] = []any{"/usr/local/bin/node", root + "/node_modules/.bin/playwright", "test", "--reporter=json"}
		}), ""},
		{"shard", "sharded", edit(func(_, config map[string]any) { config["shard"] = map[string]any{"current": 1, "total": 2} }), ""},
		{"other-config", "is not playwright.config.ts", edit(func(_, config map[string]any) { config["configFile"] = filepath.Join(outside, "playwright.config.ts") }), ""},
		{"root-dir-outside", "outside the repository root", edit(func(_, config map[string]any) { config["rootDir"] = outside }), ""},
		{"escaping-file", "not a repository-relative source path", edit(func(report, _ map[string]any) { firstSpec(report)["file"] = "../../escape.spec.ts" }), ""},
		{"missing-file", "not a repository-relative source path", edit(func(report, _ map[string]any) { firstSpec(report)["file"] = "absent.spec.ts" }), ""},
		{"no-project-name", "no projectName", edit(func(report, _ map[string]any) {
			delete(firstSpec(report)["tests"].([]any)[0].(map[string]any), "projectName")
		}), ""},
		{"revision", "revision", listing, "HEAD"},
	} {
		t.Run(row.name, func(t *testing.T) {
			revision := discoveryFixtureRevision
			if row.revision != "" {
				revision = row.revision
			}
			raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", revision, row.listing)
			if err == nil || raw != nil || !strings.Contains(err.Error(), row.want) {
				t.Fatalf("raw=%s err=%v want %q", raw, err, row.want)
			}
		})
	}
}

func toAny(values []string) []any {
	out := make([]any, len(values))
	for index, value := range values {
		out[index] = value
	}
	return out
}

// minimalPlaywrightListing is the subset of a `playwright test --list --reporter=json` report the
// producer reads: one suite per file under rootDir, each with one test per named project.
func minimalPlaywrightListing(t *testing.T, root, rootDir string, projects, files []string) []byte {
	t.Helper()
	suites := []any{}
	for _, file := range files {
		tests := []any{}
		for _, project := range projects {
			tests = append(tests, map[string]any{"projectName": project})
		}
		suites = append(suites, map[string]any{"file": file, "specs": []any{map[string]any{"file": file, "tests": tests}}, "suites": []any{}})
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

// GitHub #709 review: the producer stamps the current HEAD and source digest, so it must
// independently enumerate the files the config selects and refuse a listing that is stale,
// names a file the config does not select, or cannot be checked statically (TJAA-V0-019).
func TestPlaywrightDiscoveryFromListMembership_V1_1066(t *testing.T) {
	// The new spec is found by path alone, so a stale listing is caught even when the static
	// profile cannot parse the file (.mts/.cts) or read it as UTF-8.
	for _, row := range []struct{ name, body string }{
		{"b.spec.ts", "test('b', async () => {});\n"},
		{"b.spec.mts", "test('b', async () => {});\n"},
		{"b.spec.cts", "test('b', async () => {});\n"},
		{"b.test.mjs", "test('b', async () => {});\n"},
		{"b.spec.ts", "test('\xff', async () => {});\n"},
		// Playwright's default testMatch is matched case-insensitively.
		{"B.SPEC.ts", "test('b', async () => {});\n"},
		{"c.Test.js", "test('c', async () => {});\n"},
	} {
		t.Run("stale listing after a new "+row.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "playwright.config.ts", "export default defineConfig({ projects: [{ name: 'p', testDir: 'p' }] });\n")
			write(t, root, "p/a.spec.ts", "test('a', async () => {});\n")
			listing := minimalPlaywrightListing(t, root, "p", []string{"p"}, []string{"a.spec.ts"})
			if _, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, listing); err != nil {
				t.Fatalf("fresh listing refused: %v", err)
			}
			write(t, root, "p/"+row.name, row.body)
			raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, listing)
			if err == nil || raw != nil || !strings.Contains(err.Error(), `project "p" test p/`+row.name) {
				t.Fatalf("stale listing stamped: raw=%s err=%v", raw, err)
			}
		})
	}
	// Playwright prefixes `**/` to a string glob without it, so `b.spec.ts` matches p/b.spec.ts.
	t.Run("stale listing after a new file a relative glob selects", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "playwright.config.ts", "export default defineConfig({ projects: [{ name: 'p', testDir: 'p', testMatch: ['**/a.spec.ts', 'b.spec.ts'] }] });\n")
		write(t, root, "p/a.spec.ts", "test('a', async () => {});\n")
		listing := minimalPlaywrightListing(t, root, "p", []string{"p"}, []string{"a.spec.ts"})
		if _, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, listing); err != nil {
			t.Fatalf("fresh listing refused: %v", err)
		}
		write(t, root, "p/b.spec.ts", "test('b', async () => {});\n")
		raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, listing)
		if err == nil || raw != nil || !strings.Contains(err.Error(), `project "p" test p/b.spec.ts`) {
			t.Fatalf("stale listing stamped: raw=%s err=%v", raw, err)
		}
	})
	// GitHub #709 review round 5: minimatch treats `**` as globstar only as a whole path component
	// (elsewhere it is `*`), and Playwright tests a regular expression from lastIndex 0, so a sticky
	// one cannot match an absolute path. Neither ignore removes e2e/sub/b.spec.ts in Playwright, so
	// a listing captured before that file existed is stale and must not be stamped.
	// Round 6: in JavaScript `\A` is a literal A, so `/\A.*b\.spec\.ts$/` ignores nothing here; Go
	// reads it as a begin-text anchor and would ignore e2e/sub/b.spec.ts.
	// Round 7: JavaScript reads `b{01}` as one b (Go as the literal text `b{01}`), and `😀?` as a
	// high surrogate followed by an optional low surrogate (Go as an optional emoji), so in
	// JavaScript both configs select e2e/sub/b.spec.ts.
	for _, matcher := range []string{`testIgnore: '**/e2e/**.spec.ts'`, `testIgnore: /b\.spec\.ts$/y`, `testIgnore: /\A.*b\.spec\.ts$/`,
		`testMatch: /keep\.test\.ts$|b{01}\.spec\.ts$/`, "testIgnore: /\U0001F600?b\\.spec\\.ts$/"} {
		t.Run("stale listing after a new file "+matcher+" keeps", func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "playwright.config.ts", "export default defineConfig({ projects: [{ name: 'p', testDir: 'e2e', "+matcher+" }] });\n")
			write(t, root, "e2e/keep.test.ts", "test('keep', async () => {});\n")
			listing := minimalPlaywrightListing(t, root, "e2e", []string{"p"}, []string{"keep.test.ts"})
			write(t, root, "e2e/sub/b.spec.ts", "test('b', async () => {});\n")
			raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, listing)
			if err == nil || raw != nil || !strings.Contains(err.Error(), `project "p" test e2e/sub/b.spec.ts`) && !strings.Contains(err.Error(), "not static") {
				t.Fatalf("stale listing stamped: raw=%s err=%v", raw, err)
			}
		})
	}
	// GitHub #709 review round 5 follow-up: Go's `.` and `(?m)` anchors treat only LF as a line
	// terminator, JavaScript also CR, U+2028 and U+2029, so a path holding one refuses the listing
	// whether the config selects it (and the listing names it) or not.
	for _, row := range []struct{ name, file, match string }{
		{"listed and selected", "e2e/a\u2028b.spec.ts", "**/*.spec.ts"},
		{"unselected candidate", "e2e/x\ry.ts", "**/keep.test.ts"},
		{"outside the Basic Multilingual Plane", "e2e/a\U0001F600b.spec.ts", "**/*.spec.ts"},
	} {
		t.Run("line terminator in path "+row.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "playwright.config.ts", "export default defineConfig({ projects: [{ name: 'p', testDir: 'e2e', testMatch: ['**/keep.test.ts', '"+row.match+"'] }] });\n")
			write(t, root, "e2e/keep.test.ts", "test('keep', async () => {});\n")
			write(t, root, row.file, "test('b', async () => {});\n")
			files := []string{"keep.test.ts"}
			if row.match == "**/*.spec.ts" {
				files = append(files, strings.TrimPrefix(row.file, "e2e/"))
			}
			raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, minimalPlaywrightListing(t, root, "e2e", []string{"p"}, files))
			if err == nil || raw != nil || !strings.Contains(err.Error(), "line terminator") {
				t.Fatalf("listing over a %q path stamped: raw=%s err=%v", row.file, raw, err)
			}
		})
	}
	t.Run("unparsed module spec outside every testDir", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "playwright.config.ts", "export default defineConfig({ projects: [{ name: 'p', testDir: 'p' }] });\n")
		write(t, root, "p/a.spec.ts", "test('a', async () => {});\n")
		write(t, root, "unit/b.spec.mts", "test('b', async () => {});\n")
		if _, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, minimalPlaywrightListing(t, root, "p", []string{"p"}, []string{"a.spec.ts"})); err != nil {
			t.Fatalf("a file no project selects refused the listing: %v", err)
		}
	})
	// The shared source walker skips build output, vendored and hidden directories, so a test file
	// Playwright selects inside one is outside the bound source digest and refuses the listing.
	for _, row := range []struct{ file, want string }{
		{"e2e/build/b.spec.ts", "e2e/build"},
		{"e2e/build/b.spec.mts", "e2e/build"},
		{"e2e/flows/dist/c.test.js", "e2e/flows/dist"},
		{"e2e/vendor/d.spec.tsx", "e2e/vendor"},
		{"e2e/.cache/e.spec.ts", "e2e/.cache"},
		{"e2e/target/deep/f.spec.cjs", "e2e/target"},
		{"e2e/build/B.SPEC.ts", "e2e/build"},
	} {
		t.Run("selected test in excluded directory "+row.file, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "playwright.config.ts", "export default defineConfig({ testDir: 'e2e', projects: [{ name: 'p' }] });\n")
			write(t, root, "e2e/a.spec.ts", "test('a', async () => {});\n")
			listing := minimalPlaywrightListing(t, root, "e2e", []string{"p"}, []string{"a.spec.ts"})
			if _, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, listing); err != nil {
				t.Fatalf("fresh listing refused: %v", err)
			}
			write(t, root, row.file, "test('b', async () => {});\n")
			raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, listing)
			if err == nil || raw != nil || !strings.Contains(err.Error(), "directory "+row.want) {
				t.Fatalf("test in an excluded directory stamped: raw=%s err=%v", raw, err)
			}
		})
	}
	t.Run("testDir inside an excluded directory", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "playwright.config.ts", "export default defineConfig({ testDir: 'build/e2e', projects: [{ name: 'p' }] });\n")
		write(t, root, "build/e2e/a.spec.ts", "test('a', async () => {});\n")
		raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, minimalPlaywrightListing(t, root, "build/e2e", []string{"p"}, []string{"a.spec.ts"}))
		if err == nil || raw != nil || !strings.Contains(err.Error(), "directory build") {
			t.Fatalf("raw=%s err=%v", raw, err)
		}
	})
	t.Run("testDir reached through a symbolic link", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "playwright.config.ts", "export default defineConfig({ testDir: 'e2e', projects: [{ name: 'p' }] });\n")
		write(t, root, "suites/a.spec.ts", "test('a', async () => {});\n")
		if err := os.Symlink("suites", filepath.Join(root, "e2e")); err != nil {
			t.Fatal(err)
		}
		raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, minimalPlaywrightListing(t, root, "e2e", []string{"p"}, []string{"a.spec.ts"}))
		if err == nil || raw != nil || !strings.Contains(err.Error(), "symbolic link") {
			t.Fatalf("raw=%s err=%v", raw, err)
		}
	})
	t.Run("excluded directories Playwright does not select from", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "playwright.config.ts", "export default defineConfig({ projects: [{ name: 'p' }] });\n")
		write(t, root, "a.spec.ts", "test('a', async () => {});\n")
		write(t, root, ".git/HEAD", "ref: refs/heads/main\n")
		write(t, root, ".auth/user.json", "{}\n")
		write(t, root, "dist/bundle.js", "export const x = 1;\n")
		write(t, root, "node_modules/pkg/x.spec.ts", "test('x', async () => {});\n") // Playwright never descends node_modules
		write(t, root, "vendor/lib/helper.ts", "export const y = 1;\n")
		if err := os.MkdirAll(filepath.Join(root, "unit", "target"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, minimalPlaywrightListing(t, root, ".", []string{"p"}, []string{"a.spec.ts"})); err != nil {
			t.Fatalf("an excluded directory holding no selected test refused the listing: %v", err)
		}
	})
	t.Run("listed file the static profile cannot read as UTF-8", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "playwright.config.ts", "export default defineConfig({ projects: [{ name: 'p', testDir: 'p' }] });\n")
		write(t, root, "p/a.spec.ts", "test('a', async () => {});\n")
		listing := minimalPlaywrightListing(t, root, "p", []string{"p"}, []string{"a.spec.ts"})
		write(t, root, "p/a.spec.ts", "test('\xff', async () => {});\n")
		raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, listing)
		if err == nil || raw != nil || !strings.Contains(err.Error(), `project "p" test p/a.spec.ts`) {
			t.Fatalf("non-UTF-8 listed test stamped: raw=%s err=%v", raw, err)
		}
	})
	root, listing := multiProjectListFixture(t)
	for _, row := range []struct{ name, config, want string }{
		{"dynamic testDir", "export default defineConfig({ testDir: process.env.DIR, projects: [{ name: 'chromium' }] });\n", "not static"},
		{"dynamic project set", "export default defineConfig({ projects: projectList });\n", "not static"},
		{"global testMatch", "export default defineConfig({ testDir: './e2e', testMatch: '**/*.e2e.ts', projects: [{ name: 'chromium' }] });\n", "not static"},
	} {
		t.Run(row.name, func(t *testing.T) {
			write(t, root, "playwright.config.ts", row.config)
			raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, listing)
			if err == nil || raw != nil || !strings.Contains(err.Error(), row.want) {
				t.Fatalf("raw=%s err=%v want %q", raw, err, row.want)
			}
		})
	}
	t.Run("listed file the config does not select", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "playwright.config.ts", "export default defineConfig({ projects: [{ name: 'p', testDir: 'p' }] });\n")
		write(t, root, "p/a.spec.ts", "test('a', async () => {});\n")
		write(t, root, "p/helper.ts", "export const x = 1;\n")
		raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, minimalPlaywrightListing(t, root, "p", []string{"p"}, []string{"a.spec.ts", "helper.ts"}))
		if err == nil || raw != nil || !strings.Contains(err.Error(), `names project "p" test p/helper.ts`) {
			t.Fatalf("raw=%s err=%v", raw, err)
		}
	})
	t.Run("implicit default project", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "playwright.config.ts", "export default { testDir: 'e2e' };\n")
		write(t, root, "e2e/a.spec.ts", "test('a', async () => {});\n")
		raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, minimalPlaywrightListing(t, root, "e2e", []string{""}, []string{"a.spec.ts"}))
		if err != nil || !bytes.Contains(raw, []byte(`"units":[{"project":"","test":"e2e/a.spec.ts"}]`)) {
			t.Fatalf("raw=%s err=%v", raw, err)
		}
	})
}
