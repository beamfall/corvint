// Package native adapts explicit native runner profiles. It does not establish
// dependency closure or authorize execution of a project or test binary.
package native

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

func Runners() []string {
	return []string{"cmocka-xml", "ginkgo-v2", "go-test", "ctest", "googletest", "catch2", "dotnet-vstest-nunit", "dotnet-vstest-mstest", "dotnet-vstest-xunit", "cargo-test", "cargo-doctest", "cargo-integration", "cargo-bin", "nextest", "dotnet-mtp-nunit", "dotnet-mtp-mstest", "dotnet-mtp-xunit"}
}
func known(r string) bool {
	for _, v := range Runners() {
		if r == v {
			return true
		}
	}
	return false
}
func Build(r tr.Request) (tr.Invocation, error) {
	v := tr.Invocation{Format: r.Runner, Environment: map[string]string{}, SuccessExitCodes: []int{0}, FailureExitCodes: failureExits(r.Runner)}
	if !known(r.Runner) {
		return v, errors.New("unsupported native runner")
	}
	if r.Executable == "" || r.ReportDir == "" {
		return v, errors.New("executable and fresh report directory required")
	}
	for _, s := range r.Selectors {
		if s == "" || strings.ContainsAny(s, "\x00\r\n") {
			return v, errors.New("invalid selector")
		}
	}
	report := func(name string) string { v.ReportPaths = []string{name}; return filepath.Join(r.ReportDir, name) }
	switch r.Runner {
	case cmockaRunner:
		return buildCMocka(r)
	case ginkgoRunner:
		return buildGinkgo(r)
	case "dotnet-mtp-nunit", "dotnet-mtp-mstest", "dotnet-mtp-xunit":
		return buildMTP(r, v)
	case "go-test":
		v.Environment["GOTOOLCHAIN"] = "local"
		v.Environment["GOPROXY"] = "off"
		v.Environment["GOSUMDB"] = "off"
		if r.Project == "" || strings.HasPrefix(r.Project, "-") || strings.Contains(r.Project, "...") {
			return v, errors.New("one explicit Go package required")
		}
		v.Argv = []string{"test", "-json", "-count=1"}
		if len(r.Selectors) > 0 {
			var names []string
			for _, sel := range r.Selectors {
				pkg, name, ok := strings.Cut(sel, "::")
				if !ok || pkg != r.Project || name == "" || strings.Contains(name, "/") {
					return v, errors.New("Go selectors require exact Project::TestName without subtest segments")
				}
				names = append(names, name)
			}
			v.Argv = append(v.Argv, "-run", exactRegex(names))
		}
		v.Argv = append(v.Argv, r.Project)
	case "ctest":
		v.Argv = []string{"--output-junit", report("ctest.xml"), "--no-tests=error"}
		if len(r.Selectors) > 0 {
			v.Argv = append(v.Argv, "-R", exactRegex(r.Selectors))
		}
	case "googletest":
		v.Argv = []string{"--gtest_output=xml:" + report("googletest.xml"), "--gtest_repeat=1", "--gtest_shuffle=0"}
		for _, s := range r.Selectors {
			if strings.ContainsAny(s, "*?:-") {
				return v, errors.New("ambiguous GoogleTest selector")
			}
		}
		if len(r.Selectors) > 0 {
			v.Argv = append(v.Argv, "--gtest_filter="+strings.Join(r.Selectors, ":"))
		}
	case "catch2":
		v.Argv = []string{"--reporter", "junit", "--out", report("catch2.xml"), "--order", "lex"}
		for _, s := range r.Selectors {
			if strings.ContainsAny(s, "*?,[]~\\\"") || strings.HasPrefix(s, "-") {
				return v, errors.New("ambiguous Catch2 selector")
			}
			v.Argv = append(v.Argv, "\""+s+"\"")
		}
	case "dotnet-vstest-nunit", "dotnet-vstest-mstest", "dotnet-vstest-xunit":
		if r.Project == "" || strings.HasPrefix(r.Project, "-") {
			return v, errors.New("explicit test project or assembly required")
		}
		report("results.trx")
		v.Argv = []string{"test", r.Project, "--no-build", "--no-restore", "--logger", "trx;LogFileName=results.trx", "--results-directory", r.ReportDir}
		var filters []string
		for _, s := range r.Selectors {
			if strings.ContainsAny(s, "&|!~=()\\\" ") {
				return v, errors.New("ambiguous VSTest selector")
			}
			filters = append(filters, "FullyQualifiedName="+s)
		}
		if len(filters) > 0 {
			v.Argv = append(v.Argv, "--filter", strings.Join(filters, "|"))
		}
	case "cargo-test", "cargo-doctest", "cargo-integration", "cargo-bin":
		if r.Project == "" {
			return v, errors.New("explicit Cargo manifest required")
		}
		if len(r.Selectors) > 1 {
			return v, errors.New("Cargo exact profile admits one selector per invocation")
		}
		mode := "--lib"
		if r.Runner == "cargo-doctest" {
			mode = "--doc"
		}
		v.Argv = []string{"test", "--offline", "--locked", "--manifest-path", r.Project}
		if r.Runner == "cargo-integration" || r.Runner == "cargo-bin" {
			if r.Target == "" || strings.HasPrefix(r.Target, "-") || strings.ContainsAny(r.Target, "/\\*?[] ") {
				return v, errors.New("literal Cargo target required")
			}
			mode = "--test"
			if r.Runner == "cargo-bin" {
				mode = "--bin"
			}
			v.Argv = append(v.Argv, mode, r.Target)
		} else {
			v.Argv = append(v.Argv, mode)
		}
		v.Argv = append(v.Argv, "--", "--format=pretty", "--color=never", "--test-threads=1")
		if len(r.Selectors) == 1 {
			v.Argv = append(v.Argv, "--exact", r.Selectors[0])
		}
	case "nextest":
		if r.Project == "" {
			return v, errors.New("explicit Cargo manifest required")
		}
		path := report("nextest.xml")
		v.Files = map[string][]byte{"nextest.toml": []byte("[profile.corvint]\nretries=0\n[profile.corvint.junit]\npath=" + strconv.Quote(path) + "\nreport-skipped=\"all\"\n")}
		v.Argv = []string{"nextest", "run", "--offline", "--locked", "--manifest-path", r.Project, "--config-file", filepath.Join(r.ReportDir, "nextest.toml"), "--profile", "corvint", "--no-fail-fast"}
		for _, s := range r.Selectors {
			if strings.ContainsAny(s, "(),|&!~ /\\\"") {
				return v, errors.New("ambiguous nextest selector")
			}
		}
		if len(r.Selectors) > 0 {
			var terms []string
			for _, s := range r.Selectors {
				binary, name, ok := strings.Cut(s, "::")
				if !ok || binary == "" || name == "" {
					return v, errors.New("nextest selector requires binary::test identity")
				}
				terms = append(terms, "(binary_id(="+binary+") & test(="+name+"))")
			}
			v.Argv = append(v.Argv, "-E", strings.Join(terms, " | "))
		}
	}
	return v, nil
}
func failureExits(runner string) []int {
	switch runner {
	case cmockaRunner:
		exits := make([]int, 64)
		for i := range exits {
			exits[i] = i + 1
		}
		return exits
	case "dotnet-mtp-nunit", "dotnet-mtp-mstest", "dotnet-mtp-xunit":
		return []int{2}
	case "ctest":
		return []int{8}
	case "catch2":
		return []int{42}
	case "cargo-test", "cargo-doctest", "cargo-integration", "cargo-bin":
		return []int{101}
	case "nextest":
		return []int{100}
	default:
		return []int{1}
	}
}
func containsExit(codes []int, n int) bool {
	for _, code := range codes {
		if code == n {
			return true
		}
	}
	return false
}

