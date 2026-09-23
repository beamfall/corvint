package doccorpus

import (
	"context"
	jsonstd "encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type behaviorAdapterTestFixture struct {
	root         string
	providerPath string
	manifest     Manifest
	request      BehaviorAdapterRequest
	original     ProviderRecord
}

func behaviorAdapterFixture(t *testing.T) behaviorAdapterTestFixture {
	t.Helper()
	root, manifest := behaviorFixtureWithRun(t, nil, true)
	providerPath := ""
	for _, provider := range manifest.Providers {
		if provider.ID == "behavior" {
			providerPath = provider.Record
		}
	}
	if providerPath == "" {
		t.Fatalf("provider path missing: %+v", manifest.Providers)
	}
	providerRaw, err := os.ReadFile(filepath.Join(root, providerPath))
	if err != nil {
		t.Fatal(err)
	}
	var provider ProviderRecord
	if err := decode(providerRaw, &provider); err != nil {
		t.Fatal(err)
	}
	registry := provider.BehaviorContracts
	flows := []any{}
	variations := []any{}
	for _, flow := range registry.Flows {
		flows = append(flows, map[string]any{
			"flowKey": flow.ID, "sourceKind": flow.Derivation, "anchor": flow.Evidence,
			"pages": flow.RequiredPages, "controls": flow.NegativeControls,
			"sequence": flow.OrderedEvents, "absenceReview": flow.MissingReview,
		})
		for _, criterion := range flow.Criteria {
			projects := []string{}
			outcomes := []BehaviorAdapterOutcome{}
			for _, test := range registry.Tests {
				if !slices.Contains(test.Criteria, criterion) {
					continue
				}
				projects = append(projects, test.Project)
				for _, assertion := range test.Assertions {
					if assertion.Criterion == criterion {
						outcomes = append(outcomes, BehaviorAdapterOutcome{ID: assertion.ID, Behavior: assertion.Behavior, Matcher: assertion.Matcher, Locator: assertion.Locator, Value: assertion.Value})
					}
				}
			}
			variations = append(variations, map[string]any{
				"variationKey": criterion, "flowKey": flow.ID, "testKeys": flow.Tests,
				"conditions": []string{"fixture-ready"}, "actions": []string{"open-summary"},
				"facts": []string{"summary-count-visible"}, "outcomes": outcomes, "projects": projects,
			})
		}
	}
	candidates := []any{}
	for _, candidate := range registry.Behaviors {
		candidates = append(candidates, map[string]any{"candidateKey": candidate.ID, "anchor": candidate.Evidence, "flowKeys": candidate.Flows})
	}
	tests := []any{}
	for _, test := range registry.Tests {
		claims := []BehaviorAdapterTestClaim{}
		for _, criterion := range test.Criteria {
			claims = append(claims, BehaviorAdapterTestClaim{VariationID: criterion, Preconditions: []string{"fixture-ready"}, Actions: []string{"open-summary"}, ObservableFacts: []string{"summary-count-visible"}})
		}
		tests = append(tests, map[string]any{
			"testKey": test.ID, "browserProject": test.Project, "displayTitle": test.Title,
			"anchor": test.Evidence, "flowKeys": test.Flows, "criterionKeys": test.Criteria,
			"checks": test.Assertions, "variationClaims": claims, "witness": test.Runtime, "fixtures": test.Fixtures, "roles": test.Roles,
		})
	}
	documents := map[string]jsonstd.RawMessage{
		"flows":      behaviorAdapterRaw(t, map[string]any{"inventory": map[string]any{"items": flows}}),
		"variations": behaviorAdapterRaw(t, map[string]any{"inventory": map[string]any{"items": variations}}),
		"candidates": behaviorAdapterRaw(t, map[string]any{"inventory": map[string]any{"items": candidates}}),
		"tests":      behaviorAdapterRaw(t, map[string]any{"inventory": map[string]any{"items": tests}}),
	}
	inputs := retainBehaviorAdapterDocuments(t, root, manifest.Repository.ID, documents)
	inputs = append(inputs,
		behaviorAdapterInputFromAnchor(t, root, "migration", registry.Manifest),
		behaviorAdapterInputFromAnchor(t, root, "discovery", registry.Discovery),
		behaviorAdapterInputFromAnchor(t, root, "runtime", registry.Tests[0].Runtime.Evidence),
		behaviorAdapterReceiptInput(t, root, manifest, provider.Observations[0]),
	)
	observations := slices.Clone(provider.Observations)
	for index := range observations {
		observations[index].Subject = provider.ID + ":test:" + observations[index].TestID
	}
	request := BehaviorAdapterRequest{
		Schema: BehaviorAdapterRequestSchema, ProviderID: provider.ID, ProviderVersion: provider.Version,
		ContractID: registry.ContractID, Source: provider.Source, Revisions: registry.Revisions,
		SourceRevision: registry.SourceRevision, DocumentationRevision: registry.DocumentationRevision,
		MigrationInput: "migration", DiscoveryInput: "discovery", Inputs: inputs, Observations: observations,
		Mappings: []BehaviorAdapterMapping{
			{Kind: "flows", Input: "flows", Records: "/inventory/items", Fields: map[string]string{"id": "/flowKey", "derivation": "/sourceKind", "evidence": "/anchor", "required_pages": "/pages", "negative_controls": "/controls", "ordered_events": "/sequence", "missing_e2e_review": "/absenceReview"}},
			{Kind: "variations", Input: "variations", Records: "/inventory/items", Fields: map[string]string{"id": "/variationKey", "flow": "/flowKey", "preconditions": "/conditions", "actions": "/actions", "observable_facts": "/facts", "expected_outcomes": "/outcomes", "projects": "/projects", "tests": "/testKeys"}},
			{Kind: "candidates", Input: "candidates", Records: "/inventory/items", Fields: map[string]string{"id": "/candidateKey", "evidence": "/anchor", "flows": "/flowKeys"}},
			{Kind: "tests", Input: "tests", Records: "/inventory/items", Fields: map[string]string{"id": "/testKey", "project": "/browserProject", "title": "/displayTitle", "evidence": "/anchor", "flows": "/flowKeys", "criteria": "/criterionKeys", "assertions": "/checks", "variation_claims": "/variationClaims", "runtime": "/witness", "fixtures": "/fixtures", "roles": "/roles"}},
		},
	}
	return behaviorAdapterTestFixture{root: root, providerPath: providerPath, manifest: manifest, request: request, original: provider}
}

