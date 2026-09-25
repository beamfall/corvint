package cli

import (
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"path/filepath"
	"testing"
)

func TestTMV0008_AS36_ShowRetriesDiscardPriorAttempt(t *testing.T) {
	t.Run("removed gate", func(t *testing.T) {
		r := fixture.TempRepo(t)
		fixture.WriteState(t, r)
		fixture.WriteIntent(t, r)
		calls := 0
		res := gateShow(Env{Cwd: r.Root, afterRead: func() {
			calls++
			if calls == 1 {
				p := fixture.PolicyValue()
				p.Obj.Set("gates", wire.Array())
				fixture.Write(t, filepath.Join(r.IntentDir, intent.PolicyFile), wire.EncodeFile(p))
			}
		}}, []string{"verify"})
		if calls != 2 || res.Outcome != wire.OutcomeRefused || len(res.Items) != 0 || len(res.Codes) != 1 || res.Codes[0] != wire.CodeGateUnknown {
			t.Fatalf("calls=%d result=%+v", calls, res)
		}
	})
	for _, record := range []bool{true, false} {
		t.Run(map[bool]string{true: "show appearing ticket", false: "blockers appearing ticket"}[record], func(t *testing.T) {
			r := fixture.TempRepo(t)
			fixture.WriteState(t, r)
			fixture.WriteIntent(t, r)
			calls := 0
			res := ticketShow(Env{Cwd: r.Root, afterRead: func() {
				calls++
				if calls == 1 {
					fixture.WriteIntent(t, r, fixture.Ticket("A"))
				}
			}}, []string{"A"}, record)
			if calls != 2 || res.Outcome != wire.OutcomeOK || len(res.Items) != 1 || len(res.Warnings) != 0 {
				t.Fatalf("calls=%d result=%+v", calls, res)
			}
		})
	}
}
