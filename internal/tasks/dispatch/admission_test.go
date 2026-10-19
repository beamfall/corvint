//go:build darwin || linux

package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// runOnlyConfig has a run-only role over pool lanes and an author role.
func runOnlyConfig(t *testing.T, bypass bool) *Config {
	c := testConfig(t, "exit 0")
	c.GlobalCap = 6
	c.Roles = []Role{
		{Name: "runner", Host: "sh", Cap: 2, RunOnly: true, BypassFocus: bypass, Match: &Match{PlanSelected: true, Pool: "lanes"}, Prompt: "p {pool}", IdleSeconds: 30, WallSeconds: 60},
		{Name: "impl", Host: "sh", Cap: 2, Match: &Match{PlanSelected: true}, Prompt: "p", IdleSeconds: 30, WallSeconds: 60},
	}
	return c
}

// unselectedRun is a lanes ticket the plan window has not selected.
func unselectedRun(local string, order uint64) Ticket {
	t := ticket(local, "P1", order)
	t.RequiresPool, t.Plan, t.PlanReason = "lanes", "WAITING", "outside the plan window"
	return t
}

func admRoster(c *Config, obs *Observation, busy []Busy, calm bool) ([]Assignment, *admission) {
	adm := &admission{calm: calm}
	out, _ := roster(c, obs, busy, nil, nil, nil, nil, nil, nil, adm)
	return out, adm
}

func holdFences(adm *admission) map[string]string {
	out := map[string]string{}
	for _, h := range adm.holds {
		out[h.a.Role+"/"+h.a.Ticket] = h.fence
	}
	return out
}

// ATR-V0-151: a run-only ticket the plan window alone keeps out is held
// with PLAN_WINDOW unless the role opts into bypassFocus; with the bypass it
// launches while its pool has a free member, never without one, and a role
// that is not run-only (author-session work) neither bypasses nor is noted.
func TestATRV0151_RunOnlyBypassFocus(t *testing.T) {
	obs := &Observation{Tickets: []Ticket{unselectedRun("r1", 1)}, PoolFree: map[string]int{"lanes": 1}}
	out, adm := admRoster(runOnlyConfig(t, false), obs, nil, true)
	if len(out) != 0 || !reflect.DeepEqual(holdFences(adm), map[string]string{"runner/ticket:a:q:r1": FencePlanWindow}) {
		t.Fatalf("without bypass: launches %v holds %v", keysOf(out), holdFences(adm))
	}
	for _, h := range adm.holds {
		if h.detail != "outside the plan window" {
			t.Fatalf("plan reason not kept: %q", h.detail)
		}
	}
	out, adm = admRoster(runOnlyConfig(t, true), obs, nil, true)
	if !reflect.DeepEqual(keysOf(out), []string{"runner/r1/1"}) || len(adm.holds) != 0 {
		t.Fatalf("bypass: launches %v holds %v", keysOf(out), holdFences(adm))
	}
	// Without adm the bypass still applies (the pure Roster path).
	if out := Roster(runOnlyConfig(t, true), obs, nil, nil); !reflect.DeepEqual(keysOf(out), []string{"runner/r1/1"}) {
		t.Fatalf("bypass without adm: %v", keysOf(out))
	}
	// No free member: nothing launches; the hold is noted (the observation
	// drops it, ATR-V0-150).
	obs.PoolFree["lanes"] = 0
	if out, adm = admRoster(runOnlyConfig(t, true), obs, nil, true); len(out) != 0 || len(adm.holds) != 1 {
		t.Fatalf("no free member: launches %v holds %v", keysOf(out), holdFences(adm))
	}
	// Author-session work: the same ticket under a non-run-only pool role.
	c := runOnlyConfig(t, true)
	c.Roles[0].RunOnly, c.Roles[0].BypassFocus = false, false
	obs.PoolFree["lanes"] = 1
	if out, adm = admRoster(c, obs, nil, true); len(out) != 0 || len(adm.holds) != 0 {
		t.Fatalf("author role: launches %v holds %v", keysOf(out), holdFences(adm))
	}
	// A pending detached relaunch keeps its role among bypassing roles.
	c = runOnlyConfig(t, true)
	second := c.Roles[0]
	second.Name = "runner2"
	c.Roles = append(c.Roles, second)
	prefer := map[string]string{"ticket:a:q:r1": "runner2"}
	if out, _ := roster(c, obs, nil, nil, nil, nil, prefer, nil, nil, &admission{calm: true}); !reflect.DeepEqual(keysOf(out), []string{"runner2/r1/1"}) {
		t.Fatalf("relaunch role: launches %v", keysOf(out))
	}
	// A ticket another match member excludes is not a plan-window hold.
	other := unselectedRun("r2", 2)
	other.RequiresPool = "other"
	obs.Tickets = []Ticket{other}
	if out, adm = admRoster(runOnlyConfig(t, false), obs, nil, true); len(out) != 0 || len(adm.holds) != 0 {
		t.Fatalf("unmatched ticket: launches %v holds %v", keysOf(out), holdFences(adm))
	}
}

