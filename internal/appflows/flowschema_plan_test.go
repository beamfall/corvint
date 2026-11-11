// SPDX-License-Identifier: Apache-2.0

package appflows

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// FSV-V1-004 (V1-5410): a declaration of a schema version this build does
// not read selects every E2E test with FLOWS_SCHEMA_UNSUPPORTED, naming the
// found and supported versions, narrows nothing on the file and leaves the
// unit and integration selections as they are without a declaration; a
// malformed profile stays FLOWS_DECLARATION_INVALID.
func TestFSVV1004UnsupportedVersionSelectsEveryE2ETest_V15410(t *testing.T) {
	f := flowPlanRepo(t, true)
	// The supported file narrows: a change to checkout leaves search out.
	_, supported := f.plan(t, "src/checkout.ts", "lib/lib.go")
	if supported.E2E == nil || supported.E2E.Scope != affected.E2EScopeBounded || len(supported.E2E.Selected) >= supported.E2E.Inventory.Count {
		t.Fatalf("the version 0 control does not narrow: %+v", supported.E2E)
	}
	for _, body := range []string{
		`{"profile":"corvint-flows/7","flows":"flows"}`,
		// A newer file's new member and a value version 0 would refuse.
		`{"profile":"corvint-flows/7","flows":"flows","kinds":[{"path":"lib","kind":"e2e"}],"variants":[]}`,
	} {
		writeRaw(t, f.root, FlowsDeclarationPath, []byte(body))
		before, plan := f.plan(t, "src/checkout.ts", "lib/lib.go")
		if !reflect.DeepEqual(plan.Selected, before.Selected) {
			t.Fatalf("%s: unit and integration selection changed:\n%+v\n%+v", body, before.Selected, plan.Selected)
		}
		e2e := plan.E2E
		if e2e == nil || e2e.Scope != affected.E2EScopeFull || len(e2e.Selected) != 0 || len(e2e.Unknown) != 1 {
			t.Fatalf("%s: %+v", body, e2e)
		}
		unknown := e2e.Unknown[0]
		if unknown.Reason != affected.UnknownFlowsSchemaUnsupported || unknown.Detail != FlowsDeclarationPath+" version 7; supported 0" {
			t.Fatalf("%s: unknown %+v", body, unknown)
		}
		for _, want := range []string{"schema version 7", "reads version 0", "every E2E test is selected"} {
			if !strings.Contains(unknown.Hint, want) {
				t.Fatalf("%s: hint %q lacks %q", body, unknown.Hint, want)
			}
		}
	}
	writeRaw(t, f.root, FlowsDeclarationPath, []byte(`{"profile":"flows/1","flows":"flows"}`))
	_, plan := f.plan(t, "src/checkout.ts")
	if plan.E2E == nil || !reflect.DeepEqual(e2eUnknownReasons(plan), []string{affected.UnknownFlowsDeclarationInvalid}) {
		t.Fatalf("flows/1: %+v", plan.E2E)
	}
}
