package typescript

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// Synthetic example-app-e2e shape: this is not an observation of the consumer checkout.
func exampleAppPlaywrightFixture(t *testing.T) string {
	t.Helper()
	root := qualificationCorpus(t)
	write(t, root, "playwright.config.ts", strings.Replace(qualificationConfig, "defineConfig({ projects:", `defineConfig({ use: { browserName: "chromium", baseURL: "http://localhost:4200", trace: "retain-on-failure" }, globalSetup: "./support/global-setup.ts", projects:`, 1))
	write(t, root, "tsconfig.json", `{"compilerOptions":{"baseUrl":".","paths":{"@pages/*":["support/page*"],"@scenarios/*":["support/scenario*"],"@fixtures":["support/shared.ts"],"@workflows/*":["support/workflow*"]}}}`)
	write(t, root, "support/global-setup.ts", `import { setup } from "./setup-helper"; export default setup`)
	write(t, root, "support/setup-helper.ts", `export const setup = () => {}`)
	write(t, root, "support/workflow0.ts", `export { value } from "@pages/0"`)
	write(t, root, "support/scenario0.ts", `export { value } from "@workflows/0"`)
	write(t, root, "support/page0.ts", `export { value } from "support/helper0"`)
	write(t, root, "tests/case000.spec.ts", `import { test } from "@fixtures"; import { value } from "@scenarios/0"; test("example-app", () => value)`)
	return root
}

func TestPlaywrightExampleAppQualification(t *testing.T) {
	t.Run("TJAA-V0-017 global use aliases and narrow closure", func(t *testing.T) {
		root := exampleAppPlaywrightFixture(t)
		for _, dirty := range []string{"support/helper0.ts", "support/page0.ts", "support/workflow0.ts", "support/scenario0.ts"} {
			plan := qualifySelection(t, root, dirty)
			if plan.Scope != affected.ScopeBounded || plan.Fallback != PlaywrightFallbackNone || !slices.Equal(playwrightSelectionIDs(plan), qualificationOracle(0, 13)) {
				t.Fatalf("%s: selected=%v unknown=%v", dirty, playwrightSelectionIDs(plan), plan.Unknown)
			}
			if !hasPlaywrightUnknown(plan, PlaywrightAxisExecution, PlaywrightUnknownExternalApplication) {
				t.Fatal("external application frontier lost")
			}
			first, _ := plan.Canonical()
			repeated := qualifySelection(t, root, dirty)
			second, _ := repeated.Canonical()
			if !bytes.Equal(first, second) {
				t.Fatal("receipt bytes differ")
			}
		}
		shared, err := New().Units(root)
		if err != nil || !slices.Contains(shared.Frontier, FrontierPathAlias) {
			t.Fatal("general adapter alias boundary changed", err)
		}
	})
	t.Run("TJAA-V0-013 setup helper and config closure", func(t *testing.T) {
		root := exampleAppPlaywrightFixture(t)
		for _, dirty := range []string{"support/setup-helper.ts", "setup/global.setup.ts", "playwright.config.ts"} {
			plan := qualifySelection(t, root, dirty)
			if plan.Scope != affected.ScopeBounded || !slices.Equal(playwrightSelectionIDs(plan), qualificationOracle(0, 117)) {
				t.Fatalf("%s: selected=%d unknown=%v", dirty, len(plan.Selected), plan.Unknown)
			}
		}
	})
	t.Run("TJAA-V0-015 known widening frontiers", func(t *testing.T) {
		for _, row := range []struct{ name, file, body, reason string }{
			{"computed import", "support/page0.ts", `export const value = import(process.env.PAGE)`, FrontierDynamicImport},
			{"missing alias", "support/page0.ts", `export { value } from "@missing/page"`, FrontierPathAlias},
			{"missing target", "support/page0.ts", `export { value } from "@pages/missing"`, FrontierPathAlias},
			{"inherited tsconfig", "tsconfig.json", `{"extends":"./base.json","compilerOptions":{"baseUrl":"."}}`, FrontierPathAlias},
			{"equal prefix alias overlap", "tsconfig.json", `{"compilerOptions":{"baseUrl":".","paths":{"@pages/*":["support/page*"],"@pages/*0":["support/helper0"]}}}`, FrontierPathAlias},
			{"multiple existing targets", "tsconfig.json", `{"compilerOptions":{"baseUrl":".","paths":{"@pages/*":["support/page*","support/helper*"]}}}`, FrontierPathAlias},
			{"ambiguous target", "support/helper0.js", `export const value = 1`, FrontierPathAlias},
			{"dynamic global use", "playwright.config.ts", `export default { use: inheritedUse, projects: [{name:"chromium"},{name:"angular"},{name:"react"},{name:"setup"},{name:"cleanup"}] }`, PlaywrightUnknownBrowserIdentity},
			{"explicit global browser default", "playwright.config.ts", `export default { use: {defaultBrowserType:"firefox"}, projects: [{name:"chromium"},{name:"angular"},{name:"react"},{name:"setup"},{name:"cleanup"}] }`, PlaywrightUnknownBrowserIdentity},
			{"explicit project browser default", "playwright.config.ts", `export default { projects: [{name:"chromium",use:{defaultBrowserType:"firefox"}},{name:"angular"},{name:"react"},{name:"setup"},{name:"cleanup"}] }`, PlaywrightUnknownBrowserIdentity},
			{"dynamic setup", "playwright.config.ts", `export default { globalSetup: setupPath, projects: [{name:"chromium"},{name:"angular"},{name:"react"},{name:"setup"},{name:"cleanup"}] }`, PlaywrightUnknownConfigSyntax},
		} {
			t.Run(row.name, func(t *testing.T) {
				root := exampleAppPlaywrightFixture(t)
				write(t, root, row.file, row.body)
				plan := qualifySelection(t, root, "support/helper0.ts")
				if plan.Scope != affected.ScopeUnknown || plan.Fallback != PlaywrightFallbackFullSuite || len(plan.Excluded) != 0 || !hasPlaywrightUnknown(plan, PlaywrightAxisSelection, row.reason) {
					t.Fatalf("frontier did not widen: %+v", plan.Unknown)
				}
				if row.file == "playwright.config.ts" {
					if len(plan.Selected) != 0 || len(plan.FallbackArgv) != 4 {
						t.Fatal("unresolved configuration emitted file units")
					}
					return
				}
				for _, required := range qualificationOracle(0, 117) {
					if !slices.Contains(playwrightSelectionIDs(plan), required) {
						t.Fatalf("unsafe exclusion %s", required)
					}
				}
			})
		}
	})
}