func behaviorAdapterRaw(t *testing.T, value any) jsonstd.RawMessage {
	t.Helper()
	raw, err := Encode(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func retainBehaviorAdapterDocuments(t *testing.T, root, repository string, documents map[string]jsonstd.RawMessage) []BehaviorAdapterInput {
	t.Helper()
	ids := []string{"flows", "variations", "candidates", "tests"}
	for _, id := range ids {
		path := "evidence/adapter-" + id + ".json"
		if err := os.WriteFile(filepath.Join(root, path), documents[id], 0600); err != nil {
			t.Fatal(err)
		}
		git(t, root, "add", path)
	}
	git(t, root, "commit", "-qm", "retain behavior adapter inputs")
	revision := git(t, root, "rev-parse", "HEAD")
	inputs := []BehaviorAdapterInput{}
	for _, id := range ids {
		path := "evidence/adapter-" + id + ".json"
		digest := Digest(documents[id])
		kind := "declared"
		if id == "flows" || id == "variations" {
			kind = "review"
		}
		inputs = append(inputs, BehaviorAdapterInput{ID: id, Anchor: Anchor{Repository: repository, Revision: revision, Path: path, Blob: git(t, root, "rev-parse", revision+":"+path), SHA256: digest, Start: 1, End: 1, SpanSHA256: digest, Authority: "external-provider", Kind: kind, Reason: "synthetic mapped input"}, Document: string(documents[id])})
	}
	return inputs
}

func behaviorAdapterInputFromAnchor(t *testing.T, root, id string, anchor Anchor) BehaviorAdapterInput {
	t.Helper()
	if anchor.Path == "" {
		t.Fatalf("%s anchor path missing: %+v", id, anchor)
	}
	raw, err := os.ReadFile(filepath.Join(root, anchor.Path))
	if err != nil {
		t.Fatal(err)
	}
	return BehaviorAdapterInput{ID: id, Anchor: anchor, Document: string(raw)}
}

func behaviorAdapterReceiptInput(t *testing.T, root string, manifest Manifest, link ObservationLink) BehaviorAdapterInput {
	t.Helper()
	for _, input := range manifest.Inputs {
		if input.Path != link.Input || input.Revision != link.InputRevision {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, input.Path))
		if err != nil {
			t.Fatal(err)
		}
		anchor := Anchor{Repository: manifest.Repository.ID, Revision: input.Revision, Path: input.Path, Blob: input.Blob, SHA256: input.SHA256, Start: 1, End: 1, SpanSHA256: input.SHA256, Authority: "external-provider", Kind: "observed", Reason: "synthetic qualified receipt"}
		return BehaviorAdapterInput{ID: "receipt", Anchor: anchor, Document: string(raw)}
	}
	t.Fatal("receipt input missing")
	return BehaviorAdapterInput{}
}

func buildBehaviorAdapter(t *testing.T, request BehaviorAdapterRequest, previous []byte) BehaviorAdapterResult {
	t.Helper()
	raw, err := Encode(request)
	if err != nil {
		t.Fatal(err)
	}
	result, err := BuildBehaviorAdapter(raw, previous)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestBehaviorAdapterBuildOpen(t *testing.T) {
	fixture := behaviorAdapterFixture(t)
	result := buildBehaviorAdapter(t, fixture.request, nil)
	if result.Fallback != "full-relevant-suite" || len(result.Coverage) != 5 || result.Provider.BehaviorContracts.ContractSHA256 != fixture.original.BehaviorContracts.ContractSHA256 {
		t.Fatalf("invalid adapter result: %+v", result)
	}
	if result.Coverage[0].Denominator != 1 || result.Coverage[1].Denominator != 1 || result.Coverage[4].Value != 1 {
		t.Fatalf("separate denominators or runtime witness lost: coverage=%+v frontier=%+v", result.Coverage, result.Frontier)
	}
	firstBytes, _ := Encode(result)
	secondBytes, _ := Encode(buildBehaviorAdapter(t, fixture.request, nil))
	if string(firstBytes) != string(secondBytes) {
		t.Fatal("identical request produced nondeterministic result")
	}
	providerRaw, err := Encode(result.Provider)
	if err != nil {
		t.Fatal(err)
	}
	path := fixture.providerPath
	if err := os.WriteFile(filepath.Join(fixture.root, path), providerRaw, 0600); err != nil {
		t.Fatal(err)
	}
	git(t, fixture.root, "add", path)
	git(t, fixture.root, "commit", "-qm", "emit mapped behavior provider")
	revision := git(t, fixture.root, "rev-parse", "HEAD")
	for index := range fixture.manifest.Providers {
		if fixture.manifest.Providers[index].Record == path {
			fixture.manifest.Providers[index].Revision = revision
		}
	}
	for index := range fixture.manifest.Scopes {
		if fixture.manifest.Scopes[index].Path == path {
			fixture.manifest.Scopes[index].Revision = revision
		}
	}
	for index := range fixture.manifest.Inputs {
		if fixture.manifest.Inputs[index].Path == path && fixture.manifest.Inputs[index].Purpose == "provider" {
			fixture.manifest.Inputs[index].Revision = revision
			fixture.manifest.Inputs[index].Blob = git(t, fixture.root, "rev-parse", revision+":"+path)
			fixture.manifest.Inputs[index].SHA256 = Digest(providerRaw)
		}
	}
	artifact, err := Build(context.Background(), fixture.root, fixture.manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifact.BehaviorContracts) != 1 || len(artifact.BehaviorContracts[0].VerifiedFlows) != 1 || artifact.BehaviorContracts[0].Fallback != "full-relevant-suite" {
		t.Fatalf("mapped provider did not retain issue-40 behavior: %+v", artifact.BehaviorContracts)
	}
	encoded, err := Encode(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(context.Background(), fixture.root, encoded); err != nil {
		t.Fatal(err)
	}
}

func TestBehaviorAdapterMappingParity(t *testing.T) {
	fixture := behaviorAdapterFixture(t)
	first := buildBehaviorAdapter(t, fixture.request, nil)
	secondRequest := fixture.request
	secondRequest.Mappings = slices.Clone(fixture.request.Mappings)
	for index := range secondRequest.Mappings {
		mapping := &secondRequest.Mappings[index]
		mapping.Records = "/alternate"
		input := behaviorAdapterRequestInput(&secondRequest, mapping.Input)
		var original any
		if err := jsonstd.Unmarshal([]byte(input.Document), &original); err != nil {
			t.Fatal(err)
		}
		records, ok := resolveJSONPointer(original, "/inventory/items")
		if !ok {
			t.Fatal("records missing")
		}
		input.Document = string(behaviorAdapterRaw(t, map[string]any{"alternate": records}))
		input.Anchor.SHA256 = Digest([]byte(input.Document))
		input.Anchor.SpanSHA256 = input.Anchor.SHA256
	}
	second := buildBehaviorAdapter(t, secondRequest, nil)
	left, _ := Encode(first.Provider)
	right, _ := Encode(second.Provider)
	if string(left) != string(right) {
		t.Fatal("outer vocabulary changed provider bytes")
	}
	left, _ = Encode(first.Variations)
	right, _ = Encode(second.Variations)
	if string(left) != string(right) {
		t.Fatal("outer vocabulary changed normative variation bytes")
	}
}

func TestBehaviorAdapterConformance(t *testing.T) {
	for _, tc := range []struct {
		name, kind string
		edit       func(*testing.T, *BehaviorAdapterRequest)
	}{
		{"wrong project", "unregistered-test-execution", func(t *testing.T, request *BehaviorAdapterRequest) {
			behaviorAdapterEditRow(t, request, "tests", func(row map[string]any) { row["browserProject"] = "wrong-project" })
		}},
		{"route only", "assertion-free-ui-execution", func(t *testing.T, request *BehaviorAdapterRequest) {
			behaviorAdapterEditRow(t, request, "tests", func(row map[string]any) { row["checks"] = []any{} })
		}},
		{"missing reverse", "missing-reverse-link", func(t *testing.T, request *BehaviorAdapterRequest) {
			behaviorAdapterEditRow(t, request, "variations", func(row map[string]any) { row["testKeys"] = []any{} })
		}},
		{"unreviewed assertion candidate", "assertion-contradiction", func(t *testing.T, request *BehaviorAdapterRequest) {
			behaviorAdapterEditRow(t, request, "tests", func(row map[string]any) {
				assertions := row["checks"].([]any)
				assertions[0].(map[string]any)["behavior"] = "unreviewed-candidate"
			})
		}},
		{"semantic fact mismatch", "semantic-mismatch", func(t *testing.T, request *BehaviorAdapterRequest) {
			behaviorAdapterEditRow(t, request, "tests", func(row map[string]any) {
				claims := row["variationClaims"].([]any)
				claims[0].(map[string]any)["observable_facts"] = []any{"different-fact"}
			})
		}},
		{"undocumented tested behavior", "undocumented-tested-behavior", func(t *testing.T, request *BehaviorAdapterRequest) {
			behaviorAdapterEditRow(t, request, "tests", func(row map[string]any) {
				row["criterionKeys"] = []any{"proposed-only"}
				claims := row["variationClaims"].([]any)
				claims[0].(map[string]any)["variation_id"] = "proposed-only"
			})
		}},
		{"stale documentation", "stale-anchor", func(t *testing.T, request *BehaviorAdapterRequest) {
			behaviorAdapterEditRow(t, request, "flows", func(row map[string]any) {
				anchor := row["anchor"].(map[string]any)
				anchor["revision"] = strings.Repeat("a", 40)
			})
		}},
		{"unregistered discovery", "unregistered-discovered-execution", func(t *testing.T, request *BehaviorAdapterRequest) {}},
		{"empty source candidate", "source-candidate-orphan", func(t *testing.T, request *BehaviorAdapterRequest) {
			behaviorAdapterEditRow(t, request, "candidates", func(row map[string]any) { row["flowKeys"] = []any{} })
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := behaviorAdapterFixture(t)
			tc.edit(t, &fixture.request)
			result := buildBehaviorAdapter(t, fixture.request, nil)
			if !slices.ContainsFunc(result.Frontier, func(d BehaviorAdapterDiagnostic) bool { return d.Kind == tc.kind }) {
				t.Fatalf("missing %s: %+v", tc.kind, result.Frontier)
			}
			if result.Fallback != "full-relevant-suite" {
				t.Fatal("fallback relaxed")
			}
			for _, diagnostic := range result.Frontier {
				if diagnostic.Input == "" || diagnostic.Field == "" || diagnostic.Revision == "" || diagnostic.Digest == "" || diagnostic.Correction == "" {
					t.Fatalf("diagnostic lacks exact correction context: %+v", diagnostic)
				}
			}
		})
	}
	t.Run("wrong locator value", func(t *testing.T) {
		fixture := behaviorAdapterFixture(t)
		behaviorAdapterEditRow(t, &fixture.request, "tests", func(row map[string]any) {
			assertions := row["checks"].([]any)
			assertion := assertions[0].(map[string]any)
			assertion["locator"] = "#other"
			assertion["value"] = "different"
		})
		first := buildBehaviorAdapter(t, fixture.request, nil)
		behaviorAdapterUpdateRun(t, &fixture.request, func(run *BehaviorRun) { run.ContractSHA256 = first.Provider.BehaviorContracts.ContractSHA256 })
		result := buildBehaviorAdapter(t, fixture.request, nil)
		if !slices.ContainsFunc(result.Frontier, func(d BehaviorAdapterDiagnostic) bool { return d.Kind == "assertion-contradiction" }) {
			t.Fatalf("wrong locator/value not detected: %+v", result.Frontier)
		}
	})
	t.Run("out of order page events", func(t *testing.T) {
		fixture := behaviorAdapterFixture(t)
		behaviorAdapterUpdateRun(t, &fixture.request, func(run *BehaviorRun) {
			run.Events[0], run.Events[1] = run.Events[1], run.Events[0]
			run.Events[0].Sequence, run.Events[1].Sequence = 1, 2
		})
		result := buildBehaviorAdapter(t, fixture.request, nil)
		if !slices.ContainsFunc(result.Frontier, func(d BehaviorAdapterDiagnostic) bool { return d.Kind == "out-of-order-events" }) {
			t.Fatalf("event order not detected: %+v", result.Frontier)
		}
	})
	for _, tc := range []struct {
		name, eventKind, want string
	}{
		{"missing page", "page", "missing-page-event"},
		{"missing assertion", "assertion", "missing-assertion-event"},
		{"missing negative control", "negative-control", "missing-negative-control"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := behaviorAdapterFixture(t)
			behaviorAdapterUpdateRun(t, &fixture.request, func(run *BehaviorRun) {
				filtered := run.Events[:0]
				removed := false
				for _, event := range run.Events {
					if event.Kind == tc.eventKind && !removed {
						removed = true
						continue
					}
					filtered = append(filtered, event)
				}
				run.Events = filtered
				for index := range run.Events {
					run.Events[index].Sequence = index + 1
				}
			})
			result := buildBehaviorAdapter(t, fixture.request, nil)
			if !slices.ContainsFunc(result.Frontier, func(d BehaviorAdapterDiagnostic) bool { return d.Kind == tc.want }) {
				t.Fatalf("missing %s: %+v", tc.want, result.Frontier)
			}
		})
	}
	t.Run("equally invalid declared and runtime event", func(t *testing.T) {
		fixture := behaviorAdapterFixture(t)
		behaviorAdapterEditRow(t, &fixture.request, "flows", func(row map[string]any) {
			events := row["sequence"].([]any)
			events[0].(map[string]any)["passed"] = false
		})
		first := buildBehaviorAdapter(t, fixture.request, nil)
		behaviorAdapterUpdateRun(t, &fixture.request, func(run *BehaviorRun) {
			run.ContractSHA256 = first.Provider.BehaviorContracts.ContractSHA256
			run.Events[0].Passed = false
		})
		result := buildBehaviorAdapter(t, fixture.request, nil)
		testsInput := behaviorAdapterRequestInput(&fixture.request, "tests")
		if !slices.ContainsFunc(result.Frontier, func(d BehaviorAdapterDiagnostic) bool {
			return d.Kind == "invalid-runtime-event" && d.Input == "tests" && d.Field == "/inventory/items/0/witness" && d.Revision == testsInput.Anchor.Revision && d.Digest == testsInput.Anchor.SHA256 && d.Correction == "retain one unique passing closed event at its exact sequence"
		}) {
			t.Fatalf("invalid matching event lacked frontier: %+v", result.Frontier)
		}
	})
}

// TestBehaviorAdapterPreviousToleratesDanglingCriterion is the V1-0044
// end-to-end regression: run 1 emits a real "undocumented-tested-behavior"
// finding for a criterion the test declares but the normative variation set
// does not (behavior_adapter.go:1195), exactly as forward emission
// (reconcileTest) retains rather than prunes it; run 1 must still exit 0
// (buildBehaviorAdapter fails the test on error). Run 2 then feeds that
// same result back as --previous and must also succeed, which is the DCP-V1-032
// delta case that was unusable before this fix.
func TestBehaviorAdapterPreviousToleratesDanglingCriterion(t *testing.T) {
	fixture := behaviorAdapterFixture(t)
	behaviorAdapterEditRow(t, &fixture.request, "tests", func(row map[string]any) {
		// Append rather than replace: the test keeps its documented criteria
		// (and each variation's own "tests" back-reference stays valid), and
		// gains one additional criterion the normative variation set does not
		// declare, plus the matching claim reconcileTest requires to accept it.
		row["criterionKeys"] = append(row["criterionKeys"].([]any), "proposed-only")
		claims := row["variationClaims"].([]any)
		row["variationClaims"] = append(claims, map[string]any{
			"variation_id": "proposed-only", "preconditions": []any{"fixture-ready"},
			"actions": []any{"open-summary"}, "observable_facts": []any{"summary-count-visible"},
		})
	})
	run1 := buildBehaviorAdapter(t, fixture.request, nil)
	if run1.Fallback != "full-relevant-suite" {
		t.Fatalf("run1 fallback relaxed: %+v", run1)
	}
	if !slices.ContainsFunc(run1.Frontier, func(d BehaviorAdapterDiagnostic) bool { return d.Kind == "undocumented-tested-behavior" }) {
		t.Fatalf("run1 missing the dangling-criterion finding: %+v", run1.Frontier)
	}
	run1Raw, err := Encode(run1)
	if err != nil {
		t.Fatal(err)
	}
	run2 := buildBehaviorAdapter(t, fixture.request, run1Raw)
	if !run2.Delta.PreviousAvailable {
		t.Fatalf("run2 did not accept run1 as --previous: %+v", run2.Delta)
	}
}

func TestBehaviorAdapterReviewedAbsenceAndBounds(t *testing.T) {
	fixture := behaviorAdapterFixture(t)
	behaviorAdapterEditRow(t, &fixture.request, "variations", func(row map[string]any) { row["testKeys"] = []any{} })
	behaviorAdapterEditRow(t, &fixture.request, "tests", func(row map[string]any) {
		row["flowKeys"] = []any{}
		row["criterionKeys"] = []any{}
	})
	unreviewed := buildBehaviorAdapter(t, fixture.request, nil)
	if !slices.ContainsFunc(unreviewed.Frontier, func(d BehaviorAdapterDiagnostic) bool { return d.Kind == "unreviewed" }) || slices.ContainsFunc(unreviewed.Frontier, func(d BehaviorAdapterDiagnostic) bool { return d.Kind == "confirmed-missing_e2e" }) {
		t.Fatalf("missing joins were promoted without review: %+v", unreviewed.Frontier)
	}
	behaviorAdapterEditRow(t, &fixture.request, "flows", func(row map[string]any) {
		anchor := row["anchor"].(map[string]any)
		anchor["evidence_kind"] = "review"
		row["absenceReview"] = anchor
	})
	reviewed := buildBehaviorAdapter(t, fixture.request, nil)
	if !slices.ContainsFunc(reviewed.Frontier, func(d BehaviorAdapterDiagnostic) bool { return d.Kind == "confirmed-missing_e2e" }) {
		t.Fatalf("explicit reviewed absence not retained: %+v", reviewed.Frontier)
	}

	broken := fixture.request
	broken.Mappings = slices.Clone(fixture.request.Mappings)
	broken.Mappings[0].Records = "/bad~pointer"
	raw, _ := Encode(broken)
	if _, err := BuildBehaviorAdapter(raw, nil); err == nil {
		t.Fatal("malformed mapping pointer accepted")
	}
	broken = fixture.request
	broken.Inputs = slices.Clone(fixture.request.Inputs)
	broken.Inputs[0].Anchor.SHA256 = strings.Repeat("f", 64)
	raw, _ = Encode(broken)
	if _, err := BuildBehaviorAdapter(raw, nil); err == nil || !strings.Contains(err.Error(), "input=") || !strings.Contains(err.Error(), "field=anchor") {
		t.Fatalf("digest diagnostic missing exact input/field: %v", err)
	}
	fresh := behaviorAdapterFixture(t)
	broken = fresh.request
	behaviorAdapterEditRow(t, &broken, "variations", func(row map[string]any) { delete(row, "actions") })
	variationInput := behaviorAdapterRequestInput(&broken, "variations")
	raw, _ = Encode(broken)
	if _, err := BuildBehaviorAdapter(raw, nil); err == nil || !strings.Contains(err.Error(), "input=variations") || !strings.Contains(err.Error(), "field=/inventory/items/0/actions") || !strings.Contains(err.Error(), "revision="+variationInput.Anchor.Revision) || !strings.Contains(err.Error(), "digest="+variationInput.Anchor.SHA256) || !strings.Contains(err.Error(), "map a present field with the required closed shape") {
		t.Fatalf("mapped-field diagnostic lacks exact context: %v", err)
	}
	fresh = behaviorAdapterFixture(t)
	broken = fresh.request
	behaviorAdapterEditRow(t, &broken, "flows", func(row map[string]any) { row["sourceKind"] = "invalid" })
	flowInput := behaviorAdapterRequestInput(&broken, "flows")
	raw, _ = Encode(broken)
	if _, err := BuildBehaviorAdapter(raw, nil); err == nil || !strings.Contains(err.Error(), "input=flows") || !strings.Contains(err.Error(), "field=/inventory/items/0/sourceKind") || !strings.Contains(err.Error(), "revision="+flowInput.Anchor.Revision) || !strings.Contains(err.Error(), "digest="+flowInput.Anchor.SHA256) || !strings.Contains(err.Error(), "supply one supported derivation") {
		t.Fatalf("declaration diagnostic lacks exact context: %v", err)
	}
	fresh = behaviorAdapterFixture(t)
	broken = fresh.request
	behaviorAdapterEditRow(t, &broken, "tests", func(row map[string]any) { row["displayTitle"] = "" })
	raw, _ = Encode(broken)
	if _, err := BuildBehaviorAdapter(raw, nil); err == nil || !strings.Contains(err.Error(), "field=/inventory/items/0/displayTitle") {
		t.Fatalf("test-title diagnostic named the wrong field: %v", err)
	}
	fresh = behaviorAdapterFixture(t)
	broken = fresh.request
	behaviorAdapterEditRow(t, &broken, "flows", func(row map[string]any) { row["pages"] = []any{"/summary", "/summary"} })
	raw, _ = Encode(broken)
	if _, err := BuildBehaviorAdapter(raw, nil); err == nil || !strings.Contains(err.Error(), "field=/inventory/items/0/pages") {
		t.Fatalf("required-page diagnostic named the wrong field: %v", err)
	}
	fresh = behaviorAdapterFixture(t)
	broken = fresh.request
	behaviorAdapterEditRow(t, &broken, "variations", func(row map[string]any) {
		delete(row, "actions")
		delete(row, "facts")
	})
	raw, _ = Encode(broken)
	var firstError string
	for attempt := 0; attempt < 20; attempt++ {
		_, err := BuildBehaviorAdapter(raw, nil)
		if err == nil {
			t.Fatal("multiply incomplete variation accepted")
		}
		if attempt == 0 {
			firstError = err.Error()
		}
		if err.Error() != firstError || !strings.Contains(err.Error(), "field=/inventory/items/0/actions") {
			t.Fatalf("rejection diagnostic is nondeterministic: first=%q current=%q", firstError, err)
		}
	}
	fresh = behaviorAdapterFixture(t)
	broken = fresh.request
	for index := range broken.Mappings {
		if broken.Mappings[index].Kind == "variations" {
			broken.Mappings[index].Records = "/inventory/"
		}
	}
	trailingInput := behaviorAdapterRequestInput(&broken, "variations")
	var trailingDocument map[string]any
	if err := jsonstd.Unmarshal([]byte(trailingInput.Document), &trailingDocument); err != nil {
		t.Fatal(err)
	}
	records := trailingDocument["inventory"].(map[string]any)["items"].([]any)
	delete(records[0].(map[string]any), "actions")
	trailingDocument["inventory"] = map[string]any{"": records}
	trailingInput.Document = string(behaviorAdapterRaw(t, trailingDocument))
	trailingInput.Anchor.SHA256 = Digest([]byte(trailingInput.Document))
	trailingInput.Anchor.SpanSHA256 = trailingInput.Anchor.SHA256
	raw, _ = Encode(broken)
	if _, err := BuildBehaviorAdapter(raw, nil); err == nil || !strings.Contains(err.Error(), "field=/inventory//0/actions") {
		t.Fatalf("trailing-slash JSON pointer lost exact identity: %v", err)
	}
}

func TestBehaviorAdapterUnreviewedNormativeInput(t *testing.T) {
	fixture := behaviorAdapterFixture(t)
	behaviorAdapterRequestInput(&fixture.request, "variations").Anchor.Kind = "declared"
	result := buildBehaviorAdapter(t, fixture.request, nil)
	if !slices.ContainsFunc(result.Frontier, func(d BehaviorAdapterDiagnostic) bool { return d.Kind == "unreviewed-normative-input" }) || result.Coverage[3].Value != 0 || result.Coverage[4].Value != 0 {
		t.Fatalf("unreviewed normative input qualified coverage: coverage=%+v frontier=%+v", result.Coverage, result.Frontier)
	}
}

func TestBehaviorAdapterObservationAndPriorIntegrity(t *testing.T) {
	fixture := behaviorAdapterFixture(t)
	for _, subject := range []string{"", "behavior:test:wrong"} {
		request := fixture.request
		request.Observations = slices.Clone(fixture.request.Observations)
		request.Observations[0].Subject = subject
		raw, _ := Encode(request)
		if _, err := BuildBehaviorAdapter(raw, nil); err == nil {
			t.Fatalf("observation subject %q was silently repaired", subject)
		}
	}

	previous := buildBehaviorAdapter(t, fixture.request, nil)
	for _, tc := range []struct {
		name string
		edit func(*BehaviorAdapterResult)
	}{
		{"truncated", func(result *BehaviorAdapterResult) { result.Provider = ProviderRecord{} }},
		{"duplicate variation", func(result *BehaviorAdapterResult) {
			result.Variations = append(result.Variations, result.Variations[0])
		}},
		{"missing claim", func(result *BehaviorAdapterResult) { result.Claims = []BehaviorAdapterClaimRecord{} }},
		{"orphan claim test", func(result *BehaviorAdapterResult) {
			orphan := result.Claims[0]
			orphan.TestID = "unknown-test"
			result.Claims = append(result.Claims, orphan)
		}},
		{"orphan claim variation", func(result *BehaviorAdapterResult) {
			orphan := result.Claims[0]
			orphan.Claim.VariationID = "unknown-variation"
			result.Claims = append(result.Claims, orphan)
		}},
		{"schema", func(result *BehaviorAdapterResult) { result.Schema = "corvint-behavior-adapter-result/0" }},
		{"contract digest", func(result *BehaviorAdapterResult) {
			result.Provider.BehaviorContracts.ContractSHA256 = strings.Repeat("f", 64)
		}},
		{"artifact digest", func(result *BehaviorAdapterResult) {
			result.Artifacts[0].SHA256 = strings.Repeat("f", 64)
		}},
		{"cross contract", func(result *BehaviorAdapterResult) { result.Provider.BehaviorContracts.ContractID = "other-contract" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := Encode(previous)
			var tampered BehaviorAdapterResult
			if err := decode(raw, &tampered); err != nil {
				t.Fatal(err)
			}
			tc.edit(&tampered)
			previousRaw, _ := Encode(tampered)
			requestRaw, _ := Encode(fixture.request)
			if _, err := BuildBehaviorAdapter(requestRaw, previousRaw); err == nil {
				t.Fatal("invalid previous result accepted")
			}
		})
	}
}

func TestBehaviorAdapterDelta(t *testing.T) {
	fixture := behaviorAdapterFixture(t)
	behaviorAdapterEditRow(t, &fixture.request, "variations", func(row map[string]any) {
		row["variationKey"] = "first"
	})
	behaviorAdapterEditRow(t, &fixture.request, "tests", func(row map[string]any) {
		row["criterionKeys"] = []any{"first"}
		claims := row["variationClaims"].([]any)
		claims[0].(map[string]any)["variation_id"] = "first"
		assertions := row["checks"].([]any)
		assertions[0].(map[string]any)["criterion"] = "first"
	})
	previous := buildBehaviorAdapter(t, fixture.request, nil)
	if err := validatePreviousBehaviorAdapterResult(fixture.request, previous); err != nil {
		t.Fatal(err)
	}
	previousRaw, _ := Encode(previous)
	behaviorAdapterEditRow(t, &fixture.request, "variations", func(row map[string]any) { row["variationKey"] = "second" })
	behaviorAdapterEditRow(t, &fixture.request, "tests", func(row map[string]any) {
		row["criterionKeys"] = []any{"second"}
		claims := row["variationClaims"].([]any)
		claims[0].(map[string]any)["variation_id"] = "second"
		assertions := row["checks"].([]any)
		assertions[0].(map[string]any)["criterion"] = "second"
	})
	current := buildBehaviorAdapter(t, fixture.request, previousRaw)
	if len(current.Delta.AddedCriteria) != 1 || len(current.Delta.RemovedCriteria) != 1 || current.Coverage[0].Denominator != previous.Coverage[0].Denominator {
		t.Fatalf("unchanged total hid identity delta: %+v", current.Delta)
	}
	assertIndependentReverseLinkDeltas(t, previous)
	semanticFixture := behaviorAdapterFixture(t)
	semanticPrevious := buildBehaviorAdapter(t, semanticFixture.request, nil)
	semanticPreviousRaw, _ := Encode(semanticPrevious)
	behaviorAdapterEditRow(t, &semanticFixture.request, "variations", func(row map[string]any) { row["actions"] = []any{"open-summary-differently"} })
	semanticCurrent := buildBehaviorAdapter(t, semanticFixture.request, semanticPreviousRaw)
	if !slices.Equal(semanticCurrent.Delta.ChangedCriteria, []string{"count-visible"}) || semanticCurrent.Coverage[0].Denominator != semanticPrevious.Coverage[0].Denominator {
		t.Fatalf("unchanged total hid semantic delta: %+v", semanticCurrent.Delta)
	}
	linkFixture := behaviorAdapterFixture(t)
	linkPrevious := buildBehaviorAdapter(t, linkFixture.request, nil)
	linkPreviousRaw, _ := Encode(linkPrevious)
	behaviorAdapterEditRow(t, &linkFixture.request, "tests", func(row map[string]any) {
		row["variationClaims"] = []any{}
		row["checks"] = []any{}
	})
	linkCurrent := buildBehaviorAdapter(t, linkFixture.request, linkPreviousRaw)
	claimLost := slices.ContainsFunc(linkCurrent.Delta.LostReverseLinks, func(link string) bool { return strings.HasPrefix(link, "claim:") })
	assertionLost := slices.ContainsFunc(linkCurrent.Delta.LostReverseLinks, func(link string) bool { return strings.HasPrefix(link, "assertion:") })
	if !claimLost || !assertionLost || linkCurrent.Coverage[0].Denominator != linkPrevious.Coverage[0].Denominator {
		t.Fatalf("same-count claim/assertion deletion was hidden: %+v", linkCurrent.Delta)
	}
	crossRevisionFixture := behaviorAdapterFixture(t)
	crossRevisionPrevious := buildBehaviorAdapter(t, crossRevisionFixture.request, nil)
	crossRevisionPreviousRaw, _ := Encode(crossRevisionPrevious)
	behaviorAdapterAdvanceRevision(t, &crossRevisionFixture.request, strings.Repeat("a", 40))
	behaviorAdapterEditRow(t, &crossRevisionFixture.request, "variations", func(row map[string]any) { row["actions"] = []any{"open-summary-after-revision"} })
	crossRevisionCurrent := buildBehaviorAdapter(t, crossRevisionFixture.request, crossRevisionPreviousRaw)
	if !slices.Equal(crossRevisionCurrent.Delta.ChangedCriteria, []string{"count-visible"}) {
		t.Fatalf("cross-revision semantic delta refused or hidden: %+v", crossRevisionCurrent.Delta)
	}
}

func assertIndependentReverseLinkDeltas(t *testing.T, previous BehaviorAdapterResult) {
	t.Helper()
	variation := previous.Variations[0]
	testID := variation.Tests[0]
	links := []string{
		"variation-flow:" + reverseLinkJoin(variation.ID, variation.Flow),
		"variation-test:" + reverseLinkJoin(variation.ID, testID),
		"test-variation:" + reverseLinkJoin(testID, variation.ID),
	}
	for index, link := range links {
		currentRaw, _ := Encode(previous)
		var current BehaviorAdapterResult
		if err := decode(currentRaw, &current); err != nil {
			t.Fatal(err)
		}
		switch index {
		case 0:
			current.Variations[0].Flow = "replacement-flow"
		case 1:
			current.Variations[0].Tests = []string{}
		case 2:
			current.Provider.BehaviorContracts.Tests[0].Criteria = []string{}
		}
		delta, err := behaviorAdapterDelta(&previous, current)
		if err != nil {
			t.Fatal(err)
		}
		lost := stringSet(delta.LostReverseLinks)
		if !lost[link] {
			t.Fatalf("reverse-link loss %q was hidden: %+v", link, delta)
		}
		for otherIndex, other := range links {
			if otherIndex != index && lost[other] {
				t.Fatalf("reverse-link loss %q also removed independent link %q: %+v", link, other, delta)
			}
		}
	}
}

func stringSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}

func behaviorAdapterRequestInput(request *BehaviorAdapterRequest, id string) *BehaviorAdapterInput {
	for index := range request.Inputs {
		if request.Inputs[index].ID == id {
			return &request.Inputs[index]
		}
	}
	return nil
}

func behaviorAdapterEditRow(t *testing.T, request *BehaviorAdapterRequest, inputID string, edit func(map[string]any)) {
	t.Helper()
	input := behaviorAdapterRequestInput(request, inputID)
	var document map[string]any
	if err := jsonstd.Unmarshal([]byte(input.Document), &document); err != nil {
		t.Fatal(err)
	}
	records := document["inventory"].(map[string]any)["items"].([]any)
	edit(records[0].(map[string]any))
	input.Document = string(behaviorAdapterRaw(t, document))
	input.Anchor.SHA256 = Digest([]byte(input.Document))
	input.Anchor.SpanSHA256 = input.Anchor.SHA256
}

func behaviorAdapterUpdateRun(t *testing.T, request *BehaviorAdapterRequest, edit func(*BehaviorRun)) {
	t.Helper()
	runtimeInput := behaviorAdapterRequestInput(request, "runtime")
	var run BehaviorRun
	if err := decode([]byte(runtimeInput.Document), &run); err != nil {
		t.Fatal(err)
	}
	edit(&run)
	runtimeInput.Document = string(behaviorAdapterRaw(t, run))
	runtimeInput.Anchor.SHA256 = Digest([]byte(runtimeInput.Document))
	runtimeInput.Anchor.SpanSHA256 = runtimeInput.Anchor.SHA256
	behaviorAdapterEditRow(t, request, "tests", func(row map[string]any) {
		witness := row["witness"].(map[string]any)
		evidence := witness["evidence"].(map[string]any)
		evidence["sha256"] = runtimeInput.Anchor.SHA256
		evidence["span_sha256"] = runtimeInput.Anchor.SpanSHA256
	})
}

func behaviorAdapterAdvanceRevision(t *testing.T, request *BehaviorAdapterRequest, revision string) {
	t.Helper()
	old := request.SourceRevision
	request.Source.Revision = revision
	request.SourceRevision = revision
	request.DocumentationRevision = revision
	request.Revisions.App.Revision = revision
	request.Revisions.E2E.Revision = revision
	request.Revisions.Docs.Revision = revision
	for index := range request.Inputs {
		input := &request.Inputs[index]
		var document any
		if err := jsonstd.Unmarshal([]byte(input.Document), &document); err != nil {
			t.Fatal(err)
		}
		behaviorAdapterReplaceRevision(document, old, revision)
		input.Document = string(behaviorAdapterRaw(t, document))
		input.Anchor.Revision = revision
		input.Anchor.SHA256 = Digest([]byte(input.Document))
		input.Anchor.SpanSHA256 = input.Anchor.SHA256
	}
	receipt := behaviorAdapterRequestInput(request, "receipt")
	for index := range request.Observations {
		request.Observations[index].InputRevision = revision
		request.Observations[index].SourceRevision = revision
		request.Observations[index].RunID = receipt.Anchor.SHA256
	}
	var migration BehaviorMigration
	if err := decode([]byte(behaviorAdapterRequestInput(request, request.MigrationInput).Document), &migration); err != nil || migration.Schema != 2 || migration.ContractID != request.ContractID || migration.SourceRevision != request.SourceRevision || migration.DocumentationRevision != request.DocumentationRevision || migration.Revisions != request.Revisions {
		t.Fatalf("revision rewrite left migration inconsistent: migration=%+v request=%+v err=%v", migration, request.Revisions, err)
	}
	behaviorAdapterUpdateRun(t, request, func(run *BehaviorRun) { run.RunSHA256 = receipt.Anchor.SHA256 })
	provisional := buildBehaviorAdapter(t, *request, nil)
	behaviorAdapterUpdateRun(t, request, func(run *BehaviorRun) { run.ContractSHA256 = provisional.Provider.BehaviorContracts.ContractSHA256 })
}

func behaviorAdapterReplaceRevision(value any, old, revision string) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if text, ok := child.(string); ok && text == old && (key == "revision" || strings.HasSuffix(key, "_revision")) {
				typed[key] = revision
				continue
			}
			behaviorAdapterReplaceRevision(child, old, revision)
		}
	case []any:
		for _, child := range typed {
			behaviorAdapterReplaceRevision(child, old, revision)
		}
	}
}

