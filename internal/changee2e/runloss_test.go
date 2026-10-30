package changee2e

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/behaviorfalsify"
	"github.com/Beamfall/corvint/internal/jstestprovider"
)

// fatalRecorder captures a Fatalf message and stops the helper that called it.
type fatalRecorder struct {
	testing.TB
	msg string
}

func (f *fatalRecorder) Helper() {}

func (f *fatalRecorder) Fatalf(format string, args ...any) {
	f.msg = fmt.Sprintf(format, args...)
	panic(f)
}

// failureOutput runs requireVerdict against a recorder and returns its failure message.
func failureOutput(t *testing.T, p scenarioProof, want string) string {
	t.Helper()
	rec := &fatalRecorder{TB: t}
	func() {
		defer func() {
			if r := recover(); r != nil && r != any(rec) {
				panic(r)
			}
		}()
		requireVerdict(rec, p, want)
	}()
	return rec.msg
}

const lossSecret = "ghp_abcdefghijklmnopqrstuvwxyz0123456789"

// CCE-V0-116 (V1-2620): an omitted-event control whose hook run raised an unlocated run-level
// error keeps, per lost attempt, the qualify reason, the error message (bounded, path-free and
// secret-screened) and `unlocated`; an attempt that kept no receipt keeps its hook process
// evidence. The control's status, reasons and the verdict are unchanged, and a failed live
// verdict prints every loss.
func TestCCEV0116LostControlAttemptDetail(t *testing.T) {
	root, base := fixtureRepo(t)
	head := commitBehaviourChange(t, root)
	sc := compiledScenario(t, root, base, head)
	specs := controlSpecs(sc)
	setup := "control:omitted-event:setup-1"
	if !slices.ContainsFunc(specs, func(s controlSpec) bool { return s.id == setup }) {
		t.Fatalf("the compiled scenario has no %s control: %+v", setup, specs)
	}
	tree := t.TempDir()
	roots := treeRoots(tree)
	message := "\x1b[31mError: page.goto: net::ERR_CONNECTION_REFUSED at " + filepath.Join(tree, "e2e", "fixtures.ts") +
		"\n  from /Users/someone/.cache/ms-playwright/chromium token=" + lossSecret + "\n" + strings.Repeat("x", 2000)
	receipt := jstestprovider.Receipt{Tests: []jstestprovider.TestOutcome{{State: jstestprovider.StatePassed}},
		Infrastructure: &jstestprovider.InfrastructureFailure{Reason: jstestprovider.ReasonReporterGlobalError, Detail: message}}
	_, reason := qualify(receipt, nil, 1, roots)
	if reason != "infrastructure:reporter-global-error" {
		t.Fatalf("qualify reason %q, want the unlocated run-level error's infrastructure reason", reason)
	}
	detail := hookInfrastructureDetail(reason, receipt, nil, roots)
	if len(detail) > 4096 {
		t.Fatalf("hook detail is %d bytes, over BBF's 4096", len(detail))
	}
	lost := behaviorfalsify.AttemptResult{Attempt: 1, Status: behaviorfalsify.StatusInfrastructureFailed, Reasons: []string{"playwright-infrastructure"},
		Receipt: &behaviorfalsify.HookReceipt{TestOutcome: "infrastructure_failed", TargetObservation: behaviorfalsify.CriterionObservation{State: "not_run"},
			Infrastructure: &behaviorfalsify.InfrastructureFailure{Reason: "environment-unavailable", Detail: detail}}}
	noReceipt := behaviorfalsify.AttemptResult{Attempt: 2, Status: behaviorfalsify.StatusInfrastructureFailed,
		Reasons:     []string{"hook-process-failed", "execution-boundary-failed"},
		HookProcess: behaviorfalsify.ProcessEvidence{Started: true, TimedOut: true, Exit: -1, Error: "signal: killed " + filepath.Join(tree, "hook")}}
	var report behaviorfalsify.Report
	for _, s := range specs {
		r := behaviorfalsify.ControlResult{ID: s.id, Kind: s.kind, Status: behaviorfalsify.StatusKilled, Reasons: []string{"every-attempt-killed"},
			Attempts: []behaviorfalsify.AttemptResult{{Attempt: 1, Status: behaviorfalsify.StatusKilled}}}
		if s.id == setup {
			r.Status, r.Reasons, r.Attempts = behaviorfalsify.StatusInfrastructureFailed, []string{"one-or-more-attempts-lost-to-infrastructure"},
				[]behaviorfalsify.AttemptResult{lost, noReceipt}
		}
		report.Results = append(report.Results, r)
	}
	mismatch := assertionMessage(sc, `"Pending"`)
	f := &fakeRunner{sides: map[string]fakeSide{head: {states: repeat("passed")}, base: {states: repeat("failed"), message: mismatch, line: sc.AssertionLine}},
		controlReport: &report}
	p := f.env(root).proveScenario(bg, base, head, sc, 3)

	if p.Verdict != VerdictInconclusive || p.Note != "control-not-killed:"+setup {
		t.Fatalf("verdict %s (%s), want %s (control-not-killed:%s): classification must not change", p.Verdict, p.Note, VerdictInconclusive, setup)
	}
	var control controlProof
	for _, c := range p.Controls {
		if c.ID == setup {
			control = c
		} else if len(c.LostAttempts) != 0 {
			t.Fatalf("killed control %s carries lost attempts %+v", c.ID, c.LostAttempts)
		}
	}
	if control.Status != string(behaviorfalsify.StatusInfrastructureFailed) || !slices.Equal(control.Reasons, []string{"one-or-more-attempts-lost-to-infrastructure"}) {
		t.Fatalf("control %s status %s reasons %q changed", setup, control.Status, control.Reasons)
	}
	if len(control.LostAttempts) != 2 {
		t.Fatalf("lost attempts %+v, want both", control.LostAttempts)
	}
	got := control.LostAttempts[0]
	if got.Attempt != 1 || got.Reason != reason || got.ErrorLocation != locationUnlocated || !slices.Equal(got.AttemptReasons, []string{"playwright-infrastructure"}) {
		t.Fatalf("lost attempt 1 = %+v", got)
	}
	if !strings.HasPrefix(got.ErrorMessage, "Error: page.goto: net::ERR_CONNECTION_REFUSED at e2e/fixtures.ts") || len(got.ErrorMessage) > lossMessageBytes {
		t.Fatalf("lost attempt 1 message %q (%d bytes)", got.ErrorMessage, len(got.ErrorMessage))
	}
	second := control.LostAttempts[1]
	if second.Attempt != 2 || second.Reason != "" || !strings.Contains(second.HookProcess, "timed_out=true") || !strings.Contains(second.HookProcess, "error=signal: killed <path>") {
		t.Fatalf("lost attempt 2 = %+v", second)
	}
	doc, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, leaked := range []string{tree, "/Users/someone", lossSecret, "\\u001b"} {
		if strings.Contains(string(doc), leaked) {
			t.Fatalf("proof carries %q: %s", leaked, doc)
		}
	}
	if !strings.Contains(string(doc), `"lost_attempts":[{"attempt":1,"attempt_reasons":["playwright-infrastructure"],"reason":"infrastructure:reporter-global-error","error_location":"unlocated"`) {
		t.Fatalf("proof lacks the lost_attempts field: %s", doc)
	}

	out := failureOutput(t, p, VerdictPreserves)
	for _, want := range []string{"verdict = INCONCLUSIVE (control-not-killed:" + setup + ")", setup + " attempt 1: reason=\"infrastructure:reporter-global-error\"",
		`error_location="unlocated"`, "ERR_CONNECTION_REFUSED", setup + " attempt 2:", "timed_out=true"} {
		if !strings.Contains(out, want) {
			t.Fatalf("live failure output lacks %q:\n%s", want, out)
		}
	}
}

