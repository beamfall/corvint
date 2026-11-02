// SPDX-License-Identifier: Apache-2.0
package typescript

import (
	"strconv"
	"strings"
	"testing"
)

// tjaa725Stop is the per-file stop prefix of e2e/plain.spec.ts and its helpers.
func tjaa725Stop(file string, line int, why string) string {
	return file + " holds a tag or construct that is not one literal @\\S+ string (line " + strconv.Itoa(line) + ": " + why + ")"
}

const tjaa725Reflective = "can build or enumerate a property at run time"

// tjaa725Run writes plain.spec.ts (and any helper files) into the GitHub #1647 grep repro and
// returns the discovery omission stops of its `plain` test.
func tjaa725Run(t *testing.T, plain string, helpers map[string]string) ([]PlaywrightDiscoveryUnit, []PlaywrightUnknown) {
	t.Helper()
	root := grepFixture(t, grepRepro)
	write(t, root, "e2e/plain.spec.ts", plain)
	for name, body := range helpers {
		write(t, root, name, body)
	}
	return gh1647Omission(t, root, grepReproCases())
}

// TJAA-V0-725 (V1-2583; GitHub #1701): a reflective name inside the block body of a test or hook
// whose callee is the `test` binding a named import from @playwright/test proves leaves the tag
// closure exact, since the runner collects tests without running those bodies.
func TestPlaywrightTagClosureDefersProvenTestBody_TJAA725(t *testing.T) {
	for _, row := range []struct {
		label, plain string
		helpers      map[string]string
	}{
		{"proven test body", "import { test, expect } from '@playwright/test';\ntest('plain', async ({ page }) => {\n  const keys = Object.keys({ a: 1 });\n  expect(keys).toEqual(['a']);\n});\n", nil},
		{"aliased setup binding", "import { test as setup } from '@playwright/test';\nsetup('plain', async () => {\n  Reflect.ownKeys({});\n});\n", nil},
		{"function literal body", "import { type Page, test } from '@playwright/test';\ntest('plain', async function ({ page }: { page: Page }) {\n  JSON.parse('{}');\n});\n", nil},
		{"hook and modifier bodies", "import { test } from '@playwright/test';\ntest.beforeEach(async () => {\n  Object.entries({});\n});\ntest.afterAll(() => {\n  globalThis.done = true;\n});\ntest('plain', async () => {\n  const copy = structuredClone({ a: 1 });\n});\n", nil},
		{"fixture module beside the proven binding", "import { test } from '@playwright/test';\nimport { helper } from './helper';\ntest('plain', async () => {\n  Object.values(helper());\n});\n", map[string]string{"e2e/helper.ts": "import { test as base } from '@playwright/test';\nexport const extended = base.extend({});\nexport function helper() {\n  return { a: 1 };\n}\n"}},
	} {
		t.Run(row.label, func(t *testing.T) {
			unverified, omissions := tjaa725Run(t, row.plain, row.helpers)
			if len(unverified) != 0 || len(omissions) != 0 {
				t.Fatalf("not verified: unverified=%+v omissions=%+v", unverified, omissions)
			}
		})
	}
}

// TJAA-V0-725 (V1-2583): without import provenance, or when a file of the closure can rewrite,
// rebind or leak the `test` binding, the same body still stops and names its line.
func TestPlaywrightTagClosureBodyGuardStops_TJAA725(t *testing.T) {
	const plain = "e2e/plain.spec.ts"
	body := "test('plain', async () => {\n  Object.keys({});\n});\n"
	guard := "`test` is rewritten, rebound or used other than as a direct call, so a test body the closure defers can run while tests are collected"
	for _, row := range []struct {
		label, plain string
		helpers      map[string]string
		want         string
	}{
		{"no import is a name match", body, nil, tjaa725Stop(plain, 2, "`Object` "+tjaa725Reflective)},
		{"test from another module", "import { test } from './fixtures';\n" + body, map[string]string{"e2e/fixtures.ts": "import { test as base } from '@playwright/test';\nexport const test = base.extend({});\n"}, tjaa725Stop(plain, 3, "`Object` "+tjaa725Reflective)},
		{"type-only import", "import type { test } from '@playwright/test';\n" + body, nil, tjaa725Stop(plain, 3, "`Object` "+tjaa725Reflective)},
		{"namespace import", "import * as pw from '@playwright/test';\nconst test = pw.test;\n" + body, nil, tjaa725Stop(plain, 4, "`Object` "+tjaa725Reflective)},
		{"describe callback runs at collection", "import { test } from '@playwright/test';\ntest.describe('group', () => {\n  Object.keys({});\n});\n" + "test('plain', async () => {});\n", nil, tjaa725Stop(plain, 3, "`Object` "+tjaa725Reflective)},
		{"member reassigned", "import { test } from '@playwright/test';\ntest.only = (title: string, run: () => void) => run();\n" + body, nil, tjaa725Stop(plain, 4, "`Object` "+tjaa725Reflective)},
		{"binding shadowed by a parameter", "import { test } from '@playwright/test';\nfunction wrap(test: unknown) {\n  return test;\n}\n" + body, nil, tjaa725Stop(plain, 6, "`Object` "+tjaa725Reflective)},
		{"binding escapes as a value", "import { test } from '@playwright/test';\nimport { patch } from './helper';\npatch(test);\n" + body, map[string]string{"e2e/helper.ts": "export function patch(value: unknown) {}\n"}, tjaa725Stop(plain, 5, "`Object` "+tjaa725Reflective)},
		{"body assigned before it is the argument", "import { test } from '@playwright/test';\nlet run: () => void;\ntest('plain', run = () => {\n  Object.keys({});\n});\nrun();\n", nil, tjaa725Stop(plain, 4, "`Object` "+tjaa725Reflective)},
		{"helper rewrites a test member", "import { test } from '@playwright/test';\nimport { noop } from './helper';\nnoop();\n" + body, map[string]string{"e2e/helper.ts": "import { test } from '@playwright/test';\ntest.only = (title: string, run: () => void) => run();\nexport function noop() {}\n"}, tjaa725Stop("e2e/helper.ts", 2, guard)},
		{"helper re-exports the binding", "import { test } from '@playwright/test';\nimport { noop } from './helper';\nnoop();\n" + body, map[string]string{"e2e/helper.ts": "import { test as base } from '@playwright/test';\nexport function noop() {}\nexport { base };\n"}, tjaa725Stop("e2e/helper.ts", 3, "`base` is rewritten, rebound or used other than as a direct call, so a test body the closure defers can run while tests are collected")},
	} {
		t.Run(row.label, func(t *testing.T) {
			_, omissions := tjaa725Run(t, row.plain, row.helpers)
			if len(omissions) != 1 || !strings.Contains(omissions[0].Detail, row.want) {
				t.Fatalf("omissions=%+v, want a stop naming %q", omissions, row.want)
			}
		})
	}
}
