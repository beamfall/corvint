package typescript

import (
	"strings"
	"testing"
)

// gh1881Helper is the GitHub #1881 route handler: a fresh object patched by computed writes and
// handed to `.fulfill(...)`, in a scope where a type annotation names an optional key of the same
// name.
const gh1881Helper = "export async function mockSettings(route: any, response: any, keys: string[], value: string) {\n" +
	"  const body = (await response.json()) as { settings?: Record<string, unknown> } | undefined;\n" +
	"  const settings = { ...body.settings };\n" +
	"  for (const key of keys) settings[key] = value;\n" +
	"  await route.fulfill({ response, json: { ...body, settings } });\n" +
	"}\n"

// gh1881Walk collects the keys of a fresh object it fills by computed writes and enumerates
// with `Object.entries`.
const gh1881Walk = "export function ids(keys: string[]) {\n" +
	"  const seen = {};\n" +
	"  for (const key of keys) seen[key] = true;\n" +
	"  return Object.entries(seen).map(([key]) => key);\n" +
	"}\n"

// TJAA-V0-116 and TJAA-V0-117 (GitHub #1881): a type member key that spells a fresh object's name
// is no use of the binding, and `Object.entries(NAME)` over an admitted fresh object without a
// spread is an admitted use. Each near miss still stops and names its line.
func TestPlaywrightTagClosureFreshObjectTypeKeyAndEntries_GH1881(t *testing.T) {
	const helperStop = "lib/control.ts holds a tag or construct that is not one literal @\\S+ string (line "
	const computed = "a computed member access can reach a tag array without naming it)"
	const reflective = "`Object` can build or enumerate a property at run time)"
	const self = "`this` can bind the fresh object a computed member write sets, so its value can reach a test without its name)"
	const plain = "import { mockSettings } from '../lib/control';\ntest('plain', async () => {});\n"
	before := func(helper, statement string) string {
		return strings.Replace(helper, "  await route", "  "+statement+"\n  await route", 1)
	}
	walk := func(old, replacement string) string { return strings.Replace(gh1881Walk, old, replacement, 1) }
	for _, row := range []struct {
		requirement, label, helper, stop string
	}{
		// TJAA-V0-116: a type member key names no binding.
		{"116", "optional type key of the same name (the issue's handler)", gh1881Helper, ""},
		{"116", "type key after a semicolon", strings.Replace(gh1881Helper, "{ settings?: Record<string, unknown> }", "{ id: number; settings: Record<string, unknown> }", 1), ""},
		{"116", "optional type key after a comma", strings.Replace(gh1881Helper, "{ settings?: Record<string, unknown> }", "{ id: number, settings ?: Record<string, unknown> }", 1), ""},
		{"116", "optional parameter of the same name", before(gh1881Helper, "const pick = (settings?: string) => settings;"), helperStop + "4: " + computed},
		{"116", "conditional read in a block", before(gh1881Helper, "if (value) { settings ? keep() : keep(); }"), helperStop + "4: " + computed},
		{"116", "optional member read", before(gh1881Helper, "{ settings?.a; }"), helperStop + "4: " + computed},
		// TJAA-V0-117: Object.entries over an admitted fresh object without a spread.
		{"117", "entries over a fresh object filled by computed writes", gh1881Walk, ""},
		{"117", "entries over a fresh object with no computed access", "export function pairs() {\n  const seen = { a: 1 };\n  return Object.entries(seen);\n}\n", ""},
		{"117", "entries over a spread initializer", walk("const seen = {};", "const seen = { ...keys };"), helperStop + "4: " + reflective},
		{"117", "entries over a nested spread", walk("const seen = {};", "const seen = { a: { ...keys } };"), helperStop + "4: " + reflective},
		{"117", "entries over a parameter", walk("Object.entries(seen)", "Object.entries(keys)"), helperStop + "4: " + reflective},
		{"117", "entries with a second argument", walk("Object.entries(seen)", "Object.entries(seen, keys)"), helperStop + "4: " + reflective},
		// AFP-V0-960 admits Object.values on the same terms as Object.entries.
		{"117", "values over a fresh object", walk("Object.entries(seen)", "Object.values(seen)"), ""},
		{"117", "entries over an escaping fresh object", walk("  return", "  keep(seen);\n  return"), helperStop + "5: " + reflective},
		{"117", "entries over a parenthesized argument", walk("Object.entries(seen)", "Object.entries((seen))"), helperStop + "4: " + reflective},
		{"117", "optional entries call", walk("Object.entries(seen)", "Object.entries?.(seen)"), helperStop + "4: " + reflective},
		{"117", "entries over a shadowing parameter", walk("  return", "  const inner = (seen: any) => Object.entries(seen);\n  return"), helperStop + "4: " + reflective},
		{"117", "this beside an enumerated fresh object", walk("const seen = {};", "const seen = { f() { return this; } };"), helperStop + "2: " + self},
	} {
		t.Run("TJAA-V0-"+row.requirement+" "+row.label, func(t *testing.T) { gh1737FreshObjectRow(t, row.helper, plain, row.stop) })
	}
}
