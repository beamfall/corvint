package store

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// V1-0829: a program or supervisor transition the writer did not complete is
// a coded error, so wire.CodeOf keeps the writer's refusal code.
func TestV10829_TransitionRefusalCarriesTheReportCode(t *testing.T) {
	refused := &Report{Kind: "Refused", Detail: "stale", Outcome: mutation.Outcome{Outcome: mutation.OutcomeBlocked, Codes: []string{wire.CodeStalePolicy, wire.CodeFenced}}}
	uncoded := &Report{Kind: "NoChange", Outcome: mutation.Outcome{Outcome: mutation.OutcomeBlocked}}
	for _, c := range []struct {
		name string
		err  error
		code string
	}{
		{"refused report", transitionRefusal("program transition WORKTREE_ADD", refused), wire.CodeStalePolicy},
		{"uncoded report", transitionRefusal("program transition ADMITTED", uncoded), wire.CodeMalformed},
		{"no report", transitionRefusal("program transition ADMITTED", nil), wire.CodeMalformed},
		{"transitionOK refused", transitionOK(refused, nil), wire.CodeStalePolicy},
	} {
		var e *wire.Error
		if !asWire(c.err, &e) || wire.CodeOf(c.err) != c.code {
			t.Fatalf("%s: %T %v, want coded %s", c.name, c.err, c.err, c.code)
		}
	}
	if e := transitionOK(&Report{Kind: "Transaction", Outcome: mutation.Outcome{Outcome: mutation.OutcomeCompleted}}, nil); e != nil {
		t.Fatalf("completed transition refused: %v", e)
	}
}

func asWire(err error, target **wire.Error) bool {
	e, ok := err.(*wire.Error)
	*target = e
	return ok
}
