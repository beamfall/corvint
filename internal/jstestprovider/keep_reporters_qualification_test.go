package jstestprovider

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// hexInputs gives a fixture the hex config-input digests a real reporter
// observes, plus a helper loaded by the config.
func hexInputs(r *Receipt) {
	digest := strings.Repeat("a", 64)
	r.Identity.ConfigDigest = digest
	r.Identity.ConfigInputDigests = map[string]string{r.Identity.ConfigFile: digest, "/repo/helper.cjs": strings.Repeat("d", 64)}
	for i := range r.Tests {
		r.Tests[i].Project.ConfigDigest = digest
		r.Tests[i].ID = qualifiedTestID(r.Identity, r.Tests[i])
	}
}

// keepQualificationPair is a replace-only control receipt and a provider-first
// keep-reporters receipt of the same configuration whose kept entries are all
// known.
func keepQualificationPair(t *testing.T) (Receipt, Receipt) {
	t.Helper()
	control := qualifiedFixture(t)
	keep := qualifiedFixture(t)
	hexInputs(&control)
	hexInputs(&keep)
	keep.External.ConfigOverride = "controlled-fixture-config" + keptConfigSuffix
	keep.Identity.ConfigInputDigests["/repo/project-reporter.cjs"] = strings.Repeat("b", 64)
	keep.Tests[0].ID = qualifiedTestID(keep.Identity, keep.Tests[0])
	if err := bindProjectReporters(&keep, []reportedProjectReporter{
		{Name: "list", Options: "absent"},
		{Name: "/repo/project-reporter.cjs", Options: "bound", OptionsDigest: strings.Repeat("c", 64)},
	}); err != nil {
		t.Fatal(err)
	}
	return control, keep
}

// PWP-V0-015: two complete, matching runs qualify the kept entries and bind
// every config input of the keep run, including files the config also loads.
func TestQualifyKeepReportersQualified(t *testing.T) {
	control, keep := keepQualificationPair(t)
	q := QualifyKeepReporters(control, nil, keep, nil)
	if q.Verdict != KeepReportersQualified || len(q.Reasons) != 0 || q.Tests != 1 {
		t.Fatalf("matching runs not qualified: %+v", q)
	}
	if !reflect.DeepEqual(q.ConfigInputs, keep.Identity.ConfigInputDigests) || q.ConfigInputs["/repo/helper.cjs"] == "" {
		t.Fatalf("config inputs %v", q.ConfigInputs)
	}
	// A reporter module the original config also imports is a control input
	// too; the record still binds it and encodes.
	shared := control
	shared.Identity.ConfigInputDigests = map[string]string{}
	for path, digest := range keep.Identity.ConfigInputDigests {
		shared.Identity.ConfigInputDigests[path] = digest
	}
	shared.Tests = []TestOutcome{control.Tests[0]}
	shared.Tests[0].ID = qualifiedTestID(shared.Identity, shared.Tests[0])
	if sharedRecord := QualifyKeepReporters(shared, nil, keep, nil); sharedRecord.Verdict != KeepReportersQualified {
		t.Fatalf("config-imported reporter not qualified: %v", sharedRecord.Reasons)
	} else if _, err := EncodeKeepReportersQualification(sharedRecord); err != nil {
		t.Fatalf("config-imported reporter record refused: %v", err)
	}
	if q.ReceiptProfile != ExternalProfile || q.RunnerVersion != "1.60.0" || q.NodeVersion != "v22" || len(q.ControlReceiptSHA256) != 64 || len(q.KeepReceiptSHA256) != 64 || q.ControlReceiptSHA256 == q.KeepReceiptSHA256 {
		t.Fatalf("record identity %+v", q)
	}
	encoded, err := EncodeKeepReportersQualification(q)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeKeepReportersQualification(encoded)
	if err != nil || !reflect.DeepEqual(decoded, q) {
		t.Fatalf("round trip: %v\n%s", err, encoded)
	}
}

