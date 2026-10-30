package typescript

import (
	"slices"
	"strings"
	"testing"
)

// PWR-V0-177 (GitHub #1920): a use key that cannot change which files a test loads does not leave
// every test closure unknown.
func TestPlaywrightTestClosuresNonIdentityUse_PWRV0177(t *testing.T) {
	for name, use := range map[string]string{
		"computed launchOptions args": "use: { launchOptions: { args: [...extraArgs(), '--no-sandbox'] } }",
		"launchOptions call":          "use: { launchOptions: launchOptionsFor(process.env.CI) }",
		"computed reporter metadata":  "use: { trace: pickTrace(), launchOptions: { args: browserArgs } }",
	} {
		t.Run(name, func(t *testing.T) {
			root := closureFixture(t, "export default defineConfig({ testDir: 'e2e', "+use+", projects: [{ name: 'main' }] });\n")
			got := closuresByTest(t, root)
			cart := got["e2e/cart.spec.ts"]
			if len(got) != 2 || cart.Unknown != "" || !slices.Contains(cart.Files, "helpers/cart.ts") {
				t.Fatalf("closures = %+v", got)
			}
		})
	}
}

func TestPlaywrightTestClosuresIdentityCallLiterals_PWRV0177(t *testing.T) {
	config := "import { execFileSync } from 'node:child_process';\n" +
		"export default defineConfig({ testDir: 'e2e', metadata: { corvintRunIdentity: JSON.parse(execFileSync('corvint', ['runs', 'identity', '--config', __filename, '--project-test-dir', 'setup:.*\\\\.ts$']).toString()).corvintRunIdentity },\n" +
		"  use: { launchOptions: { args: computed() } }, projects: [{ name: 'main' }] });\n"
	got := closuresByTest(t, closureFixture(t, config))
	if cart := got["e2e/cart.spec.ts"]; len(got) != 2 || cart.Unknown != "" {
		t.Fatalf("closures = %+v", got)
	}
}

// issueConfig1920 is the reporter's config shape (GitHub #1920): the config's own `runs identity`
// call carries an escaped regexp string, the global `use` reads process.env, and a setup project
// with its own directory runs before chromium.
const issueConfig1920 = "import { defineConfig, devices } from '@playwright/test';\n" +
	"import { execFileSync } from 'node:child_process';\n" +
	"const bin = process.env.CORVINT_BIN ?? 'corvint';\n" +
	"const extraArgs = process.env.CI ? ['--no-sandbox'] : [];\n" +
	"export default defineConfig({\n  testDir: './e2e',\n" +
	"  metadata: { corvintRunIdentity: JSON.parse(execFileSync(bin, ['runs', 'identity', '--config', __filename, '--test-dir', 'e2e', '--project-test-dir', 'setup:.*\\\\.ts$', '--compute-closures'], { encoding: 'utf8' })).corvintRunIdentity },\n" +
	"  use: { baseURL: process.env.BASE_URL ?? 'http://localhost:3000', trace: 'on-first-retry'USE },\n" +
	"  projects: [\n    { name: 'setup', testDir: './setup', testMatch: /.*\\.ts$/ },\n" +
	"    { name: 'chromium', use: { ...devices['Desktop Chrome'] }, dependencies: ['setup'] },\n  ],\n});\n"

func issueFixture1920(t *testing.T, config string) string {
	t.Helper()
	root := closureFixture(t, config)
	write(t, root, "package.json", `{"name":"shop","devDependencies":{"@playwright/test":"1.63.0"}}`)
	write(t, root, "setup/auth.setup.ts", "import { auth } from '../helpers/auth';\nsetup('auth', async () => { auth(); });\n")
	write(t, root, "helpers/auth.ts", "export function auth() {}\n")
	return root
}