// CCE-V0-116 with CCE-V0-087: a located teardown's hook detail now carries its message, and the
// control still keeps exactly `teardown-failed at <file>:<line>` among its reasons; a one-line
// detail, as hooks wrote before, reads as the reason alone.
func TestCCEV0116TeardownDetailCompatible(t *testing.T) {
	tree := t.TempDir()
	roots := treeRoots(tree)
	receipt := jstestprovider.Receipt{Tests: []jstestprovider.TestOutcome{{State: jstestprovider.StatePassed}},
		Infrastructure: &jstestprovider.InfrastructureFailure{Reason: jstestprovider.ReasonReporterGlobalError, Detail: "Error: worker teardown",
			Location: &jstestprovider.Anchor{File: filepath.Join(tree, "e2e", "fixtures.ts"), Line: 9}}}
	_, reason := qualify(receipt, nil, 1, roots)
	teardown := FailureTeardownFailed + " at e2e/fixtures.ts:9"
	if reason != teardown {
		t.Fatalf("qualify reason %q", reason)
	}
	attempt := func(detail string) behaviorfalsify.ControlResult {
		return behaviorfalsify.ControlResult{ID: "control:omitted-event:setup-1", Kind: behaviorfalsify.OmittedEvent, Status: behaviorfalsify.StatusInfrastructureFailed,
			Reasons: []string{"one-or-more-attempts-lost-to-infrastructure"}, Attempts: []behaviorfalsify.AttemptResult{{Attempt: 1,
				Status: behaviorfalsify.StatusInfrastructureFailed, Receipt: &behaviorfalsify.HookReceipt{Infrastructure: &behaviorfalsify.InfrastructureFailure{Reason: "environment-unavailable", Detail: detail}}}}}
	}
	specs := []controlSpec{{id: "control:omitted-event:setup-1", kind: behaviorfalsify.OmittedEvent}}
	c := controlResults(specs, "sha256:plan", behaviorfalsify.Report{Results: []behaviorfalsify.ControlResult{attempt(hookInfrastructureDetail(reason, receipt, nil, roots))}})[0]
	if !slices.Contains(c.Reasons, teardown) || len(c.LostAttempts) != 1 || c.LostAttempts[0].ErrorLocation != "e2e/fixtures.ts:9" || c.LostAttempts[0].ErrorMessage != "Error: worker teardown" {
		t.Fatalf("teardown control %+v", c)
	}
	old := controlResults(specs, "sha256:plan", behaviorfalsify.Report{Results: []behaviorfalsify.ControlResult{attempt(teardown)}})[0]
	if !slices.Contains(old.Reasons, teardown) || old.LostAttempts[0].Reason != teardown || old.LostAttempts[0].ErrorMessage != "" {
		t.Fatalf("one-line detail control %+v", old)
	}
}

