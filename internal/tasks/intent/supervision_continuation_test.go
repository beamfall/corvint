package intent_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0089_PolicyContinuationsBound proves supervision.continuations is
// optional, absent means no checkpointed continuation, and a declared value
// outside 1..MaxStageContinuations is refused with LIMIT_EXCEEDED.
func TestCALV0089_PolicyContinuationsBound(t *testing.T) {
	p, e := supervisionPolicy(t, "")
	if e != nil || p.Supervision.StageContinuations() != 0 {
		t.Fatalf("absent continuations %v", e)
	}
	var none *intent.SupervisionPolicy
	if none.StageContinuations() != 0 {
		t.Fatal("nil supervision must allow no continuation")
	}
	for want, raw := range map[int]string{1: `"1"`, 2: `"2"`, intent.MaxStageContinuations: `"16"`} {
		p, e = supervisionPolicy(t, `"continuations":`+raw+`,`)
		if e != nil || p.Supervision.StageContinuations() != want {
			t.Fatalf("continuations %s: %v", raw, e)
		}
	}
	for _, over := range []string{`"0"`, `"17"`, `"1000"`} {
		if _, e := supervisionPolicy(t, `"continuations":`+over+`,`); wire.CodeOf(e) != wire.CodeLimitExceeded {
			t.Fatalf("continuations %s: %v", over, e)
		}
	}
	for _, bad := range []string{`"-1"`, `"01"`, `"two"`} {
		if _, e := supervisionPolicy(t, `"continuations":`+bad+`,`); e == nil {
			t.Fatalf("accepted continuations %s", bad)
		}
	}
}
