package main

import (
	"bytes"
	json "encoding/json/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
)

// e2eCorpusListing writes the `playwright test --list --reporter=json` report the e2e-safe corpus
// config would print for files (relative to its e2e/ rootDir) and returns its path.
func e2eCorpusListing(t *testing.T, root string, files []string) string {
	t.Helper()
	suites := []any{}
	for _, file := range files {
		suites = append(suites, map[string]any{"file": file, "specs": []any{map[string]any{"file": file, "tests": []any{map[string]any{"projectName": "chromium"}}}}, "suites": []any{}})
	}
	raw, err := json.Marshal(map[string]any{
		"config": map[string]any{
			"configFile": filepath.Join(root, "playwright.config.ts"), "rootDir": filepath.Join(root, "e2e"), "shard": nil,
			"argv": []any{"/usr/local/bin/node", root + "/node_modules/.bin/playwright", "test", "--list", "--reporter=json"},
		},
		"suites": suites, "errors": []any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	listing := filepath.Join(t.TempDir(), "list.json")
	if err := os.WriteFile(listing, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return listing
}

func runAffectedDiscoveryProducer(t *testing.T, root, listing string) (int, []byte, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runContext(t.Context(), []string{"--root", root, "affected", "discovery", "--playwright-config", "playwright.config.ts", "--playwright-list", listing}, strings.NewReader(""), &stdout, &stderr)
	return code, stdout.Bytes(), stderr.String()
}

// GitHub #709 review (TJAA-V0-019): a listing taken before a new spec was committed must not be
// stamped with the new HEAD and source digest, so e2e-safe selection cannot narrow from it.
func TestAffectedPlaywrightDiscoveryStaleListing_GH709(t *testing.T) {
	corpus := loadE2ECorpus(t)
	// .mts and .cts specs are not parsed by the static profile; membership finds them by path.
	for _, added := range []string{"e2e/refund.spec.ts", "e2e/refund.spec.mts", "e2e/refund.spec.cts"} {
		t.Run(added, func(t *testing.T) {
			root, base, _ := buildE2ECase(t, corpus, corpus.find(t, "search-source"))
			listing := e2eCorpusListing(t, root, []string{"cart.spec.ts", "checkout.spec.ts", "profile.spec.ts", "search.spec.ts"})
			code, produced, stderr := runAffectedDiscoveryProducer(t, root, listing)
			if code != 0 {
				t.Fatalf("fresh listing: exit=%d stderr=%s", code, stderr)
			}
			shopWrite(t, root, map[string]string{"discovery.json": string(produced)})
			if got := runE2ESelection(t, root, base); got.Discovery.State != "MATCHED" {
				t.Fatalf("control: produced receipt did not match: %+v", got.Discovery)
			}
			shopWrite(t, root, map[string]string{added: "import { test } from '@playwright/test';\ntest('refunds', async () => {});\n"})
			shopCommit(t, root, "add a spec after the listing")
			code, produced, stderr = runAffectedDiscoveryProducer(t, root, listing)
			if code != 2 || len(produced) != 0 || !strings.Contains(stderr, "unsupported-playwright-discovery") || !strings.Contains(stderr, added) {
				t.Fatalf("stale listing stamped: exit=%d stdout=%s stderr=%s", code, produced, stderr)
			}
			shopWrite(t, root, map[string]string{"discovery.json": string(produced)})
			got := runE2ESelection(t, root, base)
			if got.Discovery.State == "MATCHED" || got.State != "full-relevant-suite-required" || len(got.OmittedTests) != 0 || !slices.Contains(e2eCodes(got), appflows.CodeInventoryIncomplete) {
				t.Fatalf("e2e-safe narrowed without a current listing: %+v", got)
			}
		})
	}
}

// GitHub #709 review (TJAA-V0-019): HEAD is read again after the last source observation, so a
// commit that lands while sources are verified is refused as drift, even with an unchanged tree.
func TestAffectedPlaywrightDiscoveryHeadDriftAfterSources_GH709(t *testing.T) {
	corpus := loadE2ECorpus(t)
	root, _, _ := buildE2ECase(t, corpus, corpus.find(t, "search-source"))
	listing := e2eCorpusListing(t, root, []string{"cart.spec.ts", "checkout.spec.ts", "profile.spec.ts", "search.spec.ts"})
	t.Cleanup(func() { playwrightDiscoverySourcesVerified = func() {} })
	playwrightDiscoverySourcesVerified = func() { shopGit(t, root, "commit", "-q", "--allow-empty", "-m", "same tree, new HEAD") }
	code, produced, stderr := runAffectedDiscoveryProducer(t, root, listing)
	if code != 2 || len(produced) != 0 || !strings.Contains(stderr, "unsupported-affected-drift") {
		t.Fatalf("HEAD drift after the source check was accepted: exit=%d stdout=%s stderr=%s", code, produced, stderr)
	}
}
