package typescript

import (
	json "encoding/json/v2"
	"path/filepath"
	"strings"
	"testing"
)

// gh1883Listing is the GitHub #790 ungrepped listing as Playwright reports it when run through
// the wrapper config at wrapper instead of the bound playwright.config.ts.
func gh1883Listing(t *testing.T, root, wrapper string) []byte {
	t.Helper()
	var report map[string]any
	if err := json.Unmarshal(grepPlaywrightListing(t, root, "e2e", gh1881Ungrepped("main", "alt-a", "alt-b")), &report); err != nil {
		t.Fatal(err)
	}
	report["config"].(map[string]any)["configFile"] = filepath.Join(root, filepath.FromSlash(wrapper))
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// TJAA-V0-124, TJAA-V0-125 (GitHub #1883, V1-2784): an ungrepped listing taken through the
// documented wrapper config verifies the grep omissions without rewriting the tracked config;
// any other wrapper, or a listing that names neither, is refused.
func TestPlaywrightUngreppedWrapperConfig_GH1883(t *testing.T) {
	canonical := strings.ReplaceAll(PlaywrightUngreppedWrapperTemplate, "CONFIG", "playwright.config.ts")
	produce := func(t *testing.T, wrapperBody, wrapper string, listedConfig string) (PlaywrightDiscovery, error) {
		t.Helper()
		root := gh1881HiddenTagFixture(t, grepRepro)
		if wrapperBody != "" {
			write(t, root, filepath.FromSlash(wrapper), wrapperBody)
		}
		raw, err := PlaywrightDiscoveryFromWrappedLists(root, "playwright.config.ts", discoveryFixtureRevision,
			grepPlaywrightListing(t, root, "e2e", grepReproCases()), gh1883Listing(t, root, listedConfig), wrapper)
		if err != nil {
			return PlaywrightDiscovery{}, err
		}
		return decodeGrepReceipt(t, raw), nil
	}
	accepted := map[string]string{
		"canonical": canonical,
		"double quotes, no semicolons, no trailing commas, extensionless import, comments": "// produced for corvint\nimport config from \"./playwright.config\"\nexport default {\n  ...config,\n  grep: undefined,\n  grepInvert: undefined,\n  /* strip per project */ projects: (config.projects ?? []).map(({ grep, grepInvert, ...project }) => project)\n}\n",
	}
	for name, body := range accepted {
		t.Run(name, func(t *testing.T) {
			receipt, err := produce(t, body, "playwright.ungrepped.config.ts", "playwright.ungrepped.config.ts")
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			if len(receipt.UnverifiedUnits) != 0 || len(receipt.Units) != 6 {
				t.Fatalf("receipt: %+v", receipt)
			}
		})
	}
	refused := []struct{ name, body, wrapper, listed, want string }{
		{"also strips testIgnore", strings.Replace(canonical, "grepInvert, ...project", "grepInvert, testIgnore, ...project", 1), "playwright.ungrepped.config.ts", "playwright.ungrepped.config.ts", "documented wrapper"},
		{"overrides testDir", strings.Replace(canonical, "grep: undefined,", "grep: undefined, testDir: 'other',", 1), "playwright.ungrepped.config.ts", "playwright.ungrepped.config.ts", "documented wrapper"},
		{"imports another config", strings.Replace(canonical, "./playwright.config.ts", "./other.config.ts", 1), "playwright.ungrepped.config.ts", "playwright.ungrepped.config.ts", "documented wrapper"},
		{"another directory", strings.Replace(canonical, "./playwright.config.ts", "../playwright.config.ts", 1), "wrap/playwright.ungrepped.config.ts", "wrap/playwright.ungrepped.config.ts", "directory"},
		{"is the config itself", "", "playwright.config.ts", "playwright.config.ts", "directory"},
		{"absent", "", "playwright.ungrepped.config.ts", "playwright.ungrepped.config.ts", "unreadable"},
		{"listing names the config, not the wrapper", canonical, "playwright.ungrepped.config.ts", "playwright.config.ts", "is not playwright.ungrepped.config.ts"},
	}
	for _, row := range refused {
		t.Run(row.name, func(t *testing.T) {
			if _, err := produce(t, row.body, row.wrapper, row.listed); err == nil || !strings.Contains(err.Error(), row.want) {
				t.Fatalf("err = %v, want %q", err, row.want)
			}
		})
	}
	// Without the declared wrapper, a listing that names one is refused as before (TJAA-V0-118).
	root := gh1881HiddenTagFixture(t, grepRepro)
	write(t, root, "playwright.ungrepped.config.ts", canonical)
	if _, err := PlaywrightDiscoveryFromLists(root, "playwright.config.ts", discoveryFixtureRevision,
		grepPlaywrightListing(t, root, "e2e", grepReproCases()), gh1883Listing(t, root, "playwright.ungrepped.config.ts")); err == nil || !strings.Contains(err.Error(), "ungrepped listing") {
		t.Fatalf("undeclared wrapper: err = %v", err)
	}
	// A wrapper without an ungrepped listing is refused (TJAA-V0-125).
	if _, err := PlaywrightDiscoveryFromWrappedLists(root, "playwright.config.ts", discoveryFixtureRevision,
		grepPlaywrightListing(t, root, "e2e", grepReproCases()), nil, "playwright.ungrepped.config.ts"); err == nil {
		t.Fatal("wrapper without an ungrepped listing was accepted")
	}
}
