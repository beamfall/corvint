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

// pytestRunStub is a python on PATH that answers `--version`, lists
// `<file>::<test>` for every `def test_` of every test_*.py under the
// working directory for the inventory's `--collect-only -q`, logs every
// other argv to $PYR_LOG, and, for a run with --junitxml, writes a pytest
// JUnit XML report (xunit1, the shape src/_pytest/junitxml.py writes) with
// one passing testcase per `def test_` of each selected file argument. It
// is a stand-in: pytest is not installed on this host.
const pytestRunStub = `#!/bin/sh
if [ "$1" = --version ]; then echo "Python 3.12.1"; exit 0; fi
case " $* " in *" --collect-only "*)
  for f in $(find . -name 'test_*.py' | sort); do
    f=${f#./}
    grep -o '^def test_[A-Za-z0-9_]*' "$f" | sed "s|^def |$f::|"
  done
  echo
  exit 0;;
esac
echo "python $*" >> "$PYR_LOG"
report=""
cases=""
for arg in "$@"; do
  case "$arg" in
    --junitxml=*) report="${arg#--junitxml=}";;
    *.py) file=${arg#./}
          module=$(printf '%s' "$file" | sed 's/\.py$//; s|/|.|g')
          for name in $(grep -o '^def test_[A-Za-z0-9_]*' "$file" | sed 's/^def //'); do
            cases="$cases<testcase classname=\"$module\" name=\"$name\" file=\"$file\" time=\"0.004\"/>"
          done;;
  esac
done
[ -n "$report" ] && printf '<testsuites><testsuite name="pytest">%s</testsuite></testsuites>' "$cases" > "$report"
echo "platform linux -- Python 3.12.1, pytest-8.3.2, pluggy-1.5.0"
exit 0
`

