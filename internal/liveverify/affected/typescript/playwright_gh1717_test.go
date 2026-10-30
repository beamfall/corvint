package typescript

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// gh1717Config is the reporter's project shape (GitHub #1717): a setup project per literal
// testMatch, each with testDir './', and a test project whose dependencies are a ternary on
// process.env. Line 6 is the global-setup testMatch and line 7 the setup testMatch.
func gh1717Config(globalSetup, setup string) string {
	return "import { defineConfig, devices } from '@playwright/test';\n" +
		"const testProjectDependencies = process.env.SKIP_SETUP === '1' ? [] : ['setup'];\n" +
		"export default defineConfig({\n" +
		"  testDir: './tests',\n" +
		"  projects: [\n" +
		"    { name: 'global-setup', testDir: './', testMatch: " + globalSetup + " },\n" +
		"    { name: 'setup', testDir: './', testMatch: " + setup + ", dependencies: process.env.CI ? ['global-setup'] : [] },\n" +
		"    { name: 'chromium', use: { ...devices['Desktop Chrome'] }, dependencies: testProjectDependencies },\n" +
		"  ],\n" +
		"});\n"
}

func gh1717Repository(t *testing.T, config string) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
	write(t, root, "playwright.config.ts", config)
	write(t, root, "setup/global-setup.ts", "import { test as setup } from '@playwright/test';\nsetup('global', async () => {});\n")
	write(t, root, "setup/auth/user.setup.ts", "import { test as setup } from '@playwright/test';\nsetup('user', async () => {});\n")
	write(t, root, "lib/fixtures/control.ts", "export function splitIds(value: string) {\n  return value.split(',');\n}\n")
	write(t, root, "tests/a.spec.ts", "import { test } from '@playwright/test';\nimport { splitIds } from '../lib/fixtures/control';\ntest('a', async () => { splitIds('x'); });\n")
	return root
}

