package runidentity

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// closureRepo is a committed Playwright suite whose two specs load disjoint helpers; src/price.ts
// is shared with application code outside the suite.
func closureRepo(t *testing.T, extra map[string]string) (string, Options) {
	t.Helper()
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	git(t, root, "init", "-q", "-b", "main")
	files := map[string]string{
		"playwright.config.ts": "export default defineConfig({ testDir: 'e2e', projects: [{ name: 'main' }] });\n",
		"e2e/cart.spec.ts":     "import { cart } from '../helpers/cart';\ntest('cart', async () => { cart(); });\n",
		"e2e/login.spec.ts":    "import { login } from '../helpers/login';\ntest('login', async () => { login(); });\n",
		"helpers/cart.ts":      "export function cart() {}\n",
		"helpers/login.ts":     "import { price } from '../src/price';\nexport function login() { price(); }\n",
		"src/price.ts":         "export function price() {}\n",
		"src/app.ts":           "import { price } from './price';\nexport const app = price;\n",
		"README.md":            "suite\n",
	}
	for name, body := range extra {
		files[name] = body
	}
	write(t, root, files)
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "init")
	return root, Options{Config: filepath.Join(root, "playwright.config.ts"), TestDir: filepath.Join(root, "e2e"), ComputeClosures: true}
}

func commitEdit(t *testing.T, root, name, body string) {
	t.Helper()
	write(t, root, map[string]string{name: body})
	git(t, root, "commit", "-q", "-am", "edit "+name)
}

// PWR-V0-072: --compute-closures digests each test's closure under rule corvint-test-closure/0; an
// edit to a file only another test loads leaves the digest unchanged, an edit to a file the test
// loads, a shared file or a non-TypeScript file changes it, and a modified file is never left out.
func TestPWRV0072_ComputedClosureDigests(t *testing.T) {
	root, opts := closureRepo(t, nil)
	before := produce(t, opts)
	d := before.CorvintRunIdentity
	if d.TestClosureRule != ClosureRuleComputed || len(d.TestClosureDigests) != 2 || before.Counts.ClosureDigests != 2 ||
		before.Closure == nil || before.Closure.Source != ClosureSourceComputed || before.Closure.Unknown != nil || before.Closure.Widened != "" {
		t.Fatalf("computed closure = %+v closure %+v", d, before.Closure)
	}
	plain := opts
	plain.ComputeClosures = false
	if base := produce(t, plain); base.CorvintRunIdentity.ConfigDigest != d.ConfigDigest || base.Closure != nil || base.CorvintRunIdentity.TestClosureRule != "" {
		t.Fatal("computing closures changed the inputs-mode declaration")
	}
	cart, login := d.TestClosureDigests["cart.spec.ts"], d.TestClosureDigests["login.spec.ts"]
	if cart == "" || login == "" || cart == login || !strings.HasPrefix(cart, "sha256:") {
		t.Fatalf("digests = %v", d.TestClosureDigests)
	}

	// Uncommitted, the login helper stays in the cart digest; committed, it is left out.
	write(t, root, map[string]string{"helpers/login.ts": "import { price } from '../src/price';\nexport function login() { price(); price(); }\n"})
	if dirty := produce(t, opts).CorvintRunIdentity.TestClosureDigests; dirty["cart.spec.ts"] == cart || dirty["login.spec.ts"] == login {
		t.Fatalf("a modified excludable file was left out: %v", dirty)
	}
	git(t, root, "commit", "-q", "-am", "login helper")
	after := produce(t, opts).CorvintRunIdentity
	if after.TestClosureDigests["cart.spec.ts"] != cart || after.TestClosureDigests["login.spec.ts"] == login || after.ConfigDigest == d.ConfigDigest {
		t.Fatalf("committed login helper edit: before %v after %v", d.TestClosureDigests, after.TestClosureDigests)
	}
	for _, edit := range []string{"helpers/cart.ts", "src/price.ts", "README.md"} {
		prior := produce(t, opts).CorvintRunIdentity.TestClosureDigests["cart.spec.ts"]
		commitEdit(t, root, edit, "// edited\nexport function cart() {}\nexport function price() {}\n")
		if produce(t, opts).CorvintRunIdentity.TestClosureDigests["cart.spec.ts"] == prior {
			t.Fatalf("an edit to %s left the cart closure digest unchanged", edit)
		}
	}

	// A declared closure wins.
	declared := opts
	declared.ClosureDigests = map[string]string{"cart.spec.ts": closureA}
	won := produce(t, declared)
	if won.CorvintRunIdentity.TestClosureRule != ClosureRuleDeclared || won.Closure == nil || won.Closure.Source != ClosureSourceDeclared ||
		len(won.CorvintRunIdentity.TestClosureDigests) != 1 {
		t.Fatalf("declared beside computed = %+v closure %+v", won.CorvintRunIdentity, won.Closure)
	}
}

