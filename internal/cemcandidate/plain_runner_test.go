package cemcandidate

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	cw "github.com/Beamfall/corvint/internal/cem/wire"
	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// runnerDocs is a synthetic go-test plan pinning the neutral case's two Go
// sources at its target, and a complete receipt binding it with one passed
// test. Each admission case edits a copy before the documents are encoded.
type runnerDocs struct {
	plan    tr.PlanDocument
	receipt tr.ReceiptDocument
}

func neutralRunnerDocs(t *testing.T, repo, target string) runnerDocs {
	t.Helper()
	inputs := map[string]string{}
	for _, name := range []string{"greet.go", "greet_test.go"} {
		raw, e := exec.Command("git", "-C", repo, "cat-file", "blob", target+":"+name).Output()
		if e != nil {
			t.Fatal(e)
		}
		inputs[name] = tr.Digest(raw)
	}
	request := tr.Request{Runner: "go-test", Root: repo, Project: "example.com/greet", InputFiles: inputs, ExpectedTests: []string{"example.com/greet::TestGreetEmpty"}, Selectors: []string{"example.com/greet::TestGreetEmpty"}, TimeoutSeconds: 120}
	plan := tr.PlanDocument{Profile: runnerPlanProfile, Request: request, Invocation: tr.Invocation{Environment: map[string]string{}}}
	receipt := tr.ReceiptDocument{Profile: runnerReceiptProfile, PlanSha256: tr.Identity(plan),
		Execution:   tr.Execution{Runner: "go-test", InputSha256: tr.Identity(inputs)},
		Observation: tr.Observation{Runner: "go-test", Complete: true, Tests: []tr.Test{{ID: "example.com/greet::TestGreetEmpty", Name: "TestGreetEmpty", State: tr.Passed}}}}
	return runnerDocs{plan: plan, receipt: receipt}
}

// admit runs the plain producer with runner admission over the given runner
// file bytes and returns the bundle directory and the producer's error.
func admit(t *testing.T, r PlainRequest, out string, plan, receipt []byte) (string, error) {
	t.Helper()
	dir := realTemp(t)
	r.RunnerAdmission = json.RawMessage(strconv.Quote(RunnerAdmissionProfile))
	r.RunnerPlan = save(t, filepath.Join(dir, "plan.json"), plan)
	r.RunnerReceipt = save(t, filepath.Join(dir, "receipt.json"), receipt)
	_, e := AssemblePlain(context.Background(), r, out)
	return out, e
}

// TestCEMV1280RunnerAdmissionIsOptIn: a request naming any other admission
// profile is refused before any bundle exists, the default request keeps the
// opaque runner files of the public case (CEM-V1-240 tests), and an admitted
// receipt yields the same ACCEPTed plain map shape with the runner documents
// as its artifacts.
func TestCEMV1280RunnerAdmissionIsOptIn(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	r, out, _ := plainFixture(t)
	other := r
	other.RunnerAdmission = json.RawMessage(`"corvint-test-runner/1"`)
	if _, e := AssemblePlain(context.Background(), other, out); e == nil {
		t.Fatal("an unknown runner admission profile was accepted")
	}
	if _, e := os.Stat(out); !os.IsNotExist(e) {
		t.Fatalf("refused request left a bundle: %v", e)
	}
	docs := neutralRunnerDocs(t, r.Repository, r.Target)
	plan, receipt := mustJSON(t, docs.plan), mustJSON(t, docs.receipt)
	if _, e := admit(t, r, out, plan, receipt); e != nil {
		t.Fatalf("admitted receipt refused: %v", e)
	}
	for name, want := range map[string][]byte{"artifacts/runner-plan.json": plan, "artifacts/runner-receipt.json": receipt} {
		if got, e := os.ReadFile(filepath.Join(out, name)); e != nil || string(got) != string(want) {
			t.Fatalf("%s differs from the admitted runner document: %v", name, e)
		}
	}
	if _, e := os.Stat(filepath.Join(out, "stable.json")); e != nil {
		t.Fatalf("admitted receipt wrote no stable map: %v", e)
	}
}

