package cemcandidate

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// omissionCase is the V1-2599 fixture both stable readers check: runner files
// whose receipt records a FAILED test, and the opaque single-form map that
// cites them (CEM-V1-633).
const omissionCase = "interop/cem01-go/testdata/omission"

// omissionRunnerDocs derives the omission case's runner files from the
// two-profile case's go-test pair: the plan also selects TestGreetName, which
// the target tree defines, and the receipt records it FAILED.
func omissionRunnerDocs(t *testing.T, multiRoot string) ([]byte, []byte) {
	t.Helper()
	var plan tr.PlanDocument
	var receipt tr.ReceiptDocument
	readJSON(t, filepath.Join(multiRoot, "artifacts", "runner-plan-0.json"), &plan)
	readJSON(t, filepath.Join(multiRoot, "artifacts", "runner-receipt-0.json"), &receipt)
	name := "example.com/greet::TestGreetName"
	plan.Request.ExpectedTests = append(plan.Request.ExpectedTests, name)
	plan.Request.Selectors = append(plan.Request.Selectors, name)
	receipt.PlanSha256 = tr.Identity(plan)
	receipt.Observation.Tests = append(receipt.Observation.Tests, tr.Test{ID: name, Name: "TestGreetName", State: tr.Failed})
	return mustJSON(t, plan), mustJSON(t, receipt)
}

// TestCEMV1633ProducerKeepsAFailedReceiptOnlyWithoutAdmission: without
// runnerAdmission the plain producer writes the receipt that records a FAILED
// test as an opaque reference, and its map equals the omission case's
// failed-cited.json byte for byte; with admission the same receipt is refused
// runner-receipt-failed (CEM-V1-284, CEM-V1-286). The readers' verdicts over
// that map are pinned by the native and portable CEM-V1-633 tests.
func TestCEMV1633ProducerKeepsAFailedReceiptOnlyWithoutAdmission(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	r, out, multiRoot := multiRunnerFixture(t)
	caseRoot := filepath.Join(filepath.Dir(multiRoot), filepath.Base(omissionCase))
	plan, receipt := omissionRunnerDocs(t, multiRoot)
	inputs := filepath.Dir(r.SourceMap.Path)
	r.Runners, r.RunnerAdmission = nil, nil
	r.RunnerPlan = save(t, filepath.Join(inputs, "omission-plan.json"), plan)
	r.RunnerReceipt = save(t, filepath.Join(inputs, "omission-receipt.json"), receipt)
	for i := range r.Criteria {
		r.Criteria[i].RunnerReceiptSha256s = nil
	}
	result, e := AssemblePlain(context.Background(), r, out)
	if e != nil || result.Verification.Outcome != "ACCEPT" {
		t.Fatalf("unadmitted single form over a failed receipt: %+v, %v", result, e)
	}
	for bundled, committed := range map[string]string{"stable.json": "failed-cited.json", "artifacts/runner-plan.json": "artifacts/runner-plan.json", "artifacts/runner-receipt.json": "artifacts/runner-receipt.json"} {
		got, e := os.ReadFile(filepath.Join(out, bundled))
		if e != nil {
			t.Fatal(e)
		}
		want, e := os.ReadFile(filepath.Join(caseRoot, committed))
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("%s differs from the omission case's %s:\n%s", bundled, committed, got)
		}
	}
	admitted := filepath.Join(realTemp(t), "bundle")
	if _, e := admit(t, r, admitted, plan, receipt); cemcode.CodeOf(e) != "runner-receipt-failed" {
		t.Fatalf("admitted failed receipt: %v (code %q), want runner-receipt-failed", e, cemcode.CodeOf(e))
	}
	if _, e := os.Stat(admitted); !os.IsNotExist(e) {
		t.Fatalf("refused admission left a bundle: %v", e)
	}
}
