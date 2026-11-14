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

// fixtureJest is the Jest a fixture repository installs at
// node_modules/.bin/jest: it logs its argv, answers --version and the
// inventory's --listTests, and for a run writes a Jest --json report for
// the files after --runTestsByPath (every test file without it) to the
// --outputFile path, one test per file titled by the file, failing when
// JEST_FIXTURE_FAIL names the file. The live runner is not on this host
// (NOT_RUN); the report shape is Jest's documented --json output.
const fixtureJest = `#!/bin/sh
printf 'jest %s\n' "$*" >> "$JEST_FIXTURE_LOG"
here=$(pwd -P)
case "$1" in
--version) echo 29.7.0; exit 0 ;;
--listTests) for f in "$here"/src/*.test.js; do echo "$f"; done; exit 0 ;;
esac
out="" files="" bypath=0
for a in "$@"; do
  case "$a" in
  --outputFile=*) out="${a#--outputFile=}" ;;
  --runTestsByPath) bypath=1 ;;
  -*) ;;
  *) [ "$bypath" = 1 ] && files="$files $a" ;;
  esac
done
[ -n "$files" ] || files=$(echo src/*.test.js)
success=true code=0 sep="" body=""
for f in $files; do
  status=passed
  [ "$f" = "$JEST_FIXTURE_FAIL" ] && { status=failed; success=false; code=1; }
  title=$(basename "$f" .test.js)
  body="$body$sep{\"name\":\"$here/$f\",\"status\":\"$status\",\"assertionResults\":[{\"ancestorTitles\":[\"$title\"],\"title\":\"works\",\"status\":\"$status\",\"duration\":4}]}"
  sep=","
done
[ -n "$out" ] && printf '{"success":%s,"testResults":[%s]}\n' "$success" "$body" > "$out"
exit $code
`

// fixtureNpx runs the repository's own node_modules/.bin tool, as npx does.
const fixtureNpx = `#!/bin/sh
[ "$1" = --version ] && { echo 10.0.0; exit 0; }
tool=$1
shift
exec "$(pwd -P)/node_modules/.bin/$tool" "$@"
`

// jestFixtureRepository is a committed Jest repository with two source
// files, each with its own test file, the fixture Jest installed under the
// ignored node_modules, a stub npx first on PATH and, when inventory is set,
// a committed test inventory. It returns the root and the Jest argv log.
func jestFixtureRepository(t *testing.T, inventory bool) (root, log string) {
	t.Helper()
	root = inventoryRepository(t, map[string]string{
		".gitignore":     "node_modules/\n",
		"package.json":   `{"name": "fixture", "private": true, "devDependencies": {"jest": "29.7.0"}}` + "\n",
		"jest.config.js": "module.exports = {};\n",
		"src/a.js":       "exports.add = (a, b) => a + b;\n",
		"src/a.test.js":  "const { add } = require('./a');\ntest('works', () => expect(add(1, 2)).toBe(3));\n",
		"src/b.js":       "exports.mul = (a, b) => a * b;\n",
		"src/b.test.js":  "const { mul } = require('./b');\ntest('works', () => expect(mul(2, 3)).toBe(6));\n",
	})
	log = filepath.Join(t.TempDir(), "jest.log")
	t.Setenv("JEST_FIXTURE_LOG", log)
	t.Setenv("JEST_FIXTURE_FAIL", "")
	if err := os.MkdirAll(filepath.Join(root, "node_modules", ".bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := execfile.WriteFile(filepath.Join(root, "node_modules", ".bin", "jest"), []byte(fixtureJest), 0o755); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := execfile.WriteFile(filepath.Join(bin, "npx"), []byte(fixtureNpx), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if inventory {
		if code, _, stderr := inventoryCLI(t, "--root", root, "inventory", "refresh"); code != 0 {
			t.Fatalf("refresh: exit %d %s", code, stderr)
		}
		commitAll(t, root, "inventory")
	}
	if err := os.Remove(log); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return root, log
}

// jestFixtureChange commits a change to src/a.js and returns the base.
func jestFixtureChange(t *testing.T, root string) string {
	t.Helper()
	base := gitOutput(t, root, "rev-parse", "HEAD")
	writeInventoryFiles(t, root, map[string]string{"src/a.js": "exports.add = (a, b) => b + a;\n"})
	commitAll(t, root, "change a")
	return base
}

type jestFixtureReceipt struct {
	Result      string
	Scope       string
	Invocations []struct {
		Adapter, Instance, Tier string
		Exact                   *bool
		Exit                    int
		Argv                    []string
		Outcomes                []struct{ Outcome, Slice, TestID, Unit string }
		Tests                   []struct {
			TestID         string
			Outcome        string
			DurationMillis int64
			Granularity    string
		}
	}
}

func readJestFixtureReceipt(t *testing.T, name string) jestFixtureReceipt {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	var receipt jestFixtureReceipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		t.Fatalf("receipt %s: %v", data, err)
	}
	return receipt
}

