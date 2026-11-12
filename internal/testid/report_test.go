// SPDX-License-Identifier: Apache-2.0
package testid

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

// mapped reads one recorded report under testdata/ with read and returns
// each named test as its ID string, with " (reason)" when it has a reason.
func mapped(t *testing.T, name string, read func(*os.File) ([]Named, error)) []string {
	t.Helper()
	file, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	named, err := read(file)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	var got []string
	for _, n := range named {
		text := n.ID.String()
		if n.Reason != "" {
			text += " (" + n.Reason + ")"
		}
		if _, err := Parse(n.ID.String()); err != nil {
			t.Errorf("%s: %q does not parse: %v", name, n.ID.String(), err)
		}
		got = append(got, text)
	}
	return got
}

func wantIDs(t *testing.T, name string, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s maps to\n  %s\nwant\n  %s", name, strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// CTI-V1-003: a recorded `go test -json -count=1 ./...` run of module
// example.test/calc names each package directory and the test function
// followed by each subtest, split on `/` as go test reports it.
func TestCTIV1003GoTestJSON(t *testing.T) {
	t.Parallel()
	got := mapped(t, "testdata/go-test.jsonl", func(f *os.File) ([]Named, error) {
		return GoTestJSON(f, GoModuleDir("example.test/calc", "."))
	})
	wantIDs(t, "testdata/go-test.jsonl", got, []string{
		"go:internal/calc::TestAdd",
		"go:internal/calc::TestAdd::negative",
		"go:internal/calc::TestAdd::GET_::a%3Ab",
		"go:pkg/p::TestParse",
		"go:pkg/p::TestParse::empty",
		"go:pkg/p::TestParse::empty::utf8",
		"go:pkg/p::FuzzParse",
		"go:pkg/p::FuzzParse::seed#0",
		"go:pkg/p::ExampleParse",
	})
	if id, err := GoTest("pkg/p", "TestParse/empty/utf8"); err != nil || id.String() != "go:pkg/p::TestParse::empty::utf8" {
		t.Errorf("GoTest: %q %v", id, err)
	}
	if _, err := GoTestJSON(strings.NewReader(`{"Action":"run","Package":"other.test/x","Test":"TestX"}`), GoModuleDir("example.test/calc", ".")); err == nil {
		t.Error("a package outside the module was given a guessed directory")
	}
	dir := GoModuleDir("example.test/m", "tools/m")
	for importPath, want := range map[string]string{"example.test/m": "tools/m", "example.test/m/a/b": "tools/m/a/b", "example.test/mx": "", "example.test/m/../x": "", "other/x": ""} {
		if got, ok := dir(importPath); got != want || ok != (want != "") {
			t.Errorf("GoModuleDir(%q) = %q, %t; want %q", importPath, got, ok, want)
		}
	}
	if got, ok := GoModuleDir("example.test/m", ".")("example.test/m"); got != "." || !ok {
		t.Errorf("root package dir %q %t", got, ok)
	}
}

// CTI-V1-004: Vitest (a recorded JSON reporter run) and Jest (`--json`)
// name the test file under the repository root, then ancestorTitles, then
// the title as the runner interpolated it.
func TestCTIV1004VitestAndJestJSON(t *testing.T) {
	t.Parallel()
	got := mapped(t, "testdata/vitest-mixed.json", func(f *os.File) ([]Named, error) {
		return JestJSON(Vitest, f, "/private/tmp/corvint-public-release-20260912/js-fixture")
	})
	wantIDs(t, "testdata/vitest-mixed.json", got, []string{
		"vitest:math.test.js::add::adds two positive numbers",
		"vitest:math.test.js::add::adds a negative number",
		"vitest:math.test.js::add::is skipped on purpose",
	})
	got = mapped(t, "testdata/jest.json", func(f *os.File) ([]Named, error) { return JestJSON(Jest, f, "/repo/") })
	wantIDs(t, "testdata/jest.json", got, []string{
		"jest:web/sum.test.js::sum::adds",
		"jest:web/sum.test.js::sum::each::case 1",
		"jest:web/sum.test.js::sum::each::case 2",
		"jest:a.test.js::works",
		"jest:a.test.js::works",
	})
	if id, err := JestTest(Vitest, "src/a.test.ts", nil, "GET /a:b"); err != nil || id.String() != "vitest:src/a.test.ts::GET /a%3Ab" || id.Segments[0] != "GET /a:b" {
		t.Errorf("JestTest colon: %q %v", id, err)
	}
	if _, err := JestJSON(Playwright, strings.NewReader(`{"testResults":[{"name":"/repo/a.js","assertionResults":[{"title":"x"}]}]}`), "/repo"); err == nil {
		t.Error("a Playwright token was accepted for a Jest-shaped report")
	}
	if _, err := JestJSON(Jest, strings.NewReader(`{"testResults":[{"name":"/elsewhere/a.js","assertionResults":[{"title":"x"}]}]}`), "/repo"); err == nil {
		t.Error("a test file outside the repository root was accepted")
	}
}

// CTI-V1-005: Playwright names the spec file under the repository root
// (config.rootDir joined with the suite file), then the project name, empty
// in a recorded run whose configuration defines no projects, each describe
// title and the test title.
func TestCTIV1005PlaywrightJSON(t *testing.T) {
	t.Parallel()
	got := mapped(t, "testdata/playwright-mixed.json", func(f *os.File) ([]Named, error) {
		return PlaywrightJSON(f, "/private/tmp/corvint-public-release-20260912/js-fixture")
	})
	wantIDs(t, "testdata/playwright-mixed.json", got, []string{
		"playwright:e2e.spec.js::::increments the counter",
		"playwright:e2e.spec.js::::deliberately wrong workflow expectation",
		"playwright:e2e.spec.js::::skipped browser workflow",
		"playwright:e2e.spec.js::::flaky once then passes on retry",
	})
	got = mapped(t, "testdata/playwright-projects.json", func(f *os.File) ([]Named, error) { return PlaywrightJSON(f, "/repo") })
	wantIDs(t, "testdata/playwright-projects.json", got, []string{
		"playwright:e2e/login.spec.ts::chromium::signs in",
		"playwright:e2e/login.spec.ts::webkit::signs in",
		"playwright:e2e/login.spec.ts::chromium::logout::GET /a%3Ab",
	})
}

// CTI-V1-006: pytest names the node ID's file part, then each `::` part,
// never splitting inside the `[...]` parameter part, which is kept verbatim
// and then escaped.
func TestCTIV1006PytestNodeIDs(t *testing.T) {
	t.Parallel()
	got := mapped(t, "testdata/pytest.json", func(f *os.File) ([]Named, error) { return PytestJSON(f, ".") })
	wantIDs(t, "testdata/pytest.json", got, []string{
		"pytest:tests/test_a.py::TestA::test_b[x%3A%3Ay]",
		"pytest:tests/test_a.py::TestA::test_b[plain]",
		"pytest:tests/test_a.py::test_c",
	})
	for nodeID, want := range map[string]string{
		"tests/test_a.py::TestA::test_b[x::y]": "pytest:py/tests/test_a.py::TestA::test_b[x%3A%3Ay]",
		"t.py::test[a[b::c]d]":                 "pytest:py/t.py::test[a[b%3A%3Ac]d]",
		"t.py::TestA::TestInner::test_x":       "pytest:py/t.py::TestA::TestInner::test_x",
	} {
		if id, err := PytestNodeID("py", nodeID); err != nil || id.String() != want {
			t.Errorf("PytestNodeID(%q) = %q, %v; want %q", nodeID, id, err, want)
		}
	}
	for _, bad := range []string{"tests/test_a.py", "/abs.py::t", "../x.py::t"} {
		if _, err := PytestNodeID(".", bad); !errors.Is(err, ErrMalformed) {
			t.Errorf("PytestNodeID(%q) err %v", bad, err)
		}
	}
}

// CTI-V1-007, CTI-V1-011: Minitest names the method's source file, the
// class and the method from the Corvint hook's records; RSpec names the
// example's rerun file path (the including spec for a shared example), and
// because its JSON report carries no example group chain each example is
// its file-level ID with test-id-unmapped, never a guessed split of
// full_description.
func TestCTIV1007RSpecAndMinitest(t *testing.T) {
	t.Parallel()
	got := mapped(t, "testdata/minitest.jsonl", func(f *os.File) ([]Named, error) { return MinitestJSONL(f) })
	wantIDs(t, "testdata/minitest.jsonl", got, []string{
		"minitest:test/calc_test.rb::CalcTest::test_add",
		"minitest:test/calc_test.rb::Calc%3A%3AEdgeTest::test_zero",
	})
	got = mapped(t, "testdata/rspec.json", func(f *os.File) ([]Named, error) { return RSpecJSON(f) })
	wantIDs(t, "testdata/rspec.json", got, []string{
		"rspec:spec/user_spec.rb (test-id-unmapped)",
		"rspec:spec/user_spec.rb (test-id-unmapped)",
		"rspec:spec/user_spec.rb (test-id-unmapped)",
	})
	if id, err := RSpecExample("./spec/user_spec.rb", []string{"User", "validations"}, "rejects a blank name"); err != nil || id.String() != "rspec:spec/user_spec.rb::User::validations::rejects a blank name" {
		t.Errorf("RSpecExample: %q %v", id, err)
	}
	if id, err := MinitestMethod("test/calc_test.rb", "CalcTest", "test_add"); err != nil || id.String() != "minitest:test/calc_test.rb::CalcTest::test_add" {
		t.Errorf("MinitestMethod: %q %v", id, err)
	}
	if _, err := MinitestJSONL(strings.NewReader(`{"kind":"test","package":"spec/a_spec.rb","name":"./spec/a_spec.rb[1:1]"}`)); err == nil {
		t.Error("an RSpec hook record was read as a Minitest test")
	}
}
