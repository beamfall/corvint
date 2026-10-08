package dynamic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

type jasmineFixture struct {
	SourceRoot string   `json:"sourceRoot"`
	Selectors  []string `json:"selectors"`
	ExitCode   int      `json:"exitCode"`
	SHA256     string   `json:"sha256"`
}

// jasmineFixtures loads the runner-generated Jasmine 7.0.0 reports and checks
// that their bytes and the reporter that wrote them still match provenance.
func jasmineFixtures(t *testing.T) map[string]jasmineFixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/jasmine/provenance.json")
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		ReporterSHA256 string                    `json:"reporterSha256"`
		Fixtures       map[string]jasmineFixture `json:"fixtures"`
	}
	if err = json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	reporter, _ := reporters.ReadFile("reporters/jasmine.cjs")
	if tr.Digest(reporter) != p.ReporterSHA256 {
		t.Fatal("embedded Jasmine reporter differs from the one that produced the fixtures")
	}
	if len(p.Fixtures) != 7 {
		t.Fatalf("unexpected Jasmine fixture inventory: %d", len(p.Fixtures))
	}
	return p.Fixtures
}

func jasmineInput(t *testing.T, name string, f jasmineFixture, report []byte) tr.Input {
	t.Helper()
	if report == nil {
		var err error
		if report, err = os.ReadFile(filepath.Join("testdata/jasmine", name)); err != nil {
			t.Fatal(err)
		}
		if tr.Digest(report) != f.SHA256 {
			t.Fatalf("%s bytes differ from provenance", name)
		}
	}
	inv, err := Build(tr.Request{Runner: "jasmine", Root: f.SourceRoot, Executable: "/x/jasmine.js", ReportDir: filepath.Join(f.SourceRoot, "reports"), Selectors: f.Selectors})
	if err != nil {
		t.Fatal(err)
	}
	return tr.Input{Runner: "jasmine", SourceRoot: f.SourceRoot, Selectors: f.Selectors, ExitCode: f.ExitCode, SuccessExitCodes: inv.SuccessExitCodes, FailureExitCodes: inv.FailureExitCodes, Reports: map[string][]byte{"jasmine.json": report}}
}

func problemCodes(o tr.Observation) map[string]bool {
	codes := map[string]bool{}
	for _, p := range o.Problems {
		codes[p.Code] = true
	}
	return codes
}

// TRE-V0-031: the invocation is fixed; only literal spec files are selectors.
func TestJasmineBuildIsFixed(t *testing.T) {
	req := tr.Request{Runner: "jasmine", Root: "/src", Executable: "/x/jasmine.js", ReportDir: "/r", Config: "spec/support/jasmine.json", Selectors: []string{"a.spec.js", "b/c.spec.mjs"}}
	inv, err := Build(req)
	if err != nil {
		t.Fatal(err)
	}
	reporter, _ := reporters.ReadFile("reporters/jasmine.cjs")
	want := []string{"--reporter=/r/jasmine.cjs", "--config=spec/support/jasmine.json", "a.spec.js", "b/c.spec.mjs"}
	if !reflect.DeepEqual(inv.Argv, want) || !reflect.DeepEqual(inv.ReportPaths, []string{"jasmine.json"}) || len(inv.ReportPatterns) != 0 ||
		!reflect.DeepEqual(inv.SuccessExitCodes, []int{0}) || !reflect.DeepEqual(inv.FailureExitCodes, []int{3}) ||
		len(inv.Files) != 1 || string(inv.Files["jasmine.cjs"]) != string(reporter) || len(inv.Environment) != 0 {
		t.Fatalf("%+v", inv)
	}
	req.Config, req.Selectors = "", nil
	if inv, err = Build(req); err != nil || !reflect.DeepEqual(inv.Argv, []string{"--reporter=/r/jasmine.cjs"}) {
		t.Fatalf("%+v %v", inv, err)
	}
	for _, s := range []string{"A=b.spec.js", `a\*.spec.js`, "--parallel=2", "-x", "*.spec.js", "../a.spec.js", "", "a.spec.js\n", "init", "examples", "help", "version", "enumerate", "-h", "-v"} {
		req.Selectors = []string{s}
		if _, err := Build(req); err == nil {
			t.Fatalf("selector %q admitted", s)
		}
	}
}

