package intent_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func failurePatternGate(pattern *string) wire.Value {
	o := wire.NewObject().Set("gateId", wire.String("patterned")).Set("kind", wire.String("COMMAND")).
		Set("argv", wire.Strings([]string{"make", "gate"})).Set("cwd", wire.String("WORKTREE")).
		Set("env", wire.Strings(nil)).Set("timeoutSeconds", wire.String("30")).
		Set("expected", wire.ObjectValue(wire.NewObject().Set("exitCode", wire.String("0")).Set("reducer", wire.Null()))).
		Set("evidence", wire.Strings(nil)).Set("inputs", wire.Strings(nil)).Set("sharedResource", wire.Null()).
		Set("reusable", wire.Bool(false)).Set("required", wire.Bool(false))
	if pattern != nil {
		o.Set("failurePattern", wire.String(*pattern))
	}
	return wire.ObjectValue(o)
}

// TestTCDV0801_PolicyGateFailurePattern: a gate's optional failurePattern
// decodes to a compiled expression, its absence leaves nil, and an empty,
// over-long or invalid expression refuses MALFORMED.
func TestTCDV0801_PolicyGateFailurePattern(t *testing.T) {
	decode := func(gate wire.Value) (*intent.Policy, error) {
		v := fixture.PolicyValue()
		gates, _ := v.Obj.Get("gates")
		v.Obj.Set("gates", wire.Array(append(append([]wire.Value{}, gates.Arr...), gate)...))
		return intent.DecodePolicy(wire.EncodeFile(v))
	}
	p, err := decode(failurePatternGate(nil))
	if err != nil || p.Gate("patterned") == nil || p.Gate("patterned").FailurePattern != nil {
		t.Fatalf("absent pattern: %v", err)
	}
	ok := `^FAIL `
	p, err = decode(failurePatternGate(&ok))
	if err != nil || p.Gate("patterned").FailurePattern == nil || !p.Gate("patterned").FailurePattern.MatchString("FAIL x") {
		t.Fatalf("pattern: %v", err)
	}
	long := make([]byte, intent.MaxGateFailurePatternBytes+1)
	for i := range long {
		long[i] = 'a'
	}
	for name, bad := range map[string]string{"empty": "", "over-long": string(long), "invalid": "(unclosed"} {
		if _, err := decode(failurePatternGate(&bad)); wire.CodeOf(err) != wire.CodeMalformed {
			t.Fatalf("%s: %v", name, err)
		}
	}
}
