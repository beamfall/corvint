package dispatch

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CAL-V0-084: the dispatcher carries the queue's advisory nextStage from the
// observed ticket to the launch: the {nextStage} placeholder renders it and
// the launched event records it. It never filters or refuses a launch.
func TestCALV0084_DispatcherNextStage(t *testing.T) {
	t.Run("CAL-V0-084 DispatcherNextStage", func(t *testing.T) {
		c := testConfig(t, `echo "$1" > "$CORVINT_DISPATCH_WORKER.prompt"`)
		c.Roles[0].Prompt = "work on {ticketLocal} next {nextStage}"
		raw, _ := json.Marshal(c)
		if _, err := DecodeConfig(raw); err != nil {
			t.Fatalf("{nextStage} placeholder refused: %v", err)
		}
		t1, t2 := ticket("t1", "P1", 1), ticket("t2", "P1", 2)
		t1.NextStage, t2.NextStage = "review", StateNone
		q := &fakeQueue{obs: Observation{Tickets: []Ticket{t1, t2}}}
		d, err := Open("prog", c, q, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()
		if err := d.Tick(context.Background()); err != nil || d.Running() != 2 {
			t.Fatalf("tick: %v running %d", err, d.Running())
		}
		waitEnded(t, d)
		want := map[string]string{"ticket:a:q:t1": "work on t1 next review", "ticket:a:q:t2": "work on t2 next NONE"}
		for _, w := range d.ledger.Workers {
			prompt, _ := os.ReadFile(filepath.Join(c.WorkRoot, w.ID+".prompt"))
			if got := strings.TrimSpace(string(prompt)); got != want[w.Ticket] {
				t.Fatalf("%s prompt = %q", w.Ticket, got)
			}
		}
		events, _ := ReadEvents(d.dir, 1000)
		seen := 0
		for _, e := range events {
			if e.Kind == "launched" {
				seen++
				if stage := map[string]string{"ticket:a:q:t1": "review", "ticket:a:q:t2": StateNone}[e.Ticket]; e.Detail["nextStage"] != stage {
					t.Fatalf("launched detail for %s: %v", e.Ticket, e.Detail)
				}
			}
		}
		if seen != 2 {
			t.Fatalf("launched events: %d", seen)
		}
	})
}