// TestCEMV1280PresentAdmissionMemberIsNeverOpaque decodes request JSON through
// the CLI's closed decoder: a present runnerAdmission of "" or null is refused
// before any read, like any other value, rather than read as absent, so a
// failed receipt cannot reach an opaque ACCEPT; the admitted profile still
// refuses that receipt as failed.
func TestCEMV1280PresentAdmissionMemberIsNeverOpaque(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	r, _, _ := plainFixture(t)
	docs := neutralRunnerDocs(t, r.Repository, r.Target)
	docs.receipt.Observation.Tests[0].State = tr.Failed
	dir := realTemp(t)
	r.RunnerPlan = save(t, filepath.Join(dir, "plan.json"), mustJSON(t, docs.plan))
	r.RunnerReceipt = save(t, filepath.Join(dir, "receipt.json"), mustJSON(t, docs.receipt))
	raw := mustJSON(t, r)
	if strings.Contains(string(raw), "runnerAdmission") {
		t.Fatalf("absent member was encoded: %s", raw)
	}
	for member, want := range map[string]string{`""`: "", `null`: "", `"corvint-test-runner/0"`: "runner-receipt-failed"} {
		request := append(raw[:len(raw)-1:len(raw)-1], []byte(`,"runnerAdmission":`+member+`}`)...)
		var decoded PlainRequest
		if e := Decode(request, &decoded); e != nil {
			t.Fatalf("%s: closed decoder refused the request: %v", member, e)
		}
		out := filepath.Join(realTemp(t), "bundle")
		result, e := AssemblePlain(context.Background(), decoded, out)
		if e == nil {
			t.Fatalf("runnerAdmission %s produced %s with a failed receipt", member, result.Verification.Outcome)
		}
		if got := cemcode.CodeOf(e); got != want || want == "" && e.Error() != "incomplete plain stable assembly request" {
			t.Fatalf("runnerAdmission %s: code %q (%v), want %q", member, got, e, want)
		}
		if _, e := os.Stat(out); !os.IsNotExist(e) {
			t.Fatalf("runnerAdmission %s left a bundle: %v", member, e)
		}
	}
}

