// SPDX-License-Identifier: Apache-2.0

package typescript

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AFP-V0-2245 (V1-4173): a TypeScript module's anchored token ("/repo") names a dirty path as a
// Go unit's does under AFP-V0-2203, from the bases a runner may join it to: the repository root,
// the module's directory and its ancestors, and each directory the plugin records a runner may
// start in. Here web/vitest.config.mjs has an unmodelled `include:` that may own ../tests, so
// web is a run directory of tests/e2e. A fake absolute value no longer names a fixture
// repository's testdata/repo. The controls keep every reader the narrowing must not drop:
// `process.cwd()`, `path.resolve()` and a relative join, each rooted at web, and a renamed
// destructured `cwd`, `process.env.PWD` and `process.env['INIT_CWD']`, which refuse the
// certificate (`process` may appear only in the forms unrecordedDirectory allows).
func TestTypeScriptAnchoredTokensResolveFromRunDirectories_AFPV02245(t *testing.T) {
	root := t.TempDir()
	const head = "import { test } from 'node:test'\nimport { readFileSync } from 'node:fs'\nimport { resolve } from 'node:path'\n"
	write(t, root, "package.json", `{"name":"fixture","type":"module"}`)
	write(t, root, "web/vitest.config.mjs", "export default { test: { include: ['../tests/**'] } }\n")
	modules := map[string]string{
		"tests/e2e/fake.test.mjs":     head + "test('a', () => run('/repo', { kind: 'x' }))\n",
		"tests/e2e/key.test.mjs":      head + "test('a', () => run({ cwd: '/repo', mode: 'print' }))\n",
		"tests/e2e/cwd.test.mjs":      head + "test('a', () => readFileSync(process.cwd() + '/repo/lib.js'))\n",
		"tests/e2e/rename.test.mjs":   head + "const { cwd: here } = process\ntest('a', () => readFileSync(here() + '/repo/lib.js'))\n",
		"tests/e2e/resolve.test.mjs":  head + "test('a', () => readFileSync(resolve() + '/repo/lib.js'))\n",
		"tests/e2e/relative.test.mjs": head + "test('a', () => readFileSync('fixtures' + '/repo/lib.js'))\n",
		"tests/e2e/pwd.test.mjs":      head + "test('a', () => readFileSync(process.env.PWD + '/repo/lib.js'))\n",
		"tests/e2e/initcwd.test.mjs":  head + "test('a', () => readFileSync(process.env['INIT_CWD'] + '/repo/lib.js'))\n",
	}
	for name, body := range modules {
		write(t, root, name, body)
	}
	for _, name := range []string{"internal/x/testdata/repo/lib.js", "web/repo/lib.js", "web/fixtures/repo/lib.js"} {
		write(t, root, name, "export const x = 1\n")
	}
	graph, err := affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatal(err)
	}
	readers := func(dirty string) []string {
		var got []string
		for _, selection := range affected.Select(graph, []string{dirty}).Selected {
			if selection.Witness.Kind == affected.WitnessPathLiteralReader {
				got = append(got, selection.Tests...)
			}
		}
		slices.Sort(got)
		return got
	}
	e2e := func(names ...string) []string {
		for i, name := range names {
			names[i] = "tests/e2e/" + name + ".test.mjs"
		}
		return names
	}
	for _, tc := range []struct {
		dirty string
		want  []string
	}{
		// Narrowed: no base or placed component puts "/repo" in a fixture repository.
		{"internal/x/testdata/repo/lib.js", e2e("initcwd", "pwd", "rename")},
		// Controls: every module that can root the token at web.
		{"web/repo/lib.js", e2e("cwd", "fake", "initcwd", "key", "pwd", "relative", "rename", "resolve")},
		// Control: the relative join's own word places fixtures/ below web.
		{"web/fixtures/repo/lib.js", e2e("initcwd", "pwd", "relative", "rename")},
	} {
		if got := readers(tc.dirty); !slices.Equal(got, tc.want) {
			t.Errorf("%s: path-literal readers %v, want %v", tc.dirty, got, tc.want)
		}
	}
	certified := map[string]bool{"fake": true, "key": true, "cwd": true, "rename": false, "resolve": true, "relative": true, "pwd": false, "initcwd": false}
	for name, want := range certified {
		for _, id := range graph.UnitIDs() {
			unit, _ := graph.Unit(id)
			if len(unit.Tests) != 1 || unit.Tests[0] != "tests/e2e/"+name+".test.mjs" {
				continue
			}
			if unit.AnchorsPlaced != want || want && !slices.Equal(unit.RunDirectories, []string{"web"}) {
				t.Errorf("%s: AnchorsPlaced = %v, run directories %v, want %v and [web]", name, unit.AnchorsPlaced, unit.RunDirectories, want)
			}
		}
	}
}

