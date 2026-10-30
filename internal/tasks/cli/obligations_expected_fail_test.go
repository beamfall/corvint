package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// expectedFail is a test.fail(...) test: expectedStatus failed, a test-level
// fail annotation carrying description, and one retry-0 result.
func expectedFail(project, description, status string, steps ...pwStep) pwTest {
	if steps == nil {
		steps = []pwStep{}
	}
	return pwTest{ExpectedStatus: "failed", ProjectName: project, Annotations: []pwAnnotation{{Type: "fail", Description: description}},
		Results: []pwResult{{Status: status, Steps: steps, Stdout: []string{}}}}
}

// stepErr is a step that failed with message.
func stepErr(title, message string) pwStep {
	return pwStep{Title: title, Error: map[string]string{"message": message}, Steps: []pwStep{}}
}

// defectList renders defectConfirmed as id:defect:error.
func defectList(v wire.Value) string {
	var out []string
	for _, x := range field(v, "defectConfirmed").Arr {
		out = append(out, field(x, "id").Str+":"+field(x, "defect").Str+":"+field(x, "error").Str)
	}
	return strings.Join(out, ",")
}

// mixedList renders mixedExpectedFail as id@tests and checks each remedy.
func mixedList(t *testing.T, v wire.Value) string {
	t.Helper()
	var out []string
	for _, x := range field(v, "mixedExpectedFail").Arr {
		var tests []string
		for _, s := range field(x, "tests").Arr {
			tests = append(tests, s.Str)
		}
		if !strings.Contains(field(x, "remedy").Str, "own test") {
			t.Fatalf("mixed entry has no remedy: %s", wire.Encode(x))
		}
		out = append(out, field(x, "id").Str+"@"+strings.Join(tests, "+"))
	}
	return strings.Join(out, ",")
}

// TestTOLV0022_ExpectedFailDefectConfirmed: a test.fail test whose own
// obligation step failed with an error naming a defect id, or whose title id
// sits under a fail annotation naming one, is reported defectConfirmed with
// the defect id and the error's first line; it is never credited, never
// listed as failed, and the entry stays OPEN (witness never sets DEFECT).
func TestTOLV0022_ExpectedFailDefectConfirmed(t *testing.T) {
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	spec := "e2e/login.spec.ts"
	report := writeReport(t, t.TempDir(), r.Root, pwVersion,
		pwSpec{Title: "receipt", ID: "f1", File: spec, Tests: []pwTest{expectedFail("chromium", "known defect", "failed",
			stepErr("AC-5 failing", "Error: BUG-17 receipt is blank\n    at receipt.spec.ts:4"))}},
		pwSpec{Title: "AC-4 conflicting", ID: "f2", File: spec, Tests: []pwTest{expectedFail("chromium", "BUG-21 export hangs", "failed")}},
		pwSpec{Title: "steps", ID: "f3", File: spec, Tests: []pwTest{expectedFail("chromium", "", "failed",
			stepErr("AC-3 sibling expected-fail", "Timeout 5000ms exceeded"))}},
	)
	x := atm(t, r.Root, nil, witnessArgs(id, "w-defect", obligationRevision(t, r.Root, id), head, "--from-playwright-report", report)...)
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("witness: %s", x.stdout)
	}
	item := x.res.Items[0]
	if got := defectList(item); got != "AC-3::Timeout 5000ms exceeded,AC-4:BUG-21:,AC-5:BUG-17:Error: BUG-17 receipt is blank" {
		t.Fatalf("defectConfirmed %q: %s", got, x.stdout)
	}
	if field(item, "written").Bool || strList(item, "credited") != "" || strList(item, "failed") != "" || strList(item, "conflicting") != "" {
		t.Fatalf("an expected failure was credited or failed: %s", x.stdout)
	}
	for eid, st := range entryStates(obligationsShow(t, r.Root, id)) {
		if st != "OPEN" {
			t.Fatalf("%s is %s", eid, st)
		}
	}
	// A defect step that unexpectedly passed confirms nothing: it is failed.
	fixed := writeReport(t, t.TempDir(), r.Root, pwVersion,
		pwSpec{Title: "receipt", ID: "f1", File: spec, Tests: []pwTest{expectedFail("chromium", "BUG-17", "passed", step("AC-5 failing expected-fail", false))}})
	y := atm(t, r.Root, nil, witnessArgs(id, "w-fixed", obligationRevision(t, r.Root, id), head, "--from-playwright-report", fixed)...)
	if y.res.Outcome != wire.OutcomeOK || defectList(y.res.Items[0]) != "" || strList(y.res.Items[0], "failed") != "AC-5" {
		t.Fatalf("unexpected pass: %s", y.stdout)
	}
}