// PWP-V0-015, PWP-V0-018: missing Playwright, a refused reporter and a
// partial run are not-run with a named reason, never a qualification; a
// mismatch is not-qualified.
func TestQualifyKeepReportersReasons(t *testing.T) {
	type run struct {
		r   Receipt
		err error
	}
	for name, test := range map[string]struct {
		mutate func(control, keep *run)
		want   string
		reason string
	}{
		"playwright-missing": {func(c, k *run) {
			c.r.Infrastructure = &InfrastructureFailure{Reason: "runner-unavailable"}
		}, KeepReportersNotRun, "keep-reporters-control-run-incomplete"},
		"control-error": {func(c, k *run) { c.err = errors.New("external-server-required") }, KeepReportersNotRun, "keep-reporters-control-run-incomplete"},
		"reporter-refused": {func(c, k *run) {
			k.r.External.ProjectReporters.Entries = nil
			k.r.Infrastructure = &InfrastructureFailure{Reason: projectReportersInvalid}
		}, KeepReportersNotRun, "keep-reporters-keep-run-incomplete"},
		"keep-partial":      {func(c, k *run) { k.r.Cancelled = true }, KeepReportersNotRun, "keep-reporters-keep-run-incomplete"},
		"keep-error":        {func(c, k *run) { k.err = errors.New("keep-reporters-unsupported-mode") }, KeepReportersNotRun, "keep-reporters-keep-run-incomplete"},
		"unbindable-input":  {func(c, k *run) { k.r.Identity.ConfigInputDigests["/repo/helper.cjs"] = "short" }, KeepReportersNotRun, "keep-reporters-keep-run-incomplete"},
		"keep-without-mode": {func(c, k *run) { k.r = c.r }, KeepReportersNotRun, "keep-reporters-keep-run-incomplete"},
		"control-with-mode": {func(c, k *run) { c.r = k.r }, KeepReportersNotRun, "keep-reporters-control-run-incomplete"},
		"legacy-order":      {func(c, k *run) { k.r.External.ConfigOverride = "controlled-fixture-config" + legacyKeptConfigSuffix }, KeepReportersNotQualified, "keep-reporters-order-unsupported"},
		"unknown-entry": {func(c, k *run) {
			k.r.External.ProjectReporters.Entries[1] = ProjectReporter{Name: "/outside/reporter.mjs", Module: "unknown", Options: "unknown"}
		}, KeepReportersNotQualified, "keep-reporters-entries-unknown"},
		"inputs-differ":   {func(c, k *run) { k.r.Identity.ConfigInputDigests["/repo/helper.cjs"] = strings.Repeat("f", 64) }, KeepReportersNotQualified, "keep-reporters-inputs-differ"},
		"runner-differs":  {func(c, k *run) { k.r.Identity.RunnerVersion = "1.61.0" }, KeepReportersNotQualified, "keep-reporters-inputs-differ"},
		"outcome-differs": {func(c, k *run) { k.r.Tests[0].State, k.r.Tests[0].Attempts[0].State = StateFailed, StateFailed }, KeepReportersNotQualified, "keep-reporters-observation-differs"},
		"retries-differ":  {func(c, k *run) { k.r.Tests[0].Retries = 1 }, KeepReportersNotQualified, "keep-reporters-observation-differs"},
		"test-missing":    {func(c, k *run) { k.r.Tests = []TestOutcome{} }, KeepReportersNotQualified, "keep-reporters-observation-differs"},
		"use-differs":     {func(c, k *run) { k.r.Tests[0].Project.Use = []byte(`{"headless":false}`) }, KeepReportersNotQualified, "keep-reporters-observation-differs"},
		"no-tests":        {func(c, k *run) { c.r.Tests, k.r.Tests = []TestOutcome{}, []TestOutcome{} }, KeepReportersNotQualified, "keep-reporters-no-tests"},
		"control-unknown": {func(c, k *run) { c.r.External.ReadyAtPublish = false }, KeepReportersNotQualified, "keep-reporters-control-unqualified"},
		"keep-unknown":    {func(c, k *run) { k.r.External.RunnerDescendantsGone = false }, KeepReportersNotQualified, "keep-reporters-keep-unqualified"},
		"duplicate-outcomes": {func(c, k *run) {
			c.r.Tests = append(c.r.Tests, c.r.Tests[0])
			k.r.Tests = append(k.r.Tests, k.r.Tests[0])
		}, KeepReportersNotQualified, "keep-reporters-observation-differs"},
	} {
		t.Run(name, func(t *testing.T) {
			control, keep := keepQualificationPair(t)
			c, k := run{r: control}, run{r: keep}
			test.mutate(&c, &k)
			q := QualifyKeepReporters(c.r, c.err, k.r, k.err)
			if q.Verdict != test.want || !contains(q.Reasons, test.reason) {
				t.Fatalf("got %s %v, want %s with %s", q.Verdict, q.Reasons, test.want, test.reason)
			}
			encoded, err := EncodeKeepReportersQualification(q)
			if err != nil {
				t.Fatalf("derived record refused: %v", err)
			}
			if decoded, err := DecodeKeepReportersQualification(encoded); err != nil || decoded.Verdict == KeepReportersQualified {
				t.Fatalf("decoded %v %v", decoded.Verdict, err)
			}
		})
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// PWP-V0-017: the record is closed and canonical; every refusal is the
// value-free keep-reporters-qualification-invalid.
func TestKeepReportersQualificationRecordClosed(t *testing.T) {
	control, keep := keepQualificationPair(t)
	q := QualifyKeepReporters(control, nil, keep, nil)
	encoded, err := EncodeKeepReportersQualification(q)
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{
		"unknown-field":   strings.Replace(string(encoded), `"profile"`, `"extra":1,"profile"`, 1),
		"non-canonical":   strings.Replace(string(encoded), `{"profile"`, `{ "profile"`, 1),
		"missing-newline": strings.TrimSuffix(string(encoded), "\n"),
		"trailing-value":  string(encoded) + "{}\n",
		"oversized":       strings.Repeat(" ", MaxKeepReportersQualificationSize+1),
	} {
		if _, err := DecodeKeepReportersQualification([]byte(data)); err == nil || err.Error() != keepReportersQualificationInvalid {
			t.Fatalf("%s decoded: %v", name, err)
		}
	}
	for name, mutate := range map[string]func(*KeepReportersQualification){
		"profile":                 func(q *KeepReportersQualification) { q.Profile = "other/0" },
		"verdict-without-reasons": func(q *KeepReportersQualification) { q.Verdict = KeepReportersNotQualified },
		"reason-with-verdict":     func(q *KeepReportersQualification) { q.Reasons = []string{"keep-reporters-no-tests"} },
		"invented-reason": func(q *KeepReportersQualification) {
			q.Reasons, q.Verdict = []string{"keep-reporters-trusted"}, KeepReportersNotQualified
		},
		"nil-reasons":               func(q *KeepReportersQualification) { q.Reasons = nil },
		"nil-inputs":                func(q *KeepReportersQualification) { q.ConfigInputs = nil },
		"relative-input":            func(q *KeepReportersQualification) { q.ConfigInputs["reporter.cjs"] = strings.Repeat("b", 64) },
		"input-digest-shape":        func(q *KeepReportersQualification) { q.ConfigInputs["/repo/project-reporter.cjs"] = "short" },
		"receipt-digest-shape":      func(q *KeepReportersQualification) { q.KeepReceiptSHA256 = "short" },
		"qualified-without-digest":  func(q *KeepReportersQualification) { q.ControlReceiptSHA256 = "" },
		"qualified-without-tests":   func(q *KeepReportersQualification) { q.Tests = 0 },
		"qualified-unknown-entry":   func(q *KeepReportersQualification) { q.Entries[1].Module, q.Entries[1].ModuleDigest = "unknown", "" },
		"qualified-module-unbound":  func(q *KeepReportersQualification) { delete(q.ConfigInputs, "/repo/project-reporter.cjs") },
		"qualified-nil-entries":     func(q *KeepReportersQualification) { q.Entries = nil },
		"qualified-default-profile": func(q *KeepReportersQualification) { q.ReceiptProfile = "" },
		"qualified-no-runner":       func(q *KeepReportersQualification) { q.RunnerVersion = "" },
		"unsorted-reasons": func(q *KeepReportersQualification) {
			q.Reasons, q.Verdict = []string{"keep-reporters-no-tests", "keep-reporters-inputs-differ"}, KeepReportersNotQualified
		},
		"secret-shaped": func(q *KeepReportersQualification) { q.NodeVersion = "AKIA" + strings.Repeat("A", 16) },
	} {
		t.Run(name, func(t *testing.T) {
			mutated := QualifyKeepReporters(control, nil, keep, nil)
			mutate(&mutated)
			if _, err := EncodeKeepReportersQualification(mutated); err == nil {
				t.Fatal("malformed record encoded")
			}
		})
	}
}

// PWP-V0-016: a keep-reporters receipt projects passing only with a carried
// qualification that matches it exactly; the legacy order and every mismatch
// keep abstaining.
func TestKeepReportersQualifiedProjection(t *testing.T) {
	control, keep := keepQualificationPair(t)
	q := QualifyKeepReporters(control, nil, keep, nil)
	carried := func(t *testing.T) Receipt {
		t.Helper()
		r := keep
		external := *keep.External
		reporters := *keep.External.ProjectReporters
		record := q
		record.Entries = append([]ProjectReporter{}, q.Entries...)
		record.ConfigInputs = map[string]string{}
		for path, digest := range q.ConfigInputs {
			record.ConfigInputs[path] = digest
		}
		reporters.Qualification = &record
		reporters.Entries = append([]ProjectReporter{}, keep.External.ProjectReporters.Entries...)
		external.ProjectReporters = &reporters
		r.External = &external
		r.Identity.ConfigInputDigests = map[string]string{}
		for path, digest := range keep.Identity.ConfigInputDigests {
			r.Identity.ConfigInputDigests[path] = digest
		}
		r.Tests = append([]TestOutcome{}, keep.Tests...)
		return r
	}
	if ReceiptTestProjection(keep, keep.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
		t.Fatal("keep receipt without a qualification projected passing")
	}
	r := carried(t)
	if ReceiptTestProjection(r, r.Tests[0]).Execution.State != testvalidity.ExecutionPassed {
		t.Fatal("matching qualification did not pass")
	}
	encoded, err := EncodeQualified(r)
	if err != nil || !strings.Contains(string(encoded), `"qualification":{"profile":"`+KeepReportersQualificationProfile+`"`) {
		t.Fatalf("qualification not retained: %v", err)
	}
	var document struct {
		Receipt Receipt `json:"receipt"`
	}
	if err := json.Unmarshal(encoded, &document); err != nil || ReceiptTestProjection(document.Receipt, document.Receipt.Tests[0]).Execution.State != testvalidity.ExecutionPassed {
		t.Fatalf("retained qualified receipt did not pass after decode: %v", err)
	}
	for name, mutate := range map[string]func(*Receipt){
		"runner-version": func(r *Receipt) { r.Identity.RunnerVersion = "1.61.0" },
		"node-version":   func(r *Receipt) { r.Identity.NodeVersion = "v24" },
		"profile":        func(r *Receipt) { r.External.ProjectReporters.Qualification.ReceiptProfile = AttemptExternalProfile },
		"entry-options":  func(r *Receipt) { r.External.ProjectReporters.Entries[1].OptionsDigest = strings.Repeat("d", 64) },
		"entry-added": func(r *Receipt) {
			r.External.ProjectReporters.Entries = append(r.External.ProjectReporters.Entries, ProjectReporter{Name: "dot", Module: "builtin", Options: "absent"})
		},
		"reporter-input-drift": func(r *Receipt) {
			r.Identity.ConfigInputDigests["/repo/project-reporter.cjs"] = strings.Repeat("e", 64)
			r.External.ProjectReporters.Entries[1].ModuleDigest = strings.Repeat("e", 64)
		},
		"input-missing": func(r *Receipt) {
			r.External.ProjectReporters.Qualification.ConfigInputs["/repo/other.cjs"] = strings.Repeat("f", 64)
		},
		"shared-helper-drift": func(r *Receipt) { r.Identity.ConfigInputDigests["/repo/helper.cjs"] = strings.Repeat("e", 64) },
		"new-input-loaded":    func(r *Receipt) { r.Identity.ConfigInputDigests["/repo/new.cjs"] = strings.Repeat("e", 64) },
	} {
		t.Run(name, func(t *testing.T) {
			r := carried(t)
			mutate(&r)
			r.Tests[0].ID = qualifiedTestID(r.Identity, r.Tests[0])
			if ReceiptTestProjection(r, r.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
				t.Fatal("mismatched qualification projected passing")
			}
		})
	}
	for name, mutate := range map[string]func(*Receipt){
		"legacy-order":        func(r *Receipt) { r.External.ConfigOverride = "controlled-fixture-config" + legacyKeptConfigSuffix },
		"not-qualified":       func(r *Receipt) { r.External.ProjectReporters.Qualification.Verdict = KeepReportersNotQualified },
		"invalid-record":      func(r *Receipt) { r.External.ProjectReporters.Qualification.Profile = "other/0" },
		"qualification-alone": func(r *Receipt) { r.External.ProjectReporters.Qualification.Reasons = nil },
	} {
		t.Run(name, func(t *testing.T) {
			r := carried(t)
			mutate(&r)
			if ReceiptTestProjection(r, r.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
				t.Fatal("invalid qualification projected passing")
			}
			if _, err := EncodeQualified(r); err == nil || !strings.Contains(err.Error(), projectReportersInvalid) {
				t.Fatalf("invalid qualification encoded: %v", err)
			}
		})
	}
}

// PWP-V0-016: a qualification is refused outside keep-reporters mode and
// unless it is a closed qualified record, before any process starts.
func TestKeepReportersQualificationRunGuards(t *testing.T) {
	control, keep := keepQualificationPair(t)
	q := QualifyKeepReporters(control, nil, keep, nil)
	if _, err := RunE2E(context.Background(), E2EConfig{ExternalServer: true, KeepReportersQualification: &q}); err == nil || err.Error() != "keep-reporters-qualification-requires-keep-reporters" {
		t.Fatalf("qualification without keep mode: %v", err)
	}
	bad := q
	bad.Verdict, bad.Reasons = KeepReportersNotQualified, []string{"keep-reporters-no-tests"}
	if _, err := RunE2E(context.Background(), E2EConfig{ExternalServer: true, KeepReporters: true, KeepReportersQualification: &bad}); err == nil || err.Error() != keepReportersQualificationInvalid {
		t.Fatalf("not-qualified record accepted: %v", err)
	}
}
