package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

func TestClosedRequestAndPlanAdmission(t *testing.T) {
	for _, raw := range []string{`{"runner":"go-test","runner":"ctest"}`, `{"Runner":"go-test"}`, `{"tools":{"java":{"executable":"/x","sha256":"a","ignored":true}}}`, `{} {}`} {
		var r tr.Request
		if e := tr.DecodeDocument([]byte(raw), &r); e == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	p := plan{Profile: profile, Request: tr.Request{Runner: "go-test", Executable: "/independent/go", ExecutableSha256: "pinned", Project: "p", ReportDir: filepath.Join(t.TempDir(), "reports")}, Invocation: tr.Invocation{Argv: []string{"arbitrary", "command"}}}
	path := filepath.Join(t.TempDir(), "plan.json")
	b, _ := json.Marshal(p)
	os.WriteFile(path, b, 0600)
	var out, errout bytes.Buffer
	if command(context.Background(), []string{"run", "--plan", path, "--approve", tr.Identity(p), "--executable", p.Request.Executable, "--executable-sha256", "pinned", "--experimental", "--trusted-local"}, &out, &errout) != 1 {
		t.Fatal("changed fixed invocation admitted")
	}
	if _, e := os.Stat(p.Request.ReportDir); !os.IsNotExist(e) {
		t.Fatal("refused plan executed")
	}
}
func TestActualGoPlanRunReceipt(t *testing.T) {
	exe, e := exec.LookPath("go")
	if e != nil {
		t.Skip("Go unavailable")
	}
	exe, e = filepath.EvalSymlinks(exe)
	if e != nil {
		t.Fatal(e)
	}
	tool, e := os.ReadFile(exe)
	if e != nil {
		t.Fatal(e)
	}
	root := t.TempDir()
	files := map[string][]byte{"go.mod": []byte("module example.invalid/clirunner\n\ngo 1.27.1\n"), "runner_test.go": []byte("package clirunner\nimport \"testing\"\nfunc TestPass(t *testing.T){}\nfunc TestFail(t *testing.T){t.Fatal(\"intentional\")}\nfunc TestSkip(t *testing.T){t.Skip(\"intentional\")}\n")}
	r := tr.Request{Runner: "go-test", Root: root, Project: "example.invalid/clirunner", ReportDir: filepath.Join(t.TempDir(), "reports"), TimeoutSeconds: 120, InputFiles: map[string]string{}, ExpectedTests: []string{"example.invalid/clirunner::TestPass", "example.invalid/clirunner::TestFail", "example.invalid/clirunner::TestSkip"}}
	for n, b := range files {
		if e = os.WriteFile(filepath.Join(root, n), b, 0600); e != nil {
			t.Fatal(e)
		}
		r.InputFiles[n] = tr.Digest(b)
	}
	req := filepath.Join(t.TempDir(), "request.json")
	b, _ := json.Marshal(r)
	os.WriteFile(req, b, 0600)
	var output, errors bytes.Buffer
	identity := []string{"--executable", exe, "--executable-sha256", tr.Digest(tool)}
	if command(context.Background(), append([]string{"plan", "--request", req}, identity...), &output, &errors) != 0 {
		t.Fatal(errors.String())
	}
	var p plan
	if e = tr.DecodeDocument(output.Bytes(), &p); e != nil {
		t.Fatal(e)
	}
	planPath := filepath.Join(t.TempDir(), "plan.json")
	os.WriteFile(planPath, output.Bytes(), 0600)
	output.Reset()
	errors.Reset()
	receiptPath := filepath.Join(t.TempDir(), "receipt.json")
	argv := append([]string{"run", "--out", receiptPath, "--plan", planPath, "--approve", tr.Identity(p), "--experimental", "--trusted-local"}, identity...)
	if code := command(context.Background(), argv, &output, &errors); code != 1 {
		t.Fatalf("failure receipt exit=%d: %s", code, errors.String())
	}
	var result receipt
	receiptBytes, readErr := os.ReadFile(receiptPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if e = json.Unmarshal(receiptBytes, &result); e != nil {
		t.Fatalf("missing recorded failure: %s %s", output.String(), errors.String())
	}
	if !result.Observation.Complete || result.Error != "" || len(result.Observation.Tests) != 3 {
		t.Fatalf("incomplete native execution: %+v", result)
	}
	states := map[string]bool{}
	for _, test := range result.Observation.Tests {
		states[test.State] = true
	}
	if !states[tr.Passed] || !states[tr.Failed] || !states[tr.Skipped] {
		t.Fatal(states)
	}
	if result.Execution.DependencyClosure != "NOT_OBSERVED" {
		t.Fatal("invented dependency attestation")
	}
}

func TestExportedDocumentsPreserveNativeBytesAndIdentity(t *testing.T) {
	// These local declarations freeze the old companion ABI independently of aliases.
	type originalPlan struct {
		Profile    string        `json:"profile"`
		Request    tr.Request    `json:"request"`
		Invocation tr.Invocation `json:"invocation"`
	}
	type originalReceipt struct {
		Profile     string         `json:"profile"`
		PlanSha256  string         `json:"planSha256"`
		Execution   tr.Execution   `json:"execution"`
		Observation tr.Observation `json:"observation"`
		Error       string         `json:"error"`
	}
	p := originalPlan{profile, tr.Request{Runner: "go-test", InputFiles: map[string]string{"z": "last", "a": "first"}, Selectors: []string{"example::TestA"}}, tr.Invocation{Argv: []string{"test", "-json", "./..."}}}
	r := originalReceipt{"corvint-test-runner-receipt/0", tr.Identity(p), tr.Execution{Profile: "corvint-test-runner-execution/0", InputSha256: tr.Identity(p.Request.InputFiles)}, tr.Observation{Tests: []tr.Test{{ID: "x", State: tr.Failed, FailureKind: tr.Infrastructure, Attempts: []tr.Attempt{{State: tr.Failed, FailureKind: tr.Infrastructure}}}}}, "retained failure"}
	for _, pair := range [][2]any{{p, tr.PlanDocument{Profile: p.Profile, Request: p.Request, Invocation: p.Invocation}}, {r, tr.ReceiptDocument{Profile: r.Profile, PlanSha256: r.PlanSha256, Execution: r.Execution, Observation: r.Observation, Error: r.Error}}} {
		before, e := json.Marshal(pair[0])
		if e != nil {
			t.Fatal(e)
		}
		after, e := json.Marshal(pair[1])
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(before, after) || tr.Identity(pair[0]) != tr.Identity(pair[1]) {
			t.Fatalf("native serialization changed: %s != %s", before, after)
		}
	}
}

func TestCMockaPlanBindsTargetAndFixedEnvironment(t *testing.T) {
	req := tr.Request{Runner: "cmocka-xml", Target: "G", ExpectedTests: []string{"G::test_pass"}, Executable: "/independent/native-test", ExecutableSha256: strings.Repeat("a", 64), ReportDir: filepath.Join(t.TempDir(), "reports")}
	inv, e := registry.Build(req)
	if e != nil {
		t.Fatal(e)
	}
	base := plan{Profile: profile, Request: req, Invocation: inv}
	for _, kind := range []string{"target-approval", "environment", "report", "executable"} {
		t.Run(kind, func(t *testing.T) {
			p := base
			p.Invocation.Phases = append([]tr.Phase{}, base.Invocation.Phases...)
			p.Invocation.Phases[0].Environment = map[string]string{}
			for k, v := range base.Invocation.Phases[0].Environment {
				p.Invocation.Phases[0].Environment[k] = v
			}
			p.Invocation.ReportPaths = append([]string{}, base.Invocation.ReportPaths...)
			approval := tr.Identity(base)
			want := "plan does not match current fixed runner profile"
			switch kind {
			case "target-approval":
				p.Request.Target = "Other"
				want = "plan admission or independently trusted executable mismatch"
			case "environment":
				p.Invocation.Phases[0].Environment["CMOCKA_MESSAGE_OUTPUT"] = "XML"
				approval = tr.Identity(p)
			case "report":
				p.Invocation.ReportPaths = []string{"other.xml"}
				approval = tr.Identity(p)
			case "executable":
				p.Request.Executable = "/other"
				approval = tr.Identity(p)
				want = "plan admission or independently trusted executable mismatch"
			}
			b, _ := json.Marshal(p)
			file := filepath.Join(t.TempDir(), "plan.json")
			if e := os.WriteFile(file, b, 0600); e != nil {
				t.Fatal(e)
			}
			var out, errout bytes.Buffer
			code := command(context.Background(), []string{"run", "--plan", file, "--approve", approval, "--out", filepath.Join(t.TempDir(), "receipt.json"), "--executable", req.Executable, "--executable-sha256", req.ExecutableSha256, "--experimental", "--trusted-local"}, &out, &errout)
			if code != 1 {
				t.Fatal("mutated plan admitted", kind, code)
			}
			if !strings.Contains(errout.String(), want) {
				t.Fatalf("%s refused for another reason: %s", kind, errout.String())
			}
			if _, e := os.Stat(req.ReportDir); !os.IsNotExist(e) {
				t.Fatal("invalid plan launched", e)
			}
		})
	}
}

func TestGinkgoPlanBindsTargetAndFixedArgv(t *testing.T) {
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	req := tr.Request{Runner: "ginkgo-v2", Target: "TestCalc::Calc", ExpectedTests: []string{"TestCalc::Calc::Math adds", "TestCalc::Calc::Math subtracts"}, Selectors: []string{"TestCalc::Calc::Math adds"}, Root: root, Project: "calc_suite_test.go", InputFiles: map[string]string{"calc_suite_test.go": strings.Repeat("b", 64)}, Executable: "/independent/calc.test", ExecutableSha256: strings.Repeat("a", 64), ReportDir: filepath.Join(t.TempDir(), "reports")}
	inv, e := registry.Build(req)
	if e != nil || inv.Argv[len(inv.Argv)-1] != `-ginkgo.focus=^Calc (Math adds)$` {
		t.Fatal(inv, e)
	}
	base := plan{Profile: profile, Request: req, Invocation: inv}
	for _, kind := range []string{"target-approval", "environment", "focus", "report", "executable"} {
		t.Run(kind, func(t *testing.T) {
			p := base
			p.Invocation.Argv = append([]string{}, base.Invocation.Argv...)
			p.Invocation.Environment = map[string]string{}
			for k, v := range base.Invocation.Environment {
				p.Invocation.Environment[k] = v
			}
			p.Invocation.ReportPaths = append([]string{}, base.Invocation.ReportPaths...)
			approval := tr.Identity(base)
			want := "plan does not match current fixed runner profile"
			switch kind {
			case "target-approval":
				p.Request.Target = "TestCalc::Other"
				want = "plan admission or independently trusted executable mismatch"
			case "environment":
				p.Invocation.Environment["GOPROXY"] = "https://proxy.golang.org"
				approval = tr.Identity(p)
			case "focus":
				p.Invocation.Argv[len(p.Invocation.Argv)-1] = `-ginkgo.focus=^(Math adds)$`
				approval = tr.Identity(p)
			case "report":
				p.Invocation.ReportPaths = []string{"other.json"}
				approval = tr.Identity(p)
			case "executable":
				p.Request.Executable = "/other"
				approval = tr.Identity(p)
				want = "plan admission or independently trusted executable mismatch"
			}
			b, _ := json.Marshal(p)
			file := filepath.Join(t.TempDir(), "plan.json")
			if e := os.WriteFile(file, b, 0600); e != nil {
				t.Fatal(e)
			}
			var out, errout bytes.Buffer
			code := command(context.Background(), []string{"run", "--plan", file, "--approve", approval, "--out", filepath.Join(t.TempDir(), "receipt.json"), "--executable", req.Executable, "--executable-sha256", req.ExecutableSha256, "--experimental", "--trusted-local"}, &out, &errout)
			if code != 1 {
				t.Fatal("mutated plan admitted", kind, code)
			}
			if !strings.Contains(errout.String(), want) {
				t.Fatalf("%s refused for another reason: %s", kind, errout.String())
			}
			if _, e := os.Stat(req.ReportDir); !os.IsNotExist(e) {
				t.Fatal("invalid plan launched", e)
			}
		})
	}
}

// Frozen bytes produced by the companion at origin/main 0c94c66c, before the
// additive expectedSelection field existed (V1-0620).
const (
	historicalNightwatchPlan    = `{"profile":"corvint-test-runner-plan/0","request":{"expectedTests":["tests/sample.js::sample::default::wd-1::pass"],"target":"","inputFiles":{"tests/sample.js":"bb"},"runner":"nightwatch","root":"/src","executable":"/x/nightwatch","executableSha256":"aa","selectors":null,"project":"","config":"","configSha256":"","reporter":"","reporterSha256":"","reportFiles":["sample.json"],"tools":null,"reportDir":"/r","timeoutSeconds":60},"invocation":{"outcomeNeutralExitCodes":null,"successExitCodes":[0],"failureExitCodes":[5],"argv":["--reporter=json","--output","/r"],"phases":null,"files":{},"reportPaths":["sample.json"],"reportPatterns":null,"environment":{},"format":"nightwatch"}}`
	historicalNightwatchReceipt = `{"profile":"corvint-test-runner-receipt/0","planSha256":"7ed7e187a85628d164eeaf4924e2ace4d88d17bfac1ab9a7ae2bb1b22992a13a","execution":{"profile":"corvint-test-runner-execution/0","runner":"nightwatch","inputSha256":"","invocationSha256":"","phases":null,"reportSha256":null,"executionAuthority":"","dependencyClosure":""},"observation":{"runner":"nightwatch","tests":[{"failureKind":"","message":"","id":"tests/sample.js::sample::default::wd-1::pass","name":"pass","file":"tests/sample.js","suite":"sample","state":"PASSED","attempts":[{"state":"PASSED","failureKind":"","message":""}]}],"problems":null,"complete":true,"retryInformation":"RETAINED"},"error":""}`
)

func TestHistoricalPlanAndReceiptBytesSurviveSelectionContract(t *testing.T) {
	for _, c := range []struct {
		raw string
		dst any
	}{{historicalNightwatchPlan, new(plan)}, {historicalNightwatchReceipt, new(receipt)}} {
		if e := tr.DecodeDocument([]byte(c.raw), c.dst); e != nil {
			t.Fatal(e)
		}
		b, e := json.Marshal(c.dst)
		if e != nil || string(b) != c.raw || tr.Identity(c.dst) != tr.Digest([]byte(c.raw)) {
			t.Fatalf("historical bytes or identity changed:\n%s\n%s", b, c.raw)
		}
	}
	var p plan
	tr.DecodeDocument([]byte(historicalNightwatchPlan), &p)
	// Detached retirement (TRE-V0-025) is additive: historical plans keep
	// their bytes and are executed without it.
	if p.Request.ExpectedSelection != nil || p.Invocation.RetireDetachedDescendants || tr.Identity(p) != "7ed7e187a85628d164eeaf4924e2ace4d88d17bfac1ab9a7ae2bb1b22992a13a" {
		t.Fatal("historical plan identity moved")
	}
	if _, e := registry.Build(p.Request); e != nil {
		t.Fatal("historical exact ExpectedTests plan no longer admitted", e)
	}
}

// TestNightwatchSelectionPreAdmittedAcrossFreshSessions drives plan and run
// with a pinned stand-in executable that writes the Nightwatch 3 module report
// shape with a WebDriver session ID chosen only at launch. No real browser or
// Nightwatch runs here; that live qualification is NOT_RUN.
func TestNightwatchSelectionPreAdmittedAcrossFreshSessions(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "nightwatch")
	script := "#!/bin/sh\nprintf '{\"name\":\"sample\",\"systemerr\":\"\",\"report\":{\"modulePath\":\"tests/sample.js\",\"sessionId\":\"wd-%s\",\"testEnv\":\"default\",\"testsCount\":1,\"errorsCount\":0,\"completed\":{\"pass\":{\"status\":\"pass\"}},\"skipped\":[\"skip\"]}}' \"$$\" > \"$3/sample.json\"\n"
	if e := os.WriteFile(bin, []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	identity := []string{"--executable", bin, "--executable-sha256", tr.Digest([]byte(script))}
	root := t.TempDir()
	source := []byte("module.exports = {};\n")
	if e := os.WriteFile(filepath.Join(root, "sample.js"), source, 0600); e != nil {
		t.Fatal(e)
	}
	const module = "tests/sample.js::sample::default::"
	request := func(keys ...string) tr.Request {
		return tr.Request{Runner: "nightwatch", Root: root, ReportDir: filepath.Join(t.TempDir(), "reports"), ReportFiles: []string{"sample.json"}, TimeoutSeconds: 60, InputFiles: map[string]string{"sample.js": tr.Digest(source)}, ExpectedSelection: &tr.Selection{Version: tr.SelectionVersion, Matcher: tr.NightwatchSessionElided, Tests: keys}}
	}
	planned := func(r tr.Request) (string, plan, int, string) {
		path := filepath.Join(t.TempDir(), "request.json")
		b, _ := json.Marshal(r)
		os.WriteFile(path, b, 0600)
		var out, errout bytes.Buffer
		code := command(context.Background(), append([]string{"plan", "--request", path}, identity...), &out, &errout)
		var p plan
		if code == 0 {
			if e := tr.DecodeDocument(out.Bytes(), &p); e != nil {
				t.Fatal(e)
			}
			path = filepath.Join(t.TempDir(), "plan.json")
			os.WriteFile(path, out.Bytes(), 0600)
		}
		return path, p, code, errout.String()
	}
	run := func(path string, p plan) (int, receipt) {
		out := filepath.Join(t.TempDir(), "receipt.json")
		var stdout, errout bytes.Buffer
		code := command(context.Background(), append([]string{"run", "--plan", path, "--approve", tr.Identity(p), "--out", out, "--experimental", "--trusted-local"}, identity...), &stdout, &errout)
		var r receipt
		b, e := os.ReadFile(out)
		if e != nil || json.Unmarshal(b, &r) != nil {
			t.Fatalf("missing receipt: %v %s", e, errout.String())
		}
		return code, r
	}
	sessions := map[string]bool{}
	for i := 0; i < 2; i++ {
		path, p, code, msg := planned(request(module+"pass", module+"skip"))
		if code != 0 || p.Request.ExpectedSelection == nil || p.Request.ExpectedSelection.Matcher != tr.NightwatchSessionElided {
			t.Fatalf("selection not admitted into the plan: %d %s", code, msg)
		}
		code, r := run(path, p)
		if code != 0 || !r.Observation.Complete || r.Error != "" || len(r.Observation.Tests) != 2 {
			t.Fatalf("run %d: exit=%d %+v", i, code, r)
		}
		for _, x := range r.Observation.Tests {
			parts := strings.Split(x.ID, "::")
			if len(parts) != 5 || !strings.HasPrefix(parts[3], "wd-") {
				t.Fatalf("native session identity not retained: %s", x.ID)
			}
			sessions[parts[3]] = true
		}
	}
	if len(sessions) != 2 {
		t.Fatalf("fresh launches did not produce differing sessions: %v", sessions)
	}
	path, p, _, _ := planned(request(module+"pass", module+"other"))
	if code, r := run(path, p); code != 1 || r.Observation.Complete {
		t.Fatalf("wrong expected selection completed: %d %+v", code, r.Observation)
	}
	for name, r := range map[string]tr.Request{"duplicate": request(module+"pass", module+"pass"), "session-bearing": request("tests/sample.js::sample::default::wd-1::pass")} {
		if _, _, code, msg := planned(r); code != 1 || !strings.Contains(msg, "selection") {
			t.Fatalf("%s selection planned: %d %s", name, code, msg)
		}
		// A hand-made plan carrying the invalid selection must refuse before launch.
		p := plan{Profile: profile, Request: r}
		p.Request.Executable, p.Request.ExecutableSha256 = bin, tr.Digest([]byte(script))
		file := filepath.Join(t.TempDir(), "plan.json")
		b, _ := json.Marshal(p)
		os.WriteFile(file, b, 0600)
		var out, errout bytes.Buffer
		if command(context.Background(), append([]string{"run", "--plan", file, "--approve", tr.Identity(p), "--out", filepath.Join(t.TempDir(), "receipt.json"), "--experimental", "--trusted-local"}, identity...), &out, &errout) != 1 || !strings.Contains(errout.String(), "selection") {
			t.Fatalf("%s selection ran: %s", name, errout.String())
		}
		if _, e := os.Stat(r.ReportDir); !os.IsNotExist(e) {
			t.Fatalf("%s selection launched", name)
		}
	}
}

// TestPreRetirementXCTestPlanRefusedForExecution binds TRE-V0-025's
// compatibility rule: a historical swift-xctest plan keeps its identity but is
// refused for new execution with a re-plan diagnosis, a current plan runs with
// retirement, and no plan can add retirement where its profile does not.
func TestPreRetirementXCTestPlanRefusedForExecution(t *testing.T) {
	r := tr.Request{Runner: "swift-xctest", Root: "/source", Config: "/source/Package.swift", ConfigSha256: strings.Repeat("a", 64), Executable: "/usr/bin/swift", ExecutableSha256: strings.Repeat("b", 64), ReportDir: "/fresh", TimeoutSeconds: 60, InputFiles: map[string]string{"Package.swift": strings.Repeat("a", 64)}, Selectors: []string{"ProofTests.Proof/testPass"}}
	current, e := registry.Build(r)
	if e != nil || !current.RetireDetachedDescendants {
		t.Fatalf("%+v %v", current, e)
	}
	historical := current
	historical.RetireDetachedDescendants = false
	if v, e := admittedInvocation(plan{Request: r, Invocation: current}); e != nil || !v.RetireDetachedDescendants {
		t.Fatalf("current plan not admitted with retirement: %+v %v", v, e)
	}
	if _, e := admittedInvocation(plan{Request: r, Invocation: historical}); e == nil || !strings.Contains(e.Error(), "re-plan") {
		t.Fatalf("pre-retirement plan admitted: %v", e)
	}
	changed := historical
	changed.Argv = append([]string{}, historical.Argv...)
	changed.Argv[len(changed.Argv)-1] = "^ProofTests.Proof/testFail$"
	if _, e := admittedInvocation(plan{Request: r, Invocation: changed}); e == nil {
		t.Fatal("changed historical invocation admitted")
	}
	n := tr.Request{Runner: "nightwatch", Root: "/src", Executable: "/x/nightwatch", ExecutableSha256: "aa", InputFiles: map[string]string{"tests/sample.js": "bb"}, ReportFiles: []string{"sample.json"}, ReportDir: "/r", TimeoutSeconds: 60}
	nv, e := registry.Build(n)
	if e != nil {
		t.Fatal(e)
	}
	nv.RetireDetachedDescendants = true
	if _, e := admittedInvocation(plan{Request: n, Invocation: nv}); e == nil {
		t.Fatal("plan added retirement outside its fixed profile")
	}
}
