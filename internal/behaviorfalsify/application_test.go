package behaviorfalsify

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"
)

var testMarker = ApplicationMarker{Kind: MarkerAnnotation, Type: "control-applied"}

func markedClassification() (Plan, PlannedControl, AttemptResult) {
	plan := classificationPlan()
	marker := testMarker
	plan.Controls[0].ApplicationMarker = &marker
	control := plan.Controls[0]
	success := successfulProcess()
	base := AttemptResult{Attempt: 1, Receipt: receiptPointer(classificationReceipt(plan, control)), HookProcess: success, CleanupProcess: success, WorkspaceBefore: digestN("workspace"), WorkspaceAfter: digestN("workspace")}
	return plan, control, base
}

func ownMarker(control PlannedControl) MarkerObservation {
	return MarkerObservation{Kind: testMarker.Kind, Type: testMarker.Type, ControlID: control.ID}
}

func TestBBFV0015ApplicationClassification(t *testing.T) {
	t.Parallel()
	plan, control, base := markedClassification()
	own := ownMarker(control)
	tests := []struct {
		name        string
		mutate      func(*AttemptResult)
		want        Status
		reason      string
		application string
	}{
		{name: "applied and failed", mutate: func(r *AttemptResult) { r.Receipt.Markers = []MarkerObservation{own} }, want: StatusKilled, reason: "expected-assertion-failed", application: ApplicationAppliedFailed},
		{name: "applied and passed", mutate: func(r *AttemptResult) { r.Receipt.Markers = []MarkerObservation{own}; survive(r) }, want: StatusSurvived, reason: "target-criterion-survived", application: ApplicationAppliedPassed},
		{name: "inert green run", mutate: func(r *AttemptResult) { survive(r) }, want: StatusInvalidControl, reason: "control-not-applied", application: ApplicationNotApplied},
		{name: "failure without the marker is not a kill", mutate: func(*AttemptResult) {}, want: StatusInvalidControl, reason: "control-not-applied", application: ApplicationNotApplied},
		{name: "marker emitted twice counts once", mutate: func(r *AttemptResult) { r.Receipt.Markers = []MarkerObservation{own, own}; survive(r) }, want: StatusSurvived, reason: "target-criterion-survived", application: ApplicationAppliedPassed},
		{name: "marker from a different control", mutate: func(r *AttemptResult) {
			foreign := own
			foreign.ControlID = "control:other"
			r.Receipt.Markers = []MarkerObservation{foreign}
		}, want: StatusInvalidControl, reason: "control-not-applied", application: ApplicationNotApplied},
		{name: "marker of another type", mutate: func(r *AttemptResult) {
			other := own
			other.Type = "other-annotation"
			r.Receipt.Markers = []MarkerObservation{other}
			survive(r)
		}, want: StatusInvalidControl, reason: "control-not-applied", application: ApplicationNotApplied},
		{name: "marker of another kind", mutate: func(r *AttemptResult) {
			other := own
			other.Kind = MarkerProviderEvent
			r.Receipt.Markers = []MarkerObservation{other}
		}, want: StatusInvalidControl, reason: "control-not-applied", application: ApplicationNotApplied},
		{name: "marker present but target step skipped", mutate: func(r *AttemptResult) {
			r.Receipt.Markers = []MarkerObservation{own}
			r.Receipt.TargetObservation.State = "skipped"
			r.Receipt.TargetObservation.FailureKind = ""
			r.Receipt.TestOutcome = "skipped"
		}, want: StatusInvalidControl, reason: "target-failed-for-unexpected-reason", application: ""},
		{name: "early failure plus marker", mutate: func(r *AttemptResult) {
			r.Receipt.Markers = []MarkerObservation{own}
			r.Receipt.Setup[0].State = "failed"
		}, want: StatusInvalidControl, reason: "required-setup-failed-or-missing", application: ApplicationAppliedEarlyFail},
		{name: "undeclared failed setup is not an early failure", mutate: func(r *AttemptResult) {
			r.Receipt.Markers = []MarkerObservation{own}
			r.Receipt.Setup = append(r.Receipt.Setup, SetupObservation{ID: "undeclared-setup", State: "failed"})
		}, want: StatusInvalidControl, reason: "required-setup-failed-or-missing", application: ""},
		{name: "early failure without marker stays unclassified", mutate: func(r *AttemptResult) { r.Receipt.Setup[0].State = "failed" }, want: StatusInvalidControl, reason: "required-setup-failed-or-missing", application: ""},
		{name: "malformed marker observation", mutate: func(r *AttemptResult) {
			bad := own
			bad.Kind = "console"
			r.Receipt.Markers = []MarkerObservation{own, bad}
		}, want: StatusInvalidControl, reason: "marker-observation-invalid", application: ""},
		{name: "marker observations over the bound", mutate: func(r *AttemptResult) {
			for range maxMarkerObservations + 1 {
				r.Receipt.Markers = append(r.Receipt.Markers, own)
			}
		}, want: StatusInvalidControl, reason: "marker-observation-invalid", application: ""},
		{name: "infrastructure loss keeps its class", mutate: func(r *AttemptResult) { r.HookProcess.TimedOut = true }, want: StatusInfrastructureFailed, reason: "execution-boundary-failed", application: ""},
	}
	for _, test := range tests {
		t.Run("BBF-V0-015 "+test.name, func(t *testing.T) {
			result := cloneAttempt(base)
			test.mutate(&result)
			status, reasons, application := classifyAttempt(plan, control, result)
			if status != test.want || !slices.Contains(reasons, test.reason) || application != test.application {
				t.Fatalf("got %q %v %q, want %q %q %q", status, reasons, application, test.want, test.reason, test.application)
			}
		})
	}
}

