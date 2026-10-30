package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
)

func TestAffectedPlaywrightProfileEmitsProjectDistinctUnits(t *testing.T) {
	t.Run("AFP-V0-018 separate Playwright receipt remains deterministic and read-only", func(t *testing.T) {
		root := t.TempDir()
		writePlaywrightCLIFile(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
		writePlaywrightCLIFile(t, root, "playwright.config.ts", `
import { defineConfig } from "@playwright/test"
export default defineConfig({
  projects: [
    { name: "angular", grep: /@angular/, use: { browserName: "chromium" } },
    { name: "react", grep: /@react/, use: { browserName: "chromium" } },
  ],
})
`)
		writePlaywrightCLIFile(t, root, "src/page.ts", `export const page = "page"`)
		writePlaywrightCLIFile(t, root, "tests/page.spec.ts", `import { test } from "@playwright/test"; import { page } from "../src/page"; test("@angular @react page", () => page)`)
		affectedGit(t, root, "init", "-q")
		affectedGit(t, root, "config", "user.email", "corvint@example.test")
		affectedGit(t, root, "config", "user.name", "Corvint Test")
		affectedGit(t, root, "add", ".")
		affectedGit(t, root, "commit", "-qm", "fixture")
		writePlaywrightCLIFile(t, root, "src/page.ts", `export const page = "changed"`)

		first := runPlaywrightAffectedCLI(t, root)
		second := runPlaywrightAffectedCLI(t, root)
		if !bytes.Equal(first, second) {
			t.Fatalf("profile bytes changed across runs:\n%s\n%s", first, second)
		}
		var receipt map[string]any
		if err := json.Unmarshal(first, &receipt); err != nil {
			t.Fatal(err)
		}
		if receipt["profile"] != typescript.PlaywrightProfile || receipt["tool"] != "affected" || receipt["mutates"] != false {
			t.Fatalf("receipt identity=%v", receipt)
		}
		plan := receipt["plan"].(map[string]any)
		if plan["scope"] != "BOUNDED" || plan["fallback"] != typescript.PlaywrightFallbackNone {
			t.Fatalf("plan scope=%v fallback=%v unknown=%v", plan["scope"], plan["fallback"], plan["unknown"])
		}
		selected := plan["selected"].([]any)
		if len(selected) != 2 {
			t.Fatalf("selected=%v", selected)
		}
		if selected[0].(map[string]any)["project"] != "angular" || selected[1].(map[string]any)["project"] != "react" {
			t.Fatalf("project units=%v", selected)
		}
		if status := affectedGit(t, root, "status", "--porcelain"); strings.TrimSpace(status) != "M src/page.ts" {
			t.Fatalf("command mutated fixture: %q", status)
		}
	})
}

func TestAffectedPlaywrightArgumentsFailClosed(t *testing.T) {
	root := affectedFixtureRepository(t)
	for _, arguments := range [][]string{
		{"--root", root, "affected", "--playwright-config", "../playwright.config.ts"},
		{"--root", root, "affected", "--playwright-config", "playwright.config.ts", "--provider", "record.json"},
		{"--root", root, "affected", "--playwright-discovery", "discovery.json"},
		{"--root", root, "affected", "--playwright-config", "playwright.config.ts", "--playwright-discovery", "a", "--playwright-discovery", "b"},
	} {
		var stdout, stderr bytes.Buffer
		if code := runContext(t.Context(), arguments, strings.NewReader(""), &stdout, &stderr); code != 2 || stdout.Len() != 0 {
			t.Fatalf("args=%v code=%d stdout=%s stderr=%s", arguments, code, stdout.String(), stderr.String())
		}
	}
}

func TestAffectedPlaywrightDiscoveryFallback(t *testing.T) {
	t.Run("AFP-V0-018 missing discovery emits one complete config command", func(t *testing.T) {
		root := affectedFixtureRepository(t)
		writePlaywrightCLIFile(t, root, "playwright.config.ts", `export default {projects:[{name:"chromium"}]}`)
		var stdout, stderr bytes.Buffer
		code := runContext(t.Context(), []string{"--root", root, "affected", "--playwright-config", "playwright.config.ts"}, strings.NewReader(""), &stdout, &stderr)
		if code != 0 {
			t.Fatal(code, stderr.String())
		}
		var receipt playwrightAffectedReceipt
		if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil {
			t.Fatal(err)
		}
		if receipt.Plan.Discovery.State != "MISSING" || len(receipt.Plan.Selected) != 0 || len(receipt.Plan.Excluded) != 0 || len(receipt.Plan.FallbackArgv) != 4 {
			t.Fatal(receipt.Plan)
		}
	})
}

func runPlaywrightAffectedCLI(t *testing.T, root string) []byte {
	t.Helper()
	discovery := writePlaywrightDiscoveryFile(t, root)
	var stdout, stderr bytes.Buffer
	code := runContext(t.Context(), []string{"--root", root, "affected", "--playwright-config", "playwright.config.ts", "--playwright-discovery", discovery}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	return stdout.Bytes()
}

// writePlaywrightDiscoveryFile writes a discovery receipt for the HEAD of the
// two-project fixture and returns its path.
func writePlaywrightDiscoveryFile(t *testing.T, root string) string {
	t.Helper()
	config, err := os.ReadFile(filepath.Join(root, "playwright.config.ts"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(config)
	digest, err := typescript.ObservePlaywrightSources(root, "playwright.config.ts")
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(typescript.PlaywrightDiscovery{Profile: "playwright-discovery/0", Revision: strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD")), Config: typescript.PlaywrightConfigIdentity{Path: "playwright.config.ts", SHA256: hex.EncodeToString(sum[:])}, SourceDigest: digest, Units: []typescript.PlaywrightDiscoveryUnit{{Project: "angular", Test: "tests/page.spec.ts"}, {Project: "react", Test: "tests/page.spec.ts"}}}, json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	discovery := filepath.Join(t.TempDir(), "discovery.json")
	if err := os.WriteFile(discovery, body, 0600); err != nil {
		t.Fatal(err)
	}
	return discovery
}

func writePlaywrightCLIFile(t *testing.T, root, relative, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// GitHub #709 (V1-1066, V1-1067): `affected discovery` converts a real multi-project Playwright
// listing into the canonical receipt that `affected --playwright-discovery` matches, refuses a
// filtered listing, and a MALFORMED receipt reports why (TJAA-V0-019, TJAA-V0-020).
func TestAffectedPlaywrightDiscoveryProducer_GH709(t *testing.T) {
	root := t.TempDir()
	writePlaywrightCLIFile(t, root, "playwright.config.ts", `import { defineConfig, devices } from '@playwright/test';

const authFile = 'playwright/.auth/user.json';
const server = { baseURL: 'http://localhost:3000' };

export default defineConfig({
  testDir: './e2e',
  use: { baseURL: server.baseURL, trace: 'on-first-retry' },
  projects: [
    { name: 'setup', testMatch: /.*\.setup\.ts/ },
    { name: 'chromium', use: { ...devices['Desktop Chrome'], storageState: authFile }, dependencies: ['setup'] },
    { name: 'firefox', use: { ...devices['Desktop Firefox'], storageState: authFile }, dependencies: ['setup'] },
    { name: 'admin', testDir: './e2e/admin', use: { ...devices['Desktop Chrome'] } },
  ],
});
`)
	writePlaywrightCLIFile(t, root, "e2e/auth.setup.ts", "import { test as setup } from '@playwright/test';\nsetup('authenticate', async () => {});\n")
	writePlaywrightCLIFile(t, root, "e2e/home.spec.ts", "import { test, expect } from '@playwright/test';\nimport { title } from './shared/page';\ntest.describe('home', () => {\n  test('has title', async () => { expect(title).toBe('home'); });\n  test('second', async () => {});\n});\n")
	writePlaywrightCLIFile(t, root, "e2e/search.spec.ts", "import { test } from '@playwright/test';\ntest('search', async () => {});\n")
	writePlaywrightCLIFile(t, root, "e2e/admin/users.spec.ts", "import { test } from '@playwright/test';\ntest('users', async () => {});\n")
	writePlaywrightCLIFile(t, root, "e2e/shared/page.ts", "export const title = 'home';\n")
	affectedGit(t, root, "init", "-q")
	affectedGit(t, root, "config", "user.email", "corvint@example.test")
	affectedGit(t, root, "config", "user.name", "Corvint Test")
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "fixture")
	template, err := os.ReadFile(filepath.Join("..", "..", "internal", "liveverify", "affected", "typescript", "testdata", "playwright-list", "multi-project.json"))
	if err != nil {
		t.Fatal(err)
	}
	scratch := t.TempDir()
	listing := filepath.Join(scratch, "list.json")
	if err := os.WriteFile(listing, bytes.ReplaceAll(template, []byte("@ROOT@"), []byte(root)), 0o600); err != nil {
		t.Fatal(err)
	}
	run := func(arguments ...string) (int, []byte, string) {
		var stdout, stderr bytes.Buffer
		code := runContext(t.Context(), append([]string{"--root", root, "affected"}, arguments...), strings.NewReader(""), &stdout, &stderr)
		return code, stdout.Bytes(), stderr.String()
	}
	code, produced, stderr := run("discovery", "--playwright-config", "playwright.config.ts", "--playwright-list", listing)
	if code != 0 || !bytes.HasSuffix(produced, []byte("}\n")) || bytes.Count(produced, []byte("\n")) != 1 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, produced, stderr)
	}
	var receipt typescript.PlaywrightDiscovery
	if err := json.Unmarshal(produced, &receipt); err != nil {
		t.Fatal(err)
	}
	if len(receipt.Units) != 8 || receipt.Units[0] != (typescript.PlaywrightDiscoveryUnit{Project: "admin", Test: "e2e/admin/users.spec.ts"}) || receipt.Revision != strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD")) {
		t.Fatalf("receipt=%+v", receipt)
	}
	// TJAA-V0-118 (GitHub #1881): with no project grep the ungrepped listing equals the listing and
	// the receipt is unchanged.
	if code, again, stderr := run("discovery", "--playwright-config", "playwright.config.ts", "--playwright-list", listing, "--playwright-ungrepped-list", listing); code != 0 || !bytes.Equal(again, produced) {
		t.Fatalf("ungrepped listing: exit=%d stdout=%s stderr=%s", code, again, stderr)
	}
	// TJAA-V0-124, TJAA-V0-125 (GitHub #1883): an ungrepped listing taken through the documented
	// untracked wrapper config is accepted only with --playwright-ungrepped-config; the wrapper is
	// held out of the source digest, so the receipt still matches after it is deleted.
	wrapperListing := filepath.Join(scratch, "wrapper-list.json")
	if err := os.WriteFile(wrapperListing, bytes.ReplaceAll(bytes.ReplaceAll(template, []byte("@ROOT@/playwright.config.ts"), []byte("@ROOT@/playwright.ungrepped.config.ts")), []byte("@ROOT@"), []byte(root)), 0o600); err != nil {
		t.Fatal(err)
	}
	wrapperBody := strings.ReplaceAll(typescript.PlaywrightUngreppedWrapperTemplate, "CONFIG", "playwright.config")
	writePlaywrightCLIFile(t, root, "playwright.ungrepped.config.ts", wrapperBody)
	if code, _, stderr := run("discovery", "--playwright-config", "playwright.config.ts", "--playwright-list", listing, "--playwright-ungrepped-list", wrapperListing); code != 2 || !strings.Contains(stderr, "unsupported-playwright-discovery") {
		t.Fatalf("undeclared wrapper: exit=%d stderr=%s", code, stderr)
	}
	if code, again, stderr := run("discovery", "--playwright-config", "playwright.config.ts", "--playwright-list", listing, "--playwright-ungrepped-list", wrapperListing, "--playwright-ungrepped-config", "playwright.ungrepped.config.ts"); code != 0 || !bytes.Equal(again, produced) {
		t.Fatalf("wrapper listing: exit=%d stdout=%s stderr=%s", code, again, stderr)
	}
	writePlaywrightCLIFile(t, root, "playwright.ungrepped.config.ts", strings.Replace(wrapperBody, "grep: undefined,", "grep: undefined, testDir: './e2e/admin',", 1))
	if code, _, stderr := run("discovery", "--playwright-config", "playwright.config.ts", "--playwright-list", listing, "--playwright-ungrepped-list", wrapperListing, "--playwright-ungrepped-config", "playwright.ungrepped.config.ts"); code != 2 || !strings.Contains(stderr, "documented wrapper") {
		t.Fatalf("altered wrapper: exit=%d stderr=%s", code, stderr)
	}
	for _, arguments := range [][]string{
		{"discovery", "--playwright-config", "playwright.config.ts", "--playwright-list", listing, "--playwright-ungrepped-config", "playwright.ungrepped.config.ts"},
		{"discovery", "--playwright-config", "playwright.config.ts", "--playwright-list", listing, "--playwright-ungrepped-list", wrapperListing, "--playwright-ungrepped-config", "../playwright.ungrepped.config.ts"},
		{"discovery", "--playwright-config", "playwright.config.ts", "--playwright-list", listing, "--playwright-ungrepped-list", wrapperListing, "--playwright-ungrepped-config", "playwright.ungrepped.config.ts", "--playwright-ungrepped-config", "playwright.ungrepped.config.ts"},
	} {
		if code, stdout, stderr := run(arguments...); code != 2 || len(stdout) != 0 || !strings.Contains(stderr, "--playwright-ungrepped-config") {
			t.Fatalf("args=%v exit=%d stdout=%s stderr=%s", arguments, code, stdout, stderr)
		}
	}
	if err := os.Remove(filepath.Join(root, "playwright.ungrepped.config.ts")); err != nil {
		t.Fatal(err)
	}
	discovery := filepath.Join(scratch, "discovery.json")
	if err := os.WriteFile(discovery, produced, 0o600); err != nil {
		t.Fatal(err)
	}
	state := func(path string) typescript.PlaywrightDiscoverySummary {
		t.Helper()
		code, stdout, stderr := run("--playwright-config", "playwright.config.ts", "--playwright-discovery", path)
		if code != 0 {
			t.Fatalf("exit=%d stderr=%s", code, stderr)
		}
		var plan playwrightAffectedReceipt
		if err := json.Unmarshal(stdout, &plan); err != nil {
			t.Fatal(err)
		}
		return plan.Plan.Discovery
	}
	if got := state(discovery); got.State != "MATCHED" || got.Reason != "" {
		t.Fatalf("produced receipt: %+v", got)
	}
	if err := os.WriteFile(discovery, bytes.Replace(produced, []byte(`{"config"`), []byte("{\n  \"config\""), 1), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := state(discovery); got.State != "MALFORMED" || got.Reason != "NON_CANONICAL_BYTES" || !strings.Contains(got.Detail, "byte 1") {
		t.Fatalf("indented receipt: %+v", got)
	}
	if err := os.WriteFile(discovery, bytes.Repeat([]byte(" "), typescript.PlaywrightDiscoveryMaxBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := state(discovery); got.State != "MALFORMED" || got.Reason != "DECODE_FAILED" || !strings.Contains(got.Detail, "bound") {
		t.Fatalf("oversize receipt: %+v", got)
	}
	if got := state(listing); got.State != "MALFORMED" || got.Reason != "DECODE_FAILED" || !strings.Contains(got.Detail, "corvint affected discovery") {
		t.Fatalf("raw listing as receipt: %+v", got)
	}
	filtered := filepath.Join(scratch, "filtered.json")
	if err := os.WriteFile(filtered, bytes.Replace(bytes.ReplaceAll(template, []byte("@ROOT@"), []byte(root)), []byte(`"--list",`), []byte(`"--list", "--project=chromium",`), 1), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{
		{"discovery", "--playwright-config", "playwright.config.ts", "--playwright-list", filtered},
		{"discovery", "--playwright-config", "playwright.config.ts", "--playwright-list", filepath.Join(scratch, "absent.json")},
		{"discovery", "--playwright-config", "playwright.config.ts"},
		{"discovery", "--playwright-list", listing},
		{"discovery", "--playwright-config", "playwright.config.ts", "--playwright-list", listing, "--full"},
		{"discovery", "--playwright-config", "../playwright.config.ts", "--playwright-list", listing},
		{"discovery", "--playwright-config", "playwright.config.ts", "--playwright-list", listing, "--playwright-list", listing},
		{"discovery", "--playwright-config", "playwright.config.ts", "--playwright-list", listing, "--playwright-ungrepped-list", listing, "--playwright-ungrepped-list", listing},
		{"discovery", "--playwright-config", "playwright.config.ts", "--playwright-ungrepped-list", listing},
		{"discovery", "--playwright-config", "playwright.config.ts", "--playwright-list", listing, "--playwright-ungrepped-list", filepath.Join(scratch, "absent.json")},
		{"discovery", "--playwright-config", "playwright.config.ts", "--playwright-list", listing, "--playwright-ungrepped-list", filtered},
	} {
		code, stdout, stderr := run(arguments...)
		if code != 2 || len(stdout) != 0 || stderr == "" {
			t.Fatalf("args=%v exit=%d stdout=%s stderr=%s", arguments, code, stdout, stderr)
		}
		if arguments[len(arguments)-1] == filtered && !strings.Contains(stderr, "unsupported-playwright-discovery") {
			t.Fatalf("filtered listing stderr=%s", stderr)
		}
	}
	if status := affectedGit(t, root, "status", "--porcelain"); strings.TrimSpace(status) != "" {
		t.Fatalf("command mutated fixture: %q", status)
	}
	// A wrapper HEAD tracks is repository source the hold would hide, so it is refused (TJAA-V0-125).
	writePlaywrightCLIFile(t, root, "playwright.ungrepped.config.ts", wrapperBody)
	affectedGit(t, root, "add", "playwright.ungrepped.config.ts")
	affectedGit(t, root, "commit", "-qm", "track wrapper")
	if code, _, stderr := run("discovery", "--playwright-config", "playwright.config.ts", "--playwright-list", listing, "--playwright-ungrepped-list", wrapperListing, "--playwright-ungrepped-config", "playwright.ungrepped.config.ts"); code != 2 || !strings.Contains(stderr, "tracked by HEAD") {
		t.Fatalf("tracked wrapper: exit=%d stderr=%s", code, stderr)
	}
}

// GitHub #1294 (V1-2042): a shared test-support module change under per-project config unknowns
// keeps FULL_RELEVANT_SUITE and reports the importing tests and the changed symbols against HEAD,
// or against the --base commit when one is given (TJAA-V0-030, TJAA-V0-033).
func TestAffectedPlaywrightNarrowedSymbols_TJAA_V0_033(t *testing.T) {
	root := t.TempDir()
	writePlaywrightCLIFile(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
	writePlaywrightCLIFile(t, root, "playwright.config.ts", `
import { defineConfig } from "@playwright/test"
export default defineConfig({
  testDir: "./tests",
  projects: [
    { name: "chromium" },
    { name: "mobile", testDir: process.env.MOBILE_DIR },
  ],
})
`)
	writePlaywrightCLIFile(t, root, "tests/support/fixture.ts", "export const user = \"u\"\n")
	writePlaywrightCLIFile(t, root, "tests/login.spec.ts", `import { test } from "@playwright/test"; import { user } from "./support/fixture"; test("login", () => user)`)
	writePlaywrightCLIFile(t, root, "tests/cart.spec.ts", `import { test } from "@playwright/test"; test("cart", () => {})`)
	affectedGit(t, root, "init", "-q")
	affectedGit(t, root, "config", "user.email", "corvint@example.test")
	affectedGit(t, root, "config", "user.name", "Corvint Test")
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "fixture")
	first := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	writePlaywrightCLIFile(t, root, "tests/support/fixture.ts", "export const user = \"u\"\nexport const admin = \"a\"\n")
	affectedGit(t, root, "commit", "-qam", "admin")
	writePlaywrightCLIFile(t, root, "tests/support/fixture.ts", "export const user = \"u\"\nexport const admin = \"a\"\nexport const role = \"r\"\n")

	run := func(extra ...string) typescript.PlaywrightPlan {
		t.Helper()
		var stdout, stderr bytes.Buffer
		arguments := append([]string{"--root", root, "affected", "--playwright-config", "playwright.config.ts"}, extra...)
		if code := runContext(t.Context(), arguments, strings.NewReader(""), &stdout, &stderr); code != 0 {
			t.Fatalf("exit=%d stderr=%s", code, stderr.String())
		}
		var receipt playwrightAffectedReceipt
		if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil {
			t.Fatal(err)
		}
		return receipt.Plan
	}
	for _, testCase := range []struct {
		name  string
		extra []string
		added []string
	}{
		{name: "HEAD", added: []string{"role"}},
		{name: "--base", extra: []string{"--base", first}, added: []string{"admin", "role"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			plan := run(testCase.extra...)
			if plan.Scope != "UNKNOWN" || plan.Fallback != typescript.PlaywrightFallbackFullSuite || len(plan.Selected) != 0 {
				t.Fatalf("fallback changed: %+v", plan)
			}
			narrowing := plan.Narrowed
			if narrowing == nil || narrowing.State != typescript.PlaywrightNarrowed || len(narrowing.Candidates) != 1 || narrowing.Candidates[0].Test != "tests/login.spec.ts" || narrowing.Candidates[0].Project != "chromium" {
				t.Fatalf("narrowing: %+v", narrowing)
			}
			if len(narrowing.Symbols) != 1 || narrowing.Symbols[0].State != typescript.PlaywrightSymbolsResolved || !narrowing.Symbols[0].AdditiveOnly || strings.Join(narrowing.Symbols[0].Added, ",") != strings.Join(testCase.added, ",") {
				t.Fatalf("symbols: %+v", narrowing.Symbols)
			}
		})
	}
	if status := affectedGit(t, root, "status", "--porcelain"); strings.TrimSpace(status) != "M tests/support/fixture.ts" {
		t.Fatalf("command mutated fixture: %q", status)
	}
}