// TJAA-V0-092 (GitHub #1717 item 1): in path mode a literal project testMatch with a globstar
// after its first segment resolves when no test path's membership depends on directories above
// the repository, and the config's computed dependencies never raise the executable-config
// frontier; a testMatch the rule refuses names its own line, not the first testMatch line.
func TestPlaywrightPathModeGlobstarTestMatch_GH1717(t *testing.T) {
	const literal, globstar = "'setup/global-setup.ts'", "'setup/**/*.setup.ts'"
	line7 := FrontierConfig + ":playwright.config.ts:7 `testMatch`"
	for _, row := range []struct {
		label, globalSetup, setup, want string
		extra                           map[string]string
	}{
		{"reporter shape", literal, globstar, "", nil},
		{"globstar array", literal, "['setup/**/*.setup.ts', 'setup/global-setup.ts']", "", nil},
		{"leading and inner globstar", literal, "'**/setup/**/*.setup.ts'", "", nil},
		{"tail matched outside the setup directory", literal, globstar, line7, map[string]string{"tests/stray.setup.ts": "export const stray = 1;\n"}},
		{"commented globstar reached outside the setup directory", literal, "/* setup */ 'setup/**/*.setup.ts'", line7, map[string]string{"tests/stray.setup.ts": "export const stray = 1;\n"}},
		{"trailing globstar", literal, "'setup/**'", line7, nil},
		{"globstar inside a segment", literal, "'setup/a**/*.setup.ts'", line7, nil},
		{"computed second testMatch", literal, "process.env.SETUP_MATCH", line7, nil},
		{"regular expression second testMatch", literal, "/.*\\.setup\\.ts/", line7, nil},
		{"computed first testMatch", "process.env.GLOBAL_MATCH", globstar, FrontierConfig + ":playwright.config.ts:6 `testMatch`", nil},
	} {
		t.Run(row.label, func(t *testing.T) {
			root := gh1717Repository(t, gh1717Config(row.globalSetup, row.setup))
			for name, body := range row.extra {
				write(t, root, name, body)
			}
			result, err := New().Units(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			config := slices.DeleteFunc(slices.Clone(result.Frontier), func(entry string) bool { return !strings.HasPrefix(entry, FrontierConfig) })
			if row.want == "" && len(config) != 0 || row.want != "" && !slices.Equal(config, []string{row.want}) {
				t.Fatalf("executable-config sites %q, want %q", config, row.want)
			}
		})
	}
}

// TJAA-V0-092: playwrightGlobReachesRoot decides a globstar glob from the path's own segments
// only when no prefix above the repository can change the match.
func TestPlaywrightGlobstarReachesRoot_GH1717(t *testing.T) {
	for _, row := range []struct {
		glob, name string
		reaches    bool
	}{
		{"'setup/**/*.setup.ts'", "setup/a/b/user.setup.ts", false},
		{"'setup/**/*.setup.ts'", "setup/user.setup.ts", false},
		{"'setup/**/*.setup.ts'", "x/setup/user.setup.ts", false},
		{"'setup/**/*.setup.ts'", "lib/control.ts", false},
		{"'setup/**/*.setup.ts'", "tests/user.setup.ts", true},
		{"'setup/**/*.setup.ts'", "user.setup.ts", true},
		{"'a/b/**/c.ts'", "b/x/c.ts", true},
		{"'a/b/**/c.ts'", "x/c.ts", true},
		{"'a/b/**/c.ts'", "x/d.ts", false},
		{"'setup/global-setup.ts'", "global-setup.ts", true},
		{"'setup/global-setup.ts'", "setup/global-setup.ts", false},
		{"'setup/**'", "setup/a.ts", true},
	} {
		if got := playwrightGlobReachesRoot(row.glob, row.name); got != row.reaches {
			t.Errorf("playwrightGlobReachesRoot(%s, %s) = %v, want %v", row.glob, row.name, got, row.reaches)
		}
	}
}

// TJAA-V0-093 (GitHub #1717 item 2): a computed index into a fresh array, the result of a direct
// `.split(...)` or `.map(...)` call or an array literal, leaves the tag closure exact, unless a
// closure file defines or reads `split` or `map` other than as a direct method call; any other
// computed member access still stops and names its line.
func TestPlaywrightTagClosureFreshArrayIndex_GH1717(t *testing.T) {
	const stop = "e2e/plain.spec.ts holds a tag or construct that is not one literal @\\S+ string (line "
	const computed = "a computed member access can reach a tag array without naming it)"
	control := "function splitIds(value: string) {\n  return value.split(',');\n}\nexport function pick(each: string, index: number) {\n  return splitIds(each.split(';')[index] ?? '');\n}\n"
	for _, row := range []struct {
		label, plain, stop string
		helper             string
	}{
		{"split result in an imported helper", "import { pick } from '../lib/control';\ntest('plain', async () => { pick('a;b', 1); });\n", "", control},
		{"split result", "const pick = (each: string, index: number) => each.split(';')[index] ?? '';\ntest('plain', async () => {});\n", "", ""},
		{"map result", "const ids = (all: string[], index: number) => all.map((id) => id.trim())[index];\ntest('plain', async () => {});\n", "", ""},
		{"array literal", "const pick = (index: number) => ['a', 'b'][index];\ntest('plain', async () => {});\n", "", ""},
		{"optional chain split", "const pick = (each?: string, index = 0) => each?.split(';')[index];\ntest('plain', async () => {});\n", "", ""},
		{"computed index on a name", "const details = { annotation: { type: 'a', description: 'b' } };\nconst key = 'annotation';\nconst value = details[key];\ntest('plain', async () => {});\n", stop + "3: " + computed, ""},
		{"computed index on a parenthesized name", "const all = ['a'];\nconst pick = (index: number) => (all)[index];\ntest('plain', async () => {});\n", stop + "2: " + computed, ""},
		{"computed index on an indexed literal", "const pick = (index: number) => [['a']][0][index];\ntest('plain', async () => {});\n", stop + "1: " + computed, ""},
		{"computed index on an indexed keyword property", "const box = { default: [['a']] };\nconst pick = (index: number) => box.default[0][index];\ntest('plain', async () => {});\n", stop + "2: " + computed, ""},
		{"computed index on another call", "const pick = (each: string, index: number) => each.slice(1)[index];\ntest('plain', async () => {});\n", stop + "1: " + computed, ""},
		{"split defined as a method", "const box = { split() { return box; } };\nconst pick = (index: number) => box.split()[index];\ntest('plain', async () => {});\n", stop + "1: `split` is defined or read other than as a direct method call, so a computed index on a split or map result can reach a tag array)", ""},
		{
			"map defined in an imported helper", "import { box } from '../lib/control';\nconst pick = (index: number) => box.map(() => 1)[index];\ntest('plain', async () => {});\n",
			"lib/control.ts holds a tag or construct that is not one literal @\\S+ string (line 1: `map` is defined or read other than as a direct method call, so a computed index on a split or map result can reach a tag array)",
			"export const box = { map: () => box };\n",
		},
		{"computed write on a split result", "const each = 'a;b';\neach.split(';')[0] = 'x';\ntest('plain', async () => {});\n", stop + "2: a computed member write can set a tag)", ""},
	} {
		t.Run(row.label, func(t *testing.T) {
			root := grepFixture(t, grepRepro)
			if row.helper != "" {
				write(t, root, "lib/control.ts", row.helper)
			}
			write(t, root, "e2e/plain.spec.ts", row.plain)
			unverified, omissions := gh1647Omission(t, root, grepReproCases())
			if row.stop == "" {
				if len(unverified) != 0 || len(omissions) != 0 {
					t.Fatalf("not verified: unverified=%+v omissions=%+v", unverified, omissions)
				}
				return
			}
			if len(omissions) != 1 || !strings.Contains(omissions[0].Detail, row.stop) {
				t.Fatalf("omissions=%+v, want a stop naming %q", omissions, row.stop)
			}
		})
	}
}
