package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestTestsSetupSelectProviderHandoffLive proves TSA-V0-871 (V1-1587 AC6): the `--format lines`
// output of `corvint tests setup-select` is handed to a real Playwright run as its setup project
// list, and that run executes only the selected setup files. A control run with an ALL selection
// executes every setup file, so the narrowing comes from the selection, not from the fixture.
func TestTestsSetupSelectProviderHandoffLive(t *testing.T) {
	modules := os.Getenv("CORVINT_PLAYWRIGHT_MODULES")
	if modules == "" {
		t.Skip("NOT_RUN: set CORVINT_PLAYWRIGHT_MODULES to installed node_modules (with @playwright/test) for live qualification")
	}
	node := os.Getenv("CORVINT_PLAYWRIGHT_NODE")
	if node == "" {
		var err error
		if node, err = exec.LookPath("node"); err != nil {
			t.Skip("NOT_RUN: no node on PATH; set CORVINT_PLAYWRIGHT_NODE")
		}
	}
	cli := filepath.Join(modules, "playwright", "cli.js")
	if _, err := os.Stat(cli); err != nil {
		t.Fatalf("CORVINT_PLAYWRIGHT_MODULES has no playwright/cli.js: %v", err)
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cemGit(t, root, "init", "-q")
	setupFile := func(id string) string {
		return "import { test as setup } from '@playwright/test';\n" +
			"import { appendFileSync } from 'fs';\n" +
			"import { SHARED } from '../lib/shared_map';\n" +
			"setup('seed " + id + "', () => {\n" +
			"  appendFileSync(process.env.CORVINT_SEED_LOG as string, SHARED.scenario_" + id + " + '\\n');\n" +
			"});\n"
	}
	testsCommit(t, root, map[string]string{
		"package.json":              `{"name":"handoff-fixture","private":true,"devDependencies":{"@playwright/test":"*"}}` + "\n",
		".gitignore":                "node_modules\ntest-results\nplaywright-report\n",
		"lib/shared_map.ts":         "export const SHARED = { scenario_a: 'alpha', scenario_b: 'beta' };\n",
		"lib/a_context.ts":          "import { SHARED } from './shared_map';\nexport const aContext = SHARED.scenario_a;\n",
		"setup/scenario_a.setup.ts": setupFile("a"),
		"setup/scenario_b.setup.ts": setupFile("b"),
		"tests/x.spec.ts": "import { test, expect } from '@playwright/test';\n" +
			"import { aContext } from '../lib/a_context';\n" +
			"test('uses scenario a', () => {\n  expect(aContext).toBe('alpha');\n});\n",
		// The setup project runs exactly the files named in CORVINT_SETUP_LIST, one path per line.
		"playwright.config.cjs": `const fs = require('fs');
const { defineConfig } = require('@playwright/test');
const esc = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
const list = fs.readFileSync(process.env.CORVINT_SETUP_LIST, 'utf8').split('\n').filter(Boolean);
module.exports = defineConfig({
  testDir: '.',
  workers: 1,
  reporter: 'line',
  projects: [
    { name: 'setup', testMatch: list.map((p) => new RegExp('(^|/)' + esc(p) + '$')) },
    { name: 'e2e', testMatch: /tests\/.*\.spec\.ts$/, dependencies: ['setup'] },
  ],
});
`,
	})
	if err = os.Symlink(modules, filepath.Join(root, "node_modules")); err != nil {
		t.Fatal(err)
	}
	run := func(name string, specs string, wantList, wantSeeds string) {
		t.Helper()
		code, out, errb := runCLI(t, "--root", root, "tests", "setup-select", "--specs", specs, "--format", "lines")
		if code != 0 || out != wantList {
			t.Fatalf("%s: setup-select lines: code=%d out=%q err=%q", name, code, out, errb)
		}
		listPath := filepath.Join(t.TempDir(), "setup-list")
		seedLog := filepath.Join(t.TempDir(), "seed-log")
		if err := os.WriteFile(listPath, []byte(out), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(seedLog, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(node, cli, "test", "--config", "playwright.config.cjs", "--project", "e2e")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "CORVINT_SETUP_LIST="+listPath, "CORVINT_SEED_LOG="+seedLog, "CI=1")
		combined, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: playwright run failed: %v\n%s", name, err, combined)
		}
		seeds, err := os.ReadFile(seedLog)
		if err != nil {
			t.Fatal(err)
		}
		if got := sortedLines(string(seeds)); got != wantSeeds {
			t.Fatalf("%s: executed setup seeds %q, want %q\n%s", name, got, wantSeeds, combined)
		}
	}
	// Selected: the spec reaches SHARED.scenario_a only, so only scenario_a's setup runs.
	run("selected", "tests/x.spec.ts", "setup/scenario_a.setup.ts\n", "alpha")
	// Control: an ALL selection (an untracked spec) hands both setup files to the run.
	run("all", "tests/absent.spec.ts", "setup/scenario_a.setup.ts\nsetup/scenario_b.setup.ts\n", "alpha,beta")
}

func sortedLines(text string) string {
	lines := strings.Fields(text)
	sort.Strings(lines)
	return strings.Join(lines, ",")
}