// TestTOLV0023_MixedExpectedFail: an ordinary obligation named inside a
// test.fail test, passing or failing, is reported mixedExpectedFail with the
// test and the remedy, not credited and not plain failed; the expected-fail
// obligation beside it stays defectConfirmed, and other tests in the same
// report still credit.
func TestTOLV0023_MixedExpectedFail(t *testing.T) {
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	spec := "e2e/login.spec.ts"
	report := writeReport(t, t.TempDir(), r.Root, pwVersion,
		pwSpec{Title: "AC-1 login", ID: "s1", File: spec, Tests: []pwTest{passed("chromium")}},
		pwSpec{Title: "steps", ID: "m1", File: spec, Tests: []pwTest{expectedFail("chromium", "known defect", "failed",
			step("AC-3 sibling", false), step("AC-2 soft", true), stepErr("AC-5 failing", "BUG-17: receipt is blank"))}},
	)
	x := atm(t, r.Root, nil, witnessArgs(id, "w-mixed", obligationRevision(t, r.Root, id), head, "--from-playwright-report", report)...)
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("witness: %s", x.stdout)
	}
	item := x.res.Items[0]
	if got := mixedList(t, item); got != "AC-2@m1@chromium,AC-3@m1@chromium" {
		t.Fatalf("mixedExpectedFail %q: %s", got, x.stdout)
	}
	if got := defectList(item); got != "AC-5:BUG-17:BUG-17: receipt is blank" {
		t.Fatalf("defectConfirmed %q", got)
	}
	if strList(item, "credited") != "AC-1" || strList(item, "failed") != "" || !field(item, "written").Bool {
		t.Fatalf("lists: %s", x.stdout)
	}
	if st := entryStates(obligationsShow(t, r.Root, id)); st["AC-3"] != "OPEN" || st["AC-1"] != "WITNESSED" {
		t.Fatalf("states: %v", st)
	}
}

// TestTOLV0024_PostCheckRefusesCredit: a run whose post-check exited
// non-zero credits nothing and quotes the log's first actionable line; a
// passing post-check credits as before; the two flags go together and only
// with a report.
func TestTOLV0024_PostCheckRefusesCredit(t *testing.T) {
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	report := writeReport(t, t.TempDir(), r.Root, pwVersion,
		pwSpec{Title: "AC-1 login", ID: "s1", File: "e2e/login.spec.ts", Tests: []pwTest{passed("chromium")}})
	dir := t.TempDir()
	log := filepath.Join(dir, "post-check.log")
	fixture.Write(t, log, []byte("\n> contract-check e2e\nchecking 12 witnesses\n  ✘ login.witness does not match declared pattern /^ok$/\n1 problem\n"))
	rev := obligationRevision(t, r.Root, id)
	before := fixture.TreeSnapshot(t, r.StateDir)
	x := atm(t, r.Root, nil, witnessArgs(id, "w-post", rev, head, "--from-playwright-report", report, "--post-check", log, "--post-check-status", "1")...)
	if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeGateFailed) || !strings.Contains(string(x.stdout), "OBLIGATION_POST_CHECK_FAILED:") ||
		!strings.Contains(string(x.stdout), "login.witness does not match declared pattern") {
		t.Fatalf("failed post-check: %s", x.stdout)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Fatal("a failed post-check wrote state")
	}
	for _, args := range [][]string{
		{"--from-playwright-report", report, "--post-check", log},
		{"--from-playwright-report", report, "--post-check-status", "0"},
		{"--from-playwright-report", report, "--post-check", log, "--post-check-status", "x"},
		{"--declared", "AC-9", "--manifest-sha256", strings.Repeat("a", 64), "--test-id", "t", "--reason", "r", "--post-check", log, "--post-check-status", "0"},
	} {
		if y := atm(t, r.Root, nil, witnessArgs(id, "w-usage", rev, head, args...)...); y.res.Outcome == wire.OutcomeOK {
			t.Fatalf("%v admitted: %s", args, y.stdout)
		}
	}
	ok := atm(t, r.Root, nil, witnessArgs(id, "w-post-ok", rev, head, "--from-playwright-report", report, "--post-check", log, "--post-check-status", "0")...)
	if ok.res.Outcome != wire.OutcomeOK || strList(ok.res.Items[0], "credited") != "AC-1" {
		t.Fatalf("passing post-check: %s", ok.stdout)
	}
}
