// SPDX-License-Identifier: Apache-2.0

//go:build unix

package testinventory

import (
	"context"
	json "encoding/json/v2"
	"os"
	"path/filepath"
	"reflect"
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

// TIV-V1-003, TIV-V1-004: two Jest roots are two instances; `jest
// --listTests` names files only, so each is listed at file level.
func TestTIVV1004JestListerTwoRoots(t *testing.T) {
	root, git := gitRepo(t, map[string]string{
		".gitignore":                "node_modules/\n",
		"package-lock.json":         "{}\n",
		"apps/web/jest.config.js":   "module.exports = {}\n",
		"apps/web/a.test.js":        "test('a', () => {})\n",
		"apps/admin/jest.config.js": "module.exports = {}\n",
		"apps/admin/b.test.js":      "test('b', () => {})\n",
	})
	stub(t, root, "node_modules/.bin/jest", "--listTests --config jest.config.js",
		`for f in *.test.js; do echo "$(pwd -P)/$f"; done`)
	inventory := refreshTwice(t, git, nil)
	web, admin := instanceByKey(t, inventory, "jest@apps/web"), instanceByKey(t, inventory, "jest@apps/admin")
	if got := testsOf(web); !reflect.DeepEqual(got, []Test{{ID: "jest:apps/web/a.test.js", File: "apps/web/a.test.js", Reason: "test-id-unmapped"}}) {
		t.Fatalf("jest web %+v", got)
	}
	if got := testsOf(admin); !reflect.DeepEqual(got, []Test{{ID: "jest:apps/admin/b.test.js", File: "apps/admin/b.test.js", Reason: "test-id-unmapped"}}) {
		t.Fatalf("jest admin %+v", got)
	}
	requireBinding(t, root, web, "apps/web/a.test.js")
	if web.Lockfile != "package-lock.json" || web.Lister.Command[0] != "node_modules/.bin/jest" {
		t.Fatalf("jest web lockfile %q command %v", web.Lockfile, web.Lister.Command)
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
// becomes one test with its parameter part kept (CTI-V1-006).
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
		{ID: "pytest:svc/tests/test_a.py::TestK::test_two[a%3A%3Ab]", File: "svc/tests/test_a.py"},
		{ID: "pytest:svc/tests/test_a.py::test_one", File: "svc/tests/test_a.py"},
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
