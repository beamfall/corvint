package typescript

import (
	"maps"
	"slices"
	"strings"
	"testing"
)

// AFP-V0-417: a loader name inside string, template or regular-expression text is data, so the
// GitHub #1660 config, whose only `global` words are project names and dependency strings, resolves
// its executablePath; a real reference in code, a computed member key, or text the scan cannot
// see through still keeps the EXECUTION unknown, and the detail names the line.
func TestPlaywrightEnvLoaderNameInStringIsData_GH1660(t *testing.T) {
	unset := NewPlaywrightEnvironmentInput("CHROME_PATH", PlaywrightEnvironmentInvocation, "", false)
	data := "require('dotenv').config();\n" +
		"const testProjectDependencies = process.env.PW_LAZY_SEEDS === '1' ? ['global-setup'] : ['setup'];\n" +
		"const note = `global ${'setup'} text`;\n" +
		"const marker = /global-setup/;\n" +
		"const labels = ['loadEnvFile', 'global'];\n"
	projects, _, _ := gh1646Resolve(t, t.TempDir(), data, unset)
	for _, project := range projects {
		if project.Executable == nil || !project.Executable.Bundled || len(project.execution) != 0 {
			t.Fatalf("%s: string text must not count as a loader reference: execution=%+v executable=%+v", project.Name, project.execution, project.Executable)
		}
	}
	for _, row := range []struct{ label, setup, detail string }{
		{"global in code", "require('dotenv').config();\nconst label = 'global-setup';\nglobal.seed = 1;", "references global at playwright.config.ts:4"},
		{"globalThis in a template substitution", "const note = `x ${globalThis.seed}`;", "references globalThis at playwright.config.ts:2"},
		{"computed member key", "const box = {};\nbox['global'] = 1;", "references global at playwright.config.ts:3"},
		{"escape beside the text", "const label = 'global-setup\\x2d';", "references global at playwright.config.ts:2"},
		{"computed member call", "const labels = ['x'];\nprocess['loadEnvFile']('.env');", "references loadEnvFile at playwright.config.ts:3"},
		{"quoted destructuring key", "const { 'loadEnvFile': load } = process;\nload('.env');", "references loadEnvFile at playwright.config.ts:2"},
		{"computed destructuring key", "const { ['loadEnvFile']: load } = process;\nload('.env');", "references loadEnvFile at playwright.config.ts:2"},
	} {
		t.Run(row.label, func(t *testing.T) {
			// Such a config also refuses browser identity, so the writer is checked directly.
			if writer, _ := playwrightEnvironmentWriter("playwright.config.ts", gh1640Config(row.setup, "process.env.CHROME_PATH")); !strings.Contains(writer, row.detail) {
				t.Fatalf("writer=%q want %q", writer, row.detail)
			}
		})
	}
}

