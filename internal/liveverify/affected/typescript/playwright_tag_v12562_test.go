package typescript

import (
	"strings"
	"testing"
)

// AFP-V0-960 (V1-2562, V1-2582): a spec that parses JSON it only serves through route.fulfill,
// enumerates a fresh object with Object.values and indexes an array by a numeric for-loop counter
// keeps the tag closure exact; a parse value, an enumerated value or a counter that can reach a
// test, and a url or dotenv member that builds an object from text, still stop and name the line.
func TestPlaywrightTagClosureParseValuesCounter_V12562(t *testing.T) {
	const stop = "e2e/plain.spec.ts holds a tag or construct that is not one literal @\\S+ string (line "
	head := "import fs from 'fs';\nimport { test, expect } from '@playwright/test';\n"
	exact := head + "const fixture = JSON.parse(fs.readFileSync('e2e/users.json', 'utf8'));\n" +
		"const counts = { a: 1, b: 2 };\n" +
		"const rows = ['a', 'b'];\n" +
		"test('plain', async ({ page }) => {\n" +
		"  await page.route('**/api/users', (route) => route.fulfill({ json: fixture }));\n" +
		"  for (const value of Object.values(counts)) {\n    expect(value).toBeGreaterThan(0);\n  }\n" +
		"  for (let i = 0; i < rows.length; i++) {\n    await expect(page.getByText(rows[i])).toBeVisible();\n  }\n" +
		"});\n"
	loop := func(body string) string {
		return head + "const rows = ['a', 'b'];\ntest('plain', async () => {\n  for (let i = 0; i < rows.length; i++) {\n" + body + "  }\n});\n"
	}
	for _, row := range []struct {
		label, plain, stop string
	}{
		{"parse, values and counter on non-tag data", exact, ""},
		{"node:path and node:fs builtins", "import path from 'node:path';\nimport { readFileSync } from 'node:fs';\nconst here = path.join('a', 'b');\ntest('plain', async () => {});\n", ""},
		{"node:vm builtin is followed", "import vm from 'node:vm';\ntest('plain', async () => {});\n", "node:vm"},
		{"url fileURLToPath and dotenv config", "import { fileURLToPath } from 'url';\nimport dotenv from 'dotenv';\ndotenv.config();\nconst here = fileURLToPath(import.meta.url);\ntest('plain', async () => {});\n", ""},
		{"parse value reaches a test", head + "const details = JSON.parse(fs.readFileSync('d.json', 'utf8'));\ntest('plain', details, async () => {});\n", stop + "3: `JSON` can build or enumerate a property at run time)"},
		{"parse with a reviver", head + "const fixture = JSON.parse('{}', (key, value) => value);\ntest('plain', async () => {});\n", stop + "3: `JSON` can build or enumerate a property at run time)"},
		{"parse member read", head + "const fixture = JSON.parse('{}').details;\ntest('plain', async () => {});\n", stop + "3: `JSON` can build or enumerate a property at run time)"},
		{"parse value read", head + "const fixture = JSON.parse('{}');\nconst copy = fixture;\ntest('plain', async () => {});\n", stop + "3: `JSON` can build or enumerate a property at run time)"},
		{"values of a parse value", head + "const fixture = JSON.parse('{}');\nconst all = Object.values(fixture);\ntest('plain', async () => {});\n", stop + "3: `JSON` can build or enumerate a property at run time)"},
		{"values of an object that reaches a test", head + "const details = { annotation: { type: 'a' } };\nconst all = Object.values(details);\ntest('plain', details, async () => {});\n", stop + "4: `Object` can build or enumerate a property at run time)"},
		{"values of a spread object", head + "const base = {};\nconst counts = { ...base };\nconst all = Object.values(counts);\ntest('plain', async () => {});\n", stop + "5: `Object` can build or enumerate a property at run time)"},
		{"counter reassigned", loop("    i = rows.length;\n    const v = rows[i];\n"), stop + "7: a computed member access can reach a tag array without naming it)"},
		{"counter grows by a value", loop("    i += rows.length;\n    const v = rows[i];\n"), stop + "7: a computed member access can reach a tag array without naming it)"},
		{"counter shadowed by a parameter", loop("    rows.forEach((i) => rows[i]);\n"), stop + "6: a computed member access can reach a tag array without naming it)"},
		{"counter read outside its loop", head + "const rows = ['a'];\nlet i = 0;\nconst v = rows[i];\ntest('plain', async () => {});\n", stop + "5: a computed member access can reach a tag array without naming it)"},
		{"counter as a computed write", loop("    rows[i] = 'b';\n"), stop + "6: a computed member write can set a tag)"},
		{"url parse query reaches a test", "import url from 'url';\nconst s = process.argv[2];\ntest('plain', url.parse(s, true).query, async () => {});\n", stop + "3: `parse` builds an object from text, so a `tag` key can exist without its name)"},
		{"dotenv parse reaches a test", "import dotenv from 'dotenv';\nconst s = process.argv[2];\ntest('plain', dotenv.parse(s), async () => {});\n", stop + "3: `parse` builds an object from text, so a `tag` key can exist without its name)"},
		{"dotenv populates a details object", "import dotenv from 'dotenv';\nconst details = {};\ndotenv.config({ processEnv: details });\ntest('plain', details, async () => {});\n", stop + "3: `processEnv` builds an object from text, so a `tag` key can exist without its name)"},
	} {
		t.Run(row.label, func(t *testing.T) {
			root := grepFixture(t, grepRepro)
			write(t, root, "e2e/plain.spec.ts", row.plain)
			unverified, omissions := gh1647Omission(t, root, grepReproCases())
			if row.stop == "" {
				if len(unverified) != 0 || len(omissions) != 0 {
					t.Fatalf("not verified: unverified=%+v omissions=%+v", unverified, omissions)
				}
				return
			}
			if len(omissions) != 1 || !strings.Contains(omissions[0].Detail, row.stop) {
				t.Fatalf("omissions=%+v, want a stop naming %q", omissions, row.stop)
			}
		})
	}
}
