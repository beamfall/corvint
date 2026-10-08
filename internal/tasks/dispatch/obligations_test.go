//go:build darwin || linux

package dispatch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ledgerTicket is a ticket that carries an obligation ledger view.
func ledgerTicket(witnessed, total int64, ar string, hw int64) Ticket {
	t := ticket("t1", "P1", 1)
	t.AcceptanceRevision = "1"
	t.Obligations = &ObligationsView{Witnessed: witnessed, Total: total, HighWaterRevision: ar, HighWater: hw}
	return t
}

// TestTOLV0017_FingerprintLegacyIdentity: a ticket without the member
// fingerprints byte for byte as before, its status, revision and work state
// line hashed alone.
func TestTOLV0017_FingerprintLegacyIdentity(t *testing.T) {
	t.Run("TOL-V0-017", func(t *testing.T) {
		tk := ticket("t1", "P1", 1)
		tk.State, tk.Revision, tk.AcceptanceRevision = "work", "7", "3"
		sum := sha256.Sum256([]byte("OPEN|7|work\n"))
		if got := baseFingerprint(&Observation{Tickets: []Ticket{tk}}, tk.ID); got != hex.EncodeToString(sum[:]) {
			t.Fatalf("legacy fingerprint changed: %s", got)
		}
		tk.Obligations = &ObligationsView{Total: 2, HighWaterRevision: "3"}
		sum = sha256.Sum256([]byte("OPEN|3|work\nobligations|3|0\n"))
		if got := baseFingerprint(&Observation{Tickets: []Ticket{tk}}, tk.ID); got != hex.EncodeToString(sum[:]) {
			t.Fatalf("ledger fingerprint lines differ: %s", got)
		}
	})
}

// TestTOLV0017_LedgerChurnIsNotProgress: revision-only writes (ledger, note
// or attachment) and a demote-then-rewitness that leaves the high water
// unchanged keep the fingerprint.
func TestTOLV0017_LedgerChurnIsNotProgress(t *testing.T) {
	t.Run("TOL-V0-017", func(t *testing.T) {
		base := ledgerTicket(2, 4, "1", 2)
		want := baseFingerprint(&Observation{Tickets: []Ticket{base}}, base.ID)
		churn := base
		churn.Revision = "9"
		churn.Obligations = &ObligationsView{Witnessed: 1, Total: 4, HighWaterRevision: "1", HighWater: 2}
		if got := baseFingerprint(&Observation{Tickets: []Ticket{churn}}, base.ID); got != want {
			t.Fatal("a revision-only write or a demotion counted as progress")
		}
	})
}

// TestTOLV0017_HighWaterRaiseIsProgress: a raised high water, or a new
// acceptance revision, changes the fingerprint; the native observation
// carries the counts to dispatch status as witnessed/total.
func TestTOLV0017_HighWaterRaiseIsProgress(t *testing.T) {
	t.Run("TOL-V0-017", func(t *testing.T) {
		base := ledgerTicket(2, 4, "1", 2)
		want := baseFingerprint(&Observation{Tickets: []Ticket{base}}, base.ID)
		raised := ledgerTicket(3, 4, "1", 3)
		if baseFingerprint(&Observation{Tickets: []Ticket{raised}}, base.ID) == want {
			t.Fatal("a high-water raise is not progress")
		}
		rebased := ledgerTicket(2, 4, "2", 2)
		rebased.AcceptanceRevision = "2"
		if baseFingerprint(&Observation{Tickets: []Ticket{rebased}}, base.ID) == want {
			t.Fatal("a new acceptance revision is not progress")
		}

		c := testConfig(t, "exit 0")
		q := &fakeQueue{obs: Observation{Tickets: []Ticket{base, ticket("t2", "P1", 2)}}}
		d, err := Open("prog", c, q, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = d.Close() })
		if err := d.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
		if got := d.ledger.Seen.Obligations; len(got) != 1 || got[base.ID] != "2/4" {
			t.Fatalf("seen obligations = %v", got)
		}
		waitEnded(t, d)
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
		l, err := LoadLedger(d.dir, "prog")
		if err != nil || l.Seen.Obligations[base.ID] != "2/4" {
			t.Fatalf("reloaded seen obligations = %+v %v", l, err)
		}
		path := filepath.Join(d.dir, "state.json")
		raw, _ := os.ReadFile(path)
		for _, bad := range []string{`"3/2"`, `"2/257"`, `"x/4"`, `"02/4"`} {
			if err := os.WriteFile(path, []byte(strings.Replace(string(raw), `"2/4"`, bad, 1)), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadLedger(d.dir, "prog"); err == nil {
				t.Errorf("seen obligations %s accepted", bad)
			}
		}
	})
}

