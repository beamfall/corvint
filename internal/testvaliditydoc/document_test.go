package testvaliditydoc

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

func projectDocument(t *testing.T, body string) Document {
	t.Helper()
	input, err := Decode([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	return Project(input)
}

func TestGoSessionIsProjectedFromObservations(t *testing.T) {
	document := projectDocument(t, `{"profile":"corvint-go-live-session-event/0","state":"passed","identity":"abc","sequence":4,"scope":["./..."],"detail":"","projection":{},"testProjections":[{"package":"example/a","name":"TestAdds","action":"pass","projection":{}}],"testProjectionsOmitted":2}`)
	if document.Source != "corvint-go-test-provider" || document.Kind != "go-session" || document.TestsOmitted != 2 || len(document.Tests) != 1 {
		t.Fatalf("document=%+v", document)
	}
	test := document.Tests[0]
	if test.Package != "example/a" || test.Name != "TestAdds" || test.State != "pass" ||
		test.Projection.Execution.State != testvalidity.ExecutionPassed ||
		test.Projection.Freshness.State != testvalidity.FreshnessCurrent ||
		document.Run.Execution.State != testvalidity.ExecutionPassed {
		t.Fatalf("document=%+v", document)
	}
}

func TestGoSessionCarriedProjectionsAreIgnored(t *testing.T) {
	document := projectDocument(t, `{"profile":"corvint-go-live-session-event/0","state":"failed","identity":"abc","sequence":4,"scope":["./..."],"detail":"","projection":{"execution":{"state":"PASSED"},"strength":{"state":"KILLED"}},"testProjections":[{"package":"example/a","name":"TestFails","action":"fail","projection":{"execution":{"state":"PASSED"},"strength":{"state":"KILLED"}}},{"package":"example/a","name":"TestIncomplete","projection":{"execution":{"state":"PASSED"},"strength":{"state":"KILLED"}}}],"testProjectionsOmitted":0}`)
	if document.Run.Execution.State != testvalidity.ExecutionFailed || document.Run.Strength.State != testvalidity.StrengthNotMeasured {
		t.Fatalf("run=%+v", document.Run)
	}
	if got := document.Tests[0].Projection; got.Execution.State != testvalidity.ExecutionFailed || got.Strength.State != testvalidity.StrengthNotMeasured {
		t.Fatalf("recomputed projection=%+v", got)
	}
	if got := document.Tests[1].Projection; got.Execution.State != testvalidity.StateUnsupported || got.Execution.Reason != "no-execution-input" {
		t.Fatalf("missing action projection=%+v", got)
	}
}

func TestUnknownAndAmbiguousProviderKindsAreRefused(t *testing.T) {
	for name, body := range map[string]string{
		"unknown":   `{"profile":"other-provider/0"}`,
		"ambiguous": `{"profile":"corvint-go-live-session-event/0","receipt":{"kind":"unit","tests":[]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Decode([]byte(body)); err == nil {
				t.Fatal("Decode accepted an unclassified document")
			}
		})
	}
}

func TestGoSessionDisclosesPreviewTierAndCannotPromote(t *testing.T) {
	document := projectDocument(t, `{"profile":"corvint-go-live-session-event/0","state":"passed","identity":"abc","sequence":4,"scope":[],"detail":"","projection":{},"testProjections":[],"testProjectionsOmitted":0}`)
	if document.Tier != "preview" || document.Promotable == nil || *document.Promotable {
		t.Fatalf("tier=%q promotable=%v", document.Tier, document.Promotable)
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatal(err)
	}
	if object["tier"] != "preview" || object["promotable"] != false {
		t.Fatalf("document=%s", encoded)
	}
}

func TestJavaScriptDocumentShapeIsUnchanged(t *testing.T) {
	document := projectDocument(t, `{"receipt":{"kind":"unit","tests":[{"name":"adds","state":"passed"}]},"testProjections":[],"runProjection":{}}`)
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	want := map[string]bool{"schema": true, "source": true, "kind": true, "tests": true, "run": true}
	got := map[string]bool{}
	for _, key := range keys {
		got[key] = true
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("keys=%v document=%s", got, encoded)
	}
	test := object["tests"].([]any)[0].(map[string]any)
	if _, present := test["package"]; present {
		t.Fatalf("JavaScript test gained package: %s", encoded)
	}
}
