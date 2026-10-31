package cli_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestONV0870_ObservationSkipsUnmatchedTerminalNotes (ON-V0-870, V1-0814):
// a running dispatcher reads a noted COMPLETED ticket's note only when one
// of its roles matches COMPLETED; an observation without a config, and every
// noted OPEN ticket, keep the ON-V0-011 view.
func TestONV0870_ObservationSkipsUnmatchedTerminalNotes(t *testing.T) {
	r, done := noteHistoryRepo(t, "SET:closed guidance")
	created := atm(t, r.Root, nil, "ticket", "create",
		"--request-id", "req-open", "--issued-at", "2026-09-07T12:00:00Z", "--payload", createPayloadJSON)
	if created.res.Outcome != wire.OutcomeOK {
		t.Fatalf("ticket create: %s", created.stdout)
	}
	open := field(created.res.Items[0], "ticketId").Str
	if x := atm(t, r.Root, nil, "ticket", "note", "set", open, "--text", "open guidance", "--request-id", "note-open",
		"--issued-at", "2026-10-05T12:00:00Z", "--supersedes", "0"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("note open: %s", x.stdout)
	}
	rev := field(field(atm(t, r.Root, nil, "ticket", "show", done).res.Items[0], "record"), "revision").Str
	if x := atm(t, r.Root, nil, "ticket", "complete-manual", "--request-id", "done-1", "--target", done, "--expected-revision", rev,
		"--issued-at", "2026-10-07T12:01:00Z", "--payload", `{"evidence":[],"reason":"manual"}`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("complete-manual: %s", x.stdout)
	}
	notes := func(c *dispatch.Config) map[string]*dispatch.NoteView {
		t.Helper()
		obs, err := cli.ObserveDispatch(r.Root)
		if c != nil {
			obs, err = cli.ObserveDispatchFor(r.Root, c)
		}
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]*dispatch.NoteView{}
		for _, tk := range obs.Tickets {
			out[tk.ID] = tk.OperatorNote
		}
		return out
	}
	role := func(statuses ...string) *dispatch.Config {
		return &dispatch.Config{Roles: []dispatch.Role{{Name: "worker", Cap: 1, Match: &dispatch.Match{Statuses: statuses}}}}
	}
	for _, c := range []struct {
		name     string
		config   *dispatch.Config
		wantDone bool
	}{{"no config", nil, true}, {"OPEN by default", role(), false}, {"OPEN only", role("OPEN"), false}, {"names COMPLETED", role("OPEN", "COMPLETED"), true}} {
		got := notes(c.config)
		if n := got[open]; n == nil || n.State != "CURRENT" || n.Text != "open guidance" {
			t.Fatalf("%s: OPEN ticket note %+v", c.name, n)
		}
		if n := got[done]; (n != nil) != c.wantDone || (n != nil && n.Text != "closed guidance") {
			t.Fatalf("%s: COMPLETED ticket note %+v, want read=%v", c.name, n, c.wantDone)
		}
	}
}