func TestBBFV0015UnmarkedControlKeepsLegacyClassification(t *testing.T) {
	t.Parallel()
	plan := classificationPlan()
	control := plan.Controls[0]
	result := AttemptResult{Attempt: 1, Receipt: receiptPointer(classificationReceipt(plan, control)), HookProcess: successfulProcess(), CleanupProcess: successfulProcess(), WorkspaceBefore: digestN("workspace"), WorkspaceAfter: digestN("workspace")}
	// Markers reported for an unmarked control are ignored rather than credited.
	result.Receipt.Markers = []MarkerObservation{ownMarker(control)}
	for _, outcome := range []Status{StatusKilled, StatusSurvived} {
		attempt := cloneAttempt(result)
		if outcome == StatusSurvived {
			survive(&attempt)
		}
		status, _, application := classifyAttempt(plan, control, attempt)
		if status != outcome || application != "" {
			t.Fatalf("unmarked control: got %q %q, want %q with no application", status, application, outcome)
		}
	}
}

func TestBBFV0015AggregateApplication(t *testing.T) {
	t.Parallel()
	plan, control, _ := markedClassification()
	_ = plan
	attempts := []AttemptResult{{Application: ApplicationAppliedEarlyFail}, {Application: ApplicationNotApplied}}
	if got := aggregateApplication(control, attempts, StatusInvalidControl); got != ApplicationNotApplied {
		t.Fatalf("any inert attempt makes the control inert, got %q", got)
	}
	if got := aggregateApplication(control, attempts[:1], StatusInvalidControl); got != ApplicationAppliedEarlyFail {
		t.Fatalf("early failure aggregate = %q", got)
	}
	if got := aggregateApplication(control, nil, StatusKilled); got != ApplicationAppliedFailed {
		t.Fatalf("killed aggregate = %q", got)
	}
	control.ApplicationMarker = nil
	if got := aggregateApplication(control, attempts, StatusInvalidControl); got != "" {
		t.Fatalf("unmarked aggregate = %q", got)
	}
}

func TestBBFV0015PlanMarkerValidation(t *testing.T) {
	t.Parallel()
	for _, marker := range []ApplicationMarker{{Kind: "console", Type: "control-applied"}, {Kind: MarkerAnnotation, Type: ""}, {Kind: MarkerProviderEvent, Type: "bad type"}} {
		control := liveControl(t, "control", "killed")
		control.ApplicationMarker = &marker
		if _, err := BuildPlan(liveRequest(t, []ControlSpec{control})); err == nil || !strings.Contains(err.Error(), "invalid application marker") {
			t.Fatalf("marker %+v: err = %v", marker, err)
		}
	}
}