func exactRegex(ss []string) string {
	var q []string
	for _, s := range ss {
		q = append(q, regexp.QuoteMeta(s))
	}
	return "^(" + strings.Join(q, "|") + ")$"
}

func Parse(in tr.Input) (tr.Observation, error) {
	o := tr.Observation{Runner: in.Runner, RetryInformation: tr.NotReported}
	if !known(in.Runner) {
		return o, errors.New("unsupported native runner")
	}
	if in.TimedOut || in.Interrupted || in.Overflow {
		o.Problems = append(o.Problems, tr.Problem{Code: "PROCESS_INCOMPLETE", Detail: "timeout, interruption, or output bound"})
		return o, nil
	}
	if len(in.Stdout) > tr.MaxReportBytes || len(in.Stderr) > tr.MaxReportBytes || len(in.Reports) > tr.MaxReports {
		return o, errors.New("native report bound exceeded")
	}
	var err error
	switch in.Runner {
	case cmockaRunner:
		return parseCMocka(in)
	case "go-test":
		err = parseGo(in.Stdout, &o)
	case "cargo-test", "cargo-doctest", "cargo-integration", "cargo-bin":
		err = parseCargo(in.Stdout, &o)
	case ginkgoRunner:
		// The default arm's report-count and per-report bounds apply here too.
		if len(in.Reports) != 1 {
			return o, errors.New("exactly one native report required")
		}
		b, ok := in.Reports["ginkgo.json"]
		if !ok {
			return o, errors.New("ginkgo report must be ginkgo.json")
		}
		if len(b) > tr.MaxReportBytes {
			return o, errors.New("native report bound exceeded")
		}
		err = parseGinkgo(in, b, &o)
	default:
		if len(in.Reports) != 1 {
			return o, errors.New("exactly one native report required")
		}
		for _, b := range in.Reports {
			if len(b) > tr.MaxReportBytes {
				return o, errors.New("native report bound exceeded")
			}
			if strings.HasPrefix(in.Runner, "dotnet-mtp-") {
				err = parseMTP(b, &o)
			} else if strings.HasPrefix(in.Runner, "dotnet-") {
				err = parseTRX(b, &o)
			} else {
				err = parseXML(b, &o)
			}
		}
	}
	if err != nil {
		return o, err
	}
	seen := map[string]bool{}
	failed := false
	for _, t := range o.Tests {
		if t.ID == "" || seen[t.ID] {
			return o, errors.New("empty or duplicate native test identity")
		}
		seen[t.ID] = true
		failed = failed || t.State == tr.Failed
		if len(t.Attempts) > tr.MaxAttempts {
			return o, errors.New("attempt bound exceeded")
		}
	}
	if len(o.Tests) > tr.MaxTests {
		return o, errors.New("test bound exceeded")
	}
	if len(o.Tests) == 0 {
		o.Problems = append(o.Problems, tr.Problem{Code: "NO_TESTS", Detail: "native runner reported no tests"})
	}
	if (failed && !containsExit(failureExits(in.Runner), in.ExitCode)) || (!failed && in.ExitCode != 0) {
		o.Problems = append(o.Problems, tr.Problem{Code: "EXIT_REPORT_CONTRADICTION", Detail: fmt.Sprintf("exit %d does not agree with native cases", in.ExitCode)})
	}
	o.Complete = len(o.Problems) == 0
	sort.Slice(o.Tests, func(i, j int) bool { return o.Tests[i].ID < o.Tests[j].ID })
	return o, nil
}
func decodeXML(b []byte, v any) error {
	b = bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})
	if bytes.Contains(b, []byte("<!")) {
		return errors.New("XML directives and CDATA unsupported")
	}
	d := xml.NewDecoder(bytes.NewReader(b))
	if err := d.Decode(v); err != nil {
		return err
	}
	for {
		t, err := d.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if c, ok := t.(xml.CharData); !ok || len(bytes.TrimSpace(c)) != 0 {
			return errors.New("trailing XML content")
		}
	}
}

