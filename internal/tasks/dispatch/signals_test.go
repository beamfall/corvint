//go:build darwin || linux

package dispatch

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestDSGV0001_UnmatchedStateIsNoRoleMatch reproduces GitHub #1820: a
// suffixed state (X~2) no role admits read as WORK_STATE_HELD, the same as a
// deliberate hold. A state an enabled role excludes, or one workState
// holdStates declares, stays WORK_STATE_HELD; any other unadmitted known
// state reads NO_ROLE_MATCH. Both leave the window (CAL-V0-105 unchanged).
func TestDSGV0001_UnmatchedStateIsNoRoleMatch(t *testing.T) {
	c, ts := heldRig(t, map[string]string{"h1": "hold-lane", "h2": "X~2", "r1": "ready"})
	ts = append(ts, ticket("p1", "P0", 4))
	c.WorkState.HoldStates = []string{"parked"}
	writeState(t, c, "p1", "parked")
	c.Roles[0].Match = &Match{States: []string{"ready", "X"}, ExcludeStates: []string{"hold-lane"}, PlanSelected: true}
	var replans []map[string]bool
	q := windowQueue(ts, &replans)
	d, err := Open("held", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	obs, err := d.observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := planOf(obs)
	want := map[string]PlanView{"h1": {"DEFERRED", PlanReasonWorkStateHeld}, "h2": {"DEFERRED", PlanReasonNoRoleMatch}, "p1": {"DEFERRED", PlanReasonWorkStateHeld}, "r1": {"SELECTED", "DEVELOPMENT_MODE"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("plan %v, want %v", got, want)
	}
	if len(replans) != 1 || len(replans[0]) != 3 {
		t.Fatalf("every unadmitted state leaves the window: %v", replans)
	}
	if !reflect.DeepEqual(d.unmatched, map[string]bool{"ticket:a:q:h2": true}) {
		t.Fatalf("unmatched %v", d.unmatched)
	}
	// DSG-V0-002: the queue observation counts the unmatched state.
	if us := unmatchedStates(obs, d.unmatched); !reflect.DeepEqual(us, []UnmatchedState{{State: "X~2", Tickets: 1, Example: "ticket:a:q:h2"}}) {
		t.Fatalf("unmatched states %+v", us)
	}
	// A ticket no longer deferred NO_ROLE_MATCH (done, say) is not counted.
	obs.Tickets = append(obs.Tickets, Ticket{ID: "ticket:a:q:h3", State: "X~2", Status: "DONE"})
	d.unmatched["ticket:a:q:h3"] = true
	if us := unmatchedStates(obs, d.unmatched); len(us) != 1 || us[0].Tickets != 1 {
		t.Fatalf("a done ticket counted: %+v", us)
	}
}

func writeState(t *testing.T, c *Config, local, state string) {
	t.Helper()
	path := strings.ReplaceAll(c.WorkState.Path, "{ticketLocal}", local)
	if err := writeAtomic(path, []byte("state: "+state+"\n")); err != nil {
		t.Fatal(err)
	}
}

// TestDSGV0001_HoldStatesValidation: holdStates is bounded, distinct and
// names known states only; idleAlertMinutes is 0..1440 and never null.
func TestDSGV0001_HoldStatesValidation(t *testing.T) {
	for name, tc := range map[string]struct {
		set  func(*Config)
		want string
	}{
		"ok":        {func(c *Config) { c.WorkState.HoldStates = []string{"parked", "blocked"} }, ""},
		"empty":     {func(c *Config) { c.WorkState.HoldStates = []string{""} }, "holdStates"},
		"repeat":    {func(c *Config) { c.WorkState.HoldStates = []string{"a", "a"} }, "holdStates"},
		"unknown":   {func(c *Config) { c.WorkState.HoldStates = []string{StateUnknown} }, "holdStates"},
		"idle 0":    {func(c *Config) { n := 0; c.IdleAlertMinutes = &n }, ""},
		"idle high": {func(c *Config) { n := 1441; c.IdleAlertMinutes = &n }, "idleAlertMinutes"},
	} {
		t.Run(name, func(t *testing.T) {
			c, _ := heldRig(t, nil)
			tc.set(c)
			raw, _ := json.Marshal(c)
			_, err := DecodeConfig(raw)
			if (tc.want == "") != (err == nil) || err != nil && !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err %v, want %q", err, tc.want)
			}
		})
	}
	c, _ := heldRig(t, nil)
	raw, _ := json.Marshal(c)
	raw = []byte(strings.Replace(string(raw), `"tickSeconds"`, `"idleAlertMinutes":null,"tickSeconds"`, 1))
	if _, err := DecodeConfig(raw); err == nil || !strings.Contains(err.Error(), "idleAlertMinutes is null") {
		t.Fatalf("null idleAlertMinutes: %v", err)
	}
	if got := (&Config{}).IdleAlertAfter(); got != time.Hour {
		t.Fatalf("default %v", got)
	}
}

// TestDSGV0003_IdleWithCapacity reproduces the second half of GitHub #1820:
// ticks that launch nothing beside free capacity while planned tickets wait
// build an idle streak in the queue observation, and DSG-V0-004 alerts once
// per threshold with the deferral reasons. A launch, a full global cap or no
// planned work ends the streak.
func TestDSGV0003_IdleWithCapacity(t *testing.T) {
	c := runOnlyConfig(t, false)
	c.Roles[1].Match = &Match{PlanSelected: true}
	d := openAdmission(t, c)
	held := ticket("i1", "P1", 1)
	held.Plan, held.PlanReason = "DEFERRED", PlanReasonNoRoleMatch
	held2 := ticket("i2", "P1", 2)
	held2.Plan, held2.PlanReason = "DEFERRED", PlanReasonWorkStateHeld
	done := ticket("i3", "P1", 3)
	done.Status = "DONE"
	obs := &Observation{Tickets: []Ticket{held, held2, done}}
	t0 := time.Date(2026, 10, 25, 1, 0, 0, 0, time.UTC)
	tick := func(at time.Duration) *AdmissionRecord {
		t.Helper()
		_, adm := admRoster(c, obs, nil, true)
		d.recordAdmission(obs, adm, AdmissionPressureCalm, t0.Add(at))
		return readAdm(t, d)
	}
	alerts := func() []Event {
		events, err := ReadEvents(d.dir, 1000)
		if err != nil {
			t.Fatal(err)
		}
		var out []Event
		for _, e := range events {
			if e.Kind == "alert" && e.Detail["code"] == "IDLE_WITH_CAPACITY" {
				out = append(out, e)
			}
		}
		return out
	}
	rec := tick(0)
	if rec.IdleAlertSeconds != 3600 || rec.IdleTicks != 1 || !rec.IdleSince.Equal(t0) || !reflect.DeepEqual(rec.IdleDeferred, []DeferralCount{{PlanReasonNoRoleMatch, 1}, {PlanReasonWorkStateHeld, 1}}) {
		t.Fatalf("idle: %+v", rec)
	}
	tick(59 * time.Minute)
	if n := len(alerts()); n != 0 {
		t.Fatalf("alerted before the threshold: %d", n)
	}
	rec = tick(60 * time.Minute)
	if a := alerts(); len(a) != 1 || rec.IdleTicks != 3 || !strings.Contains(a[0].Message, "NO_ROLE_MATCH=1") || a[0].Detail["running"] != "0" {
		t.Fatalf("alert at threshold: %+v %+v", a, rec)
	}
	tick(90 * time.Minute)
	if n := len(alerts()); n != 1 {
		t.Fatalf("rate limit: %d", n)
	}
	tick(120 * time.Minute)
	if n := len(alerts()); n != 2 {
		t.Fatalf("second threshold: %d", n)
	}
	// A new worker (here a lane launch) ends the streak; one that keeps
	// running beside free capacity does not.
	d.ledger.Workers = []*Worker{{ID: "w8", Role: "impl", Key: "lane:y", Ticket: "other2"}}
	if rec = tick(120*time.Minute + time.Second); rec.IdleTicks != 0 {
		t.Fatalf("launch: %+v", rec)
	}
	if rec = tick(120*time.Minute + 2*time.Second); rec.IdleTicks != 1 {
		t.Fatalf("a running worker is no launch: %+v", rec)
	}
	// A full global cap is not idle capacity.
	c.GlobalCap = 1
	if rec = tick(121 * time.Minute); rec.IdleTicks != 0 || rec.IdleSince != nil {
		t.Fatalf("full cap: %+v", rec)
	}
	// Planned work that is running is not idle, nor is no planned work.
	c.GlobalCap, d.ledger.Workers = 6, nil
	obs.Tickets = []Ticket{done}
	if rec = tick(122 * time.Minute); rec.IdleTicks != 0 {
		t.Fatalf("no work: %+v", rec)
	}
	// Off: 0 records the streak but never alerts.
	off := 0
	c.IdleAlertMinutes = &off
	obs.Tickets = []Ticket{held}
	tick(123 * time.Minute)
	if rec = tick(300 * time.Minute); rec.IdleTicks != 2 || len(alerts()) != 2 {
		t.Fatalf("off: %+v %d", rec, len(alerts()))
	}
}

// TestDSGV0008_OpencodePluginDelivery reproduces GitHub #1821: an opencode
// role had only UNSUPPORTED or the polled file. A host that declares
// contextNotices "plugin" reads as HOOK delivery for opencode roles and is
// refused for any other usage format; the finished event records how far the
// session ran past HANDOFF (DSG-V0-009).
func TestDSGV0008_OpencodePluginDelivery(t *testing.T) {
	c := testConfig(t, "exit 0")
	h := c.Hosts["sh"]
	h.ContextNotices = ContextNoticesPlugin
	c.Hosts["sh"] = h
	for format, ok := range map[string]bool{"opencode": true, "claude-code": false, "": false} {
		c.Roles[0].UsageFormat = format
		raw, _ := json.Marshal(c)
		if _, err := DecodeConfig(raw); (err == nil) != ok {
			t.Errorf("usageFormat %q on a plugin host: err %v, want accepted %v", format, err, ok)
		}
	}
	dir := t.TempDir()
	if mode, _, ok := ContextDelivery(dir, "w1", "opencode", &h); !ok || mode != DeliveryHook {
		t.Fatalf("plugin mode %q %v", mode, ok)
	}
	if mode, _, _ := ContextDelivery(dir, "w1", "opencode", &Host{}); mode != DeliveryUnsupported {
		t.Fatalf("undeclared opencode host %q", mode)
	}
	rec := freshContextRecord("w1")
	rec.Tokens, rec.ObservedAt, rec.HandoffTokens, rec.Level = "185000", "2026-10-25T09:05:00Z", "170000", ContextHandoff
	rec.Crossings = []ContextCrossing{{Level: ContextHandoff, Threshold: "170000", Tokens: "171000", At: "2026-10-25T09:00:00Z"}}
	if err := os.MkdirAll(filepath.Join(dir, "workers", "w1"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeAtomic(ContextRecordPath(dir, "w1"), rec.encode()); err != nil {
		t.Fatal(err)
	}
	detail := map[string]string{}
	afterHandoff(dir, "w1", detail, time.Date(2026, 10, 25, 9, 7, 30, 0, time.UTC))
	if detail["tokensAfterHandoff"] != "14000" || detail["secondsAfterHandoff"] != "450" {
		t.Fatalf("after handoff %+v", detail)
	}
	detail = map[string]string{}
	afterHandoff(dir, "w2", detail, time.Now())
	if len(detail) != 0 {
		t.Fatalf("no record: %+v", detail)
	}
}

// TestDSGV0004_IdleGateWakesForTheAlert: the idle gate's skip window ends at
// an idle streak's next alert threshold, so skipped ticks never delay the
// DSG-V0-004 alert.
func TestDSGV0004_IdleGateWakesForTheAlert(t *testing.T) {
	c := testConfig(t, "exit 0")
	one := 1
	c.IdleAlertMinutes = &one
	t0 := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	clock := t0
	q := &witnessQueue{witness: "w1"}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	d.Now = func() time.Time { return clock }
	for i := 0; i < 2; i++ {
		if err := d.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(d.dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	mark := idleMark{witness: q.witness, ok: true, state: sha256.Sum256(raw), since: t0}
	clock = t0.Add(time.Second)
	d.idle = nil
	d.idleSettle(mark, nil)
	if d.idle == nil || !d.idle.due.After(t0.Add(time.Minute)) {
		t.Fatalf("no streak: gate %+v, want one past the alert threshold", d.idle)
	}
	d.idleRun = streak{since: t0, ticks: 1}
	d.idle = nil
	d.idleSettle(mark, nil)
	if d.idle == nil || !d.idle.due.Equal(t0.Add(time.Minute)) {
		t.Fatalf("streak: gate %+v, want one due at the alert threshold", d.idle)
	}
	d.idleAlerted = t0.Add(30 * time.Second)
	d.idle = nil
	d.idleSettle(mark, nil)
	if d.idle == nil || !d.idle.due.Equal(t0.Add(90*time.Second)) {
		t.Fatalf("after an alert: gate %+v, want one due a threshold later", d.idle)
	}
}