func TestPlaywrightGlobalUseInheritance(t *testing.T) {
	for _, row := range []struct{ global, local, browser, device string }{
		{`{ browserName: "firefox", trace: "on" }`, `{ locale: "en-US" }`, "firefox", ""},
		{`{ browserName: "firefox" }`, `{ browserName: "webkit" }`, "webkit", ""},
		{`{ browserName: "firefox" }`, `{ ...devices["Desktop Chrome"] }`, "firefox", "Desktop Chrome"},
		{`{}`, `{ browserName: "firefox", ...devices["Desktop Chrome"] }`, "firefox", "Desktop Chrome"},
		{`{ ...devices["Desktop Firefox"] }`, `{ ...devices["Desktop Chrome"] }`, "chromium", "Desktop Chrome"},
		{`{ ...devices["Desktop Firefox"] }`, `{ browserName: "webkit" }`, "webkit", "Desktop Firefox"},
	} {
		browser, device, ok := playwrightInheritedUseIdentity(playwrightPureScope{devices: true}, row.global, row.local)
		if !ok || browser != row.browser || device != row.device {
			t.Fatalf("%+v: %s %s %v", row, browser, device, ok)
		}
	}
}

func TestPlaywrightAliasResolutionBoundaries(t *testing.T) {
	t.Run("TJAA-V0-012 declared aliases remain closed", func(t *testing.T) {
		bodies := map[string]string{"src/exact.ts": "", "src/wild/x.ts": "", "a/local.ts": "", "src/fallback.ts": ""}
		config := parseTypeScriptAliases("tsconfig.json", `{
  // The source observer admits JSONC without executing configuration.
  "compilerOptions": { "baseUrl": ".", "paths": {
    "@x": ["src/exact.ts"], "@*": ["src/wild/*"], "fallback": ["missing", "src/fallback"],
  }, },
}`)
		if !config.valid {
			t.Fatal("JSONC rejected")
		}
		for _, raw := range []string{
			`{"compilerOptions":{"paths":{"@*t":["src/exact.ts"],"@*act":["src/fallback.ts"]}}}`,
			`{"compilerOptions":{"paths":{"@*act":["src/fallback.ts"],"@*t":["src/exact.ts"]}}}`,
			`{"compilerOptions":{"paths":{"@exact":["src/exact.ts","src/fallback.ts"]}}}`,
		} {
			ambiguous := parseTypeScriptAliases("tsconfig.json", raw)
			if got, unknown := ambiguous.resolve("@exact", bodies); got != "" || !unknown {
				t.Fatalf("ambiguous alias narrowed: %s -> %s, unknown=%v", raw, got, unknown)
			}
		}
		for _, row := range []struct{ ref, want string }{{"@x", "src/exact.ts"}, {"@x/x", ""}, {"fallback", "src/fallback.ts"}, {"src/exact", "src/exact.ts"}} {
			got, _ := config.resolve(row.ref, bodies)
			if got != row.want {
				t.Fatalf("%s: got %s want %s", row.ref, got, row.want)
			}
		}
		child := parseTypeScriptAliases("a/tsconfig.json", `{"compilerOptions":{"paths":{"@x":["local.ts"]}}}`)
		frontier := map[string]bool{}
		got := resolveTypeScriptAliases("a/example.spec.ts", []string{"@x"}, []typeScriptAliases{config, child}, bodies, frontier)
		if !slices.Equal(got, []string{"./local.ts"}) || len(frontier) != 0 {
			t.Fatalf("nearest config: %v %v", got, frontier)
		}
		for _, raw := range []string{
			`{"compilerOptions":{"baseUrl":"../outside"}}`,
			`{"compilerOptions":{"paths":{"@x":["../../outside"]}}}`,
			`{"compilerOptions":{"paths":{"@x":["x"],"@x":["y"]}}}`,
			`{"extends":"./base.json"}`,
			`{"compilerOptions":{"rootDirs":["src","generated"]}}`,
			`{"compilerOptions":{"moduleSuffixes":[".native",""]}}`,
		} {
			if parseTypeScriptAliases("tsconfig.json", raw).valid {
				t.Fatalf("unsupported config accepted: %s", raw)
			}
		}
	})
}