type failure struct {
	Type    string `xml:"type,attr"`
	Message string `xml:"message,attr"`
	Text    string `xml:",chardata"`
}
type xmlCase struct {
	Name   string    `xml:"name,attr"`
	Class  string    `xml:"classname,attr"`
	File   string    `xml:"file,attr"`
	Status string    `xml:"status,attr"`
	Result string    `xml:"result,attr"`
	Fail   []failure `xml:"failure"`
	Error  []failure `xml:"error"`
	Skip   []failure `xml:"skipped"`
	Flaky  []failure `xml:"flakyFailure"`
	Rerun  []failure `xml:"rerunFailure"`
}
type xmlSuite struct {
	Skipped  string `xml:"skipped,attr"`
	Disabled string `xml:"disabled,attr"`
	Failures string `xml:"failures,attr"`
	XMLName  xml.Name
	Name     string     `xml:"name,attr"`
	Count    string     `xml:"tests,attr"`
	Errors   string     `xml:"errors,attr"`
	Cases    []xmlCase  `xml:"testcase"`
	Suites   []xmlSuite `xml:"testsuite"`
	Failure  []failure  `xml:"failure"`
	Error    []failure  `xml:"error"`
}

func parseXML(b []byte, o *tr.Observation) error {
	scan := xml.NewDecoder(bytes.NewReader(b))
	allowed := map[string]bool{"testsuites": true, "testsuite": true, "testcase": true, "failure": true, "error": true, "skipped": true, "properties": true, "property": true, "system-out": true, "system-err": true, "flakyFailure": true, "rerunFailure": true}
	for {
		tok, err := scan.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		switch x := tok.(type) {
		case xml.Directive:
			return errors.New("XML directive unsupported")
		case xml.StartElement:
			if x.Name.Space != "" || !allowed[x.Name.Local] {
				return errors.New("unknown native XML element")
			}
		}
	}

	// GoogleTest legitimately encloses assertion text in CDATA. The decoder never resolves external entities.
	var root xmlSuite
	d := xml.NewDecoder(bytes.NewReader(b))
	if err := d.Decode(&root); err != nil {
		return err
	}
	if root.XMLName.Space != "" || (root.XMLName.Local != "testsuite" && root.XMLName.Local != "testsuites") {
		return errors.New("native XML root mismatch")
	}
	for {
		t, e := d.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		if c, ok := t.(xml.CharData); !ok || len(bytes.TrimSpace(c)) != 0 {
			return errors.New("trailing XML")
		}
	}
	disabledIDs := map[string]bool{}
	var walk func(xmlSuite) (int, error)
	walk = func(s xmlSuite) (int, error) {
		first := len(o.Tests)
		n := len(s.Cases)
		if s.Errors != "" && s.Errors != "0" || len(s.Failure)+len(s.Error) > 0 {
			o.Problems = append(o.Problems, tr.Problem{Code: "SUITE_ERROR", Detail: s.Name})
		}
		for _, c := range s.Cases {
			if len(o.Tests) >= tr.MaxTests {
				return 0, errors.New("test bound exceeded")
			}
			state := tr.Passed
			switch c.Status {
			case "", "run", "fail", "notrun", "disabled":
			default:
				return 0, errors.New("unknown native status")
			}
			if len(c.Error) > 0 {
				state = tr.Unknown
				o.Problems = append(o.Problems, tr.Problem{Code: "TEST_INFRASTRUCTURE", Detail: c.Name})
			} else if len(c.Fail) > 0 {
				state = tr.Failed
			} else if len(c.Skip) > 0 || c.Status == "disabled" || c.Status == "notrun" {
				state = tr.Skipped
			}
			if c.Status == "fail" && len(c.Fail) == 0 {
				return 0, errors.New("failure status without failure evidence")
			}
			var attempts []tr.Attempt
			if len(c.Flaky)+len(c.Rerun) > 0 {
				if o.Runner != "nextest" {
					return 0, errors.New("retry extension outside nextest profile")
				}
				o.RetryInformation = tr.Retained
				if len(c.Flaky) > 0 && len(c.Rerun) > 0 {
					return 0, errors.New("conflicting nextest retries")
				}
				if len(c.Rerun) > 0 {
					if state != tr.Failed {
						return 0, errors.New("rerun failures without initial failure")
					}
					if len(c.Fail) != 1 {
						return 0, errors.New("ambiguous nextest initial attempt")
					}
					attempts = append(attempts, nextestAttempt(c.Fail[0]))
					for _, f := range c.Rerun {
						attempts = append(attempts, nextestAttempt(f))
					}
				}
				if len(c.Flaky) > 0 {
					if state != tr.Passed {
						return 0, errors.New("flaky failures without final pass")
					}
					for _, f := range c.Flaky {
						attempts = append(attempts, nextestAttempt(f))
					}
					attempts = append(attempts, tr.Attempt{State: tr.Passed})
					state = tr.Flaky
				}
			}
			id := c.Name
			if o.Runner == "googletest" {
				if c.Class == "" {
					c.Class = s.Name
				}
				id = c.Class + "." + c.Name
				switch c.Result {
				case "completed":
					if c.Status != "run" || len(c.Skip) > 0 || len(c.Error) > 0 {
						return 0, errors.New("contradictory GoogleTest completed result")
					}
				case "skipped":
					if c.Status != "run" || len(c.Skip) != 1 || len(c.Fail)+len(c.Error) > 0 {
						return 0, errors.New("incomplete or contradictory GoogleTest skipped result")
					}
					state = tr.Skipped
				case "suppressed":
					if c.Status != "notrun" || len(c.Skip)+len(c.Fail)+len(c.Error) > 0 {
						return 0, errors.New("contradictory GoogleTest suppressed result")
					}
					state = tr.Skipped
					disabledIDs[id] = true
				default:
					return 0, errors.New("unknown GoogleTest result")
				}

			} else if o.Runner == "nextest" {
				id = c.Class + "::" + c.Name
			}
			kind, message := "", ""
			if state == tr.Failed {
				kind = tr.Unknown
			}
			if o.Runner == "nextest" && len(c.Fail) > 0 {
				if len(c.Fail) != 1 {
					return 0, errors.New("ambiguous nextest failure")
				}
				initial := nextestAttempt(c.Fail[0])
				kind, message = initial.FailureKind, initial.Message
			}
			o.Tests = append(o.Tests, tr.Test{ID: id, Name: c.Name, File: c.File, Suite: c.Class, State: state, Attempts: attempts, FailureKind: kind, Message: message})
		}
		for _, child := range s.Suites {
			m, e := walk(child)
			if e != nil {
				return 0, e
			}
			n += m
		}
		if s.Count != "" {
			count, e := strconv.Atoi(s.Count)
			if e != nil || count != n {
				return 0, errors.New("native suite count mismatch")
			}
		}
		if s.Failures != "" && o.Runner != "catch2" {
			count, e := strconv.Atoi(s.Failures)
			actual := 0
			for _, row := range o.Tests[first:] {
				if row.State == tr.Failed {
					actual++
				}
			}
			if e != nil || count != actual {
				return 0, errors.New("native failure count mismatch")
			}
		}
		if o.Runner == "googletest" {
			if s.Count == "" || s.Failures == "" || s.Errors == "" || s.Disabled == "" || (s.XMLName.Local == "testsuite" && s.Skipped == "") {
				return 0, errors.New("GoogleTest native counts missing")
			}
			skips, disabled := 0, 0
			for _, row := range o.Tests[first:] {
				if disabledIDs[row.ID] {
					disabled++
				} else if row.State == tr.Skipped {
					skips++
				}
			}
			for _, c := range []struct {
				raw    string
				actual int
			}{{s.Skipped, skips}, {s.Disabled, disabled}} {
				if c.raw == "" {
					continue
				}
				n, e := strconv.Atoi(c.raw)
				if e != nil || n != c.actual {
					return 0, errors.New("GoogleTest skip/disabled count mismatch")
				}
			}
		}
		return n, nil
	}
	_, err := walk(root)
	return err
}

