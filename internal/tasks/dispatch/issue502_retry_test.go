//go:build darwin || linux

package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const retryKey = "ticket:a:q:t1"

func intp(n int) *int { return &n }

// retryDispatcher opens a dispatcher on one ticket at acceptance revision 1
// with the ESC-V0-007 policy, a parkAfter of 1 that would park any ordinary
// no-progress session at once, and a controllable clock.
func retryDispatcher(t *testing.T, policy *InfraRetryConfig) (*Dispatcher, *fakeQueue, *time.Time) {
	t.Helper()
	c := testConfig(t, "exit 0")
	c.Backoff.ParkAfter = 1
	c.InfrastructureRetry = policy
	tk := ticket("t1", "P1", 1)
	tk.AcceptanceRevision = "1"
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{tk}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	now := time.Now()
	d.Now = func() time.Time { return now }
	return d, q, &now
}

// infraSession launches one worker, has it raise a typed infrastructure
// request as its holder, lets it end, and accounts it.
func infraSession(t *testing.T, d *Dispatcher, q *fakeQueue) string {
	t.Helper()
	ctx := context.Background()
	if err := d.Tick(ctx); err != nil || d.Running() != 1 {
		t.Fatalf("tick: %v running %d", err, d.Running())
	}
	id := d.ledger.Workers[0].ID
	q.obs.Tickets[0].Infrastructure = []string{id}
	waitEnded(t, d)
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	return id
}

func lastDetail(t *testing.T, d *Dispatcher, kind string) map[string]string {
	t.Helper()
	ev := eventsOf(t, d, kind)
	if len(ev) == 0 {
		t.Fatalf("no %s event", kind)
	}
	return ev[len(ev)-1].Detail
}

func hasCode(t *testing.T, d *Dispatcher, code string) bool {
	t.Helper()
	for _, ev := range eventsOf(t, d, "needs-owner") {
		if ev.Detail["code"] == code {
			return true
		}
	}
	return false
}

