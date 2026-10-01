package native

import (
	"encoding/xml"
	"errors"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// This profile executes an already-built MTP application. Project evaluation,
// restore, SDK selection, and the complete runtime dependency closure are external.
func buildMTP(r tr.Request, v tr.Invocation) (tr.Invocation, error) {
	if r.Project == "" || filepath.IsAbs(r.Project) || filepath.Clean(r.Project) != r.Project || strings.HasPrefix(r.Project, "-") || strings.HasPrefix(r.Project, "../") || !strings.HasSuffix(r.Project, ".dll") {
		return v, errors.New("MTP requires one root-relative prebuilt DLL")
	}
	stem := strings.TrimSuffix(r.Project, ".dll")
	for _, name := range []string{r.Project, stem + ".runtimeconfig.json", stem + ".deps.json"} {
		if r.InputFiles[name] == "" {
			return v, errors.New("MTP assembly, runtimeconfig and deps must be declared hashed inputs")
		}
	}
	v.ReportPaths = []string{"results.trx"}
	v.Environment["DOTNET_CLI_TELEMETRY_OPTOUT"] = "1"
	v.Environment["TESTINGPLATFORM_TELEMETRY_OPTOUT"] = "1"
	v.Environment["DOTNET_ROOT"] = filepath.Dir(r.Executable)
	v.Argv = []string{r.Project, "--report-trx", "--report-trx-filename", "results.trx", "--results-directory", r.ReportDir, "--no-ansi", "--no-progress"}
	var filters []string
	for _, s := range r.Selectors {
		if !mtpSelector.MatchString(s) {
			return v, errors.New("MTP selector must be a literal fully-qualified method name")
		}
		filters = append(filters, "FullyQualifiedName="+s)
	}
	if len(filters) > 0 {
		v.Argv = append(v.Argv, "--filter", strings.Join(filters, "|"))
	}
	return v, nil
}

var mtpSelector = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.+]*$`)
var mtpGUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type mtpDoc struct {
	XMLName xml.Name
	Results []struct {
		ID        string    `xml:"testId,attr"`
		Execution string    `xml:"executionId,attr"`
		Name      string    `xml:"testName,attr"`
		Outcome   string    `xml:"outcome,attr"`
		Error     *trxError `xml:"Output>ErrorInfo"`
	} `xml:"Results>UnitTestResult"`
	Definitions []struct {
		ID        string `xml:"id,attr"`
		Execution struct {
			ID string `xml:"id,attr"`
		} `xml:"Execution"`
		Method struct {
			Class   string `xml:"className,attr"`
			Name    string `xml:"name,attr"`
			Adapter string `xml:"adapterTypeName,attr"`
		} `xml:"TestMethod"`
	} `xml:"TestDefinitions>UnitTest"`
	Entries []struct {
		ID        string `xml:"testId,attr"`
		Execution string `xml:"executionId,attr"`
	} `xml:"TestEntries>TestEntry"`
	Summary struct {
		Error    *trxError `xml:"Output>ErrorInfo"`
		Outcome  string    `xml:"outcome,attr"`
		Counters struct {
			Attributes []xml.Attr `xml:",any,attr"`
		} `xml:"Counters"`
		Infos []struct {
			Outcome string `xml:"outcome,attr"`
			Text    string `xml:"Text"`
		} `xml:"RunInfos>RunInfo"`
	} `xml:"ResultSummary"`
}

func parseMTP(b []byte, o *tr.Observation) error {
	if err := checkNativeTRXXML(b); err != nil {
		return err
	}
	var d mtpDoc
	if err := decodeXML(b, &d); err != nil {
		return err
	}
	if d.XMLName.Local != "TestRun" || d.XMLName.Space != "http://microsoft.com/schemas/VisualStudio/TeamTest/2010" {
		return errors.New("MTP TRX root/namespace mismatch")
	}
	if len(d.Results) > tr.MaxTests || len(d.Definitions) != len(d.Results) || len(d.Entries) != len(d.Results) {
		return errors.New("MTP TRX inventory mismatch")
	}
	adapter := map[string]string{
		"dotnet-mtp-nunit":  "executor://NUnitExtension/6.3.0+b5a66c0f74e52257b3ec1e04abc3ef812a7e74d5",
		"dotnet-mtp-mstest": "executor://MSTest.Sdk/4.4.1",
		"dotnet-mtp-xunit":  "executor://30ea7c6e-dd24-4152-a360-1387158cd41d/4.0.1",
	}[o.Runner]
	defs := map[string]string{}
	executions := map[string]string{}
	entries := map[string]string{}
	usedExecution := map[string]bool{}
	for _, v := range d.Definitions {
		if !mtpGUID.MatchString(v.ID) || !mtpGUID.MatchString(v.Execution.ID) || defs[v.ID] != "" || usedExecution[v.Execution.ID] || v.Method.Class == "" || v.Method.Name == "" || v.Method.Adapter != adapter {
			return errors.New("MTP TRX definition or qualified adapter mismatch")
		}
		defs[v.ID] = v.Method.Class + "." + v.Method.Name + "::" + v.ID
		executions[v.ID] = v.Execution.ID
		usedExecution[v.Execution.ID] = true
	}
	for _, v := range d.Entries {
		if entries[v.ID] != "" || executions[v.ID] == "" || executions[v.ID] != v.Execution {
			return errors.New("MTP TRX entry mismatch")
		}
		entries[v.ID] = v.Execution
	}
	actual := map[string]int{}
	seen := map[string]bool{}
	for _, v := range d.Results {
		if defs[v.ID] == "" || seen[v.ID] || entries[v.ID] != v.Execution || v.Name == "" {
			return errors.New("MTP TRX result identity mismatch")
		}
		seen[v.ID] = true
		state := tr.Unknown
		switch v.Outcome {
		case "Passed":
			state = tr.Passed
		case "Failed":
			state = tr.Failed
		case "NotExecuted":
			state = tr.Skipped
		default:
			o.Problems = append(o.Problems, tr.Problem{Code: "MTP_TEST_OUTCOME", Detail: v.Outcome})
		}
		t := tr.Test{ID: defs[v.ID], Name: v.Name, State: state}
		if v.Error != nil {
			t.Message = v.Error.Message
			if state == tr.Passed {
				o.Problems = append(o.Problems, tr.Problem{Code: "MTP_CONTRADICTORY_ERROR", Detail: "Passed row contains ErrorInfo: " + v.Name})
			}
		}
		// TRX does not distinguish assertions from fixture/setup exceptions.
		if state == tr.Failed {
			t.FailureKind = tr.Unknown
		}
		o.Tests = append(o.Tests, t)
		actual[v.Outcome]++
	}
	counts := map[string]int{}
	keys := strings.Fields("total executed passed failed error timeout aborted inconclusive passedButRunAborted notRunnable notExecuted disconnected warning completed inProgress pending")
	for _, a := range d.Summary.Counters.Attributes {
		if a.Name.Space != "" {
			return errors.New("MTP TRX counter namespace")
		}
		if _, ok := counts[a.Name.Local]; ok {
			return errors.New("duplicate MTP counter")
		}
		n, e := strconv.Atoi(a.Value)
		if e != nil || n < 0 || n > tr.MaxTests {
			return errors.New("invalid MTP counter")
		}
		counts[a.Name.Local] = n
	}
	if len(counts) != len(keys) {
		return errors.New("MTP counter inventory mismatch")
	}
	for _, k := range keys {
		if _, ok := counts[k]; !ok {
			return errors.New("missing MTP counter")
		}
	}
	if counts["total"] != len(d.Results) || counts["executed"] != len(d.Results)-actual["NotExecuted"] || counts["passed"] != actual["Passed"] || counts["failed"] != actual["Failed"] || counts["notExecuted"] != actual["NotExecuted"] {
		return errors.New("MTP native counters contradict rows")
	}
	for _, k := range keys {
		if k != "total" && k != "executed" && k != "passed" && k != "failed" && k != "notExecuted" && counts[k] != 0 {
			o.Problems = append(o.Problems, tr.Problem{Code: "MTP_RUN_COUNTER", Detail: k})
		}
	}
	want := "Completed"
	if actual["Failed"] > 0 {
		want = "Failed"
	}
	if d.Summary.Outcome != want {
		o.Problems = append(o.Problems, tr.Problem{Code: "MTP_RUN_OUTCOME", Detail: d.Summary.Outcome})
	}
	if d.Summary.Error != nil {
		o.Problems = append(o.Problems, tr.Problem{Code: "MTP_SUMMARY_ERROR", Detail: d.Summary.Error.Message})
	}
	for _, v := range d.Summary.Infos {
		// MTP emits this diagnostic for ordinary failed tests, including setup failures.
		if v.Outcome == "Error" && actual["Failed"] > 0 && v.Text == "Exit code indicates failure: '2'. Please refer to https://aka.ms/testingplatform/exitcodes for more information." {
			continue
		}
		o.Problems = append(o.Problems, tr.Problem{Code: "MTP_RUN_INFO", Detail: v.Outcome + ": " + v.Text})
	}
	return nil
}