// ATR-V0-150: each admission fence that keeps a run-only candidate out is
// named, the first one per ticket; a launched ticket leaves no hold.
func TestATRV0150_RosterNamesFences(t *testing.T) {
	c := runOnlyConfig(t, false)
	c.Roles[0].Cap = 1
	sel := func(local string, order uint64) Ticket {
		x := unselectedRun(local, order)
		x.Plan = "SELECTED"
		return x
	}
	// i1..i3 are author-session work for impl: its fences are never noted.
	authored := func(local string, order uint64) Ticket {
		x := ticket(local, "P1", order)
		x.Plan = "SELECTED"
		return x
	}
	obs := &Observation{Tickets: []Ticket{sel("a", 1), sel("b", 2), authored("i1", 3), authored("i2", 4), authored("i3", 5)}, PoolFree: map[string]int{"lanes": 2}}
	out, adm := admRoster(c, obs, nil, true)
	if len(out) != 3 || out[0].Ticket != "ticket:a:q:a" || !reflect.DeepEqual(holdFences(adm), map[string]string{"runner/ticket:a:q:b": FenceRoleCap}) {
		t.Fatalf("role cap: launches %v holds %v", keysOf(out), holdFences(adm))
	}
	c.GlobalCap = 1
	c.Roles[0].Cap = 2
	if out, adm = admRoster(c, obs, nil, true); len(out) != 1 || !reflect.DeepEqual(holdFences(adm), map[string]string{"runner/ticket:a:q:b": FenceGlobalCap}) {
		t.Fatalf("global cap: launches %v holds %v", keysOf(out), holdFences(adm))
	}
	c.GlobalCap = 6
	budget := &PressureBudget{enabled: true}
	adm = &admission{calm: false}
	if out, _ = roster(c, obs, nil, nil, nil, budget, nil, nil, nil, adm); len(out) != 0 || !reflect.DeepEqual(holdFences(adm), map[string]string{"runner/ticket:a:q:a": FencePressure, "runner/ticket:a:q:b": FencePressure}) {
		t.Fatalf("pressure: launches %v holds %v", keysOf(out), holdFences(adm))
	}
}

