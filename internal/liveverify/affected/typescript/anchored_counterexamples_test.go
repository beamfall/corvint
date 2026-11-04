// SPDX-License-Identifier: Apache-2.0

package typescript

import (
	"context"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AFP-V0-2245 (withdrawn): the counterexamples of the re-review of 2026-11-03 (its list has no
// case 10). Each TypeScript test joins an anchored "/repo" token, or a glob, to a directory taken
// from data, the filesystem or a launcher the plugin cannot see, so it can read app/x/repo/lib.js
// (cases 2, 6 and 12 show the shape; their fixtures resolve the directory elsewhere) and must stay
// selected for it.
// Every case passes with the component-run relation TypeScript keeps (AFP-V0-072 (3)); a future
// anchored narrowing for TypeScript must keep passing them.
func TestTypeScriptAnchoredCounterexamplesStaySelected_AFPV02245(t *testing.T) {
	const head = "import { test } from 'node:test'\nimport { readFileSync, readdirSync, globSync } from 'node:fs'\n"
	const module = `{"name":"f","type":"module"}`
	for _, tc := range []struct {
		name  string
		files map[string]string
	}{
		{"1 glob", map[string]string{
			"package.json":     module,
			"tests/a.test.mjs": head + "test('a', () => globSync('**/repo/*'))\n"}},
		{"2 directory listing", map[string]string{
			"package.json":     module,
			"tests/a.test.mjs": head + "test('a', () => readFileSync(readdirSync(process.cwd())[0] + '/repo/lib.js'))\n"}},
		{"3 environment variable", map[string]string{
			"package.json":     `{"name":"f","type":"module","scripts":{"test":"FIXTURES=app/x node --test"}}`,
			"tests/a.test.mjs": head + "test('a', () => readFileSync(process.env.FIXTURES + '/repo/lib.js'))\n"}},
		{"4 parsed JSON", map[string]string{
			"package.json":     module,
			"tests/dirs.json":  `{"app": "app/x"}`,
			"tests/a.test.mjs": head + "const dirs = JSON.parse(readFileSync(new URL('./dirs.json', import.meta.url)))\ntest('a', () => readFileSync(dirs.app + '/repo/lib.js'))\n"}},
		{"5 shell launcher", map[string]string{
			"package.json":        module,
			"script/run-tests.sh": "#!/bin/sh\ncd app/x && node --test ../../tests\n",
			"tests/a.test.mjs":    head + "test('a', () => readFileSync(process.cwd() + '/repo/lib.js'))\n"}},
		{"5 Makefile launcher", map[string]string{
			"package.json":     module,
			"Makefile":         "test:\n\tcd app/x && node --test ../../tests\n",
			"tests/a.test.mjs": head + "test('a', () => readFileSync(process.cwd() + '/repo/lib.js'))\n"}},
		{"5 CI working directory", map[string]string{
			"package.json":             module,
			".github/workflows/ci.yml": "jobs:\n  t:\n    steps:\n      - run: node --test ../../tests\n        working-directory: app/x\n",
			"tests/a.test.mjs":         head + "test('a', () => readFileSync(process.cwd() + '/repo/lib.js'))\n"}},
		{"5 nx target", map[string]string{
			"package.json":     module,
			"project.json":     `{"targets":{"test":{"executor":"nx:run-commands","options":{"cwd":"app/x","command":"node --test ../../tests"}}}}`,
			"tests/a.test.mjs": head + "test('a', () => readFileSync(process.cwd() + '/repo/lib.js'))\n"}},
		{"6 mocharc in YAML", map[string]string{
			"package.json":       `{"name":"f","type":"module","devDependencies":{"mocha":"10"}}`,
			"app/x/.mocharc.yml": "spec: '../../tests/**'\n",
			"tests/a.test.mjs":   "import { readFileSync } from 'node:fs'\ndescribe('a', () => it('b', () => readFileSync(process.cwd() + '/repo/lib.js')))\n"}},
		{"7 setup file changes directory", map[string]string{
			"package.json":      `{"name":"f","type":"module","devDependencies":{"vitest":"1"}}`,
			"vitest.config.mjs": "export default { test: { setupFiles: ['./setup.mjs'] } }\n",
			"setup.mjs":         "process.chdir('app/x')\n",
			"tests/a.test.mjs":  "import { test } from 'vitest'\nimport { readFileSync } from 'node:fs'\ntest('a', () => readFileSync(process.cwd() + '/repo/lib.js'))\n"}},
		{"7 mocha require changes directory", map[string]string{
			"package.json":     `{"name":"f","type":"module","devDependencies":{"mocha":"10"},"scripts":{"test":"mocha --require ./chdir.cjs"}}`,
			"chdir.cjs":        "process.chdir('app/x')\n",
			"tests/a.test.mjs": "import { readFileSync } from 'node:fs'\ndescribe('a', () => it('b', () => readFileSync(process.cwd() + '/repo/lib.js')))\n"}},
		{"8 dynamic import of a template", map[string]string{
			"package.json":     module,
			"tests/dirs.mjs":   "export const APP = 'app/x'\n",
			"tests/a.test.mjs": head + "const { APP } = await import(`./dirs.mjs`)\ntest('a', () => readFileSync(APP + '/repo/lib.js'))\n"}},
		{"8 package imports", map[string]string{
			"package.json":     `{"name":"f","type":"module","imports":{"#dirs":"./tests/dirs.mjs"}}`,
			"tests/dirs.mjs":   "export const APP = 'app/x'\n",
			"tests/a.test.mjs": head + "import { APP } from '#dirs'\ntest('a', () => readFileSync(APP + '/repo/lib.js'))\n"}},
		{"9 identifiers as components", map[string]string{
			"package.json":     module,
			"tests/a.test.mjs": head + "test('a', () => readFileSync(Object.keys({ app: 1, x: 2 }).join('/') + '/repo/lib.js'))\n"}},
		{"11 case transform", map[string]string{
			"package.json":     module,
			"tests/a.test.mjs": head + "test('a', () => readFileSync('APP/X'.toLowerCase() + '/repo/lib.js'))\n"}},
		{"12 parent module file", map[string]string{
			"package.json":     `{"name":"f"}`,
			"tests/a.test.cjs": "const { test } = require('node:test')\nconst { readFileSync } = require('node:fs')\ntest('a', () => readFileSync(module.parent.filename + '/../repo/lib.js'))\n"}},
		{"12 stack trace", map[string]string{
			"package.json":     module,
			"tests/a.test.mjs": head + "test('a', () => readFileSync(new Error().stack.split('(')[1].split(':')[0] + '/../repo/lib.js'))\n"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			test := ""
			for name, body := range tc.files {
				write(t, root, name, body)
				if name == "tests/a.test.mjs" || name == "tests/a.test.cjs" {
					test = name
				}
			}
			write(t, root, "app/x/repo/lib.js", "export const x = 1\n")
			graph, err := affected.Build(context.Background(), root, New())
			if err != nil {
				t.Fatal(err)
			}
			selected := false
			for _, selection := range affected.Select(graph, []string{"app/x/repo/lib.js"}).Selected {
				selected = selected || slices.Contains(selection.Tests, test)
			}
			if !selected {
				t.Errorf("%s is not selected for app/x/repo/lib.js", test)
			}
		})
	}
}
