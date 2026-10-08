package native

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// boostCase is one actual Boost 1.92.0 JUNIT log sink run retained in
// testdata/boosttest-provenance.json (TRE-V0-039). Report bytes are runner
// generated, never shaped by hand.
type boostCase struct {
	Name     string   `json:"name"`
	Mode     int      `json:"mode"`
	Args     []string `json:"args"`
	ExitCode int      `json:"exitCode"`
	Report   *string  `json:"report"`
	Stdout   string   `json:"stdout"`
	Stderr   string   `json:"stderr"`
}

type boostProvenance struct {
	Profile       string      `json:"profile"`
	ArchiveSHA256 string      `json:"archiveSHA256"`
	FixtureSource string      `json:"fixtureSource"`
	Version       int         `json:"boostVersionMacro"`
	Cases         []boostCase `json:"cases"`
}

func boostRecorded(t *testing.T) (boostProvenance, map[string]boostCase) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "boosttest-provenance.json"))
	if err != nil {
		t.Fatal(err)
	}
	var p boostProvenance
	if err = json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	if p.Profile != boostRunner || p.Version != 109200 || p.ArchiveSHA256 != "5c1d40cb8e19adbf740a4ec2da35b3e58f3f5804b1dce44deb53df72193cbc6c" {
		t.Fatal("provenance does not pin Boost 1.92.0", p.Profile, p.Version)
	}
	cases := map[string]boostCase{}
	for _, c := range p.Cases {
		cases[c.Name] = c
	}
	return p, cases
}

const boostTarget = "BoostProof"

func boostID(path string) string { return boostTarget + "::" + path }

func boostInventoryFor(mode int) []string {
	ids := []string{boostID("math/passes"), boostID("math/disabled_case"), boostID("top_level")}
	if mode != 1 {
		ids = append(ids, boostID("math/fails"))
	}
	switch mode {
	case 3:
		ids = append(ids, boostID("throws"))
	case 4:
		ids = append(ids, boostID("two_failures"), boostID("requires"))
	case 5:
		ids = append(ids, boostID("check_then_throw"))
	}
	return ids
}

