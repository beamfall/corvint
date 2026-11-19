// SPDX-License-Identifier: Apache-2.0
package forwardrow

import (
	"encoding/json"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestRuntimeEdgeDegradationRecord_CINV1942(t *testing.T) {
	detail := `runtime:unresolved-edge: tests="holder_test.g" runs="new:target": runs names no unit; reason="new runtime tool"`
	for _, mode := range []string{"NARROWED", "FULL"} {
		for _, degraded := range []bool{false, true} {
			record := doc{"mode": mode, "reasonCodes": []any{}, "reasons": []any{}, "selected": []any{"holder", "downstream"}, "executed": []any{"holder", "downstream"}}
			unknown := []any{doc{"reason": "LANGUAGE_FRONTIER", "detail": "go:future-frontier"}}
			if degraded {
				unknown = append(unknown, doc{"reason": "LANGUAGE_FRONTIER", "detail": detail})
			}
			plan := doc{"plan": doc{"unknown": unknown}}
			input, err := json.Marshal([]any{record, plan})
			if err != nil {
				t.Fatal(err)
			}
			// -s in production supplies this array from the two pinned documents.
			command := exec.Command("jq", "-f", "runtime-edges.jq")
			command.Stdin = strings.NewReader(string(input))
			raw, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("record projection: %v: %s", err, raw)
			}
			var got doc
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			if degraded {
				record["degradedRuntimeEdges"] = []any{detail}
			}
			if !reflect.DeepEqual(got, record) {
				t.Fatalf("record=%v want=%v", got, record)
			}
		}
	}
}
