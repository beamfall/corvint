package registry_test

import (
	"encoding/json"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/native"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCMockaRegistryDispatchAndTargetBoundary(t *testing.T) {
	count := 0
	for _, r := range registry.Runners() {
		if r == "cmocka-xml" {
			count++
		}
	}
	if count != 1 {
		t.Fatal(count)
	}
	req := tr.Request{Runner: "cmocka-xml", Executable: "/test", ReportDir: "/fresh", Target: "G", ExpectedTests: []string{"G::test_pass"}}
	inv, e := registry.Build(req)
	if e != nil || inv.Phases[0].Environment["CMOCKA_MESSAGE_OUTPUT"] != "STANDARD,XML" || len(inv.Argv) != 0 {
		t.Fatal(inv, e)
	}
	in := tr.Input{Runner: req.Runner, Target: req.Target, Expected: req.ExpectedTests, Reports: map[string][]byte{"cmocka.xml": []byte(`<testsuites><testsuite name="G" time="0.000" tests="1" failures="0" errors="0" skipped="0"><testcase name="test_pass" time="0.000"/></testsuite></testsuites>`)}, Stdout: []byte("[==========] G: Running 1 test(s).\n[ RUN      ] test_pass\n[       OK ] test_pass\n[==========] G: 1 test(s) run.\n"), Stderr: []byte("[  PASSED  ] 1 test(s).\n"), SuccessExitCodes: inv.SuccessExitCodes, FailureExitCodes: inv.FailureExitCodes}
	o, e := registry.Parse(in)
	if e != nil || !o.Complete || len(o.Tests) != 1 || o.Tests[0].ID != "G::test_pass" {
		t.Fatal(o, e)
	}
	in.Stderr = append([]byte("[  FAILED  ] GROUP TEARDOWN\n[  ERROR   ] G\n"), in.Stderr...)
	o, e = registry.Parse(in)
	if e != nil || o.Complete || o.Tests[0].State != tr.Unknown {
		t.Fatal(o, e)
	}
	in.Target = "Other"
	in.Expected = []string{"Other::test_pass"}
	o, e = registry.Parse(in)
	if e == nil || o.Complete {
		t.Fatal("wrong group admitted", o, e)
	}
	in.Target = "G"
	in.Expected = []string{"G::test_pass"}
	in.Stderr = []byte("[  PASSED  ] 1 test(s).\n")
	in.Reports["cmocka.xml"] = []byte(strings.Replace(string(in.Reports["cmocka.xml"]), "G", "Other", 1))
	o, e = registry.Parse(in)
	if e == nil || o.Complete {
		t.Fatal(o, e)
	}
}

// TestCMockaNativeReceiptReadback independently compares original native bytes
// through direct parsing and the public normalized registry/CLI path.
func TestCMockaNativeReceiptReadback(t *testing.T) {
	dir := os.Getenv("CORVINT_CMOCKA_PROOF")
	if dir == "" {
		t.Skip("actual cmocka receipt proof is opt-in")
	}
	var input struct {
		Cases []struct {
			Name         string
			Out          string
			Request      tr.Request
			Complete     bool
			NativeStates map[string]string
			PublicStates map[string]string
		}
	}
	data, e := os.ReadFile(filepath.Join(dir, "input.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(data, &input); e != nil {
		t.Fatal(e)
	}
	for _, c := range input.Cases {
		t.Run(c.Name, func(t *testing.T) {
			data, e := os.ReadFile(filepath.Join(c.Out, "receipt.json"))
			if e != nil {
				t.Fatal(e)
			}
			var receipt tr.ReceiptDocument
			if e = json.Unmarshal(data, &receipt); e != nil {
				t.Fatal(e)
			}
			var plan tr.PlanDocument
			data, e = os.ReadFile(filepath.Join(c.Out, "plan.json"))
			if e != nil {
				t.Fatal(e)
			}
			if e = json.Unmarshal(data, &plan); e != nil {
				t.Fatal(e)
			}
			if receipt.PlanSha256 != tr.Identity(plan) || plan.Request.Target != c.Request.Target || len(receipt.Execution.Phases) != 1 {
				t.Fatal("receipt/source target binding", receipt)
			}
			phase := receipt.Execution.Phases[0]
			out, e := os.ReadFile(filepath.Join(c.Request.ReportDir, ".phase-00-stdout"))
			if e != nil {
				t.Fatal(e)
			}
			stderr, e := os.ReadFile(filepath.Join(c.Request.ReportDir, ".phase-00-stderr"))
			if e != nil {
				t.Fatal(e)
			}
			xml, e := os.ReadFile(filepath.Join(c.Request.ReportDir, "cmocka.xml"))
			if e != nil {
				t.Fatal(e)
			}
			if tr.Digest(out) != phase.StdoutSha256 || tr.Digest(stderr) != phase.StderrSha256 || tr.Digest(xml) != receipt.Execution.ReportSha256["cmocka.xml"] {
				t.Fatal("original stream/report hashes changed")
			}
			in := tr.Input{Runner: c.Request.Runner, Target: c.Request.Target, Selectors: c.Request.Selectors, Expected: c.Request.ExpectedTests, Reports: map[string][]byte{"cmocka.xml": xml}, Stdout: out, Stderr: stderr, ExitCode: phase.ExitCode, SuccessExitCodes: plan.Invocation.SuccessExitCodes, FailureExitCodes: plan.Invocation.FailureExitCodes}
			direct, _ := native.Parse(in)
			public, _ := registry.Parse(in)
			states := func(o tr.Observation) map[string]string {
				m := map[string]string{}
				for _, v := range o.Tests {
					m[v.ID] = v.State
				}
				return m
			}
			if direct.Complete != c.Complete || public.Complete != c.Complete || receipt.Observation.Complete != c.Complete || !reflect.DeepEqual(states(direct), c.NativeStates) || !reflect.DeepEqual(states(public), c.PublicStates) || !reflect.DeepEqual(states(receipt.Observation), c.PublicStates) {
				t.Fatalf("layer mismatch direct%+v public%+v receipt%+v", direct, public, receipt.Observation)
			}
			if !c.Complete && len(public.Problems) == 0 {
				t.Fatal("incomplete report lacks retained uncertainty")
			}
		})
	}
}