// ATR-V0-152: a capMax role's effective cap rises toward capMax, never
// above the queue depth, only while pressure is calm and the queue is deeper
// than cap, and falls back to cap when the queue drains.
func TestATRV0152_CapMaxScalesAtCalmPressure(t *testing.T) {
	c := testConfig(t, "exit 0")
	c.GlobalCap = 8
	c.Roles[0].Cap, c.Roles[0].CapMax = 1, 3
	obs := &Observation{Tickets: []Ticket{ticket("t1", "P1", 1), ticket("t2", "P1", 2), ticket("t3", "P1", 3), ticket("t4", "P1", 4)}}
	out, adm := admRoster(c, obs, nil, true)
	if len(out) != 3 || adm.eff["impl"] != 3 || !reflect.DeepEqual(adm.waiting["impl"], []string{"ticket:a:q:t4"}) {
		t.Fatalf("calm deep: launches %v eff %d waiting %v", keysOf(out), adm.eff["impl"], adm.waiting)
	}
	if out, adm = admRoster(c, obs, nil, false); len(out) != 1 || adm.eff["impl"] != 1 {
		t.Fatalf("not calm: launches %v eff %d", keysOf(out), adm.eff["impl"])
	}
	if out := Roster(c, obs, nil, nil); len(out) != 1 {
		t.Fatalf("no pressure input: launches %v", keysOf(out))
	}
	// Two waiting: the cap rises only to the depth.
	obs.Tickets = obs.Tickets[:2]
	if out, adm = admRoster(c, obs, nil, true); len(out) != 2 || adm.eff["impl"] != 2 {
		t.Fatalf("depth bound: launches %v eff %d", keysOf(out), adm.eff["impl"])
	}
	// Drained: one running session and nothing waiting is back at cap.
	obs.Tickets = nil
	busy := []Busy{{Role: "impl", Key: "ticket:a:q:t1"}}
	if out, adm = admRoster(c, obs, busy, true); len(out) != 0 || adm.eff["impl"] != 1 {
		t.Fatalf("drained: launches %v eff %d", keysOf(out), adm.eff["impl"])
	}
}