// TRE-V0-032, TRE-V0-033: actual Jasmine 7.0.0 reports keep native identity and
// never turn a failure, skip, hook error, focus, empty run or missing file into a pass.
func TestJasmineRunnerGeneratedReports(t *testing.T) {
	fixtures := jasmineFixtures(t)
	cases := []struct {
		name     string
		complete bool
		states   map[string]int
		codes    []string
	}{
		{"mixed.json", true, map[string]int{tr.Passed: 2, tr.Failed: 1, tr.Skipped: 2}, nil},
		{"passing.json", true, map[string]int{tr.Passed: 1}, nil},
		{"hooks.json", false, map[string]int{tr.Passed: 1, tr.Failed: 1}, []string{"jasmine-suite-error"}},
		{"global.json", false, map[string]int{tr.Passed: 1}, []string{"jasmine-global-error"}},
		{"focused.json", false, map[string]int{tr.Passed: 1, tr.Skipped: 1}, []string{"jasmine-incomplete"}},
		{"empty.json", false, map[string]int{}, []string{"jasmine-incomplete", "no-tests"}},
		{"missing.json", false, map[string]int{tr.Passed: 1}, []string{"jasmine-selector-without-specs"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := jasmineInput(t, c.name, fixtures[c.name], nil)
			o, err := Parse(in)
			if err != nil {
				t.Fatal(err)
			}
			if o.Complete != c.complete || o.RetryInformation != tr.NotApplicable {
				t.Fatalf("complete=%v retry=%s problems=%+v", o.Complete, o.RetryInformation, o.Problems)
			}
			states := map[string]int{}
			for _, x := range o.Tests {
				states[x.State]++
				if !strings.HasPrefix(x.ID, x.File+"::") || filepath.IsAbs(x.File) || len(x.Attempts) != 0 {
					t.Fatalf("identity %+v", x)
				}
			}
			if !reflect.DeepEqual(states, c.states) {
				t.Fatalf("states=%v tests=%+v", states, o.Tests)
			}
			codes := problemCodes(o)
			for _, code := range c.codes {
				if !codes[code] {
					t.Fatalf("missing %s in %+v", code, o.Problems)
				}
			}
			n := tr.Normalize(in, o)
			if n.Complete != c.complete {
				t.Fatalf("normalized complete=%v problems=%+v", n.Complete, n.Problems)
			}
			for _, x := range n.Tests {
				if !c.complete && x.State == tr.Passed {
					t.Fatalf("incomplete observation kept a pass: %+v", x)
				}
			}
		})
	}
	o, _ := Parse(jasmineInput(t, "mixed.json", fixtures["mixed.json"], nil))
	ids := map[string]string{}
	for _, x := range o.Tests {
		ids[x.ID] = x.Suite + "|" + x.Name
	}
	want := map[string]string{
		"fixture.spec.js::fixture passes":              "fixture|passes",
		"fixture.spec.js::fixture fails deliberately":  "fixture|fails deliberately",
		"fixture.spec.js::fixture is skipped with xit": "fixture|is skipped with xit",
		"fixture.spec.js::fixture is pending":          "fixture|is pending",
		"fixture.spec.js::fixture nested passes too":   "fixture nested|passes too",
	}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("identities %v", ids)
	}
}

