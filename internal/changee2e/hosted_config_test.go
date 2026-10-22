package changee2e

import (
	"encoding/json"
	"strings"
	"testing"
)

const adopterConfig = `import { defineConfig, devices } from '@playwright/test';
// retries: 9 in a comment is not a setting
const authFile = 'playwright/.auth/user.json';
export default defineConfig({
  testDir: './e2e',
  retries: 2,
  workers: 4,
  globalSetup: require.resolve('./global-setup'),
  use: {
    baseURL: "http://localhost:3000",
    storageState: authFile,
    browserName: 'chromium',
  },
  projects: [{ name: 'setup', testMatch: /.*\.setup\.ts/ }, { name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
});
`

// CCE-V0-049: a hosted proof records the adopter config it found at head and every key whose
// effective value differs from the prove config; equal and default-equal keys are not listed.
func TestCCEV0049HostedRunnerConfigDeviation(t *testing.T) {
	root, base, _ := hostedRepo(t)
	head := commitFiles(t, root, map[string]*string{"e2e/playwright.config.ts": text(adopterConfig)}, "adopter config")
	rc := adopterRunnerConfig(bg, root, head, hostedPath)
	if rc.AdopterConfig != "e2e/playwright.config.ts" || !strings.HasPrefix(rc.AdopterConfigSHA256, "sha256:") || rc.Unread != "" || rc.Applied != proveConfigName {
		t.Fatalf("config %+v", rc)
	}
	got := map[string]string{}
	for _, d := range rc.Deviations {
		got[d.Key] = d.Adopter + " -> " + d.Prove
	}
	want := map[string]string{
		"testDir":          "'./e2e' -> '.'",
		"retries":          "2 -> 0",
		"workers":          "4 -> 1",
		"globalSetup":      "<set> -> <not applied>",
		"use.baseURL":      "'http://localhost:3000' -> <CORVINT_E2E_BASE_URL>",
		"use.storageState": "<set> -> <not applied>",
		"projects":         "<set> -> <set>",
	}
	if len(got) != len(want) {
		t.Fatalf("deviations %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("deviation %s = %q, want %q (all %v)", k, got[k], v, got)
		}
	}

	// No adopter config: the prove keys are compared against Playwright's defaults.
	none := adopterRunnerConfig(bg, root, base, hostedPath)
	if none.AdopterConfig != "" || none.Unread != "adopter-config-absent" {
		t.Fatalf("absent config %+v", none)
	}
	keys := []string{}
	for _, d := range none.Deviations {
		keys = append(keys, d.Key+"="+d.Adopter)
	}
	if strings.Join(keys, ",") != "projects=<unset>,use.baseURL=<unset>,workers=<unset>" {
		t.Fatalf("default deviations %v", keys)
	}

	// A config built from a spread is read partially and says so.
	spread := commitFiles(t, root, map[string]*string{"playwright.config.js": text("const base = require('./base');\nmodule.exports = { ...base, retries: 1 };\n")}, "spread config")
	if rc := adopterRunnerConfig(bg, root, spread, hostedPath); rc.AdopterConfig != "e2e/playwright.config.ts" {
		t.Fatalf("the nearest config to the test file wins: %+v", rc)
	}
	if rc := adopterRunnerConfig(bg, root, spread, "specs/other.spec.ts"); rc.AdopterConfig != "playwright.config.js" || rc.Unread != "adopter-config-spread" {
		t.Fatalf("spread config %+v", rc)
	}

	// The proof carries it for a hosted scenario only.
	sc, err := hostedScenario(bg, root, base, head, HostedSelection{File: hostedPath, Title: "shows the booked status"})
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeRunner{sides: map[string]fakeSide{head: {states: repeat("passed")}, base: {states: repeat("passed")}}, controls: "killed"}
	p := f.env(root).proveScenario(bg, base, head, sc, 3)
	b, _ := json.Marshal(p)
	if p.RunnerConfig == nil || !strings.Contains(string(b), `"runner_config":{"applied":"`+proveConfigName+`","adopter_config":"e2e/playwright.config.ts"`) {
		t.Fatalf("hosted proof without runner_config: %s", b)
	}
	sc.Hosted = false
	if p := f.env(root).proveScenario(bg, base, head, sc, 3); p.RunnerConfig != nil {
		t.Fatal("a compiled scenario runs only under the prove config and records no runner_config")
	}
}
