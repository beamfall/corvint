package doccorpus

import (
	"context"
	"github.com/Beamfall/corvint/internal/contextindex"
	"os"
	"path/filepath"
	"testing"
)

func TestCorpusJourneyRequiresIndependentStepEvidence(t *testing.T) {
	t.Run("DCP-V1-008 journey", func(t *testing.T) {
		root, m := fixture(t)
		evidence := declaredEvidence(t, root, m, "src/view.ts", 1)
		var testInput Input
		for _, in := range m.Inputs {
			if in.Path == "src/view.ts" {
				testInput = in
			}
		}
		run := map[string]any{"receipt": map[string]any{"kind": "unit", "identity": map[string]any{"testFileDigests": map[string]string{"src/view.ts": testInput.SHA256}}, "appBuildAtStart": map[string]any{"unknown": true}, "appBuildAtPublish": map[string]any{"unknown": true}, "tests": []any{map[string]any{"name": "Summary visible", "fullName": "Summary visible", "state": "passed", "retries": 0, "durationMs": 1}}}}
		raw, err := Encode(run)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(root, "evidence"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "evidence/run.json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
		git(t, root, "add", "evidence/run.json")
		git(t, root, "commit", "-qm", "native run")
		runRev := git(t, root, "rev-parse", "HEAD")
		observed := JourneyRun{Schema: "corvint-corpus-journey-observations/1", RunSHA256: Digest(raw), SourceRevision: m.Repository.Revision, Test: "Summary visible", Cleanup: "passed", Steps: []StepObservation{{ID: "show", Action: "inspect", Operation: "summary", Expected: "Summary visible", Observed: "Summary visible", Passed: true}}}
		stepBytes, _ := Encode(observed)
		if err := os.WriteFile(filepath.Join(root, "evidence/steps.json"), stepBytes, 0600); err != nil {
			t.Fatal(err)
		}
		git(t, root, "add", "evidence/steps.json")
		git(t, root, "commit", "-qm", "independent ordered step observations")
		stepRev := git(t, root, "rev-parse", "HEAD")
		stepAnchor := Anchor{Repository: m.Repository.ID, Revision: stepRev, Path: "evidence/steps.json", Blob: git(t, root, "rev-parse", stepRev+":evidence/steps.json"), SHA256: Digest(stepBytes), Start: 1, End: 1, SpanSHA256: Digest(stepBytes), Authority: "external-provider", Kind: "observed", Reason: "independently retained exact ordered step observation"}
		observation := ObservationLink{ID: "journey:run", Subject: "journey:test", Input: "evidence/run.json", InputRevision: runRev, SourceRevision: m.Repository.Revision, RunID: Digest(raw), Test: "Summary visible", StepInput: "evidence/steps.json", StepRevision: stepRev, StepEvidence: []Anchor{stepAnchor}}
		record := ProviderRecord{Schema: ProviderSchema, ID: "journey", Version: "1", Source: m.Repository, Subjects: []Subject{{"journey:test", "test", "Summary visible", "journey", evidence}, {"journey:subject", "ui_surface", "Summary", "journey", evidence}}, Claims: []Claim{}, Relations: []Relation{}, Journeys: []Journey{{ID: "journey:summary", Subject: "journey:subject", Provider: "journey", Status: "generated_not_verified", Cleanup: "passed", Preconditions: []string{}, Steps: []Step{{ID: "show", Action: "inspect", Operation: "summary", Expected: "Summary visible", Observation: "journey:run", Evidence: evidence}}, Evidence: evidence}}, Observations: []ObservationLink{observation}, Capabilities: []CapabilityDeclaration{{"subjects", "present", "explicit subjects"}, {"journeys", "present", "explicit journey"}, {"observations", "present", "native receipt and independent step observation"}}}
		providerBytes, _ := Encode(record)
		if err := os.WriteFile(filepath.Join(root, "evidence/provider.json"), providerBytes, 0600); err != nil {
			t.Fatal(err)
		}
		git(t, root, "add", "evidence/provider.json")
		git(t, root, "commit", "-qm", "journey declarations")
		providerRev := git(t, root, "rev-parse", "HEAD")
		m.Providers = append(m.Providers, Provider{ID: "journey", Kind: "records", Version: "1", Revision: providerRev, Record: "evidence/provider.json"})
		for _, in := range []struct {
			path, rev, purpose string
			data               []byte
		}{{"evidence/run.json", runRev, "observation", raw}, {"evidence/steps.json", stepRev, "evidence", stepBytes}, {"evidence/provider.json", providerRev, "provider", providerBytes}} {
			m.Scopes = append(m.Scopes, Scope{in.path, in.rev})
			m.Inputs = append(m.Inputs, Input{Path: in.path, Revision: in.rev, Blob: git(t, root, "rev-parse", in.rev+":"+in.path), SHA256: Digest(in.data), Provider: "journey", Purpose: in.purpose})
		}
		a, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		if a.Journeys[0].Status != "verified" {
			t.Fatalf("exact native run/step join did not qualify: %+v %+v", a.Journeys, a.Observations)
		}
		// This is an explicitly labelled synthetic contract fixture, not an observed
		// browser journey. Removing independent step evidence must remove the status.
		for _, change := range []string{"missing-steps", "mismatched-step", "wrong-cleanup", "stale-run", "incomplete-run", "e2e-unknown-build"} {
			t.Run(change, func(t *testing.T) {
				c := &compiler{manifest: m, sources: map[string]contextindex.Source{inputKey(stepRev, "evidence/steps.json"): {Data: stepBytes}}}
				step := a.Journeys[0].Steps[0]
				o := a.Observations[0]
				cleanup := "passed"
				switch change {
				case "missing-steps":
					o.Link.StepEvidence = nil
				case "mismatched-step":
					step.Expected = "invented"
				case "wrong-cleanup":
					cleanup = "failed"
				case "incomplete-run":
					o.Document.Run.Execution.State = "INCOMPLETE"
				case "e2e-unknown-build":
					o.Document.Run.Freshness.State = "UNKNOWN"
				case "stale-run":
					o.Document.Run.Freshness.State = "STALE"
				}
				if c.stepVerified(step, o, cleanup, 0, 1) {
					t.Fatal("missing/mismatched independent evidence qualified")
				}
			})
		}
	})
}
