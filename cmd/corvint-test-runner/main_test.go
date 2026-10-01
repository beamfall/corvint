package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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