// PWR-V0-073: a test whose closure cannot be computed gets no digest and a stated reason; a
// frontier outside the suite leaves no file out of any digest.
func TestPWRV0073_ComputedClosureFailsClosed(t *testing.T) {
	t.Run("dynamic import", func(t *testing.T) {
		_, opts := closureRepo(t, map[string]string{"e2e/cart.spec.ts": "const name = process.env.CART;\ntest('cart', async () => { await import(name); });\n"})
		doc := produce(t, opts)
		if _, has := doc.CorvintRunIdentity.TestClosureDigests["cart.spec.ts"]; has || doc.Closure.Unknown["cart.spec.ts"] == "" ||
			doc.CorvintRunIdentity.TestClosureDigests["login.spec.ts"] == "" || !slices.Contains(doc.Limits, LimitClosureUnknown) {
			t.Fatalf("dynamic import: %+v closure %+v limits %v", doc.CorvintRunIdentity.TestClosureDigests, doc.Closure, doc.Limits)
		}
	})
	t.Run("untracked closure file", func(t *testing.T) {
		root, opts := closureRepo(t, nil)
		write(t, root, map[string]string{"helpers/new.ts": "export const n = 1;\n", "e2e/cart.spec.ts": "import { n } from '../helpers/new';\ntest('cart', async () => { n; });\n"})
		doc := produce(t, opts)
		if !strings.Contains(doc.Closure.Unknown["cart.spec.ts"], "helpers/new.ts") || doc.CorvintRunIdentity.TestClosureDigests["cart.spec.ts"] != "" {
			t.Fatalf("untracked closure file: %+v", doc.Closure)
		}
	})
	t.Run("unresolved config", func(t *testing.T) {
		_, opts := closureRepo(t, map[string]string{"playwright.config.ts": "export default defineConfig({ testDir: 'e2e', projects: makeProjects() });\n"})
		doc := produce(t, opts)
		if len(doc.Closure.Unknown) != 2 || doc.CorvintRunIdentity.TestClosureDigests != nil || doc.Counts.ClosureDigests != 0 ||
			doc.CorvintRunIdentity.TestClosureRule != ClosureRuleComputed {
			t.Fatalf("unresolved config: %+v closure %+v", doc.CorvintRunIdentity, doc.Closure)
		}
	})
	t.Run("frontier outside the suite widens", func(t *testing.T) {
		root, opts := closureRepo(t, map[string]string{"src/app.ts": "import { price } from './price';\nconst name = process.env.APP;\nexport async function app() { price(); await import(name); }\n"})
		doc := produce(t, opts)
		if doc.Closure.Widened == "" || !slices.Contains(doc.Limits, LimitClosureWidened) || len(doc.CorvintRunIdentity.TestClosureDigests) != 2 {
			t.Fatalf("widened: %+v limits %v", doc.Closure, doc.Limits)
		}
		cart := doc.CorvintRunIdentity.TestClosureDigests["cart.spec.ts"]
		commitEdit(t, root, "helpers/login.ts", "export function login() {}\n")
		if produce(t, opts).CorvintRunIdentity.TestClosureDigests["cart.spec.ts"] == cart {
			t.Fatal("a widened closure left a file out")
		}
	})
}

// PWR-V0-178: a reason shared by several tests is stated once with its count.
func TestSharedClosureCauses_PWRV0178(t *testing.T) {
	got := sharedClosureCauses(map[string]string{"a": "config dynamic", "b": "config dynamic", "c": "import in c"})
	if len(got) != 1 || got[0].Reason != "config dynamic" || got[0].Tests != 2 {
		t.Fatalf("shared = %+v", got)
	}
	if sharedClosureCauses(map[string]string{"a": "x"}) != nil || sharedClosureCauses(nil) != nil {
		t.Fatal("a single test's reason is not shared")
	}
}