func TestIssue502_InfraRetryPolicyBoundsAndCooldown(t *testing.T) {
	if n, c, m := (&InfraRetryConfig{}).Limits(); n != 3 || c != 30 || m != 300 {
		t.Fatalf("defaults = %d %d %d", n, c, m)
	}
	for n, want := range map[int]time.Duration{1: 30, 2: 60, 3: 120, 4: 240, 5: 300, 10: 300, 1 << 30: 300} {
		if got := RetryCooldown(30, 300, n); got != want*time.Second {
			t.Errorf("RetryCooldown(30, 300, %d) = %v, want %vs", n, got, int(want))
		}
	}
	if got := RetryCooldown(3600, 86400, 64); got != 86400*time.Second {
		t.Errorf("saturation = %v", got)
	}
	for name, p := range map[string]*InfraRetryConfig{
		"maxRetries 11":       {MaxRetries: intp(11)},
		"maxRetries -1":       {MaxRetries: intp(-1)},
		"cooldown 0":          {CooldownSeconds: intp(0)},
		"cooldown 3601":       {CooldownSeconds: intp(3601), MaxCooldownSeconds: intp(4000)},
		"max below cooldown":  {CooldownSeconds: intp(60), MaxCooldownSeconds: intp(59)},
		"max above 86400":     {MaxCooldownSeconds: intp(86401)},
		"default max below c": {CooldownSeconds: intp(301)},
	} {
		c := testConfig(t, "exit 0")
		c.InfrastructureRetry = p
		raw, _ := json.Marshal(c)
		if _, err := DecodeConfig(raw); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	c := testConfig(t, "exit 0")
	c.InfrastructureRetry = &InfraRetryConfig{MaxRetries: intp(0)}
	raw, _ := json.Marshal(c)
	if _, err := DecodeConfig(raw); err != nil {
		t.Fatalf("maxRetries 0 refused: %v", err)
	}
	if _, err := DecodeConfig([]byte(strings.Replace(string(raw), `"maxRetries":0`, `"maxRetries":0,"extra":1`, 1))); err == nil {
		t.Fatal("unknown infrastructureRetry member accepted")
	}
}

// ESC-V0-007/008: an infrastructure session neither counts nor parks; it
// charges one reserved retry with a doubling, capped cooldown, and the last
// allowed retry is followed by an exhausted hold an operator release cannot
// refill.
func TestIssue502_InfraSessionChargesWithoutParking(t *testing.T) {
	d, q, now := retryDispatcher(t, &InfraRetryConfig{MaxRetries: intp(2), CooldownSeconds: intp(10), MaxCooldownSeconds: intp(15)})
	ctx := context.Background()
	first := infraSession(t, d, q)
	if d.ledger.Backoff[retryKey] != nil {
		t.Fatalf("infrastructure session counted as no-progress: %+v", d.ledger.Backoff[retryKey])
	}
	if got := lastDetail(t, d, "finished")["session"]; got != SessionInfrastructure {
		t.Fatalf("finished session = %q", got)
	}
	e := d.ledger.InfraRetry[retryKey]
	if e == nil || e.State != InfraWaiting || e.Sessions != 1 || e.Charged != 1 || e.Limit != 2 || !e.CooldownUntil.Equal(now.Add(10*time.Second)) || e.Launch != "" {
		t.Fatalf("episode after session 1 = %+v", e)
	}
	if dt := lastDetail(t, d, "cooldown"); dt["code"] != "INFRA_RETRY" || dt["charged"] != "1" {
		t.Fatalf("cooldown detail = %v", dt)
	}
	if d.Running() != 0 {
		t.Fatal("launched before the retry deadline")
	}
	// The due retry is reserved in the saved ledger before it spawns, and
	// is RUNNING once recorded.
	*now = now.Add(10 * time.Second)
	second := infraSession(t, d, q)
	if second == first {
		t.Fatal("a new retry reused an ended session's identity")
	}
	e = d.ledger.InfraRetry[retryKey]
	if e.State != InfraWaiting || e.Sessions != 2 || e.Charged != 2 || !e.CooldownUntil.Equal(now.Add(15*time.Second)) {
		t.Fatalf("episode after session 2 = %+v", e)
	}
	*now = now.Add(15 * time.Second)
	if err := d.Tick(ctx); err != nil || d.Running() != 1 {
		t.Fatalf("retry 2: %v running %d", err, d.Running())
	}
	third := d.ledger.Workers[0].ID
	l, err := LoadLedger(d.dir, "prog")
	if err != nil || l.InfraRetry[retryKey].State != InfraRunning || l.InfraRetry[retryKey].Launch != third {
		t.Fatalf("saved episode = %+v %v", l.InfraRetry[retryKey], err)
	}
	// The final allowed retry ends in another infrastructure session: the
	// next would exceed maxRetries, so the ticket holds and nothing parks.
	q.obs.Tickets[0].Infrastructure = []string{first, second, third}
	waitEnded(t, d)
	if err := d.Tick(ctx); err != nil || d.Running() != 0 {
		t.Fatalf("exhausted tick: %v running %d", err, d.Running())
	}
	e = d.ledger.InfraRetry[retryKey]
	if e.State != InfraExhausted || e.Sessions != 3 || e.Charged != 2 || d.ledger.Backoff[retryKey] != nil {
		t.Fatalf("episode after session 3 = %+v backoff %+v", e, d.ledger.Backoff[retryKey])
	}
	if dt := lastDetail(t, d, "needs-owner"); dt["code"] != "INFRA_RETRY_EXHAUSTED" || dt["kind"] != "infrastructure" {
		t.Fatalf("needs-owner detail = %v", dt)
	}
	*now = now.Add(time.Hour)
	if err := d.Tick(ctx); err != nil || d.Running() != 0 {
		t.Fatalf("exhausted ticket launched: %v", err)
	}
	// An operator release lets it run again with its debt kept: the next
	// infrastructure session holds again at once.
	raw, _ := json.Marshal(UnparkRequest{Unpark: retryKey})
	os.WriteFile(filepath.Join(d.dir, "requests", "u.json"), raw, 0o600)
	if err := d.Tick(ctx); err != nil || d.Running() != 1 {
		t.Fatalf("release: %v running %d", err, d.Running())
	}
	if e.State != InfraIdle || e.Charged != 2 {
		t.Fatalf("released episode = %+v", e)
	}
	q.obs.Tickets[0].Infrastructure = append(q.obs.Tickets[0].Infrastructure, d.ledger.Workers[0].ID)
	waitEnded(t, d)
	if err := d.Tick(ctx); err != nil || e.State != InfraExhausted || e.Charged != 2 || e.Sessions != 4 {
		t.Fatalf("release refilled the budget: %v %+v", err, e)
	}
}

// maxRetries 0 holds at the first infrastructure session; a native
// RETRY_EXHAUSTED plan reason is its own named hold.
func TestIssue502_InfraRetryDisabledAndNativeExhausted(t *testing.T) {
	d, q, _ := retryDispatcher(t, &InfraRetryConfig{MaxRetries: intp(0)})
	infraSession(t, d, q)
	if e := d.ledger.InfraRetry[retryKey]; e == nil || e.State != InfraDisabled || e.Charged != 0 || d.Running() != 0 {
		t.Fatalf("disabled episode = %+v running %d", e, d.Running())
	}
	if dt := lastDetail(t, d, "needs-owner"); dt["code"] != "INFRA_RETRY_DISABLED" {
		t.Fatalf("needs-owner detail = %v", dt)
	}

	d, q, _ = retryDispatcher(t, &InfraRetryConfig{})
	if err := d.Tick(context.Background()); err != nil || d.Running() != 1 {
		t.Fatalf("tick: %v", err)
	}
	q.obs.Tickets[0].Infrastructure = []string{d.ledger.Workers[0].ID}
	q.obs.Tickets[0].PlanReason = wire.CodeRetryExhausted
	waitEnded(t, d)
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if e := d.ledger.InfraRetry[retryKey]; e == nil || e.State != InfraNativeExhausted {
		t.Fatalf("native exhausted episode = %+v", e)
	}
	if !hasCode(t, d, "NATIVE_RETRY_EXHAUSTED") {
		t.Fatal("no NATIVE_RETRY_EXHAUSTED needs-owner event")
	}
}

// Another worker's request, a request without the policy, and a session
// whose escalation material is UNKNOWN are all ordinary no-progress.
func TestIssue502_InfraClassificationNeedsTheHoldersOwnRequest(t *testing.T) {
	for name, set := range map[string]func(*Dispatcher, *Ticket, string){
		"other holder": func(_ *Dispatcher, tk *Ticket, _ string) { tk.Infrastructure = []string{"prog.impl.1.other-1"} },
		"no policy": func(d *Dispatcher, tk *Ticket, id string) {
			d.Config.InfrastructureRetry, tk.Infrastructure = nil, []string{id}
		},
		"unknown material": func(_ *Dispatcher, tk *Ticket, id string) {
			tk.Infrastructure, tk.EscalationUnknown = []string{id}, true
		},
	} {
		t.Run(name, func(t *testing.T) {
			d, q, _ := retryDispatcher(t, &InfraRetryConfig{})
			if err := d.Tick(context.Background()); err != nil || d.Running() != 1 {
				t.Fatalf("tick: %v", err)
			}
			set(d, &q.obs.Tickets[0], d.ledger.Workers[0].ID)
			waitEnded(t, d)
			if err := d.Tick(context.Background()); err != nil {
				t.Fatal(err)
			}
			if b := d.ledger.Backoff[retryKey]; b == nil || !b.Parked || d.ledger.InfraRetry != nil {
				t.Fatalf("not ordinary no-progress: backoff %+v episodes %+v", b, d.ledger.InfraRetry)
			}
		})
	}
}

// ESC-V0-008: a session that ends behind a typed decision, scope or
// blocked hold is the typed policy's, so it neither counts nor parks.
func TestIssue502_HeldSessionIsNotFailureParked(t *testing.T) {
	c := ladderConfig(t)
	c.Backoff.ParkAfter = 1
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := d.Tick(context.Background()); err != nil || d.Running() != 1 {
		t.Fatalf("tick: %v", err)
	}
	q.obs.Tickets[0].EscalationPending = []string{"q-decide"}
	waitEnded(t, d)
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d.ledger.Backoff[retryKey] != nil || d.ledger.Escalation[retryKey] != nil || d.Running() != 0 {
		t.Fatalf("held session counted: backoff %+v ladder %+v running %d", d.ledger.Backoff[retryKey], d.ledger.Escalation[retryKey], d.Running())
	}
	if got := lastDetail(t, d, "finished")["session"]; got != SessionHeld {
		t.Fatalf("finished session = %q", got)
	}
}

// A typed hold outranks the session's own infrastructure request: a retry
// that raised both spends nothing, leaves no exhaustion hold behind the
// owner's answer, and launches once the hold is answered.
func TestIssue502_HoldOutranksInfrastructure(t *testing.T) {
	d, q, now := retryDispatcher(t, &InfraRetryConfig{MaxRetries: intp(1), CooldownSeconds: intp(1), MaxCooldownSeconds: intp(1)})
	ctx := context.Background()
	infraSession(t, d, q)
	*now = now.Add(time.Second)
	if err := d.Tick(ctx); err != nil || d.Running() != 1 {
		t.Fatalf("retry: %v running %d", err, d.Running())
	}
	retry := d.ledger.Workers[0].ID
	q.obs.Tickets[0].Infrastructure = append(q.obs.Tickets[0].Infrastructure, retry)
	q.obs.Tickets[0].EscalationPending = []string{"q-decide"}
	waitEnded(t, d)
	if err := d.Tick(ctx); err != nil || d.Running() != 0 {
		t.Fatalf("held tick: %v running %d", err, d.Running())
	}
	if got := lastDetail(t, d, "finished")["session"]; got != SessionHeld {
		t.Fatalf("finished session = %q", got)
	}
	e := d.ledger.InfraRetry[retryKey]
	if e.State != InfraIdle || e.Sessions != 1 || e.Charged != 1 || e.Launch != "" || d.ledger.Backoff[retryKey] != nil || d.ledger.Escalation[retryKey] != nil {
		t.Fatalf("held session accounted: episode %+v backoff %+v ladder %+v", e, d.ledger.Backoff[retryKey], d.ledger.Escalation[retryKey])
	}
	if hasCode(t, d, "INFRA_RETRY_EXHAUSTED") {
		t.Fatal("held session left an exhaustion hold")
	}
	q.obs.Tickets[0].EscalationPending = nil
	if err := d.Tick(ctx); err != nil || d.Running() != 1 {
		t.Fatalf("answered ticket did not launch: %v running %d", err, d.Running())
	}
	waitEnded(t, d)
}

// ESC-V0-008 with CAL-V0-057 tiers: an infrastructure session is unknown
// progress. It drops the proved failure suffix beyond the selected tier's
// threshold and keeps that tier, so the next failure does not climb.
func TestIssue502_InfraSessionKeepsTierAndResetsSuffix(t *testing.T) {
	c := ladderConfig(t)
	c.Roles[0].Escalate = []Tier{{After: 1, Model: "B"}, {After: 3, Model: "C"}}
	c.InfrastructureRetry = &InfraRetryConfig{CooldownSeconds: intp(1), MaxCooldownSeconds: intp(1)}
	tk := ticket("t1", "P1", 1)
	tk.AcceptanceRevision = "1"
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{tk}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	now := time.Now()
	d.Now = func() time.Time { return now }
	ctx := context.Background()
	var tiers []int
	run := func(infra bool) {
		t.Helper()
		if d.Running() == 0 {
			if err := d.Tick(ctx); err != nil {
				t.Fatal(err)
			}
		}
		if d.Running() != 1 {
			t.Fatalf("running %d", d.Running())
		}
		w := d.ledger.Workers[0]
		tiers = append(tiers, w.Tier)
		if infra {
			q.obs.Tickets[0].Infrastructure = append(q.obs.Tickets[0].Infrastructure, w.ID)
		}
		waitEnded(t, d)
		if err := d.Tick(ctx); err != nil {
			t.Fatal(err)
		}
		now = now.Add(2 * time.Second)
	}
	run(false) // streak 1: tier 1
	run(false) // streak 2: still tier 1
	run(true)  // unknown progress: streak back to 1, tier 1 kept
	if e := d.ledger.Escalation[retryKey]; e == nil || e.Streak != 1 {
		t.Fatalf("ladder after the infrastructure session = %+v", e)
	}
	run(false) // streak 2, not 3: no climb to tier 2
	run(false)
	if want := []int{0, 1, 1, 1, 1}; len(tiers) != len(want) || tiers[3] != 1 || tiers[4] != 1 || tiers[1] != 1 {
		t.Fatalf("launch tiers = %v, want %v", tiers, want)
	}
	if e := d.ledger.Escalation[retryKey]; e.Streak != 3 {
		t.Fatalf("streak = %d", e.Streak)
	}
}

// Restart safety: a reservation proved never spawned is republished under
// the same identity without another charge; one that may have spawned is
// UNKNOWN and holds; a reload narrows the limit but never refills it.
func TestIssue502_InfraReservationAcrossRestart(t *testing.T) {
	reopen := func(t *testing.T, d *Dispatcher, edit func(*InfraEpisode), policy *InfraRetryConfig) *Dispatcher {
		t.Helper()
		edit(d.ledger.InfraRetry[retryKey])
		if err := d.ledger.save(d.dir); err != nil {
			t.Fatal(err)
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
		d.Config.InfrastructureRetry = policy
		r, err := Open(d.Program, d.Config, d.Queue, io.Discard)
		if err != nil {
			t.Fatal("restart refused its own ledger:", err)
		}
		t.Cleanup(func() { _ = r.Close() })
		return r
	}
	policy := &InfraRetryConfig{MaxRetries: intp(3), CooldownSeconds: intp(1), MaxCooldownSeconds: intp(1)}
	t.Run("no spawn reuses the identity", func(t *testing.T) {
		d, q, _ := retryDispatcher(t, policy)
		infraSession(t, d, q)
		const reserved = "prog.impl.1.dead0000-99"
		r := reopen(t, d, func(e *InfraEpisode) {
			e.State, e.Launch, e.CooldownUntil = InfraReserved, reserved, time.Now().Add(-time.Second)
		}, policy)
		if e := r.ledger.InfraRetry[retryKey]; e.State != InfraWaiting || e.Launch != reserved || e.Charged != 1 {
			t.Fatalf("reconciled episode = %+v", e)
		}
		if err := r.Tick(context.Background()); err != nil || r.Running() != 1 || r.ledger.Workers[0].ID != reserved {
			t.Fatalf("republish: %v running %d", err, r.Running())
		}
		if e := r.ledger.InfraRetry[retryKey]; e.State != InfraRunning || e.Charged != 1 {
			t.Fatalf("republished episode = %+v", e)
		}
		waitEnded(t, r)
	})
	t.Run("possible spawn holds UNKNOWN", func(t *testing.T) {
		d, q, _ := retryDispatcher(t, policy)
		infraSession(t, d, q)
		const reserved = "prog.impl.1.dead0000-98"
		if err := os.MkdirAll(d.workerDir(reserved), 0o700); err != nil {
			t.Fatal(err)
		}
		r := reopen(t, d, func(e *InfraEpisode) {
			e.State, e.Launch, e.CooldownUntil = InfraReserved, reserved, time.Now().Add(-time.Second)
		}, policy)
		if e := r.ledger.InfraRetry[retryKey]; e.State != InfraUnknown || e.Launch != "" || e.Charged != 1 {
			t.Fatalf("reconciled episode = %+v", e)
		}
		if dt := lastDetail(t, r, "needs-owner"); dt["code"] != "INFRA_RETRY_UNKNOWN" {
			t.Fatalf("needs-owner detail = %v", dt)
		}
		if err := r.Tick(context.Background()); err != nil || r.Running() != 0 {
			t.Fatalf("UNKNOWN reservation replaced: %v running %d", err, r.Running())
		}
	})
	t.Run("slot churn keeps the reserved slot", func(t *testing.T) {
		d, q, _ := retryDispatcher(t, policy)
		infraSession(t, d, q)
		const reserved = "prog.impl.2.dead0000-97"
		r := reopen(t, d, func(e *InfraEpisode) {
			e.State, e.Launch, e.CooldownUntil = InfraReserved, reserved, time.Now().Add(-time.Second)
		}, policy)
		// Slot 1 is free and the roster picks it; the launch keeps slot 2.
		if err := r.Tick(context.Background()); err != nil || r.Running() != 1 || r.ledger.Workers[0].ID != reserved || r.ledger.Workers[0].Slot != 2 {
			t.Fatalf("republish: %v running %d workers %+v", err, r.Running(), r.ledger.Workers)
		}
		if e := r.ledger.InfraRetry[retryKey]; e.State != InfraRunning || e.Charged != 1 {
			t.Fatalf("republished episode = %+v", e)
		}
		waitEnded(t, r)
	})
	t.Run("another role holds instead of replacing", func(t *testing.T) {
		d, q, _ := retryDispatcher(t, policy)
		infraSession(t, d, q)
		const reserved = "prog.review.1.dead0000-96"
		r := reopen(t, d, func(e *InfraEpisode) {
			e.State, e.Launch, e.CooldownUntil = InfraReserved, reserved, time.Now().Add(-time.Second)
		}, policy)
		if err := r.Tick(context.Background()); err != nil || r.Running() != 0 {
			t.Fatalf("reserved identity replaced: %v running %d", err, r.Running())
		}
		if e := r.ledger.InfraRetry[retryKey]; e.State != InfraUnknown || e.Launch != "" || e.Charged != 1 {
			t.Fatalf("held episode = %+v", e)
		}
		if !hasCode(t, r, "INFRA_RETRY_UNKNOWN") {
			t.Fatal("no named hold")
		}
	})
	t.Run("reload narrows and never refills", func(t *testing.T) {
		d, q, _ := retryDispatcher(t, policy)
		infraSession(t, d, q)
		deadline := d.ledger.InfraRetry[retryKey].CooldownUntil
		r := reopen(t, d, func(*InfraEpisode) {}, &InfraRetryConfig{MaxRetries: intp(10), CooldownSeconds: intp(3600), MaxCooldownSeconds: intp(3600)})
		if e := r.ledger.InfraRetry[retryKey]; e.Limit != 3 || e.State != InfraWaiting || !e.CooldownUntil.Equal(deadline) {
			t.Fatalf("widened reload changed the episode: %+v", e)
		}
		r2 := reopen(t, r, func(*InfraEpisode) {}, &InfraRetryConfig{MaxRetries: intp(0)})
		if e := r2.ledger.InfraRetry[retryKey]; e.Limit != 0 || e.State != InfraDisabled {
			t.Fatalf("narrowed reload = %+v", e)
		}
		r3 := reopen(t, r2, func(*InfraEpisode) {}, nil)
		if e := r3.ledger.InfraRetry[retryKey]; e == nil || e.Limit != 0 {
			t.Fatalf("absent policy dropped the debt: %+v", e)
		}
	})
}

// reserveWitness reads the saved ledger at the launch fence, after the
// reservation and before any spawn, then refuses the launch there.
type reserveWitness struct {
	dir    string
	intent string
	saw    *InfraEpisode
	err    error
}

func (w *reserveWitness) Admit(intent string) (func(bool), error) {
	l, err := LoadLedger(w.dir, "prog")
	if err == nil {
		w.saw = l.InfraRetry[retryKey]
	}
	w.intent, w.err = intent, err
	return nil, errors.New("interrupted before spawn")
}

func (w *reserveWitness) Boundary(bool, bool) bool { return false }

// The reservation is in the saved ledger at the pre-spawn boundary, with
// its identity, charge and deadline; interrupted there, a reopen proves no
// spawn and launches that identity under the same charge.
func TestIssue502_InfraReservationSavedBeforeSpawn(t *testing.T) {
	d, q, now := retryDispatcher(t, &InfraRetryConfig{CooldownSeconds: intp(1), MaxCooldownSeconds: intp(1)})
	infraSession(t, d, q)
	deadline := d.ledger.InfraRetry[retryKey].CooldownUntil
	*now = now.Add(time.Second)
	w := &reserveWitness{dir: d.dir}
	d.control = w
	_ = d.Tick(context.Background())
	if w.err != nil || w.saw == nil || w.saw.State != InfraReserved || w.saw.Launch != w.intent || w.saw.Charged != 1 || !w.saw.CooldownUntil.Equal(deadline) {
		t.Fatalf("saved at the fence: %+v intent %q err %v", w.saw, w.intent, w.err)
	}
	if d.Running() != 0 {
		t.Fatal("spawned past a refused fence")
	}
	d.control = nil
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	r, err := Open(d.Program, d.Config, d.Queue, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	r.Now = d.Now
	if err := r.Tick(context.Background()); err != nil || r.Running() != 1 || r.ledger.Workers[0].ID != w.intent {
		t.Fatalf("republish: %v running %d", err, r.Running())
	}
	if e := r.ledger.InfraRetry[retryKey]; e.State != InfraRunning || e.Charged != 1 {
		t.Fatalf("republished episode = %+v", e)
	}
	waitEnded(t, r)
}

// A reservation that cannot be saved launches nothing; the next tick
// launches under the same identity and the same charge.
func TestIssue502_InfraReservationSaveFailure(t *testing.T) {
	d, q, now := retryDispatcher(t, &InfraRetryConfig{CooldownSeconds: intp(1), MaxCooldownSeconds: intp(1)})
	infraSession(t, d, q)
	*now = now.Add(time.Second)
	if err := os.Chmod(d.dir, 0o500); err != nil {
		t.Fatal(err)
	}
	_ = d.Tick(context.Background())
	if err := os.Chmod(d.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	e := d.ledger.InfraRetry[retryKey]
	if d.Running() != 0 || e.State != InfraWaiting || e.Launch == "" || e.Charged != 1 {
		t.Fatalf("unsaved reservation launched: running %d episode %+v", d.Running(), e)
	}
	reserved := e.Launch
	if err := d.Tick(context.Background()); err != nil || d.Running() != 1 || d.ledger.Workers[0].ID != reserved || e.Charged != 1 {
		t.Fatalf("republish: %v running %d episode %+v", err, d.Running(), e)
	}
	waitEnded(t, d)
}

// Checked progress marks the episode RECOVERED and keeps its debt; a new
// acceptance revision starts a new episode.
func TestIssue502_InfraRecoveredAndNewAcceptance(t *testing.T) {
	d, q, now := retryDispatcher(t, &InfraRetryConfig{CooldownSeconds: intp(1), MaxCooldownSeconds: intp(1)})
	infraSession(t, d, q)
	*now = now.Add(time.Second)
	if err := d.Tick(context.Background()); err != nil || d.Running() != 1 {
		t.Fatalf("retry: %v", err)
	}
	q.obs.Tickets[0].Revision = "2" // the retry made progress
	waitEnded(t, d)
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if e := d.ledger.InfraRetry[retryKey]; e.State != InfraRecovered || e.Charged != 1 || e.Launch != "" {
		t.Fatalf("recovered episode = %+v", e)
	}
	waitEnded(t, d)
	q.obs.Tickets[0].AcceptanceRevision = "2"
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d.ledger.InfraRetry != nil {
		t.Fatalf("old acceptance episode kept: %+v", d.ledger.InfraRetry)
	}
	waitEnded(t, d)
}

// The episode survives the strict reader alone; malformed episodes refuse.
func TestIssue502_LedgerRefusesMalformedInfraRetry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	l := &Ledger{Profile: StateProfile, Program: "prog", Workers: []*Worker{}, Backoff: map[string]*BackoffState{},
		InfraRetry: map[string]*InfraEpisode{retryKey: {AcceptanceRevision: "1", State: InfraRunning, Sessions: 1, Charged: 1, Limit: 3, CooldownUntil: time.Unix(1700000000, 0).UTC(), Launch: "prog.impl.1.ab-3"}}}
	good, err := ledgerBytes(l)
	if err != nil {
		t.Fatal(err)
	}
	valid := string(good)
	if err := os.WriteFile(path, good, 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := LoadLedger(dir, "prog"); err != nil || got.InfraRetry[retryKey].Launch != "prog.impl.1.ab-3" {
		t.Fatal("valid ledger refused:", err)
	}
	for name, bad := range map[string]string{
		"alias":          strings.Replace(valid, `"infraRetry":`, `"InfraRetry":`, 1),
		"duplicate":      strings.Replace(valid, `"infraRetry": {`, `"infraRetry": {}, "infraRetry": {`, 1),
		"null":           strings.Replace(valid, `"infraRetry": {`, `"infraRetry": null, "x": {`, 1),
		"unknown field":  strings.Replace(valid, `"sessions":`, `"extra": 1, "sessions":`, 1),
		"member alias":   strings.Replace(valid, `"charged":`, `"Charged":`, 1),
		"state":          strings.Replace(valid, `"RUNNING"`, `"BOGUS"`, 1),
		"running no id":  strings.Replace(valid, `"launch": "prog.impl.1.ab-3"`, `"launch": ""`, 1),
		"charged > 10":   strings.Replace(valid, `"charged": 1`, `"charged": 11`, 1),
		"no sessions":    strings.Replace(valid, `"sessions": 1`, `"sessions": 0`, 1),
		"bad key":        strings.Replace(valid, `"`+retryKey+`": {`, `"t1": {`, 1),
		"bad acceptance": strings.Replace(valid, `"acceptanceRevision": "1"`, `"acceptanceRevision": "01"`, 1),
		"empty":          strings.Replace(valid, `"infraRetry": {`, `"infraRetry": {}, "y": {`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if bad == valid {
				t.Fatal("malformed fixture not reached")
			}
			if err := os.WriteFile(path, []byte(bad), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadLedger(dir, "prog"); err == nil {
				t.Fatal("malformed infraRetry admitted")
			}
		})
	}
	// An omitted or null member, or retry state without its charge or
	// deadline, would decode as spendable debt or an elapsed cooldown.
	edited := func(edit func(map[string]any)) string {
		var doc map[string]any
		if err := json.Unmarshal(good, &doc); err != nil {
			t.Fatal(err)
		}
		edit(doc["infraRetry"].(map[string]any)[retryKey].(map[string]any))
		raw, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	cases := map[string]string{
		"waiting uncharged": edited(func(e map[string]any) { e["state"], e["charged"] = InfraWaiting, 0; delete(e, "launch") }),
		"waiting over limit": edited(func(e map[string]any) {
			e["state"], e["charged"], e["sessions"], e["limit"] = InfraWaiting, 3, 3, 2
			delete(e, "launch")
		}),
		"running no deadline":   edited(func(e map[string]any) { e["cooldownUntil"] = "0001-01-01T00:00:00Z" }),
		"reserved uncharged":    edited(func(e map[string]any) { e["state"], e["charged"] = InfraReserved, 0 }),
		"charged over sessions": edited(func(e map[string]any) { e["charged"] = 2 }),
	}
	for _, field := range []string{"acceptanceRevision", "state", "sessions", "charged", "limit", "cooldownUntil", "launch"} {
		cases["null "+field] = edited(func(e map[string]any) { e[field] = nil })
		if field != "launch" {
			cases["omitted "+field] = edited(func(e map[string]any) { delete(e, field) })
		}
	}
	for name, bad := range cases {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(bad), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadLedger(dir, "prog"); err == nil {
				t.Fatal("malformed infraRetry admitted")
			}
		})
	}
	// An idle episode keeps debt without a pending retry, and a disabled one
	// never charged; both load.
	for name, ok := range map[string]string{
		"idle": edited(func(e map[string]any) { e["state"] = InfraIdle; delete(e, "launch") }),
		"disabled": edited(func(e map[string]any) {
			e["state"], e["charged"], e["limit"], e["cooldownUntil"] = InfraDisabled, 0, 0, "0001-01-01T00:00:00Z"
			delete(e, "launch")
		}),
	} {
		if err := os.WriteFile(path, []byte(ok), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadLedger(dir, "prog"); err != nil {
			t.Fatalf("%s episode refused: %v", name, err)
		}
	}
}
