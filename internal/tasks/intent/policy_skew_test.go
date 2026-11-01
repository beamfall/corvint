package intent_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCTSV0991_NewerPolicyKeyIsVersionSkew: a policy with every required key
// plus one this build does not know is version skew, named with its profile
// and policyVersion; a missing required key stays MALFORMED (CTS-V0-991).
func TestCTSV0991_NewerPolicyKeyIsVersionSkew(t *testing.T) {
	v := fixture.PolicyValue()
	v.Obj.Set("zFutureKey", wire.Array())
	_, err := intent.DecodePolicy(wire.EncodeFile(v))
	if code(err) != wire.CodeUnsupportedVersion {
		t.Fatalf("newer policy key: %v", err)
	}
	for _, want := range []string{"zFutureKey", intent.ProfilePolicy, "policyVersion 1", "newer corvint-tasks"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not name %q: %v", want, err)
		}
	}
	full := fixture.PolicyValue()
	trimmed := wire.NewObject()
	for _, k := range full.Obj.Keys {
		if k != "gates" {
			f, _ := full.Obj.Get(k)
			trimmed.Set(k, f)
		}
	}
	trimmed.Set("zFutureKey", wire.Array())
	missing := wire.ObjectValue(trimmed)
	if _, err := intent.DecodePolicy(wire.EncodeFile(missing)); code(err) != wire.CodeMalformed {
		t.Fatalf("missing required key with an unknown one: %v", err)
	}
	if _, err := intent.DecodePolicy(fixture.PolicyBytes()); err != nil {
		t.Fatalf("current policy: %v", err)
	}
}