// ATR-V0-151/152: the configuration refuses an unsupported combination.
func TestATRV0151_ConfigValidation(t *testing.T) {
	for name, tc := range map[string]struct {
		edit func(*Role)
		want string
	}{
		"runOnly without pool":   {func(r *Role) { r.RunOnly = true }, "runOnly needs match.pool"},
		"bypass without runOnly": {func(r *Role) { r.Match.Pool = "lanes"; r.BypassFocus = true }, "bypassFocus needs runOnly"},
		"capMax not above cap":   {func(r *Role) { r.CapMax = 2 }, "capMax needs"},
		"capMax above 64":        {func(r *Role) { r.CapMax = 65 }, "capMax needs"},
	} {
		c := testConfig(t, "exit 0")
		tc.edit(&c.Roles[0])
		if err := c.validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	c := testConfig(t, "exit 0")
	c.Roles[0].Match.Pool, c.Roles[0].RunOnly, c.Roles[0].BypassFocus, c.Roles[0].CapMax = "lanes", true, true, 3
	if err := c.validate(); err != nil {
		t.Fatalf("valid run-only capMax role refused: %v", err)
	}
}

func openAdmission(t *testing.T, c *Config) *Dispatcher {
	t.Helper()
	d, err := Open("prog", c, &fakeQueue{}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func readAdm(t *testing.T, d *Dispatcher) *AdmissionRecord {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(d.dir, AdmissionFile))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	rec, err := DecodeAdmission(raw, "prog")
	if err != nil || rec == nil {
		t.Fatalf("read: %v %v", rec, err)
	}
	return rec
}

// ATR-V0-150: the observation counts consecutive held ticks only beside a
// free member, keeps their first tick, names the free members, restarts
// when a tick launches nothing at all, and counts a deep queue only at CALM.
func TestATRV0150_ObservationStreaks(t *testing.T) {
	c := runOnlyConfig(t, false)
	c.Roles[1].Cap, c.Roles[1].Match = 1, &Match{}
	d := openAdmission(t, c)
	d.ledger.Workers = []*Worker{{ID: "w1", Role: "impl", Key: "ticket:a:q:i1"}}
	obs := &Observation{Tickets: []Ticket{unselectedRun("r1", 1), ticket("i1", "P1", 2), ticket("i2", "P1", 3)}, PoolFree: map[string]int{"lanes": 2}, PoolFreeMembers: map[string][]string{"lanes": {"m1", "m2"}}}
	t0 := time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC)
	tick := func(i int, pressure string) *AdmissionRecord {
		t.Helper()
		_, adm := admRoster(c, obs, []Busy{{Role: "impl", Key: "ticket:a:q:i1"}}, pressure == AdmissionPressureCalm)
		d.recordAdmission(obs, adm, pressure, t0.Add(time.Duration(i)*time.Second))
		return readAdm(t, d)
	}
	tick(0, AdmissionPressureCalm)
	rec := tick(1, AdmissionPressureCalm)
	if len(rec.Held) != 1 || rec.Held[0].Ticks != 2 || !rec.Held[0].Since.Equal(t0) || rec.Held[0].Fence != FencePlanWindow || rec.Held[0].Detail != "outside the plan window" || !reflect.DeepEqual(rec.Held[0].FreeMembers, []string{"m1", "m2"}) || rec.Held[0].Free != 2 {
		t.Fatalf("held: %+v", rec.Held)
	}
	if len(rec.Roles) != 2 || rec.Roles[1].Role != "impl" || rec.Roles[1].DeepTicks != 2 || !rec.Roles[1].DeepSince.Equal(t0) || rec.Roles[1].Running != 1 || rec.Roles[1].Waiting != 1 || rec.Roles[1].OldestWaitTicket != "ticket:a:q:i2" || !rec.Roles[1].OldestWaitSince.Equal(t0) || rec.Pressure != AdmissionPressureCalm || rec.GlobalCap != 6 {
		t.Fatalf("roles: %+v", rec.Roles)
	}
	// Raised pressure ends the deep streak, not the hold.
	if rec = tick(2, AdmissionPressureRaised); rec.Roles[1].DeepTicks != 0 || rec.Roles[1].DeepSince != nil || rec.Held[0].Ticks != 3 {
		t.Fatalf("raised: %+v %+v", rec.Roles, rec.Held)
	}
	if rec = tick(3, AdmissionPressureUnknown); rec.Roles[1].DeepTicks != 0 {
		t.Fatalf("unknown: %+v", rec.Roles)
	}
	// No free member: the hold is not reported and its streak restarts.
	obs.PoolFree["lanes"] = 0
	if rec = tick(4, AdmissionPressureCalm); len(rec.Held) != 0 || rec.Roles[1].DeepTicks != 1 {
		t.Fatalf("no free member: %+v %+v", rec.Held, rec.Roles)
	}
	obs.PoolFree["lanes"] = 2
	if rec = tick(5, AdmissionPressureCalm); rec.Held[0].Ticks != 1 || !rec.Held[0].Since.Equal(t0.Add(5*time.Second)) {
		t.Fatalf("restarted hold: %+v", rec.Held)
	}
	// A tick whose launches are held restarts every streak.
	d.recordAdmission(obs, nil, AdmissionPressureCalm, t0.Add(6*time.Second))
	if rec = readAdm(t, d); !rec.LaunchesHeld || len(rec.Held) != 0 || len(rec.Roles) != 0 {
		t.Fatalf("launches held: %+v", rec)
	}
	if rec = tick(7, AdmissionPressureCalm); rec.Held[0].Ticks != 1 || rec.Roles[1].DeepTicks != 1 || !rec.Roles[1].OldestWaitSince.Equal(t0.Add(7*time.Second)) {
		t.Fatalf("after held tick: %+v %+v", rec.Held, rec.Roles)
	}
}

// ATR-V0-150: the observation counts the workers recorded after the launch
// loop, so a planned launch that did not become a worker is still waiting.
func TestATRV0150_RefusedLaunchStaysWaiting(t *testing.T) {
	c := runOnlyConfig(t, false)
	c.Roles[1].Match = &Match{}
	d := openAdmission(t, c)
	obs := &Observation{Tickets: []Ticket{ticket("i1", "P1", 1), ticket("i2", "P1", 2), ticket("i3", "P1", 3)}}
	out, adm := admRoster(c, obs, nil, true)
	if len(out) != 2 {
		t.Fatalf("planned: %+v", out)
	}
	d.ledger.Workers = []*Worker{{ID: "w1", Role: "impl", Key: out[0].Key}}
	d.recordAdmission(obs, adm, AdmissionPressureCalm, time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC))
	rec := readAdm(t, d)
	if rec.Running != 1 || rec.Roles[1].Running != 1 || rec.Roles[1].Waiting != 2 || rec.Roles[1].OldestWaitTicket != out[1].Ticket || rec.Roles[1].DeepTicks != 1 {
		t.Fatalf("record: running %d roles %+v", rec.Running, rec.Roles)
	}
}