// GitHub #709 part 1 (V1-1065): a literal known device spread keeps its browser identity when the
// same `use` layer, or the inherited global layer, also carries runtime-computed values for keys
// that are not browser/device identity (TJAA-V0-018). Computed identity stays unresolved.
func TestPlaywrightDeviceSpreadBesideRuntimeUseValues_V1_1065(t *testing.T) {
	for _, row := range []struct{ name, global, local, browser, device string }{
		{"global const literal baseURL", `{ baseURL: server.baseURL ?? 'http://localhost:3000', trace: 'on-first-retry' }`, `{ ...devices['Desktop Chrome'] }`, "chromium", "Desktop Chrome"},
		{"project storageState identifier", `{}`, `{ ...devices['Desktop Firefox'], storageState: authFile }`, "firefox", "Desktop Firefox"},
		{"both layers computed", `{ trace: server.ci ? 'on' : 'off' }`, `{ ...devices['Desktop Safari'], extraHTTPHeaders: headers, video: mode }`, "webkit", "Desktop Safari"},
	} {
		t.Run(row.name, func(t *testing.T) {
			source := "import { defineConfig, devices } from '@playwright/test';\nconst authFile = 'playwright/.auth/user.json';\nconst server = { baseURL: 'http://localhost:3000', ci: false };\nexport default defineConfig({\n  use: " + row.global + ",\n  projects: [{ name: 'p', use: " + row.local + " }],\n});\n"
			projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", source)
			if len(unknown) != 0 || len(projects) != 1 || projects[0].Browser != row.browser || projects[0].Device != row.device {
				t.Fatalf("projects=%+v unknown=%+v", projects, unknown)
			}
		})
	}
	for _, use := range []string{
		`{ ...devices[name] }`,
		`{ ...devices['Desktop Chrome Canary'] }`,
		`{ ...devices['Desktop Chrome'], viewport: size }`,
		`{ ...devices['Desktop Chrome'], channel: process.env.CHANNEL }`,
		`{ ...devices['Desktop Chrome'], launchOptions: options }`,
		`{ ...devices['Desktop Chrome'], ...extra }`,
		`{ ...devices['Desktop Chrome'], baseURL: process.env.BASE_URL ?? 'http://localhost:3000' }`,
		`{ ...devices['Desktop Chrome'], [key]: value }`,
		`{ ...devices['Desktop Chrome'], 'browserName': chosen }`,
	} {
		_, _, unknown := parsePlaywrightConfig("playwright.config.ts", "export default defineConfig({ projects: [{ name: 'p', use: "+use+" }] })")
		if !hasPlaywrightUnknownReason(unknown, PlaywrightUnknownBrowserIdentity) {
			t.Fatalf("computed identity was resolved: %s %+v", use, unknown)
		}
	}
	_, _, unknown := parsePlaywrightConfig("playwright.config.ts", "export default defineConfig({ use: { userAgent: agent() }, projects: [{ name: 'p', use: { ...devices['Desktop Chrome'] } }] })")
	if !hasPlaywrightUnknownReason(unknown, PlaywrightUnknownBrowserIdentity) {
		t.Fatalf("computed inherited identity key was resolved: %+v", unknown)
	}
}

