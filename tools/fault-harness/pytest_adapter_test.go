// SPDX-License-Identifier: Apache-2.0

//go:build unix

package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/execfile"
)

// pytestStub stands in for `python -m pytest` (pytest is not installed on
// this host): it logs its argv, writes the JUnit XML report in
// $PYR_REPORT_SRC to the --junitxml path (a hand-written report in pytest's
// xunit1 shape, src/_pytest/junitxml.py) and exits with $PYR_EXIT.
const pytestStub = `#!/bin/sh
echo "$@" > "$PYR_ARGV"
for arg in "$@"; do
  case "$arg" in --junitxml=*) [ -n "$PYR_REPORT_SRC" ] && cp "$PYR_REPORT_SRC" "${arg#--junitxml=}";; esac
done
exit "${PYR_EXIT:-0}"
`

func pytestHarness(t *testing.T, report string, exit string) (Adapter, string) {
	t.Helper()
	dir := t.TempDir()
	stub := filepath.Join(dir, "pytest-stub")
	if err := execfile.WriteFile(stub, []byte(pytestStub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PYR_ARGV", filepath.Join(dir, "argv"))
	t.Setenv("PYR_EXIT", exit)
	t.Setenv("PYR_REPORT_SRC", "")
	if report != "" {
		src := filepath.Join(dir, "report.xml")
		if err := os.WriteFile(src, []byte(report), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PYR_REPORT_SRC", src)
	}
	adapter, err := adapterFor("pytest", adapterOptions{Pytest: []string{stub}, Timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	return adapter, dir
}

// PYR-V1-010, FIH-V1-008: a fault case naming three node IDs gets three
// per-test outcomes from the pytest adapter, keyed `UNIT#NAME`, through the
// contract's node-ID selection and JUnit parser.
func TestPYRV1010PytestHarnessAdapter(t *testing.T) {
	report := `<testsuites><testsuite name="pytest">
<testcase classname="tests.test_a" name="test_one" file="tests/test_a.py" time="0.01"/>
<testcase classname="tests.test_a.TestK" name="test_two[x]" file="tests/test_a.py" time="0.01"><failure message="assert">E</failure></testcase>
<testcase classname="tests.test_b" name="test_three" file="tests/test_b.py" time="0"><skipped type="pytest.skip" message="s"/></testcase>
</testsuite></testsuites>`
	adapter, dir := pytestHarness(t, report, "1")
	entries := []planEntry{{Unit: "pytest:tests/test_a.py", Run: []string{"test_one", "TestK::test_two[x]"}}, {Unit: "pytest:tests/test_b.py", Run: []string{"test_three"}}}
	got, err := adapter.Run(context.Background(), t.TempDir(), entries)
	if err != nil {
		t.Fatal(err)
	}
	want := RunResult{"pytest:tests/test_a.py#test_one": Pass, "pytest:tests/test_a.py#TestK::test_two[x]": Fail, "pytest:tests/test_b.py#test_three": Skip}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("outcomes %v, want %v", got, want)
	}
	argv, _ := os.ReadFile(filepath.Join(dir, "argv"))
	if !strings.HasSuffix(strings.TrimSpace(string(argv)), "-o junit_duration_report=call tests/test_a.py::TestK::test_two[x] tests/test_a.py::test_one tests/test_b.py::test_three") {
		t.Fatalf("argv %q", argv)
	}
	for _, id := range []string{"pytest:tests/test_a.py#test_one", "pytest:tests/test_b.py#test_three"} {
		if !selects(entries, id) {
			t.Fatalf("the plan does not select %s", id)
		}
	}
}

// PYR-V1-010, FIH-V1-005: a pytest run that is not a clean set of outcomes
// is a failed measurement, never a kill or a pass.
func TestPYRV1010PytestHarnessFailedMeasurements(t *testing.T) {
	one := `<testsuites><testsuite><testcase classname="tests.test_a" name="test_one" file="tests/test_a.py" time="0"/></testsuite></testsuites>`
	for name, test := range map[string]struct{ report, exit, unit string }{
		"no tests collected":          {`<testsuites><testsuite/></testsuites>`, "5", "pytest:tests/test_a.py"},
		"usage error":                 {"", "4", "pytest:tests/test_a.py"},
		"no report":                   {"", "0", "pytest:tests/test_a.py"},
		"selected test not run":       {one, "0", "pytest:tests/test_b.py"},
		"exit 1 with nothing failing": {one, "1", "pytest:tests/test_a.py"},
		"collection error":            {`<testsuites><testsuite><testcase classname="" name="tests.test_a" file="tests/test_a.py"><error message="collection failure"/></testcase></testsuite></testsuites>`, "1", "pytest:tests/test_a.py"},
		"not a pytest unit":           {one, "0", "go:m/a"},
	} {
		t.Run(name, func(t *testing.T) {
			adapter, _ := pytestHarness(t, test.report, test.exit)
			if got, err := adapter.Run(context.Background(), t.TempDir(), []planEntry{{Unit: test.unit}}); err == nil {
				t.Fatalf("outcomes %v, want a failed measurement", got)
			}
		})
	}
}

// PYR-V1-010, pyr review P2: a parameter id holding `::` (test_x[a::b]) is
// one node segment, as pytest's report keys it, so the round trip through
// the selection and the JUnit report finds the executed test.
func TestPYRV1010PytestHarnessParameterWithSeparator(t *testing.T) {
	report := `<testsuites><testsuite name="pytest">
<testcase classname="tests.test_a" name="test_x[a::b]" file="tests/test_a.py" time="0.01"/>
</testsuite></testsuites>`
	adapter, dir := pytestHarness(t, report, "0")
	entries := []planEntry{{Unit: "pytest:tests/test_a.py", Run: []string{"test_x[a::b]"}}}
	got, err := adapter.Run(context.Background(), t.TempDir(), entries)
	if err != nil {
		t.Fatal(err)
	}
	if want := (RunResult{"pytest:tests/test_a.py#test_x[a::b]": Pass}); !reflect.DeepEqual(got, want) {
		t.Fatalf("outcomes %v, want %v", got, want)
	}
	if argv, _ := os.ReadFile(filepath.Join(dir, "argv")); !strings.HasSuffix(strings.TrimSpace(string(argv)), " tests/test_a.py::test_x[a::b]") {
		t.Fatalf("argv %q", argv)
	}
}
