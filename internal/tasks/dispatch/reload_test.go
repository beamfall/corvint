//go:build darwin || linux

package dispatch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// reloadFixture is a dispatcher watching a configuration file.
type reloadFixture struct {
	t    *testing.T
	path string
	d    *Dispatcher
	q    *fakeQueue
}

func newReloadFixture(t *testing.T, c *Config, q *fakeQueue) *reloadFixture {
	t.Helper()
	f := &reloadFixture{t: t, path: filepath.Join(t.TempDir(), "dispatch.json"), q: q}
	raw := f.write(c)
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	d.WatchConfig(func() ([]byte, error) { return os.ReadFile(f.path) }, raw)
	f.d = d
	return f
}

func (f *reloadFixture) write(c *Config) []byte {
	f.t.Helper()
	raw, err := json.Marshal(c)
	if err != nil {
		f.t.Fatal(err)
	}
	f.writeRaw(raw)
	return raw
}

func (f *reloadFixture) writeRaw(raw []byte) {
	f.t.Helper()
	if err := os.WriteFile(f.path, raw, 0o600); err != nil {
		f.t.Fatal(err)
	}
}

func (f *reloadFixture) tick() {
	f.t.Helper()
	if err := f.d.Tick(context.Background()); err != nil {
		f.t.Fatal(err)
	}
}