// GitHub #709 review: a non-identity `use` value is ignorable only when evaluating it cannot run
// code or write state, so it cannot rewrite a devices descriptor; quoted literal identity keys
// are the same keys (TJAA-V0-018).
func TestPlaywrightUseValueSideEffects_V1_1065(t *testing.T) {
	config := func(global, local string) string {
		return "import { defineConfig, devices } from '@playwright/test';\nconst options = { base: 'http://localhost:3000', ci: true };\nexport default defineConfig({\n  use: " + global + ",\n  projects: [{ name: 'p', use: " + local + " }],\n});\n"
	}
	for _, value := range []string{
		"options.base ?? 'http://localhost:3000'",
		"`http://localhost:${3000 + 1}/`",
		"options.ci ? 'on' : 'off'",
		"{ 'X-Token': token, Accept: 'application/json', nested: [a, options.base] }",
		"!flag && mode === 'x' || typeof limit === 'number'",
		"-2 * 2 + 1 > 0 ? options['base'] : options.base",
		"url",
		"options?.base ?? (fallback)",
	} {
		projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", config("{ baseURL: "+value+" }", "{ ...devices['Desktop Firefox'] }"))
		if len(unknown) != 0 || len(projects) != 1 || projects[0].Browser != "firefox" {
			t.Fatalf("side-effect-free value widened: %s projects=%+v unknown=%+v", value, projects, unknown)
		}
	}
	for _, value := range []string{
		"(devices['Desktop Chrome'].browserName = 'firefox', 'http://localhost:3000')",
		"path.join(dir, 'user.json')",
		"headers()",
		"settings?.()",
		"`http://${host()}/`",
		"tag`x`",
		"count++",
		"--count",
		"limit += 1",
		"cache ||= 'x'",
		"delete devices['Desktop Chrome'].browserName",
		"new URL('http://localhost')",
		"() => 'http://localhost'",
		"function () { return 1 }",
		"class {}",
		"{ get value() { return 1 } }",
		"{ value() { return 1 } }",
		"{ ...extra }",
		"[...list]",
		"(a, b)",
		"await token",
		"import('./x')",
		"/x/.source",
		"Object.assign(devices['Desktop Chrome'], { browserName: 'firefox' })",
		// Implicit coercion can call a user toString, valueOf, Symbol.toPrimitive or
		// Symbol.hasInstance, so a coercing operator admits only operands proven primitive.
		"`${url}`",
		"`http://${process.env.HOST ?? host}/`",
		"port * 2",
		"+port",
		"base + '/login'",
		"port < 1024",
		"port == 80",
		"options[key]",
		"options?.[key]",
		"{ [key]: 'x' }",
		"'k' in env",
		"env instanceof Object",
		// process.env.NAME is not proven primitive, a number's member read is not either, and
		// update operators are refused
		"`http://${process.env.HOST ?? 'localhost'}:${process.env.PORT ?? 3000}/`",
		// GitHub #709 review round 8: a member read of any root other than the devices import or
		// an unwritten plain const literal can invoke a getter or Proxy trap.
		"process.env.BASE_URL ?? 'http://localhost:3000'",
		"process.env.CI ? 'on' : 'off'",
		"{ 'X-Token': process.env.TOKEN }",
		"settings?.trace ?? (fallback)",
		"1..payload",
		"options[0]",
		"-process.env.RETRIES * 2 + 1 > 0 ? options['base'] : options[0]",
		"process.env.BASE + '/login'",
		"1..payload + ''",
		"++process.env.COUNTER",
		"--process.env.COUNTER",
	} {
		for _, layer := range [][2]string{{"{ baseURL: " + value + " }", "{ ...devices['Desktop Chrome'] }"}, {"{}", "{ ...devices['Desktop Chrome'], storageState: " + value + " }"}} {
			_, _, unknown := parsePlaywrightConfig("playwright.config.ts", config(layer[0], layer[1]))
			if !hasPlaywrightUnknownReason(unknown, PlaywrightUnknownBrowserIdentity) {
				t.Fatalf("value that can run code or write state was ignored: use %s / %s: %+v", layer[0], layer[1], unknown)
			}
		}
	}
	// The GitHub #709 round-two repro: the template converts url to a string, which runs its
	// toString before the project's device spread is evaluated, so Chromium becomes Firefox.
	repro := "import { defineConfig, devices } from '@playwright/test';\nconst url = {\n  toString() {\n    devices['Desktop Chrome'].defaultBrowserType = 'firefox';\n    return 'http://localhost:3000';\n  }\n};\nexport default defineConfig({\n  use: { baseURL: `${url}` },\n  projects: [{\n    name: 'p',\n    use: { ...devices['Desktop Chrome'] }\n  }]\n});\n"
	if _, _, unknown := parsePlaywrightConfig("playwright.config.ts", repro); !hasPlaywrightUnknownReason(unknown, PlaywrightUnknownBrowserIdentity) {
		t.Fatalf("template coercion of an object with toString was ignored: %+v", unknown)
	}
	// The GitHub #709 round-three repros: `1..payload` is a member read on the number 1, so the
	// concatenation converts an inherited object through its toString; and a replaced
	// process.env can return an object whose toString runs the same way.
	for _, setup := range []string{
		"Object.defineProperty(Number.prototype, 'payload', { get() { return { toString() { devices['Desktop Chrome'].defaultBrowserType = 'firefox'; return 'http://localhost:3000'; } }; } });",
		"process.env = new Proxy({}, { get() { return { toString() { devices['Desktop Chrome'].defaultBrowserType = 'firefox'; return 'http://localhost:3000'; } }; } });",
	} {
		value := "1..payload + ''"
		if strings.HasPrefix(setup, "process.env") {
			value = "process.env.BASE_URL + ''"
		}
		source := "import { defineConfig, devices } from '@playwright/test';\n" + setup + "\nexport default defineConfig({\n  use: { baseURL: " + value + " },\n  projects: [{ name: 'p', use: { ...devices['Desktop Chrome'] } }],\n});\n"
		if _, _, unknown := parsePlaywrightConfig("playwright.config.ts", source); !hasPlaywrightUnknownReason(unknown, PlaywrightUnknownBrowserIdentity) {
			t.Fatalf("coercion of %s was ignored: %+v", value, unknown)
		}
	}
	for _, row := range []struct{ use, browser string }{
		{`{ 'browserName': 'firefox' }`, "firefox"},
		{`{ "browserName": "webkit", 'viewport': { width: 1280, height: 720 } }`, "webkit"},
		{`{ ...devices['Desktop Chrome'], 'locale': 'en-US', "storageState": authFile }`, "chromium"},
	} {
		projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", config("{}", row.use))
		if len(unknown) != 0 || len(projects) != 1 || projects[0].Browser != row.browser {
			t.Fatalf("quoted literal identity key widened: %s projects=%+v unknown=%+v", row.use, projects, unknown)
		}
	}
}

