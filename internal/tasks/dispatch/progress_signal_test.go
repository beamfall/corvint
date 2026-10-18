//go:build darwin || linux

package dispatch

import (
	"context"
	"io"
	"strings"
	"testing"
)

// ATR-V0-084: under a declared HANDOFF_EVIDENCE signal a changed hand-off
// reference changes the CAL-V0-057 fingerprint and the same one does not;
// without the declaration the reference is ignored.
func TestATRV0084_FingerprintHonoursProgressSignal(t *testing.T) {
	t.Run("ATR-V0-084 FingerprintHonoursProgressSignal", func(t *testing.T) {
		const key = "ticket:a:q:t"
		obs := func(signal, ref string) *Observation {
			return &Observation{
				ProgressSignal: signal,
				Tickets:        []Ticket{{ID: key, Status: "READY", Revision: "1"}},
				Attempts:       []Attempt{{ID: "at-1", Ticket: key, Phase: "RELEASED", HandoffEvidence: ref}},
			}
		}
		empty := Fingerprint(obs(ProgressSignalHandoffEvidence, ""), key)
		a := Fingerprint(obs(ProgressSignalHandoffEvidence, "local:a"), key)
		if a == empty || a != Fingerprint(obs(ProgressSignalHandoffEvidence, "local:a"), key) {
			t.Fatal("declared: a reference is not stable progress identity")
		}
		if Fingerprint(obs(ProgressSignalHandoffEvidence, "local:b"), key) == a {
			t.Fatal("declared: a changed reference left the fingerprint unchanged")
		}
		plain := Fingerprint(obs("", ""), key)
		if plain != empty || Fingerprint(obs("", "local:a"), key) != plain || Fingerprint(obs("", "local:b"), key) != plain {
			t.Fatal("undeclared: the hand-off reference changed the fingerprint")
		}
	})
}

// ATR-V0-083, ATR-V0-084: under a declared signal the dispatcher's
// LOOP_DETECTED needs-owner event and its no-progress cooldown name the
// signal that judged them; without the declaration neither carries it.
func TestATRV0083_DispatcherEventsNameProgressSignal(t *testing.T) {
	t.Run("ATR-V0-083 ATR-V0-084 DispatcherEventsNameProgressSignal", func(t *testing.T) {
		for _, signal := range []string{ProgressSignalHandoffEvidence, ""} {
			c := testConfig(t, "exit 0")
			held, free := ticket("h", "P0", 1), ticket("f", "P1", 2)
			held.Loop = &LoopHold{Signal: "NO_PROGRESS", AcceptanceRevision: "0", Generations: []string{"1", "2", "3"}}
			q := &fakeQueue{obs: Observation{ProgressSignal: signal, Tickets: []Ticket{held, free}}}
			d, err := Open("prog", c, q, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Tick(context.Background()); err != nil {
				t.Fatal(err)
			}
			waitEnded(t, d)
			if err := d.Tick(context.Background()); err != nil {
				t.Fatal(err)
			}
			named := 0
			for _, e := range eventsOf(t, d, "needs-owner") {
				if e.Detail["code"] != "LOOP_DETECTED" {
					continue
				}
				named++
				if e.Detail["progressSignal"] != signal || strings.Contains(e.Message, "progress signal") != (signal != "") {
					t.Fatalf("signal %q: loop escalation %+v", signal, e)
				}
			}
			cool := eventsOf(t, d, "cooldown")
			if named != 1 || len(cool) != 1 || cool[0].Detail["progressSignal"] != signal || strings.Contains(cool[0].Message, "progress signal") != (signal != "") {
				t.Fatalf("signal %q: escalations %d cooldowns %+v", signal, named, cool)
			}
			d.Close()
		}
	})
}