// events returns the events of kind recorded so far.
func (f *reloadFixture) events(kind string) []Event {
	f.t.Helper()
	all, err := ReadEvents(f.d.dir, 1000)
	if err != nil {
		f.t.Fatal(err)
	}
	var out []Event
	for _, e := range all {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

func digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func cloneConfig(t *testing.T, c *Config) *Config {
	t.Helper()
	raw, _ := json.Marshal(c)
	out, err := DecodeConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// CAL-V0-127: a changed valid file applies at the next tick; an invalid,
// unreadable or relocating file is refused once per content with the
// applied configuration kept; restoring the applied file clears the refusal.
func TestCALV0127_ConfigReloadAppliesAndRefuses(t *testing.T) {
	c := testConfig(t, "exit 0")
	f := newReloadFixture(t, c, &fakeQueue{})
	f.tick()
	if f.d.ledger.Config != nil || len(f.events("config")) != 0 {
		t.Fatalf("unchanged file recorded a reload: %+v", f.d.ledger.Config)
	}

	next := cloneConfig(t, c)
	next.Roles[0].Prompt = "changed {ticketLocal}"
	raw := f.write(next)
	f.tick()
	if f.d.Config.Roles[0].Prompt != "changed {ticketLocal}" {
		t.Fatal("valid change not applied")
	}
	rec := f.d.ledger.Config
	if rec == nil || rec.AppliedSha256 != digest(raw) || rec.Refused != nil || rec.AppliedAt.IsZero() {
		t.Fatalf("config record %+v", rec)
	}
	if ev := f.events("config"); len(ev) != 1 || ev[0].Detail["outcome"] != "APPLIED" || ev[0].Detail["sha256"] != digest(raw) {
		t.Fatalf("applied events %+v", ev)
	}

	invalid := []byte(`{"profile":"nope"}`)
	f.writeRaw(invalid)
	f.tick()
	f.tick() // the same refused content is reported once
	if f.d.Config.Roles[0].Prompt != "changed {ticketLocal}" {
		t.Fatal("invalid file replaced the applied configuration")
	}
	if r := f.d.ledger.Config.Refused; r == nil || r.Sha256 != digest(invalid) || r.Reason == "" || f.d.ledger.Config.AppliedSha256 != digest(raw) {
		t.Fatalf("refusal %+v", f.d.ledger.Config)
	}
	alerts := f.events("alert")
	if len(alerts) != 1 || alerts[0].Detail["config"] != "REFUSED" || alerts[0].Detail["sha256"] != digest(invalid) {
		t.Fatalf("refusal alerts %+v", alerts)
	}
	l, err := LoadLedger(f.d.dir, "prog")
	if err != nil || l.Config == nil || l.Config.Refused == nil {
		t.Fatalf("saved ledger config %+v %v", l, err)
	}

	f.writeRaw(raw)
	f.tick()
	if f.d.ledger.Config.Refused != nil {
		t.Fatal("restored file kept the refusal")
	}
	if ev := f.events("config"); len(ev) != 2 || ev[1].Detail["outcome"] != "RESTORED" {
		t.Fatalf("restore events %+v", ev)
	}

	moved := cloneConfig(t, next)
	moved.StateDir = filepath.Join(t.TempDir(), "elsewhere")
	f.write(moved)
	f.tick()
	if r := f.d.ledger.Config.Refused; r == nil || !strings.Contains(r.Reason, "stateDir and workRoot") || f.d.Config.StateDir != c.StateDir {
		t.Fatalf("stateDir change %+v", f.d.ledger.Config)
	}

	if err := os.Remove(f.path); err != nil {
		t.Fatal(err)
	}
	f.tick()
	f.tick()
	if r := f.d.ledger.Config.Refused; r == nil || r.Sha256 != "" || !strings.HasPrefix(r.Reason, "read: ") {
		t.Fatalf("unreadable file %+v", f.d.ledger.Config)
	}
	if alerts := f.events("alert"); len(alerts) != 3 || alerts[2].Detail["sha256"] != StateUnknown {
		t.Fatalf("unreadable alerts %+v", alerts)
	}

	// A restart applies the file afresh and drops the run's record.
	f.write(next)
	f.d.Close()
	d, err := Open("prog", next, f.q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if d.ledger.Config != nil {
		t.Fatalf("restart kept the reload record %+v", d.ledger.Config)
	}
}

// CAL-V0-127: a role removed by a reload launches nothing further while its
// running worker keeps running; pressure state follows the configuration.
func TestCALV0127_ReloadRemovedRoleKeepsWorkers(t *testing.T) {
	c := testConfig(t, "sleep 300")
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	f := newReloadFixture(t, c, q)
	f.tick()
	if f.d.Running() != 1 {
		t.Fatalf("running %d", f.d.Running())
	}
	w := f.d.ledger.Workers[0]
	t.Cleanup(func() { syscall.Kill(w.PID, syscall.SIGKILL) })

	next := cloneConfig(t, c)
	next.Roles = []Role{{Name: "other", Host: "sh", Cap: 1, Match: &Match{Labels: []string{"never"}}, Prompt: "p", IdleSeconds: 30, WallSeconds: 60}}
	p := issue497Config()
	next.Pressure = &p
	f.write(next)
	q.obs.Tickets = append(q.obs.Tickets, ticket("t2", "P1", 2))
	f.tick()
	if f.d.Running() != 1 || f.d.ledger.Workers[0].ID != w.ID {
		t.Fatalf("reload changed workers: running %d %+v", f.d.Running(), f.d.ledger.Workers)
	}
	ev := f.events("config")
	if len(ev) != 1 || ev[0].Detail["removedRoles"] != "impl" || !strings.Contains(ev[0].Message, "1 running worker(s) continue") {
		t.Fatalf("removed role event %+v", ev)
	}
	if f.d.ledger.Pressure == nil {
		t.Fatal("added pressure not tracked")
	}

	f.write(c)
	f.tick()
	if f.d.ledger.Pressure != nil {
		t.Fatal("removed pressure kept its record")
	}
}

// CAL-V0-128: cap 0 is valid and disables the role; other roles still launch.
func TestCALV0128_CapZeroDisablesRole(t *testing.T) {
	c := testConfig(t, "exit 0")
	c.Roles = append(c.Roles, Role{Name: "off", Host: "sh", Cap: 0, Match: &Match{}, Prompt: "p", IdleSeconds: 30, WallSeconds: 60})
	raw, _ := json.Marshal(c)
	if _, err := DecodeConfig(raw); err != nil {
		t.Fatalf("cap 0 refused: %v", err)
	}
	obs := &Observation{Tickets: []Ticket{ticket("a", "P1", 1), ticket("b", "P1", 2)}}
	got := Roster(c, obs, nil, nil)
	for _, a := range got {
		if a.Role == "off" {
			t.Fatalf("disabled role assigned: %+v", got)
		}
	}
	if len(got) != 2 {
		t.Fatalf("enabled role roster %+v", got)
	}
	c.Roles[0].Cap = 0
	if got := Roster(c, obs, nil, nil); len(got) != 0 {
		t.Fatalf("all roles disabled, roster %+v", got)
	}
	for _, cap := range []int{-1, 65} {
		c.Roles[1].Cap = cap
		raw, _ := json.Marshal(c)
		if _, err := DecodeConfig(raw); err == nil {
			t.Errorf("cap %d accepted", cap)
		}
	}
}

// CAL-V0-129: a lane admits a member only after it has stayed in one state
// episode for minAgeSeconds on the dispatcher clock.
func TestCALV0129_LaneMinAge(t *testing.T) {
	c := testConfig(t, "exit 0")
	c.Roles = []Role{{Name: "lane", Host: "sh", Cap: 4, Lane: &Lane{Pool: "pool", MinAgeSeconds: 60}, Prompt: "p", IdleSeconds: 30, WallSeconds: 60}}
	raw, _ := json.Marshal(c)
	if _, err := DecodeConfig(raw); err != nil {
		t.Fatalf("minAgeSeconds refused: %v", err)
	}
	q := &fakeQueue{}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	t0 := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	observe := func(at time.Duration, members ...Member) []Assignment {
		t.Helper()
		obs := &Observation{Members: members}
		d.stampMemberAges(obs, t0.Add(at))
		return Roster(c, obs, nil, nil)
	}
	m1 := Member{Pool: "pool", Member: "m1", State: "QUARANTINED", Changed: "7"}
	m2 := Member{Pool: "pool", Member: "m2", State: "QUARANTINED", Changed: "9"}
	if got := observe(0, m1); len(got) != 0 {
		t.Fatalf("new member admitted %+v", got)
	}
	if got := observe(59*time.Second, m1, m2); len(got) != 0 {
		t.Fatalf("young member admitted %+v", got)
	}
	if got := observe(60*time.Second, m1, m2); len(got) != 1 || got[0].Member != "m1" {
		t.Fatalf("aged member not admitted %+v", got)
	}
	// A new change sequence (re-quarantine) or a state change restarts
	// the episode; a member that disappears is forgotten.
	m1.Changed = "8"
	if got := observe(120*time.Second, m1, m2); len(got) != 1 || got[0].Member != "m2" {
		t.Fatalf("changed member kept its age %+v", got)
	}
	m2.State = "CLEANING"
	observe(130*time.Second, m1, m2)
	m2.State = "QUARANTINED"
	if got := observe(185*time.Second, m2); len(got) != 0 {
		t.Fatalf("state change kept the age %+v", got)
	}
	if got := observe(185*time.Second, m1, m2); len(got) != 0 {
		t.Fatalf("forgotten member kept its age %+v", got)
	}
	// A clock step backwards restarts the episode at the new time instead
	// of leaving a negative age that would stall the lane.
	if got := observe(150*time.Second, m1); len(got) != 0 {
		t.Fatalf("clock regression admitted %+v", got)
	}
	if got := observe(210*time.Second, m1); len(got) != 1 {
		t.Fatalf("member not aged after a clock regression %+v", got)
	}
	// Without minAgeSeconds the age is not consulted.
	c.Roles[0].Lane.MinAgeSeconds = 0
	if got := Roster(c, &Observation{Members: []Member{m1}}, nil, nil); len(got) != 1 {
		t.Fatalf("lane without minAgeSeconds %+v", got)
	}
	for _, v := range []int{-1, 7*86400 + 1} {
		c.Roles[0].Lane.MinAgeSeconds = v
		raw, _ := json.Marshal(c)
		if _, err := DecodeConfig(raw); err == nil {
			t.Errorf("minAgeSeconds %d accepted", v)
		}
	}
}

// CAL-V0-127: the ledger refuses a malformed reload record.
func TestCALV0127_ConfigRecordValidation(t *testing.T) {
	sum := strings.Repeat("a", 64)
	at := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	ok := []ConfigRecord{
		{AppliedSha256: sum, AppliedAt: at},
		{AppliedSha256: sum, AppliedAt: at, Refused: &ConfigRefusal{At: at, Reason: "read: gone"}},
		{AppliedSha256: sum, AppliedAt: at, Refused: &ConfigRefusal{Sha256: sum, At: at, Reason: "bad"}},
	}
	for i, r := range ok {
		if err := r.validate(); err != nil {
			t.Errorf("valid record %d refused: %v", i, err)
		}
	}
	bad := []ConfigRecord{
		{AppliedSha256: "abc", AppliedAt: at},
		{AppliedSha256: strings.Repeat("A", 64), AppliedAt: at},
		{AppliedSha256: sum},
		{AppliedSha256: sum, AppliedAt: at, Refused: &ConfigRefusal{Sha256: "x", At: at, Reason: "r"}},
		{AppliedSha256: sum, AppliedAt: at, Refused: &ConfigRefusal{At: at}},
		{AppliedSha256: sum, AppliedAt: at, Refused: &ConfigRefusal{Reason: "r"}},
		{AppliedSha256: sum, AppliedAt: at, Refused: &ConfigRefusal{At: at, Reason: strings.Repeat("r", maxConfigRefusal+1)}},
		{AppliedSha256: sum, AppliedAt: at, Refused: &ConfigRefusal{At: at, Reason: "\xff"}},
	}
	for i, r := range bad {
		if err := r.validate(); err == nil {
			t.Errorf("invalid record %d accepted", i)
		}
	}
}
