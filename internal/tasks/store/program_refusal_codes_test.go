package store_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// V1-0829: supervisor transitions and program controls that refuse before the
// writer runs return coded errors, so wire.CodeOf keeps the refusal.
func TestV10829_ProgramRefusalsCarryWireCodes(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	ctx := context.Background()
	for _, c := range []struct {
		action, want, msg string
	}{
		{"HEARTBEAT", wire.CodeFenced, "heartbeat lane absent"},
		{"DISPATCH", wire.CodeFenced, "dispatch attempt missing"},
	} {
		_, e := store.SupervisorTransition(ctx, s.repo, operator(), "queue:acme:main", "absent-"+c.action, "attempt:absent", "1", transaction.SupervisorChange{Action: c.action, ProgramID: "program", Stage: "implement", Worktree: s.root})
		if wire.CodeOf(e) != c.want || !strings.Contains(e.Error(), c.msg) {
			t.Fatalf("%s on an absent attempt: %v, want %s", c.action, e, c.want)
		}
	}
	e := store.RequestProgramControl(ctx, s.repo, operator(), "program", "STOP")
	if wire.CodeOf(e) != wire.CodeMalformed || !strings.Contains(e.Error(), "program program absent") {
		t.Fatalf("control on an absent program: %v", e)
	}
	var coded *wire.Error
	if !asCoded(e, &coded) {
		t.Fatalf("control on an absent program is uncoded: %T", e)
	}
}

func asCoded(err error, target **wire.Error) bool {
	e, ok := err.(*wire.Error)
	*target = e
	return ok
}
