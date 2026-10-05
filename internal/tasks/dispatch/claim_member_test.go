package dispatch

import (
	"io"
	"strings"
	"testing"
)

// CAL-V0-079: the dispatcher claim event names the claim's pool and member,
// and leaves both empty for a claim without a pool allocation.
func TestCALV0079_ClaimEventNamesMember(t *testing.T) {
	t.Run("CAL-V0-079 ClaimEventNamesMember", func(t *testing.T) {
		c := testConfig(t, "exit 0")
		q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1), ticket("t2", "P1", 1)}}}
		d, err := Open("prog", c, q, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		d.diff(&q.obs)
		q.obs.Attempts = []Attempt{
			{ID: "pooled", Ticket: "t1", Phase: "RUNNING", Holder: "w1", Stage: "implement", Live: true, Pool: "db", Member: "b"},
			{ID: "plain", Ticket: "t2", Phase: "RUNNING", Holder: "w2", Live: true},
		}
		d.diff(&q.obs)
		got := map[string]Event{}
		for _, e := range eventsOf(t, d, "claim") {
			got[e.Detail["attempt"]] = e
		}
		if e := got["pooled"]; e.Detail["pool"] != "db" || e.Detail["member"] != "b" || !strings.Contains(e.Message, " on db/b ") {
			t.Fatalf("pooled claim event %+v", e)
		}
		if e, ok := got["plain"]; !ok || e.Detail["pool"] != "" || e.Detail["member"] != "" || strings.Contains(e.Message, " on ") {
			t.Fatalf("plain claim event %+v", e)
		}
	})
}
