package native

import (
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"strings"
	"testing"
)

const cmockaMixedXML = "<?xml version=\"1.0\" encoding=\"UTF-8\" ?>\n<testsuites>\n  <testsuite name=\"CProof\" time=\"0.000\" tests=\"3\" failures=\"1\" errors=\"0\" skipped=\"1\" >\n    <testcase name=\"test_pass\" time=\"0.000\" >\n    </testcase>\n    <testcase name=\"test_fail\" time=\"0.000\" >\n      <failure><![CDATA[7 != 9\n[   LINE   ] --- /private/tmp/cem10-build/stable-next/cmocka-profile-proposal/native/fixtures/probe.c:5: error: Failure!/private/tmp/cem10-build/stable-next/cmocka-profile-proposal/native/fixtures/probe.c:5: error: Failure!]]></failure>\n    </testcase>\n    <testcase name=\"test_skip\" time=\"0.000\" >\n      <skipped/>\n    </testcase>\n  </testsuite>\n</testsuites>\n"
const cmockaMixedOut = "[==========] CProof: Running 3 test(s).\n[ RUN      ] test_pass\n[       OK ] test_pass\n[ RUN      ] test_fail\n[  FAILED  ] test_fail\n[ RUN      ] test_skip\n[  SKIPPED ] test_skip\n[==========] CProof: 3 test(s) run.\n"
const cmockaMixedErr = "[  ERROR   ] --- 7 != 9\n[   LINE   ] --- /private/tmp/cem10-build/stable-next/cmocka-profile-proposal/native/fixtures/probe.c:5: error: Failure!/private/tmp/cem10-build/stable-next/cmocka-profile-proposal/native/fixtures/probe.c:5: error: Failure!\n[  PASSED  ] 1 test(s).\n[  SKIPPED ] CProof: 1 test(s), listed below:\n[  SKIPPED ] test_skip\n\n 1 SKIPPED TEST(S)\n[  FAILED  ] CProof: 1 test(s), listed below:\n[  FAILED  ] test_fail\n\n 1 FAILED TEST(S)\n"
const cmockaTeardownXML = "<?xml version=\"1.0\" encoding=\"UTF-8\" ?>\n<testsuites>\n  <testsuite name=\"CProof\" time=\"0.000\" tests=\"1\" failures=\"0\" errors=\"0\" skipped=\"0\" >\n    <testcase name=\"test_pass\" time=\"0.000\" >\n    </testcase>\n  </testsuite>\n</testsuites>\n"
const cmockaTeardownOut = "[==========] CProof: Running 1 test(s).\n[ RUN      ] test_pass\n[       OK ] test_pass\n[==========] CProof: 1 test(s) run.\n"
const cmockaTeardownErr = "[  FAILED  ] GROUP TEARDOWN\n[  ERROR   ] CProof\n[  PASSED  ] 1 test(s).\n"

