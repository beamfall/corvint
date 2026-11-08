// SPDX-License-Identifier: Apache-2.0
package flows

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRSPV0051FlowProjection(t *testing.T) {
	fx := newShopFixture(t)
	code, out, diag := runFlowsCLI(fx.root, "test-evidence", "--flows", "flows", "checkout/checkout.go")
	if code != 0 || diag != "" {
		t.Fatalf("%d %s", code, diag)
	}
	var d struct {
		Schema   string
		Flows    json.RawMessage
		Evidence struct {
			Status  string
			Reasons []string
		}
	}
	if err := json.Unmarshal([]byte(out), &d); err != nil {
		t.Fatal(err)
	}
	if d.Schema != "application-flow-test-evidence/1" || d.Evidence.Status != "UNKNOWN" || !strings.Contains(string(d.Flows), "checkout") {
		t.Fatal(out)
	}
	for _, args := range [][]string{{"test-evidence", "checkout/checkout.go"}, {"test-evidence", "--flows", "flows", "checkout/checkout.go", "--revision", fx.b}} {
		code, _, _ = runFlowsCLI(fx.root, args...)
		if code != 2 {
			t.Fatalf("accepted %v", args)
		}
	}
}
