package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
)

// GitHub #1750 (V1-2645, TJAA-V0-100): a gitignored, untracked Playwright report whose bundles
// hold a dynamic import is no input of the shared TypeScript graph, so a one-spec change narrows.
// A tracked but ignored file, and an untracked file Git does not ignore, still raise the frontier;
// the untracked one is dirty and widens, the unchanged tracked one is informational (GitHub #1878).
func TestAffectedPlaywrightIgnoredReportNarrows_GH1750(t *testing.T) {
	const report = "playwright-report/trace/assets/defaultSettingsView.js"
	fixture := func(t *testing.T, gitignore string) string {
		t.Helper()
		root := t.TempDir()
		writePlaywrightCLIFile(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
		writePlaywrightCLIFile(t, root, "playwright.config.ts", `
import { defineConfig } from "@playwright/test"
export default defineConfig({
  projects: [
    { name: "chromium", use: { browserName: "chromium" } },
  ],
})
`)
		writePlaywrightCLIFile(t, root, "tests/page.spec.ts", `import { test } from "@playwright/test"; test("page", () => {})`)
		writePlaywrightCLIFile(t, root, "tests/other.spec.ts", `import { test } from "@playwright/test"; test("other", () => {})`)
		if gitignore != "" {
			writePlaywrightCLIFile(t, root, ".gitignore", gitignore)
		}
		affectedGit(t, root, "init", "-q")
		affectedGit(t, root, "config", "user.email", "corvint@example.test")
		affectedGit(t, root, "config", "user.name", "Corvint Test")
		affectedGit(t, root, "add", ".")
		affectedGit(t, root, "commit", "-qm", "fixture")
		writePlaywrightCLIFile(t, root, report, "export async function load(name) { return import(name) }\n")
		writePlaywrightCLIFile(t, root, "playwright-report/index.html", "<html></html>\n")
		return root
	}
	plan := func(t *testing.T, root string) (typescript.PlaywrightPlan, []byte) {
		t.Helper()
		writePlaywrightCLIFile(t, root, "tests/page.spec.ts", `import { test } from "@playwright/test"; test("page changed", () => {})`)
		discovery := writeIgnoredPlaywrightDiscovery(t, root)
		var stdout, stderr bytes.Buffer
		code := runContext(t.Context(), []string{"--root", root, "affected", "--playwright-config", "playwright.config.ts", "--playwright-discovery", discovery}, strings.NewReader(""), &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit=%d stderr=%s", code, stderr.String())
		}
		var receipt playwrightAffectedReceipt
		if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil {
			t.Fatal(err)
		}
		if receipt.Plan.Discovery.State != "MATCHED" {
			t.Fatalf("discovery=%+v", receipt.Plan.Discovery)
		}
		return receipt.Plan, stdout.Bytes()
	}

	t.Run("ignored and untracked report is no graph input", func(t *testing.T) {
		root := fixture(t, "/playwright-report/\n")
		got, raw := plan(t, root)
		if got.Scope != affected.ScopeBounded || got.Fallback != typescript.PlaywrightFallbackNone {
			t.Fatalf("scope=%s fallback=%s unknown=%+v", got.Scope, got.Fallback, got.Unknown)
		}
		if len(got.Selected) != 1 || got.Selected[0].Test != "tests/page.spec.ts" || len(got.Excluded) != 1 || got.Excluded[0].Test != "tests/other.spec.ts" {
			t.Fatalf("selected=%+v excluded=%+v", got.Selected, got.Excluded)
		}
		if bytes.Contains(raw, []byte("playwright-report")) {
			t.Fatalf("receipt names the ignored report: %s", raw)
		}
		if status := affectedGit(t, root, "status", "--porcelain"); strings.TrimSpace(status) != "M tests/page.spec.ts" {
			t.Fatalf("command mutated fixture: %q", status)
		}
	})
	t.Run("untracked report Git does not ignore stays a frontier", func(t *testing.T) {
		got, _ := plan(t, fixture(t, ""))
		assertReportFrontier(t, got, report)
	})
	// A tracked but ignored file is still a graph input, so it still raises the frontier; with no
	// site dirty, in the Playwright closure or reached by a dirty path it is informational and the
	// one-spec change narrows (TJAA-V0-108, TJAA-V0-109; GitHub #1878).
	t.Run("tracked but ignored file stays a frontier", func(t *testing.T) {
		root := fixture(t, "/playwright-report/\n")
		affectedGit(t, root, "add", "-f", report)
		affectedGit(t, root, "commit", "-qm", "track one report file")
		got, _ := plan(t, root)
		if got.Scope != affected.ScopeBounded || len(got.Selected) != 1 || got.Selected[0].Test != "tests/page.spec.ts" {
			t.Fatalf("scope=%s selected=%+v unknown=%+v", got.Scope, got.Selected, got.Unknown)
		}
		for _, entry := range got.Informational {
			if entry.Axis == typescript.PlaywrightAxisInformational && entry.Reason == typescript.FrontierDynamicImport && strings.Contains(entry.Detail, report) {
				return
			}
		}
		t.Fatalf("no informational frontier names %s: %+v", report, got.Informational)
	})
}

func assertReportFrontier(t *testing.T, got typescript.PlaywrightPlan, report string) {
	t.Helper()
	if got.Fallback != typescript.PlaywrightFallbackFullSuite || len(got.Unknown) == 0 {
		t.Fatalf("scope=%s fallback=%s unknown=%+v", got.Scope, got.Fallback, got.Unknown)
	}
	for _, unknown := range got.Unknown {
		if strings.Contains(unknown.Detail, report) {
			return
		}
	}
	t.Fatalf("no unknown names %s: %+v", report, got.Unknown)
}

// writeIgnoredPlaywrightDiscovery writes a discovery receipt for the one-project fixture whose
// source digest is observed under the same ignored-path exclusion the CLI holds.
func writeIgnoredPlaywrightDiscovery(t *testing.T, root string) string {
	t.Helper()
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	ignored, err := affected.IgnoredPaths(t.Context(), gitExecutable, root)
	if err != nil {
		t.Fatal(err)
	}
	release, err := affected.HoldIgnoredExclusion(root, ignored)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := typescript.ObservePlaywrightSources(root, "playwright.config.ts")
	release()
	if err != nil {
		t.Fatal(err)
	}
	config, err := os.ReadFile(filepath.Join(root, "playwright.config.ts"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(config)
	body, err := json.Marshal(typescript.PlaywrightDiscovery{Profile: "playwright-discovery/0", Revision: strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD")), Config: typescript.PlaywrightConfigIdentity{Path: "playwright.config.ts", SHA256: hex.EncodeToString(sum[:])}, SourceDigest: digest, Units: []typescript.PlaywrightDiscoveryUnit{{Project: "chromium", Test: "tests/other.spec.ts"}, {Project: "chromium", Test: "tests/page.spec.ts"}}}, json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	discovery := filepath.Join(t.TempDir(), "discovery.json")
	if err := os.WriteFile(discovery, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return discovery
}