func nextestAttempt(f failure) tr.Attempt {
	kind := tr.Unknown
	lower := strings.ToLower(f.Type)
	for _, word := range []string{"signal", "abort", "crash", "setup", "leak", "timeout"} {
		if strings.Contains(lower, word) {
			kind = tr.Infrastructure
		}
	}
	return tr.Attempt{State: tr.Failed, FailureKind: kind, Message: "native type: " + f.Type + "\n" + f.Message + "\n" + f.Text}
}

type trxDoc struct {
	XMLName xml.Name
	Results []struct {
		Error     *trxError `xml:"Output>ErrorInfo"`
		ID        string    `xml:"testId,attr"`
		Execution string    `xml:"executionId,attr"`
		Name      string    `xml:"testName,attr"`
		Outcome   string    `xml:"outcome,attr"`
	} `xml:"Results>UnitTestResult"`
	Definitions []struct {
		ID     string `xml:"id,attr"`
		Method struct {
			Class   string `xml:"className,attr"`
			Name    string `xml:"name,attr"`
			Adapter string `xml:"adapterTypeName,attr"`
		} `xml:"TestMethod"`
	} `xml:"TestDefinitions>UnitTest"`
	Summary struct {
		Error    *trxError `xml:"Output>ErrorInfo"`
		Counters struct {
			Attributes []xml.Attr `xml:",any,attr"`
		} `xml:"Counters"`
		Outcome string `xml:"outcome,attr"`
		Infos   []struct {
			Outcome string `xml:"outcome,attr"`
			Text    string `xml:"Text"`
		} `xml:"RunInfos>RunInfo"`
	} `xml:"ResultSummary"`
}

