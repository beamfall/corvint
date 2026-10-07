//go:build darwin || linux

package dispatch

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// stallSession runs one session to its end, applying during while it runs,
// and returns its finished detail. The tick that finishes a session may
// launch the next one, which the following call then runs.
func stallSession(t *testing.T, d *Dispatcher, during func()) map[string]string {
	t.Helper()
	ctx := context.Background()
	n := len(eventsOf(t, d, "finished"))
	if d.Running() == 0 {
		if err := d.Tick(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if d.Running() != 1 {
		t.Fatalf("running %d, want one session", d.Running())
	}
	if during != nil {
		during()
	}
	waitEnded(t, d)
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	ev := eventsOf(t, d, "finished")
	if len(ev) != n+1 {
		t.Fatalf("%d finished events, want %d", len(ev), n+1)
	}
	return ev[n].Detail
}

// TestCALV0185_StallCountsSessionsWithoutStatusChange: every finished
// session of a ticket whose native status did not change counts, even when
// the session edited the ticket and was recorded as progress; the typed
// stalled event fires once at the threshold and holds nothing; a status
// change restarts the count and a terminal status drops it; the count
// survives a ledger reload.
func TestCALV0185_StallCountsSessionsWithoutStatusChange(t *testing.T) {
	t.Run("CAL-V0-185", func(t *testing.T) {
		c := testConfig(t, "exit 0")
		c.StalledAfterSessions = intp(2)
		q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
		d, err := Open("prog", c, q, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = d.Close() })
		const key = "ticket:a:q:t1"
		for i := 1; i <= 3; i++ {
			// The session edits the ticket (a new revision), which the
			// dispatcher records as progress, but leaves the status OPEN.
			detail := stallSession(t, d, func() { q.obs.Tickets[0].Revision = strconv.Itoa(i + 1) })
			if detail["progress"] != "true" || detail["sessionsSinceStatusChange"] != strconv.Itoa(i) {
				t.Fatalf("session %d finished detail = %v", i, detail)
			}
			stalled := eventsOf(t, d, "stalled")
			want := 0
			if i >= 2 {
				want = 1
			}
			if len(stalled) != want {
				t.Fatalf("session %d: %d stalled events, want %d", i, len(stalled), want)
			}
			if want == 1 {
				dt := stalled[0].Detail
				if stalled[0].Ticket != key || dt["sessions"] != "2" || dt["status"] != "OPEN" || dt["threshold"] != "2" {
					t.Fatalf("stalled event = %+v", stalled[0])
				}
			}
			// Advisory only: nothing parks, cools down or holds.
			if b := d.ledger.Backoff[key]; b != nil && (b.Parked || b.NoProgress != 0) {
				t.Fatalf("session %d: backoff %+v", i, b)
			}
		}
		l, err := LoadLedger(d.dir, "prog")
		if err != nil || l.Stall[key] == nil || l.Stall[key].Status != "OPEN" || l.Stall[key].Sessions < 3 {
			t.Fatalf("saved stall = %+v %v", l.Stall, err)
		}
		if rows, omitted := l.StallRows(); len(rows) != 1 || rows[0].Ticket != key || rows[0].Sessions != l.Stall[key].Sessions || omitted != 0 {
			t.Fatalf("stall rows = %+v %d", rows, omitted)
		}

		// A session that moves the ticket to HELD reports zero and restarts
		// the count against HELD; a HELD ticket is not launched.
		if detail := stallSession(t, d, func() { q.obs.Tickets[0].Status = "HELD" }); detail["sessionsSinceStatusChange"] != "0" {
			t.Fatalf("held session = %v", detail)
		}
		if s := d.ledger.Stall[key]; s == nil || *s != (StallState{Status: "HELD"}) || d.Running() != 0 {
			t.Fatalf("stall after the hold = %+v running %d", s, d.Running())
		}
		// A status change between sessions restarts the count too.
		q.obs.Tickets[0].Status = "OPEN"
		if detail := stallSession(t, d, nil); detail["sessionsSinceStatusChange"] != "1" {
			t.Fatalf("count after the release = %v", detail)
		}

		// A session that completes the ticket reports zero and drops it.
		if detail := stallSession(t, d, func() { q.obs.Tickets[0].Status = "COMPLETED" }); detail["sessionsSinceStatusChange"] != "0" || len(d.ledger.Stall) != 0 {
			t.Fatalf("completed: detail %v stall %+v", detail, d.ledger.Stall)
		}
		if n := len(eventsOf(t, d, "stalled")); n != 1 {
			t.Fatalf("%d stalled events in all, want 1", n)
		}
		if l, err := LoadLedger(d.dir, "prog"); err != nil || l.Stall != nil {
			t.Fatalf("saved stall after completion = %+v %v", l, err)
		}
	})
}

// TestCALV0185_StallWithoutThresholdOrSeed: without the threshold the
// counts are kept and no stalled event is emitted; a session the ledger has
// no launch seed for is UNKNOWN, never a guessed count.
func TestCALV0185_StallWithoutThresholdOrSeed(t *testing.T) {
	t.Run("CAL-V0-185", func(t *testing.T) {
		c := testConfig(t, "exit 0")
		c.Backoff.ParkAfter = 100
		q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
		d, err := Open("prog", c, q, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = d.Close() })
		// As a ledger saved before the launch seed existed.
		if got := stallSession(t, d, func() { d.ledger.Stall = nil })["sessionsSinceStatusChange"]; got != "UNKNOWN" {
			t.Fatalf("unseeded session = %q", got)
		}
		for i := 1; i <= 3; i++ {
			if detail := stallSession(t, d, nil); detail["sessionsSinceStatusChange"] != strconv.Itoa(i) {
				t.Fatalf("session %d detail = %v", i, detail)
			}
		}
		if n := len(eventsOf(t, d, "stalled")); n != 0 {
			t.Fatalf("%d stalled events without a threshold", n)
		}
	})
}