func TestBehaviorAdapterReverseLinkKeysDoNotCollide(t *testing.T) {
	registry := &BehaviorRegistry{}
	previous := BehaviorAdapterResult{Provider: ProviderRecord{BehaviorContracts: registry}, Variations: []BehaviorAdapterVariation{{ID: "a", Tests: []string{"b:c"}}}}
	current := BehaviorAdapterResult{Provider: ProviderRecord{BehaviorContracts: registry}, Variations: []BehaviorAdapterVariation{{ID: "a:b", Tests: []string{"c"}}}}
	delta, err := behaviorAdapterDelta(&previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(delta.LostReverseLinks, "variation-test:"+reverseLinkJoin("a", "b:c")) {
		t.Fatalf("colliding variation/test pair hid the lost link: %+v", delta.LostReverseLinks)
	}
}

func TestBehaviorAdapterMissingReverseLinkNamesEachTest(t *testing.T) {
	fixture := behaviorAdapterFixture(t)
	behaviorAdapterEditRow(t, &fixture.request, "variations", func(row map[string]any) { row["testKeys"] = []any{} })
	behaviorAdapterAppendRow(t, &fixture.request, "tests", func(row map[string]any) {
		row["testKey"] = "second-test"
		row["displayTitle"] = "Second test"
		row["variationClaims"] = []any{}
		delete(row, "witness")
	})
	result := buildBehaviorAdapter(t, fixture.request, nil)
	details := map[string]bool{}
	for _, diagnostic := range result.Frontier {
		if diagnostic.Kind == "missing-reverse-link" && diagnostic.Input == "variations" {
			details[diagnostic.Detail] = true
		}
	}
	if len(details) != 2 {
		t.Fatalf("two offending tests did not yield two distinct diagnostics: %+v", result.Frontier)
	}
}

func TestBehaviorAdapterArtifactsMatchPreviousValidator(t *testing.T) {
	fixture := behaviorAdapterFixture(t)
	previous := buildBehaviorAdapter(t, fixture.request, nil)
	if !slices.ContainsFunc(previous.Artifacts, func(artifact BehaviorAdapterArtifact) bool { return artifact.Role == "receipt" }) {
		t.Fatalf("receipt artifact missing: %+v", previous.Artifacts)
	}
	previousRaw, err := Encode(previous)
	if err != nil {
		t.Fatal(err)
	}
	buildBehaviorAdapter(t, fixture.request, previousRaw)
	unretained := fixture.request
	unretained.Inputs = slices.DeleteFunc(slices.Clone(fixture.request.Inputs), func(input BehaviorAdapterInput) bool { return input.ID == "receipt" })
	raw, _ := Encode(unretained)
	if _, err := BuildBehaviorAdapter(raw, nil); err == nil || !strings.Contains(err.Error(), "retained receipt input") {
		t.Fatalf("unretained observation input accepted: %v", err)
	}
	shared := unretained
	discovery := behaviorAdapterRequestInput(&shared, "discovery")
	shared.Observations = slices.Clone(fixture.request.Observations)
	shared.Observations[0].Input = discovery.Anchor.Path
	shared.Observations[0].InputRevision = discovery.Anchor.Revision
	shared.Observations[0].RunID = discovery.Anchor.SHA256
	raw, _ = Encode(shared)
	if _, err := BuildBehaviorAdapter(raw, nil); err == nil || !strings.Contains(err.Error(), "already serves the discovery role") {
		t.Fatalf("observation on the discovery input accepted: %v", err)
	}
}

func behaviorAdapterAppendRow(t *testing.T, request *BehaviorAdapterRequest, inputID string, edit func(map[string]any)) {
	t.Helper()
	input := behaviorAdapterRequestInput(request, inputID)
	var document map[string]any
	if err := jsonstd.Unmarshal([]byte(input.Document), &document); err != nil {
		t.Fatal(err)
	}
	inventory := document["inventory"].(map[string]any)
	records := inventory["items"].([]any)
	var row map[string]any
	if err := jsonstd.Unmarshal(behaviorAdapterRaw(t, records[0]), &row); err != nil {
		t.Fatal(err)
	}
	edit(row)
	inventory["items"] = append(records, row)
	input.Document = string(behaviorAdapterRaw(t, document))
	input.Anchor.SHA256 = Digest([]byte(input.Document))
	input.Anchor.SpanSHA256 = input.Anchor.SHA256
}
