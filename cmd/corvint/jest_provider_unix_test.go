// SPDX-License-Identifier: Apache-2.0

//go:build unix

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
)

// fixtureNpxUp runs the nearest node_modules/.bin tool at or above the
// working directory, as npx does in a workspace.
const fixtureNpxUp = `#!/bin/sh
[ "$1" = --version ] && { echo 10.0.0; exit 0; }
tool=$1
shift
dir=$(pwd -P)
while [ ! -x "$dir/node_modules/.bin/$tool" ] && [ "$dir" != / ]; do dir=$(dirname "$dir"); done
exec "$dir/node_modules/.bin/$tool" "$@"
`

// jestTwoRootRepository is a committed repository with two Jest roots,
// apps/a and apps/b, each with one source file and its test file, the
// fixture Jest and its package installed under the ignored root
// node_modules, a stub npx first on PATH, and the committed test inventory
// the fixture Jest lists. It returns the root and the Jest argv log,
// emptied after the refresh.
func jestTwoRootRepository(t *testing.T) (root, log string) {
	t.Helper()
	root = inventoryRepository(t, map[string]string{
		".gitignore":            "node_modules/\n",
		"package.json":          `{"name": "fixture", "private": true, "devDependencies": {"jest": "29.7.0"}}` + "\n",
		"apps/a/jest.config.js": "module.exports = {};\n",
		"apps/a/src/a.js":       "exports.add = (a, b) => a + b;\n",
		"apps/a/src/a.test.js":  "const { add } = require('./a');\ntest('works', () => expect(add(1, 2)).toBe(3));\n",
		"apps/b/jest.config.js": "module.exports = {};\n",
		"apps/b/src/b.js":       "exports.mul = (a, b) => a * b;\n",
		"apps/b/src/b.test.js":  "const { mul } = require('./b');\ntest('works', () => expect(mul(2, 3)).toBe(6));\n",
	})
	log = filepath.Join(t.TempDir(), "jest.log")
	t.Setenv("JEST_FIXTURE_LOG", log)
	t.Setenv("JEST_FIXTURE_FAIL", "")
	for _, dir := range []string{filepath.Join(root, "node_modules", ".bin"), filepath.Join(root, "node_modules", "jest")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := execfile.WriteFile(filepath.Join(root, "node_modules", ".bin", "jest"), []byte(fixtureJest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules", "jest", "package.json"), []byte(`{"name":"jest","version":"29.7.0"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := execfile.WriteFile(filepath.Join(bin, "npx"), []byte(fixtureNpxUp), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if code, _, stderr := inventoryCLI(t, "--root", root, "inventory", "refresh"); code != 0 {
		t.Fatalf("refresh: exit %d %s", code, stderr)
	}
	commitAll(t, root, "inventory")
	if err := os.Remove(log); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return root, log
}

// JST-V1-009, RNC-V1-010, JST-V1-001, JST-V1-004, JST-V1-005: a two-root
// Jest fixture is planned and run end to end. A one-line change to
// apps/b/src/b.js makes `corvint affected` select apps/b's one test file
// and the MCP affected tool answer the same verdict as `corvint affected
// --summary`; `corvint test` runs only that file through the built-in `npx
// jest` from apps/b, narrowed (tier file, exact), and every test its
// receipt records is one the committed inventory lists, with Jest's exit
// status kept. No consumer starts Jest except `corvint test`. `corvint
// watch` answers only Go selections (change watch-seven-runners), so its
// part of the check is NOT_PRODUCED here.
func TestJSTV1009TwoRootFixtureThroughTheConsumers(t *testing.T) {
	root, log := jestTwoRootRepository(t)
	data, err := os.ReadFile(filepath.Join(root, ".corvint", "test-inventory.json"))
	if err != nil {
		t.Fatal(err)
	}
	var inventory struct {
		Instances []struct {
			Key   string
			Tests []struct{ ID string }
		}
	}
	if err := json.Unmarshal(data, &inventory); err != nil {
		t.Fatal(err)
	}
	listed := map[string][]string{}
	for _, instance := range inventory.Instances {
		for _, test := range instance.Tests {
			listed[instance.Key] = append(listed[instance.Key], test.ID)
		}
	}
	if !slices.Equal(listed["jest@apps/a"], []string{"jest:apps/a/src/a.test.js::a::works"}) || !slices.Equal(listed["jest@apps/b"], []string{"jest:apps/b/src/b.test.js::b::works"}) {
		t.Fatalf("inventory lists %q", listed)
	}

	base := gitOutput(t, root, "rev-parse", "HEAD")
	writeInventoryFiles(t, root, map[string]string{"apps/b/src/b.js": "exports.mul = (a, b) => b * a;\n"})
	commitAll(t, root, "change b")

	code, stdout, stderr := runCLI(t, "--root", root, "affected", "--base", base)
	var plan struct {
		Plan struct{ Selected []struct{ UnitID string } }
	}
	if code != 0 || json.Unmarshal([]byte(stdout), &plan) != nil {
		t.Fatalf("affected: exit %d %s %s", code, stdout, stderr)
	}
	var selected []string
	for _, selection := range plan.Plan.Selected {
		selected = append(selected, selection.UnitID)
	}
	if !slices.Equal(selected, []string{"typescript:jest:apps/b/src/b.test.js"}) {
		t.Fatalf("affected selected %q", selected)
	}

	_, summary, stderr, code := runAffectedCLI(t, root, "--summary", "--base", base)
	if code != 0 || !strings.Contains(string(summary), "jest:apps/b/src/b.test.js") || strings.Contains(string(summary), "apps/a/src/a.test.js") {
		t.Fatalf("affected --summary: exit %d %s %s", code, summary, stderr)
	}
	responses := mcpAffectedChild(t, root, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"corvint.affected","arguments":{"base":"`+base+`"}}}`)
	checkMCPAffectedAnswer(t, responses["1"], summary)
	if got := launched(t, log); got != "" {
		t.Fatalf("affected or the MCP tool started Jest:\n%s", got)
	}

	receiptPath := filepath.Join(t.TempDir(), "r.json")
	code, _, stderr = runCLI(t, "--root", root, "test", "--base", base, "--receipt", receiptPath)
	if code != 0 || !strings.Contains(stderr, "corvint-test: CONTRACT jest@apps/b tier=file exact=true tests=1\n") || strings.Contains(stderr, "jest@apps/a") {
		t.Fatalf("test: exit %d stderr %s", code, stderr)
	}
	calls := strings.Split(strings.TrimSpace(launched(t, log)), "\n")
	if len(calls) != 1 || !strings.HasPrefix(calls[0], "jest --json --outputFile=") || !strings.HasSuffix(calls[0], " --runTestsByPath src/b.test.js") {
		t.Fatalf("Jest calls %q", calls)
	}
	receipt := readJestFixtureReceipt(t, receiptPath)
	if receipt.Result != "PASS" || receipt.Scope != "NARROWED" || len(receipt.Invocations) != 1 {
		t.Fatalf("receipt %+v", receipt)
	}
	run := receipt.Invocations[0]
	if run.Adapter != "contract" || run.Instance != "jest@apps/b" || run.Tier != "file" || run.Exact == nil || !*run.Exact || run.Exit != 0 || len(run.Tests) == 0 {
		t.Fatalf("invocation %+v", run)
	}
	for _, test := range run.Tests {
		if !slices.Contains(listed[run.Instance], test.TestID) || test.Outcome != "pass" || test.Granularity != "test" {
			t.Fatalf("receipt test %+v is not one the inventory lists for %s: %q", test, run.Instance, listed[run.Instance])
		}
	}

	// JST-V1-005: a failing selected test fails the run, Jest's exit status kept.
	t.Setenv("JEST_FIXTURE_FAIL", "src/b.test.js")
	code, _, stderr = runCLI(t, "--root", root, "test", "--base", base, "--receipt", receiptPath)
	receipt = readJestFixtureReceipt(t, receiptPath)
	if code != 1 || receipt.Result != "FAIL" || receipt.Invocations[0].Exit != 1 || receipt.Invocations[0].Tests[0].Outcome != "fail" {
		t.Fatalf("failing run: exit %d receipt %+v stderr %s", code, receipt, stderr)
	}
}

// JST-V1-007, RNC-V1-008: CORVINT_SELECTION=full:jest runs every Jest
// instance of the two-root fixture in full, each from its own root, with
// reason kill-switch, and no contract selection.
func TestJSTV1007FullJestRunsEveryInstance(t *testing.T) {
	root, log := jestTwoRootRepository(t)
	base := gitOutput(t, root, "rev-parse", "HEAD")
	writeInventoryFiles(t, root, map[string]string{"apps/b/src/b.js": "exports.mul = (a, b) => b * a;\n"})
	commitAll(t, root, "change b")
	t.Setenv("CORVINT_SELECTION", "full:jest")
	receiptPath := filepath.Join(t.TempDir(), "r.json")
	code, _, stderr := runCLI(t, "--root", root, "test", "--base", base, "--receipt", receiptPath)
	if code != 0 || !strings.Contains(stderr, "corvint-test: KILL-SWITCH kill-switch: CORVINT_SELECTION=full:jest makes jest run the full command on every instance\n") || strings.Contains(stderr, "CONTRACT jest@") {
		t.Fatalf("full:jest: exit %d stderr %s", code, stderr)
	}
	if got := launched(t, log); got != "jest \njest \n" {
		t.Fatalf("Jest calls under full:jest:\n%s", got)
	}
	data, _ := os.ReadFile(receiptPath)
	for _, unit := range []string{`"unit":"declared:jest@apps/a"`, `"unit":"declared:jest@apps/b"`} {
		if !strings.Contains(string(data), unit) {
			t.Fatalf("receipt has no %s: %s", unit, data)
		}
	}
}
