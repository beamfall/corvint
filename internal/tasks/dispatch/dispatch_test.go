//go:build darwin || linux

package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/supervisor"
)

func testConfig(t *testing.T, script string) *Config {
	t.Helper()
	root := t.TempDir()
	host := filepath.Join(root, "host.sh")
	if err := os.WriteFile(host, []byte("#!/bin/sh\n"+script+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return &Config{
		Profile: ConfigProfile, StateDir: filepath.Join(root, "state"), WorkRoot: root,
		TickSeconds: 1, GlobalCap: 4, KillGraceSeconds: 1,
		Hosts:   map[string]Host{"sh": {Argv: []string{host, "{prompt}"}}},
		Roles:   []Role{{Name: "impl", Host: "sh", Cap: 2, Match: &Match{}, Prompt: "work on {ticketLocal} as {holder}", IdleSeconds: 30, WallSeconds: 60}},
		Backoff: Backoff{CooldownSeconds: 0, ParkAfter: 2},
		Heal:    Heal{Handoff: true, Reap: true},
	}
}

var identityOf = supervisor.ProcessIdentity

// gone waits briefly for a killed process to be reaped.
func gone(pid int) bool {
	for i := 0; i < 50; i++ {
		if id, _ := identityOf(pid); id == "" {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

func ticket(local, prio string, order uint64, labels ...string) Ticket {
	return Ticket{ID: "ticket:a:q:" + local, Local: local, Status: "OPEN", Priority: prio, Kind: "TASK", Revision: "1", Order: order, Labels: labels, State: StateNone}
}

func TestCALV0052_DecodeConfigIsClosedAndBounded(t *testing.T) {
	c := testConfig(t, "exit 0")
	raw, _ := json.Marshal(c)
	if _, err := DecodeConfig(raw); err != nil {
		t.Fatalf("valid config refused: %v", err)
	}
	mutate := map[string]func(*Config){
		"relative executable": func(c *Config) { c.Hosts["sh"] = Host{Argv: []string{"sh"}} },
		"reserved env": func(c *Config) {
			c.Hosts["sh"] = Host{Argv: c.Hosts["sh"].Argv, Env: map[string]string{"CORVINT_DISPATCH_ROLE": "x"}}
		},
		"two prompts":         func(c *Config) { c.Hosts["sh"] = Host{Argv: []string{c.Hosts["sh"].Argv[0], "{prompt}", "{prompt}"}} },
		"unknown placeholder": func(c *Config) { c.Roles[0].Prompt = "{secret}" },
		"match and lane":      func(c *Config) { c.Roles[0].Lane = &Lane{Pool: "p"} },
		"states no reader":    func(c *Config) { c.Roles[0].Match.States = []string{"review"} },
		"repeated role":       func(c *Config) { c.Roles = append(c.Roles, c.Roles[0]) },
		"global cap":          func(c *Config) { c.GlobalCap = 0 },
		"short idle":          func(c *Config) { c.Roles[0].IdleSeconds = 5 },
		"relative state dir":  func(c *Config) { c.StateDir = "state" },
		"status-line path":    func(c *Config) { c.WorkState = &WorkState{Kind: "status-line", Path: "/x/{ticket}.md", Key: "state"} },
	}
	for name, f := range mutate {
		c := testConfig(t, "exit 0")
		f(c)
		raw, _ := json.Marshal(c)
		if _, err := DecodeConfig(raw); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	raw = append(raw[:len(raw)-1], []byte(`,"extra":1}`)...)
	if _, err := DecodeConfig(raw); err == nil {
		t.Error("unknown field accepted")
	}
}

func TestCALV0052_RenderIsSinglePass(t *testing.T) {
	got := Render("{ticket} {holder} {nope}", map[string]string{"{ticket}": "{holder}", "{holder}": "h"})
	if got != "{holder} h {nope}" {
		t.Fatalf("Render = %q", got)
	}
}

func TestCALV0054_RosterIsDeterministicAndCapped(t *testing.T) {
	c := testConfig(t, "exit 0")
	c.GlobalCap = 3
	c.Roles = append(c.Roles, Role{Name: "review", Host: "sh", Cap: 1, Match: &Match{Labels: []string{"review"}}, Prompt: "review {ticket}", IdleSeconds: 30, WallSeconds: 60})
	c.Roles[0].Priority = 10
	c.Pinned = []string{"t4"}
	obs := &Observation{
		Tickets: []Ticket{ticket("t1", "P2", 1), ticket("t2", "P0", 9), ticket("t3", "P1", 1, "review"), ticket("t4", "P3", 1), ticket("t5", "P0", 1)},
		Attempts: []Attempt{
			{ID: "a5", Ticket: "ticket:a:q:t5", Live: true, LeaseExpires: time.Now().Add(-time.Hour)},
		},
	}
	got := Roster(c, obs, nil, nil)
	want := []Assignment{
		{Role: "impl", Key: "ticket:a:q:t4", Ticket: "ticket:a:q:t4", Local: "t4", State: StateNone, Slot: 1},
		{Role: "review", Key: "ticket:a:q:t3", Ticket: "ticket:a:q:t3", Local: "t3", State: StateNone, Slot: 1},
		{Role: "impl", Key: "ticket:a:q:t2", Ticket: "ticket:a:q:t2", Local: "t2", State: StateNone, Slot: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("roster\n got %+v\nwant %+v", got, want)
	}
	if again := Roster(c, obs, nil, nil); !reflect.DeepEqual(again, got) {
		t.Fatal("roster is not deterministic")
	}
	// A busy slot and key and a skipped key are honored; the free slot is reused.
	got = Roster(c, obs, []Busy{{Role: "impl", Key: "ticket:a:q:t4", Slot: 1}}, map[string]bool{"ticket:a:q:t2": true})
	want = []Assignment{
		{Role: "review", Key: "ticket:a:q:t3", Ticket: "ticket:a:q:t3", Local: "t3", State: StateNone, Slot: 1},
		{Role: "impl", Key: "ticket:a:q:t1", Ticket: "ticket:a:q:t1", Local: "t1", State: StateNone, Slot: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("busy roster\n got %+v\nwant %+v", got, want)
	}
}

func TestCALV0054_RosterStatePredicatesAndLanes(t *testing.T) {
	c := testConfig(t, "exit 0")
	c.WorkState = &WorkState{Kind: "status-line", Path: "/x/{ticketLocal}.md", Key: "state"}
	c.Roles = []Role{
		{Name: "review", Host: "sh", Cap: 4, Match: &Match{States: []string{"built"}, ExcludeStates: []string{"review-passed"}}, Prompt: "p", IdleSeconds: 30, WallSeconds: 60},
		{Name: "lane", Host: "sh", Cap: 4, Lane: &Lane{Pool: "pool"}, Prompt: "p", IdleSeconds: 30, WallSeconds: 60},
	}
	a, b, u := ticket("a", "P1", 1), ticket("b", "P1", 2), ticket("u", "P1", 3)
	a.State, b.State, u.State = "built", "review-passed", StateUnknown
	obs := &Observation{Tickets: []Ticket{a, b, u}, Members: []Member{{Pool: "pool", Member: "m1", State: "QUARANTINED"}, {Pool: "pool", Member: "m2", State: "READY"}}}
	got := Roster(c, obs, nil, nil)
	if len(got) != 2 || got[0].Local != "a" || got[1].Key != "lane:pool/m1" || got[1].Member != "m1" {
		t.Fatalf("roster = %+v", got)
	}
}

func TestCALV0057_FingerprintIgnoresNonDurableAttempts(t *testing.T) {
	obs := &Observation{Tickets: []Ticket{ticket("t", "P1", 1)}}
	base := Fingerprint(obs, "ticket:a:q:t")
	obs.Attempts = []Attempt{{ID: "x", Ticket: "ticket:a:q:t", Phase: "CANCELLED"}}
	if Fingerprint(obs, "ticket:a:q:t") != base {
		t.Fatal("a cancelled claim without output counted as progress")
	}
	obs.Attempts[0].Candidate = "abc"
	if Fingerprint(obs, "ticket:a:q:t") == base {
		t.Fatal("a submitted candidate did not count as progress")
	}
}

func TestCALV0053_WorkStateReaders(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "t1.md"), []byte("# t1\nstate: built\nstate: later\n"), 0o600)
	os.WriteFile(filepath.Join(dir, "t2.md"), []byte("state: bad\x01value\n"), 0o600)
	c := &Config{WorkRoot: dir, WorkState: &WorkState{Kind: "status-line", Path: filepath.Join(dir, "{ticketLocal}.md"), Key: "state"}}
	ts := []Ticket{ticket("t1", "P1", 1), ticket("t2", "P1", 1), ticket("t3", "P1", 1)}
	alerts := ReadStates(context.Background(), c, ts)
	if ts[0].State != "built" || ts[1].State != StateUnknown || ts[2].State != StateNone || len(alerts) != 1 {
		t.Fatalf("states %q %q %q alerts %v", ts[0].State, ts[1].State, ts[2].State, alerts)
	}
	script := filepath.Join(dir, "states.sh")
	os.WriteFile(script, []byte("#!/bin/sh\necho '{\"t1\":\"review\",\"ticket:a:q:t2\":\"done\"}'\n"), 0o700)
	c.WorkState = &WorkState{Kind: "command", Argv: []string{script}}
	if alerts := ReadStates(context.Background(), c, ts); len(alerts) != 0 || ts[0].State != "review" || ts[1].State != "done" || ts[2].State != StateNone {
		t.Fatalf("command states %+v alerts %v", ts, alerts)
	}
	os.WriteFile(script, []byte("#!/bin/sh\nexit 3\n"), 0o700)
	if alerts := ReadStates(context.Background(), c, ts); len(alerts) != 1 || ts[0].State != StateUnknown {
		t.Fatalf("failed command states %+v alerts %v", ts, alerts)
	}
}

// fakeQueue is an in-memory Queue whose release records handoffs.
type fakeQueue struct {
	obs      Observation
	fail     error
	released []string
	evidence []string
	reaped   []string
}

func (q *fakeQueue) Observe(context.Context) (*Observation, error) {
	if q.fail != nil {
		return nil, q.fail
	}
	o := q.obs
	o.Tickets = append([]Ticket(nil), q.obs.Tickets...)
	o.Attempts = append([]Attempt(nil), q.obs.Attempts...)
	return &o, nil
}

func (q *fakeQueue) Release(_ context.Context, a Attempt, evidence, _ string) error {
	q.released, q.evidence = append(q.released, a.ID), append(q.evidence, evidence)
	q.end(a.ID)
	return nil
}

func (q *fakeQueue) Reap(_ context.Context, a Attempt, _ string) error {
	q.reaped = append(q.reaped, a.ID)
	q.end(a.ID)
	return nil
}

func (q *fakeQueue) end(id string) {
	for i := range q.obs.Attempts {
		if q.obs.Attempts[i].ID == id {
			q.obs.Attempts[i].Live, q.obs.Attempts[i].Phase = false, "CANCELLED"
		}
	}
}

func kinds(t *testing.T, d *Dispatcher) []string {
	t.Helper()
	events, err := ReadEvents(d.dir, 1000)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range events {
		out = append(out, e.Kind)
	}
	return out
}

func has(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func waitEnded(t *testing.T, d *Dispatcher) {
	t.Helper()
	for i := 0; i < 100; i++ {
		ended := true
		for _, w := range d.ledger.Workers {
			if id, _ := identityOf(w.PID); id != "" && id == w.LeaderIdentity {
				ended = false
			}
		}
		if ended {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("worker did not exit")
}

func TestCALV0055_LaunchFinishBackoffAndPark(t *testing.T) {
	c := testConfig(t, `echo "$1" > "$CORVINT_DISPATCH_WORKER.prompt"; echo '{"type":"text","part":{"text":"nothing to do"}}'`)
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, err := Open("prog", c, q, io.Discard); err == nil {
		t.Fatal("a second dispatcher opened the same program")
	}
	if OwnerState(d.dir) != "RUNNING" {
		t.Fatal("owner state is not RUNNING")
	}
	ctx := context.Background()
	if err := d.Tick(ctx); err != nil || d.Running() != 1 {
		t.Fatalf("tick: %v running %d", err, d.Running())
	}
	w := d.ledger.Workers[0]
	waitEnded(t, d)
	prompt, _ := os.ReadFile(filepath.Join(c.WorkRoot, w.ID+".prompt"))
	if strings.TrimSpace(string(prompt)) != "work on t1 as "+w.ID {
		t.Fatalf("prompt = %q", prompt)
	}
	// Run 1 ends without progress: cooldown 0 allows an immediate relaunch.
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	waitEnded(t, d)
	// Run 2 ends without progress: parkAfter 2 parks the ticket.
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if d.Running() != 0 || !d.ledger.Backoff["ticket:a:q:t1"].Parked {
		t.Fatalf("not parked: running %d backoff %+v", d.Running(), d.ledger.Backoff)
	}
	got := kinds(t, d)
	for _, k := range []string{"started", "launched", "finished", "cooldown", "parked", "needs-owner"} {
		if !has(got, k) {
			t.Errorf("missing %s event in %v", k, got)
		}
	}
	events, _ := ReadEvents(d.dir, 1000)
	for _, e := range events {
		if e.Kind == "finished" && !strings.Contains(e.Message, "nothing to do") {
			t.Errorf("finished event lacks the host summary: %q", e.Message)
		}
	}
	// An operator unpark request is consumed on the next tick and the ticket relaunches.
	raw, _ := json.Marshal(UnparkRequest{Unpark: "ticket:a:q:t1"})
	os.WriteFile(filepath.Join(d.dir, "requests", "u.json"), raw, 0o600)
	if err := d.Tick(ctx); err != nil || d.Running() != 1 || !has(kinds(t, d), "unparked") {
		t.Fatalf("unpark: %v running %d", err, d.Running())
	}
	waitEnded(t, d)
}

func TestCALV0056_HandoffAndReap(t *testing.T) {
	c := testConfig(t, "exit 0")
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1), ticket("t2", "P1", 2)}}}
	q.obs.Attempts = []Attempt{{ID: "old", Ticket: "ticket:a:q:t2", Phase: "RUNNING", Generation: "1", Holder: "gone", Live: true, LeaseExpires: time.Now().Add(-time.Minute)}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(q.reaped, []string{"old"}) || d.Running() != 2 {
		t.Fatalf("reaped %v running %d", q.reaped, d.Running())
	}
	// The worker on t1 "claims" (the fake records a live attempt it holds), then exits.
	w := d.ledger.Workers[0]
	q.obs.Attempts = append(q.obs.Attempts, Attempt{ID: "claim", Ticket: w.Ticket, Phase: "RUNNING", Generation: "1", Holder: w.ID, Live: true, LeaseExpires: time.Now().Add(time.Hour)})
	waitEnded(t, d)
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(q.released, []string{"claim"}) || q.evidence[0] != "dispatch:"+w.ID {
		t.Fatalf("released %v evidence %v", q.released, q.evidence)
	}
	if !has(kinds(t, d), "handoff") || !has(kinds(t, d), "reaped") {
		t.Fatalf("events %v", kinds(t, d))
	}
	waitEnded(t, d)
}

func TestCALV0056_KillsWholeTreeAndAdoptsAcrossRestart(t *testing.T) {
	// The leader starts a child in its own process group (as opencode's
	// server does) and both sleep; a restart adopts them and the wall cap
	// then kills the whole tree.
	c := testConfig(t, `set -m; sleep 300 & echo "$!" > "$CORVINT_DISPATCH_WORKER.child"; sleep 300`)
	c.GlobalCap = 1
	c.Backoff.CooldownSeconds = 3600
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	w := *d.ledger.Workers[0]
	var child int
	for i := 0; i < 100 && child == 0; i++ {
		raw, _ := os.ReadFile(filepath.Join(c.WorkRoot, w.ID+".child"))
		child = atoi(strings.TrimSpace(string(raw)))
		time.Sleep(20 * time.Millisecond)
	}
	if child == 0 {
		t.Fatal("child pid not written")
	}
	t.Cleanup(func() { syscall.Kill(child, syscall.SIGKILL); syscall.Kill(w.PID, syscall.SIGKILL) })
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	d, err = Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if d.Running() != 1 || !has(kinds(t, d), "adopted") {
		t.Fatalf("not adopted: running %d events %v", d.Running(), kinds(t, d))
	}
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if n := len(d.ledger.Workers[0].Members); n < 3 {
		t.Fatalf("tree has %d members, want leader, sleep and the grouped child", n)
	}
	d.Now = func() time.Time { return time.Now().Add(2 * time.Minute) }
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if !gone(child) {
		t.Fatal("the child in its own process group survived")
	}
	if !gone(w.PID) {
		t.Fatal("the leader survived")
	}
	got := kinds(t, d)
	if !has(got, "killing") || !has(got, "killed") || !has(got, "finished") {
		t.Fatalf("events %v", got)
	}
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func TestCALV0056_KillsOrphanedProcessesBySession(t *testing.T) {
	// The leader exits at once and leaves a child in its own process group;
	// the session still ties the child to the worker.
	c := testConfig(t, `set -m; sleep 300 & echo "$!" > "$CORVINT_DISPATCH_WORKER.child"; exit 0`)
	c.Backoff.CooldownSeconds = 3600
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	w := *d.ledger.Workers[0]
	waitEnded(t, d)
	raw, _ := os.ReadFile(filepath.Join(c.WorkRoot, w.ID+".child"))
	child := atoi(strings.TrimSpace(string(raw)))
	if child == 0 {
		t.Fatal("child pid not written")
	}
	t.Cleanup(func() { syscall.Kill(child, syscall.SIGKILL) })
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if !gone(child) {
		t.Fatal("the orphaned child survived")
	}
	events, _ := ReadEvents(d.dir, 1000)
	orphaned := false
	for _, e := range events {
		orphaned = orphaned || (e.Kind == "killing" && e.Detail["reason"] == "ORPHANED")
	}
	if !orphaned || d.Running() != 0 {
		t.Fatalf("orphan not killed: running %d events %v", d.Running(), kinds(t, d))
	}
}

// Review regressions: worker IDs cannot collide across dashed program and
// role names and carry a per-start nonce; an unreadable identity never ends a
// worker; an UNKNOWN work state is not progress; wall enforcement survives a
// store outage and the kill deadline is kept on the worker.
func TestCALV0056_IdentityOutageAndUnknownState(t *testing.T) {
	c := testConfig(t, `sleep 300`)
	c.Roles[0].Name = "impl-x"
	c.Backoff.CooldownSeconds = 3600
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	d, err := Open("prog-a", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if err := d.Tick(ctx); err != nil || d.Running() != 1 {
		t.Fatalf("tick: %v running %d", err, d.Running())
	}
	w := d.ledger.Workers[0]
	t.Cleanup(func() { syscall.Kill(w.PID, syscall.SIGKILL) })
	if !regexp.MustCompile(`^prog-a\.impl-x\.1\.`+d.nonce+`-1$`).MatchString(w.ID) || len(d.nonce) != 8 {
		t.Fatalf("worker ID %q (nonce %q)", w.ID, d.nonce)
	}
	if l, err := LoadLedger(d.dir, "prog-a"); err != nil || len(l.Workers) != 1 {
		t.Fatalf("worker not saved at launch: %v", err)
	}

	processIdentity = func(int) (string, error) { return "", errors.New("injected") }
	syscall.Kill(w.PID, syscall.SIGKILL)
	if !gone(w.PID) {
		t.Fatal("leader survived")
	}
	err = d.Tick(ctx)
	processIdentity = supervisor.ProcessIdentity
	if err != nil || d.Running() != 1 || len(d.ledger.Workers[0].Members) != 1 || has(kinds(t, d), "finished") {
		t.Fatalf("unreadable identity ended the worker: %v running %d events %v", err, d.Running(), kinds(t, d))
	}

	q.obs.Tickets[0].State = StateUnknown
	if err := d.Tick(ctx); err != nil || d.Running() != 0 {
		t.Fatalf("tick: %v running %d", err, d.Running())
	}
	if b := d.ledger.Backoff["ticket:a:q:t1"]; b == nil || b.NoProgress != 1 {
		t.Fatalf("UNKNOWN state counted as progress: %+v", b)
	}

	q.obs.Tickets[0].State = ""
	delete(d.ledger.Backoff, "ticket:a:q:t1")
	if err := d.Tick(ctx); err != nil || d.Running() != 1 {
		t.Fatalf("relaunch: %v running %d", err, d.Running())
	}
	w2 := d.ledger.Workers[0]
	t.Cleanup(func() { syscall.Kill(w2.PID, syscall.SIGKILL) })
	if w2.ID == w.ID {
		t.Fatalf("worker ID reused: %s", w2.ID)
	}
	q.fail = errors.New("store unreadable")
	d.Now = func() time.Time { return time.Now().Add(2 * time.Minute) }
	if err := d.Tick(ctx); err == nil {
		t.Fatal("store outage not reported")
	}
	if !gone(w2.PID) || !has(kinds(t, d), "killed") {
		t.Fatalf("wall cap not enforced during a store outage: %v", kinds(t, d))
	}
	if d.Running() != 1 || d.ledger.Workers[0].KillDeadline.IsZero() {
		t.Fatal("killed worker not kept for accounting with its kill deadline")
	}
	q.fail = nil
	if err := d.Tick(ctx); err != nil || d.Running() != 0 {
		t.Fatalf("accounting after outage: %v running %d", err, d.Running())
	}
}

// CAL-V0-058: the finished summary is the host's final agent text for every
// recognized event stream, and the raw tail otherwise.
func TestCALV0058_SummaryReadsHostFinalText(t *testing.T) {
	for name, tc := range map[string]struct{ stdout, want string }{
		"opencode": {`{"type":"step_start"}` + "\n" + `{"type":"text","part":{"text":"opencode done"}}` + "\n", "opencode done"},
		"codex":    {`{"type":"item.completed","item":{"type":"agent_message","text":"codex done"}}` + "\n" + `{"type":"turn.completed"}` + "\n", "codex done"},
		"codex exec --json": {`{"type":"thread.started","thread_id":"t"}` + "\n" + `{"type":"turn.started"}` + "\n" +
			`{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"I'll run it.\n"}}` + "\n" +
			`{"type":"item.started","item":{"id":"item_1","type":"command_execution","command":"ls","aggregated_output":"","exit_code":null,"status":"in_progress"}}` + "\n" +
			`{"type":"item.completed","item":{"id":"item_1","type":"command_execution","command":"ls","aggregated_output":"README\n","exit_code":0,"status":"completed"}}` + "\n" +
			`{"type":"item.completed","item":{"id":"item_2","type":"file_change","changes":[{"path":"src/hello.txt","kind":"add"}],"status":"completed"}}` + "\n" +
			`{"type":"item.completed","item":{"id":"item_3","type":"agent_message","text":"Claimed and done."}}` + "\n" +
			`{"type":"item.completed","item":{"id":"item_4","type":"reasoning","text":"thinking it over"}}` + "\n" +
			`{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1}}` + "\n", "Claimed and done."},
		"claude stream-json": {`{"type":"system","subtype":"init","session_id":"s"}` + "\n" +
			`{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"x"}]}}` + "\n" +
			`{"type":"assistant","message":{"content":[{"type":"text","text":"draft"},{"type":"tool_use","name":"Bash"}]}}` + "\n" +
			`{"type":"user","message":{"content":[{"type":"tool_result","content":"ok"}]}}` + "\n" +
			`{"type":"assistant","message":{"content":[{"type":"text","text":"claude final"}]}}` + "\n" +
			`{"type":"rate_limit_event"}` + "\n" +
			`{"type":"result","subtype":"success","is_error":false,"result":"claude done"}` + "\n", "claude done"},
		"claude json":             {`{"type":"result","subtype":"success","result":"claude json done","session_id":"s"}`, "claude json done"},
		"claude error result":     {`{"type":"assistant","message":{"content":[{"type":"text","text":"last words"}]}}` + "\n" + `{"type":"result","subtype":"error_max_turns","is_error":true}` + "\n", "last words"},
		"claude subagent ignored": {`{"type":"assistant","parent_tool_use_id":null,"message":{"content":[{"type":"text","text":"main words"}]}}` + "\n" + `{"type":"assistant","parent_tool_use_id":"toolu_1","message":{"content":[{"type":"text","text":"subagent words"}]}}` + "\n", "main words"},
		"gemini stream-json": {`{"type":"init","timestamp":"t","session_id":"s","model":"m"}` + "\n" +
			`{"type":"message","timestamp":"t","role":"user","content":"claim AT-0002"}` + "\n" +
			`{"type":"message","timestamp":"t","role":"assistant","content":"I'll ","delta":true}` + "\n" +
			`{"type":"message","timestamp":"t","role":"assistant","content":"claim it.","delta":true}` + "\n" +
			`{"type":"tool_use","timestamp":"t","tool_name":"run_shell_command","tool_id":"c1","parameters":{"command":"ct claim"}}` + "\n" +
			`{"type":"tool_result","timestamp":"t","tool_id":"c1","status":"success","output":"ok"}` + "\n" +
			`{"type":"message","timestamp":"t","role":"assistant","content":"Claimed ","delta":true}` + "\n" +
			`{"type":"message","timestamp":"t","role":"assistant","content":"and done.","delta":true}` + "\n" +
			`{"type":"error","timestamp":"t","severity":"warning","message":"loop detected"}` + "\n" +
			`{"type":"result","timestamp":"t","status":"success","stats":{"total_tokens":1}}` + "\n", "Claimed and done."},
		"gemini json":            {"{\n  \"session_id\": \"s\",\n  \"response\": \"gemini json done\",\n  \"stats\": {\n    \"models\": {}\n  }\n}", "gemini json done"},
		"plain text":             {"working\nall done\n", "working\nall done"},
		"mixed text is raw tail": {`{"type":"result","result":"x"}` + "\nplain\n", `{"type":"result","result":"x"}` + "\nplain"},
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "stdout.log"), []byte(tc.stdout), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := Summary(dir); got != tc.want {
			t.Errorf("%s: summary %q, want %q", name, got, tc.want)
		}
	}
}