// TestCEMV1281To1285RunnerAdmissionRefusals: each admission check refuses its
// own control with its registered code and writes no bundle at all.
func TestCEMV1281To1285RunnerAdmissionRefusals(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	r, _, caseRoot := plainFixture(t)
	opaque := func(name string) []byte {
		b, e := os.ReadFile(filepath.Join(caseRoot, "artifacts", name))
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	cases := []struct {
		name, code string
		edit       func(*runnerDocs)
		raw        func(plan, receipt []byte) ([]byte, []byte)
	}{
		{name: "opaque public-case files", code: "runner-document-invalid", raw: func([]byte, []byte) ([]byte, []byte) {
			return opaque("runner-plan.json"), opaque("runner-receipt.json")
		}},
		{name: "unknown receipt member", code: "runner-document-invalid", raw: func(plan, receipt []byte) ([]byte, []byte) {
			return plan, append(receipt[:len(receipt)-1], []byte(`,"authority":"VERIFIED"}`)...)
		}},
		{name: "input path escapes", code: "runner-document-invalid", edit: func(d *runnerDocs) {
			d.plan.Request.InputFiles["../greet.go"] = d.plan.Request.InputFiles["greet.go"]
			d.receipt.Execution.InputSha256 = tr.Identity(d.plan.Request.InputFiles)
		}},
		{name: "receipt of another plan", code: "runner-receipt-tampered", edit: func(d *runnerDocs) {
			d.receipt.PlanSha256 = tr.Digest([]byte("another plan"))
		}},
		{name: "receipt over other inputs", code: "runner-receipt-tampered", edit: func(d *runnerDocs) {
			d.receipt.Execution.InputSha256 = tr.Identity(map[string]string{"greet.go": tr.Digest(nil)})
		}},
		{name: "receipt of another runner", code: "runner-receipt-tampered", edit: func(d *runnerDocs) {
			d.receipt.Execution.Runner = "pytest"
		}},
		{name: "plan pins earlier source", code: "runner-receipt-stale", edit: func(d *runnerDocs) {
			d.plan.Request.InputFiles["greet.go"] = tr.Digest([]byte("package greet\n"))
		}},
		{name: "plan pins a path absent at target", code: "runner-receipt-stale", edit: func(d *runnerDocs) {
			d.plan.Request.InputFiles["greet_extra.go"] = tr.Digest(nil)
		}},
		{name: "plan pins no inputs", code: "runner-receipt-stale", edit: func(d *runnerDocs) {
			d.plan.Request.InputFiles = map[string]string{}
		}},
		{name: "failed test", code: "runner-receipt-failed", edit: func(d *runnerDocs) {
			d.receipt.Observation.Tests = append(d.receipt.Observation.Tests, tr.Test{ID: "example.com/greet::TestGreetName", Name: "TestGreetName", State: tr.Failed})
		}},
		{name: "incomplete observation", code: "runner-receipt-failed", edit: func(d *runnerDocs) {
			d.receipt.Observation.Complete = false
		}},
		{name: "execution error", code: "runner-receipt-failed", edit: func(d *runnerDocs) {
			d.receipt.Error = "exit status 2"
		}},
		{name: "only skipped tests", code: "runner-receipt-skipped", edit: func(d *runnerDocs) {
			d.receipt.Observation.Tests[0].State = tr.Skipped
		}},
		{name: "no tests", code: "runner-receipt-skipped", edit: func(d *runnerDocs) {
			d.receipt.Observation.Tests = []tr.Test{}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			docs := neutralRunnerDocs(t, r.Repository, r.Target)
			if c.edit != nil {
				c.edit(&docs)
				// A plan edit re-binds the receipt so only the named check fails.
				if c.code != "runner-receipt-tampered" {
					docs.receipt.PlanSha256 = tr.Identity(docs.plan)
					docs.receipt.Execution.InputSha256 = tr.Identity(docs.plan.Request.InputFiles)
				}
			}
			plan, receipt := mustJSON(t, docs.plan), mustJSON(t, docs.receipt)
			if c.raw != nil {
				plan, receipt = c.raw(plan, receipt)
			}
			out, e := admit(t, r, filepath.Join(realTemp(t), "bundle"), plan, receipt)
			if got := cemcode.CodeOf(e); got != c.code {
				t.Fatalf("code %q (%v), want %q", got, e, c.code)
			}
			if _, e := os.Stat(out); !os.IsNotExist(e) {
				t.Fatalf("refused receipt left a bundle: %v", e)
			}
		})
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}

// TestCEMV1310UnadmittedReceiptIsLabelledOpaque proves that the only runner
// receipt record of a map assembled without runner admission is labelled
// opaque/0, that an admitted receipt keeps the corvint-test-runner-receipt/0
// label, and that unrelated JSON pinned as plan and receipt can no longer be
// read as a runner receipt by any reader of the map (CEM-V1-310, CEM-V1-312).
func TestCEMV1310UnadmittedReceiptIsLabelledOpaque(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	label := func(t *testing.T, out string) string {
		t.Helper()
		raw, e := os.ReadFile(filepath.Join(out, "stable.json"))
		if e != nil {
			t.Fatal(e)
		}
		d, e := cw.ParseStable(raw)
		if e != nil || len(d.Receipts) != 1 {
			t.Fatalf("stable map: %v", e)
		}
		return d.Receipts[0].Profile
	}
	r, out, _ := plainFixture(t)
	dir := realTemp(t)
	// A framework report and an unrelated provenance document, as in #1328.
	r.RunnerPlan = save(t, filepath.Join(dir, "provenance.json"), []byte(`{"_type":"https://in-toto.io/Statement/v1","subject":[]}`))
	r.RunnerReceipt = save(t, filepath.Join(dir, "report.json"), []byte(`{"numPassedTests":1,"success":true}`))
	if result, e := AssemblePlain(context.Background(), r, out); e != nil || result.Verification.Outcome != "ACCEPT" || result.ReceiptAuthority != "NOT_OBSERVED" {
		t.Fatalf("unadmitted assembly: %+v %v", result, e)
	}
	if got := label(t, out); got != cw.StableOpaqueReceiptProfile {
		t.Fatalf("unadmitted receipt label %q, want %q", got, cw.StableOpaqueReceiptProfile)
	}
	admitted, admittedOut, _ := plainFixture(t)
	docs := neutralRunnerDocs(t, admitted.Repository, admitted.Target)
	if _, e := admit(t, admitted, admittedOut, mustJSON(t, docs.plan), mustJSON(t, docs.receipt)); e != nil {
		t.Fatalf("admitted receipt refused: %v", e)
	}
	if got := label(t, admittedOut); got != cw.StableRunnerReceiptProfile {
		t.Fatalf("admitted receipt label %q, want %q", got, cw.StableRunnerReceiptProfile)
	}
}

// TestCEMV1280AdmitsContainerisedRSpecReceipt traces TRE-V0-083 and
// TRE-V0-085 through the one admitted profile: a corvint-test-runner rspec
// plan carrying an exec prefix, path map and the Corvint formatter, with a
// receipt that names shared-example inclusion locations, is admitted
// unchanged; editing any of those plan members without re-binding the receipt
// is refused as tampered, so a receipt cannot be replayed under another
// prefix, mapping or formatter.
func TestCEMV1280AdmitsContainerisedRSpecReceipt(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	r, _, _ := plainFixture(t)
	rspec := func() runnerDocs {
		d := neutralRunnerDocs(t, r.Repository, r.Target)
		d.plan.Request = tr.Request{Runner: "rspec", Root: r.Repository, Executable: "/usr/local/bin/docker", ReportDir: "/host/reports/run", InputFiles: d.plan.Request.InputFiles, TimeoutSeconds: 120,
			ExecPrefix: []string{"run", "--rm", "-v", "/host/reports:/reports", "ruby@sha256:" + tr.Digest(nil), "rspec"}, PathMap: []tr.PathMapping{{Host: "/host/reports", Container: "/reports"}}, Formatter: "corvint-rspec-json/0"}
		d.receipt.PlanSha256 = tr.Identity(d.plan)
		d.receipt.Execution.Runner, d.receipt.Observation.Runner = "rspec", "rspec"
		d.receipt.Observation.Tests = []tr.Test{{ID: "./spec/widget_spec.rb[1:2:1]", Name: "is valid", State: tr.Passed, InclusionLocations: []string{"./spec/widget_spec.rb:7"}}}
		return d
	}
	d := rspec()
	plan, receipt := mustJSON(t, d.plan), mustJSON(t, d.receipt)
	for _, member := range []string{`"execPrefix"`, `"pathMap"`, `"formatter"`} {
		if !strings.Contains(string(plan), member) {
			t.Fatalf("plan omits %s: %s", member, plan)
		}
	}
	if !strings.Contains(string(receipt), `"inclusionLocations":["./spec/widget_spec.rb:7"]`) {
		t.Fatalf("receipt omits inclusion locations: %s", receipt)
	}
	out := filepath.Join(realTemp(t), "bundle")
	if _, e := admit(t, r, out, plan, receipt); e != nil {
		t.Fatalf("containerised rspec receipt refused: %v", e)
	}
	if got, e := os.ReadFile(filepath.Join(out, "artifacts/runner-receipt.json")); e != nil || string(got) != string(receipt) {
		t.Fatalf("admitted receipt differs: %v", e)
	}
	for name, edit := range map[string]func(*tr.Request){
		"prefix":    func(q *tr.Request) { q.ExecPrefix[4] = "ruby:latest" },
		"path map":  func(q *tr.Request) { q.PathMap = []tr.PathMapping{{Host: "/host/reports", Container: "/elsewhere"}} },
		"formatter": func(q *tr.Request) { q.Formatter = "" },
	} {
		t.Run(name, func(t *testing.T) {
			d := rspec()
			edit(&d.plan.Request)
			out, e := admit(t, r, filepath.Join(realTemp(t), "bundle"), mustJSON(t, d.plan), mustJSON(t, d.receipt))
			if got := cemcode.CodeOf(e); got != "runner-receipt-tampered" {
				t.Fatalf("code %q (%v), want runner-receipt-tampered", got, e)
			}
			if _, e := os.Stat(out); !os.IsNotExist(e) {
				t.Fatalf("refused receipt left a bundle: %v", e)
			}
		})
	}
}