// TRE-V0-032, TRE-V0-033: contradictions in the recorded events never pass.
func TestJasmineParserRefusals(t *testing.T) {
	fixtures := jasmineFixtures(t)
	base, err := os.ReadFile("testdata/jasmine/mixed.json")
	if err != nil {
		t.Fatal(err)
	}
	mutate := func(edit func(r map[string]any)) []byte {
		var r map[string]any
		if err := json.Unmarshal(base, &r); err != nil {
			t.Fatal(err)
		}
		edit(r)
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	specs := func(r map[string]any) []any { return r["specs"].([]any) }
	spec := func(r map[string]any, description string) map[string]any {
		for _, s := range specs(r) {
			if s.(map[string]any)["description"] == description {
				return s.(map[string]any)
			}
		}
		t.Fatal("no spec", description)
		return nil
	}
	suite := func(r map[string]any, id string) map[string]any {
		for _, s := range r["suites"].([]any) {
			if s.(map[string]any)["id"] == id {
				return s.(map[string]any)
			}
		}
		t.Fatal("no suite", id)
		return nil
	}
	f := fixtures["mixed.json"]
	refused := map[string]func(r map[string]any){
		"profile":         func(r map[string]any) { r["profile"] = "corvint-mocha/0" },
		"no done":         func(r map[string]any) { delete(r, "done") },
		"no parallel":     func(r map[string]any) { delete(r["started"].(map[string]any), "parallel") },
		"no total":        func(r map[string]any) { r["started"].(map[string]any)["totalSpecsDefined"] = nil },
		"no specs":        func(r map[string]any) { r["specs"] = nil },
		"duplicate suite": func(r map[string]any) { suite(r, "suite2")["id"] = "suite1" },
		"duplicate spec":  func(r map[string]any) { r["specs"] = append(specs(r), specs(r)[0]) },
	}
	for name, edit := range refused {
		if o, err := Parse(jasmineInput(t, "mixed.json", f, mutate(edit))); err == nil {
			t.Fatalf("%s accepted: %+v", name, o)
		}
	}
	incomplete := map[string]struct {
		code string
		edit func(r map[string]any)
		in   func(in *tr.Input)
	}{
		"parallel":        {"jasmine-parallel-unqualified", func(r map[string]any) { r["started"].(map[string]any)["parallel"] = true }, nil},
		"count":           {"jasmine-count-mismatch", func(r map[string]any) { r["started"].(map[string]any)["totalSpecsDefined"] = 6.0 }, nil},
		"unknown status":  {"unknown-state", func(r map[string]any) { spec(r, "is pending")["status"] = "retried" }, nil},
		"pass with error": {"jasmine-outcome-conflict", func(r map[string]any) { spec(r, "fails deliberately")["status"] = "passed" }, nil},
		"passed overall":  {"jasmine-status-conflict", func(r map[string]any) { r["done"].(map[string]any)["overallStatus"] = "passed" }, nil},
		"unknown overall": {"jasmine-unknown-overall-status", func(r map[string]any) { r["done"].(map[string]any)["overallStatus"] = "" }, nil},
		"full name":       {"jasmine-identity-conflict", func(r map[string]any) { spec(r, "fails deliberately")["fullName"] = "fixture  fails deliberately" }, nil},
		"unknown parent":  {"jasmine-identity-conflict", func(r map[string]any) { spec(r, "fails deliberately")["parentSuiteId"] = "suite9" }, nil},
		"suite cycle":     {"jasmine-identity-conflict", func(r map[string]any) { suite(r, "suite1")["parentSuiteId"] = "suite2" }, nil},
		"suite full name": {"jasmine-identity-conflict", func(r map[string]any) { suite(r, "suite2")["fullName"] = "fixture  nested" }, nil},
		"outside root":    {"jasmine-file-outside-root", func(r map[string]any) { spec(r, "fails deliberately")["filename"] = "/elsewhere/fixture.spec.js" }, nil},
		"unselected file": {"jasmine-unselected-file", func(r map[string]any) {
			spec(r, "fails deliberately")["filename"] = filepath.Join(f.SourceRoot, "other.spec.js")
		}, nil},
		"suite error": {"jasmine-suite-error", func(r map[string]any) { suite(r, "suite2")["status"] = "failed" }, nil},
		"global error": {"jasmine-global-error", func(r map[string]any) {
			r["done"].(map[string]any)["failedExpectations"] = []any{map[string]any{"message": "late"}}
		}, nil},
		"empty root":     {"jasmine-file-outside-root", func(map[string]any) {}, func(in *tr.Input) { in.SourceRoot = "" }},
		"other selector": {"jasmine-selector-without-specs", func(map[string]any) {}, func(in *tr.Input) { in.Selectors = []string{"fixture.spec.js", "other.spec.js"} }},
		"failed no reason": {"jasmine-status-conflict", func(r map[string]any) {
			s := spec(r, "fails deliberately")
			s["status"], s["failedExpectations"] = "pending", []any{}
		}, nil},
	}
	for name, c := range incomplete {
		in := jasmineInput(t, "mixed.json", f, mutate(c.edit))
		if c.in != nil {
			c.in(&in)
		}
		o, err := Parse(in)
		if err != nil || o.Complete || !problemCodes(o)[c.code] {
			t.Fatalf("%s: %v %+v", name, err, o.Problems)
		}
		for _, x := range tr.Normalize(in, o).Tests {
			if x.State == tr.Passed {
				t.Fatalf("%s kept a pass", name)
			}
		}
	}
	// Specs that did not run their assertions stay skipped, never passed.
	for _, status := range []string{"notApplicable", "excluded", "pending"} {
		o, err := Parse(jasmineInput(t, "mixed.json", f, mutate(func(r map[string]any) { spec(r, "is skipped with xit")["status"] = status })))
		if err != nil || !o.Complete {
			t.Fatalf("%s: %v %+v", status, err, o.Problems)
		}
		for _, x := range o.Tests {
			if strings.HasSuffix(x.ID, "is skipped with xit") && x.State != tr.Skipped {
				t.Fatalf("%s mapped to %s", status, x.State)
			}
		}
	}
	// Without selectors the configured spec set is not reconciled.
	in := jasmineInput(t, "mixed.json", f, nil)
	in.Selectors = nil
	if o, err := Parse(in); err != nil || !o.Complete {
		t.Fatalf("%v %+v", err, o.Problems)
	}
	// An absent report (load error before jasmineDone) is incomplete.
	in.Reports = nil
	in.ExitCode = 1
	if o, err := Parse(in); err != nil || o.Complete || !problemCodes(o)["missing-report"] {
		t.Fatalf("%v %+v", err, o)
	}
}

// TRE-V0-032: identity validation compares reported names in place, so a bounded
// report with a deep chain of long suite descriptions and many specs with
// inconsistent names cannot amplify into joined copies of the whole chain.
func TestJasmineIdentityValidationDoesNotAmplify(t *testing.T) {
	const depth = 4000
	long := strings.Repeat("d", 500)
	suites := make([]map[string]any, 0, depth)
	var parent any
	for i := range depth {
		id := "suite" + strconv.Itoa(i)
		suites = append(suites, map[string]any{"id": id, "description": long, "fullName": "x", "parentSuiteId": parent, "filename": "/src/a.spec.js", "status": "passed", "failedExpectations": []any{}})
		parent = id
	}
	specs := make([]map[string]any, 0, tr.MaxTests)
	for i := range tr.MaxTests {
		specs = append(specs, map[string]any{"id": "spec" + strconv.Itoa(i), "description": "y", "fullName": "x y" + strconv.Itoa(i), "parentSuiteId": parent, "filename": "/src/a.spec.js", "status": "passed", "failedExpectations": []any{}})
	}
	report, err := json.Marshal(map[string]any{"profile": jasmineProfile, "started": map[string]any{"totalSpecsDefined": tr.MaxTests, "parallel": false}, "suites": suites, "specs": specs, "done": map[string]any{"overallStatus": "passed", "failedExpectations": []any{}}})
	if err != nil || len(report) > tr.MaxReportBytes {
		t.Fatalf("report %d bytes: %v", len(report), err)
	}
	in := tr.Input{Runner: "jasmine", SourceRoot: "/src", SuccessExitCodes: []int{0}, FailureExitCodes: []int{3}, Reports: map[string][]byte{"jasmine.json": report}}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	o, err := Parse(in)
	runtime.ReadMemStats(&after)
	if err != nil || o.Complete || !problemCodes(o)["jasmine-identity-conflict"] {
		t.Fatalf("%v %v", err, problemCodes(o))
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 256<<20 {
		t.Fatalf("identity validation allocated %d bytes for a %d-byte report", allocated, len(report))
	}
}