// TestCALV0185_StallConfigAndLedgerAreClosed: the threshold is 1..1000 and
// a present 0 or null is refused; the ledger member is strict and bounded,
// and a build that does not know it refuses it as UNSUPPORTED_VERSION.
func TestCALV0185_StallConfigAndLedgerAreClosed(t *testing.T) {
	t.Run("CAL-V0-185", func(t *testing.T) {
		base := testConfig(t, "exit 0")
		raw, _ := json.Marshal(base)
		with := func(v string) []byte {
			return []byte(strings.Replace(string(raw), `"profile":`, `"stalledAfterSessions":`+v+`,"profile":`, 1))
		}
		for _, v := range []string{"1", "1000"} {
			if c, err := DecodeConfig(with(v)); err != nil || c.StalledAfterSessions == nil || strconv.Itoa(*c.StalledAfterSessions) != v {
				t.Fatalf("stalledAfterSessions %s: %+v %v", v, c, err)
			}
		}
		for _, v := range []string{"0", "-1", "1001", "null", `"2"`, "1.5"} {
			if _, err := DecodeConfig(with(v)); err == nil {
				t.Errorf("stalledAfterSessions %s accepted", v)
			}
		}
		if _, err := DecodeConfig([]byte(strings.Replace(string(raw), `"profile":`, `"StalledAfterSessions":null,"profile":`, 1))); err == nil {
			t.Error("case-folded null accepted")
		}

		dir := t.TempDir()
		write := func(stall string) error {
			l := `{"profile":"` + StateProfile + `","program":"prog","launchSeq":0,"eventSeq":0,"workers":[],"backoff":{}` + stall + `}`
			if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(l), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := LoadLedger(dir, "prog")
			return err
		}
		if err := write(`,"stall":{"ticket:a:q:t1":{"status":"OPEN","sessions":4}}`); err != nil {
			t.Fatalf("valid stall refused: %v", err)
		}
		for name, stall := range map[string]string{
			"empty map":         `,"stall":{}`,
			"null map":          `,"stall":null`,
			"null entry":        `,"stall":{"ticket:a:q:t1":null}`,
			"missing sessions":  `,"stall":{"ticket:a:q:t1":{"status":"OPEN"}}`,
			"missing status":    `,"stall":{"ticket:a:q:t1":{"sessions":1}}`,
			"null sessions":     `,"stall":{"ticket:a:q:t1":{"status":"OPEN","sessions":null}}`,
			"unknown member":    `,"stall":{"ticket:a:q:t1":{"status":"OPEN","sessions":1,"x":1}}`,
			"repeated member":   `,"stall":{"ticket:a:q:t1":{"status":"OPEN","sessions":1,"sessions":2}}`,
			"terminal status":   `,"stall":{"ticket:a:q:t1":{"status":"COMPLETED","sessions":1}}`,
			"negative sessions": `,"stall":{"ticket:a:q:t1":{"status":"OPEN","sessions":-1}}`,
			"huge sessions":     `,"stall":{"ticket:a:q:t1":{"status":"OPEN","sessions":1048577}}`,
			"bad ticket key":    `,"stall":{"t1":{"status":"OPEN","sessions":1}}`,
			"array":             `,"stall":[]`,
		} {
			if err := write(stall); err == nil {
				t.Errorf("%s: accepted", name)
			}
		}
	})
}

// TestCALV0185_DrainedVersion2LedgerIsAdopted: the stall counts moved the
// ledger to taskman-dispatch-state/3 (CAL-V0-131). A drained /2 ledger is
// adopted as /3 with its budget history kept; a /2 ledger that records a
// worker or carries the /3 stall member refuses UNSUPPORTED_VERSION.
func TestCALV0185_DrainedVersion2LedgerIsAdopted(t *testing.T) {
	t.Run("CAL-V0-185", func(t *testing.T) {
		dir := t.TempDir()
		budget := `"budget":{"sessions":[],"truncated":"0001-01-01T00:00:00Z","historyFrom":"2026-10-01T00:00:00Z","held":[]}`
		load := func(workers, extra string) (*Ledger, error) {
			l := `{"profile":"` + drainedState2Profile + `","program":"prog","launchSeq":3,"eventSeq":9,"workers":` + workers + `,"backoff":{},` + budget + extra + `}`
			if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(l), 0o600); err != nil {
				t.Fatal(err)
			}
			return LoadLedger(dir, "prog")
		}
		l, err := load(`[]`, ``)
		if err != nil || l.Profile != StateProfile || l.Budget == nil || l.Budget.HistoryFrom.Format(time.RFC3339) != "2026-10-01T00:00:00Z" || l.LaunchSeq != 3 || l.Stall != nil {
			t.Fatalf("drained /2 ledger: %+v %v", l, err)
		}
		for name, c := range map[string][2]string{
			"a recorded worker": {`[{"id":"w1"}]`, ``},
			"the stall counts":  {`[]`, `,"stall":{"ticket:a:q:t1":{"status":"OPEN","sessions":1}}`},
		} {
			if _, err := load(c[0], c[1]); wire.CodeOf(err) != wire.CodeUnsupportedVersion {
				t.Errorf("%s: %v", name, err)
			}
		}
	})
}