// AFP-V0-2245: a manifest script that starts a runner in another directory (`cd web && ...`)
// leaves the repository's runner directories unrecorded, so no module is certified; nor is one
// that changes directory itself or resolves a module path.
func TestTypeScriptAnchorCertificateRefusals_AFPV02245(t *testing.T) {
	const head = "import { test } from 'node:test'\n"
	for _, tc := range []struct {
		name, manifest, body string
	}{
		{"manifest cd", `{"name":"f","type":"module","scripts":{"test":"cd web && node --test ../tests"}}`, head + "test('a', () => run('/repo'))\n"},
		{"manifest prefix", `{"name":"f","type":"module","scripts":{"test":"npm --prefix web test"}}`, head + "test('a', () => run('/repo'))\n"},
		{"chdir", `{"name":"f","type":"module"}`, head + "process.chdir('web')\ntest('a', () => run('/repo'))\n"},
		{"argv", `{"name":"f","type":"module"}`, head + "test('a', () => run(process.argv[1] + '/repo'))\n"},
		{"require.resolve", `{"name":"f","type":"module"}`, head + "test('a', () => run(require.resolve('pkg') + '/repo'))\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "package.json", tc.manifest)
			write(t, root, "tests/a.test.mjs", tc.body)
			graph, err := affected.Build(context.Background(), root, New())
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, id := range graph.UnitIDs() {
				if unit, _ := graph.Unit(id); len(unit.Tests) == 1 && unit.Tests[0] == "tests/a.test.mjs" {
					found = true
					if unit.AnchorsPlaced {
						t.Errorf("certified despite %s", tc.name)
					}
				}
			}
			if !found {
				t.Fatal("no unit for tests/a.test.mjs")
			}
		})
	}
}

// AFP-V0-2245: the ancestors of a run directory are bases too, since a module may climb from it
// (`..`, `dirname`, `path.parse(...).dir`), so every module run from app/web still reads app/repo.
// `argv` as an object key and a module-relative `require.resolve('./x')` keep the certificate.
func TestTypeScriptClimbingModulesReachRunDirectoryAncestors_AFPV02245(t *testing.T) {
	root := t.TempDir()
	const head = "import { test } from 'node:test'\nimport { readFileSync } from 'node:fs'\nimport { dirname } from 'node:path'\n"
	write(t, root, "package.json", `{"name":"fixture","type":"module"}`)
	write(t, root, "app/web/vitest.config.mjs", "export default { test: { include: ['../../tests/**'] } }\n")
	write(t, root, "tests/dirname.test.mjs", head+"test('a', () => readFileSync(dirname(process.cwd()) + '/repo/lib.js'))\n")
	write(t, root, "tests/dots.test.mjs", "import { test } from 'node:test'\ntest('a', () => read(process.cwd() + '/' + '..' + '/repo/lib.js'))\n")
	write(t, root, "tests/flat.test.mjs", "import { test } from 'node:test'\ntest('a', () => read(process.cwd() + '/repo/lib.js', { argv: ['x'] }))\n")
	write(t, root, "tests/local.test.cjs", "const { test } = require('node:test')\ntest('a', () => read(`${require.resolve('./flat.test.mjs')}` + '/repo/lib.js'))\n")
	// A config whose membership names only literals below its own directory owns no module
	// outside it, so it is no run directory (localMembership); one that imports a relative
	// module may, so it is.
	write(t, root, "fixture/repo/vitest.config.mjs", "export default { test: { globals: true, include: [\"test/a.test.js\"] } }\n")
	write(t, root, "other/vitest.config.mjs", "import shared from './shared.mjs'\nexport default { test: { include: shared } }\n")
	write(t, root, "app/repo/lib.js", "export const x = 1\n")
	write(t, root, "app/web/repo/lib.js", "export const y = 1\n")
	graph, err := affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatal(err)
	}
	readers := func(dirty string) []string {
		var got []string
		for _, selection := range affected.Select(graph, []string{dirty}).Selected {
			if selection.Witness.Kind == affected.WitnessPathLiteralReader {
				got = append(got, selection.Tests...)
			}
		}
		slices.Sort(got)
		return got
	}
	for dirty, want := range map[string][]string{
		"app/repo/lib.js":     {"tests/dirname.test.mjs", "tests/dots.test.mjs", "tests/flat.test.mjs", "tests/local.test.cjs"},
		"app/web/repo/lib.js": {"tests/dirname.test.mjs", "tests/dots.test.mjs", "tests/flat.test.mjs", "tests/local.test.cjs"},
	} {
		if got := readers(dirty); !slices.Equal(got, want) {
			t.Errorf("%s: path-literal readers %v, want %v", dirty, got, want)
		}
	}
	for _, id := range graph.UnitIDs() {
		unit, _ := graph.Unit(id)
		if len(unit.Tests) == 1 && strings.HasPrefix(unit.Tests[0], "tests/") && !unit.AnchorsPlaced {
			t.Errorf("%s is not certified", unit.Tests[0])
		}
		want := []string{"app", "app/web", "other"}
		if len(unit.Tests) == 1 && strings.HasPrefix(unit.Tests[0], "tests/") && !slices.Equal(unit.RunDirectories, want) {
			t.Errorf("%s: run directories %v, want %v", unit.Tests[0], unit.RunDirectories, want)
		}
	}
}