func parseTRX(b []byte, o *tr.Observation) error {
	if err := checkNativeTRXXML(b); err != nil {
		return err
	}
	var doc trxDoc
	if err := decodeXML(b, &doc); err != nil {
		return err
	}
	if doc.XMLName.Local != "TestRun" || doc.XMLName.Space != "http://microsoft.com/schemas/VisualStudio/TeamTest/2010" {
		return errors.New("TRX namespace/root mismatch")
	}
	counts := map[string]int{}
	allowedCounts := strings.Fields("total executed passed failed error timeout aborted inconclusive passedButRunAborted notRunnable notExecuted disconnected warning completed inProgress pending")
	for _, a := range doc.Summary.Counters.Attributes {
		if a.Name.Space != "" {
			return errors.New("TRX counter namespace")
		}
		if _, ok := counts[a.Name.Local]; ok {
			return errors.New("duplicate TRX counter")
		}
		n, e := strconv.Atoi(a.Value)
		if e != nil || n < 0 || n > tr.MaxTests {
			return errors.New("invalid TRX counter")
		}
		counts[a.Name.Local] = n
	}
	if len(counts) != len(allowedCounts) {
		return errors.New("TRX native counter inventory incomplete")
	}
	for _, key := range allowedCounts {
		if _, ok := counts[key]; !ok {
			return errors.New("TRX required counter missing")
		}
	}
	if counts["total"] != len(doc.Results) {
		return errors.New("TRX result count mismatch")
	}
	switch doc.Summary.Outcome {
	case "Completed", "Failed":
	default:
		o.Problems = append(o.Problems, tr.Problem{Code: "TRX_RUN_OUTCOME", Detail: doc.Summary.Outcome})
	}

	defs := map[string]string{}
	adapter := map[string]string{"dotnet-vstest-nunit": "executor://nunit3testexecutor/", "dotnet-vstest-mstest": "executor://mstestadapter/v2", "dotnet-vstest-xunit": "executor://xunit/VsTestRunner2/netcoreapp"}[o.Runner]
	for _, d := range doc.Definitions {
		if _, ok := defs[d.ID]; ok {
			return errors.New("duplicate TRX definition")
		}
		if d.Method.Adapter != adapter {
			return errors.New("TRX adapter mismatch")
		}
		if d.Method.Class == "" || d.Method.Name == "" {
			return errors.New("missing TRX test method")
		}
		defs[d.ID] = d.Method.Class + "." + d.Method.Name
	}
	for _, r := range doc.Results {
		id, ok := defs[r.ID]
		if !ok {
			return errors.New("TRX result missing definition")
		}
		state := ""
		switch r.Outcome {
		case "Passed":
			state = tr.Passed
		case "Failed":
			state = tr.Failed
		case "NotExecuted":
			state = tr.Skipped
		case "Timeout":
			state = tr.TimedOut
			o.Problems = append(o.Problems, tr.Problem{Code: "TRX_TEST_TIMEOUT", Detail: r.Name})
		case "Aborted":
			state = tr.Interrupted
			o.Problems = append(o.Problems, tr.Problem{Code: "TRX_TEST_INTERRUPTED", Detail: r.Name})
		default:
			state = tr.Unknown
			o.Problems = append(o.Problems, tr.Problem{Code: "TRX_UNKNOWN_OUTCOME", Detail: r.Outcome})
		}
		row := tr.Test{ID: id, Name: r.Name, State: state}
		if state == tr.Failed {
			row.FailureKind = tr.Unknown
		}
		if r.Error != nil {
			row.Message = r.Error.Message
			if state == tr.Passed {
				o.Problems = append(o.Problems, tr.Problem{Code: "TRX_CONTRADICTORY_ERROR", Detail: "Passed row contains ErrorInfo: " + r.Name})
			}
		}
		o.Tests = append(o.Tests, row)
	}
	actual := map[string]int{}
	for _, r := range doc.Results {
		actual[r.Outcome]++
	}
	if counts["passed"] != actual["Passed"] || counts["failed"] != actual["Failed"] || counts["executed"] != len(doc.Results)-actual["NotExecuted"] {
		return errors.New("TRX executed/passed/failed counters contradict native rows")
	}
	// VSTest's native 17.14.1 logger leaves notExecuted=0 for skipped rows.
	// Total-minus-executed is the authoritative skipped denominator in this profile.
	if counts["notExecuted"] != 0 && counts["notExecuted"] != actual["NotExecuted"] {
		return errors.New("TRX skipped counter mismatch")
	}
	for _, key := range []string{"error", "timeout", "aborted", "inconclusive", "passedButRunAborted", "notRunnable", "disconnected", "warning", "completed", "inProgress", "pending"} {
		if counts[key] != 0 {
			o.Problems = append(o.Problems, tr.Problem{Code: "TRX_NONPASS_COUNTER", Detail: key})
		}
	}
	if (doc.Summary.Outcome == "Failed") != (actual["Failed"] > 0) {
		o.Problems = append(o.Problems, tr.Problem{Code: "TRX_SUMMARY_CONTRADICTION", Detail: doc.Summary.Outcome})
	}
	if doc.Summary.Error != nil {
		o.Problems = append(o.Problems, tr.Problem{Code: "TRX_SUMMARY_ERROR", Detail: doc.Summary.Error.Message})
	}
	for _, info := range doc.Summary.Infos {
		nativeDiagnostic := false
		if o.Runner == "dotnet-vstest-xunit" && strings.HasPrefix(info.Text, "[xUnit.net ") {
			for _, row := range o.Tests {
				if (row.State == tr.Failed && info.Outcome == "Error" && strings.HasSuffix(info.Text, "     "+row.Name+" [FAIL]")) || (row.State == tr.Skipped && info.Outcome == "Warning" && strings.HasSuffix(info.Text, "     "+row.Name+" [SKIP]")) {
					nativeDiagnostic = true
				}
			}
		}
		if info.Outcome != "Passed" && !nativeDiagnostic {
			o.Problems = append(o.Problems, tr.Problem{Code: "TRX_RUN_INFO", Detail: info.Outcome})
		}
	}
	if len(doc.Results) > tr.MaxTests {
		return errors.New("TRX test bound")
	}
	return nil
}
func parseGo(b []byte, o *tr.Observation) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	type event struct {
		Action      string
		Package     string
		Test        string
		Output      string
		FailedBuild string
	}
	active := map[string]bool{}
	finishedPackages := map[string]bool{}
	packageFailed := map[string]bool{}
	packages := map[string]bool{}
	for {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		if err := uniqueGoFields(raw); err != nil {
			return err
		}
		var e event
		if err := json.Unmarshal(raw, &e); err != nil {
			return err
		}
		if e.Package == "" {
			return errors.New("Go event package missing")
		}
		packages[e.Package] = true
		switch e.Action {
		case "start", "run", "pause", "cont", "output", "pass", "fail", "skip", "build-output", "build-fail":
		default:
			return errors.New("unknown Go event action")
		}
		if finishedPackages[e.Package] {
			return errors.New("Go event after package terminal")
		}
		if e.Test == "" {
			if e.Action == "pass" || e.Action == "fail" || e.Action == "skip" {
				if finishedPackages[e.Package] {
					return errors.New("duplicate Go package terminal")
				}
				finishedPackages[e.Package] = true
				if e.Action == "fail" {
					packageFailed[e.Package] = true
				}
			}
			if e.Action == "build-fail" || e.FailedBuild != "" {
				o.Problems = append(o.Problems, tr.Problem{Code: "BUILD_FAILURE", Detail: e.Package})
			}
			continue
		}
		id := e.Package + "::" + e.Test
		if e.Action == "run" {
			if active[id] {
				return errors.New("duplicate Go run")
			}
			active[id] = true
		}
		if e.Action == "pass" || e.Action == "fail" || e.Action == "skip" {
			if !active[id] {
				return errors.New("Go terminal event without run")
			}
			delete(active, id)
			state := map[string]string{"pass": tr.Passed, "fail": tr.Failed, "skip": tr.Skipped}[e.Action]
			o.Tests = append(o.Tests, tr.Test{ID: id, Name: e.Test, Suite: e.Package, State: state})
			if len(o.Tests) > tr.MaxTests {
				return errors.New("Go test bound")
			}
		}
	}
	if len(active) > 0 {
		return errors.New("incomplete Go test stream")
	}
	for p := range packages {
		if packageFailed[p] {
			failed := false
			for _, row := range o.Tests {
				failed = failed || (row.Suite == p && row.State == tr.Failed)
			}
			if !failed {
				o.Problems = append(o.Problems, tr.Problem{Code: "GO_PACKAGE_FAILURE", Detail: p})
			}
		}
		if !finishedPackages[p] {
			o.Problems = append(o.Problems, tr.Problem{Code: "GO_PACKAGE_INCOMPLETE", Detail: p})
		}
	}
	return nil
}

