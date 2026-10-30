package typescript

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// gh1737Config is the GitHub #1717 config with a testIgnore on the chromium project (line 8)
// and, when global is not empty, a global testIgnore (line 5), so lines 6 to 9 hold the projects.
func gh1737Config(chromiumIgnore, global string) string {
	config := gh1717Config("'setup/global-setup.ts'", "'setup/**/*.setup.ts'")
	if chromiumIgnore != "" {
		config = strings.Replace(config, "dependencies: testProjectDependencies }", "dependencies: testProjectDependencies, testIgnore: "+chromiumIgnore+" }", 1)
	}
	if global != "" {
		config = strings.Replace(config, "  testDir: './tests',\n", "  testDir: './tests',\n  testIgnore: "+global+",\n", 1)
	}
	return config
}

// TJAA-V0-094 (GitHub #1737 item 1): in path mode a project testIgnore of literal root-free
// string globs resolves when no test path its project's testMatch holds depends on directories
// above the repository for the ignore verdict; a testIgnore the rule refuses, a global testIgnore
// and an unsupported matcher name their own line and token instead of an unlined `testMatch`.
func TestPlaywrightPathModeLiteralTestIgnore_GH1737(t *testing.T) {
	line8 := FrontierConfig + ":playwright.config.ts:8 `testIgnore`"
	for _, row := range []struct {
		label, chromium, global, want string
		extra                         map[string]string
	}{
		{"literal project testIgnore", "'**/*.setup.ts'", "", "", nil},
		{"literal testIgnore array", "['**/*.setup.ts', 'tests/skip/*.spec.ts']", "", "", nil},
		{"testIgnore reached through a parent directory (TJAA-V0-105)", "['**/*.setup.ts', 'fixtures/**/*.ts']", "", "", nil},
		{"testIgnore on a path outside the matcher model", "'**/*.setup.ts'", "", line8, map[string]string{"tests/ b.spec.ts": "export const b = 1;\n"}},
		{"regular expression testIgnore (TJAA-V0-127)", "/.*\\.setup\\.ts/", "", "", nil},
		{"word-boundary regular expression testIgnore", "/\\bsetup\\.ts$/", "", line8, nil},
		{"computed testIgnore", "process.env.IGNORE", "", line8, nil},
		{"trailing globstar testIgnore", "'tests/skip/**'", "", line8, nil},
		{"global testIgnore", "", "'**/*.setup.ts'", FrontierConfig + ":playwright.config.ts:5 `testIgnore`", nil},
	} {
		t.Run(row.label, func(t *testing.T) {
			root := gh1717Repository(t, gh1737Config(row.chromium, row.global))
			write(t, root, "tests/skip/b.spec.ts", "import { test } from '@playwright/test';\ntest('b', async () => {});\n")
			for name, body := range row.extra {
				write(t, root, name, body)
			}
			result, err := New().Units(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			config := slices.DeleteFunc(slices.Clone(result.Frontier), func(entry string) bool { return !strings.HasPrefix(entry, FrontierConfig) })
			if row.want == "" && len(config) != 0 || row.want != "" && !slices.Equal(config, []string{row.want}) {
				t.Fatalf("executable-config sites %q, want %q", config, row.want)
			}
		})
	}
}

// gh1737Helper is the reporter's route-mocking helper shape (GitHub #1737 item 2): a fresh object
// spread written by computed keys (line 5) whose only other uses are an `in` test and a property
// of a route.fulfill argument (line 6).
const gh1737Helper = "export async function mockSettings(route: any, body: any, settingKeys: string[], value: string) {\n" +
	"  const response = await route.fetch();\n" +
	"  const settings = { ...body.settings };\n" +
	"  const present = settingKeys.filter(key => key in settings);\n" +
	"  for (const key of present.length > 0 ? present : settingKeys) settings[key] = value;\n" +
	"  await route.fulfill({ response, json: { ...body, settings } });\n" +
	"}\n"

// TJAA-V0-095 (GitHub #1737 item 2): a computed member write `NAME[key] = value` into a const
// fresh object literal leaves the tag closure exact when every other use of NAME provably keeps
// its value from a test or describe call: an `in` operand, a for-in subject, or a property or
// spread of object literals nested directly in a `.fulfill(...)` argument. Any other use, a
// `this` or `super` in the closure, or a `fulfill` defined in the closure while the object reaches
// one still stops and names its line.
func TestPlaywrightTagClosureFreshObjectWrite_GH1737(t *testing.T) {
	const helperStop = "lib/control.ts holds a tag or construct that is not one literal @\\S+ string (line "
	const specStop = "e2e/plain.spec.ts holds a tag or construct that is not one literal @\\S+ string (line "
	const computed = "a computed member access can reach a tag array without naming it)"
	const self = "can bind the fresh object a computed member write sets, so its value can reach a test without its name)"
	const sink = "`fulfill` is defined or read other than as a direct method call, so a fresh object a computed member write sets can reach a test through it)"
	const plain = "import { mockSettings } from '../lib/control';\ntest('plain', async () => {});\n"
	replace := func(old, replacement string) string { return strings.Replace(gh1737Helper, old, replacement, 1) }
	for _, row := range []struct {
		label, helper, plain, stop string
	}{
		{"reporter helper shape", gh1737Helper, plain, ""},
		{"declaration without a semicolon", replace("body.settings };", "body.settings }"), plain, ""},
		{"typed declaration", replace("const settings =", "const settings: Record<string, string> ="), plain, ""},
		{"for-in subject and spread sink", replace("{ ...body, settings }", "{ ...settings }") + "export function keys(all: string[]) {\n  const seen = { a: 1 };\n  for (const key in seen) all.push(key);\n  seen[all[0]] = 2;\n}\n", plain, ""},
		{"fulfill defined while no object reaches one", replace("  await route.fulfill({ response, json: { ...body, settings } });\n", ""), "import { mockSettings } from '../lib/control';\nconst fake = { fulfill: (options: any) => options };\ntest('plain', async () => {});\n", ""},
		{"returned", replace("  await route", "  return settings;\n  await route"), plain, helperStop + "5: " + computed},
		{"passed to a call", replace("  await route", "  keep(settings);\n  await route"), plain, helperStop + "5: " + computed},
		{"aliased", replace("  await route", "  const alias = settings;\n  await route"), plain, helperStop + "5: " + computed},
		{"template expression", replace("  await route", "  const text = `${settings}`;\n  await route"), plain, helperStop + "5: " + computed},
		{"shadowed by a parameter", replace("  await route", "  const inner = (settings: any) => settings;\n  await route"), plain, helperStop + "5: " + computed},
		{"property read", replace("  await route", "  const all = settings.all;\n  await route"), plain, helperStop + "5: " + computed},
		{"another route method", replace("route.fulfill(", "route.continue("), plain, helperStop + "5: " + computed},
		{"wrapped inside the fulfill argument", replace("json: { ...body, settings }", "json: wrap({ settings })"), plain, helperStop + "5: " + computed},
		{"conditional inside the fulfill argument", replace("json: { ...body, settings }", "json: value ? { settings } : body"), plain, helperStop + "5: " + computed},
		{"fulfill read without a call", replace("  await route", "  const send = route.fulfill;\n  await route"), plain, helperStop + "6: " + sink},
		{"compound write whose value is used", replace("settings[key] = value", "keep(settings[key] ||= value)"), plain, helperStop + "5: " + computed},
		{"computed read", replace("  await route", "  const first = settings[present[0]];\n  await route"), plain, helperStop + "5: " + computed},
		{"let declaration reassigned", strings.Replace(replace("const settings =", "let settings ="), "  await route", "  settings = body;\n  await route", 1), plain, helperStop + "5: " + computed},
		{"exported declaration", "export const settings = { a: 1 };\nexport function set(key: string, value: string) {\n  settings[key] = value;\n}\n", plain, helperStop + "3: " + computed},
		{"aliasing initializer", replace("{ ...body.settings }", "body.settings"), plain, helperStop + "5: " + computed},
		{"this in the test file", gh1737Helper, "import { mockSettings } from '../lib/control';\nconst box = { me() { return this; } };\ntest('plain', async () => {});\n", specStop + "2: `this` " + self},
		{"super in the helper", gh1737Helper + "export const box = { me() { return super.valueOf(); } };\n", plain, helperStop + "8: `super` " + self},
		{"escaped fulfill method name", gh1737Helper, "import { mockSettings } from '../lib/control';\nconst fake = { '\\x66ulfill'(options: any) { return options; } };\ntest('plain', async () => {});\n", specStop + "2: an escaped string method name can spell a method the closure trusts)"},
		{"fulfill defined in the test file", gh1737Helper, "import { mockSettings } from '../lib/control';\nconst fake = { fulfill: (options: any) => options };\ntest('plain', async () => {});\n", specStop + "2: " + sink},
	} {
		t.Run(row.label, func(t *testing.T) { gh1737FreshObjectRow(t, row.helper, row.plain, row.stop) })
	}
}

// gh1737FreshObjectRow writes helper as lib/control.ts and plain as e2e/plain.spec.ts in the grep
// fixture and checks that the listing verifies when stop is empty, or else stops with one omission
// naming stop.
func gh1737FreshObjectRow(t *testing.T, helper, plain, stop string) {
	t.Helper()
	root := grepFixture(t, grepRepro)
	write(t, root, "lib/control.ts", helper)
	write(t, root, "e2e/plain.spec.ts", plain)
	unverified, omissions := gh1647Omission(t, root, grepReproCases())
	if stop == "" {
		if len(unverified) != 0 || len(omissions) != 0 {
			t.Fatalf("not verified: unverified=%+v omissions=%+v", unverified, omissions)
		}
		return
	}
	if len(omissions) != 1 || !strings.Contains(omissions[0].Detail, stop) {
		t.Fatalf("omissions=%+v, want a stop naming %q", omissions, stop)
	}
}

// TJAA-V0-096 to TJAA-V0-099 (GitHub #1737 item 2): the scoped escape analysis keeps the tag
// closure exact for a computed member access on a fresh `const` or `let` object only when every use
// of that binding in its lexical scope provably keeps its value from a test or describe title, tag
// or details argument. Each admitted construct has a positive row; each near miss whose value can
// flow on, or whose binding the analysis cannot prove, stops and names the line of the first
// computed member access it cannot admit.
func TestPlaywrightTagClosureFreshObjectProof_GH1737(t *testing.T) {
	const helperStop = "lib/control.ts holds a tag or construct that is not one literal @\\S+ string (line "
	const computed = "a computed member access can reach a tag array without naming it)"
	const reflective = "`JSON` can build or enumerate a property at run time)"
	const accessor = "defines an accessor that a computed member access on a fresh object can run, so a value it reads or writes can reach a test without its name)"
	const sink = "`fulfill` is defined or read other than as a direct method call, so a fresh object a computed member write sets can reach a test through it)"
	const specStop = "e2e/plain.spec.ts holds a tag or construct that is not one literal @\\S+ string (line "
	const plain = "import { mockSettings } from '../lib/control';\ntest('plain', async () => {});\n"
	replace := func(old, replacement string) string { return strings.Replace(gh1737Helper, old, replacement, 1) }
	before := func(statement string) string { return replace("  await route", "  "+statement+"\n  await route") }
	const other = "export function other(all: string[]) {\n  const settings = { a: 1 };\n  for (const key of all) settings[key] = 'x';\n"
	for _, row := range []struct {
		requirement, label, helper, stop string
	}{
		// TJAA-V0-096: the binding, its lexical scope and its shadows.
		{"096", "same name bound again in another function", gh1737Helper + other + "  return all.filter(key => key in settings);\n}\n", ""},
		{"096", "nested block redeclaration", before("{\n    const settings = { b: 2 };\n    settings[value] = value;\n  }"), ""},
		{"096", "let without reassignment", replace("const settings =", "let settings ="), ""},
		{"096", "other function's binding escapes", gh1737Helper + other + "  return settings;\n}\n", helperStop + "10: " + computed},
		{"096", "nested block binding escapes", before("{\n    const settings = { b: 2 };\n    settings[value] = value;\n    keep(settings);\n  }"), helperStop + "8: " + computed},
		{"096", "receiver outside the declaration's scope", gh1737Helper + "export function late(key: string) {\n  settings[key] = key;\n}\n", helperStop + "9: " + computed},
		{"096", "var declaration", replace("const settings =", "var settings ="), helperStop + "5: " + computed},
		{"096", "declaration list after its first name", replace("const settings =", "const other = 1, settings ="), helperStop + "5: " + computed},
		{"096", "var in a nested function", before("const inner = () => { var settings = body; return settings; };"), helperStop + "5: " + computed},
		{"096", "function declaration of the name", before("function settings() {}"), helperStop + "5: " + computed},
		{"096", "with statement in the file", gh1737Helper + "export function scoped(o: any) {\n  with (o) {}\n}\n", helperStop + "5: " + computed},
		{"096", "combining mark makes a different name", before("{\n    const settings\u0301 = 0;\n    keep(settings);\n  }"), helperStop + "5: " + computed},
		{"096", "getter on the fresh literal", replace("{ ...body.settings }", "{ ...body.settings, get first() { return value; } }"), helperStop + "3: `get` " + accessor},
		{"096", "class setter in the helper", gh1737Helper + "export class Box { set size(v: string) {} }\n", helperStop + "8: `set` " + accessor},
		{"096", "get and set as names, not accessors", before("const named = { get: 1, set(v: string) { return v; } }; body.get(value);"), ""},
		{"096", "destructuring assignment into the binding", replace("route.fulfill({ response, json: { ...body, settings } })", "route.fulfill({ settings } = body)"), helperStop + "5: " + computed},
		// TJAA-V0-097: a compound computed write whose value the statement discards.
		{"097", "compound write after a for header", replace("settings[key] = value", "settings[key] ||= value"), ""},
		{"097", "compound write after an if header", before("if (value) settings[value] += value;"), ""},
		{"097", "compound write after a semicolon and else", before("if (value) settings[value] = value; else settings[value] ??= value;"), ""},
		{"097", "compound write as an arrow body", before("const set = () => settings[value] ||= value;"), helperStop + "5: " + computed},
		{"097", "compound write as an initializer", before("const old = settings[value] += value;"), helperStop + "5: " + computed},
		{"097", "compound write after a label", before("here: settings[value] ||= value;"), helperStop + "5: " + computed},
		{"097", "compound write in a template", before("const text = `${settings[value] ||= value}`;"), helperStop + "5: " + computed},
		// TJAA-V0-098: a computed read only an equality, `!`, `typeof` or a test consumes.
		{"098", "read compared by strict equality", before("if (settings[value] === value) return;"), ""},
		{"098", "read as a right equality operand", before("const same = value !== settings[value];"), ""},
		{"098", "read negated", before("const missing = !settings[value];"), ""},
		{"098", "read type compared", before("const kind = typeof settings[value] === 'string';"), ""},
		{"098", "read as an if test", before("if (settings[value]) return;"), ""},
		{"098", "read as a while test", before("while (settings[value]) break;"), ""},
		{"098", "read loosely compared before ||", before("const either = settings[value] == null || present.length > 0;"), ""},
		{"098", "read with a member call", before("settings[value].push(value);"), helperStop + "5: " + computed},
		{"098", "read as an || operand", before("const pick = settings[value] || value;"), helperStop + "5: " + computed},
		{"098", "read as a ?? operand", before("const pick = settings[value] ?? value;"), helperStop + "5: " + computed},
		{"098", "read member compared", before("const n = value === settings[value].length;"), helperStop + "5: " + computed},
		{"098", "read called", before("settings[value]();"), helperStop + "5: " + computed},
		{"098", "read constructed", before("const n = new settings[value] === value;"), helperStop + "5: " + computed},
		{"098", "read awaited", before("const n = await settings[value] === value;"), helperStop + "5: " + computed},
		{"098", "right operand before a line break", before("const same = value === settings[value]"), helperStop + "5: " + computed},
		{"098", "right operand before addition", before("const same = value === settings[value] + 1;"), helperStop + "5: " + computed},
		{"098", "parenthesized test", before("if ((settings[value])) return;"), helperStop + "5: " + computed},
		{"098", "read as a computed key", before("const pick = body[settings[value]];"), helperStop + "5: " + computed},
		// TJAA-V0-099: the whole first argument of a plain JSON.stringify call.
		{"099", "stringified into the fulfill body", replace("json: { ...body, settings }", "body: JSON.stringify(settings)"), ""},
		{"099", "stringified with null replacer and space into the fulfill body", replace("json: { ...body, settings }", "body: JSON.stringify(settings, null, 2)"), ""},
		{"099", "stringified into an initializer", before("const text = JSON.stringify(settings, null, 2);"), helperStop + "5: " + computed},
		{"099", "stringified into a test title", before("test(JSON.stringify(settings), () => {});"), helperStop + "5: " + computed},
		{"099", "stringified in a conditional fulfill value", replace("json: { ...body, settings }", "body: value ? JSON.stringify(settings) : ''"), helperStop + "5: " + computed},
		{"099", "stringified through a wrapper call", before("const text = JSON.stringify(wrap(settings));"), helperStop + "5: " + computed},
		{"099", "stringified inside an object literal", before("const text = JSON.stringify({ settings });"), helperStop + "5: " + computed},
		{"099", "stringified with a replacer", before("const text = JSON.stringify(settings, (key, item) => item);"), helperStop + "6: " + reflective},
		{"099", "stringified through JSON?.", before("const text = JSON?.stringify(settings);"), helperStop + "6: " + reflective},
	} {
		t.Run("TJAA-V0-"+row.requirement+" "+row.label, func(t *testing.T) { gh1737FreshObjectRow(t, row.helper, plain, row.stop) })
	}
	t.Run("TJAA-V0-096 setter in the test file", func(t *testing.T) {
		gh1737FreshObjectRow(t, gh1737Helper, "import { mockSettings } from '../lib/control';\nconst box = { set x(v: string) {} };\ntest('plain', async () => {});\n", specStop+"2: `set` "+accessor)
	})
	t.Run("TJAA-V0-099 stringified into fulfill while fulfill is defined", func(t *testing.T) {
		helper := strings.Replace(gh1737Helper, "json: { ...body, settings }", "body: JSON.stringify(settings)", 1)
		gh1737FreshObjectRow(t, helper, "import { mockSettings } from '../lib/control';\nconst fake = { fulfill: (options: any) => options };\ntest('plain', async () => {});\n", specStop+"2: "+sink)
	})
}

// TJAA-V0-103 (GitHub #1737, V1-2629): a computed member read or write whose receiver is an object
// literal, `{ ... }[k]`, stops the tag closure and names its line, because `}` before `[` can end
// the literal whose computed member reaches a tag array without naming it. A `}` also ends a
// block, so a block followed by a non-literal array literal stops too (conservatively); a literal
// index and the `T[]` array type after an object type stay exact.
func TestPlaywrightTagClosureObjectLiteralReceiver_GH1737(t *testing.T) {
	const helperStop = "lib/control.ts holds a tag or construct that is not one literal @\\S+ string (line "
	const receiver = "a `[` after `}` can index an object literal and reach a tag array without naming it)"
	const plain = "import { mockSettings } from '../lib/control';\ntest('plain', async () => {});\n"
	for _, row := range []struct {
		label, extra, stop string
	}{
		{"computed read on an object literal", "export function f(k: string) { return { tag: ['@a'] }[k]; }\n", helperStop + "8: " + receiver},
		{"computed write on an object literal", "export function g(k: string, v: any) {\n  return ({ tag: ['@a'] }[k] = v);\n}\n", helperStop + "9: " + receiver},
		{"block then a non-literal array literal", "export function e(x: boolean, all: string[]) {\n  if (x) {}\n  [all[0], 'b'].forEach(n => all.push(n));\n}\n", helperStop + "10: " + receiver},
		{"literal index on an object literal", "export function l() { return { a: 1 }[0]; }\n", ""},
		{"array of an object type", "export const rows: { a: number }[] = [];\n", ""},
	} {
		t.Run(row.label, func(t *testing.T) { gh1737FreshObjectRow(t, gh1737Helper+row.extra, plain, row.stop) })
	}
}
