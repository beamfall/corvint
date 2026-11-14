// SPDX-License-Identifier: Apache-2.0

//go:build unix

package testinventory

import (
	"context"
	json "encoding/json/v2"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
)

// stub writes an executable shell script at root/name that refuses any
// argv but want and otherwise runs body.
func stub(t *testing.T, root, name, want, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n[ \"$*\" = '" + want + "' ] || { echo \"unexpected argv: $*\" >&2; exit 9; }\n" + body + "\n"
	if err := execfile.WriteFile(full, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

// TIV-V1-004: `vitest list --json` runs from the instance root with its
// config and names each test by its title path (CTI-V1-004).
func TestTIVV1004VitestLister(t *testing.T) {
	root, git := gitRepo(t, map[string]string{
		".gitignore":            "node_modules/\n",
		"web/vitest.config.ts":  "export default {}\n",
		"web/package-lock.json": "{}\n",
		"web/src/sum.test.ts":   "describe('sum', () => { test('adds', () => {}) })\ntest('top', () => {})\n",
		"web/src/empty.spec.ts": "\n",
	})
	stub(t, root, "web/node_modules/.bin/vitest", "list --json --config vitest.config.ts",
		`printf '[{"name":"sum > adds","file":"%s/src/sum.test.ts"},{"name":"top","file":"%s/src/sum.test.ts"}]\n' "$(pwd -P)" "$(pwd -P)"`)
	inventory := refreshTwice(t, git, nil)
	vitest := instanceByKey(t, inventory, "vitest@web")
	want := []Test{
		{ID: "vitest:web/src/sum.test.ts::sum::adds", File: "web/src/sum.test.ts"},
		{ID: "vitest:web/src/sum.test.ts::top", File: "web/src/sum.test.ts"},
	}
	if got := testsOf(vitest); !reflect.DeepEqual(got, want) {
		t.Fatalf("vitest tests %+v", got)
	}
	requireBinding(t, root, vitest, "web/src/empty.spec.ts", "web/src/sum.test.ts")
	if !reflect.DeepEqual(vitest.Lister.Command, []string{"web/node_modules/.bin/vitest", "list", "--json", "--config", "vitest.config.ts"}) {
		t.Fatalf("vitest command %v", vitest.Lister.Command)
	}
}

// jestStub writes node_modules/.bin/jest: `--listTests` prints every
// *.test.js of the working directory, and the collection run prints the
// report of collect, keyed by the working directory's base name, with ROOT
// replaced by the working directory. The report has Jest's `--json` shape
// (the AggregatedResult of @jest/test-result, https://jestjs.io/docs/cli#--json);
// it is hand-written, as this host has no Jest install. Any other argv exits 9.
func jestStub(t *testing.T, root string, collect map[string]string, collectExit int) {
	t.Helper()
	full := filepath.Join(root, "node_modules", ".bin", "jest")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\ncase \"$*\" in\n" +
		"'--listTests --config jest.config.js') for f in *.test.js; do echo \"$(pwd -P)/$f\"; done ;;\n" +
		"'--config jest.config.js --ci --coverage=false --passWithNoTests --json --testNamePattern=^(?!)')\n" +
		"  echo 'Determining test suites to run...'\n" +
		"  case \"$(basename \"$(pwd -P)\")\" in\n"
	for name, report := range collect {
		script += "  " + name + ") printf '%s\\n' '" + report + "' | sed \"s|ROOT|$(pwd -P)|g\" ;;\n"
	}
	script += "  esac\n  exit " + strconv.Itoa(collectExit) + " ;;\n*) echo \"unexpected argv: $*\" >&2; exit 9 ;;\nesac\n"
	if err := execfile.WriteFile(full, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

// TIV-V1-003, TIV-V1-004, JST-V1-002, JST-V1-007: two Jest roots are two
// instances. `jest --listTests` names the files and Jest's collection run
// names each test by canonical ID from its ancestorTitles and title
// (CTI-V1-004); a listed file the collection names no test of stays at file
// level with test-id-unmapped.
func TestTIVV1004JestListerTwoRoots(t *testing.T) {
	root, git := gitRepo(t, map[string]string{
		".gitignore":                "node_modules/\n",
		"package-lock.json":         "{}\n",
		"apps/web/jest.config.js":   "module.exports = {}\n",
		"apps/web/a.test.js":        "describe('cart', () => { it('adds item', () => {}) })\ntest('handles (a+b)', () => {})\n",
		"apps/web/empty.test.js":    "\n",
		"apps/admin/jest.config.js": "module.exports = {}\n",
		"apps/admin/b.test.js":      "test('b', () => {})\n",
	})
	jestStub(t, root, map[string]string{
		"web":   `{"success":true,"testResults":[{"name":"ROOT/a.test.js","status":"passed","assertionResults":[{"ancestorTitles":["cart"],"title":"adds item","status":"pending","duration":null},{"ancestorTitles":[],"title":"handles (a+b)","status":"pending","duration":null}]}]}`,
		"admin": `{"success":true,"testResults":[{"name":"ROOT/b.test.js","status":"passed","assertionResults":[{"ancestorTitles":[],"title":"b","status":"pending","duration":null}]}]}`,
	}, 0)
	inventory := refreshTwice(t, git, nil)
	web, admin := instanceByKey(t, inventory, "jest@apps/web"), instanceByKey(t, inventory, "jest@apps/admin")
	wantWeb := []Test{
		{ID: "jest:apps/web/a.test.js::cart::adds item", File: "apps/web/a.test.js"},
		{ID: "jest:apps/web/a.test.js::handles (a+b)", File: "apps/web/a.test.js"},
		{ID: "jest:apps/web/empty.test.js", File: "apps/web/empty.test.js", Reason: "test-id-unmapped"},
	}
	if got := testsOf(web); !reflect.DeepEqual(got, wantWeb) {
		t.Fatalf("jest web %+v", got)
	}
	if got := testsOf(admin); !reflect.DeepEqual(got, []Test{{ID: "jest:apps/admin/b.test.js::b", File: "apps/admin/b.test.js"}}) {
		t.Fatalf("jest admin %+v", got)
	}
	requireBinding(t, root, web, "apps/web/a.test.js", "apps/web/empty.test.js")
	if web.Lockfile != "package-lock.json" || web.Lister.Command[0] != "node_modules/.bin/jest" || web.Lister.Kind != KindExecuting || web.Lister.Version != "2" {
		t.Fatalf("jest web lockfile %q lister %+v", web.Lockfile, web.Lister)
	}
}

// JST-V1-002, TIV-V1-011: a failing collection run (a test file that does
// not load exits 1) fails the lister, so the instance is absent from the
// inventory and recorded as a failure, never listed by guess.
func TestJSTV1002FailedCollectionLeavesTheInstanceAbsent(t *testing.T) {
	root, git := gitRepo(t, map[string]string{
		".gitignore":         "node_modules/\n",
		"package-lock.json":  "{}\n",
		"web/jest.config.js": "module.exports = {}\n",
		"web/a.test.js":      "test('a', () => {})\n",
	})
	jestStub(t, root, map[string]string{
		"web": `{"success":false,"testResults":[{"name":"ROOT/a.test.js","status":"failed","assertionResults":[]}]}`,
	}, 1)
	inventory, err := Build(context.Background(), Options{Git: git})
	if err != nil {
		t.Fatal(err)
	}
	for _, instance := range inventory.Instances {
		if instance.Key == "jest@web" {
			t.Fatalf("instance listed despite a failed collection: %+v", instance)
		}
	}
	if len(inventory.Failures) != 1 || inventory.Failures[0].Key != "jest@web" || !strings.Contains(inventory.Failures[0].Error, "jest collection run") {
		t.Fatalf("failures %+v", inventory.Failures)
	}
}

// TIV-V1-004: the Playwright lister converts `playwright test --list
// --reporter=json` through playwright-discovery/0 and records the
// executing kind, its command and the receipt's source digest.
func TestTIVV1004PlaywrightLister(t *testing.T) {
	root, git := gitRepo(t, map[string]string{
		".gitignore":           "node_modules/\n",
		"package-lock.json":    "{}\n",
		"playwright.config.ts": "export default defineConfig({ projects: [{ name: 'p', testDir: 'p' }] });\n",
		"p/a.spec.ts":          "test.describe('home', () => { test('loads', async () => {}); });\n",
	})
	listing, err := json.Marshal(map[string]any{
		"config": map[string]any{
			"configFile": filepath.Join(root, "playwright.config.ts"), "rootDir": filepath.Join(root, "p"), "shard": nil,
			"argv": []any{"/usr/local/bin/node", root + "/node_modules/.bin/playwright", "test", "--list", "--reporter=json", "--config=playwright.config.ts"},
		},
		"suites": []any{map[string]any{
			"title": "a.spec.ts", "file": "a.spec.ts", "specs": []any{},
			"suites": []any{map[string]any{
				"title": "home", "file": "a.spec.ts", "suites": []any{},
				"specs": []any{map[string]any{"title": "loads", "file": "a.spec.ts", "tests": []any{map[string]any{"projectName": "p"}}}},
			}},
		}},
		"errors": []any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	listingFile := filepath.Join(t.TempDir(), "listing.json")
	if err := os.WriteFile(listingFile, listing, 0o644); err != nil {
		t.Fatal(err)
	}
	stub(t, root, "node_modules/.bin/playwright", "test --list --reporter=json --config=playwright.config.ts", "cat '"+listingFile+"'")
	inventory := refreshTwice(t, git, nil)
	playwright := instanceByKey(t, inventory, "playwright@.")
	if got := testsOf(playwright); !reflect.DeepEqual(got, []Test{{ID: "playwright:p/a.spec.ts::p::home::loads", File: "p/a.spec.ts"}}) {
		t.Fatalf("playwright tests %+v", got)
	}
	requireBinding(t, root, playwright, "p/a.spec.ts")
	if playwright.Lister.Kind != KindExecuting || playwright.Lister.Command[0] != "node_modules/.bin/playwright" {
		t.Fatalf("playwright lister %+v", playwright.Lister)
	}
	if playwright.Lister.Receipt == nil || playwright.Lister.Receipt.Profile != "playwright-discovery/0" || !strings.HasPrefix(playwright.Lister.Receipt.SourceDigest, "playwright-sources:sha256:") {
		t.Fatalf("playwright receipt %+v", playwright.Lister.Receipt)
	}
}

// TIV-V1-004, TIV-V1-005: `pytest --collect-only -q` runs through the
// project's virtualenv with the instance root as rootdir, and each node ID
// becomes one test with its parameter part kept (CTI-V1-006). Each test
// carries its kind (PYR-V1-005): TestK is not in the file, so its kind is
// UNKNOWN.
func TestTIVV1004PytestLister(t *testing.T) {
	root, git := gitRepo(t, map[string]string{
		".gitignore":          ".venv/\n",
		"svc/pytest.ini":      "[pytest]\n",
		"svc/uv.lock":         "",
		"svc/tests/test_a.py": "def test_one(): pass\n",
		"svc/tests/b_test.py": "\n",
	})
	stub(t, root, ".venv/bin/python", "-m pytest --collect-only -q -p no:cacheprovider --rootdir=. -c pytest.ini",
		`printf 'tests/test_a.py::test_one\ntests/test_a.py::TestK::test_two[a::b]\n\n2 tests collected in 0.01s\n'`)
	inventory := refreshTwice(t, git, nil)
	pytest := instanceByKey(t, inventory, "pytest@svc")
	want := []Test{
		{ID: "pytest:svc/tests/test_a.py::TestK::test_two[a%3A%3Ab]", File: "svc/tests/test_a.py", Kind: KindUnknown, KindSource: KindSourceUnresolved},
		{ID: "pytest:svc/tests/test_a.py::test_one", File: "svc/tests/test_a.py", Kind: KindUnit, KindSource: KindSourceDefault},
	}
	if got := testsOf(pytest); !reflect.DeepEqual(got, want) {
		t.Fatalf("pytest tests %+v", got)
	}
	requireBinding(t, root, pytest, "svc/tests/b_test.py", "svc/tests/test_a.py")
	if pytest.Lockfile != "svc/uv.lock" || pytest.Lister.Command[0] != ".venv/bin/python" {
		t.Fatalf("pytest lockfile %q command %v", pytest.Lockfile, pytest.Lister.Command)
	}
}

// TIV-V1-005, TIV-V1-008: a Vitest config with include is unproven, so a
// refresh binds only the files the lister names, never every JavaScript or
// TypeScript file under the root, and a later reader judges each file of
// those languages the record does not bind as unassigned, in a testdata or
// node_modules-named path too.
func TestTIVV1005UnprovenRefreshBindsOnlyListedFiles(t *testing.T) {
	root, git := gitRepo(t, map[string]string{
		".gitignore":          "node_modules/\n",
		"vitest.config.ts":    "export default { test: { include: ['src/**/*.check.ts'] } }\n",
		"package-lock.json":   "{}\n",
		"src/a.check.ts":      "test('a', () => {})\n",
		"src/lib.ts":          "export const x = 1\n",
		"src/readme.md":       "not code\n",
		"src/testdata/f.ts":   "fixture\n",
		"src/node_modules.ts": "a file, not the directory\n",
	})
	stub(t, root, "node_modules/.bin/vitest", "list --json --config vitest.config.ts",
		`printf '[{"name":"a","file":"%s/src/a.check.ts"}]\n' "$(pwd -P)"`)
	inventory := refreshTwice(t, git, nil)
	vitest := instanceByKey(t, inventory, "vitest@.")
	if got := testsOf(vitest); !reflect.DeepEqual(got, []Test{{ID: "vitest:src/a.check.ts::a", File: "src/a.check.ts"}}) {
		t.Fatalf("vitest tests %+v", got)
	}
	requireBinding(t, root, vitest, "src/a.check.ts")
	if _, err := Refresh(context.Background(), Options{Git: git}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "src/b.check.ts", "test('b', () => {})\n")
	work, err := NewWorkTree(context.Background(), git)
	if err != nil {
		t.Fatal(err)
	}
	instance := judgedInstance(t, Judge(work), "vitest@.")
	want := []string{"src/b.check.ts", "src/lib.ts", "src/node_modules.ts", "src/testdata/f.ts"}
	if instance.Full || !reflect.DeepEqual(instance.Unassigned, want) || !strings.Contains(instance.Cause, "(sets include)") {
		t.Fatalf("verdict after a new custom-named test file: %+v", instance)
	}
}

// TIV-V1-003, TIV-V1-004: a package.json script that runs vitest declares
// an unproven Vitest instance with no config file, which the lister runs
// without --config.
func TestTIVV1004VitestDeclaredByPackageScript(t *testing.T) {
	root, git := gitRepo(t, map[string]string{
		".gitignore":        "node_modules/\n",
		"package.json":      `{"scripts": {"test": "vitest run"}}`,
		"package-lock.json": "{}\n",
		"src/a.test.ts":     "test('a', () => {})\n",
	})
	stub(t, root, "node_modules/.bin/vitest", "list --json",
		`printf '[{"name":"a","file":"%s/src/a.test.ts"}]\n' "$(pwd -P)"`)
	inventory := refreshTwice(t, git, nil)
	vitest := instanceByKey(t, inventory, "vitest@.")
	if vitest.Config != "package.json" || !reflect.DeepEqual(vitest.Lister.Command, []string{"node_modules/.bin/vitest", "list", "--json"}) {
		t.Fatalf("vitest instance config %q command %v", vitest.Config, vitest.Lister.Command)
	}
	requireBinding(t, root, vitest, "src/a.test.ts")
}
