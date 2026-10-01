package native

import (
	"bytes"
	"encoding/json"
	"fmt"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"os"
	"path/filepath"
	"reflect"
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
		q := tr.Request{Runner: r, Executable: "/tool", ReportDir: "/out", Project: "fixture", Selectors: nil, Target: "fixture"}
		if q.Runner == cmockaRunner {
			q.Project = ""
			q.ExpectedTests = []string{q.Target + "::test_pass"}
		}
		mtpTestRequest(&q)
		v, e := Build(q)
		if e != nil || (len(v.Argv) == 0 && r != cmockaRunner) {
			t.Fatalf("%s: %v", r, e)
		}
	}
	for _, c := range []struct{ runner, selector string }{{"googletest", "X.*"}, {"catch2", "[all]"}, {"dotnet-vstest-nunit", "X|Y"}, {"go-test", "Test/Sub"}, {"nextest", "x) | all("}} {
		if _, e := Build(tr.Request{Runner: c.runner, Executable: "/tool", ReportDir: "/out", Project: "fixture", Selectors: []string{c.selector}}); e == nil {
			t.Fatal(c)
		}
	}
	v, e := Build(tr.Request{Runner: "nextest", Executable: "/tool", ReportDir: "/out", Project: "Cargo.toml"})
	if e != nil || !strings.Contains(string(v.Files["nextest.toml"]), "retries=0") {
		t.Fatal(v, e)
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
	o, e = Parse(tr.Input{Runner: "go-test", Stdout: []byte(`{"Action":"build-fail","Package":"p"}`), ExitCode: 1})
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
		q := tr.Request{Runner: runner, Executable: "/tools/runner", ReportDir: "/report", Project: "project", Target: "target"}
		if q.Runner == cmockaRunner {
			q.Project = ""
			q.ExpectedTests = []string{q.Target + "::test_pass"}
		}
		mtpTestRequest(&q)
		v, e := Build(q)
		if e != nil {
			t.Fatal(e)
		}
		if (len(v.Argv) == 0 && runner != cmockaRunner) || (len(v.Argv) > 0 && v.Argv[0] == "/tools/runner") {
			t.Fatal(runner, v.Argv)
		}
		if len(v.SuccessExitCodes) != 1 || v.SuccessExitCodes[0] != 0 || !reflect.DeepEqual(v.FailureExitCodes, failureExits(runner)) {
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
	if len(inventory) != 68 {
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