// CCE-V0-116: an unqualified side keeps its loss: an unlocated run-level error's reason, message
// and `unlocated`, or a provider run error's path-free message; a qualified side keeps none.
func TestCCEV0116SideLoss(t *testing.T) {
	root, base := fixtureRepo(t)
	head := commitBehaviourChange(t, root)
	sc := compiledScenario(t, root, base, head)
	f := &fakeRunner{sides: map[string]fakeSide{head: {states: repeat("passed"), globalError: "Error: browserType.launch: Target page closed"},
		base: {states: repeat("passed"), reason: "provider-error", runErr: errors.New("spawn /opt/hosted/node ENOENT")}}, controls: "killed"}
	p := f.env(root).proveScenario(bg, base, head, sc, 3)
	if p.Head.Qualified || p.Head.Loss == nil || !reflect.DeepEqual(*p.Head.Loss, runLoss{Reason: "infrastructure:reporter-global-error", ErrorLocation: locationUnlocated,
		ErrorMessage: "Error: browserType.launch: Target page closed"}) {
		t.Fatalf("head loss %+v", p.Head.Loss)
	}
	if p.Base.Loss == nil || !reflect.DeepEqual(*p.Base.Loss, runLoss{Reason: "provider-error", ErrorMessage: "spawn <path> ENOENT"}) {
		t.Fatalf("base loss %+v", p.Base.Loss)
	}
	if p.Verdict != VerdictInconclusive {
		t.Fatalf("verdict %s (%s)", p.Verdict, p.Note)
	}
	out := failureOutput(t, p, VerdictDiscriminates)
	for _, want := range []string{"receipts not qualified", `head side: reason="infrastructure:reporter-global-error"`, `error_location="unlocated"`, `base side: reason="provider-error"`, "spawn <path> ENOENT"} {
		if !strings.Contains(out, want) {
			t.Fatalf("live failure output lacks %q:\n%s", want, out)
		}
	}
	f = &fakeRunner{sides: map[string]fakeSide{head: {states: repeat("passed")}, base: {states: repeat("passed")}}, controls: "killed"}
	if q := f.env(root).proveScenario(bg, base, head, sc, 3); q.Head.Loss != nil || q.Base.Loss != nil || proofLosses(q) != "" {
		t.Fatalf("qualified sides carry a loss: %+v %+v", q.Head.Loss, q.Base.Loss)
	}
}

