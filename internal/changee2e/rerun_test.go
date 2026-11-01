package changee2e

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/behaviorfalsify"
)

// CCE-V0-940 (V1-2620): a control lost to infrastructure reruns once, alone, before the verdict;
// its first run's losses stay in retried_lost_attempts. A killed, survived or teardown-attributed
// control never reruns, a second loss keeps reading control-not-killed, and no rerun runs when no
// control was lost.
func TestCCEV0940LostControlRerunOnce(t *testing.T) {
	root, base := fixtureRepo(t)
	head := commitBehaviourChange(t, root)
	sc := compiledScenario(t, root, base, head)
	specs := controlSpecs(sc)
	setup := "control:omitted-event:setup-1"
	if !slices.ContainsFunc(specs, func(s controlSpec) bool { return s.id == setup }) || len(specs) < 2 {
		t.Fatalf("the compiled scenario needs %s and another control: %+v", setup, specs)
	}
	lostAttempt := func(detail string) behaviorfalsify.AttemptResult {
		return behaviorfalsify.AttemptResult{Attempt: 1, Status: behaviorfalsify.StatusInfrastructureFailed, Reasons: []string{"playwright-infrastructure"},
			Receipt: &behaviorfalsify.HookReceipt{TestOutcome: "infrastructure_failed", TargetObservation: behaviorfalsify.CriterionObservation{State: "not_run"},
				Infrastructure: &behaviorfalsify.InfrastructureFailure{Reason: "environment-unavailable", Detail: detail}}}
	}
	// report reads every control killed except those named in odd, which read the given status.
	report := func(odd map[string]behaviorfalsify.Status, detail string) *behaviorfalsify.Report {
		var r behaviorfalsify.Report
		for _, s := range specs {
			c := behaviorfalsify.ControlResult{ID: s.id, Kind: s.kind, Status: behaviorfalsify.StatusKilled, Reasons: []string{"every-attempt-killed"},
				Attempts: []behaviorfalsify.AttemptResult{{Attempt: 1, Status: behaviorfalsify.StatusKilled}}}
			switch odd[s.id] {
			case behaviorfalsify.StatusInfrastructureFailed:
				c.Status, c.Reasons, c.Attempts = behaviorfalsify.StatusInfrastructureFailed, []string{"one-or-more-attempts-lost-to-infrastructure"},
					[]behaviorfalsify.AttemptResult{lostAttempt(detail)}
			case behaviorfalsify.StatusSurvived:
				c.Status, c.Reasons, c.Attempts = behaviorfalsify.StatusSurvived, []string{"one-or-more-attempts-survived"},
					[]behaviorfalsify.AttemptResult{{Attempt: 1, Status: behaviorfalsify.StatusSurvived}}
			}
			r.Results = append(r.Results, c)
		}
		return &r
	}
	lost := map[string]behaviorfalsify.Status{setup: behaviorfalsify.StatusInfrastructureFailed}
	other := specs[0].id
	if other == setup {
		other = specs[1].id
	}
	infra := "infrastructure:reporter-global-error"
	teardown := FailureTeardownFailed + " at e2e/fixtures.ts:9"
	mismatch := assertionMessage(sc, `"Pending"`)
	for _, tc := range []struct {
		name        string
		first       *behaviorfalsify.Report
		rerun       *behaviorfalsify.Report // nil: the rerun reads the first report again
		wantCalls   [][]string
		wantVerdict string
		wantNote    string
		retried     bool // the setup control carries retried_lost_attempts
	}{
		{"lost then killed", report(lost, infra), report(nil, ""), [][]string{nil, {setup}}, VerdictDiscriminates, "", true},
		{"lost twice", report(lost, infra), nil, [][]string{nil, {setup}}, VerdictInconclusive, "control-not-killed:" + setup, true},
		{"lost then survived", report(lost, infra), report(map[string]behaviorfalsify.Status{setup: behaviorfalsify.StatusSurvived}, ""),
			[][]string{nil, {setup}}, VerdictControlSurvived, "control-survived:" + setup, true},
		{"no loss", report(nil, ""), nil, [][]string{nil}, VerdictDiscriminates, "", false},
		{"teardown-attributed loss never reruns", report(lost, teardown), report(nil, ""), [][]string{nil}, VerdictInconclusive, teardown + " (control " + setup + ")", false},
		{"survivor beside a loss: only the loss reruns", report(map[string]behaviorfalsify.Status{setup: behaviorfalsify.StatusInfrastructureFailed, other: behaviorfalsify.StatusSurvived}, infra),
			report(nil, ""), [][]string{nil, {setup}}, VerdictControlSurvived, "control-survived:" + other, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeRunner{sides: map[string]fakeSide{head: {states: repeat("passed")}, base: {states: repeat("failed"), message: mismatch, line: sc.AssertionLine}},
				controlReport: tc.first, rerunReport: tc.rerun}
			p := f.env(root).proveScenario(bg, base, head, sc, 3)
			if !reflect.DeepEqual(f.controlCalls, tc.wantCalls) {
				t.Fatalf("control calls %q, want %q", f.controlCalls, tc.wantCalls)
			}
			if p.Verdict != tc.wantVerdict || tc.wantNote != "" && p.Note != tc.wantNote {
				t.Fatalf("verdict %s (%s), want %s (%s)", p.Verdict, p.Note, tc.wantVerdict, tc.wantNote)
			}
			for _, c := range p.Controls {
				if c.ID != setup {
					if len(c.RetriedLostAttempts) != 0 || c.PlanDigest != "sha256:plan" {
						t.Fatalf("control %s was rerun: %+v", c.ID, c)
					}
					continue
				}
				if !tc.retried {
					if len(c.RetriedLostAttempts) != 0 {
						t.Fatalf("control %s carries retried losses without a rerun: %+v", c.ID, c)
					}
					continue
				}
				if c.PlanDigest != "sha256:rerun" || len(c.RetriedLostAttempts) != 1 || c.RetriedLostAttempts[0].Reason != infra {
					t.Fatalf("rerun control %+v, want the rerun's plan and the first run's loss %q", c, infra)
				}
				if (tc.rerun == nil) != (len(c.LostAttempts) == 1) {
					t.Fatalf("rerun control lost attempts %+v", c.LostAttempts)
				}
			}
			doc, err := json.Marshal(p)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(doc), `"retried_lost_attempts"`) != tc.retried {
				t.Fatalf("encoded proof retried_lost_attempts presence != %v: %s", tc.retried, doc)
			}
		})
	}
}
