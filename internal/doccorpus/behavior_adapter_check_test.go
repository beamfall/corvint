package doccorpus

import (
	"bytes"
	jsonstd "encoding/json"
	"errors"
	"strings"
	"testing"
)

func behaviorAdapterEditDocument(t *testing.T, request *BehaviorAdapterRequest, inputID string, edit func(map[string]any)) {
	t.Helper()
	input := behaviorAdapterRequestInput(request, inputID)
	var document map[string]any
	if err := jsonstd.Unmarshal([]byte(input.Document), &document); err != nil {
		t.Fatal(err)
	}
	edit(document)
	input.Document = string(behaviorAdapterRaw(t, document))
	input.Anchor.SHA256 = Digest([]byte(input.Document))
	input.Anchor.SpanSHA256 = input.Anchor.SHA256
}

func behaviorAdapterCheckStates(report BehaviorAdapterCheck) map[string]BehaviorAdapterCheckStage {
	states := map[string]BehaviorAdapterCheckStage{}
	for _, stage := range report.Stages {
		states[stage.Stage] = stage
	}
	return states
}

// TestBehaviorAdapterCheckParity proves DCP-V1-044: a check is accepted exactly
// when Build succeeds, its first refusal is Build's error, and the report is
// deterministic.
func TestBehaviorAdapterCheckParity(t *testing.T) {
	fixture := behaviorAdapterFixture(t)
	other := strings.Repeat("e", 40)
	previousResult := buildBehaviorAdapter(t, fixture.request, nil)
	previousRaw, err := Encode(previousResult)
	if err != nil {
		t.Fatal(err)
	}
	staleResult := previousResult
	staleResult.Fallback = "none"
	staleRaw, err := Encode(staleResult)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		previous []byte
		raw      []byte
		edit     func(*testing.T, *BehaviorAdapterRequest)
	}{
		{name: "accepted"},
		{name: "accepted with previous", previous: previousRaw},
		{name: "request decode", raw: []byte(`{"schema":`)},
		{name: "previous decode", previous: []byte(`[]`)},
		{name: "previous lineage", previous: staleRaw},
		{name: "identity", edit: func(t *testing.T, r *BehaviorAdapterRequest) { r.Schema = "unknown" }},
		{name: "revisions", edit: func(t *testing.T, r *BehaviorAdapterRequest) { r.DocumentationRevision = "bad" }},
		{name: "mapping bound", edit: func(t *testing.T, r *BehaviorAdapterRequest) { r.Mappings = r.Mappings[:3] }},
		{name: "input anchor", edit: func(t *testing.T, r *BehaviorAdapterRequest) {
			behaviorAdapterRequestInput(r, "flows").Anchor.Repository = other
		}},
		{name: "required input", edit: func(t *testing.T, r *BehaviorAdapterRequest) { r.MigrationInput = "absent" }},
		{name: "mapping field", edit: func(t *testing.T, r *BehaviorAdapterRequest) { r.Mappings[3].Fields["bogus"] = "/bogus" }},
		{name: "observation identity", edit: func(t *testing.T, r *BehaviorAdapterRequest) { r.Observations[0].RunID = "bad" }},
		{name: "migration", edit: func(t *testing.T, r *BehaviorAdapterRequest) {
			behaviorAdapterEditDocument(t, r, "migration", func(document map[string]any) { document["contract_id"] = "other" })
		}},
		{name: "discovery", edit: func(t *testing.T, r *BehaviorAdapterRequest) {
			behaviorAdapterEditDocument(t, r, "discovery", func(document map[string]any) { document["mode"] = "static" })
		}},
		{name: "flow shape", edit: func(t *testing.T, r *BehaviorAdapterRequest) {
			behaviorAdapterEditRow(t, r, "flows", func(row map[string]any) { row["pages"] = "not-a-list" })
		}},
		{name: "test project", edit: func(t *testing.T, r *BehaviorAdapterRequest) {
			behaviorAdapterEditRow(t, r, "tests", func(row map[string]any) { row["browserProject"] = "" })
		}},
		{name: "observation subject", edit: func(t *testing.T, r *BehaviorAdapterRequest) { r.Observations[0].Subject = "other" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := fixture.request
			request.Inputs = append([]BehaviorAdapterInput(nil), fixture.request.Inputs...)
			request.Mappings = append([]BehaviorAdapterMapping(nil), fixture.request.Mappings...)
			request.Mappings[3].Fields = map[string]string{}
			for key, value := range fixture.request.Mappings[3].Fields {
				request.Mappings[3].Fields[key] = value
			}
			request.Observations = append([]ObservationLink(nil), fixture.request.Observations...)
			if tc.edit != nil {
				tc.edit(t, &request)
			}
			raw := tc.raw
			if raw == nil {
				raw = behaviorAdapterRaw(t, request)
			}
			_, buildErr := BuildBehaviorAdapter(raw, tc.previous)
			report := CheckBehaviorAdapter(raw, tc.previous)
			if report.Accepted != (buildErr == nil) {
				t.Fatalf("accepted=%v build error=%v report=%+v", report.Accepted, buildErr, report)
			}
			if buildErr == nil {
				if len(report.Refusals) != 0 {
					t.Fatalf("accepted report lists refusals: %+v", report.Refusals)
				}
			} else {
				var refused *Error
				if !errors.As(buildErr, &refused) {
					t.Fatalf("build error is untyped: %v", buildErr)
				}
				first := report.Refusals[0]
				if first.State != "refused" || first.Code != refused.Code || first.Message != refused.Message {
					t.Fatalf("first refusal %+v, build refused %q", first, refused.Message)
				}
			}
			again := CheckBehaviorAdapter(raw, tc.previous)
			left, leftErr := Encode(report)
			right, rightErr := Encode(again)
			if leftErr != nil || rightErr != nil || !bytes.Equal(left, right) {
				t.Fatalf("check report is not deterministic: %v %v", leftErr, rightErr)
			}
			if bytes.Contains(left, []byte(legacyV1Member(t))) {
				t.Fatalf("check report vocabulary carries the legacy key: %s", left)
			}
		})
	}
}