// ATR-V0-150: an observation whose holds would exceed the 1 MiB bound keeps
// the longest-held half repeatedly until it fits and sets heldTruncated.
func TestATRV0150_OversizeHeldIsTruncated(t *testing.T) {
	c := runOnlyConfig(t, false)
	d := openAdmission(t, c)
	names := make([]string, maxAdmissionFreeNames)
	for i := range names {
		names[i] = fmt.Sprintf("member-%03d-%s", i, strings.Repeat("x", 100))
	}
	obs := &Observation{PoolFree: map[string]int{"lanes": len(names)}, PoolFreeMembers: map[string][]string{"lanes": names}}
	adm := &admission{calm: true, holds: map[string]admissionHold{}}
	for i := 0; i < 200; i++ {
		a := Assignment{Role: "runner", Ticket: fmt.Sprintf("ticket:a:q:r%03d", i), Key: fmt.Sprintf("ticket:a:q:r%03d", i), Pool: "lanes"}
		adm.holds["runner/"+a.Key] = admissionHold{a: a, fence: FencePlanWindow, detail: "outside the plan window"}
	}
	d.recordAdmission(obs, adm, AdmissionPressureCalm, time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC))
	raw, err := os.ReadFile(filepath.Join(d.dir, AdmissionFile))
	if err != nil {
		t.Fatal(err)
	}
	rec := readAdm(t, d)
	if len(raw) > MaxAdmissionBytes || !rec.HeldTruncated || len(rec.Held) == 0 || len(rec.Held) >= 200 || d.admissionFailed {
		t.Fatalf("size %d truncated %v held %d failed %v", len(raw), rec.HeldTruncated, len(rec.Held), d.admissionFailed)
	}
}

// ATR-V0-150: a failed write alerts once until a write succeeds again and
// never stops the tick.
func TestATRV0150_WriteFailureAlertsOnce(t *testing.T) {
	d := openAdmission(t, runOnlyConfig(t, false))
	path := filepath.Join(d.dir, AdmissionFile)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "x"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	alerts := func() int {
		events, err := ReadEvents(d.dir, 1000)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, e := range events {
			if e.Kind == "alert" && strings.Contains(e.Message, "queue observation not written") {
				n++
			}
		}
		return n
	}
	now := time.Now()
	d.recordAdmission(&Observation{}, nil, AdmissionPressureCalm, now)
	d.recordAdmission(&Observation{}, nil, AdmissionPressureCalm, now)
	if alerts() != 1 || !d.admissionFailed {
		t.Fatalf("alerts %d failed %v", alerts(), d.admissionFailed)
	}
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
	d.recordAdmission(&Observation{}, nil, AdmissionPressureCalm, now)
	if d.admissionFailed || readAdm(t, d) == nil {
		t.Fatal("recovered write")
	}
}