// pytestChange is a pytest adopter (no Go module) whose base commit has
// two test files importing app/calc.py and the committed test inventory
// the stub's collection lists, and a commit that edits app/calc.py, so
// both test files are selected unchanged; the stub python
// (also as python3, which the inventory's lister runs) is first on PATH. It
// returns the root, the base and the log, emptied after the refresh.
func pytestChange(t *testing.T, extra map[string]string) (root, base, log string) {
	t.Helper()
	files := map[string]string{
		"tests/test_calc.py": "from app.calc import add\n\n\ndef test_add():\n    assert add(1, 2) == 3\n\ndef test_add_zero():\n    assert add(1, 0) == 1\n",
		"tests/test_more.py": "from app.calc import add\n\n\ndef test_more():\n    assert add(0, 0) == 0\n",
	}
	for name, body := range extra {
		files[name] = body
	}
	root, _ = pytestAdopter(t, files)
	bin := t.TempDir()
	log = filepath.Join(t.TempDir(), "pytest.log")
	for _, name := range []string{"python", "python3"} {
		if err := execfile.WriteFile(filepath.Join(bin, name), []byte(pytestRunStub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("PYR_LOG", log)
	if code, _, stderr := inventoryCLI(t, "--root", root, "inventory", "refresh"); code != 0 {
		t.Fatalf("refresh: exit %d %s", code, stderr)
	}
	commitAll(t, root, "inventory")
	base = strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
	writeInventoryFiles(t, root, map[string]string{"app/calc.py": "def add(a, b):\n    return b + a\n"})
	commitAll(t, root, "edit calc")
	if err := os.Remove(log); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return root, base, log
}

// PYR-V1-003, PYR-V1-004, PYR-V1-002: `corvint test` runs the plan's
// pytest files through the compiled-in runner, needing no provider
// executable: one pytest invocation from the rootdir with the selected
// files, labelled tier file on stderr and in the receipt, which records
// each test's canonical ID and outcome and the environment record
// (PYR-V1-008).
func TestPYRV1003CLIRunsPytestSelection(t *testing.T) {
	root, base, log := pytestChange(t, nil)
	receiptPath := filepath.Join(t.TempDir(), "r.json")
	code, _, stderr := runCLI(t, "--root", root, "test", "--base", base, "--receipt", receiptPath)
	if code != 0 || !strings.Contains(stderr, "corvint-test: CONTRACT pytest@. tier=file exact=true tests=2\n") {
		t.Fatalf("exit %d stderr %s", code, stderr)
	}
	calls := launched(t, log)
	if !strings.HasPrefix(calls, "python -m pytest --rootdir=. --junitxml=") || !strings.HasSuffix(calls, " tests/test_calc.py tests/test_more.py\n") || strings.Count(calls, "\n") != 1 {
		t.Fatalf("pytest calls:\n%s", calls)
	}
	data, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	var receipt struct {
		Result      string
		Invocations []struct {
			Adapter, Instance, Tier string
			Tests                   []struct{ TestID, Outcome string }
			Environment             *struct{ Python, Pytest string }
		}
	}
	if err := json.Unmarshal(data, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Result != "PASS" || len(receipt.Invocations) != 1 {
		t.Fatalf("receipt %s", data)
	}
	run := receipt.Invocations[0]
	var tests []string
	for _, test := range run.Tests {
		tests = append(tests, test.TestID+" "+test.Outcome)
	}
	if run.Adapter != "contract" || run.Instance != "pytest@." || run.Tier != "file" || run.Environment == nil || run.Environment.Python != "3.12.1" || run.Environment.Pytest != "8.3.2" ||
		!slices.Equal(tests, []string{"pytest:tests/test_calc.py::test_add pass", "pytest:tests/test_calc.py::test_add_zero pass", "pytest:tests/test_more.py::test_more pass"}) {
		t.Fatalf("runner invocation %+v in %s", run, data)
	}

	// PYR-V1-006, RNC-V1-002 (review P2-c): a changed test file is
	// unassigned in the inventory (TIV-V1-008), so no collected test list
	// vouches for it. It runs whole and is reconciled against the testcases
	// pytest reported: the run passes and the edit's new test is in the
	// receipt.
	head := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
	writeInventoryFiles(t, root, map[string]string{"tests/test_more.py": "def test_more():\n    assert True\n\ndef test_more_new():\n    assert True\n"})
	commitAll(t, root, "edit a test file")
	code, _, stderr = runCLI(t, "--root", root, "test", "--base", head, "--receipt", receiptPath)
	if code != 0 || strings.Contains(stderr, "INCOMPLETE") {
		t.Fatalf("changed test file: exit %d stderr %s", code, stderr)
	}
	data, err = os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	receipt.Invocations = nil
	if err := json.Unmarshal(data, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Result != "PASS" || len(receipt.Invocations) != 1 || !slices.ContainsFunc(receipt.Invocations[0].Tests, func(test struct{ TestID, Outcome string }) bool {
		return test.TestID == "pytest:tests/test_more.py::test_more_new" && test.Outcome == "pass"
	}) {
		t.Fatalf("changed test file: receipt %s", data)
	}
}

// PYR-V1-007: the advisory `python -m pytest` command `corvint affected`
// prints names exactly the test files `corvint test` runs for the same
// plan.
func TestPYRV1007AdvisoryCommandNamesTheRunTests(t *testing.T) {
	root, base, _ := pytestChange(t, nil)
	code, stdout, stderr := runCLI(t, "--root", root, "affected", "--base", base)
	if code != 0 {
		t.Fatalf("affected exit %d stderr %s", code, stderr)
	}
	var plan struct {
		Advice struct{ Checks []affectedCheck } `json:"advice"`
	}
	if err := json.Unmarshal([]byte(stdout), &plan); err != nil {
		t.Fatal(err)
	}
	var advised []string
	for _, check := range plan.Advice.Checks {
		if command, ok := strings.CutPrefix(check.Command, adviceAdvisoryPytest+" "); ok {
			for _, operand := range strings.Fields(command) {
				advised = append(advised, strings.TrimPrefix(strings.Trim(operand, "'"), "./"))
			}
		}
	}
	code, stdout, stderr = runCLI(t, "--root", root, "test", "--base", base, "--dry-run", "--full")
	if code != 0 {
		t.Fatalf("dry run exit %d stderr %s", code, stderr)
	}
	var dry struct {
		Invocations []struct {
			Adapter string
			Units   []struct{ Unit string }
		}
	}
	if err := json.Unmarshal([]byte(stdout), &dry); err != nil {
		t.Fatal(err)
	}
	var run []string
	for _, invocation := range dry.Invocations {
		for _, unit := range invocation.Units {
			if invocation.Adapter == "contract" {
				run = append(run, strings.TrimPrefix(unit.Unit, "pytest:"))
			}
		}
	}
	slices.Sort(advised)
	slices.Sort(run)
	if len(run) == 0 || !slices.Equal(advised, run) {
		t.Fatalf("advisory files %q, run files %q\n%s", advised, run, stdout)
	}
}

// PYR-V1-009: CORVINT_SELECTION=full:pytest runs the pytest full suite with
// reason kill-switch, printed; a value outside the kill-switch grammar (owner decision R13) exits
// 2 with kill-switch-value-unknown before any run.
func TestPYRV1009CLIKillSwitch(t *testing.T) {
	root, base, log := pytestChange(t, nil)
	t.Setenv("CORVINT_SELECTION", "full:pytest")
	receiptPath := filepath.Join(t.TempDir(), "r.json")
	code, _, stderr := runCLI(t, "--root", root, "test", "--base", base, "--receipt", receiptPath)
	if code != 0 || !strings.Contains(stderr, "corvint-test: KILL-SWITCH kill-switch: CORVINT_SELECTION=full:pytest makes pytest run the full command on every instance\n") ||
		strings.Contains(stderr, "CONTRACT pytest@.") {
		t.Fatalf("exit %d stderr %s", code, stderr)
	}
	if calls := launched(t, log); calls != "python -m pytest\n" {
		t.Fatalf("pytest calls:\n%s", calls)
	}
	t.Setenv("CORVINT_SELECTION", "fast")
	code, _, stderr = runCLI(t, "--root", root, "test", "--base", base, "--receipt", receiptPath)
	if code != 2 || !strings.Contains(stderr, "kill-switch-value-unknown") {
		t.Fatalf("unknown value: exit %d stderr %s", code, stderr)
	}
	if calls := launched(t, log); calls != "python -m pytest\n" {
		t.Fatalf("an unknown value ran pytest:\n%s", calls)
	}
}