// RNC-V1-010, RNC-V1-019, RNC-V1-005, RNC-V1-012: in a Jest fixture
// repository with a committed inventory, `corvint affected` selects the one
// Jest test file a change reaches, and `corvint test` runs exactly that file
// through the built-in default `npx jest` and records each of its tests'
// outcome and duration under its canonical test ID in the receipt; the other
// test file never runs. A failing test fails the run with Jest's exit
// status kept in the receipt.
func TestRNCV1010JestSelectionRunsEndToEnd(t *testing.T) {
	root, log := jestFixtureRepository(t, true)
	base := jestFixtureChange(t, root)
	code, stdout, stderr := runCLI(t, "--root", root, "affected", "--base", base)
	var plan struct {
		Plan struct {
			Selected []struct{ UnitID string }
		}
	}
	if code != 0 || json.Unmarshal([]byte(stdout), &plan) != nil {
		t.Fatalf("affected: exit %d %s %s", code, stdout, stderr)
	}
	var selected []string
	for _, selection := range plan.Plan.Selected {
		selected = append(selected, selection.UnitID)
	}
	if !slices.Equal(selected, []string{"typescript:jest:src/a.test.js"}) {
		t.Fatalf("affected selected %q", selected)
	}
	if got := launched(t, log); got != "" {
		t.Fatalf("affected started Jest:\n%s", got)
	}

	receiptPath := filepath.Join(t.TempDir(), "r.json")
	code, _, stderr = runCLI(t, "--root", root, "test", "--base", base, "--receipt", receiptPath)
	if code != 0 || !strings.Contains(stderr, "corvint-test: CONTRACT jest@. tier=file exact=true tests=1\n") || !strings.Contains(stderr, "corvint-test: PASS selected units passed: 1 of 1") {
		t.Fatalf("test: exit %d stderr %s", code, stderr)
	}
	calls := strings.Split(strings.TrimSpace(launched(t, log)), "\n")
	// The stub npx answers --version itself, as npx does.
	if len(calls) != 1 || !strings.HasPrefix(calls[0], "jest --json --outputFile=") || !strings.HasSuffix(calls[0], " --runTestsByPath src/a.test.js") {
		t.Fatalf("Jest calls %q", calls)
	}
	receipt := readJestFixtureReceipt(t, receiptPath)
	if receipt.Result != "PASS" || receipt.Scope != "NARROWED" || len(receipt.Invocations) != 1 {
		t.Fatalf("receipt %+v", receipt)
	}
	run := receipt.Invocations[0]
	if run.Adapter != "contract" || run.Instance != "jest@." || run.Tier != "file" || run.Exact == nil || !*run.Exact || run.Exit != 0 ||
		run.Argv[0] != "npx" || run.Argv[1] != "jest" ||
		len(run.Outcomes) != 1 || run.Outcomes[0].Outcome != "PASS" || run.Outcomes[0].TestID != "jest:src/a.test.js" || run.Outcomes[0].Slice != "jest@." {
		t.Fatalf("invocation %+v", run)
	}
	if len(run.Tests) != 1 || run.Tests[0].TestID != "jest:src/a.test.js::a::works" || run.Tests[0].Outcome != "pass" || run.Tests[0].DurationMillis != 4 || run.Tests[0].Granularity != "test" {
		t.Fatalf("per-test results %+v", run.Tests)
	}

	t.Setenv("JEST_FIXTURE_FAIL", "src/a.test.js")
	code, _, stderr = runCLI(t, "--root", root, "test", "--base", base, "--receipt", receiptPath)
	receipt = readJestFixtureReceipt(t, receiptPath)
	if code != 1 || receipt.Result != "FAIL" || receipt.Invocations[0].Exit != 1 || receipt.Invocations[0].Outcomes[0].Outcome != "FAIL" ||
		len(receipt.Invocations[0].Tests) != 1 || receipt.Invocations[0].Tests[0].Outcome != "fail" {
		t.Fatalf("failing run: exit %d receipt %+v stderr %s", code, receipt, stderr)
	}
}

// RNC-V1-002, RNC-V1-019: without a committed inventory the Jest instance
// runs its declared full command, with reason inventory-absent, rather than
// a contract selection.
func TestRNCV1002JestWithoutInventoryRunsItsFullCommand(t *testing.T) {
	root, log := jestFixtureRepository(t, false)
	base := jestFixtureChange(t, root)
	code, stdout, stderr := runCLI(t, "--root", root, "test", "--base", base, "--dry-run", "--full")
	if code != 0 || !strings.Contains(stdout, `"reason":"inventory-absent`) || !strings.Contains(stdout, `"unit":"declared:jest@."`) || strings.Contains(stdout, `"adapter":"contract"`) {
		t.Fatalf("exit %d stdout %s stderr %s", code, stdout, stderr)
	}
	if got := launched(t, log); got != "" {
		t.Fatalf("a dry run launched Jest:\n%s", got)
	}
}