// PWR-V0-183, TJAA-V0-147 (GitHub #1920): a `\` escape in a string element of the config's own
// identity argv (an array literal opening `'runs', 'identity'`) is admitted, so the process.env reads
// elsewhere in the config stay pure and every test keeps a known closure.
func TestPlaywrightTestClosuresIdentityArgvEscape_PWRV0183(t *testing.T) {
	for name, use := range map[string]string{"static use": "", "dynamic launchOptions": ", launchOptions: { args: [...extraArgs, '--disable-gpu'] }"} {
		t.Run(name, func(t *testing.T) {
			got := closuresByTest(t, issueFixture1920(t, strings.Replace(issueConfig1920, "USE", use, 1)))
			cart, setup := got["e2e/cart.spec.ts"], got["setup/auth.setup.ts"]
			if len(got) != 3 || cart.Unknown != "" || cart.Widened != "" || setup.Unknown != "" || setup.Widened != "" {
				t.Fatalf("closures = %+v", got)
			}
			if !slices.Contains(cart.Files, "setup/auth.setup.ts") || !slices.Contains(cart.Excludable, "helpers/login.ts") || !slices.Contains(setup.Excludable, "helpers/cart.ts") {
				t.Fatalf("cart = %+v setup = %+v", cart, setup)
			}
		})
	}
	// The same escape outside an identity argv still refuses the process root; the unverified use
	// value then widens (PWR-V0-184) rather than hiding the whole suite.
	for name, config := range map[string]string{
		"other argv":   strings.Replace(strings.Replace(issueConfig1920, "'runs', 'identity'", "'runs', 'compare'", 1), "USE", "", 1),
		"computed key": strings.Replace(issueConfig1920, "USE", ", label: devices['Desk\\\\x74op Chrome']", 1),
		"plain const":  strings.Replace(issueConfig1920, "USE", ", label: 'a\\\\tb'", 1),
	} {
		t.Run(name, func(t *testing.T) {
			cart := closuresByTest(t, issueFixture1920(t, config))["e2e/cart.spec.ts"]
			if cart.Unknown != "" || !strings.Contains(cart.Widened, "use value can run code") {
				t.Fatalf("cart = %+v", cart)
			}
		})
	}
}

// PWR-V0-184 (GitHub #1920): a use value the static selection cannot verify widens the closures of
// the tests whose projects, or whose dependency and teardown projects, hold it; every other test
// keeps its excludable files.
func TestPlaywrightTestClosuresUnverifiedUseWidensProject_PWRV0184(t *testing.T) {
	config := "export default defineConfig({ testDir: 'e2e', projects: [\n" +
		"  { name: 'setup', testDir: 'setup', testMatch: /.*\\.ts$/, use: { trace: pickTrace() } },\n" +
		"  { name: 'chromium', dependencies: ['setup'] },\n" +
		"  { name: 'api', testDir: 'api' },\n] });\n"
	root := issueFixture1920(t, config)
	write(t, root, "api/health.spec.ts", "import { cart } from '../helpers/cart';\ntest('health', async () => { cart(); });\n")
	got := closuresByTest(t, root)
	if len(got) != 4 {
		t.Fatalf("closures = %+v", got)
	}
	for _, test := range []string{"setup/auth.setup.ts", "e2e/cart.spec.ts", "e2e/login.spec.ts"} {
		closure := got[test]
		if closure.Unknown != "" || closure.Excludable == nil || len(closure.Excludable) != 0 || !strings.Contains(closure.Widened, "setup use value can run code") || !strings.Contains(closure.Widened, "pickTrace") {
			t.Fatalf("%s = %+v", test, closure)
		}
	}
	if health := got["api/health.spec.ts"]; health.Unknown != "" || health.Widened != "" || !slices.Contains(health.Excludable, "helpers/login.ts") {
		t.Fatalf("api test = %+v", health)
	}
}

// PWR-V0-185 (GitHub #1920): a project whose metadata is dynamic widens that project's tests only.
func TestPlaywrightTestClosuresDynamicProjectMetadataWidens_PWRV0185(t *testing.T) {
	config := "export default defineConfig({ testDir: 'e2e', projects: [\n" +
		"  { name: 'setup', testDir: 'setup', testMatch: /.*\\.ts$/ },\n" +
		"  { name: 'chromium', metadata: { build: buildId() }, dependencies: ['setup'] },\n] });\n"
	got := closuresByTest(t, issueFixture1920(t, config))
	cart, setup := got["e2e/cart.spec.ts"], got["setup/auth.setup.ts"]
	if len(got) != 3 || cart.Unknown != "" || len(cart.Excludable) != 0 || cart.Widened != "chromium metadata is dynamic" {
		t.Fatalf("cart = %+v", cart)
	}
	if setup.Unknown != "" || setup.Widened != "" || !slices.Contains(setup.Excludable, "helpers/cart.ts") {
		t.Fatalf("setup = %+v", setup)
	}
}
