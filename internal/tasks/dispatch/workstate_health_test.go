//go:build darwin || linux

package dispatch

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// healthRig is a dispatcher whose work-state command is /bin/cat of a file
// the test removes to make the command fail and writes to recover it.
func healthRig(t *testing.T, script string, ws func(*WorkState)) (*Dispatcher, *fakeQueue, string) {
	t.Helper()
	c := testConfig(t, script)
	source := filepath.Join(c.WorkRoot, "states.json")
	c.WorkState = &WorkState{Kind: "command", Argv: []string{"/bin/cat", source}}
	if ws != nil {
		ws(c.WorkState)
	}
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d, q, source
}

func writeStates(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(`{"t1":"work"}`), 0o600); err != nil {
		t.Fatal(err)
	}
}

func workStateEvents(t *testing.T, d *Dispatcher, kind, health string) []Event {
	t.Helper()
	var out []Event
	for _, e := range eventsOf(t, d, kind) {
		if e.Detail["provider"] == "workState" && (health == "" || e.Detail["health"] == health) {
			out = append(out, e)
		}
	}
	return out
}

// TestCALV0270_FailingWorkStateCommandHealthEscalatesOnceAndRecovers is the
// issue 984 fixture: a failing work-state command is recorded in the ledger
// after one tick, escalates once after the default three failed ticks, and
// emits one recovery event when a read succeeds; the next episode escalates
// again.
func TestCALV0270_FailingWorkStateCommandHealthEscalatesOnceAndRecovers(t *testing.T) {
	t.Run("CAL-V0-270", func(t *testing.T) {
		d, _, source := healthRig(t, "exit 0", func(w *WorkState) {})
		d.Config.Roles = nil // health only; no worker launches
		ctx := context.Background()
		writeStates(t, source)
		if err := d.Tick(ctx); err != nil {
			t.Fatal(err)
		}
		h := d.ledger.WorkState
		if h == nil || !h.OK || h.FailedTicks != 0 || h.LastError != "" || h.Since.IsZero() {
			t.Fatalf("healthy read = %+v", h)
		}
		if err := os.Remove(source); err != nil {
			t.Fatal(err)
		}
		if err := d.Tick(ctx); err != nil {
			t.Fatal(err)
		}
		// The health is in the saved ledger, which dispatch status reads,
		// after the first failed tick.
		l, err := LoadLedger(d.dir, "prog")
		if err != nil {
			t.Fatal(err)
		}
		h = l.WorkState
		if h == nil || h.OK || h.FailedTicks != 1 || !strings.Contains(h.LastError, "work state command") || h.Escalated {
			t.Fatalf("first failed tick = %+v", h)
		}
		since := h.Since
		if n := len(workStateEvents(t, d, "needs-owner", "")); n != 0 {
			t.Fatalf("escalated after one failed tick: %d", n)
		}
		for i := 0; i < 4; i++ {
			if err := d.Tick(ctx); err != nil {
				t.Fatal(err)
			}
		}
		h = d.ledger.WorkState
		if h.FailedTicks != 5 || !h.Escalated || !h.Since.Equal(since) {
			t.Fatalf("five failed ticks = %+v", h)
		}
		esc := workStateEvents(t, d, "needs-owner", "FAILING")
		if len(esc) != 1 || esc[0].Detail["failedTicks"] != "3" {
			t.Fatalf("escalations %+v", esc)
		}
		if n := len(eventsOf(t, d, "alert")); n < 5 {
			t.Fatalf("per-tick alerts %d", n)
		}
		writeStates(t, source)
		for i := 0; i < 2; i++ {
			if err := d.Tick(ctx); err != nil {
				t.Fatal(err)
			}
		}
		h = d.ledger.WorkState
		if !h.OK || h.FailedTicks != 0 || h.Escalated || h.LastError == "" || h.Since.Before(since) {
			t.Fatalf("recovered = %+v", h)
		}
		rec := workStateEvents(t, d, "alert", "RECOVERED")
		if len(rec) != 1 || rec[0].Detail["failedTicks"] != "5" || rec[0].Detail["escalated"] != "true" {
			t.Fatalf("recoveries %+v", rec)
		}
		// A new episode escalates again, once.
		if err := os.Remove(source); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 3; i++ {
			if err := d.Tick(ctx); err != nil {
				t.Fatal(err)
			}
		}
		if n := len(workStateEvents(t, d, "needs-owner", "FAILING")); n != 2 {
			t.Fatalf("second episode escalations %d", n)
		}
	})
	t.Run("CAL-V0-271", func(t *testing.T) {
		d, _, _ := healthRig(t, "exit 0", func(w *WorkState) { w.EscalateAfterTicks = 1 })
		d.Config.Roles = nil
		for i := 0; i < 3; i++ {
			if err := d.Tick(context.Background()); err != nil {
				t.Fatal(err)
			}
		}
		if esc := workStateEvents(t, d, "needs-owner", "FAILING"); len(esc) != 1 || esc[0].Detail["failedTicks"] != "1" {
			t.Fatalf("escalateAfterTicks 1: %+v", esc)
		}
		if d.ledger.WorkState.FailedTicks != 3 {
			t.Fatalf("failed ticks %+v", d.ledger.WorkState)
		}
	})
	t.Run("no reader keeps no health", func(t *testing.T) {
		d, _, _ := healthRig(t, "exit 0", nil)
		d.Config.Roles, d.Config.WorkState = nil, nil
		d.ledger.WorkState = &WorkStateHealth{FailedTicks: 2, Since: d.Now(), LastError: "x"}
		if err := d.Tick(context.Background()); err != nil || d.ledger.WorkState != nil {
			t.Fatalf("health kept without a reader: %+v %v", d.ledger.WorkState, err)
		}
	})
}

