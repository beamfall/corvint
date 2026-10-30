package typescript

import (
	"slices"
	"strings"
	"testing"
)

func closureFixture(t *testing.T, config string) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "playwright.config.ts", config)
	write(t, root, "e2e/cart.spec.ts", "import { cart } from '../helpers/cart';\ntest('cart', async () => { cart(); });\n")
	write(t, root, "e2e/login.spec.ts", "import { login } from '../helpers/login';\ntest('login', async () => { login(); });\n")
	write(t, root, "helpers/cart.ts", "export function cart() {}\n")
	write(t, root, "helpers/login.ts", "import { price } from '../src/price';\nexport function login() { price(); }\n")
	write(t, root, "src/price.ts", "export function price() {}\n")
	write(t, root, "src/app.ts", "import { price } from './price';\nexport const app = price;\n")
	return root
}

const closureConfig = "export default defineConfig({ testDir: 'e2e', projects: [{ name: 'main' }] });\n"

func closuresByTest(t *testing.T, root string) map[string]PlaywrightTestClosure {
	t.Helper()
	closures, err := PlaywrightTestClosures(root, "playwright.config.ts")
	if err != nil {
		t.Fatal(err)
	}
	byTest := map[string]PlaywrightTestClosure{}
	for _, closure := range closures {
		byTest[closure.Test] = closure
	}
	return byTest
}

// TJAA-V0-140: one test's closure holds the config, its test file and what it loads; a file only
// another test loads is excludable, and a suite file that code outside the suite reaches is not.
func TestPlaywrightTestClosures_TJAA140(t *testing.T) {
	root := closureFixture(t, closureConfig)
	got := closuresByTest(t, root)
	cart, login := got["e2e/cart.spec.ts"], got["e2e/login.spec.ts"]
	if len(got) != 2 || cart.Unknown != "" || login.Unknown != "" || cart.Widened != "" {
		t.Fatalf("closures = %+v", got)
	}
	if want := []string{"e2e/cart.spec.ts", "helpers/cart.ts", "playwright.config.ts"}; !slices.Equal(cart.Files, want) {
		t.Fatalf("cart files = %v, want %v", cart.Files, want)
	}
	if want := []string{"e2e/login.spec.ts", "helpers/login.ts"}; !slices.Equal(cart.Excludable, want) {
		t.Fatalf("cart excludable = %v, want %v (src/price.ts is reached by src/app.ts)", cart.Excludable, want)
	}
	if want := []string{"e2e/login.spec.ts", "helpers/login.ts", "playwright.config.ts", "src/price.ts"}; !slices.Equal(login.Files, want) {
		t.Fatalf("login files = %v, want %v", login.Files, want)
	}
	if want := []string{"e2e/cart.spec.ts", "helpers/cart.ts"}; !slices.Equal(login.Excludable, want) {
		t.Fatalf("login excludable = %v, want %v", login.Excludable, want)
	}
	// The whole-suite plan is unchanged by the closure walk's seeds.
	plan, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"helpers/cart.ts"})
	if err != nil || len(plan.Selected) != 1 || plan.Selected[0].Test != "e2e/cart.spec.ts" {
		t.Fatalf("plan = %+v, %v", plan.Selected, err)
	}
}

// TJAA-V0-141: a closure that cannot be computed is unknown, never narrowed.
func TestPlaywrightTestClosuresFailClosed_TJAA141(t *testing.T) {
	t.Run("dynamic import in the test", func(t *testing.T) {
		root := closureFixture(t, closureConfig)
		write(t, root, "e2e/cart.spec.ts", "const name = process.env.CART;\ntest('cart', async () => { await import(name); });\n")
		got := closuresByTest(t, root)
		cart, login := got["e2e/cart.spec.ts"], got["e2e/login.spec.ts"]
		if cart.Unknown == "" || cart.Files != nil || cart.Excludable != nil {
			t.Fatalf("cart with a dynamic import = %+v", cart)
		}
		if login.Unknown != "" || !slices.Contains(login.Excludable, "e2e/cart.spec.ts") {
			t.Fatalf("login beside a dynamic import in another test = %+v", login)
		}
	})
	t.Run("dynamic import outside the suite widens", func(t *testing.T) {
		root := closureFixture(t, closureConfig)
		write(t, root, "src/app.ts", "import { price } from './price';\nconst name = process.env.APP;\nexport async function app() { price(); await import(name); }\n")
		for test, closure := range closuresByTest(t, root) {
			if closure.Unknown != "" || closure.Widened == "" || len(closure.Excludable) != 0 || len(closure.Files) == 0 {
				t.Fatalf("%s: %+v", test, closure)
			}
		}
	})
	t.Run("unresolved config", func(t *testing.T) {
		root := closureFixture(t, "export default defineConfig({ testDir: 'e2e', projects: makeProjects() });\n")
		got := closuresByTest(t, root)
		for test, closure := range got {
			if !strings.HasPrefix(closure.Unknown, "Playwright config is not statically resolved") || closure.Files != nil {
				t.Fatalf("%s: %+v", test, closure)
			}
		}
	})
	t.Run("dependency project tests join", func(t *testing.T) {
		root := closureFixture(t, "export default defineConfig({ testDir: 'e2e', projects: [{ name: 'setup', testMatch: /.*\\.setup\\.ts/ }, { name: 'main', dependencies: ['setup'] }] });\n")
		write(t, root, "e2e/auth.setup.ts", "import { auth } from '../helpers/auth';\ntest('auth', async () => { auth(); });\n")
		write(t, root, "helpers/auth.ts", "export function auth() {}\n")
		cart := closuresByTest(t, root)["e2e/cart.spec.ts"]
		if cart.Unknown != "" || !slices.Contains(cart.Files, "e2e/auth.setup.ts") || !slices.Contains(cart.Files, "helpers/auth.ts") {
			t.Fatalf("cart under a dependency project = %+v", cart)
		}
	})
}