// RNC-V1-008: on `corvint test`, CORVINT_SELECTION=full:jest runs Jest's
// full command on every instance and says so; an unknown value exits 2
// with kill-switch-value-unknown, naming the accepted forms, and runs
// nothing.
func TestRNCV1008KillSwitchOnCorvintTest(t *testing.T) {
	root, log := jestFixtureRepository(t, true)
	base := jestFixtureChange(t, root)
	t.Setenv("CORVINT_SELECTION", "full:jest")
	receiptPath := filepath.Join(t.TempDir(), "r.json")
	code, _, stderr := runCLI(t, "--root", root, "test", "--base", base, "--receipt", receiptPath)
	if code != 0 || !strings.Contains(stderr, "corvint-test: KILL-SWITCH kill-switch: CORVINT_SELECTION=full:jest makes jest run the full command on every instance\n") ||
		!strings.Contains(stderr, "reasons=selection-full") {
		t.Fatalf("full:jest: exit %d stderr %s", code, stderr)
	}
	if got := launched(t, log); got != "jest \n" {
		t.Fatalf("Jest calls under full:jest:\n%s", got)
	}
	data, _ := os.ReadFile(receiptPath)
	if !strings.Contains(string(data), `"killSwitch":"kill-switch: CORVINT_SELECTION=full:jest`) || !strings.Contains(string(data), `"unit":"declared:jest@."`) {
		t.Fatalf("receipt %s", data)
	}
	for _, value := range []string{"fulll", "fast", "full:cypress"} {
		t.Setenv("CORVINT_SELECTION", value)
		if err := os.Remove(log); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		code, stdout, stderr := runCLI(t, "--root", root, "test", "--base", base, "--receipt", receiptPath)
		if code != 2 || stdout != "" || inventoryRefusalCode(t, stderr) != "kill-switch-value-unknown" || !strings.Contains(stderr, "full:\\u003crunner\\u003e") && !strings.Contains(stderr, "full:<runner>") {
			t.Fatalf("%s: exit %d stdout %s stderr %s", value, code, stdout, stderr)
		}
		if got := launched(t, log); got != "" {
			t.Fatalf("%s launched:\n%s", value, got)
		}
	}
}

// RNC-V1-008: `corvint affected` and `corvint watch` refuse a kill-switch
// value outside the grammar with exit 2 and kill-switch-value-unknown;
// affected names an active switch on stderr, since its frozen plan is not
// widened by it, and watch, which answers only narrowed Go selections,
// refuses a switch that sends Go to its full command.
func TestRNCV1008KillSwitchOnAffectedAndWatch(t *testing.T) {
	root, _ := jestFixtureRepository(t, true)
	base := jestFixtureChange(t, root)
	t.Setenv("CORVINT_SELECTION", "fulll")
	if code, stdout, stderr := runCLI(t, "--root", root, "affected", "--base", base); code != 2 || stdout != "" || inventoryRefusalCode(t, stderr) != "kill-switch-value-unknown" {
		t.Fatalf("affected fulll: exit %d stdout %s stderr %s", code, stdout, stderr)
	}
	if code, _, stderr := runCLI(t, "--root", root, "watch", "--index", filepath.Join(t.TempDir(), "absent.idx"), "--once"); code != 2 || !strings.Contains(stderr, `"code":"kill-switch-value-unknown"`) {
		t.Fatalf("watch fulll: exit %d stderr %s", code, stderr)
	}
	t.Setenv("CORVINT_SELECTION", "full:jest")
	if code, _, stderr := runCLI(t, "--root", root, "affected", "--base", base); code != 0 ||
		!strings.Contains(stderr, "corvint affected: KILL-SWITCH kill-switch: CORVINT_SELECTION=full:jest makes jest run the full command on every instance; this plan is not widened by it, corvint test applies it\n") {
		t.Fatalf("affected full:jest: exit %d stderr %s", code, stderr)
	}
	for _, value := range []string{"full", "full:go,jest"} {
		t.Setenv("CORVINT_SELECTION", value)
		if code, _, stderr := runCLI(t, "--root", root, "watch", "--index", filepath.Join(t.TempDir(), "absent.idx"), "--once"); code != 2 || !strings.Contains(stderr, `"code":"kill-switch-full-unsupported"`) {
			t.Fatalf("watch %s: exit %d stderr %s", value, code, stderr)
		}
	}
}
