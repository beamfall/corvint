package bridge

import (
	"context"
	"encoding/json"
	"github.com/Beamfall/corvint/internal/flowcoverage"
	"github.com/Beamfall/corvint/internal/flowcoverage/testfixture"
	"reflect"
	"testing"
)

func TestFlowCoverageMCPParity(t *testing.T) {
	root := testfixture.Repository(t)
	registry, e := NewFlows(root)
	if e != nil {
		t.Fatal(e)
	}
	result, callErr := registry.Call(context.Background(), ToolFlowsCoverage, []byte(`{"denominator":"denominator.json","receipts":"runs.json","offset":0,"limit":1}`))
	if callErr != nil {
		t.Fatal(callErr)
	}
	coverage, err := flowcoverage.Compile(context.Background(), root, flowcoverage.Options{Denominator: "denominator.json", Receipts: "runs.json"})
	if err != nil {
		t.Fatal(err)
	}
	page, _ := coverage.Page(0, 1)
	var expected any
	json.Unmarshal(page, &expected)
	b, _ := json.Marshal(result)
	var envelope map[string]any
	json.Unmarshal(b, &envelope)
	found := false
	for _, value := range envelope {
		if reflect.DeepEqual(value, expected) {
			found = true
		}
	}
	if !found {
		t.Fatalf("MCP differs from CLI page: %s", b)
	}
	if !EnvelopeOnly(ToolFlowsCoverage) {
		t.Fatal("repository text escaped envelope")
	}
	for _, arg := range []string{`{"denominator":"denominator.json","receipts":"runs.json","write-back":"x"}`, `{"denominator":"denominator.json","receipts":"runs.json","limit":0}`} {
		if _, e := registry.Call(context.Background(), ToolFlowsCoverage, []byte(arg)); e == nil {
			t.Fatal("invalid/write argument accepted")
		}
	}
}
func TestDefaultCoverageAbsent(t *testing.T) {
	r, e := New(testfixture.Repository(t))
	if e != nil {
		t.Fatal(e)
	}
	if _, e := r.Call(context.Background(), ToolFlowsCoverage, []byte(`{}`)); e == nil {
		t.Fatal("default profile acquired coverage")
	}
}