// CCE-V0-116 (review): host paths written as file URLs, with spaces or as drive paths leave no
// trace in a kept message, while a path inside the staged tree stays readable.
func TestCCEV0116LossTextRedactsHostPaths(t *testing.T) {
	tree := t.TempDir()
	in := "at file:///Users/alice/private/fixture.mjs:12:3 and /Users/alice smith/private notes/x.ts:4 " +
		`and C:\Users\alice\fixture.ts, in ` + filepath.Join(tree, "e2e", "app.spec.ts") + ":7 and file://" + filepath.Join(tree, "e2e", "a.ts")
	got := lossText(in, treeRoots(tree))
	for _, leaked := range []string{"alice", "smith", "private", "notes", tree} {
		if strings.Contains(got, leaked) {
			t.Fatalf("lossText kept %q: %q", leaked, got)
		}
	}
	for _, kept := range []string{"in e2e/app.spec.ts:7", "file://e2e/a.ts"} {
		if !strings.Contains(got, kept) {
			t.Fatalf("lossText lost %q: %q", kept, got)
		}
	}
}

// CCE-V0-116 (review): a long teardown reason BBF would accept alone stays whole and the hook
// detail it heads stays within BBF's 4096-byte bound, so the attempt keeps reading
// infrastructure_failed with its CCE-V0-087 attribution.
func TestCCEV0116HookDetailWithinBBFBound(t *testing.T) {
	tree := t.TempDir()
	roots := treeRoots(tree)
	file := filepath.Join(append([]string{tree, "e2e"}, slices.Repeat([]string{strings.Repeat("d", 200)}, 18)...)...)
	receipt := jstestprovider.Receipt{Infrastructure: &jstestprovider.InfrastructureFailure{Reason: jstestprovider.ReasonReporterGlobalError,
		Detail: strings.Repeat("Error: teardown ", 100), Location: &jstestprovider.Anchor{File: filepath.Join(file, "fixtures.ts"), Line: 9}}}
	_, reason := qualify(receipt, nil, 1, roots)
	if !strings.HasPrefix(reason, FailureTeardownFailed+" at e2e/") || len(reason) < 3500 || len(reason) > hookDetailBytes {
		t.Fatalf("reason is %d bytes: %.80q", len(reason), reason)
	}
	detail := hookInfrastructureDetail(reason, receipt, nil, roots)
	if len(detail) > hookDetailBytes {
		t.Fatalf("hook detail is %d bytes, over BBF's %d", len(detail), hookDetailBytes)
	}
	if d, _, _ := parseHookDetail(detail); d != reason {
		t.Fatalf("hook detail does not lead with the whole reason")
	}
	if long := strings.Repeat("r", hookDetailBytes); hookInfrastructureDetail(long, receipt, nil, roots) != long {
		t.Fatalf("an over-long reason gained detail")
	}
}

// CCE-V0-116: a qualified document the encoder refuses loses with the encoder's own refusal
// code, so the report names the refusing check rather than the generic refusal; the code is
// value-free and within the reason grammar, so no document text reaches the report.
func TestCCEV0116EncodeRefusalReasonNamesCode(t *testing.T) {
	root, base := fixtureRepo(t)
	head := commitBehaviourChange(t, root)
	sc := compiledScenario(t, root, base, head)
	roots := treeRoots(syntheticTree)
	clean := syntheticReceipt(sc, true, []string{"passed"}, "")
	if digest, reason := qualify(clean, nil, 1, roots); reason != "" || !strings.HasPrefix(digest, "sha256:") {
		t.Fatalf("clean receipt: digest %q reason %q", digest, reason)
	}
	shaped := syntheticReceipt(sc, true, []string{"failed"}, "token="+lossSecret)
	digest, reason := qualify(shaped, nil, 1, roots)
	if digest != "" || reason != "qualified-document-secret-shaped" || !reasonCode.MatchString(reason) || strings.Contains(reason, lossSecret) {
		t.Fatalf("secret-shaped document: digest %q reason %q", digest, reason)
	}
}
