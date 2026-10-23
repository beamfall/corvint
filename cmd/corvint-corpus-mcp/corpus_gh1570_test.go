package main

import (
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// DCP-V1-320: a tool failure envelope records a null verified_at and says why.
func TestDCPV1320ToolFailureVerifiedAtNull(t *testing.T) {
	value, rpc := toolFailureResult("corvint.docs_get", "query-refused", "refused")
	if rpc != nil {
		t.Fatal(rpc)
	}
	envelope := value["structuredContent"].(map[string]any)["trust_envelope"].(map[string]any)
	verified, present := envelope["verified_at"]
	if !present || verified != nil || !slices.Contains(envelope["limitations"].([]string), doccorpus.RefusalUnverifiedLimitation) {
		t.Fatalf("failure envelope: %v", envelope)
	}
}
