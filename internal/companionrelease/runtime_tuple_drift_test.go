package companionrelease

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
)

// The one declared Node/npm/Playwright tuple (V1-2690, PUB-V0-013,
// PUB-V0-016, PUB-V0-020). requiredNodeVersion and requiredNPMVersion carry
// its Node and npm half.
const (
	declaredPlaywrightVersion = "1.64.0"
	declaredHeadlessRevision  = "1248"
	declaredHeadlessBrowser   = "156.0.8078.4"
	declaredHeadlessSHA256    = "77d8278b1ac9d0687fad4e817ea83023ede09b1deef36e1864b49ba194ac9634"
)

// declaredPlaywrightPackages pins each locked Playwright package's npm
// integrity; its resolved URL follows from the package name and version.
var declaredPlaywrightPackages = map[string]string{
	"playwright":       "sha512-kVzTnYFYEyQ9RGzBjNCd2+yEqWmVbxl3AHPynv3qQ6JTjv4a+kwRcnqQwIpuP3NnrnIrE/Q6wWAIAvMfIE2/Yg==",
	"playwright-core":  "sha512-T9r+MZkTECl2+oUcZ26YgZzTrYidTsIhYYkJix+6iDOymOb1FB2RLLSOxRGnZGv5DStt+aduIVbt+E8WygXYpg==",
	"@playwright/test": "sha512-oTN9FNYuN6bqQMwiho6jJKLCG2/QOYCixGXuCjRmzOq23eODeukuY2XiAVoVpAECbuezMt1MHQjAiQxK8F+Y+g==",
}

type lockedPackage struct {
	Version, Resolved, Integrity  string
	Dependencies, DevDependencies map[string]string
}

func readRepoFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile("../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func lockedPackages(t *testing.T, path string) map[string]lockedPackage {
	t.Helper()
	var lock struct {
		Packages map[string]lockedPackage `json:"packages"`
	}
	if err := json.Unmarshal([]byte(readRepoFile(t, path)), &lock); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return lock.Packages
}

// checkLockedTuple asserts version, resolved URL, integrity and the
// declared-version dependency edges for each named Playwright package, plus
// the root manifest's direct pin.
func checkLockedTuple(t *testing.T, path, direct string, names ...string) {
	t.Helper()
	packages := lockedPackages(t, path)
	root := packages[""]
	if pin := root.Dependencies[direct] + root.DevDependencies[direct]; pin != declaredPlaywrightVersion {
		t.Fatalf("%s root %s = %q", path, direct, pin)
	}
	for _, name := range names {
		entry := packages["node_modules/"+name]
		base := name[strings.LastIndex(name, "/")+1:]
		resolved := "https://registry.npmjs.org/" + name + "/-/" + base + "-" + declaredPlaywrightVersion + ".tgz"
		if entry.Version != declaredPlaywrightVersion || entry.Resolved != resolved || entry.Integrity != declaredPlaywrightPackages[name] {
			t.Fatalf("%s %s drifted: %+v", path, name, entry)
		}
		for dep, pin := range entry.Dependencies {
			if _, pinned := declaredPlaywrightPackages[dep]; pinned && pin != declaredPlaywrightVersion {
				t.Fatalf("%s %s -> %s = %q", path, name, dep, pin)
			}
		}
	}
}

// V1-2690: every pinned copy of the runtime tuple names the declared tuple,
// and the shared Playwright table admits it for the bundled external profile.
func TestV12690DeclaredRuntimeTupleDrift(t *testing.T) {
	if m := jstestprovider.AdmittedRuntimeTupleMismatch(jstestprovider.ExternalProfile, declaredPlaywrightVersion, requiredNodeVersion, jstestprovider.BrowserSourceBundled); m != nil {
		t.Fatalf("declared tuple is not an admitted row: %+v", m)
	}
	checkLockedTuple(t, "tools/web-flows/package-lock.json", "playwright", "playwright", "playwright-core")
	checkLockedTuple(t, "conformance/interactive-alpha/fixture/package-lock.json", "@playwright/test", "@playwright/test", "playwright", "playwright-core")
	contains := map[string][]string{
		"tools/web-flows/package.json":                       {`"playwright": "` + declaredPlaywrightVersion + `"`},
		"conformance/interactive-alpha/fixture/package.json": {`"@playwright/test": "` + declaredPlaywrightVersion + `"`},
		"tools/web-flows/navigation.mjs":                     {`e.playwrightVersion!=='` + declaredPlaywrightVersion + `'`},
		"tools/web-flows/observe.mjs":                        {`e.playwrightVersion!=='` + declaredPlaywrightVersion + `'`},
		"conformance/interactive-alpha/core-process.mjs": {
			`version === '` + declaredPlaywrightVersion + `', 'unsupported core Playwright version'`,
			`entry.revision === '` + declaredHeadlessRevision + `' && entry.browserVersion === '` + declaredHeadlessBrowser + `'`,
		},
		"extensions/vscode/test/installed/run.mjs": {
			`node: "` + requiredNodeVersion + `", npm: "` + requiredNPMVersion + `"`,
			`playwright: "` + declaredPlaywrightVersion + `"`,
			`"chromium_headless_shell-` + declaredHeadlessRevision + `"`,
			`"Google Chrome for Testing ` + declaredHeadlessBrowser + `"`,
			`expectedBrowserSHA256 = "` + declaredHeadlessSHA256 + `"`,
		},
		"conformance/interactive-alpha/core-provider-proof.mjs": {`'--runner-version', e2e ? '` + declaredPlaywrightVersion + `' : '5.0.3'`},
		"extensions/vscode/test/installed/index.cjs":            {`"--runner-version", "` + declaredPlaywrightVersion + `"`},
		"docs/specs/public-release-v0.md": {
			"verify Node " + strings.TrimPrefix(requiredNodeVersion, "v") + ", npm " + requiredNPMVersion + ",",
			"Node " + requiredNodeVersion + ", npm " + requiredNPMVersion + ",\nVitest 5.0.3 and Playwright " + declaredPlaywrightVersion,
			"`chromium_headless_shell-" + declaredHeadlessRevision + "`",
		},
	}
	for path, wants := range contains {
		text := readRepoFile(t, path)
		for _, want := range wants {
			if !strings.Contains(text, want) {
				t.Fatalf("%s does not pin %q", path, want)
			}
		}
	}
}
