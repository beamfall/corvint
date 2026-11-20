// SPDX-License-Identifier: Apache-2.0

package testinventory

import (
	"strings"
	"testing"
)

// FLP-V1-007, TIV-V1-009: the shared reader preserves inventory diagnostics and
// grants no listed IDs when the record shape or profile cannot be interpreted.
func TestSharedReaderInvalidInventoryKeepsInstanceFull(t *testing.T) {
	for _, tc := range []struct{ name, inventory, detail string }{
		{"invalid instance", `{"profile":"corvint-test-inventory/0","instances":[false],"failures":[]}`, "testinventory.Instance"},
		{"future profile", `{"profile":"corvint-test-inventory/9","future":[]}`, "unknown profile"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			verdict, listed := JudgeListed(MapTree{"go.mod": "module example.test/shop\n", Path: tc.inventory})
			if verdict.State != StateInvalid || verdict.Reason != ReasonInvalid || !strings.Contains(verdict.Detail, tc.detail) {
				t.Fatalf("verdict: %+v", verdict)
			}
			if len(verdict.Instances) != 1 || verdict.Instances[0].Key != "go@." || !verdict.Instances[0].Full || len(listed) != 0 {
				t.Fatalf("instances: %+v, listed: %v", verdict.Instances, listed)
			}
		})
	}
}
