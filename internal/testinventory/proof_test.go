// SPDX-License-Identifier: Apache-2.0
package testinventory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
)

// namedByFixture reports whether the fixture lister names file: it holds a
// test (a `test(` call, a `def test_` function or a doctest prompt).
func namedByFixture(content string) bool {
	return strings.Contains(content, "test(") || strings.Contains(content, "def test_") || strings.Contains(content, ">>>")
}

// listedInventory is the inventory a refresh of tree writes with the
// fixture lister: each instance binds the files the lister names and, when
// proven (Pattern empty), every default candidate too (TIV-V1-005).
func listedInventory(t *testing.T, tree MapTree) string {
	t.Helper()
	files, _ := tree.Files()
	declared := Detect(files, tree.Read)
	inventory := Inventory{Profile: Profile, Instances: []Instance{}, Failures: []Failure{}}
	for _, instance := range declared {
		identity, _ := CurrentLister(instance.Runner)
		record := Instance{
			Key: instance.Key, Runner: instance.Runner, Root: instance.Root, Config: instance.Config,
			ConfigDigest: files[instance.Config], Lockfile: instance.Lockfile, LockfileDigest: files[instance.Lockfile],
			Lister: Lister{Name: identity.Name, Version: identity.Version, Kind: identity.Kind, Command: []string{"runner", "list"}},
			Commit: fixtureCommit, Files: []File{}, Tests: []Test{},
		}
		if instance.Lockfile == "" {
			record.LockfileDigest = ""
		}
		var bound []string
		if instance.Pattern == "" {
			bound = Candidates(files, instance, declared)
		}
		for name, content := range tree {
			if name != Path && under(instance.Root, name) && namedByFixture(content) && !slices.Contains(bound, name) {
				bound = append(bound, name)
			}
		}
		slices.Sort(bound)
		for _, name := range bound {
			record.Files = append(record.Files, File{Path: name, Blob: files[name]})
			record.Tests = append(record.Tests, Test{ID: instance.Runner + ":" + name, File: name, Reason: "test-id-unmapped"})
		}
		inventory.Instances = append(inventory.Instances, record)
	}
	data, err := Encode(inventory)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TIV-V1-005, TIV-V1-008: an executing instance counts as default-pattern
// only when the reader proves it, so each config here, which may move test
// files off the default names in a way the old whole-word scan of the
// config's bytes missed, is unproven: a new file of the runner's languages
// under its root that the record does not bind is unassigned, with a cause
// naming why. The first five cases are the custom patterns the old scan
// caught; every later one fails on 45152cea1.
func TestTIVV1008UnprovenConfigFailsClosed(t *testing.T) {
	assertUnproven(t, []unprovenCase{
		{"vitest include", "vitest@.", "src/b.check.ts", []string{"sets include"}, MapTree{
			"vitest.config.ts": "export default { test: { include: ['src/**/*.check.ts'] } }\n",
			"src/a.check.ts":   "test('a', () => {})\n", "src/lib.ts": "export const x = 1\n",
		}},
		{"vitest relative import", "vitest@web", "web/src/b.check.ts", []string{"imports ./vitest.shared"}, MapTree{
			"web/vitest.config.ts": "import shared from './vitest.shared'\nexport default shared\n",
			"web/vitest.shared.ts": "export default { test: { include: ['src/**/*.check.ts'] } }\n",
			"web/src/a.check.ts":   "test('a', () => {})\n",
		}},
		{"jest testMatch", "jest@.", "src/b.check.js", []string{"sets testMatch"}, MapTree{
			"jest.config.js": "module.exports = { testMatch: ['**/*.check.js'] }\n",
			"src/a.check.js": "test('a', () => {})\n", "src/lib.js": "module.exports = 1\n",
		}},
		{"playwright testMatch", "playwright@.", "e2e/b.e2e.ts", []string{"sets testMatch"}, MapTree{
			"playwright.config.ts": "export default defineConfig({ projects: [{ name: 'p', testMatch: /.*\\.e2e\\.ts/ }] })\n",
			"e2e/a.e2e.ts":         "test('a', async () => {})\n",
		}},
		{"pytest python_files", "pytest@.", "tests/check_b.py", []string{"sets python_files"}, MapTree{
			"pytest.ini":         "[pytest]\npython_files = check_*.py\n",
			"tests/check_a.py":   "def test_a(): pass\n",
			"tests/conftest.py":  "\n",
			"pkg/module.py":      "x = 1\n",
			"tests/test_dflt.py": "def test_d(): pass\n",
		}},
		{"vitest workspace file setting include", "vitest@.", "src/b.check.ts", []string{"workspace file vitest.workspace.ts"}, MapTree{
			"vitest.config.ts":    "export default {}\n",
			"vitest.workspace.ts": "export default [{ test: { include: ['src/**/*.check.ts'] } }]\n",
			"src/a.check.ts":      "test('a', () => {})\n",
		}},
		{"mergeConfig from a package import", "vitest@.", "src/b.check.ts", []string{"imports @acme/vitest-preset", "calls mergeConfig"}, MapTree{
			"vitest.config.ts": "import { mergeConfig } from 'vitest/config'\nimport preset from '@acme/vitest-preset'\nexport default mergeConfig(preset, {})\n",
			"src/a.check.ts":   "test('a', () => {})\n",
		}},
		{"includeSource", "vitest@.", "src/sum.ts", []string{"sets includeSource"}, MapTree{
			"vitest.config.ts": "import { defineConfig } from 'vitest/config'\nexport default defineConfig({ test: { includeSource: ['src/**/*.ts'] } })\n",
			"src/a.test.ts":    "test('a', () => {})\n",
		}},
		{"jest preset", "jest@.", "src/b.check.js", []string{"sets preset"}, MapTree{
			"jest.config.js": "module.exports = { preset: '@acme/jest-preset' }\n",
			"src/a.check.js": "test('a', () => {})\n",
		}},
		{"package.json jest key with no config file", "jest@.", "src/b.check.js", []string{"package.json jest member"}, MapTree{
			"package.json":   `{"name": "x", "jest": {"testMatch": ["**/*.check.js"]}}`,
			"src/a.check.js": "test('a', () => {})\n",
		}},
		{"package.json jest key beside a config", "jest@.", "src/b.check.js", []string{"package.json sets the jest member"}, MapTree{
			"jest.config.js": "module.exports = {}\n",
			"package.json":   `{"jest": {}}`,
			"src/a.check.js": "test('a', () => {})\n",
		}},
		{"package.json script running vitest", "vitest@.", "src/b.check.ts", []string{"package.json script that runs vitest"}, MapTree{
			"package.json":   `{"scripts": {"test": "vitest run"}}`,
			"vite.config.ts": "export default { test: { include: ['src/**/*.check.ts'] } }\n",
			"src/a.check.ts": "test('a', () => {})\n",
		}},
		{"pytest addopts --doctest-glob with a new .md file", "pytest@.", "docs/guide.md", []string{"addopts --doctest-glob=*.md"}, MapTree{
			"pytest.ini":        "[pytest]\naddopts = -q --doctest-glob=*.md\n",
			"tests/test_a.py":   "def test_a(): pass\n",
			"docs/intro.md":     ">>> 1 + 1\n2\n",
			"tests/conftest.py": "\n",
		}},
		{"pytest addopts --doctest-modules", "pytest@.", "pkg/calc.py", []string{"addopts --doctest-modules"}, MapTree{
			"pyproject.toml":  "[tool.pytest.ini_options]\naddopts = [\n  \"-ra\",\n  \"--doctest-modules\",\n]\n",
			"tests/test_a.py": "def test_a(): pass\n",
		}},
		{"pytest conftest collection hook", "pytest@.", "tests/cases.yaml", []string{"tests/conftest.py defines pytest_collect_file"}, MapTree{
			"pytest.ini":        "[pytest]\n",
			"tests/test_a.py":   "def test_a(): pass\n",
			"tests/conftest.py": "def pytest_collect_file(parent, file_path):\n    return None\n",
		}},
		{"skipped-directory include", "vitest@.", ".tests/b.ts", []string{"sets include"}, MapTree{
			"vitest.config.ts": "import { defineConfig } from 'vitest/config'\nexport default defineConfig({ test: { include: ['.tests/**/*.ts'] } })\n",
			".tests/a.ts":      "test('a', () => {})\n",
		}},
		{"computed key", "vitest@.", "src/b.check.ts", []string{"a computed key"}, MapTree{
			"vitest.config.ts": "const key = 'incl' + 'ude'\nexport default { test: { [key]: ['src/**/*.check.ts'] } }\n",
			"src/a.check.ts":   "test('a', () => {})\n",
		}},
		{"escaped member name", "vitest@.", "src/b.check.ts", []string{"an escape"}, MapTree{
			"vitest.config.ts": "export default { test: { '\\u0069nclude': ['src/**/*.check.ts'] } }\n",
			"src/a.check.ts":   "test('a', () => {})\n",
		}},
		// V1-5425: every later case fails on 676c335b0.
		{"pytest indented key in pyproject.toml", "pytest@.", "tests/check_b.py", []string{"sets python_files"}, MapTree{
			"pyproject.toml":   "[tool.pytest.ini_options]\n  python_files = [\"check_*.py\"]\n",
			"tests/check_a.py": "def test_a(): pass\n",
		}},
		{"pytest quoted key in pyproject.toml", "pytest@.", "tests/check_b.py", []string{"sets python_files"}, MapTree{
			"pyproject.toml":   "[tool.pytest.ini_options]\n\"python_files\" = \"check_*.py\"\n",
			"tests/check_a.py": "def test_a(): pass\n",
		}},
		{"pytest indented header in pyproject.toml", "pytest@.", "tests/check_b.py", []string{"sets python_files"}, MapTree{
			"pyproject.toml":   "[project]\nname = \"x\"\n\n  [tool.pytest.ini_options]\npython_files = \"check_*.py\"\n",
			"tests/check_a.py": "def test_a(): pass\n",
		}},
		{"pytest quoted and spaced header in pyproject.toml", "pytest@.", "tests/check_b.py", []string{"sets python_files"}, MapTree{
			"pyproject.toml":   "[ tool . \"pytest\" . ini_options ]\npython_files = \"check_*.py\"\n",
			"tests/check_a.py": "def test_a(): pass\n",
		}},
		{"pytest dotted key under [tool]", "pytest@.", "tests/check_b.py", []string{"sets python_files"}, MapTree{
			"pyproject.toml":   "[tool]\npytest.ini_options.python_files = \"check_*.py\"\n",
			"tests/check_a.py": "def test_a(): pass\n",
		}},
		{"pytest 9 native [tool.pytest] table", "pytest@.", "tests/check_b.py", []string{"sets python_files"}, MapTree{
			"pyproject.toml":   "[tool.pytest]\npython_files = [\"check_*.py\"]\n\n[tool.pytest.ini_options]\naddopts = \"-q\"\n",
			"tests/check_a.py": "def test_a(): pass\n",
		}},
		{"pytest.toml over pyproject.toml", "pytest@.", "tests/check_b.py", []string{"sets python_files"}, MapTree{
			"pytest.toml":      "[pytest]\npython_files = [\"check_*.py\"]\n",
			"pyproject.toml":   "[tool.pytest.ini_options]\naddopts = \"-q\"\n",
			"tests/check_a.py": "def test_a(): pass\n",
		}},
		{".pytest.ini over pyproject.toml", "pytest@.", "tests/check_b.py", []string{"sets python_files"}, MapTree{
			".pytest.ini":      "[pytest]\npython_files = check_*.py\n",
			"pyproject.toml":   "[tool.pytest.ini_options]\n",
			"tests/check_a.py": "def test_a(): pass\n",
		}},
		{".pytest.toml over setup.cfg", "pytest@svc", "svc/tests/check_b.py", []string{"sets testpaths"}, MapTree{
			"svc/.pytest.toml":       "[pytest]\ntestpaths = [\"checks\"]\n",
			"svc/setup.cfg":          "[tool:pytest]\n",
			"svc/tests/check_a.py":   "def test_a(): pass\n",
			"svc/tests/test_dflt.py": "def test_d(): pass\n",
		}},
		{"pytest.ini an older pytest reads beside pytest.toml", "pytest@.", "tests/check_b.py", []string{"pytest.ini sets python_files"}, MapTree{
			"pytest.toml":      "[pytest]\n",
			"pytest.ini":       "[pytest]\npython_files = check_*.py\n",
			"tests/check_a.py": "def test_a(): pass\n",
		}},
		{"tox.ini section header with a comment", "pytest@.", "tests/check_b.py", []string{"sets python_files"}, MapTree{
			"tox.ini":          "[tox]\nenvlist = py3\n\n[pytest] # pytest options\npython_files = check_*.py\n",
			"tests/check_a.py": "def test_a(): pass\n",
		}},
		{"tox.ini indented header above setup.cfg", "pytest@.", "tests/check_b.py", []string{"an indented [pytest] line", "setup.cfg sets python_files"}, MapTree{
			"tox.ini":          "[tox]\nenvlist =\n  py3\n  [pytest]\n",
			"setup.cfg":        "[tool:pytest]\npython_files = check_*.py\n",
			"tests/check_a.py": "def test_a(): pass\n",
		}},
		{"jest script --testMatch", "jest@.", "src/b.check.js", []string{`package.json script "test" runs jest with --testMatch`}, MapTree{
			"jest.config.js": "module.exports = { testEnvironment: 'node' }\n",
			"package.json":   `{"scripts": {"test": "jest --testMatch '**/*.check.js'"}}`,
			"src/a.check.js": "test('a', () => {})\n",
		}},
		{"jest script kebab --root-dir after another command", "jest@.", "src/b.check.js", []string{`script "test" runs jest with --root-dir`}, MapTree{
			"jest.config.json": `{"testEnvironment": "node"}`,
			"package.json":     `{"scripts": {"test": "npm run build && npx jest@29 --ci --root-dir=src"}}`,
			"src/a.check.js":   "test('a', () => {})\n",
		}},
		{"jest script expansion", "jest@.", "src/b.check.js", []string{"runs jest with the expansion $JEST_ARGS"}, MapTree{
			"jest.config.js": "module.exports = {}\n",
			"package.json":   `{"scripts": {"test": "jest $JEST_ARGS"}}`,
			"src/a.check.js": "test('a', () => {})\n",
		}},
		{"vitest script -c", "vitest@.", "src/b.check.ts", []string{`script "test:ci" runs vitest with -c`}, MapTree{
			"vitest.config.ts": "export default {}\n",
			"package.json":     `{"scripts": {"test:ci": "cross-env CI=1 vitest run -c vitest.ci.config.ts"}}`,
			"src/a.check.ts":   "test('a', () => {})\n",
		}},
		{"vitest script --root in the manifest above", "vitest@web", "web/src/b.check.ts", []string{`package.json script "test" runs vitest with --root`}, MapTree{
			"package.json":         `{"scripts": {"test": "vitest run --root web"}}`,
			"web/vitest.config.ts": "export default {}\n",
			"web/src/a.check.ts":   "test('a', () => {})\n",
		}},
		{"vitest script unmodelled flag", "vitest@.", "src/b.check.ts", []string{"runs vitest with --mode"}, MapTree{
			"vitest.config.ts": "export default {}\n",
			"package.json":     `{"scripts": {"test": "vitest --mode staging"}}`,
			"src/a.check.ts":   "test('a', () => {})\n",
		}},
	})
}

// TIV-V1-008, V1-5427: a package.json script that reaches Jest or Vitest
// through a package-manager run forwarding a flag, inside a quoted wrapper
// with a pattern or config flag, through a wrapper the reader cannot model,
// or with Jest's --runTestsByPath leaves the instance unproven. Every case
// fails on 42b0bd8a0.
func TestTIVV1008ScriptRunsFailClosed(t *testing.T) {
	jest := func(scripts string) MapTree {
		return MapTree{
			"jest.config.js": "module.exports = {}\n",
			"package.json":   `{"scripts": {` + scripts + `}}`,
			"src/a.check.js": "test('a', () => {})\n",
		}
	}
	vitest := func(scripts string) MapTree {
		return MapTree{
			"vitest.config.ts": "export default {}\n",
			"package.json":     `{"scripts": {` + scripts + `}}`,
			"src/a.check.ts":   "test('a', () => {})\n",
		}
	}
	assertUnproven(t, []unprovenCase{
		{"npm run forwarding --testMatch", "jest@.", "src/b.check.js",
			[]string{`script "test:ci" forwards --testMatch through npm to "test"`},
			jest(`"test": "jest", "test:ci": "npm run test -- --testMatch '**/*.int.js'"`)},
		{"npm test forwarding --config", "jest@.", "src/b.check.js",
			[]string{`script "ci" forwards --config through npm to "test"`},
			jest(`"test": "jest", "ci": "npm test -- --config jest.ci.js"`)},
		{"yarn script forwarding --config", "jest@.", "src/b.check.js",
			[]string{`script "ci" forwards --config through yarn to "test"`},
			jest(`"test": "jest", "ci": "yarn test --config jest.ci.js"`)},
		{"pnpm script forwarding --config", "vitest@.", "src/b.check.ts",
			[]string{`script "ci" forwards --config through pnpm to "test"`},
			vitest(`"test": "vitest run", "ci": "pnpm test --config vitest.ci.ts"`)},
		{"pnpm run forwarding --dir after an env assignment", "vitest@.", "src/b.check.ts",
			[]string{`script "ci" forwards --dir through pnpm to "test"`},
			vitest(`"test": "vitest run", "ci": "cross-env CI=1 pnpm run test -- --dir src"`)},
		{"package manager flag before the script", "vitest@.", "src/b.check.ts",
			[]string{`script "ci" forwards --root through pnpm`},
			vitest(`"test": "vitest run", "ci": "pnpm --filter web test --root src"`)},
		{"forwarding to a script that runs a package manager", "jest@.", "src/b.check.js",
			[]string{`script "ci" forwards --testRegex through yarn to "unit"`},
			jest(`"test": "jest", "unit": "cross-env CI=1 yarn test", "ci": "yarn unit --testRegex x"`)},
		{"sh -c", "jest@.", "src/b.check.js",
			[]string{`script "ci" runs jest with --config`},
			jest(`"ci": "sh -c \"jest --config ci.js\""`)},
		{"bash -c", "vitest@.", "src/b.check.ts",
			[]string{`script "ci" runs vitest with --root`},
			vitest(`"ci": "bash -c 'vitest run --root web'"`)},
		{"concurrently", "jest@.", "src/b.check.js",
			[]string{`script "dev" runs jest with --config`},
			jest(`"dev": "concurrently \"jest --config a.js\" \"vitest --dir b\""`)},
		{"concurrently after a bare runner word", "jest@.", "src/b.check.js",
			[]string{`script "dev" runs jest with --testMatch`},
			jest(`"dev": "concurrently jest \"jest --testMatch x\""`)},
		{"concurrently npm: shorthand", "jest@.", "src/b.check.js",
			[]string{`script "dev" forwards --config through npm to "test"`},
			jest(`"test": "jest", "dev": "concurrently \"npm:test -- --config x.js\""`)},
		{"npm-run-all script arguments", "jest@.", "src/b.check.js",
			[]string{`script "all" forwards --config through npm-run-all to "test"`},
			jest(`"test": "jest", "all": "npm-run-all \"test -- --config x.js\""`)},
		{"nodemon --exec", "vitest@.", "src/b.check.ts",
			[]string{`script "watch" runs vitest with --dir`},
			vitest(`"watch": "nodemon --exec \"vitest run --dir src\""`)},
		{"unmodelled wrapper mentioning the runner", "jest@.", "src/b.check.js",
			[]string{`script "ci" runs jest through the unmodelled command node`},
			jest(`"ci": "node -e \"require('jest').run(['--config', 'ci.js'])\""`)},
		{"escaped word in a wrapper", "jest@.", "src/b.check.js",
			[]string{`script "ci" runs jest through sh with jest\`},
			jest(`"ci": "sh -c jest\\ --config=ci.js"`)},
		{"jest --runTestsByPath", "jest@.", "src/b.check.js",
			[]string{`script "one" runs jest with --runTestsByPath`},
			jest(`"one": "jest --runTestsByPath src/a.check.js"`)},
	})
}

// TIV-V1-008, V1-5435: a wrapper passing flags after a standalone "--" or
// through a {@}, {*} or {N} placeholder, a runner or forwarded word holding
// a backslash, brace or expansion, a backslash-escaped runner name, a Vitest
// bench or typecheck command forwarded as a positional through a package
// manager, and a pyproject.toml the scanner cannot read whose escaped table
// name spells pytest leave the instance unproven. Every case fails on
// 88c6f838c.
func TestTIVV1008ScriptEscapesFailClosed(t *testing.T) {
	jest := func(scripts string) MapTree {
		return MapTree{
			"jest.config.js": "module.exports = {}\n",
			"package.json":   `{"scripts": {` + scripts + `}}`,
			"src/a.check.js": "test('a', () => {})\n",
		}
	}
	vitest := func(scripts string) MapTree {
		return MapTree{
			"vitest.config.ts": "export default {}\n",
			"package.json":     `{"scripts": {` + scripts + `}}`,
			"src/a.check.ts":   "test('a', () => {})\n",
		}
	}
	assertUnproven(t, []unprovenCase{
		{"concurrently {@} with flags after the outer --", "jest@.", "src/b.check.js",
			[]string{`script "dev" runs jest through concurrently with --testMatch after --`, `script "dev" runs jest through concurrently with the placeholder {@}`},
			jest(`"dev": "concurrently \"jest {@}\" -- --testMatch x"`)},
		{"concurrently {*}", "vitest@.", "src/b.check.ts",
			[]string{`script "dev" runs vitest through concurrently with the placeholder {*}`},
			vitest(`"dev": "concurrently \"vitest run {*}\" -- --dir src"`)},
		{"concurrently {1}", "jest@.", "src/b.check.js",
			[]string{`script "dev" runs jest through concurrently with the placeholder {1}`, `script "dev" runs jest through concurrently with --config after --`},
			jest(`"dev": "concurrently --kill-others \"jest --ci {1}\" -- --config=ci.js"`)},
		{"flags after the outer -- with no placeholder", "jest@.", "src/b.check.js",
			[]string{`script "dev" runs jest through concurrently with --testMatch after --`},
			jest(`"dev": "concurrently \"jest --ci\" -- --testMatch x"`)},
		{"bash -c with flags after --", "vitest@.", "src/b.check.ts",
			[]string{`script "ci" runs vitest through bash with --root after --`},
			vitest(`"ci": "bash -c 'vitest run' -- --root web"`)},
		{"backslash-escaped runner flag", "jest@.", "src/b.check.js",
			[]string{`script "ci" runs jest with the escaped or braced word \-\-config=ci.js`},
			jest(`"ci": "jest \\-\\-config=ci.js"`)},
		{"brace-expanded runner flag", "jest@.", "src/b.check.js",
			[]string{`script "ci" runs jest with the escaped or braced word {--config,ci.js}`},
			jest(`"ci": "jest {--config,ci.js}"`)},
		{"backslash-escaped flag forwarded through npm", "jest@.", "src/b.check.js",
			[]string{`script "ci" forwards \-\-config through npm to "test"`},
			jest(`"test": "jest", "ci": "npm test -- \\-\\-config=ci.js"`)},
		{"expansion forwarded through npm", "jest@.", "src/b.check.js",
			[]string{`script "ci" forwards $JEST_FLAGS through npm to "test"`},
			jest(`"test": "jest", "ci": "npm test -- $JEST_FLAGS"`)},
		{"backslash-escaped runner name", "jest@.", "src/b.check.js",
			[]string{`script "ci" runs jest through the unmodelled command j\est`},
			jest(`"ci": "j\\est --config ci.js"`)},
		{"vitest bench forwarded through npm", "vitest@.", "src/b.check.ts",
			[]string{`script "perf" forwards the bench command through npm to "test"`},
			vitest(`"test": "vitest", "perf": "npm test bench"`)},
		{"vitest typecheck forwarded through yarn", "vitest@.", "src/b.check.ts",
			[]string{`script "types" forwards the typecheck command through yarn to "test"`},
			vitest(`"test": "vitest", "types": "yarn test typecheck"`)},
		{"vitest bench forwarded through pnpm run", "vitest@.", "src/b.check.ts",
			[]string{`script "perf" forwards the bench command through pnpm to "test"`},
			vitest(`"test": "vitest", "perf": "pnpm run test bench"`)},
		{"byte order mark and an escaped pytest table name", "pytest@.", "tests/check_b.py",
			[]string{"a TOML document the reader cannot scan"}, MapTree{
				"pyproject.toml":   "\ufeff[tool.\"py\\u0074est\".ini_options]\npython_files = \"check_*.py\"\n",
				"tests/check_a.py": "def test_a(): pass\n",
			}},
	})
}

// TIV-V1-008, V1-5427: pytest configs with a UTF-8 byte order mark, CRLF
// line ends, literal-string keys, comments inside a multi-line value, a
// table below the pytest table, or an escaped table name stay unproven when
// they may move the test files.
func TestTIVV1008PytestConfigEdgesFailClosed(t *testing.T) {
	assertUnproven(t, []unprovenCase{
		{"byte order mark in pyproject.toml", "pytest@.", "tests/check_b.py", []string{"a TOML document the reader cannot scan"}, MapTree{
			"pyproject.toml":   "\ufeff[tool.pytest.ini_options]\npython_files = \"check_*.py\"\n",
			"tests/check_a.py": "def test_a(): pass\n",
		}},
		{"byte order mark in pytest.toml", "pytest@.", "tests/check_b.py", []string{"a TOML document the reader cannot scan"}, MapTree{
			"pytest.toml":      "\ufeff[pytest]\naddopts = \"-q\"\n",
			"tests/check_a.py": "def test_a(): pass\n",
		}},
		{"CRLF pyproject.toml", "pytest@.", "tests/check_b.py", []string{"sets python_files"}, MapTree{
			"pyproject.toml":   "[project]\r\nname = \"x\"\r\n\r\n[tool.pytest.ini_options]\r\naddopts = [\r\n  \"-q\",\r\n]\r\npython_files = \"check_*.py\"\r\n",
			"tests/check_a.py": "def test_a(): pass\n",
		}},
		{"CRLF pytest.ini", "pytest@.", "tests/check_b.py", []string{"sets python_files"}, MapTree{
			"pytest.ini":       "[pytest]\r\naddopts = -q\r\npython_files = check_*.py\r\n",
			"tests/check_a.py": "def test_a(): pass\n",
		}},
		{"literal-string key", "pytest@.", "tests/check_b.py", []string{"sets python_files"}, MapTree{
			"pyproject.toml":   "[tool.pytest.ini_options]\n'python_files' = 'check_*.py'\n",
			"tests/check_a.py": "def test_a(): pass\n",
		}},
		{"comments inside a multi-line addopts array", "pytest@.", "pkg/calc.py", []string{"addopts --doctest-modules"}, MapTree{
			"pytest.toml":     "[pytest]\n# options\naddopts = [\n  \"-q\",  # quiet\n  # doctests too\n  \"--doctest-modules\",\n]\n",
			"tests/test_a.py": "def test_a(): pass\n",
		}},
		{"comment inside an INI continuation", "pytest@.", "pkg/calc.py", []string{"addopts --doctest-modules"}, MapTree{
			"pytest.ini":      "[pytest]\naddopts =\n    -q\n    # doctests too\n    --doctest-modules\n",
			"tests/test_a.py": "def test_a(): pass\n",
		}},
		{"key under a table below ini_options", "pytest@.", "tests/check_b.py", []string{"sets python_files"}, MapTree{
			"pyproject.toml":   "[tool.pytest.ini_options.python_files]\nx = \"check_*.py\"\n",
			"tests/check_a.py": "def test_a(): pass\n",
		}},
		{"empty table below ini_options", "pytest@.", "tests/check_b.py", []string{"sets testpaths"}, MapTree{
			"pyproject.toml":   "[tool.pytest.ini_options]\naddopts = \"-q\"\n\n[tool.pytest.ini_options.testpaths]\n",
			"tests/check_a.py": "def test_a(): pass\n",
		}},
		{"escaped quoted table name", "pytest@.", "tests/check_b.py", []string{"an escape in a TOML key"}, MapTree{
			"pyproject.toml":   "[tool.\"py\\u0074est\".ini_options]\npython_files = \"check_*.py\"\n",
			"tests/check_a.py": "def test_a(): pass\n",
		}},
	})
}

// unprovenCase is a tree whose instance key the reader leaves unproven:
// adding the file added, which the record does not bind, makes it
// unassigned, and the cause names every text in causes.
type unprovenCase struct {
	name, key, added string
	causes           []string
	tree             MapTree
}

func assertUnproven(t *testing.T, cases []unprovenCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tree := MapTree{}
			for name, content := range c.tree {
				tree[name] = content
			}
			tree[Path] = listedInventory(t, tree)
			before := judgedInstance(t, Judge(tree), c.key)
			if before.Full || slices.Contains(before.Unassigned, c.added) {
				t.Fatalf("before %s: %+v", c.added, before)
			}
			tree[c.added] = "new\n"
			verdict := Judge(tree)
			instance := judgedInstance(t, verdict, c.key)
			if instance.Full || !slices.Contains(instance.Unassigned, c.added) {
				t.Fatalf("after %s: %+v, want it unassigned", c.added, instance)
			}
			if want := append(slices.Clone(before.Unassigned), c.added); !sameSet(instance.Unassigned, want) {
				t.Fatalf("unassigned %v, want %v", instance.Unassigned, want)
			}
			if !strings.Contains(instance.Cause, "that the record does not bind, or binds at another blob, is unassigned") {
				t.Fatalf("cause %q does not say which files are unassigned", instance.Cause)
			}
			for _, cause := range c.causes {
				if !strings.Contains(instance.Cause, cause) || !strings.Contains(instance.Cause, "not proven to keep the default test-file names") {
					t.Fatalf("cause %q does not name %q", instance.Cause, cause)
				}
			}
			if !slices.Equal(verdict.Reasons(), []string{ReasonUnassigned}) {
				t.Fatalf("reasons %v", verdict.Reasons())
			}
		})
	}
}

// TIV-V1-003, V1-5425: one pytest instance per root, its config the first
// present of pytest's documented search order (pytest.toml, .pytest.toml,
// pytest.ini, .pytest.ini, pyproject.toml, tox.ini, setup.cfg); the shared
// files only with a pytest table or section. Fails on 676c335b0.
func TestTIVV1003PytestConfigPrecedence(t *testing.T) {
	tree := MapTree{
		"pytest.toml":     "",
		".pytest.toml":    "[pytest]\n",
		"pytest.ini":      "",
		".pytest.ini":     "[pytest]\n",
		"pyproject.toml":  "[tool.pytest]\n",
		"tox.ini":         "[pytest]\n",
		"setup.cfg":       "[tool:pytest]\n",
		"tests/test_a.py": "def test_a(): pass\n",
	}
	for _, want := range []string{"pytest.toml", ".pytest.toml", "pytest.ini", ".pytest.ini", "pyproject.toml", "tox.ini", "setup.cfg"} {
		files, _ := tree.Files()
		var got []Declared
		for _, instance := range Detect(files, tree.Read) {
			if instance.Runner == "pytest" {
				got = append(got, instance)
			}
		}
		if len(got) != 1 || got[0].Key != "pytest@." || got[0].Config != want || got[0].Pattern != "" {
			t.Fatalf("with %s first: pytest instances %+v", want, got)
		}
		delete(tree, want)
	}
	files, _ := tree.Files()
	if got := Detect(files, tree.Read); len(got) != 0 {
		t.Fatalf("no config left: %+v", got)
	}
	// A shared file with no pytest table or section declares nothing.
	shared := MapTree{"pyproject.toml": "[tool.black]\nline-length = 100\n", "tox.ini": "[tox]\n", "setup.cfg": "[metadata]\nname = x\n"}
	sharedFiles, _ := shared.Files()
	if got := Detect(sharedFiles, shared.Read); len(got) != 0 {
		t.Fatalf("shared configs with no pytest section: %+v", got)
	}
}

func sameSet(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

// TIV-V1-008 control: a config the reader proves default keeps the default
// name pattern, so a new non-test source file under its root is not
// unassigned while a new default-named test file is.
func TestTIVV1008ProvenDefaultControl(t *testing.T) {
	cases := []struct {
		name, key, source, test string
		tree                    MapTree
	}{
		{"vitest defineConfig", "vitest@.", "src/lib.ts", "src/b.test.ts", MapTree{
			"vitest.config.ts": "import { defineConfig } from 'vitest/config'\n// include is the default\nexport default defineConfig({ test: { environment: 'node', exclude: ['dist/**'] } })\n",
			"package.json":     `{"scripts": {"test": "vitest run"}}`,
			"src/a.test.ts":    "test('a', () => {})\n",
		}},
		{"jest JSDoc config", "jest@.", "src/lib.js", "src/b.test.js", MapTree{
			"jest.config.js": "/** @type {import('jest').Config} */\nconst config = { testEnvironment: 'node', verbose: true }\nmodule.exports = config\n",
			"src/a.test.js":  "test('a', () => {})\n",
		}},
		{"jest JSON config", "jest@.", "src/lib.js", "src/b.test.js", MapTree{
			"jest.config.json": `{"testEnvironment": "node"}`,
			"src/a.test.js":    "test('a', () => {})\n",
		}},
		{"playwright device spread", "playwright@.", "e2e/helpers.ts", "e2e/b.spec.ts", MapTree{
			"playwright.config.ts": "import { defineConfig, devices } from '@playwright/test'\nexport default defineConfig({\n  testDir: './e2e',\n  retries: process.env.CI ? 2 : 0,\n  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],\n})\n",
			"e2e/a.spec.ts":        "test('a', async () => {})\n",
		}},
		{"jest safe script flags", "jest@.", "src/lib.js", "src/b.test.js", MapTree{
			"jest.config.js": "module.exports = {}\n",
			"package.json":   `{"scripts": {"test": "jest --ci --coverage --maxWorkers=2 -i --no-cache", "test:watch": "jest --watch src/", "lint": "eslint --ext .js ."}}`,
			"src/a.test.js":  "test('a', () => {})\n",
		}},
		{"jest scripts through package managers and wrappers", "jest@.", "src/lib.js", "src/b.test.js", MapTree{
			"jest.config.js": "module.exports = {}\n",
			"package.json":   `{"scripts": {"build": "tsc -p .", "build:watch": "npm run build -- --watch", "dev": "concurrently \"yarn build --watch\" \"jest --watch\"", "ci": "sh -c 'jest --ci --findRelatedTests src/a.test.js'", "test": "jest", "test:all": "npm test", "setup": "npm install --no-audit", "ci:yarn": "yarn jest --ci", "ci:npm": "npm exec jest -- --ci --coverage"}}`,
			"src/a.test.js":  "test('a', () => {})\n",
		}},
		{"forwarded positionals and wrappers that do not reach the runner", "vitest@.", "src/lib.ts", "src/b.test.ts", MapTree{
			"vitest.config.ts": "export default {}\n",
			"package.json":     `{"scripts": {"test": "vitest run", "one": "npm test src/a", "perf": "node perf.js", "bench": "npm run perf bench", "dev": "concurrently \"tsc -w\" -- --preserveWatchOutput"}}`,
			"src/a.test.ts":    "test('a', () => {})\n",
		}},
		{"vitest safe script flags", "vitest@.", "src/lib.ts", "src/b.test.ts", MapTree{
			"vitest.config.ts": "export default {}\n",
			"package.json":     `{"scripts": {"test": "vitest run --coverage.provider=v8 --reporter=dot", "dev": "vitest watch --pool forks"}}`,
			"src/a.test.ts":    "test('a', () => {})\n",
		}},
		{"pytest.toml safe addopts", "pytest@.", "pkg/calc.py", "tests/test_b.py", MapTree{
			"pytest.toml":     "[pytest]\naddopts = [\"-ra\", \"-q\"]\n",
			"tests/test_a.py": "def test_a(): pass\n",
		}},
		{"pyproject.toml indented, quoted and other tables", "pytest@.", "pkg/calc.py", "tests/test_b.py", MapTree{
			"pyproject.toml":  "[build-system]\nrequires = [\"hatchling>=1.0\"]\n\n[project]\nname = \"x\"\nauthors = [{ name = \"A\", email = \"a@example.com\" }]\nreleased = 1979-05-27 07:32:00Z\nversion = '1.0'\nlicense = { text = \"Apache-2.0\" }\nreadme = ''''\nraw [tool.pytest.ini_options] \\ text\n'''\ndescription = \"\"\"\npython_files = \"x\"\n\"\"\"\n\n[[tool.hatch.envs]]\nscripts = { test = \"pytest --doctest-modules\" }\n\n  [tool.pytest.ini_options]\n  \"addopts\" = \"-q\"\n  markers = [\n    \"slow\",  # a comment\n  ]\n\n[tool.other]\npython_files = \"check_*.py\"\n",
			"tests/test_a.py": "def test_a(): pass\n",
		}},
		{"CRLF pyproject.toml and pytest.ini", "pytest@.", "pkg/calc.py", "tests/test_b.py", MapTree{
			"pytest.ini":      "[pytest]\r\naddopts =\r\n    -q\r\n    # quiet\r\n    --strict-markers\r\n",
			"pyproject.toml":  "[project]\r\nname = \"x\"\r\n\r\n[tool.pytest.ini_options]\r\naddopts = [\r\n  \"-ra\",  # summary\r\n]\r\n",
			"tests/test_a.py": "def test_a(): pass\n",
		}},
		{"tox.ini section header with a comment", "pytest@.", "pkg/calc.py", "tests/test_b.py", MapTree{
			"tox.ini":         "[tox]\nenvlist = py3\n\n[pytest] ; options\naddopts = -q\n",
			"tests/test_a.py": "def test_a(): pass\n",
		}},
		{"pytest safe addopts", "pytest@.", "pkg/calc.py", "tests/test_b.py", MapTree{
			"pyproject.toml":    "[project]\nname = 'x'\n\n[tool.pytest.ini_options]\naddopts = \"-ra -q --strict-markers --tb=short\"\nmarkers = [\"slow\"]\n",
			"tests/test_a.py":   "def test_a(): pass\n",
			"tests/conftest.py": "import pytest\n\n@pytest.fixture\ndef value():\n    return 1\n",
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tree := MapTree{}
			for name, content := range c.tree {
				tree[name] = content
			}
			files, _ := tree.Files()
			for _, instance := range Detect(files, tree.Read) {
				if instance.Key == c.key && instance.Pattern != "" {
					t.Fatalf("%s is unproven: %q", c.key, instance.Pattern)
				}
			}
			tree[Path] = listedInventory(t, tree)
			if instance := judgedInstance(t, Judge(tree), c.key); instance.Full || len(instance.Unassigned) != 0 {
				t.Fatalf("fresh tree: %+v", instance)
			}
			tree[c.source] = "export const x = 1\n"
			if instance := judgedInstance(t, Judge(tree), c.key); len(instance.Unassigned) != 0 || instance.Cause != "" {
				t.Fatalf("new source file %s: %+v", c.source, instance)
			}
			tree[c.test] = "new\n"
			if instance := judgedInstance(t, Judge(tree), c.key); !slices.Equal(instance.Unassigned, []string{c.test}) || instance.Cause != "" {
				t.Fatalf("new test file %s: %+v", c.test, instance)
			}
		})
	}
}

// TIV-V1-005, TIV-V1-011: an unproven instance's record binds only the files
// its lister names, so a root with far more language files than fit under
// MaxBytes still refreshes; the verdict reports the unbound files instead.
func TestTIVV1005UnprovenRecordHoldsOnlyListedFiles(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"vitest", "vitest.cmd"} {
		bin := filepath.Join(root, "node_modules", ".bin", name)
		if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := execfile.WriteFile(bin, []byte("stub\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	config := MapTree{"vitest.config.ts": "export default { test: { include: ['src/**/*.check.ts'] } }\n"}
	files, _ := config.Files()
	files["src/a.check.ts"] = BlobID("sha1", []byte("test('a', () => {})\n"))
	const generated = 150000
	for index := range generated {
		name := fmt.Sprintf("src/gen/module%06d.ts", index)
		files[name] = BlobID("sha1", []byte(name))
	}
	declared := Detect(files, config.Read)
	if len(declared) != 1 || declared[0].Pattern == "" {
		t.Fatalf("declared %+v, want one unproven vitest instance", declared)
	}
	listed := fmt.Sprintf(`[{"name":"a","file":%q}]`, filepath.Join(root, "src", "a.check.ts"))
	input := listInput{
		Root: root, Tree: config, Files: files, Declared: declared, Commit: fixtureCommit,
		Exec: func(context.Context, string, []string) ([]byte, error) { return []byte(listed), nil },
	}
	record, _, err := listInstance(context.Background(), input, declared[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(record.Files) != 1 || record.Files[0].Path != "src/a.check.ts" {
		t.Fatalf("record binds %d files, want only the listed src/a.check.ts", len(record.Files))
	}
	data, err := Encode(Inventory{Profile: Profile, Instances: []Instance{record}, Failures: []Failure{}})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	verdict := JudgeData(data, true, nil, files, declared)
	if instance := verdict.Instances[0]; instance.Full || len(instance.Unassigned) != generated || instance.Tests != 1 {
		t.Fatalf("verdict full=%v unassigned=%d tests=%d", instance.Full, len(instance.Unassigned), instance.Tests)
	}
}