func TestBBFV0016LegacyWireUnchanged(t *testing.T) {
	t.Parallel()
	plan, r := receiptFixture(t, "killed")
	encoded, err := EncodeEvidence(r)
	if err != nil {
		t.Fatal(err)
	}
	planBytes, err := Encode(plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"application_marker"`, `"APPLIED-`, `"NOT-APPLIED"`, `"inert_controls"`, `"markers"`} {
		if bytes.Contains(planBytes, []byte(key)) || bytes.Contains(encoded, []byte(key)) {
			t.Fatalf("unmarked plan or receipt carries %s", key)
		}
	}
}

func markedControl(t *testing.T, id, scenario, markers string) ControlSpec {
	t.Helper()
	control := liveControl(t, id, scenario)
	control.Definition["markers"] = markers
	marker := testMarker
	control.ApplicationMarker = &marker
	return control
}

func TestBBFV0016InertControlsListedSeparately(t *testing.T) {
	t.Parallel()
	plan, err := BuildPlan(liveRequest(t, []ControlSpec{
		markedControl(t, "control-applied", "killed", "self"),
		markedControl(t, "control-inert", "survived", "none"),
		liveControl(t, "control-unmarked", "survived"),
	}))
	if err != nil {
		t.Fatal(err)
	}
	r, err := ExecuteReceipt(context.Background(), plan, plan.Digest, receiptTool())
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyReceipt(r, plan.Digest, receiptTool().Executable); err != nil {
		t.Fatalf("%v: counts %v", err, r.Report.Counts)
	}
	report := r.Report
	got := map[string]string{}
	for _, result := range report.Results {
		got[result.ID] = string(result.Status) + "/" + result.Application
	}
	want := map[string]string{"control-applied": "killed/APPLIED-FAILED", "control-inert": "invalid_control/NOT-APPLIED", "control-unmarked": "survived/"}
	for id, value := range want {
		if got[id] != value {
			t.Fatalf("%s = %q, want %q", id, got[id], value)
		}
	}
	if !slices.Equal(report.InertControls, []string{"control-inert"}) || report.Counts[StatusSurvived] != 1 || report.MutationScore.Denominator != 2 || !slices.Contains(report.CoverageGaps, "control-inert") {
		t.Fatalf("inert control credited or unlisted: %+v", report)
	}
	raw, err := EncodeEvidence(r)
	if err != nil {
		t.Fatal(err)
	}
	inert := slices.IndexFunc(report.Results, func(result ControlResult) bool { return result.ID == "control-inert" })
	for name, mutate := range map[string]func(*EvidenceReceipt){
		"inert list dropped":       func(r *EvidenceReceipt) { r.Report.InertControls = nil },
		"control application lost": func(r *EvidenceReceipt) { r.Report.Results[inert].Application = "" },
		"attempt application lost": func(r *EvidenceReceipt) { r.Report.Results[inert].Attempts[0].Application = "" },
		"inert reclaimed as survivor": func(r *EvidenceReceipt) {
			r.Report.Results[inert].Status = StatusSurvived
			r.Report.Results[inert].Application = ApplicationAppliedPassed
			r.Report.InertControls = nil
		},
		"marker injected into the bound outcome": func(r *EvidenceReceipt) {
			r.Report.Results[inert].Attempts[0].Receipt.Markers = []MarkerObservation{{Kind: testMarker.Kind, Type: testMarker.Type, ControlID: "control-inert"}}
		},
	} {
		t.Run("BBF-V0-016 rejects "+name, func(t *testing.T) {
			var tampered EvidenceReceipt
			if err := Decode(raw, &tampered); err != nil {
				t.Fatal(err)
			}
			mutate(&tampered)
			tampered.Report.Digest = digestJSON(tampered.Report)
			tampered.Digest = evidenceDigest(tampered)
			if VerifyReceipt(tampered, plan.Digest, receiptTool().Executable) == nil {
				t.Fatal("tampering accepted")
			}
		})
	}
}

// inertHistoryReceipt executes one marked inert control at the fixture repository's current
// HEAD, optionally after a further commit so the receipt pins a distinct revision.
func inertHistoryReceipt(t *testing.T, request Request, markerType string) HistoryReceipt {
	t.Helper()
	request.Controls = []ControlSpec{markedControl(t, "control-inert", "survived", "none")}
	request.Controls[0].ApplicationMarker.Type = markerType
	plan, err := BuildPlan(request)
	if err != nil {
		t.Fatal(err)
	}
	r, err := ExecuteReceipt(context.Background(), plan, plan.Digest, receiptTool())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(r.Report.InertControls, []string{"control-inert"}) {
		t.Fatalf("fixture control is not inert: %+v", r.Report)
	}
	return HistoryReceipt{Receipt: r, PlanDigest: plan.Digest, ToolDigest: receiptTool().Executable}
}