// TestTOLV0021_StallRestartsOnRaise: a high-water rise within the same
// acceptance revision restarts the stall count at the session's finish,
// when observed mid-session, and when observed by the first tick after a
// restart; a new acceptance revision only rebases the baseline.
func TestTOLV0021_StallRestartsOnRaise(t *testing.T) {
	t.Run("TOL-V0-021", func(t *testing.T) {
		const key = "ticket:a:q:t1"
		c := testConfig(t, "exit 0")
		c.Backoff.ParkAfter = 100
		q := &fakeQueue{obs: Observation{Tickets: []Ticket{ledgerTicket(0, 3, "1", 0)}}}
		d, err := Open("prog", c, q, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = d.Close() })
		ob := func(w, hw int64, ar string) func() {
			return func() {
				q.obs.Tickets[0].Obligations = &ObligationsView{Witnessed: w, Total: 3, HighWaterRevision: ar, HighWater: hw}
			}
		}
		for i, want := range []string{"1", "2"} {
			if got := stallSession(t, d, nil)["sessionsSinceStatusChange"]; got != want {
				t.Fatalf("session %d = %s, want %s", i+1, got, want)
			}
		}
		if s := d.ledger.Stall[key]; s == nil || s.Witnessed == nil || *s.Witnessed != (StallWitnessed{AcceptanceRevision: "1", HighWater: 0}) {
			t.Fatalf("baseline = %+v", s)
		}
		// The session raises the high water: its finish reports zero.
		if got := stallSession(t, d, ob(1, 1, "1"))["sessionsSinceStatusChange"]; got != "0" {
			t.Fatalf("raising session = %s", got)
		}
		if s := d.ledger.Stall[key]; s.Sessions != 0 || s.Witnessed.HighWater != 1 {
			t.Fatalf("after the raise = %+v %+v", s, s.Witnessed)
		}
		// A demotion and a new acceptance revision are not a rise.
		if got := stallSession(t, d, ob(0, 1, "1"))["sessionsSinceStatusChange"]; got != "1" {
			t.Fatalf("demoting session = %s", got)
		}
		if got := stallSession(t, d, ob(0, 0, "2"))["sessionsSinceStatusChange"]; got != "2" {
			t.Fatalf("new acceptance revision session = %s", got)
		}
		if s := d.ledger.Stall[key]; *s.Witnessed != (StallWitnessed{AcceptanceRevision: "2", HighWater: 0}) {
			t.Fatalf("rebased baseline = %+v", s.Witnessed)
		}

		// Across a restart: the first tick that observes the rise with no
		// session running restarts the count.
		waitEnded(t, d)
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
		ob(1, 1, "2")()
		d2, err := Open("prog", c, q, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = d2.Close() })
		d2.ledger.Workers = nil // the previous session has ended
		d2.pruneStall(&q.obs)
		if s := d2.ledger.Stall[key]; s == nil || s.Sessions != 0 || s.Witnessed.HighWater != 1 {
			t.Fatalf("after restart = %+v", s)
		}
	})
	t.Run("TOL-V0-021 mid-session", func(t *testing.T) {
		gate := filepath.Join(t.TempDir(), "go")
		c := testConfig(t, "while [ ! -f '"+gate+"' ]; do sleep 0.05; done")
		c.StalledAfterSessions = intp(1)
		q := &fakeQueue{obs: Observation{Tickets: []Ticket{ledgerTicket(0, 3, "1", 0)}}}
		d, err := Open("prog", c, q, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.WriteFile(gate, nil, 0o600); _ = d.Close() })
		ctx := context.Background()
		if err := d.Tick(ctx); err != nil || d.Running() != 1 {
			t.Fatalf("launch: running %d %v", d.Running(), err)
		}
		q.obs.Tickets[0].Obligations = &ObligationsView{Witnessed: 1, Total: 3, HighWaterRevision: "1", HighWater: 1}
		if err := d.Tick(ctx); err != nil {
			t.Fatal(err)
		}
		if s := d.ledger.Stall[q.obs.Tickets[0].ID]; s == nil || !s.Changed {
			t.Fatalf("mid-session raise not kept: %+v", s)
		}
		if err := os.WriteFile(gate, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		waitEnded(t, d)
		if err := d.Tick(ctx); err != nil {
			t.Fatal(err)
		}
		if ev := eventsOf(t, d, "finished"); len(ev) != 1 || ev[0].Detail["sessionsSinceStatusChange"] != "0" {
			t.Fatalf("finished = %+v", ev)
		}
		if n := len(eventsOf(t, d, "stalled")); n != 0 {
			t.Fatalf("%d stalled events for a session that raised the high water", n)
		}
	})
}