// ATR-V0-150: the pressure label is NOT_CONFIGURED without a pressure
// policy or record, UNKNOWN on an unknown state, CALM only at level 0 with
// nothing pending, and RAISED otherwise.
func TestATRV0150_PressureLabel(t *testing.T) {
	c := runOnlyConfig(t, false)
	d := openAdmission(t, c)
	if got := d.pressureLabel(); got != AdmissionPressureNotConfigured {
		t.Fatalf("no policy: %s", got)
	}
	p := issue497Config()
	d.Config.Pressure = &p
	d.ledger.Pressure = nil
	if got := d.pressureLabel(); got != AdmissionPressureNotConfigured {
		t.Fatalf("no record: %s", got)
	}
	d.ledger.Pressure = &PressureRecord{State: PressureState{Unknown: true}}
	if got := d.pressureLabel(); got != AdmissionPressureUnknown {
		t.Fatalf("unknown: %s", got)
	}
	d.ledger.Pressure.State = PressureState{}
	if got := d.pressureLabel(); got != AdmissionPressureCalm {
		t.Fatalf("calm: %s", got)
	}
	d.ledger.Pressure.State = PressureState{PendingLevel: 1}
	if got := d.pressureLabel(); got != AdmissionPressureRaised {
		t.Fatalf("pending: %s", got)
	}
}

// ATR-V0-150: a dispatcher tick writes the observation; a held run-only
// ticket beside a free member is counted across ticks.
func TestATRV0150_TickWritesObservation(t *testing.T) {
	c := runOnlyConfig(t, false)
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{unselectedRun("r1", 1)}, PoolFree: map[string]int{"lanes": 1}, PoolFreeMembers: map[string][]string{"lanes": {"m1"}}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	for i := 0; i < 2; i++ {
		if err := d.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	rec := readAdm(t, d)
	if rec.Pressure != AdmissionPressureNotConfigured || len(rec.Held) != 1 || rec.Held[0].Ticks != 2 || rec.Held[0].Ticket != "ticket:a:q:r1" || d.Running() != 0 {
		t.Fatalf("observation: %+v", rec)
	}
}

// ATR-V0-150: the reader is strict and bounded.
func TestATRV0150_DecodeAdmissionRefusals(t *testing.T) {
	at := time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC)
	ok := AdmissionRecord{Profile: AdmissionProfile, Program: "prog", At: at, TickSeconds: 5, Pressure: AdmissionPressureCalm, GlobalCap: 4, Roles: []RoleQueue{}, Held: []HeldRunner{{Role: "runner", Ticket: "t", Pool: "lanes", Fence: FencePlanWindow, Free: 1, FreeMembers: []string{"m1"}, Ticks: 1, Since: at}}}
	raw, _ := json.Marshal(ok)
	if _, err := DecodeAdmission(raw, "prog"); err != nil {
		t.Fatalf("valid refused: %v", err)
	}
	code := func(raw []byte, program string) string {
		_, err := DecodeAdmission(raw, program)
		var we *wire.Error
		if !errors.As(err, &we) {
			return "none"
		}
		return we.Code
	}
	edit := func(f func(*AdmissionRecord)) []byte {
		r := ok
		r.Held = append([]HeldRunner{}, ok.Held...)
		f(&r)
		raw, _ := json.Marshal(r)
		return raw
	}
	for name, tc := range map[string]struct {
		raw     []byte
		program string
		want    string
	}{
		"profile":       {edit(func(r *AdmissionRecord) { r.Profile = "taskman-dispatch-admission/1" }), "prog", wire.CodeUnsupportedVersion},
		"program":       {raw, "other", wire.CodeMalformed},
		"unknown field": {[]byte(strings.Replace(string(raw), `"program"`, `"extra":1,"program"`, 1)), "prog", wire.CodeMalformed},
		"trailing":      {append(append([]byte{}, raw...), []byte(" {}")...), "prog", wire.CodeMalformed},
		"fence":         {edit(func(r *AdmissionRecord) { r.Held[0].Fence = "BACKOFF" }), "prog", wire.CodeMalformed},
		"no free":       {edit(func(r *AdmissionRecord) { r.Held[0].Free = 0 }), "prog", wire.CodeMalformed},
		"pressure":      {edit(func(r *AdmissionRecord) { r.Pressure = "LOW" }), "prog", wire.CodeMalformed},
		"oversize":      {make([]byte, MaxAdmissionBytes+1), "prog", wire.CodeMalformed},
	} {
		if got := code(tc.raw, tc.program); got != tc.want {
			t.Fatalf("%s: code %s want %s", name, got, tc.want)
		}
	}
}
