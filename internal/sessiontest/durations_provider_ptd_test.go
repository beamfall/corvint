// SPDX-License-Identifier: Apache-2.0

package sessiontest

import (
	"testing"

	"github.com/Beamfall/corvint/internal/testduration"
)

// The provider documents below are hand-written in the shape corvint-js-test-provider prints: a
// {"receipt": ...} object whose tests are internal/jstestprovider TestOutcome values (receipt.go:
// name, fullName, state, retries, durationMs, anchor, attemptDetails[].{state, retry, durationMs}).
// The provider fills durationMs from Vitest's JSON reporter assertionResults[].duration
// (jstestprovider/vitest.go) and from the Playwright JSON reporter's results[].duration, one result
// per retry (jstestprovider/playwright.go). Neither runtime is installed on the delivering host.

// PTD-V1-001 "Vitest receipt", PTD-V1-002, PTD-V1-004: a Vitest unit whose `math > adds` passed in
// 3 ms records durationMs 3, durationSource vitest-json, granularity file. With one selected file
// every test the provider ran is that file's, though Vitest names no file for a passing test.
func TestPTDV1001VitestProviderReceiptDuration(t *testing.T) {
	document := `{"receipt": {"kind": "unit", "cancelled": false, "staleAppBuild": false, "tests": [
		{"name": "adds", "fullName": "math > adds", "state": "passed", "retries": 0, "durationMs": 3},
		{"name": "skips", "fullName": "math > skips", "state": "skipped", "retries": 0, "durationMs": 0}]}}`
	outcomes, reasons, _ := providerRun([]string{"typescript:vitest:src/math.test.ts"}, 0, document)
	if len(reasons) != 0 || len(outcomes) != 1 {
		t.Fatalf("outcomes %+v reasons %v", outcomes, reasons)
	}
	got := outcomes[0]
	if got.Outcome != OutcomePass || got.DurationMs == nil || *got.DurationMs != 3 || got.DurationSource != testduration.SourceVitest || got.DurationGranularity != testduration.GranularityFile {
		t.Fatalf("vitest outcome %+v", got)
	}
}

// PTD-V1-002, PTD-V1-003, PTD-V1-006, PTD-V1-007: Playwright file units sum their tests' times;
// a test retried once (failed in 900 ms, passed in 700 ms) counts 1600 from its attempt details;
// a retried test with only its last attempt's time leaves its file NOT_MEASURED; a negative time
// makes its file invalid. Several Vitest files whose passing tests name no file are NOT_MEASURED.
func TestPTDV1002ProviderReceiptDurations(t *testing.T) {
	document := `{"receipt": {"kind": "e2e", "tests": [
		{"name": "logs in", "fullName": "logs in", "state": "flaky", "retries": 1, "durationMs": 700,
		 "anchor": {"file": "e2e/login.spec.ts", "line": 3},
		 "attemptDetails": [{"state": "failed", "retry": 0, "durationMs": 900}, {"state": "passed", "retry": 1, "durationMs": 700}]},
		{"name": "logs out", "fullName": "logs out", "state": "passed", "retries": 0, "durationMs": 250.4,
		 "anchor": {"file": "e2e/login.spec.ts", "line": 9}},
		{"name": "pays", "fullName": "pays", "state": "flaky", "retries": 1, "durationMs": 300,
		 "anchor": {"file": "e2e/pay.spec.ts", "line": 4}},
		{"name": "lists", "fullName": "lists", "state": "passed", "retries": 0, "durationMs": -5,
		 "anchor": {"file": "e2e/list.spec.ts", "line": 2}}]}}`
	units := []string{"typescript:playwright:e2e/login.spec.ts", "typescript:playwright:e2e/pay.spec.ts", "typescript:playwright:e2e/list.spec.ts"}
	outcomes, reasons, _ := providerRun(units, 0, document)
	if len(reasons) != 0 || len(outcomes) != 3 {
		t.Fatalf("outcomes %+v reasons %v", outcomes, reasons)
	}
	login, pay, list := outcomes[0], outcomes[1], outcomes[2]
	if login.Outcome != OutcomePass || login.DurationMs == nil || *login.DurationMs != 1850 || login.DurationSource != testduration.SourcePlaywright || login.DurationGranularity != testduration.GranularityFile {
		t.Fatalf("login outcome %+v", login)
	}
	if pay.Outcome != OutcomePass || pay.DurationMs != nil || pay.DurationSource != testduration.NotMeasured {
		t.Fatalf("a retry with only its last attempt's time was measured: %+v", pay)
	}
	if list.DurationMs != nil || list.DurationSource != testduration.NotMeasured || !list.durationInvalid {
		t.Fatalf("negative time outcome %+v", list)
	}
	receipt := Finish(Receipt{ShardIndex: 0, Invocations: []Invocation{{Outcomes: outcomes}}}, Manifest{})
	if len(receipt.DurationInvalid) != 1 || receipt.DurationInvalid[0] != units[2] {
		t.Fatalf("durationInvalid %v", receipt.DurationInvalid)
	}

	vitest := `{"receipt": {"kind": "unit", "tests": [
		{"name": "adds", "fullName": "math > adds", "state": "passed", "retries": 0, "durationMs": 3},
		{"name": "joins", "fullName": "text > joins", "state": "passed", "retries": 0, "durationMs": 4}]}}`
	outcomes, _, _ = providerRun([]string{"typescript:vitest:src/math.test.ts", "typescript:vitest:src/text.test.ts"}, 0, vitest)
	for _, got := range outcomes {
		if got.Outcome != OutcomePass || got.DurationMs != nil || got.DurationSource != testduration.NotMeasured || got.DurationGranularity != testduration.GranularityFile {
			t.Fatalf("an unattributable Vitest time was placed: %+v", got)
		}
	}
	// A provider document without durationMs (an older provider) is NOT_MEASURED, not 0 ms.
	older := `{"receipt": {"kind": "unit", "tests": [{"fullName": "math > adds", "state": "passed"}]}}`
	outcomes, _, _ = providerRun([]string{"typescript:vitest:src/math.test.ts"}, 0, older)
	if got := outcomes[0]; got.DurationMs != nil || got.DurationSource != testduration.NotMeasured {
		t.Fatalf("a document without times was measured: %+v", got)
	}
}