// TestBehaviorAdapterCheckReportsEveryRefusal proves DCP-V1-044 lists refusals
// from independent stages at once and marks dependent stages not-evaluated.
func TestBehaviorAdapterCheckReportsEveryRefusal(t *testing.T) {
	fixture := behaviorAdapterFixture(t)
	request := fixture.request
	request.Inputs = append([]BehaviorAdapterInput(nil), fixture.request.Inputs...)
	request.Mappings = append([]BehaviorAdapterMapping(nil), fixture.request.Mappings...)
	request.Schema = "unknown"
	behaviorAdapterRequestInput(&request, "flows").Anchor.Repository = strings.Repeat("e", 40)
	behaviorAdapterRequestInput(&request, "candidates").Anchor.Blob = "bad"
	request.Mappings[1].Fields = map[string]string{"id": "/variationKey"}
	raw := behaviorAdapterRaw(t, request)
	if _, err := BuildBehaviorAdapter(raw, nil); err == nil || !strings.Contains(err.Error(), "invalid behavior adapter identity") {
		t.Fatalf("build should stop at the identity refusal: %v", err)
	}
	report := CheckBehaviorAdapter(raw, nil)
	if report.Accepted || report.Fallback != "full-relevant-suite" || len(report.Limitations) == 0 {
		t.Fatalf("report header: %+v", report)
	}
	refused := map[string]int{}
	blocked := map[string][]string{}
	order := []string{}
	for _, refusal := range report.Refusals {
		if refusal.State == "refused" {
			refused[refusal.Stage]++
		} else {
			blocked[refusal.Stage] = refusal.BlockedBy
		}
		if len(order) == 0 || order[len(order)-1] != refusal.Stage {
			order = append(order, refusal.Stage)
		}
	}
	if refused["identity"] != 1 || refused["inputs"] != 2 || refused["mappings"] < 7 {
		t.Fatalf("independent refusals missing: %+v", report.Refusals)
	}
	for stage, want := range map[string]string{"flows": "mappings", "variations": "mappings", "candidates": "mappings", "observations": "identity", "migration": "identity", "declarations": "flows"} {
		if len(blocked[stage]) == 0 || blocked[stage][0] != want {
			t.Fatalf("stage %s blocked_by=%v, want %s first: %+v", stage, blocked[stage], want, report.Refusals)
		}
	}
	if !strings.Contains(strings.Join(func() []string {
		messages := []string{}
		for _, refusal := range report.Refusals {
			if refusal.Stage == "mappings" && refusal.State == "not-evaluated" {
				messages = append(messages, refusal.Message)
			}
		}
		return messages
	}(), "\n"), "mapping flows names refused input flows") {
		t.Fatalf("mapping of a refused input is not reported as not-evaluated: %+v", report.Refusals)
	}
	states := behaviorAdapterCheckStates(report)
	if states["discovery"].State != "passed" || states["tests"].State != "passed" || states["previous"].State != "not-applicable" {
		t.Fatalf("independent stages were not evaluated: %+v", report.Stages)
	}
	stageIndex := map[string]int{}
	for index, stage := range report.Stages {
		stageIndex[stage.Stage] = index
	}
	for index := 1; index < len(order); index++ {
		if stageIndex[order[index-1]] >= stageIndex[order[index]] {
			t.Fatalf("refusals are not in stage order: %v", order)
		}
	}
}