// AFP-V0-418, AFP-V0-419: the tag closure follows a bare specifier that the nearest tsconfig.json
// paths or baseUrl resolves to a repository file, as the dependency graph does, so the GitHub
// #1660 spec shape is verified; a specifier that resolves to no repository file still keeps the
// unit unverified, and the plan detail names the file and specifier where the closure stopped.
func TestPlaywrightGrepOmissionFollowsTypeScriptAliases_GH1660(t *testing.T) {
	smoke := func() []grepListCase {
		cases := grepReproCases()
		cases[0].tags = []string{"smoke"}
		return cases
	}
	spec := "import { expect, type Page } from '@playwright/test';\nimport envConfig from 'lib/configs';\nimport { test } from 'lib/fixtures';\n" +
		"test.describe('d', { tag: ['@smoke'] }, () => { test('plain', async () => {}); });\n"
	lib := map[string]string{
		"lib/configs.ts":  "export default { name: 'x' };\n",
		"lib/fixtures.ts": "export { test } from '@playwright/test';\n",
	}
	const rootPaths = `{ "compilerOptions": { "paths": { "lib/*": ["./lib/*"] } } }`
	const supportPaths = `{ "compilerOptions": { "paths": { "lib/*": ["./support/*"] } } }`
	hidden := "export const extra = { tag: ['@smoke hidden'] };\n"
	// Playwright maps a specifier of a module that is not .ts or .tsx only under allowJs.
	jsImporter := map[string]string{
		"e2e/plain.spec.ts": "import envConfig from 'lib/configs';\nimport { test } from './helpers';\n" +
			"test.describe('d', { tag: ['@smoke'] }, () => { test('plain', async () => {}); });\n",
		"e2e/helpers.js": "export { test } from 'lib/fixtures';\n",
	}
	merge := func(base, more map[string]string) map[string]string {
		out := maps.Clone(base)
		maps.Copy(out, more)
		return out
	}
	for _, row := range []struct {
		label, tsconfig, stop string
		extra                 map[string]string
	}{
		{"paths alias", `{ "compilerOptions": { "baseUrl": ".", "paths": { "lib/*": ["./lib/*"] } } }`, "", nil},
		{"baseUrl only", `{ "compilerOptions": { "baseUrl": "." } }`, "", nil},
		{"alias target holds a hidden tag", `{ "compilerOptions": { "paths": { "lib/*": ["./lib/*"] } } }`, `lib/fixtures.ts holds a tag or construct`, map[string]string{"lib/fixtures.ts": "export const extra = { tag: ['@smoke hidden'] };\n"}},
		{"no tsconfig claims the specifier", "", `e2e/plain.spec.ts imports "lib/configs", which resolves to no single repository file`, nil},
		{"tsconfig with extends is not followed", `{ "extends": "./base.json", "compilerOptions": { "paths": { "lib/*": ["./lib/*"] } } }`, `imports "lib/configs"`, nil},
		{"alias to a missing file", `{ "compilerOptions": { "paths": { "lib/*": ["./missing/*"] } } }`, `imports "lib/configs"`, nil},
		{"nearer jsconfig redirects the alias", rootPaths, `e2e/support/fixtures.ts holds a tag or construct`, map[string]string{
			"e2e/jsconfig.json": supportPaths, "e2e/support/configs.ts": lib["lib/configs.ts"], "e2e/support/fixtures.ts": hidden}},
		{"nearer jsconfig with literal targets", rootPaths, "", map[string]string{
			"e2e/jsconfig.json": supportPaths, "e2e/support/configs.ts": lib["lib/configs.ts"], "e2e/support/fixtures.ts": lib["lib/fixtures.ts"]}},
		{"tsconfig beside a jsconfig wins", rootPaths, "", map[string]string{
			"e2e/tsconfig.json": supportPaths, "e2e/support/configs.ts": lib["lib/configs.ts"], "e2e/support/fixtures.ts": lib["lib/fixtures.ts"],
			"e2e/jsconfig.json": `{ "compilerOptions": { "paths": { "lib/*": ["./hidden/*"] } } }`, "e2e/hidden/configs.ts": hidden, "e2e/hidden/fixtures.ts": hidden}},
		{"js importer without allowJs", rootPaths, `e2e/helpers.js imports "lib/fixtures", which resolves to no single repository file`, jsImporter},
		{"js importer with allowJs", `{ "compilerOptions": { "allowJs": true, "paths": { "lib/*": ["./lib/*"] } } }`, "", jsImporter},
		{"js importer under a jsconfig", "", "", merge(jsImporter, map[string]string{"jsconfig.json": rootPaths})},
		{"js importer under a jsconfig without allowJs", "", `e2e/helpers.js imports "lib/fixtures"`, merge(jsImporter, map[string]string{"jsconfig.json": `{ "compilerOptions": { "allowJs": false, "paths": { "lib/*": ["./lib/*"] } } }`})},
		{"config names tsconfig", `{ "compilerOptions": { "baseUrl": "." } }`, `imports "lib/configs"`, map[string]string{"playwright.config.ts": "// run with --tsconfig ./tsconfig.e2e.json\nexport default defineConfig({ testDir: 'e2e', projects: [" + grepRepro + "] });\n"}},
	} {
		t.Run(row.label, func(t *testing.T) {
			root := grepFixture(t, grepRepro)
			write(t, root, "e2e/plain.spec.ts", spec)
			for file, content := range lib {
				write(t, root, file, content)
			}
			for file, content := range row.extra {
				write(t, root, file, content)
			}
			if row.tsconfig != "" {
				write(t, root, "tsconfig.json", row.tsconfig)
			}
			unverified, omissions := gh1647Omission(t, root, smoke())
			if row.stop == "" {
				if len(unverified) != 0 || len(omissions) != 0 {
					t.Fatalf("not verified: unverified=%+v omissions=%+v", unverified, omissions)
				}
				return
			}
			want := []PlaywrightDiscoveryUnit{{Project: "alt-a", Test: "e2e/plain.spec.ts"}, {Project: "alt-b", Test: "e2e/plain.spec.ts"}}
			if !slices.Equal(unverified, want) || len(omissions) != 1 ||
				!strings.Contains(omissions[0].Detail, "the tag closure of e2e/plain.spec.ts stops because") || !strings.Contains(omissions[0].Detail, row.stop) {
				t.Fatalf("unverified=%+v omissions=%+v want stop %q", unverified, omissions, row.stop)
			}
		})
	}
}

// AFP-V0-418: a jsconfig.json, which Playwright reads for aliases, is bound into the source
// identity, so editing one invalidates the discovery receipt.
func TestPlaywrightSourcesBindJSConfig_GH1660(t *testing.T) {
	root := grepFixture(t, grepRepro)
	before, err := ObservePlaywrightSources(root, "playwright.config.ts")
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, "e2e/jsconfig.json", `{ "compilerOptions": { "paths": { "lib/*": ["./support/*"] } } }`)
	after, err := ObservePlaywrightSources(root, "playwright.config.ts")
	if err != nil || after == before {
		t.Fatalf("a jsconfig.json is not bound: before=%s after=%s err=%v", before, after, err)
	}
	if !newPlaywrightTagSources(root, "playwright.config.ts").boundFiles()["e2e/jsconfig.json"] {
		t.Fatal("the tag closure does not bind e2e/jsconfig.json")
	}
}