// PWR-V0-186 (GitHub #1920): the reporter's config (a dynamic use.launchOptions, process.env reads,
// an escaped regexp string in its own identity call and a setup project from --project-test-dir)
// digests every test, nothing unknown or widened.
func TestPWRV0186_IssueConfigDigestsEveryTest(t *testing.T) {
	config := "import { defineConfig, devices } from '@playwright/test';\n" +
		"import { execFileSync } from 'node:child_process';\n" +
		"const bin = process.env.CORVINT_BIN ?? 'corvint';\n" +
		"const extraArgs = process.env.CI ? ['--no-sandbox'] : [];\n" +
		"export default defineConfig({\n  testDir: './tests',\n" +
		"  metadata: { corvintRunIdentity: JSON.parse(execFileSync(bin, ['runs', 'identity', '--config', __filename, '--test-dir', 'tests', '--project-test-dir', 'setup:.*\\\\.ts$', '--compute-closures'], { encoding: 'utf8' })).corvintRunIdentity },\n" +
		"  use: { baseURL: process.env.BASE_URL ?? 'http://localhost:3000', launchOptions: { args: [...extraArgs, '--disable-gpu'] } },\n" +
		"  projects: [\n    { name: 'setup', testDir: './setup', testMatch: /.*\\.ts$/ },\n" +
		"    { name: 'chromium', use: { ...devices['Desktop Chrome'] }, dependencies: ['setup'] },\n  ],\n});\n"
	root, opts := closureRepo(t, map[string]string{
		"playwright.config.ts": config,
		"package.json":         `{"name":"shop","devDependencies":{"@playwright/test":"1.63.0"}}`,
		"tests/cart.spec.ts":   "import { cart } from '../helpers/cart';\ntest('cart', async () => { cart(); });\n",
		"tests/login.spec.ts":  "import { login } from '../helpers/login';\ntest('login', async () => { login(); });\n",
		"setup/auth.setup.ts":  "import { auth } from '../helpers/auth';\nsetup('auth', async () => { auth(); });\n",
		"helpers/auth.ts":      "export function auth() {}\n",
		// The default e2e specs are outside this suite; importing nothing, they reach no helper.
		"e2e/cart.spec.ts":  "test('cart', async () => {});\n",
		"e2e/login.spec.ts": "test('login', async () => {});\n",
	})
	opts.TestDir = filepath.Join(root, "tests")
	opts.ProjectTestDirs = []ProjectTestDir{{Dir: filepath.Join(root, "setup"), TestMatch: `.*\.ts$`}}
	doc := produce(t, opts)
	d := doc.CorvintRunIdentity
	if doc.Closure == nil || doc.Closure.Unknown != nil || doc.Closure.Widened != "" || doc.Closure.SharedCauses != nil || doc.Counts.ClosureDigests != 3 {
		t.Fatalf("closure = %+v counts %+v", doc.Closure, doc.Counts)
	}
	for _, key := range []string{"cart.spec.ts", "login.spec.ts", "../setup/auth.setup.ts"} {
		if !strings.HasPrefix(d.TestClosureDigests[key], "sha256:") {
			t.Fatalf("%s digest = %q in %v", key, d.TestClosureDigests[key], d.TestClosureDigests)
		}
	}
	// A helper only the login test loads leaves the cart and setup digests unchanged.
	cart, setup := d.TestClosureDigests["cart.spec.ts"], d.TestClosureDigests["../setup/auth.setup.ts"]
	commitEdit(t, root, "helpers/login.ts", "import { price } from '../src/price';\nexport function login() { price(); price(); }\n")
	after := produce(t, opts).CorvintRunIdentity.TestClosureDigests
	if after["cart.spec.ts"] != cart || after["../setup/auth.setup.ts"] != setup || after["login.spec.ts"] == d.TestClosureDigests["login.spec.ts"] {
		t.Fatalf("before %v after %v", d.TestClosureDigests, after)
	}
}

// PWR-V0-186: when tests widen for different reasons, `closure.widened` is the first reason in
// sorted order, independent of map iteration.
func TestPWRV0186_WidenedReasonDeterministic(t *testing.T) {
	config := "export default defineConfig({ testDir: 'e2e', projects: [\n" +
		"  { name: 'cart', testMatch: /cart/, use: { trace: pickTrace() } },\n" +
		"  { name: 'login', testMatch: /login/, metadata: { build: buildId() } },\n] });\n"
	_, opts := closureRepo(t, map[string]string{"playwright.config.ts": config})
	for range 8 {
		doc := produce(t, opts)
		if doc.Closure == nil || doc.Closure.Unknown != nil || doc.Counts.ClosureDigests != 2 || !strings.HasPrefix(doc.Closure.Widened, "cart use value can run code") {
			t.Fatalf("closure = %+v", doc.Closure)
		}
	}
}