func cmockaCapturedInput() tr.Input {
	return tr.Input{Runner: cmockaRunner, Target: "CProof", Expected: []string{"CProof::test_pass", "CProof::test_fail", "CProof::test_skip"}, Reports: map[string][]byte{"cmocka.xml": []byte(cmockaMixedXML)}, Stdout: []byte(cmockaMixedOut), Stderr: []byte(cmockaMixedErr), ExitCode: 1, SuccessExitCodes: []int{0}, FailureExitCodes: failureExits(cmockaRunner)}
}
func TestCMockaActualDualFormatWitnesses(t *testing.T) {
	in := cmockaCapturedInput()
	o, e := Parse(in)
	if e != nil || !o.Complete || len(o.Tests) != 3 {
		t.Fatal(o, e)
	}
	states := map[string]string{}
	for _, v := range o.Tests {
		states[v.ID] = v.State
		if v.State == tr.Failed && v.FailureKind != tr.Unknown {
			t.Fatal("invented failure cause", v)
		}
	}
	if states["CProof::test_pass"] != tr.Passed || states["CProof::test_fail"] != tr.Failed || states["CProof::test_skip"] != tr.Skipped {
		t.Fatal(states)
	}
	in.Reports["cmocka.xml"] = []byte(cmockaTeardownXML)
	in.Stdout = []byte(cmockaTeardownOut)
	in.Stderr = []byte(cmockaTeardownErr)
	in.ExitCode = 0
	in.Expected = []string{"CProof::test_pass"}
	o, e = Parse(in)
	if e != nil || o.Complete || len(o.Problems) == 0 || len(o.Tests) != 1 {
		t.Fatal(o, e)
	}
	normalized := tr.Normalize(in, o)
	if normalized.Complete || normalized.Tests[0].State != tr.Unknown {
		t.Fatal(normalized)
	}
}
func TestCMockaBoundaryContradictions(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*tr.Input)
	}{
		{"xml-only", func(i *tr.Input) { i.Stdout = nil; i.Stderr = nil }},
		{"wrong-target", func(i *tr.Input) { i.Target = "Other"; i.Expected = []string{"Other::test_pass"} }},
		{"surplus", func(i *tr.Input) { i.Expected = []string{"CProof::test_pass"} }},
		{"missing", func(i *tr.Input) { i.Expected = append(i.Expected, "CProof::absent") }},
		{"empty-expected", func(i *tr.Input) { i.Expected = nil }},
		{"native-exit", func(i *tr.Input) { i.ExitCode = 2 }},
		{"unknown-stdout", func(i *tr.Input) { i.Stdout = append(i.Stdout, []byte("unexpected\n")...) }},
		{"unknown-stderr", func(i *tr.Input) { i.Stderr = append(i.Stderr, []byte("unexpected\n")...) }},
		{"count", func(i *tr.Input) {
			i.Reports["cmocka.xml"] = []byte(strings.Replace(cmockaMixedXML, `tests="3"`, `tests="4"`, 1))
		}},
		{"error-count", func(i *tr.Input) {
			i.Reports["cmocka.xml"] = []byte(strings.Replace(strings.Replace(cmockaMixedXML, `failures="1"`, `failures="0"`, 1), `errors="0"`, `errors="1"`, 1))
		}},
		{"duplicate-id", func(i *tr.Input) {
			i.Reports["cmocka.xml"] = []byte(strings.Replace(cmockaMixedXML, `name="test_skip"`, `name="test_pass"`, 1))
		}},
		{"duplicate-attribute", func(i *tr.Input) {
			i.Reports["cmocka.xml"] = []byte(strings.Replace(cmockaMixedXML, `tests="3"`, `tests="3" tests="3"`, 1))
		}},
		{"multiple-root", func(i *tr.Input) { i.Reports["cmocka.xml"] = []byte(cmockaMixedXML + cmockaMixedXML) }},
		{"unknown-outcome", func(i *tr.Input) {
			i.Reports["cmocka.xml"] = []byte(strings.Replace(cmockaMixedXML, "<skipped/>", "<rerunFailure/>", 1))
		}},
		{"truncated", func(i *tr.Input) { i.Reports["cmocka.xml"] = []byte(cmockaMixedXML[:len(cmockaMixedXML)-20]) }},
		{"report-name", func(i *tr.Input) { i.Reports = map[string][]byte{"other.xml": []byte(cmockaMixedXML)} }},
		{"conflicting-outcomes", func(i *tr.Input) {
			i.Reports["cmocka.xml"] = []byte(strings.Replace(cmockaMixedXML, "<skipped/>", "<skipped/><failure/>", 1))
		}},
		{"namespace", func(i *tr.Input) {
			i.Reports["cmocka.xml"] = []byte(strings.Replace(cmockaMixedXML, "<testsuites>", `<testsuites xmlns="unknown">`, 1))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := cmockaCapturedInput()
			tc.change(&in)
			o, e := Parse(in)
			if e == nil && o.Complete {
				t.Fatal("contradiction admitted", o)
			}
			o = tr.Normalize(in, o)
			if o.Complete {
				t.Fatal("normalized contradiction complete")
			}
			for _, v := range o.Tests {
				if v.State != tr.Unknown {
					t.Fatal("resolved incomplete state", v)
				}
			}
		})
	}
}
func TestCMockaClosedBuild(t *testing.T) {
	valid := func() tr.Request {
		return tr.Request{Runner: cmockaRunner, Executable: "/native-test", ReportDir: "/fresh", Target: "CProof", ExpectedTests: []string{"CProof::test_pass"}, Selectors: []string{"CProof::test_pass"}}
	}
	v, e := Build(valid())
	if e != nil || len(v.Argv) != 0 || len(v.Phases) != 1 || v.Phases[0].Argv == nil || len(v.Phases[0].Argv) != 0 || len(v.Environment) != 0 || len(v.Phases[0].Environment) != 4 || v.Phases[0].Environment["CMOCKA_MESSAGE_OUTPUT"] != "STANDARD,XML" || v.Phases[0].Environment["CMOCKA_TEST_FILTER"] != "test_pass" || v.Phases[0].Environment["CMOCKA_XML_FILE"] != "/fresh/cmocka.xml" || len(v.FailureExitCodes) != 64 {
		t.Fatal(v, e)
	}
	for _, change := range []func(*tr.Request){func(r *tr.Request) { r.Target = "Other" }, func(r *tr.Request) { r.ExpectedTests = nil }, func(r *tr.Request) { r.ExpectedTests = append(r.ExpectedTests, r.ExpectedTests[0]) }, func(r *tr.Request) { r.Selectors = []string{"CProof::test_*"} }, func(r *tr.Request) { r.Selectors = append(r.Selectors, r.Selectors[0]) }, func(r *tr.Request) { r.Project = "other" }, func(r *tr.Request) { r.Config = "override" }, func(r *tr.Request) { r.Reporter = "override" }, func(r *tr.Request) { r.ReportFiles = []string{"elsewhere"} }, func(r *tr.Request) { r.Tools = map[string]tr.Tool{"other": {Executable: "/other"}} }} {
		r := valid()
		change(&r)
		if _, e := Build(r); e == nil {
			t.Fatal("override accepted", r)
		}
	}
}