func uniqueGoFields(b []byte) error {
	d := json.NewDecoder(bytes.NewReader(b))
	first, e := d.Token()
	if e != nil || first != json.Delim('{') {
		return errors.New("Go event must be an object")
	}
	seen := map[string]bool{}
	for d.More() {
		key, e := d.Token()
		if e != nil {
			return e
		}
		name, ok := key.(string)
		if !ok {
			return errors.New("invalid Go field")
		}
		fold := strings.ToLower(name)
		if seen[fold] {
			return errors.New("duplicate Go event field")
		}
		seen[fold] = true
		var value json.RawMessage
		if e = d.Decode(&value); e != nil {
			return e
		}
	}
	if _, e = d.Token(); e != nil {
		return e
	}
	return nil
}

var cargoStart = regexp.MustCompile(`^running (0|[1-9][0-9]*) tests?$`)
var cargoCase = regexp.MustCompile(`^test (.+) \.\.\. (ok|FAILED|ignored(?:, .*)?)$`)
var cargoSummary = regexp.MustCompile(`^test result: (ok|FAILED)\. ([0-9]+) passed; ([0-9]+) failed; ([0-9]+) ignored; [0-9]+ measured; [0-9]+ filtered out; finished in .+$`)

func parseCargo(b []byte, o *tr.Observation) error {
	// This stable libtest profile is deliberately one --lib or --doc target with
	// pretty, serial output. Cargo's JSON compiler messages are not test results.
	started := false
	done := false
	announced := -1
	pass, fail, skip := 0, 0, 0
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "running ") {
			m := cargoStart.FindStringSubmatch(line)
			if m == nil {
				return errors.New("malformed Cargo inventory")
			}
			n, e := strconv.Atoi(m[1])
			if e != nil || n > tr.MaxTests {
				return errors.New("Cargo inventory bound")
			}
			if started {
				return errors.New("multiple Cargo suites unsupported")
			}
			started = true
			announced = n
			continue
		}

		if m := cargoCase.FindStringSubmatch(line); m != nil {
			if !started || done {
				return errors.New("Cargo case outside suite")
			}
			state := tr.Passed
			pass++
			if m[2] == "FAILED" {
				state = tr.Failed
				pass--
				fail++
			} else if strings.HasPrefix(m[2], "ignored") {
				state = tr.Skipped
				pass--
				skip++
			}
			o.Tests = append(o.Tests, tr.Test{ID: m[1], Name: m[1], State: state})
			if len(o.Tests) > tr.MaxTests {
				return errors.New("Cargo test bound")
			}
		}
		if m := cargoSummary.FindStringSubmatch(line); m != nil {
			if !started || done {
				return errors.New("duplicate Cargo summary")
			}
			for i, n := range []int{pass, fail, skip} {
				actual, _ := strconv.Atoi(m[i+2])
				if actual != n {
					return errors.New("Cargo native case count mismatch")
				}
			}
			if (m[1] == "FAILED") != (fail > 0) {
				return errors.New("Cargo summary contradiction")
			}
			if announced != pass+fail+skip {
				return errors.New("Cargo announced inventory mismatch")
			}
			done = true
		}
	}
	if !done {
		o.Problems = append(o.Problems, tr.Problem{Code: "CARGO_SUITE_INCOMPLETE", Detail: "build/collection failure or incomplete stable libtest output"})
	}
	return nil
}