// AFP-V0-2245, the review's escapes: each module joins "/repo" to a directory the plugin's
// bases would miss, so it must stay a reader of app/x/repo/lib.js. A value imported from a local
// module joins the closure's words and directories; a manifest-embedded config, a script with a
// parent component, `--rootDir` or `-c`, a renamed `argv` destructure, a computed `process`
// member, an aliased `require` and a computed environment key refuse the certificate.
func TestTypeScriptAnchorEscapesStayReaders_AFPV02245(t *testing.T) {
	const head = "import { test } from 'node:test'\nimport { readFileSync } from 'node:fs'\n"
	for _, tc := range []struct {
		name, manifest string
		files          map[string]string
	}{
		{"imported constant", `{"name":"f","type":"module"}`, map[string]string{
			"tests/dirs.mjs":   "export const APP = 'app/x'\n",
			"tests/a.test.mjs": head + "import { APP } from './dirs.mjs'\ntest('a', () => readFileSync(`${APP}/repo/lib.js`))\n"}},
		{"imported json", `{"name":"f","type":"module"}`, map[string]string{
			"tests/dirs.json":  "{\"app\": \"app/x\"}\n",
			"tests/a.test.mjs": head + "import dirs from './dirs.json' with { type: 'json' }\ntest('a', () => readFileSync(dirs.app + '/repo/lib.js'))\n"}},
		{"embedded jest roots", `{"name":"f","type":"module","jest":{"roots":["<rootDir>/../tests"]}}`, map[string]string{
			"tests/a.test.mjs": head + "test('a', () => readFileSync(process.cwd() + '/repo/lib.js'))\n"}},
		{"script rootDir", `{"name":"f","type":"module","scripts":{"test":"jest --rootDir app/x"}}`, map[string]string{
			"tests/a.test.mjs": head + "test('a', () => readFileSync(process.cwd() + '/repo/lib.js'))\n"}},
		{"script config", `{"name":"f","type":"module","scripts":{"e2e":"playwright test -c app/x/pw.config.ts"}}`, map[string]string{
			"tests/a.test.mjs": head + "test('a', () => readFileSync(process.cwd() + '/repo/lib.js'))\n"}},
		{"script parent", `{"name":"f","type":"module","scripts":{"test":"node --test ../tests"}}`, map[string]string{
			"tests/a.test.mjs": head + "test('a', () => readFileSync(process.cwd() + '/repo/lib.js'))\n"}},
		{"renamed argv", `{"name":"f","type":"module"}`, map[string]string{
			"tests/a.test.mjs": head + "const { argv: args } = process\ntest('a', () => readFileSync(args[2] + '/repo/lib.js'))\n"}},
		{"computed process member", `{"name":"f","type":"module"}`, map[string]string{
			"tests/a.test.mjs": head + "process['chdir']('app/x')\ntest('a', () => readFileSync(process.cwd() + '/repo/lib.js'))\n"}},
		{"aliased require", `{"name":"f","type":"module"}`, map[string]string{
			"tests/a.test.cjs": "const { test } = require('node:test')\nconst r = require\ntest('a', () => read(r.resolve('pkg') + '/repo/lib.js'))\n"}},
		{"computed env key", `{"name":"f","type":"module"}`, map[string]string{
			"tests/a.test.mjs": head + "test('a', () => readFileSync(process.env['P' + 'WD'] + '/repo/lib.js'))\n"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "package.json", tc.manifest)
			write(t, root, "app/x/repo/lib.js", "export const x = 1\n")
			test := ""
			for name, body := range tc.files {
				write(t, root, name, body)
				if strings.Contains(name, ".test.") {
					test = name
				}
			}
			graph, err := affected.Build(context.Background(), root, New())
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, selection := range affected.Select(graph, []string{"app/x/repo/lib.js"}).Selected {
				found = found || slices.Contains(selection.Tests, test)
			}
			if !found {
				t.Errorf("%s is no reader of app/x/repo/lib.js", test)
			}
		})
	}
}

// AFP-V0-2245: a config whose membership is built by concatenation, an escaped parent component,
// an imported value or a rootDir may own modules outside its directory, so it is a run directory;
// one naming only plain local literals is not.
func TestLocalMembershipRefusesComputedConfigs_AFPV02245(t *testing.T) {
	for body, local := range map[string]bool{
		"export default { test: { include: ['test/a.test.js'] } }\n":                                                   true,
		"import { defineConfig } from 'vitest/config'\nexport default defineConfig({ test: { include: ['t/**'] } })\n": true,
		"export default { test: { include: ['.' + './tests/**'] } }\n":                                                 false,
		"export default { test: { include: ['\\x2e\\x2e/tests/**'] } }\n":                                              false,
		"import shared from '@acme/shared'\nexport default { test: { include: shared } }\n":                            false,
		"import { dir } from '@acme/shared'\nexport default { rootDir: dir }\n":                                        false,
		"export default { test: { include: [`${base}/x`] } }\n":                                                        false,
	} {
		if got := localMembership(body); got != local {
			t.Errorf("localMembership(%q) = %v, want %v", body, got, local)
		}
	}
}
