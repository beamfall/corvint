// SPDX-License-Identifier: Apache-2.0
package contract

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testid"
	"github.com/Beamfall/corvint/internal/testinventory"
)

// JST-V1-009 (in-process part), JST-V1-002 to JST-V1-005: a Jest fixture
// with two roots goes through the chain `corvint test` will drive. The
// inventory lists each root's tests by canonical ID, the selection of one
// test builds an exact anchored command from its root, and the run's report
// reconciles to that test with its outcome and duration, every result
// matching the inventory. Jest itself is faked: --listTests and the
// collection run answer from the fixture, and the run report is
// hand-written in Jest's --json shape (https://jestjs.io/docs/cli#--json),
// as this host has no Jest install. The four consumer runs (`corvint
// affected`, `corvint test`, `corvint watch`, MCP) are NOT_PRODUCED.
func TestJSTV1009TwoRootFixtureChain(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the lister looks for node_modules/.bin/jest.cmd on Windows")
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	files := map[string]string{
		".gitignore":              "node_modules/\n",
		"package-lock.json":       "{}\n",
		"apps/a/jest.config.js":   "module.exports = {}\n",
		"apps/a/src/cart.js":      "exports.add = (c, i) => [...c, i]\n",
		"apps/a/src/cart.test.js": "describe('cart', () => { it('adds item', () => {}); it('adds items', () => {}) })\n",
		"apps/b/jest.config.js":   "module.exports = {}\n",
		"apps/b/src/user.test.js": "test('adds item', () => {})\n",
		"node_modules/.bin/jest":  "",
	}
	for name, body := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "fixture"}} {
		if out, err := exec.Command(git, append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	collected := map[string]string{
		"apps/a": `{"success":true,"testResults":[{"name":"` + root + `/apps/a/src/cart.test.js","status":"passed","assertionResults":[` +
			`{"ancestorTitles":["cart"],"title":"adds item","status":"pending","duration":null},` +
			`{"ancestorTitles":["cart"],"title":"adds items","status":"pending","duration":null}]}]}`,
		"apps/b": `{"success":true,"testResults":[{"name":"` + root + `/apps/b/src/user.test.js","status":"passed","assertionResults":[` +
			`{"ancestorTitles":[],"title":"adds item","status":"pending","duration":null}]}]}`,
	}
	fake := func(_ context.Context, dir string, argv []string) ([]byte, error) {
		rel, _ := filepath.Rel(root, dir)
		rel = filepath.ToSlash(rel)
		switch {
		case slices.Contains(argv, "--listTests"):
			test := map[string]string{"apps/a": "src/cart.test.js", "apps/b": "src/user.test.js"}[rel]
			return []byte(filepath.Join(dir, filepath.FromSlash(test)) + "\n"), nil
		case slices.Contains(argv, "--testNamePattern=^(?!)"):
			return []byte(collected[rel]), nil
		}
		t.Fatalf("unexpected lister argv %q in %s", argv, rel)
		return nil, nil
	}
	inventory, err := testinventory.Build(context.Background(), testinventory.Options{Git: testinventory.Git{Executable: git, Root: root}, Exec: fake})
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string][]string{}
	for _, instance := range inventory.Instances {
		for _, test := range instance.Tests {
			listed[instance.Key] = append(listed[instance.Key], test.ID)
		}
	}
	if want := []string{"jest:apps/a/src/cart.test.js::cart::adds item", "jest:apps/a/src/cart.test.js::cart::adds items"}; !slices.Equal(listed["jest@apps/a"], want) {
		t.Fatalf("jest@apps/a lists %q (failures %+v)", listed["jest@apps/a"], inventory.Failures)
	}
	if want := []string{"jest:apps/b/src/user.test.js::adds item"}; !slices.Equal(listed["jest@apps/b"], want) {
		t.Fatalf("jest@apps/b lists %q", listed["jest@apps/b"])
	}

	// The change to apps/a/src/cart.js selects the one test that uses it.
	selected := []string{"jest:apps/a/src/cart.test.js::cart::adds item"}
	instance := Instance{Runner: testid.Jest, Root: "apps/a", Config: "apps/a/jest.config.js"}
	read := func(rel string) ([]byte, error) {
		if rel == "node_modules/jest/package.json" {
			return []byte(`{"version":"29.7.0"}`), nil
		}
		return os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	}
	setup := JestSetupFrom(instance, read)
	report := filepath.Join(t.TempDir(), "jest.json")
	got := command(t, Request{Instance: instance, Full: []string{"npx", "jest"}, Tests: selected, Tier: TierLine, Report: report, Listed: listed["jest@apps/a"], Jest: &setup})
	want := []string{"npx", "jest", "--json", "--outputFile=" + report, "--runTestsByPath", "src/cart.test.js", "-t", "^(?:cart adds item)$"}
	if !slices.Equal(got.Argv, want) || !got.Exact || got.Dir != "apps/a" {
		t.Fatalf("command %+v\nwant argv %q", got, want)
	}

	r, _ := For(testid.Jest)
	results, err := r.Parse(ParseRequest{Instance: instance, Root: root, Report: []byte(`{"success":true,"testResults":[{"name":"` + root + `/apps/a/src/cart.test.js","status":"passed","assertionResults":[` +
		`{"ancestorTitles":["cart"],"title":"adds item","status":"passed","duration":14},` +
		`{"ancestorTitles":["cart"],"title":"adds items","status":"pending","duration":null}]}]}`)})
	run := ReconcileParsed(selected, results, err)
	if run.Incomplete || run.Failed || len(run.NotRun) != 0 {
		t.Fatalf("run %+v", run)
	}
	for _, result := range run.Results {
		if !slices.Contains(listed["jest@apps/a"], result.ID) {
			t.Fatalf("result %s is not in the inventory", result.ID)
		}
		if result.ID == selected[0] && (result.Outcome != OutcomePass || result.DurationMillis != 14) {
			t.Fatalf("selected result %+v", result)
		}
	}
	if strings.Contains(strings.Join(got.Argv, " "), "apps/b") {
		t.Fatalf("apps/a's command names apps/b: %q", got.Argv)
	}
}