// TestTOLV0021_PreviousLedgerVersionAdopted: a taskman-dispatch-state/3
// ledger, workers included, is adopted as this version with every stall
// baseline absent; the first observation seeds it without a restart. A /3
// ledger that carries a version 4 member refuses UNSUPPORTED_VERSION, and
// a malformed baseline refuses.
func TestTOLV0021_PreviousLedgerVersionAdopted(t *testing.T) {
	t.Run("TOL-V0-021", func(t *testing.T) {
		dir := t.TempDir()
		const key = "ticket:a:q:t1"
		budget := `"budget":{"sessions":[],"truncated":"0001-01-01T00:00:00Z","historyFrom":"2026-10-01T00:00:00Z","held":[]}`
		load := func(profile, stall, seen string) (*Ledger, error) {
			l := `{"profile":"` + profile + `","program":"prog","launchSeq":3,"eventSeq":9,"workers":[],"backoff":{},` + seen + budget + `,"stall":` + stall + `}`
			if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(l), 0o600); err != nil {
				t.Fatal(err)
			}
			return LoadLedger(dir, "prog")
		}
		l, err := load(drainedState3Profile, `{"`+key+`":{"status":"OPEN","sessions":2}}`, ``)
		if err != nil || l.Profile != StateProfile || l.Stall[key] == nil || l.Stall[key].Sessions != 2 || l.Stall[key].Witnessed != nil {
			t.Fatalf("adopted /3 = %+v %v", l, err)
		}
		d := &Dispatcher{ledger: l}
		tk := ledgerTicket(1, 3, "1", 1)
		d.pruneStall(&Observation{Tickets: []Ticket{tk}})
		if s := l.Stall[key]; s.Sessions != 2 || s.Witnessed == nil || s.Witnessed.HighWater != 1 {
			t.Fatalf("seeded baseline = %+v", s)
		}
		for name, c := range map[string][3]string{
			"a /4 baseline in /3":  {drainedState3Profile, `{"` + key + `":{"status":"OPEN","sessions":2,"witnessed":{"acceptanceRevision":"1","highWater":1}}}`, ``},
			"/4 seen counts in /3": {drainedState3Profile, `{"` + key + `":{"status":"OPEN","sessions":2}}`, `"seen":{"tickets":{},"claims":{},"lanes":{},"obligations":{"` + key + `":"1/3"}},`},
			"next version":         {"taskman-dispatch-state/5", `{"` + key + `":{"status":"OPEN","sessions":2}}`, ``},
		} {
			if _, err := load(c[0], c[1], c[2]); wire.CodeOf(err) != wire.CodeUnsupportedVersion {
				t.Errorf("%s: %v", name, err)
			}
		}
		for name, stall := range map[string]string{
			"negative high water": `{"` + key + `":{"status":"OPEN","sessions":2,"witnessed":{"acceptanceRevision":"1","highWater":-1}}}`,
			"bad revision":        `{"` + key + `":{"status":"OPEN","sessions":2,"witnessed":{"acceptanceRevision":"01","highWater":1}}}`,
			"missing high water":  `{"` + key + `":{"status":"OPEN","sessions":2,"witnessed":{"acceptanceRevision":"1"}}}`,
		} {
			if _, err := load(StateProfile, stall, ``); err == nil {
				t.Errorf("%s accepted", name)
			}
		}
		if l, err := load(StateProfile, `{"`+key+`":{"status":"OPEN","sessions":2,"witnessed":{"acceptanceRevision":"1","highWater":1}}}`, ``); err != nil || l.Stall[key].Witnessed.HighWater != 1 {
			t.Fatalf("current version baseline = %+v %v", l, err)
		}
	})
}
