//go:build darwin || linux

package dispatch

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// ladderConfig is testConfig with a model-rendering host and an escalating
// role: base model A, B after one session without progress, C after two.
func ladderConfig(t *testing.T) *Config {
	t.Helper()
	c := testConfig(t, `echo "$1" > "$CORVINT_DISPATCH_WORKER.model"`)
	c.Hosts["sh"] = Host{Argv: []string{c.Hosts["sh"].Argv[0], "{model}", "{prompt}"}}
	c.Backoff.ParkAfter = 5
	c.Roles[0].Model = "A"
	c.Roles[0].Escalate = []Tier{{After: 1, Model: "B"}, {After: 2, Model: "C", Cap: 1, Notes: "operator-visible"}}
	return c
}

func TestCALV0052_EscalationConfigRefusesUnsupportedModels(t *testing.T) {
	c := ladderConfig(t)
	raw, _ := json.Marshal(c)
	if _, err := DecodeConfig(raw); err != nil {
		t.Fatalf("valid ladder refused: %v", err)
	}
	no := false
	mutate := map[string]func(*Config){
		"host without {model}":       func(c *Config) { c.Hosts["sh"] = Host{Argv: c.Hosts["sh"].Argv[:1]} },
		"{model} host without model": func(c *Config) { c.Roles[0].Model, c.Roles[0].Escalate = "", nil },
		"{model} prompt without model": func(c *Config) {
			c.Hosts["sh"] = Host{Argv: c.Hosts["sh"].Argv[:1]}
			c.Roles[0].Model, c.Roles[0].Escalate, c.Roles[0].Prompt = "", nil, "use {model}"
		},
		"escalate without model":      func(c *Config) { c.Roles[0].Model = "" },
		"deescalate without escalate": func(c *Config) { c.Roles[0].Escalate, c.Roles[0].DeescalateOnProgress = nil, &no },
		"bad model":                   func(c *Config) { c.Roles[0].Model = "-x y" },
		"after not increasing":        func(c *Config) { c.Roles[0].Escalate[1].After = 1 },
		"after zero":                  func(c *Config) { c.Roles[0].Escalate[0].After = 0 },
		"same model as tier below":    func(c *Config) { c.Roles[0].Escalate[1].Model = "B" },
		"tier cap above role cap":     func(c *Config) { c.Roles[0].Escalate[1].Cap = 3 },
		"notes control character":     func(c *Config) { c.Roles[0].Escalate[1].Notes = "a\nb" },
		"{model} activity path without model": func(c *Config) {
			c.Hosts["sh"] = Host{Argv: c.Hosts["sh"].Argv[:1], ActivityPaths: []string{"/tmp/{model}"}}
			c.Roles[0].Model, c.Roles[0].Escalate = "", nil
		},
		"model only in activity path": func(c *Config) {
			c.Hosts["sh"] = Host{Argv: c.Hosts["sh"].Argv[:1], ActivityPaths: []string{"/tmp/{model}"}}
		},
		"lane ladder": func(c *Config) {
			c.Roles[0].Match, c.Roles[0].Lane = nil, &Lane{Pool: "p"}
		},
		"nine tiers": func(c *Config) {
			c.Roles[0].Escalate = nil
			for i := 1; i <= 9; i++ {
				c.Roles[0].Escalate = append(c.Roles[0].Escalate, Tier{After: i, Model: "M" + string(rune('0'+i))})
			}
		},
	}
	for name, f := range mutate {
		c := ladderConfig(t)
		f(c)
		raw, _ := json.Marshal(c)
		if _, err := DecodeConfig(raw); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	raw = append(raw[:len(raw)-1], []byte(`,"roles":[{"name":"impl","host":"sh","cap":2,"match":{},"prompt":"p","idleSeconds":30,"wallSeconds":60,"model":"A","escalate":[{"after":1,"model":"B","extra":1}]}]}`)...)
	if _, err := DecodeConfig(raw); err == nil {
		t.Error("unknown tier field accepted")
	}
}

func TestCALV0054_RosterTierCapsWaitWithoutDowngrade(t *testing.T) {
	c := ladderConfig(t)
	c.Roles[0].Cap = 3
	obs := &Observation{Tickets: []Ticket{ticket("t1", "P1", 1), ticket("t2", "P1", 2), ticket("t3", "P1", 3)}}
	tiers := map[string]int{"ticket:a:q:t1": 2, "ticket:a:q:t2": 2}
	tierOf := func(role, key string) int { return tiers[key] }
	got := RosterTiers(c, obs, nil, nil, tierOf)
	var keys []string
	for _, a := range got {
		keys = append(keys, a.Key+"@"+string(rune('0'+a.Tier)))
	}
	// Tier 2 has cap 1: t2 waits rather than launching at a lower tier.
	if want := []string{"ticket:a:q:t1@2", "ticket:a:q:t3@0"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("assignments = %v, want %v", keys, want)
	}
	busy := []Busy{{Role: "impl", Key: "ticket:a:q:t1", Slot: 1, Tier: 2}}
	if got := RosterTiers(c, obs, busy, nil, tierOf); len(got) != 1 || got[0].Key != "ticket:a:q:t3" {
		t.Fatalf("busy tier-2 worker did not hold the tier cap: %+v", got)
	}
	if !reflect.DeepEqual(Roster(c, obs, nil, nil), RosterTiers(c, obs, nil, nil, nil)) {
		t.Fatal("Roster differs from RosterTiers without tiers")
	}
}

// The CAL-V0-057 tier cap is a static fence: a candidate waiting at a full
// tier neither charges the CAL-V0-068 pressure budget nor is reported held.
func TestCALV0054_TierCapPrecedesPressureBudget(t *testing.T) {
	c := ladderConfig(t)
	c.Roles[0].Cap = 3
	p := issue497Config()
	obs := &Observation{Tickets: []Ticket{ticket("t1", "P1", 1), ticket("t2", "P1", 2), ticket("t3", "P1", 3)}}
	tiers := map[string]int{"ticket:a:q:t1": 2, "ticket:a:q:t2": 2}
	tierOf := func(role, key string) int { return tiers[key] }
	plan := func(levelCap int) (out, held []string) {
		p.LevelCaps = map[string]int{"1": levelCap, "2": levelCap}
		b, err := NewPressureBudget(&p, PressureState{Level: 2}, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		launches, holds := roster(c, obs, nil, nil, tierOf, b, nil)
		for _, a := range launches {
			out = append(out, a.Local+"@"+string(rune('0'+a.Tier)))
		}
		for _, a := range holds {
			held = append(held, a.Local)
		}
		return out, held
	}
	// Budget 2: t2 waits on the tier-2 cap without consuming budget, so t3 launches.
	if out, held := plan(2); !reflect.DeepEqual(out, []string{"t1@2", "t3@0"}) || len(held) != 0 {
		t.Fatalf("budget 2: launches %v held %v", out, held)
	}
	// Budget 1: only t3 is held; the tier-capped t2 is waiting, not held.
	if out, held := plan(1); !reflect.DeepEqual(out, []string{"t1@2"}) || !reflect.DeepEqual(held, []string{"t3"}) {
		t.Fatalf("budget 1: launches %v held %v", out, held)
	}
}

func readModel(t *testing.T, c *Config, w *Worker) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(c.WorkRoot, w.ID+".model"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(raw))
}

func eventsOf(t *testing.T, d *Dispatcher, kind string) []Event {
	t.Helper()
	events, err := ReadEvents(d.dir, 1000)
	if err != nil {
		t.Fatal(err)
	}
	var out []Event
	for _, e := range events {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

// CAL-V0-057: consecutive sessions without progress climb the ladder, the
// host receives each tier's model, an escalated event names the rise, and
// progress resets the streak and returns the role to its base model.
func TestCALV0057_EscalationLadderClimbsAndResetsOnProgress(t *testing.T) {
	c := ladderConfig(t)
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	const key = "ticket:a:q:t1"
	var models []string
	step := func() {
		t.Helper()
		if err := d.Tick(ctx); err != nil || d.Running() != 1 {
			t.Fatalf("tick: %v running %d", err, d.Running())
		}
		w := d.ledger.Workers[0]
		waitEnded(t, d)
		models = append(models, readModel(t, c, w)+"@"+string(rune('0'+w.Tier)))
		if w.Model != c.Roles[0].ModelAt(w.Tier) {
			t.Fatalf("worker model %q at tier %d", w.Model, w.Tier)
		}
	}
	step()
	step()
	step()
	step() // the top tier holds while the streak grows
	if want := []string{"A@0", "B@1", "C@2", "C@2"}; !reflect.DeepEqual(models, want) {
		t.Fatalf("models = %v, want %v", models, want)
	}
	esc := eventsOf(t, d, "escalated")
	if len(esc) != 2 || esc[0].Detail["toTier"] != "1" || esc[0].Detail["model"] != "B" || esc[0].Detail["streak"] != "1" ||
		esc[1].Detail["fromModel"] != "B" || esc[1].Detail["model"] != "C" || esc[1].Detail["notes"] != "operator-visible" || esc[1].Worker == "" {
		t.Fatalf("escalated events = %+v", esc)
	}
	// The record is durable state: a reload sees the streak and the tier.
	l, err := LoadLedger(d.dir, "prog")
	if err != nil {
		t.Fatal(err)
	}
	if e := l.Escalation[key]; e == nil || e.Streak != 3 || e.Tiers["impl"] != 2 {
		t.Fatalf("ledger escalation = %+v", l.Escalation)
	}
	// Progress: the work state changes while the fourth session runs.
	q.obs.Tickets[0].Revision = "2"
	step()
	if models[len(models)-1] != "A@0" {
		t.Fatalf("progress did not deescalate: %v", models)
	}
	if e := d.ledger.Escalation[key]; e != nil && e.Streak != 0 {
		t.Fatalf("progress did not reset the streak: %+v", e)
	}
	if len(eventsOf(t, d, "escalated")) != 2 {
		t.Fatal("deescalation emitted an escalated event")
	}
	// A ticket that leaves the observation drops its record.
	q.obs.Tickets = nil
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if len(d.ledger.Escalation) != 0 {
		t.Fatalf("record outlived its ticket: %+v", d.ledger.Escalation)
	}
}

// deescalateOnProgress false keeps the reached tier after progress; an
// operator unpark resumes a parked ticket without resetting its streak.
func TestCALV0057_StickyTierAndOperatorUnparkKeepStreak(t *testing.T) {
	c := ladderConfig(t)
	no := false
	c.Roles[0].DeescalateOnProgress = &no
	c.Roles[0].Escalate = c.Roles[0].Escalate[:1]
	c.Backoff.ParkAfter = 1
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	const key = "ticket:a:q:t1"
	if err := d.Tick(ctx); err != nil || d.Running() != 1 {
		t.Fatalf("tick: %v", err)
	}
	waitEnded(t, d)
	if err := d.Tick(ctx); err != nil || d.Running() != 0 || !d.ledger.Backoff[key].Parked {
		t.Fatalf("not parked: %v", err)
	}
	raw, _ := json.Marshal(UnparkRequest{Unpark: key})
	os.WriteFile(filepath.Join(d.dir, "requests", "u.json"), raw, 0o600)
	if err := d.Tick(ctx); err != nil || d.Running() != 1 {
		t.Fatalf("unpark: %v", err)
	}
	if w := d.ledger.Workers[0]; w.Tier != 1 || readModelWait(t, c, d) != "B" {
		t.Fatalf("operator unpark reset the ladder: tier %d", w.Tier)
	}
	q.obs.Tickets[0].Revision = "2"
	waitEnded(t, d)
	if err := d.Tick(ctx); err != nil || d.Running() != 1 {
		t.Fatalf("relaunch after progress: %v", err)
	}
	if w := d.ledger.Workers[0]; w.Tier != 1 || d.ledger.Escalation[key].Streak != 0 {
		t.Fatalf("sticky tier lost after progress: tier %d %+v", w.Tier, d.ledger.Escalation[key])
	}
	waitEnded(t, d)
}

// CAL-V0-057: progress made outside a session, by a state change or a
// newly declared progress token while the ticket is parked or cooling down,
// resets the streak, so the next session starts on the base model.
func TestCALV0057_ProgressOutsideSessionResetsLadder(t *testing.T) {
	writeToken := func(t *testing.T, source, token string) {
		t.Helper()
		if err := os.WriteFile(source, []byte(`{"t1":{"state":"work","progress":"`+token+`"}}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	const key = "ticket:a:q:t1"
	open := func(t *testing.T, c *Config) (*Dispatcher, *fakeQueue, string) {
		t.Helper()
		source := filepath.Join(c.WorkRoot, "states.json")
		c.WorkState = &WorkState{Kind: "command", Argv: []string{"/bin/cat", source}}
		writeToken(t, source, "p1")
		q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
		d, err := Open("prog", c, q, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { d.Close() })
		return d, q, source
	}
	ctx := context.Background()
	for name, change := range map[string]func(*testing.T, *fakeQueue, string){
		"state change":      func(_ *testing.T, q *fakeQueue, _ string) { q.obs.Tickets[0].Revision = "2" },
		"declared progress": func(t *testing.T, _ *fakeQueue, source string) { writeToken(t, source, "p2") },
	} {
		t.Run("parked "+name, func(t *testing.T) {
			c := ladderConfig(t)
			c.Backoff.ParkAfter = 2
			d, q, source := open(t, c)
			for i := 0; i < 2; i++ {
				if err := d.Tick(ctx); err != nil || d.Running() != 1 {
					t.Fatalf("tick %d: %v", i, err)
				}
				waitEnded(t, d)
			}
			if err := d.Tick(ctx); err != nil || d.Running() != 0 || !d.ledger.Backoff[key].Parked || d.ledger.Escalation[key].Streak != 2 {
				t.Fatalf("not parked at streak 2: %v %+v", err, d.ledger.Escalation[key])
			}
			change(t, q, source)
			if err := d.Tick(ctx); err != nil || d.Running() != 1 {
				t.Fatalf("not unparked: %v", err)
			}
			if w := d.ledger.Workers[0]; w.Tier != 0 || readModelWait(t, c, d) != "A" {
				t.Fatalf("relaunched at tier %d after progress outside a session", w.Tier)
			}
			if e := d.ledger.Escalation[key]; e != nil && e.Streak != 0 {
				t.Fatalf("streak kept after progress: %+v", e)
			}
		})
	}
	t.Run("cooling down", func(t *testing.T) {
		c := ladderConfig(t)
		c.Backoff.CooldownSeconds = 3600
		d, q, _ := open(t, c)
		if err := d.Tick(ctx); err != nil || d.Running() != 1 {
			t.Fatalf("tick: %v", err)
		}
		waitEnded(t, d)
		if err := d.Tick(ctx); err != nil || d.Running() != 0 || d.ledger.Escalation[key].Streak != 1 {
			t.Fatalf("no cooldown at streak 1: %v", err)
		}
		q.obs.Tickets[0].Revision = "2"
		if err := d.Tick(ctx); err != nil {
			t.Fatal(err)
		}
		if e := d.ledger.Escalation[key]; e != nil {
			t.Fatalf("streak kept after progress during cooldown: %+v", e)
		}
	})
}

func readModelWait(t *testing.T, c *Config, d *Dispatcher) string {
	t.Helper()
	w := d.ledger.Workers[0]
	waitEnded(t, d)
	return readModel(t, c, w)
}

// legacyLedger is the state file the pre-ladder encoder (4b10a021) wrote for
// a ledger exercising every legacy member.
const legacyLedger = `{
  "profile": "taskman-dispatch-state/1",
  "program": "prog",
  "launchSeq": 3,
  "eventSeq": 9,
  "workers": [
    {
      "id": "prog.impl.1.x-3",
      "role": "impl",
      "host": "sh",
      "slot": 1,
      "key": "ticket:a:q:t1",
      "ticket": "ticket:a:q:t1",
      "pid": 42,
      "leaderIdentity": "id42",
      "members": [
        {
          "pid": 43,
          "identity": "id43"
        }
      ],
      "started": "2026-10-04T12:00:00Z",
      "lastActive": "2026-10-04T12:01:00Z",
      "logBytes": 7,
      "activityPaths": [
        "/tmp/a"
      ],
      "activityMtime": "2026-10-04T12:01:00Z",
      "state": "KILLING",
      "killReason": "idle",
      "killDeadline": "2026-10-04T12:02:00Z",
      "fingerprint": "f1",
      "baseFingerprint": "b1",
      "progressDigest": "d1"
    },
    {
      "id": "prog.lane.1.x-2",
      "role": "lane",
      "host": "sh",
      "slot": 1,
      "key": "lane:p:m",
      "pool": "p",
      "member": "m",
      "pid": 44,
      "leaderIdentity": "id44",
      "members": [],
      "started": "2026-10-04T12:00:00Z",
      "lastActive": "2026-10-04T12:00:00Z",
      "logBytes": 0,
      "activityMtime": "0001-01-01T00:00:00Z",
      "state": "RUNNING",
      "killDeadline": "0001-01-01T00:00:00Z",
      "fingerprint": "f2"
    }
  ],
  "backoff": {
    "ticket:a:q:t2": {
      "noProgress": 2,
      "cooldownUntil": "2026-10-04T13:00:00Z",
      "parked": true,
      "fingerprint": "f3",
      "baseFingerprint": "b3",
      "progressDigest": "d3"
    }
  },
  "seen": {
    "tickets": {
      "ticket:a:q:t1": "r1"
    },
    "claims": {},
    "lanes": {
      "p": "m"
    }
  },
  "progress": {
    "ticket:a:q:t1": {
      "current": "d1",
      "seen": [
        "d1"
      ]
    }
  }
}
`

// A configuration without a ladder leaves no escalation member, and a
// dispatcher opened without one drops a stale record: the state file stays
// byte-identical to the pre-ladder encoding.
func TestCALV0057_NoLadderKeepsLegacyLedger(t *testing.T) {
	dir := t.TempDir()
	encode := func(l *Ledger) string {
		t.Helper()
		if err := l.save(dir); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(filepath.Join(dir, "state.json"))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	var l Ledger
	if err := json.Unmarshal([]byte(legacyLedger), &l); err != nil {
		t.Fatal(err)
	}
	if got := encode(&l); got != legacyLedger {
		t.Fatalf("ladder-free encoding differs from the pre-ladder bytes:\n%s", got)
	}
	l.Escalation = map[string]*EscalationState{"ticket:a:q:t1": {Streak: 4, Tiers: map[string]int{"impl": 1}}}
	(&Dispatcher{Config: testConfig(t, "exit 0"), ledger: &l}).reconcileEscalation()
	if got := encode(&l); got != legacyLedger {
		t.Fatalf("stale record survived a ladder-free reconcile:\n%s", got)
	}

	c := testConfig(t, "exit 0")
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	dir = ProgramDir(c, "prog")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	stale := &Ledger{Profile: StateProfile, Program: "prog", Workers: []*Worker{}, Backoff: map[string]*BackoffState{},
		Escalation: map[string]*EscalationState{"ticket:a:q:t1": {Streak: 4, Tiers: map[string]int{"impl": 1}}}}
	if err := stale.save(dir); err != nil {
		t.Fatal(err)
	}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitEnded(t, d)
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	d.Close()
	raw, _ := os.ReadFile(filepath.Join(dir, "state.json"))
	if strings.Contains(string(raw), "escalation") || strings.Contains(string(raw), `"model"`) || strings.Contains(string(raw), `"tier"`) {
		t.Fatalf("ladder-free ledger carries ladder members:\n%s", raw)
	}
}

func TestCALV0057_LedgerEscalationIsBounded(t *testing.T) {
	dir := t.TempDir()
	write := func(s string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	head := `{"profile":"` + StateProfile + `","program":"prog","launchSeq":0,"eventSeq":0,"workers":[],"backoff":{}`
	write(head + `,"escalation":{"ticket:a:q:t1":{"streak":2,"tiers":{"impl":1}}}}`)
	if l, err := LoadLedger(dir, "prog"); err != nil || l.Escalation["ticket:a:q:t1"].Tiers["impl"] != 1 {
		t.Fatalf("valid escalation refused: %v", err)
	}
	digest := strings.Repeat("a", 64)
	progress := `,"progress":{"ticket:a:q:t2":{"current":"` + digest + `","seen":["` + digest + `"]}}`
	write(head + progress + `,"escalation":{"ticket:a:q:t1":{"streak":2}}}`)
	if _, err := LoadLedger(dir, "prog"); err != nil {
		t.Fatalf("strict ledger refused escalation: %v", err)
	}
	for name, body := range map[string]string{
		"negative streak": `,"escalation":{"ticket:a:q:t1":{"streak":-1}}}`,
		"tier zero":       `,"escalation":{"ticket:a:q:t1":{"streak":1,"tiers":{"impl":0}}}}`,
		"tier nine":       `,"escalation":{"ticket:a:q:t1":{"streak":1,"tiers":{"impl":9}}}}`,
		"bad key":         `,"escalation":{"t1":{"streak":1}}}`,
		"bad role":        `,"escalation":{"ticket:a:q:t1":{"streak":1,"tiers":{"Impl":1}}}}`,
		"unknown field":   `,"escalation":{"ticket:a:q:t1":{"streak":1,"extra":1}}}`,
		"strict alias":    progress + `,"escalation":{"ticket:a:q:t1":{"Streak":1}}}`,
	} {
		write(head + body)
		if _, err := LoadLedger(dir, "prog"); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
