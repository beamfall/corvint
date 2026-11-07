package intent_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func policyWithGateArgv(argv []string) (*intent.Policy, error) {
	v := fixture.PolicyValue()
	gates, _ := v.Obj.Get("gates")
	gates.Arr[0].Obj.Set("argv", wire.Strings(argv))
	return intent.DecodePolicy(wire.EncodeFile(v))
}

// TestTRMV0532_PolicyGateArgvAdmitsLongPathElement (V1-4164): a policy gate
// argv element is bounded by 4,096 bytes like a PathText, so a test or lane
// gate naming a long t.TempDir path is admitted; one byte more is refused
// LIMIT_EXCEEDED naming the bound, and the Identifier character rules stay.
func TestTRMV0532_PolicyGateArgvAdmitsLongPathElement(t *testing.T) {
	long := "/" + strings.Repeat("t", 200) + "/TestGateLongPath/001/bin/check"
	p, err := policyWithGateArgv([]string{long, "--all"})
	if err != nil {
		t.Fatalf("a %d-byte argv element must be admitted: %v", len(long), err)
	}
	if got := p.Gates[0].Argv[0]; got != long {
		t.Fatalf("argv element changed: %q", got)
	}
	at := "/" + strings.Repeat("t", wire.MaxGateArgBytes-1)
	if _, err := policyWithGateArgv([]string{at}); err != nil {
		t.Fatalf("a %d-byte element is at the bound: %v", len(at), err)
	}
	_, err = policyWithGateArgv([]string{at + "x"})
	if wire.CodeOf(err) != wire.CodeLimitExceeded || !strings.Contains(err.Error(), "4096") {
		t.Fatalf("over the bound must be LIMIT_EXCEEDED naming 4096: %v", err)
	}
	if _, err := policyWithGateArgv([]string{"make\tverify"}); err == nil {
		t.Fatal("a TAB in an argv element must stay refused")
	}
}