func boostInput(t *testing.T, c boostCase) tr.Input {
	t.Helper()
	decode := func(s string) []byte {
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	in := tr.Input{Runner: boostRunner, Target: boostTarget, Expected: boostInventoryFor(c.Mode), Reports: map[string][]byte{}, Stdout: decode(c.Stdout), Stderr: decode(c.Stderr), ExitCode: c.ExitCode, SuccessExitCodes: []int{0}, FailureExitCodes: failureExits(boostRunner)}
	if c.Report != nil {
		in.Reports[boostReport] = decode(*c.Report)
	}
	for _, a := range c.Args {
		for _, p := range strings.Split(strings.TrimPrefix(a, "--run_test="), ":") {
			in.Selectors = append(in.Selectors, boostID(p))
		}
	}
	return in
}

func boostStates(o tr.Observation) map[string]string {
	states := map[string]string{}
	for _, v := range o.Tests {
		states[v.ID] = v.State
	}
	return states
}

// TestBoostTestRecordedJUnitWitnesses traces TRE-V0-037, TRE-V0-038 and TRE-V0-039.
func TestBoostTestRecordedJUnitWitnesses(t *testing.T) {
	_, cases := boostRecorded(t)
	type want struct {
		exit     int
		complete bool
		states   map[string]string
		problem  string
	}
	for name, w := range map[string]want{
		"pass-fail-disabled":        {201, true, map[string]string{"math/passes": tr.Passed, "math/fails": tr.Failed, "math/disabled_case": tr.Skipped, "top_level": tr.Passed}, ""},
		"pass-disabled":             {0, true, map[string]string{"math/passes": tr.Passed, "math/disabled_case": tr.Skipped, "top_level": tr.Passed}, ""},
		"suite-fixture-failure":     {201, false, map[string]string{"math/passes": tr.Passed, "math/fails": tr.Failed, "math/disabled_case": tr.Skipped, "top_level": tr.Passed}, "BOOST_SUITE_FIXTURE_FAILURE"},
		"uncaught-exception":        {201, false, map[string]string{"math/passes": tr.Passed, "math/fails": tr.Failed, "math/disabled_case": tr.Skipped, "top_level": tr.Passed, "throws": tr.Failed}, "BOOST_ERROR_ENTRY"},
		"selected-pass":             {0, true, map[string]string{"math/passes": tr.Passed, "math/fails": tr.Skipped, "math/disabled_case": tr.Skipped, "top_level": tr.Passed}, ""},
		"selected-disabled-enabled": {201, true, map[string]string{"math/passes": tr.Skipped, "math/fails": tr.Skipped, "math/disabled_case": tr.Failed, "top_level": tr.Skipped}, ""},
		// Two failure elements in one case, and a REQUIRE that Boost counts as aborted.
		"multiple-and-fatal-assertions": {201, true, map[string]string{"math/passes": tr.Passed, "math/fails": tr.Failed, "math/disabled_case": tr.Skipped, "top_level": tr.Passed, "two_failures": tr.Failed, "requires": tr.Failed}, ""},
		"check-then-exception":          {201, false, map[string]string{"math/passes": tr.Passed, "math/fails": tr.Failed, "math/disabled_case": tr.Skipped, "top_level": tr.Passed, "check_then_throw": tr.Failed}, "BOOST_ERROR_ENTRY"},
	} {
		c, ok := cases[name]
		if !ok || c.ExitCode != w.exit {
			t.Fatalf("%s: recorded case missing or exit drifted: %+v", name, c)
		}
		in := boostInput(t, c)
		o, err := Parse(in)
		if err != nil || o.Complete != w.complete || len(o.Tests) != len(w.states) {
			t.Fatalf("%s: %+v %v", name, o, err)
		}
		for path, state := range w.states {
			if got := boostStates(o)[boostID(path)]; got != state {
				t.Fatalf("%s: %s native state %s, want %s", name, path, got, state)
			}
		}
		for _, v := range o.Tests {
			if v.State == tr.Failed && (v.ID == boostID("math/fails") || v.ID == boostID("two_failures") || v.ID == boostID("requires")) && v.FailureKind != tr.Assertion {
				t.Fatalf("%s: assertion failure not classified: %+v", name, v)
			}
			if (v.ID == boostID("throws") || v.ID == boostID("check_then_throw")) && v.FailureKind != tr.Unknown {
				t.Fatalf("%s: aborted case given an ordinary cause: %+v", name, v)
			}
			if v.ID == boostID("math/passes") && v.Suite != boostID("math") || v.ID == boostID("top_level") && v.Suite != boostTarget {
				t.Fatalf("%s: suite path lost: %+v", name, v)
			}
		}
		if w.problem != "" && !hasBoostProblem(o, w.problem) {
			t.Fatalf("%s: missing %s: %+v", name, w.problem, o.Problems)
		}
		n := tr.Normalize(in, o)
		if n.Complete != w.complete {
			t.Fatalf("%s: normalized completeness %+v", name, n)
		}
		for _, v := range n.Tests {
			if !w.complete && v.State != tr.Unknown {
				t.Fatalf("%s: incomplete report exposed %s for %s", name, v.State, v.ID)
			}
			if v.State == tr.Passed && w.states[strings.TrimPrefix(v.ID, boostTarget+"::")] != tr.Passed {
				t.Fatalf("%s: invented pass %s", name, v.ID)
			}
		}
	}
	// exit_exception_failure with an empty sink: no-match filter.
	c := cases["selected-no-match"]
	if c.ExitCode != 200 || c.Report == nil || *c.Report != "" {
		t.Fatalf("no-match witness drifted: %+v", c)
	}
	in := boostInput(t, c)
	in.Expected = append(boostInventoryFor(0), boostID("math/nomatch"))
	o, err := Parse(in)
	if err == nil || o.Complete || len(o.Tests) != 0 || !hasBoostProblem(o, "BOOST_INVALID_REPORT") || tr.Normalize(in, o).Complete {
		t.Fatalf("empty exit-200 sink admitted: %+v %v", o, err)
	}
}

func hasBoostProblem(o tr.Observation, code string) bool {
	for _, p := range o.Problems {
		if p.Code == code {
			return true
		}
	}
	return false
}

// TestBoostTestBoundaryContradictions traces TRE-V0-037 and TRE-V0-038: every
// contradiction or unsupported shape yields uncertainty, never a pass.
func TestBoostTestBoundaryContradictions(t *testing.T) {
	_, cases := boostRecorded(t)
	base := func() tr.Input { return boostInput(t, cases["pass-fail-disabled"]) }
	report := func(in *tr.Input, old, new string) {
		s := string(in.Reports[boostReport])
		if !strings.Contains(s, old) {
			t.Fatalf("fixture lacks %q", old)
		}
		in.Reports[boostReport] = []byte(strings.Replace(s, old, new, 1))
	}
	for name, change := range map[string]func(*tr.Input){
		"tests counter":        func(in *tr.Input) { report(in, `tests="3"`, `tests="4"`) },
		"skipped counter":      func(in *tr.Input) { report(in, `skipped="1"`, `skipped="0"`) },
		"failures counter":     func(in *tr.Input) { report(in, `failures="1"`, `failures="0"`) },
		"errors counter":       func(in *tr.Input) { report(in, `errors="0"`, `errors="1"`) },
		"success exit":         func(in *tr.Input) { in.ExitCode = 0 },
		"exception exit":       func(in *tr.Input) { in.ExitCode = 200 },
		"failure exit no fail": func(in *tr.Input) { *in = boostInput(t, cases["pass-disabled"]); in.ExitCode = 201 },
		"surplus case":         func(in *tr.Input) { in.Expected = in.Expected[:len(in.Expected)-1] },
		"missing case":         func(in *tr.Input) { in.Expected = append(in.Expected, boostID("math/absent")) },
		"unselected executed":  func(in *tr.Input) { in.Selectors = []string{boostID("math/passes")} },
		"suite log":            func(in *tr.Input) { report(in, "</testsuite>", "<system-err>leak</system-err></testsuite>") },
		"system error type":    func(in *tr.Input) { report(in, `type="assertion error"`, `type="system error"`) },
		"error entry": func(in *tr.Input) {
			report(in, `<failure message="failure" type="assertion error">`, `<error message="failure" type="assertion error">`)
			report(in, `</failure>`, `</error>`)
		},
		"pseudo row": func(in *tr.Input) {
			report(in, `<testcase assertions="1" name="top_level"`, `<testcase assertions="1" name="boost_test-timed-execution" time="0"><failure message="m" type="execution timeout"/></testcase><testcase assertions="1" name="top_level"`)
		},
		"master suite":          func(in *tr.Input) { report(in, `name="BoostProof"`, `name="Other"`) },
		"suite id":              func(in *tr.Input) { report(in, `id="0"`, `id="1"`) },
		"suite extra attribute": func(in *tr.Input) { report(in, `id="0"`, `id="0" hostname="h"`) },
		"case extra attribute":  func(in *tr.Input) { report(in, `name="passes"`, `name="passes" file="x"`) },
		"case identity":         func(in *tr.Input) { report(in, `name="passes"`, `name="pass es"`) },
		"empty classname":       func(in *tr.Input) { report(in, `classname="math" name="passes"`, `classname="" name="passes"`) },
		"duplicate case":        func(in *tr.Input) { report(in, `name="top_level"`, `name="passes" classname="math"`) },
		"skipped and failure": func(in *tr.Input) {
			report(in, `<skipped/>`, `<skipped/><failure message="m" type="assertion error"/>`)
		},
		"skipped text":        func(in *tr.Input) { report(in, `<skipped/>`, `<skipped>why</skipped>`) },
		"unknown element":     func(in *tr.Input) { report(in, `<skipped/>`, `<flaky/>`) },
		"nested outcome":      func(in *tr.Input) { report(in, `<skipped/>`, `<skipped><x/></skipped>`) },
		"namespace":           func(in *tr.Input) { report(in, `<testsuite `, `<testsuite xmlns:j="urn:x" `) },
		"duplicate attribute": func(in *tr.Input) { report(in, `id="0"`, `id="0" id="0"`) },
		"comment":             func(in *tr.Input) { report(in, "</testsuite>", "<!-- c --></testsuite>") },
		"doctype":             func(in *tr.Input) { report(in, "<testsuite ", "<!DOCTYPE x><testsuite ") },
		"second root": func(in *tr.Input) {
			in.Reports[boostReport] = append(in.Reports[boostReport], []byte("<testsuite/>")...)
		},
		"time grammar":       func(in *tr.Input) { report(in, `time="0"`, `time="soon"`) },
		"counter grammar":    func(in *tr.Input) { report(in, `tests="3"`, `tests="03"`) },
		"assertions grammar": func(in *tr.Input) { report(in, `assertions="1"`, `assertions="-1"`) },
		"extra report":       func(in *tr.Input) { in.Reports["other.xml"] = []byte("<x/>") },
		"report missing":     func(in *tr.Input) { delete(in.Reports, boostReport) },
		"report renamed":     func(in *tr.Input) { in.Reports["junit.xml"] = in.Reports[boostReport]; delete(in.Reports, boostReport) },
		"fatal not aborted": func(in *tr.Input) {
			*in = boostInput(t, cases["multiple-and-fatal-assertions"])
			report(in, `type="fatal error"`, `type="assertion error"`)
		},
		"timed out":     func(in *tr.Input) { in.TimedOut = true },
		"interrupted":   func(in *tr.Input) { in.Interrupted = true },
		"caller target": func(in *tr.Input) { in.Target = "Other" },
	} {
		in := base()
		change(&in)
		o, err := Parse(in)
		if err == nil && o.Complete {
			t.Fatalf("%s admitted: %+v", name, o)
		}
		n := tr.Normalize(in, o)
		if n.Complete {
			t.Fatalf("%s normalized complete", name)
		}
		for _, v := range n.Tests {
			if v.State != tr.Unknown {
				t.Fatalf("%s exposed %s for %s", name, v.State, v.ID)
			}
		}
	}
}

// TestBoostTestClosedBuild traces TRE-V0-036.
func TestBoostTestClosedBuild(t *testing.T) {
	valid := func() tr.Request {
		return tr.Request{Runner: boostRunner, Executable: "/native-test", ReportDir: "/fresh", Target: boostTarget, ExpectedTests: []string{boostID("math/passes"), boostID("top_level"), boostID("math/fails")}}
	}
	fixed := []string{"--log_format=JUNIT", "--log_level=error", "--log_sink=/fresh/boost-junit.xml", "--report_level=no", "--random=0", "--result_code=yes", "--build_info=no", "--color_output=no", "--show_progress=no", "--catch_system_errors=yes", "--auto_start_dbg=no"}
	v, e := Build(valid())
	if e != nil || len(v.Phases) != 0 || strings.Join(v.Argv, " ") != strings.Join(fixed, " ") || v.Environment == nil || len(v.Environment) != 0 || len(v.Files) != 0 || strings.Join(v.ReportPaths, ",") != boostReport || len(v.SuccessExitCodes) != 1 || v.SuccessExitCodes[0] != 0 || len(v.FailureExitCodes) != 1 || v.FailureExitCodes[0] != 201 || len(v.OutcomeNeutralExitCodes) != 0 {
		t.Fatal(v, e)
	}
	r := valid()
	r.Selectors = []string{boostID("math/passes"), boostID("top_level")}
	v, e = Build(r)
	if e != nil || len(v.Argv) != len(fixed)+1 || v.Argv[len(fixed)] != "--run_test=math/passes:top_level" {
		t.Fatal(v, e)
	}
	long := valid()
	long.ExpectedTests = nil
	for i := 0; i < 300; i++ {
		long.ExpectedTests = append(long.ExpectedTests, boostID("suite_with_a_long_name/case_"+strings.Repeat("x", 8)+string(rune('a'+i%26))+strings.Repeat("y", i/26)))
	}
	long.Selectors = long.ExpectedTests
	if _, e = Build(long); e == nil {
		t.Fatal("oversized selector filter accepted")
	}
	for name, change := range map[string]func(*tr.Request){
		"target":             func(r *tr.Request) { r.Target = "Boost Proof" },
		"no expected":        func(r *tr.Request) { r.ExpectedTests = nil },
		"duplicate expected": func(r *tr.Request) { r.ExpectedTests = append(r.ExpectedTests, r.ExpectedTests[0]) },
		"foreign expected":   func(r *tr.Request) { r.ExpectedTests = []string{"Other::math/passes"} },
		"wildcard expected":  func(r *tr.Request) { r.ExpectedTests = []string{boostID("math/*")} },
		"empty segment":      func(r *tr.Request) { r.ExpectedTests = []string{boostID("math//passes")} },
		"deep path":          func(r *tr.Request) { r.ExpectedTests = []string{boostID("a/b/c/d/e/f/g/h/i")} },
		"selector outside":   func(r *tr.Request) { r.Selectors = []string{boostID("math/absent")} },
		"duplicate selector": func(r *tr.Request) { r.Selectors = []string{r.ExpectedTests[0], r.ExpectedTests[0]} },
		"project":            func(r *tr.Request) { r.Project = "other" },
		"config":             func(r *tr.Request) { r.Config = "override" },
		"config digest":      func(r *tr.Request) { r.ConfigSha256 = strings.Repeat("a", 64) },
		"reporter":           func(r *tr.Request) { r.Reporter = "override" },
		"reporter digest":    func(r *tr.Request) { r.ReporterSha256 = strings.Repeat("a", 64) },
		"report files":       func(r *tr.Request) { r.ReportFiles = []string{"elsewhere"} },
		"tools":              func(r *tr.Request) { r.Tools = map[string]tr.Tool{"other": {Executable: "/other"}} },
	} {
		r := valid()
		change(&r)
		if _, e := Build(r); e == nil {
			t.Fatalf("%s accepted: %+v", name, r)
		}
	}
}