func advanceRevision(t *testing.T, request Request, message string) Request {
	t.Helper()
	repository := request.Repositories.Application
	mustGitBytes(t, repository, "-c", "user.name=fixture", "-c", "user.email=fixture@example.test", "-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "-m", message)
	head := strings.TrimSpace(string(mustGitBytes(t, repository, "rev-parse", "HEAD")))
	request.Target.ApplicationRevision, request.Target.TestRevision, request.Target.DocumentationRevision = head, head, head
	return request
}

func TestBBFV0017RepeatedNotAppliedAcrossRevisions(t *testing.T) {
	t.Parallel()
	request := liveRequest(t, nil)
	first := inertHistoryReceipt(t, request, testMarker.Type)
	again := inertHistoryReceipt(t, request, testMarker.Type)
	later := advanceRevision(t, request, "next revision")
	second := inertHistoryReceipt(t, later, testMarker.Type)
	changed := inertHistoryReceipt(t, later, "changed-marker")

	t.Run("BBF-V0-017 one revision is not a repeat", func(t *testing.T) {
		defects, err := InertControlDefects([]HistoryReceipt{first, again})
		if err != nil || len(defects) != 0 {
			t.Fatalf("defects = %+v, err = %v", defects, err)
		}
	})
	t.Run("BBF-V0-017 a changed marker is a different control", func(t *testing.T) {
		defects, err := InertControlDefects([]HistoryReceipt{first, changed})
		if err != nil || len(defects) != 0 {
			t.Fatalf("defects = %+v, err = %v", defects, err)
		}
	})
	t.Run("BBF-V0-017 repeated NOT-APPLIED across revisions is a control defect", func(t *testing.T) {
		defects, err := InertControlDefects([]HistoryReceipt{second, first, again, changed})
		if err != nil || len(defects) != 1 {
			t.Fatalf("defects = %+v, err = %v", defects, err)
		}
		defect := defects[0]
		target := request.Target
		if defect.Reason != ReasonRepeatedNotApplied || defect.Control.ControlID != "control-inert" || defect.Control.Marker != testMarker || defect.Control.TestID != target.TestID || len(defect.Revisions) != 2 || defect.Revisions[0] == defect.Revisions[1] {
			t.Fatalf("unexpected defect %+v", defect)
		}
		reordered, err := InertControlDefects([]HistoryReceipt{changed, again, first, second})
		if err != nil || len(reordered) != 1 || !slices.Equal(reordered[0].Revisions, defect.Revisions) {
			t.Fatalf("history order changed the result: %+v", reordered)
		}
	})
	t.Run("BBF-V0-017 an unverifiable receipt refuses the history", func(t *testing.T) {
		tampered := second
		tampered.Receipt.Report.InertControls = nil
		tampered.Receipt.Report.Digest = digestJSON(tampered.Receipt.Report)
		tampered.Receipt.Digest = evidenceDigest(tampered.Receipt)
		if _, err := InertControlDefects([]HistoryReceipt{first, tampered}); err == nil || err.Error() != "history-receipt-invalid" {
			t.Fatalf("err = %v", err)
		}
		stale := second
		stale.PlanDigest = digestN("other-plan")
		if _, err := InertControlDefects([]HistoryReceipt{first, stale}); err == nil || err.Error() != "history-receipt-invalid" {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("BBF-V0-017 history is bounded", func(t *testing.T) {
		if _, err := InertControlDefects(nil); err == nil || err.Error() != "history-bound-exceeded" {
			t.Fatalf("empty history err = %v", err)
		}
		long := make([]HistoryReceipt, MaxHistoryReceipts+1)
		for index := range long {
			long[index] = first
		}
		if _, err := InertControlDefects(long); err == nil || err.Error() != "history-bound-exceeded" {
			t.Fatalf("oversized history err = %v", err)
		}
	})
}
