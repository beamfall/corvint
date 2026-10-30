package typescript

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// TJAA-V0-104 (GitHub #1737, V1-2631): every path-mode executable-config frontier site names the
// config line of the construct that blocked the static membership verdict: a membership unknown
// names the key, project entry or duplicate name it reports, an empty or computed `projects`
// names its key, `owns <file>` names the owning project's matcher (or the project when it uses
// the default matcher), and a matcher key reached only through an escaped key names the escaped
// key. A site with no construct to name keeps the bare config path and its token records why.
func TestPlaywrightPathModeConfigSitesNameLines_GH1737(t *testing.T) {
	const config = "import { defineConfig, devices } from '@playwright/test';\nconst deps = process.env.X === '1' ? ['setup'] : ['setup'];\nexport default defineConfig({\n  testDir: './tests',\n  projects: [\n"
	site := func(line, token string) string {
		return FrontierConfig + ":playwright.config.ts" + line + " `" + token + "`"
	}
	for _, row := range []struct {
		label, projects, config, want string
		files                         map[string]string
	}{
		{label: "empty projects", config: strings.Replace(config, "  projects: [\n", "  projects: [],\n  other: [\n", 1), want: site(":5", PlaywrightUnknownProjectSet)},
		{label: "computed projects", config: strings.Replace(config, "  projects: [\n", "  projects: makeProjects(),\n  other: [\n", 1), want: site(":5", PlaywrightUnknownProjectSet)},
		{label: "project testDir", projects: "    { name: 'auth',\n      testDir: process.env.DIR },\n" + gh1688Projects, want: site(":7", PlaywrightUnknownProjectMembership)},
		{label: "project named global", projects: "    { name: 'global',\n      testDir: process.env.DIR },\n" + gh1688Projects, want: site(":7", PlaywrightUnknownProjectMembership)},
		{label: "global testDir", config: strings.Replace(config, "'./tests'", "process.env.DIR", 1), projects: gh1688Projects, want: site(":4", PlaywrightUnknownProjectMembership)},
		{label: "project entry not static", projects: gh1688Projects + "    makeProject(),\n", want: site(":9", PlaywrightUnknownProjectSet)},
		{label: "project name not a string", projects: gh1688Projects + "    { name: process.env.N },\n", want: site(":9", PlaywrightUnknownProjectSet)},
		{label: "duplicate project name", projects: gh1688Projects + "    { name: 'setup', use: {} },\n", want: site(":9", PlaywrightUnknownProjectSet)},
		{label: "owns through a literal testMatch", projects: "    { name: 'auth', testMatch: 'setup/global-setup.ts' },\n" + gh1688Projects, files: map[string]string{"tests/setup/global-setup.ts": "import { it } from 'vitest';\nit('ready', () => {});\n"}, want: site(":6", "owns tests/setup/global-setup.ts")},
		{label: "owns through the default testMatch", projects: gh1688Projects, files: map[string]string{"tests/v.test.ts": "import { it } from 'vitest';\nit('ready', () => {});\n"}, want: site(":7", "owns tests/v.test.ts")},
		{label: "escaped testMatch key", projects: "    { name: 'auth', \"test\\u004datch\": /global-setup/ },\n" + gh1688Projects, want: site(":6", "testMatch")},
	} {
		t.Run(row.label, func(t *testing.T) {
			root := gh1688Repository(t, row.projects)
			if row.config != "" {
				write(t, root, "playwright.config.ts", row.config+row.projects+"  ],\n});\n")
			}
			for name, body := range row.files {
				write(t, root, name, body)
			}
			result, err := New().Units(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			sites := slices.DeleteFunc(slices.Clone(result.Frontier), func(entry string) bool { return !strings.HasPrefix(entry, FrontierConfig) })
			if !slices.Equal(sites, []string{row.want}) {
				t.Fatalf("executable-config sites %q, want %q", sites, row.want)
			}
		})
	}
}

// TJAA-V0-104: a site with no construct to name records why. playwrightUnknownAt reports the
// offset of the construct a membership unknown names, or -1 and the reason none has a line.
func TestPlaywrightUnknownAtRecordsWhyNoLine_TJAA104(t *testing.T) {
	body := "export default defineConfig({\n  projects: [\n    { name: 'a' },\n  ],\n});\n"
	for _, row := range []struct {
		detail string
		line   int
		why    string
	}{
		{"projects is absent", 0, "projects is absent"},
		{"config tokenization failed", 0, "the config does not tokenize"},
		{"top-level config object is dynamic", 1, ""},
		{"projects is not one nonempty static array", 2, ""},
		{"a metadata is dynamic", 0, "the project a does not spell the metadata key the unknown names"},
	} {
		at, why := playwrightUnknownAt(body, PlaywrightUnknown{Detail: row.detail})
		if row.line == 0 {
			if at >= 0 || !strings.Contains(why, row.why) {
				t.Fatalf("%s: at=%d why=%q, want no line and %q", row.detail, at, why, row.why)
			}
			continue
		}
		if at < 0 || lineAt(body, at) != row.line {
			t.Fatalf("%s: at=%d why=%q, want line %d", row.detail, at, why, row.line)
		}
	}
}
