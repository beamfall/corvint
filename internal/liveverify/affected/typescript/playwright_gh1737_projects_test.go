package typescript

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// gh1737ProjectsConfig is the GitHub #1737 residual shape: a top-level testDir and three browser
// projects, each with the same project testIgnore on line 5 + index.
func gh1737ProjectsConfig(ignore string) string {
	config := "import { defineConfig, devices } from '@playwright/test';\n" +
		"export default defineConfig({\n" +
		"  testDir: './tests',\n" +
		"  projects: [\n"
	for _, browser := range []string{"Chrome", "Firefox", "Safari"} {
		config += "    { name: '" + strings.ToLower(browser) + "', use: { ...devices['Desktop " + browser + "'] }, testIgnore: " + ignore + " },\n"
	}
	return config + "  ],\n});\n"
}

// TJAA-V0-105 (GitHub #1737 residual, V1-2675): a literal root-free testIgnore repeated in several
// projects[] entries raises no executable-config frontier, even when only directories above the
// repository could make it match a test path, because such a match can only drop that path from a
// project. A computed testIgnore, or a regular expression TJAA-V0-127 does not decide, in a
// projects[] entry still names its line.
func TestPlaywrightPathModeProjectsTestIgnore_GH1737(t *testing.T) {
	line5 := FrontierConfig + ":playwright.config.ts:5 `testIgnore`"
	for _, row := range []struct {
		label, ignore, want string
	}{
		{"reporter literal in three projects", "'tests/examples/**/*.spec.ts'", ""},
		{"literal array in three projects", "['tests/examples/**/*.spec.ts', 'tests/skip/*.spec.ts']", ""},
		{"computed testIgnore", "process.env.IGNORE", line5},
		// TJAA-V0-127: a context-free static regex testIgnore resolves like a root-free glob; a
		// start-anchored one still names its line.
		{"regular expression testIgnore", "/examples/", ""},
		{"start-anchored regular expression testIgnore", "/^examples/", line5},
		{"trailing globstar testIgnore", "'tests/examples/**'", line5},
	} {
		t.Run(row.label, func(t *testing.T) {
			root := gh1717Repository(t, gh1737ProjectsConfig(row.ignore))
			write(t, root, "tests/examples/e.spec.ts", "import { test } from '@playwright/test';\ntest('e', async () => {});\n")
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
