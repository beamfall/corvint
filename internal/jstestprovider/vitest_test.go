package jstestprovider

import (
	"os"
	"testing"
)

// TestParseVitestJSON_MixedStates parses a real vitest@5.0.0
// `--reporter=json` capture (one passing, one failing, one skipped
// assertion in one file - see evidence/ipr-08.md step 1) and checks the
// state mapping and the stack-trace-derived anchor.
func TestParseVitestJSON_MixedStates(t *testing.T) {
	data, err := os.ReadFile("testdata/vitest-mixed.json")
	if err != nil {
		t.Fatal(err)
	}
	outcomes, infra, err := ParseVitestJSON(data)
	if err != nil || infra != nil {
		t.Fatalf("err=%v infra=%+v", err, infra)
	}
	if len(outcomes) != 3 {
		t.Fatalf("want 3 outcomes, got %d: %+v", len(outcomes), outcomes)
	}
	byTitle := map[string]TestOutcome{}
	for _, o := range outcomes {
		byTitle[o.Name] = o
	}
	pass, ok := byTitle["adds two positive numbers"]
	if !ok || pass.State != StatePassed {
		t.Fatalf("want passed, got %+v", pass)
	}
	fail, ok := byTitle["adds a negative number"]
	if !ok || fail.State != StateFailed {
		t.Fatalf("want failed, got %+v", fail)
	}
	if fail.Anchor == nil || fail.Anchor.Line != 12 {
		t.Fatalf("want anchor at math.test.js:12, got %+v", fail.Anchor)
	}
	skip, ok := byTitle["is skipped on purpose"]
	if !ok || skip.State != StateSkipped {
		t.Fatalf("want skipped, got %+v", skip)
	}
}

// TestParseVitestJSON_FileLevelFailureWithPassingAssertions parses a real
// vitest@5.0.0 capture of a file whose afterAll hook threw after its only
// test passed: the file is "failed" with a message and no failed assertion.
// That failure must surface as an infrastructure outcome, never vanish
// behind the one passing assertion.
func TestParseVitestJSON_FileLevelFailureWithPassingAssertions(t *testing.T) {
	data, err := os.ReadFile("testdata/vitest-afterall-failure.json")
	if err != nil {
		t.Fatal(err)
	}
	outcomes, infra, err := ParseVitestJSON(data)
	if err != nil || infra != nil {
		t.Fatalf("err=%v infra=%+v", err, infra)
	}
	if len(outcomes) != 2 {
		t.Fatalf("want the passing assertion plus a file-level outcome, got %+v", outcomes)
	}
	if outcomes[0].State != StatePassed {
		t.Fatalf("want the assertion passed, got %+v", outcomes[0])
	}
	file := outcomes[1]
	if file.State != StateInfrastructure || file.FullName != "/fixture/hook.test.js" || file.FailureMessage != "teardown exploded" {
		t.Fatalf("want a file-level infrastructure outcome, got %+v", file)
	}
}

// TestParseVitestJSON_NoTestFiles parses a real vitest@5.0.0 capture of a run
// that found no test files (exit 1, empty testResults) and requires the same
// run-level no-suites-collected failure the Playwright path reports.
func TestParseVitestJSON_NoTestFiles(t *testing.T) {
	data, err := os.ReadFile("testdata/vitest-no-test-files.json")
	if err != nil {
		t.Fatal(err)
	}
	outcomes, infra, err := ParseVitestJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if infra == nil || infra.Reason != "no-suites-collected" || len(outcomes) != 0 {
		t.Fatalf("want no-suites-collected, got infra=%+v outcomes=%+v", infra, outcomes)
	}
}

// TestVitestAnchorFrom_NoStackFrame confirms the anchor parser returns nil,
// not a fabricated location, when a failure message carries no recognizable
// stack frame - vitest 5.0.0's JSON reporter has no structured location
// field (see runner-selection memo section 5), so nil here is "no anchor
// recoverable," never a guessed one.
func TestVitestAnchorFrom_NoStackFrame(t *testing.T) {
	if a := vitestAnchorFrom("some opaque error with no stack"); a != nil {
		t.Fatalf("want nil anchor, got %+v", a)
	}
}
