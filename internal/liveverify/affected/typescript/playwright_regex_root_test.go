package typescript

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// TJAA-V0-127 (V1-2584): a static regular-expression project testMatch resolves in path mode when
// its verdict on every test path is independent of the checkout directory; an unanchored regex, a
// start anchor, one that can match `/` without deciding the path itself, or a flag outside i, m
// and s keeps the testMatch site.
func TestPlaywrightPathModeRegexTestMatch_V12584(t *testing.T) {
	at := FrontierConfig + ":playwright.config.ts:6 `testMatch`"
	for _, row := range []struct {
		label, match, want string
	}{
		{"end anchored basename", `/global-setup\.ts$/`, ""},
		{"end anchored case-insensitive", `/GLOBAL-SETUP\.TS$/i`, ""},
		{"end anchored alternation", `/(?:global-setup|teardown)\.ts$/`, ""},
		{"array with a glob", `[/global-setup\.ts$/, 'setup/teardown.ts']`, ""},
		{"unanchored", `/global-setup\.ts/`, at},
		{"start anchored", `/^.*global-setup\.ts$/`, at},
		{"slash before the anchor", `/setup\/global-setup\.ts$/`, at},
		{"dot can match a slash", `/.*\.setup\.ts$/`, at},
		{"word boundary", `/\bglobal-setup\.ts$/`, at},
		{"unicode flag", `/global-setup\.ts$/u`, at},
		{"lookahead", `/(?=g)global-setup\.ts$/`, at},
	} {
		t.Run(row.label, func(t *testing.T) {
			root := gh1688Repository(t, "    { name: 'auth', testMatch: "+row.match+" },\n"+gh1688Projects)
			write(t, root, "tests/setup/global-setup.ts", "import { test as setup } from '@playwright/test';\nsetup('authenticate', async () => {});\n")
			write(t, root, "tests/setup/teardown.ts", "import { test as teardown } from '@playwright/test';\nteardown('clean', async () => {});\n")
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

// TJAA-V0-127: PlaywrightRegexReachesRoot decides a regex verdict from `/<path>` alone only when
// no checkout directory can change it.
func TestPlaywrightRegexReachesRoot_V12584(t *testing.T) {
	for _, row := range []struct {
		source, name     string
		reaches, matches bool
	}{
		{`/global-setup\.ts$/`, "tests/setup/global-setup.ts", false, true},
		{`/global-setup\.ts$/`, "tests/a.spec.ts", false, false},
		{`/global-setup\.ts/`, "tests/setup/global-setup.ts", false, true},
		{`/global-setup\.ts/`, "tests/a.spec.ts", true, false},
		{`/\/e2e\/.*\.spec\.ts$/`, "e2e/a.spec.ts", false, true},
		{`/\/e2e\/.*\.spec\.ts$/`, "tests/a.spec.ts", true, false},
		{`/[^x]\.ts$/`, "tests/b.js", true, false},
		{`/[a-z]\.ts$/`, "tests/b.js", false, false},
		{`/^tests\//`, "tests/a.spec.ts", true, false},
		{`/a\.ts$/y`, "tests/a.ts", true, false},
		{`'a.ts'`, "tests/a.ts", true, false},
		{`/a\.ts$/`, "tests/\U0001F600a.ts", true, true},
	} {
		if got := PlaywrightRegexReachesRoot(row.source, row.name); got != row.reaches {
			t.Errorf("PlaywrightRegexReachesRoot(%s, %s) = %v, want %v", row.source, row.name, got, row.reaches)
		}
		if !row.reaches {
			if got := PlaywrightRegexMatches(row.source, row.name); got != row.matches {
				t.Errorf("PlaywrightRegexMatches(%s, %s) = %v, want %v", row.source, row.name, got, row.matches)
			}
		}
	}
}
