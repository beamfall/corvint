package native

import (
	"bytes"
	"encoding/json"
	"fmt"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestLiveNativeReports(t *testing.T) {
	for _, c := range []struct {
		runner, file           string
		exit, pass, fail, skip int
	}{
		{"ctest", "ctest.xml", 8, 1, 1, 2}, {"googletest", "googletest.xml", 1, 1, 1, 2}, {"catch2", "catch2.xml", 42, 1, 1, 1},
		{"dotnet-vstest-xunit", "xunit.trx", 1, 1, 1, 1}, {"go-test", "go.json", 1, 1, 1, 1}, {"dotnet-vstest-nunit", "nunit.trx", 1, 1, 1, 1}, {"dotnet-vstest-mstest", "mstest.trx", 1, 1, 1, 1},
		{"nextest", "nextest.xml", 100, 1, 1, 1}, {"cargo-test", "cargo.txt", 101, 1, 1, 1}, {"cargo-integration", "integration.txt", 0, 1, 0, 0}, {"cargo-bin", "bin.txt", 0, 1, 0, 0}, {"cargo-doctest", "doctest.txt", 0, 1, 0, 0},
	} {
		t.Run(c.runner, func(t *testing.T) {
			b, e := readFixture("testdata", c.file)
			if e != nil {
				t.Fatal(e)
			}
			in := tr.Input{Runner: c.runner, ExitCode: c.exit, Reports: map[string][]byte{c.file: b}}
			if strings.HasPrefix(c.runner, "cargo") || c.runner == "go-test" {
				in.Stdout = b
				in.Reports = nil
			}
			o, e := Parse(in)
			if e != nil || !o.Complete {
				t.Fatalf("%+v %v", o, e)
			}
			counts := map[string]int{}
			for _, row := range o.Tests {
				counts[row.State]++
			}
			if counts[tr.Passed] != c.pass || counts[tr.Failed] != c.fail || counts[tr.Skipped] != c.skip {
				t.Fatal(counts)
			}
		})
	}
}
func TestRejectsIncompleteAndContradictoryReports(t *testing.T) {
	good := `<testsuite tests="1"><testcase name="a" status="run"/></testsuite>`
	for _, raw := range []string{`<testsuite tests="2"><testcase name="a"/></testsuite>`, `<testsuite tests="1"><testcase name="a" status="new"/></testsuite>`, `<testsuite tests="2"><testcase name="a"/><testcase name="a"/></testsuite>`, good + `<testsuite/>`, `<testsuite tests="1"><testcase name="a"><newFailure/></testcase></testsuite>`} {
		if _, e := Parse(tr.Input{Runner: "ctest", Reports: map[string][]byte{"ctest.xml": []byte(raw)}}); e == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	for _, in := range []tr.Input{{Runner: "ctest", Reports: map[string][]byte{"ctest.xml": []byte(good)}, ExitCode: 2}, {Runner: "ctest", Reports: map[string][]byte{"ctest.xml": []byte(`<testsuite tests="0"/>`)}}, {Runner: "ctest", Reports: map[string][]byte{"ctest.xml": []byte(good)}, TimedOut: true}, {Runner: "ctest", Reports: map[string][]byte{"ctest.xml": []byte(`<testsuite tests="1" errors="1"><testcase name="a"/></testsuite>`)}}} {
		o, e := Parse(in)
		if e != nil || o.Complete {
			t.Fatalf("%+v %v", o, e)
		}
	}
}
func TestBuildExactSelectorsAndFixedProfiles(t *testing.T) {
	for _, r := range Runners() {
		q := tr.Request{Runner: r, Executable: "/tool", Root: t.TempDir(), ReportDir: "/out", Project: "fixture", Selectors: nil, Target: "fixture"}
		if q.Runner == cmockaRunner {
			q.Project = ""
			q.ExpectedTests = []string{q.Target + "::test_pass"}
		}
		if q.Runner == ginkgoRunner {
			ginkgoTestRequest(t, &q)
		}
		if q.Runner == boostRunner {
			q.Project = ""
			q.ExpectedTests = []string{q.Target + "::suite/test_pass"}
		}
		mtpTestRequest(&q)
		v, e := Build(q)
		if e != nil || (len(v.Argv) == 0 && r != cmockaRunner) {
			t.Fatalf("%s: %v", r, e)
		}
	}
	for _, c := range []struct{ runner, selector string }{{"googletest", "X.*"}, {"catch2", "[all]"}, {"dotnet-vstest-nunit", "X|Y"}, {"go-test", "Test/Sub"}, {"nextest", "x) | all("}, {"cargo-doctest", "src/lib.rs - double (line 3)"}, {"cargo-doctest", "a\tb"}} {
		if _, e := Build(tr.Request{Runner: c.runner, Executable: "/tool", ReportDir: "/out", Project: "fixture", Selectors: []string{c.selector}}); e == nil {
			t.Fatal(c)
		}
	}
	v, e := Build(tr.Request{Runner: "nextest", Executable: "/tool", ReportDir: "/out", Project: "Cargo.toml"})
	if e != nil || !strings.Contains(string(v.Files["nextest.toml"]), "retries=0") {
		t.Fatal(v, e)
	}
}

// Actual Catch2 3.16.0 (AppleClang 21.0.0, macOS arm64) junit bytes from the
// V1-0593 live run catch2-skip, whose native exit was 4 (TRE-V0-046, TRE-V0-047).
const catch2AllSkippedReport = `<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
  <testsuite name="cprobe" errors="0" failures="0" skipped="1" tests="1" hostname="tbd" time="0.000" timestamp="2026-10-17T01:41:16Z">
    <properties>
      <property name="random-seed" value="2885424839"/>
      <property name="filters" value="&quot;&quot;skip&quot;&quot;"/>
    </properties>
    <testcase classname="cprobe.global" name="skip" time="0.000" status="run">
      <skipped type="SKIP">
SKIPPED
CATCH_SKIP
at probe.cc:7
      </skipped>
    </testcase>
    <system-out/>
    <system-err/>
  </testsuite>
</testsuites>
`

// TRE-V0-046: Catch2 reads an exclude: prefix even inside quotes, so the OR
// join must refuse it instead of widening the selection.
func TestCatch2RefusesExcludePrefix(t *testing.T) {
	for _, sel := range [][]string{{"pass", "exclude:fail"}, {"Exclude:x"}, {"EXCLUDE:x"}} {
		if _, e := Build(tr.Request{Runner: "catch2", Executable: "/tool", ReportDir: "/out", Project: "fixture", Selectors: sel}); e == nil || e.Error() != "ambiguous Catch2 selector" {
			t.Fatal(sel, e)
		}
	}
	if _, e := Build(tr.Request{Runner: "catch2", Executable: "/tool", ReportDir: "/out", Project: "fixture", Selectors: []string{"excluded name"}}); e != nil {
		t.Fatal(e)
	}
}

func TestCatch2ExactSelectionAndAllSkippedExit(t *testing.T) {
	v, e := Build(tr.Request{Runner: "catch2", Executable: "/tool", ReportDir: "/out", Project: "fixture", Selectors: []string{"pass", "fail"}})
	if e != nil || v.Argv[len(v.Argv)-1] != `"pass","fail"` || len(v.Argv) != 7 || !reflect.DeepEqual(v.SuccessExitCodes, []int{0, 4}) {
		t.Fatal(v.Argv, v.SuccessExitCodes, e)
	}
	skipped := tr.Input{Runner: "catch2", ExitCode: 4, SuccessExitCodes: v.SuccessExitCodes, FailureExitCodes: v.FailureExitCodes, Reports: map[string][]byte{"catch2.xml": []byte(catch2AllSkippedReport)}}
	o, e := Parse(skipped)
	if e != nil || !o.Complete || len(o.Tests) != 1 || o.Tests[0].State != tr.Skipped {
		t.Fatalf("%+v %v", o, e)
	}
	if n := tr.Normalize(skipped, o); !n.Complete || n.Tests[0].State != tr.Skipped {
		t.Fatalf("normalized %+v", n)
	}
	passed := strings.Replace(catch2AllSkippedReport, `skipped="1"`, `skipped="0"`, 1)
	passed = passed[:strings.Index(passed, "<skipped ")] + passed[strings.Index(passed, "</skipped>")+len("</skipped>"):]
	for _, in := range []tr.Input{{Runner: "catch2", ExitCode: 4, Reports: map[string][]byte{"catch2.xml": []byte(passed)}}, {Runner: "catch2", ExitCode: 4, Reports: map[string][]byte{"catch2.xml": []byte(`<testsuite tests="0"/>`)}}, {Runner: "ctest", ExitCode: 4, Reports: map[string][]byte{"ctest.xml": []byte(catch2AllSkippedReport)}}} {
		o, e := Parse(in)
		if e != nil || o.Complete {
			t.Fatalf("exit 4 accepted for %s: %+v %v", in.Runner, o, e)
		}
	}
}
func TestGoStructuredStreamAndBuildFailure(t *testing.T) {
	raw := `{"Action":"start","Package":"p"}
{"Action":"run","Package":"p","Test":"TestOne"}
{"Action":"pass","Package":"p","Test":"TestOne"}
{"Action":"pass","Package":"p"}`
	o, e := Parse(tr.Input{Runner: "go-test", Stdout: []byte(raw)})
	if e != nil || !o.Complete || len(o.Tests) != 1 {
		t.Fatal(o, e)
	}
	o, e = Parse(tr.Input{Runner: "go-test", Stdout: []byte(`{"ImportPath":"p [p.test]","Action":"build-fail"}`), ExitCode: 1})
	if e != nil || o.Complete {
		t.Fatal(o, e)
	}
	if _, e = Parse(tr.Input{Runner: "go-test", Stdout: []byte(`{"Action":"run","Package":"p","Test":"TestOne"}`)}); e == nil {
		t.Fatal("incomplete stream accepted")
	}
}
func TestCargoIncompleteAndCountMismatch(t *testing.T) {
	for _, raw := range []string{"", "running 1 test\ntest a ... ok\n", "running 1 test\ntest a ... ok\ntest result: ok. 2 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.01s\n"} {
		o, e := Parse(tr.Input{Runner: "cargo-test", Stdout: []byte(raw)})
		if e == nil && o.Complete {
			t.Fatal("accepted incomplete", raw)
		}
	}
}
func TestNextestNativeRetriesRetained(t *testing.T) {
	b, e := readFixture("testdata", "nextest-retry.xml")
	if e != nil {
		t.Fatal(e)
	}
	o, e := Parse(tr.Input{Runner: "nextest", Reports: map[string][]byte{"nextest.xml": b}, ExitCode: 100})
	if e != nil || !o.Complete || o.RetryInformation != tr.Retained {
		t.Fatal(o, e)
	}
	for _, row := range o.Tests {
		if row.Name == "fail" && len(row.Attempts) != 2 {
			t.Fatal(row)
		}
	}
}

func TestQualifiedIdentitySelectors(t *testing.T) {
	for _, c := range []struct{ runner, project, sel, want string }{{"go-test", "example.com/p", "example.com/p::TestPass", "^(TestPass)$"}, {"nextest", "Cargo.toml", "probe::pass", "(binary_id(=probe) & test(=pass))"}} {
		v, e := Build(tr.Request{Runner: c.runner, Executable: "/tool", ReportDir: "/out", Project: c.project, Selectors: []string{c.sel}})
		if e != nil || !strings.Contains(strings.Join(v.Argv, " "), c.want) {
			t.Fatal(v, e)
		}
	}
	for _, runner := range []string{"go-test", "nextest"} {
		if _, e := Build(tr.Request{Runner: runner, Executable: "/tool", ReportDir: "/out", Project: "fixture", Selectors: []string{"Pass"}}); e == nil {
			t.Fatal("ambiguous unqualified selector accepted")
		}
	}
}

func TestReviewRejectsGoogleTestSkippedAndDisabledContradictions(t *testing.T) {
	const root = `<testsuites tests="1" failures="0" disabled="0" errors="0"><testsuite name="A" tests="1" failures="0" disabled="0" skipped="1" errors="0">%s</testsuite></testsuites>`
	for _, body := range []string{`<testcase classname="A" name="skipped" status="run" result="skipped"/>`, `<testcase classname="A" name="skipped" status="run" result="completed"/>`, `<testcase classname="A" name="skipped" status="run" result="suppressed"/>`} {
		o, e := Parse(tr.Input{Runner: "googletest", Reports: map[string][]byte{"googletest.xml": []byte(fmt.Sprintf(root, body))}})
		if e == nil && o.Complete {
			t.Fatalf("accepted contradictory native state: %+v", o)
		}
	}
	b, e := readFixture("testdata", "googletest.xml")
	if e != nil {
		t.Fatal(e)
	}
	for _, pair := range [][2]string{{`disabled="1"`, `disabled="0"`}, {`skipped="1"`, `skipped="0"`}} {
		bad := bytes.Replace(b, []byte(pair[0]), []byte(pair[1]), 1)
		o, e := Parse(tr.Input{Runner: "googletest", ExitCode: 1, Reports: map[string][]byte{"googletest.xml": bad}})
		if e == nil && o.Complete {
			t.Fatal("accepted contradictory count")
		}
	}
}

func TestReviewTRXSummaryAndCountersCannotBecomePass(t *testing.T) {
	b, e := readFixture("testdata", "nunit-pass.trx")
	if e != nil {
		t.Fatal(e)
	}
	for _, pair := range [][2]string{{`outcome="Completed"`, `outcome="Error"`}, {`executed="1"`, `executed="0"`}, {`passed="1"`, `passed="0"`}, {`error="0"`, `error="1"`}, {`pending="0"`, `pending="1"`}} {
		raw := bytes.Replace(b, []byte(pair[0]), []byte(pair[1]), 1)
		o, e := Parse(tr.Input{Runner: "dotnet-vstest-nunit", Reports: map[string][]byte{"results.trx": raw}})
		if e == nil && o.Complete {
			t.Fatalf("accepted contradictory %s: %+v", pair[0], o)
		}
	}
	o, e := Parse(tr.Input{Runner: "dotnet-vstest-nunit", Reports: map[string][]byte{"results.trx": b}})
	if e != nil || !o.Complete {
		t.Fatal(o, e)
	}
}

func TestReviewNextestCrashRetriesRetainUnknownCausality(t *testing.T) {
	raw := `<testsuites tests="1" failures="1"><testsuite name="binary" tests="1" failures="1"><testcase classname="binary" name="crash"><failure type="test abort with signal 11" message="first segmentation fault"/><rerunFailure type="test abort with signal 11" message="second segmentation fault"/></testcase></testsuite></testsuites>`
	o, e := Parse(tr.Input{Runner: "nextest", ExitCode: 100, Reports: map[string][]byte{"nextest.xml": []byte(raw)}})
	if e != nil || !o.Complete || len(o.Tests) != 1 || len(o.Tests[0].Attempts) != 2 {
		t.Fatal(o, e)
	}
	if o.Tests[0].FailureKind != tr.Infrastructure || !strings.Contains(o.Tests[0].Message, "first segmentation fault") {
		t.Fatal(o.Tests[0])
	}
	for i, a := range o.Tests[0].Attempts {
		if a.FailureKind != tr.Infrastructure || !strings.Contains(a.Message, "test abort with signal 11") || !strings.Contains(a.Message, []string{"first", "second"}[i]) {
			t.Fatal(a)
		}
	}
	b, e := readFixture("testdata", "nextest-retry.xml")
	if e != nil {
		t.Fatal(e)
	}
	o, e = Parse(tr.Input{Runner: "nextest", ExitCode: 100, Reports: map[string][]byte{"nextest.xml": b}})
	if e != nil {
		t.Fatal(e)
	}
	for _, row := range o.Tests {
		for _, a := range row.Attempts {
			if a.FailureKind == tr.Assertion {
				t.Fatal("exit code 101 alone was classified as assertion")
			}
		}
	}
}

func TestReviewCargoAnnouncedInventory(t *testing.T) {
	for _, header := range []string{"running 2 tests", "running nonsense tests", "running 99999999999999999999999999 tests"} {
		raw := header + "\ntest only ... ok\ntest result: ok. 1 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.01s\n"
		o, e := Parse(tr.Input{Runner: "cargo-test", Stdout: []byte(raw)})
		if e == nil && o.Complete {
			t.Fatal("incomplete inventory admitted", header)
		}
	}
}

func TestReviewGoDuplicateFieldsAndTerminals(t *testing.T) {
	for _, terminal := range []string{`{"Action":"fail","Action":"pass","Package":"p","Test":"T"}`, `{"Action":"fail","action":"pass","Package":"p","Test":"T"}`, `{"Action":"fail","\u0041ction":"pass","Package":"p","Test":"T"}`, `{"Action":"pass","Package":"q","Package":"p","Test":"T"}`} {
		raw := "{\"Action\":\"run\",\"Package\":\"p\",\"Test\":\"T\"}\n" + terminal + "\n{\"Action\":\"pass\",\"Package\":\"p\"}\n"
		o, e := Parse(tr.Input{Runner: "go-test", Stdout: []byte(raw)})
		if e == nil && o.Complete {
			t.Fatalf("duplicate structural field admitted: %+v", o)
		}
	}
	raw := []byte("{\"Action\":\"run\",\"Package\":\"p\",\"Test\":\"T\"}\n{\"Action\":\"pass\",\"Package\":\"p\",\"Test\":\"T\"}\n{\"Action\":\"fail\",\"Package\":\"p\"}\n{\"Action\":\"pass\",\"Package\":\"p\"}\n")
	if o, e := Parse(tr.Input{Runner: "go-test", Stdout: raw}); e == nil && o.Complete {
		t.Fatal("duplicate package terminal admitted")
	}
}

func TestReviewBuildUsesArgsOnlyAndNativeExitCodes(t *testing.T) {
	for _, runner := range Runners() {
		q := tr.Request{Runner: runner, Executable: "/tools/runner", Root: t.TempDir(), ReportDir: "/report", Project: "project", Target: "target"}
		if q.Runner == cmockaRunner {
			q.Project = ""
			q.ExpectedTests = []string{q.Target + "::test_pass"}
		}
		if q.Runner == ginkgoRunner {
			ginkgoTestRequest(t, &q)
		}
		if q.Runner == boostRunner {
			q.Project = ""
			q.ExpectedTests = []string{q.Target + "::suite/test_pass"}
		}
		mtpTestRequest(&q)
		v, e := Build(q)
		if e != nil {
			t.Fatal(e)
		}
		if (len(v.Argv) == 0 && runner != cmockaRunner) || (len(v.Argv) > 0 && v.Argv[0] == "/tools/runner") {
			t.Fatal(runner, v.Argv)
		}
		success := []int{0}
		if runner == "catch2" {
			success = []int{0, catch2AllSkippedExit} // TRE-V0-047
		}
		if !reflect.DeepEqual(v.SuccessExitCodes, success) || !reflect.DeepEqual(v.FailureExitCodes, failureExits(runner)) {
			t.Fatal(runner, v)
		}
	}
	b, e := readFixture("testdata", "ctest.xml")
	if e != nil {
		t.Fatal(e)
	}
	o, e := Parse(tr.Input{Runner: "ctest", ExitCode: 1, Reports: map[string][]byte{"ctest.xml": b}})
	if e != nil || o.Complete {
		t.Fatal("CTest admitted nonnative failed-test exit", o, e)
	}
}

// Report bundles preserve original capture bytes; undeclared names never become empty reports.
func readFixture(root, name string) ([]byte, error) {
	raw, err := os.ReadFile(filepath.Join(root, "native-report-fixtures.json"))
	if err != nil {
		return nil, err
	}
	var inventory map[string]struct {
		Bytes  []byte `json:"bytesBase64"`
		SHA256 string `json:"sha256"`
	}
	if err := json.Unmarshal(raw, &inventory); err != nil {
		return nil, err
	}
	entry, declared := inventory[filepath.ToSlash(name)]
	if !declared {
		return nil, fmt.Errorf("native report fixture not declared: %s", name)
	}
	if entry.Bytes == nil || entry.SHA256 != tr.Digest(entry.Bytes) {
		return nil, fmt.Errorf("native report fixture digest mismatch: %s", name)
	}
	return entry.Bytes, nil
}

func TestNativeReportFixtureInventory(t *testing.T) {
	raw, err := os.ReadFile("testdata/native-report-fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var inventory map[string]json.RawMessage
	if err := json.Unmarshal(raw, &inventory); err != nil {
		t.Fatal(err)
	}
	if len(inventory) != 69 {
		t.Fatalf("unexpected report inventory: %d", len(inventory))
	}
	for name := range inventory {
		if _, err := readFixture("testdata", name); err != nil {
			t.Fatalf("fixture %s: %v", name, err)
		}
	}
	if _, err := readFixture("testdata", "undeclared-missing-fixture.txt"); err == nil {
		t.Fatal("undeclared missing fixture was hidden")
	}
	for _, declaration := range []string{`{"bytesBase64":"YQ==","sha256":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}`, `{"bytesBase64":"","sha256":"wrong"}`, `{"sha256":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}`} {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "native-report-fixtures.json"), []byte(`{"capture":`+declaration+`}`), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readFixture(root, "capture"); err == nil {
			t.Fatal("invalid declaration accepted")
		}
	}
}

// V1-1375: real go1.27 `go test -json` bytes for a package that fails to compile carry
// BuildEvents keyed by ImportPath with no Package (GLTP-V0-014). They must decode into a
// BUILD_FAILURE that names the import path, never an assertion failure or a pass (TRE-V0-003).
func TestGoActualBuildFailureStream(t *testing.T) {
	b, err := readFixture("testdata", "go-build-fail.json")
	if err != nil {
		t.Fatal(err)
	}
	o, err := Parse(tr.Input{Runner: "go-test", ExitCode: 1, Stdout: b})
	if err != nil {
		t.Fatalf("actual build-fail stream refused: %v", err)
	}
	if o.Complete || len(o.Tests) != 0 {
		t.Fatalf("build failure admitted as complete or as tests: %+v", o)
	}
	want := map[string]bool{
		"BUILD_FAILURE example.com/buildfailprobe [example.com/buildfailprobe.test]": true,
		"BUILD_FAILURE example.com/buildfailprobe":                                   true,
		"GO_PACKAGE_FAILURE example.com/buildfailprobe":                              true,
		"NO_TESTS native runner reported no tests":                                   true,
		"EXIT_REPORT_CONTRADICTION exit 1 does not agree with native cases":          true,
	}
	got := map[string]bool{}
	for _, p := range o.Problems {
		got[p.Code+" "+p.Detail] = true
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("problems = %v, want %v", got, want)
	}
	const build = `{"ImportPath":"p [p.test]","Action":"build-output","Output":"x\n"}` + "\n"
	const fail = `{"ImportPath":"p [p.test]","Action":"build-fail"}` + "\n"
	for name, raw := range map[string]string{
		"build event with Package":       `{"ImportPath":"p","Package":"p","Action":"build-fail"}`,
		"build event without ImportPath": `{"Package":"p","Action":"build-fail"}`,
		"build event with Test":          `{"ImportPath":"p","Test":"T","Action":"build-output"}`,
		"test event with ImportPath":     `{"ImportPath":"p","Package":"p","Action":"start"}`,
		"test event without Package":     `{"Action":"start"}`,
		"duplicate ImportPath field":     `{"ImportPath":"p","importpath":"q","Action":"build-fail"}`,
		"unknown build action":           `{"ImportPath":"p","Action":"build-pass"}`,
		"unknown test event action":      `{"Package":"p","Action":"build-pass"}`,
		"build event with empty Package": `{"ImportPath":"p","Package":"","Action":"build-output"}`,
		"build event with null Test":     `{"ImportPath":"p","Test":null,"Action":"build-output"}`,
		"build event with FailedBuild":   `{"ImportPath":"p","FailedBuild":"","Action":"build-fail"}`,
		"build event with Elapsed":       `{"ImportPath":"p","Elapsed":-1,"Action":"build-output"}`,
		"build event with lowercase key": `{"importpath":"p","Action":"build-output"}`,
		"build output after build-fail":  build + fail + build,
		"duplicate build-fail":           fail + fail,
	} {
		if o, err := Parse(tr.Input{Runner: "go-test", ExitCode: 1, Stdout: []byte(raw)}); err == nil {
			t.Errorf("%s admitted: %+v", name, o)
		}
	}
}

// V1-1537 (TRE-V0-111): CTest runs from a run-owned test directory that only
// includes the caller's build tree; a build path CMake would re-parse is
// refused rather than quoted.
func TestCTestRunsFromRunOwnedTestDirectory(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(t.TempDir(), "out")
	v, err := Build(tr.Request{Runner: "ctest", Executable: "/tool", Root: root, ReportDir: out})
	if err != nil {
		t.Fatal(err)
	}
	slashed := filepath.ToSlash(root)
	if got := string(v.Files[ctestRunDir+"/CTestTestfile.cmake"]); got != "subdirs(\""+slashed+"\")\n" {
		t.Fatalf("run-owned CTestTestfile %q", got)
	}
	// TRE-V0-113: the build tree's CTestCustom file is included with CTest's
	// .cmake-before-.ctest precedence.
	custom := string(v.Files[ctestRunDir+"/CTestCustom.cmake"])
	if !strings.Contains(custom, "include(\""+slashed+"/CTestCustom.cmake\")") || strings.Index(custom, ".cmake\")") > strings.Index(custom, ".ctest\")") {
		t.Fatalf("run-owned CTestCustom %q", custom)
	}
	if _, ok := v.Files[ctestRunDir+"/DartConfiguration.tcl"]; ok {
		t.Fatal("DartConfiguration.tcl invented for a build tree without one")
	}
	if len(v.Argv) < 2 || v.Argv[0] != "--test-dir" || v.Argv[1] != filepath.Join(out, ctestRunDir) {
		t.Fatalf("argv %q", v.Argv)
	}
	refused := []string{"", "build", filepath.Join(root, `a"b`), filepath.Join(root, "a$b"), filepath.Join(root, "a;b"), filepath.Join(root, "a\nb")}
	if runtime.GOOS != "windows" {
		refused = append(refused, filepath.Join(root, `a\b`))
	}
	for _, bad := range refused {
		if _, err := Build(tr.Request{Runner: "ctest", Executable: "/tool", Root: bad, ReportDir: out}); err == nil {
			t.Fatalf("root %q admitted", bad)
		}
	}
}

// TRE-V0-113: a Windows build directory reaches CMake with forward slashes.
func TestCTestWindowsRootUsesForwardSlashes(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows path separators")
	}
	root := t.TempDir()
	v, err := Build(tr.Request{Runner: "ctest", Executable: `C:\tool`, Root: root, ReportDir: filepath.Join(root, "out")})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(v.Files[ctestRunDir+"/CTestTestfile.cmake"]); strings.Contains(got, `\`) || !strings.Contains(got, filepath.ToSlash(root)) {
		t.Fatalf("run-owned CTestTestfile %q", got)
	}
}

// TRE-V0-113: DartConfiguration.tcl is carried with only BuildDirectory
// relocated; a copy that cannot be relocated exactly is refused.
func TestCTestCarriesDartConfiguration(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(t.TempDir(), "out")
	dart := "SourceDirectory: /src\r\nBuildDirectory: " + filepath.ToSlash(root) + "\r\nTimeOut: 7\r\n"
	if err := os.WriteFile(filepath.Join(root, "DartConfiguration.tcl"), []byte(dart), 0o600); err != nil {
		t.Fatal(err)
	}
	v, err := Build(tr.Request{Runner: "ctest", Executable: "/tool", Root: root, ReportDir: out})
	if err != nil {
		t.Fatal(err)
	}
	want := "SourceDirectory: /src\r\nBuildDirectory: " + filepath.ToSlash(filepath.Join(out, ctestRunDir)) + "\r\nTimeOut: 7\r\n"
	if got := string(v.Files[ctestRunDir+"/DartConfiguration.tcl"]); got != want {
		t.Fatalf("relocated DartConfiguration %q, want %q", got, want)
	}
	for _, bad := range []string{"TimeOut: 7\n", "BuildDirectory: /a\nBuildDirectory: /b\n"} {
		if err := os.WriteFile(filepath.Join(root, "DartConfiguration.tcl"), []byte(bad), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Build(tr.Request{Runner: "ctest", Executable: "/tool", Root: root, ReportDir: out}); err == nil {
			t.Fatalf("DartConfiguration %q admitted", bad)
		}
	}
	unreadable := t.TempDir()
	if err := os.Mkdir(filepath.Join(unreadable, "DartConfiguration.tcl"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(tr.Request{Runner: "ctest", Executable: "/tool", Root: unreadable, ReportDir: out}); err == nil {
		t.Fatal("unreadable DartConfiguration admitted")
	}
}