// GitHub #709 review round 4: ECMAScript ends a `//` comment at U+2028 and U+2029 as well as at
// LF and CR, so `'safe' // c<U+2028> + mutate()` evaluates mutate() before the device spread.
func TestPlaywrightUseValueUnicodeLineTerminator_V1_1065(t *testing.T) {
	for _, terminator := range []string{" ", " "} {
		source := "import { defineConfig, devices } from '@playwright/test';\nconst mutate = () => { devices['Desktop Chrome'].defaultBrowserType = 'firefox'; return ''; };\n" +
			"export default defineConfig({\n  use: { baseURL: 'safe' // comment" + terminator + " + mutate()\n  },\n  projects: [{ name: 'p', use: { ...devices['Desktop Chrome'] } }],\n});\n"
		if _, _, unknown := parsePlaywrightConfig("playwright.config.ts", source); !hasPlaywrightUnknownReason(unknown, PlaywrightUnknownBrowserIdentity) {
			t.Fatalf("call after a %U line terminator was hidden in a comment: %+v", []rune(terminator)[0], unknown)
		}
	}
}

// GitHub #709 review round 5: a backtick inside a `${}` substitution opens a nested template, so
// `//` and a U+2028/U+2029 there are template content; the call in its substitution still runs.
func TestPlaywrightUseValueNestedTemplate_V1_1065(t *testing.T) {
	for _, terminator := range []string{"\u2028", "\u2029"} {
		source := "import { defineConfig, devices } from '@playwright/test';\nconst mutate = () => { devices['Desktop Chrome'].defaultBrowserType = 'firefox'; return ''; };\n" +
			"export default defineConfig({\n  use: { baseURL: `${`//${mutate()}" + terminator + "`}` },\n  projects: [{ name: 'p', use: { ...devices['Desktop Chrome'] } }],\n});\n"
		if _, _, unknown := parsePlaywrightConfig("playwright.config.ts", source); !hasPlaywrightUnknownReason(unknown, PlaywrightUnknownBrowserIdentity) {
			t.Fatalf("call in a nested template before a %U was hidden: %+v", []rune(terminator)[0], unknown)
		}
	}
}