// TestCALV0272_UnknownWorkStateRunsAreRecorded: a run without progress
// whose work state was UNKNOWN during the run, or at its finish, is
// charged as CAL-V0-057 requires and its finished and cooldown events
// record workState UNKNOWN; a run with a readable state carries no mark.
func TestCALV0272_UnknownWorkStateRunsAreRecorded(t *testing.T) {
	ctx := context.Background()
	for _, name := range []string{"unknown during the run", "unknown at the finish", "known throughout"} {
		t.Run(name, func(t *testing.T) {
			d, _, source := healthRig(t, `while [ ! -e release ]; do sleep 0.02; done`, nil)
			writeStates(t, source)
			if err := d.Tick(ctx); err != nil || d.Running() != 1 {
				t.Fatalf("launch: %v running %d", err, d.Running())
			}
			w := d.ledger.Workers[0]
			if w.WorkStateUnknown {
				t.Fatal("a readable launch was marked")
			}
			if name != "known throughout" {
				if err := os.Remove(source); err != nil {
					t.Fatal(err)
				}
				if err := d.Tick(ctx); err != nil || d.Running() != 1 || !d.ledger.Workers[0].WorkStateUnknown {
					t.Fatalf("mid-run failure: %v running %d", err, d.Running())
				}
				if name == "unknown during the run" {
					writeStates(t, source)
				}
			}
			if err := os.WriteFile(filepath.Join(d.Config.WorkRoot, "release"), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			waitEnded(t, d)
			if err := d.Tick(ctx); err != nil {
				t.Fatal(err)
			}
			b := d.ledger.Backoff["ticket:a:q:t1"]
			if b == nil || b.NoProgress != 1 {
				t.Fatalf("run not charged as CAL-V0-057 requires: %+v", b)
			}
			want := StateUnknown
			if name == "known throughout" {
				want = ""
			}
			fin, cool := eventsOf(t, d, "finished"), eventsOf(t, d, "cooldown")
			if len(fin) != 1 || fin[0].Detail["workState"] != want || len(cool) != 1 || cool[0].Detail["workState"] != want {
				t.Fatalf("finished %+v cooldown %+v", fin, cool)
			}
			if want != "" && !strings.Contains(cool[0].Message, "work state was UNKNOWN") {
				t.Fatalf("cooldown message %q", cool[0].Message)
			}
		})
	}
}

// TestCALV0273_LaunchHoldIsOptInAndLifts: with holdLaunchesAfterTicks 2 the
// dispatcher launches nothing from the second consecutive failed tick, emits
// one throttled event, and launches again after a successful read; without
// the policy failed reads never hold launches.
func TestCALV0273_LaunchHoldIsOptInAndLifts(t *testing.T) {
	ctx := context.Background()
	t.Run("held", func(t *testing.T) {
		d, _, source := healthRig(t, "exit 0", func(w *WorkState) { w.HoldLaunchesAfterTicks = 2 })
		if err := d.Tick(ctx); err != nil || d.Running() != 1 {
			t.Fatalf("first failed tick launches: %v running %d", err, d.Running())
		}
		waitEnded(t, d)
		for i := 0; i < 3; i++ {
			if err := d.Tick(ctx); err != nil || d.Running() != 0 {
				t.Fatalf("held tick %d: %v running %d", i, err, d.Running())
			}
			if !WorkStateHoldsLaunches(d.Config, d.ledger.WorkState) {
				t.Fatalf("hold not reported: %+v", d.ledger.WorkState)
			}
		}
		if n := len(workStateEvents(t, d, "throttled", "")); n != 1 {
			t.Fatalf("throttled events %d", n)
		}
		writeStates(t, source)
		if err := d.Tick(ctx); err != nil || d.Running() != 1 {
			t.Fatalf("recovered tick: %v running %d", err, d.Running())
		}
	})
	t.Run("enabled mid-outage", func(t *testing.T) {
		d, _, _ := healthRig(t, "exit 0", nil)
		d.Config.Backoff.ParkAfter = 100
		for i := 0; i < 3; i++ {
			if err := d.Tick(ctx); err != nil {
				t.Fatal(err)
			}
			waitEnded(t, d)
		}
		// A reload that sets the hold below the episode's count holds at
		// once and still announces it once.
		d.Config.WorkState.HoldLaunchesAfterTicks = 2
		for i := 0; i < 2; i++ {
			if err := d.Tick(ctx); err != nil || d.Running() != 0 {
				t.Fatalf("held tick %d: %v running %d", i, err, d.Running())
			}
		}
		if n := len(workStateEvents(t, d, "throttled", "")); n != 1 {
			t.Fatalf("throttled events %d", n)
		}
	})
	t.Run("default off", func(t *testing.T) {
		d, _, _ := healthRig(t, "exit 0", nil)
		d.Config.Backoff.ParkAfter = 100
		for i := 0; i < 4; i++ {
			if err := d.Tick(ctx); err != nil || d.Running() != 1 {
				t.Fatalf("tick %d: %v running %d", i, err, d.Running())
			}
			waitEnded(t, d)
		}
		if WorkStateHoldsLaunches(d.Config, d.ledger.WorkState) || len(workStateEvents(t, d, "throttled", "")) != 0 {
			t.Fatal("launches held without the policy")
		}
	})
}

// TestCALV0270_ConfigAndLedgerBounds: thresholds outside 1..1440 refuse,
// an inconsistent health record refuses at load, and a version 4 ledger,
// drained or with running workers, is adopted while one carrying a version
// 5 member refuses.
func TestCALV0270_ConfigAndLedgerBounds(t *testing.T) {
	for name, f := range map[string]func(*WorkState){
		"negative escalate": func(w *WorkState) { w.EscalateAfterTicks = -1 },
		"large escalate":    func(w *WorkState) { w.EscalateAfterTicks = 1441 },
		"negative hold":     func(w *WorkState) { w.HoldLaunchesAfterTicks = -1 },
		"large hold":        func(w *WorkState) { w.HoldLaunchesAfterTicks = 1441 },
		"negative timeout":  func(w *WorkState) { w.TimeoutSeconds = -1 },
		"large timeout":     func(w *WorkState) { w.TimeoutSeconds = 601 },
		"negative stale":    func(w *WorkState) { w.StaleAfterSeconds = -1 },
		"large stale":       func(w *WorkState) { w.StaleAfterSeconds = 86401 },
	} {
		c := testConfig(t, "exit 0")
		c.WorkState = &WorkState{Kind: "command", Argv: []string{"/bin/cat", "/x"}}
		f(c.WorkState)
		raw, _ := json.Marshal(c)
		if _, err := DecodeConfig(raw); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	c := testConfig(t, "exit 0")
	c.WorkState = &WorkState{Kind: "command", Argv: []string{"/bin/cat", "/x"}, EscalateAfterTicks: 1440, HoldLaunchesAfterTicks: 1, TimeoutSeconds: 600, StaleAfterSeconds: 86400}
	raw, _ := json.Marshal(c)
	if got, err := DecodeConfig(raw); err != nil || got.WorkState.EscalateAfter() != 1440 || got.WorkState.Timeout() != 600*time.Second || got.WorkState.StaleAfter(1) != 86400*time.Second {
		t.Fatalf("bounds refused: %v", err)
	}
	if (&WorkState{}).StaleAfter(7) != 7*time.Second {
		t.Fatal("default window is not one tick")
	}
	if (&WorkState{}).EscalateAfter() != defaultWorkStateEscalateAfter {
		t.Fatal("default threshold")
	}

	dir := t.TempDir()
	load := func(profile, extra string) (*Ledger, error) {
		l := `{"profile":"` + profile + `","program":"prog","launchSeq":0,"eventSeq":0,"workers":[],"backoff":{}` + extra + `}`
		if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(l), 0o600); err != nil {
			t.Fatal(err)
		}
		return LoadLedger(dir, "prog")
	}
	if l, err := load(drainedState4Profile, ``); err != nil || l.Profile != StateProfile || l.WorkState != nil {
		t.Fatalf("drained /4 = %+v %v", l, err)
	}
	// As TOL-V0-021 adopted /3, a /4 ledger is adopted with its running
	// workers, which carry no CAL-V0-272 mark until a later read sets it.
	running := `{"profile":"` + drainedState4Profile + `","program":"prog","launchSeq":1,"eventSeq":0,"workers":[{"id":"w1","role":"dev","key":"ticket:a:q:t1","ticket":"t1","pid":4242,"leaderIdentity":"x","started":"2026-10-16T00:00:00Z"}],"backoff":{}}`
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(running), 0o600); err != nil {
		t.Fatal(err)
	}
	if l, err := LoadLedger(dir, "prog"); err != nil || l.Profile != StateProfile || len(l.Workers) != 1 || l.Workers[0].WorkStateUnknown {
		t.Fatalf("/4 with a running worker = %+v %v", l, err)
	}
	if _, err := load(drainedState4Profile, `,"workState":{"ok":true,"since":"2026-10-16T00:00:00Z","failedTicks":0}`); wire.CodeOf(err) != wire.CodeUnsupportedVersion {
		t.Fatalf("/4 carrying workState: %v", err)
	}
	if l, err := load(StateProfile, `,"workState":{"ok":false,"since":"2026-10-16T00:00:00Z","lastError":"e","failedTicks":4,"escalated":true}`); err != nil || l.WorkState.FailedTicks != 4 {
		t.Fatalf("valid health = %+v %v", l, err)
	}
	for name, h := range map[string]string{
		"zero since":         `{"ok":true,"since":"0001-01-01T00:00:00Z","failedTicks":0}`,
		"ok with failures":   `{"ok":true,"since":"2026-10-16T00:00:00Z","failedTicks":2}`,
		"failing at zero":    `{"ok":false,"since":"2026-10-16T00:00:00Z","failedTicks":0}`,
		"escalated while ok": `{"ok":true,"since":"2026-10-16T00:00:00Z","failedTicks":0,"escalated":true}`,
		"negative ticks":     `{"ok":false,"since":"2026-10-16T00:00:00Z","failedTicks":-1}`,
		"long error":         `{"ok":false,"since":"2026-10-16T00:00:00Z","failedTicks":1,"lastError":"` + strings.Repeat("e", maxWorkStateError+1) + `"}`,
	} {
		if _, err := load(StateProfile, `,"workState":`+h); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	// A ledger with progress (or any other strictly read member) takes the
	// member allowlist reader, which must admit both version 5 members and
	// still refuse an unknown health member.
	strict := `{"profile":"` + StateProfile + `","workers":[{"id":"w","workStateUnknown":true}],"progress":{},"workState":{"ok":false,"since":"2026-10-16T00:00:00Z","lastError":"e","failedTicks":1,"escalated":true}}`
	if !strictProgressJSON([]byte(strict)) {
		t.Fatal("strict reader refused the version 5 members")
	}
	if strictProgressJSON([]byte(strings.Replace(strict, `"escalated"`, `"extra"`, 1))) {
		t.Fatal("strict reader admitted an unknown health member")
	}
}