// GitHub #709 review round 4: an identity value is validated recursively before the bounded
// expression parser, so its nesting must be bounded too instead of costing quadratic work.
func TestPlaywrightStaticValueNestingIsBounded_V1_1065(t *testing.T) {
	deep := strings.Repeat("[", 20000) + "'x'" + strings.Repeat("]", 20000)
	if playwrightStaticValue(deep) {
		t.Fatalf("admitted a %d-byte nested static value", len(deep))
	}
	if shallow := strings.Repeat("[", 20) + "'x'" + strings.Repeat("]", 20); !playwrightStaticValue(shallow) {
		t.Fatal("refused a shallow nested static value")
	}
}

// GitHub #709 review round 8: a member read can invoke a getter, so a non-identity `use` value
// reads a member only of the Playwright devices import (a scalar descriptor field) or of a
// top-level const bound to a plain object literal that is never written, aliased or spread
// (TJAA-V0-018). Every other root, process.env included, is refused.
func TestPlaywrightMemberReadRoots_GH709Round8(t *testing.T) {
	const header = "import { defineConfig, devices } from '@playwright/test';\n"
	config := func(setup, value string) string {
		return header + setup + "\nexport default defineConfig({\n  use: { baseURL: " + value + " },\n  projects: [{ name: 'p', use: { ...devices['Desktop Chrome'] } }],\n});\n"
	}
	// The exact round-8 repro: the getter rewrites the Chrome descriptor before the spread runs.
	repro := header + "const options = {\n  get baseURL() {\n    devices['Desktop Chrome'].defaultBrowserType = 'firefox';\n    return 'http://localhost:3000';\n  },\n};\nexport default defineConfig({\n  use: { baseURL: options.baseURL },\n  projects: [{ name: 'p', use: { ...devices['Desktop Chrome'] } }],\n});\n"
	if _, _, unknown := parsePlaywrightConfig("playwright.config.ts", repro); !hasPlaywrightUnknownReason(unknown, PlaywrightUnknownBrowserIdentity) {
		t.Errorf("a getter read was ignored: %+v", unknown)
	}
	for _, row := range []struct{ name, setup, value string }{
		{"getter without device writes in view", "const options = { get baseURL() { return go(); } };", "options.baseURL"},
		{"setter", "const options = { set baseURL(v) {}, base: 'x' };", "options.base"},
		{"method in the literal", "const options = { baseURL: 'http://x', toString() { return 'y'; } };", "options.baseURL"},
		{"spread in the literal", "const options = { ...extra, baseURL: 'http://x' };", "options.baseURL"},
		{"computed key in the literal", "const options = { ['baseURL']: 'http://x' };", "options.baseURL"},
		{"__proto__ in the literal", "const options = { __proto__: proto, baseURL: 'http://x' };", "options.baseURL"},
		{"quoted __proto__ in the literal", "const options = { '__proto__': proto };", "options.baseURL"},
		{"shorthand in the literal", "const baseURL = 'http://x';\nconst options = { baseURL };", "options.baseURL"},
		{"non-primitive value", "const options = { baseURL: url };", "options.baseURL"},
		{"member written", "const options = { baseURL: 'http://x' };\noptions.baseURL = 'http://y';", "options.baseURL"},
		{"member written through brackets", "const options = { baseURL: 'http://x' };\noptions['baseURL'] = 'http://y';", "options.baseURL"},
		{"member updated", "const options = { n: 1, baseURL: 'http://x' };\noptions.n++;", "options.baseURL"},
		{"member deleted", "const options = { baseURL: 'http://x' };\ndelete options.baseURL;", "options.baseURL"},
		{"root reassigned", "const options = { baseURL: 'http://x' };\noptions = { get baseURL() { return 'y'; } };", "options.baseURL"},
		{"root passed to a call", "const options = { baseURL: 'http://x' };\nObject.defineProperty(options, 'baseURL', { get() { return 'y'; } });", "options.baseURL"},
		{"nested object aliased", "const options = { nested: { baseURL: 'http://x' } };\nconst alias = options.nested;", "options.nested.baseURL"},
		{"destructuring assignment", "const options = { baseURL: 'http://x' };\n[options.baseURL] = ['y'];", "options.baseURL"},
		{"second binding", "const options = { baseURL: 'http://x' };\nfunction f(options) { return options; }", "options.baseURL"},
		{"exported root", "export const options = { baseURL: 'http://x' };", "options.baseURL"},
		{"let root", "let options = { baseURL: 'http://x' };", "options.baseURL"},
		{"block-scoped root", "{\n  const options = { baseURL: 'http://x' };\n}", "options.baseURL"},
		{"root under eval", "const options = { baseURL: 'http://x' };\neval(code);", "options.baseURL"},
		{"missing key", "const options = { baseURL: 'http://x' };", "options.other"},
		{"read through a leaf", "const options = { baseURL: 'http://x' };", "options.baseURL.length"},
		{"called member", "const options = { baseURL: 'http://x' };", "options.baseURL()"},
		{"undeclared root", "", "options.baseURL"},
		{"process.env", "", "process.env.BASE_URL ?? 'http://localhost:3000'"},
		{"this", "", "this.baseURL"},
		{"devices without the read rule", "", "devices['Desktop Chrome'].viewport"},
		{"devices prototype key", "", "devices['constructor'].userAgent"},
		{"devices written", "devices['Desktop Chrome'].defaultBrowserType = 'firefox';", "devices['Desktop Chrome'].userAgent"},
	} {
		t.Run(row.name, func(t *testing.T) {
			if _, _, unknown := parsePlaywrightConfig("playwright.config.ts", config(row.setup, row.value)); !hasPlaywrightUnknownReason(unknown, PlaywrightUnknownBrowserIdentity) {
				t.Fatalf("member read %s under %q was ignored: %+v", row.value, row.setup, unknown)
			}
		})
	}
	// A top-level write to a devices descriptor rewrites the spread itself.
	if _, _, unknown := parsePlaywrightConfig("playwright.config.ts", config("devices['Desktop Chrome'].defaultBrowserType = 'firefox';", "'http://x'")); !hasPlaywrightUnknownReason(unknown, PlaywrightUnknownBrowserIdentity) {
		t.Errorf("a spread after a descriptor write kept its identity: %+v", unknown)
	}
	for _, row := range []struct{ name, setup, value string }{
		{"plain const literal", "const options = { baseURL: 'http://localhost:3000', 'retries': 2 };", "options.baseURL"},
		{"nested plain literal", "const options = { server: { url: `http://${'localhost'}:${3000 + 1}` } };", "options.server['url'] ?? 'http://x'"},
		{"optional chain", "const options = { baseURL: 'http://x' };", "options?.baseURL"},
		{"literal read elsewhere", "const options = { baseURL: 'http://x', port: 1 };\nconst port = options.port + 1;", "options.baseURL"},
		{"devices scalar", "", "devices['Desktop Chrome'].userAgent"},
		{"devices viewport field", "", "devices['Desktop Chrome'].viewport.width === 1280 ? 'http://x' : 'http://y'"},
	} {
		t.Run(row.name, func(t *testing.T) {
			projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", config(row.setup, row.value))
			if len(unknown) != 0 || len(projects) != 1 || projects[0].Browser != "chromium" || projects[0].Device != "Desktop Chrome" {
				t.Fatalf("admitted member read %s widened: projects=%+v unknown=%+v", row.value, projects, unknown)
			}
		})
	}
}
