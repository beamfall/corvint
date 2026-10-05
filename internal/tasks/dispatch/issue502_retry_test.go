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
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// issue502 is a one-ticket dispatcher with a controllable clock and an
// infrastructure retry policy of maxRetries (nil policy when max < 0).
type issue502 struct {
	t     *testing.T
	c     *Config
	q     *fakeQueue
	d     *Dispatcher
	clock time.Time
	reqs  int
}

const issue502Key = "ticket:a:q:t1"

func newIssue502(t *testing.T, c *Config, max int) *issue502 {
	t.Helper()
	if max >= 0 {
		c.InfrastructureRetry = &InfrastructureRetry{MaxRetries: &max, CooldownSeconds: 1, MaxCooldownSeconds: 4}
	}
	tk := ticket("t1", "P1", 1)
	tk.AcceptanceRevision = "1"
	x := &issue502{t: t, c: c, q: &fakeQueue{obs: Observation{Tickets: []Ticket{tk}}}, clock: time.Now().UTC()}
	x.open()
	return x
}

func (x *issue502) open() {
	x.t.Helper()
	d, err := Open("prog", x.c, x.q, io.Discard)
	if err != nil {
		x.t.Fatal(err)
	}
	d.Now = func() time.Time { return x.clock }
	x.d = d
}

func (x *issue502) tick(running int) *Worker {
	x.t.Helper()
	if err := x.d.Tick(context.Background()); err != nil {
		x.t.Fatalf("tick: %v", err)
	}
	if x.d.Running() != running {
		x.t.Fatalf("running %d, want %d; events %v", x.d.Running(), running, kinds(x.t, x.d))
	}
	if running == 0 {
		return nil
	}
	return x.d.ledger.Workers[0]
}

// raise records a typed request the running worker raised itself.
func (x *issue502) raise(w *Worker, kind string) string {
	x.reqs++
	id := "r" + string(rune('0'+x.reqs))
	x.q.obs.Tickets[0].Requests = append(x.q.obs.Tickets[0].Requests, EscalationRequest{ID: id, Kind: kind, State: "OPEN", Holder: w.ID, Opened: x.clock})
	return id
}

// session launches one worker, lets the hook act while it runs, and ends it.
func (x *issue502) session(during func(w *Worker)) *Worker {
	x.t.Helper()
	w := x.tick(1)
	if during != nil {
		during(w)
	}
	waitEnded(x.t, x.d)
	return w
}

func (x *issue502) episode() *InfraRetry { return x.d.ledger.InfraRetry[issue502Key] }

func (x *issue502) finishedSession(w *Worker) string {
	for _, e := range eventsOf(x.t, x.d, "finished") {
		if e.Worker == w.ID {
			return e.Detail["session"]
		}
	}
	x.t.Fatalf("no finished event for %s", w.ID)
	return ""
}

// ESC-V0-008: an infrastructure session neither counts toward nor causes
// parking, and keeps the no-progress count it found; a later ordinary
// failure resumes counting from there.
func TestESCV0008_InfrastructureSessionNeitherCountsNorParks(t *testing.T) {
	x := newIssue502(t, testConfig(t, "exit 0"), 3)
	defer x.d.Close()
	x.session(nil)
	x.tick(1) // ordinary failure counted, next launched
	if b := x.d.ledger.Backoff[issue502Key]; b == nil || b.NoProgress != 1 || b.Parked {
		t.Fatalf("ordinary failure not counted: %+v", b)
	}
	w := x.d.ledger.Workers[0]
	x.raise(w, "infrastructure")
	waitEnded(t, x.d)
	x.tick(0) // RETRY_WAIT holds the launch until the cooldown
	if b := x.d.ledger.Backoff[issue502Key]; b == nil || b.NoProgress != 1 || b.Parked {
		t.Fatalf("infrastructure session counted or parked: %+v", b)
	}
	if got := x.finishedSession(w); got != "infrastructure" {
		t.Fatalf("finished session = %q", got)
	}
	if e := x.episode(); e == nil || e.State != InfraWait || e.Count != 0 || !reflect.DeepEqual(e.Requests, []string{"r1"}) || !e.NextEligible.Equal(x.clock.Add(time.Second)) {
		t.Fatalf("episode = %+v", e)
	}
	x.clock = x.clock.Add(2 * time.Second)
	w = x.tick(1) // the reserved retry
	if e := x.episode(); e.State != InfraRetrying || e.Count != 1 || e.Pending == nil || e.Pending.Worker != w.ID {
		t.Fatalf("retry not reserved: %+v", e)
	}
	waitEnded(t, x.d)
	x.tick(0) // an ordinary failure parks at ParkAfter 2: the infra session did not reset the count
	if b := x.d.ledger.Backoff[issue502Key]; b == nil || !b.Parked || b.NoProgress != 2 {
		t.Fatalf("ordinary failure after infra did not resume counting: %+v", b)
	}
	if e := x.episode(); e.State != InfraReady || e.Count != 1 || e.Pending != nil {
		t.Fatalf("settled retry = %+v", e)
	}
}

// ESC-V0-008: decision, scope and blocked sessions are held by their typed
// request, never by failure parking, and leave the ladder untouched.
func TestESCV0008_HeldSessionsDoNotPark(t *testing.T) {
	for _, kind := range []string{"decision", "scope", "blocked"} {
		c := ladderConfig(t)
		c.Backoff.ParkAfter = 1
		x := newIssue502(t, c, 3)
		w := x.session(func(w *Worker) { x.raise(w, kind) })
		x.q.obs.Tickets[0].Requests[0].State = "ANSWERED" // an answer releases the native hold
		w2 := x.tick(1)
		if b := x.d.ledger.Backoff[issue502Key]; b != nil {
			t.Fatalf("%s: session counted: %+v", kind, b)
		}
		if got := x.finishedSession(w); got != "held" {
			t.Fatalf("%s: finished session = %q", kind, got)
		}
		if w2.Tier != 0 || x.d.ledger.Escalation[issue502Key] != nil || x.episode() != nil {
			t.Fatalf("%s: held session touched the ladder or retry state: tier %d %+v %+v", kind, w2.Tier, x.d.ledger.Escalation, x.episode())
		}
		waitEnded(t, x.d)
		x.tick(0) // the next worker raised nothing: ordinary accounting parks
		if b := x.d.ledger.Backoff[issue502Key]; b == nil || !b.Parked {
			t.Fatalf("%s: later ordinary failure did not park: %+v", kind, b)
		}
		x.d.Close()
	}
}

// ESC-V0-008 mixed 499 tier witness: an infrastructure session resets the
// proved failure streak but keeps the reached tier as a floor and the
// no-progress count; ordinary failures then resume to parking at the top.
func TestESCV0008_MixedTierWitness(t *testing.T) {
	c := ladderConfig(t)
	x := newIssue502(t, c, 3)
	defer x.d.Close()
	var models []string
	step := func(kind string) {
		t.Helper()
		w := x.tick(1)
		if kind != "" {
			x.raise(w, kind)
		}
		waitEnded(t, x.d)
		models = append(models, readModel(t, c, w)+"@"+string(rune('0'+w.Tier)))
		x.clock = x.clock.Add(10 * time.Second)
	}
	step("")
	step("")
	step("infrastructure") // at C@2: streak reset, tier retained
	x.tick(0)              // the retry waits for its cooldown
	if e := x.d.ledger.Escalation[issue502Key]; e == nil || e.Streak != 0 || !e.RetainTiers || e.Tiers["impl"] != 2 {
		t.Fatalf("escalation after infra = %+v", e)
	}
	if b := x.d.ledger.Backoff[issue502Key]; b == nil || b.NoProgress != 2 {
		t.Fatalf("infra session changed the no-progress count: %+v", b)
	}
	x.clock = x.clock.Add(10 * time.Second)
	step("")
	step("")
	step("")
	x.tick(0)
	if want := []string{"A@0", "B@1", "C@2", "C@2", "C@2", "C@2"}; !reflect.DeepEqual(models, want) {
		t.Fatalf("models = %v, want %v", models, want)
	}
	if b := x.d.ledger.Backoff[issue502Key]; b == nil || !b.Parked || b.NoProgress != 5 {
		t.Fatalf("ordinary failures did not resume to parking: %+v", b)
	}
	// Progress clears the retained floor with the streak.
	x.d.account(issue502Key, true)
	if e := x.d.ledger.Escalation[issue502Key]; e != nil && e.RetainTiers {
		t.Fatalf("progress kept the retained floor: %+v", e)
	}
}

// ESC-V0-007: the final allowed retry launches, the next is EXHAUSTED and
// nothing further launches automatically; the request stays as observed.
func TestESCV0007_RetryBoundExhausts(t *testing.T) {
	x := newIssue502(t, testConfig(t, "exit 0"), 2)
	defer x.d.Close()
	for i := 0; i < 3; i++ {
		x.session(func(w *Worker) { x.raise(w, "infrastructure") })
		x.tick(0) // charged; the next retry waits for its cooldown
		x.clock = x.clock.Add(10 * time.Second)
	}
	e := x.episode()
	if e.State != InfraExhausted || e.Reason != InfraRetryExhausted || e.Count != 2 || len(e.Sessions) != 3 {
		t.Fatalf("episode = %+v", e)
	}
	x.clock = x.clock.Add(time.Hour)
	x.tick(0)
	if x.d.ledger.Backoff[issue502Key] != nil {
		t.Fatal("exhaustion parked the ticket")
	}
	ev := eventsOf(t, x.d, "infra-retry")
	if last := ev[len(ev)-1]; last.Detail["reason"] != InfraRetryExhausted || !strings.Contains(last.Message, "native request stays OPEN") {
		t.Fatalf("last infra-retry event = %+v", last)
	}
	// A wider policy on reload never refills an exhausted episode.
	x.d.Close()
	wide := 5
	x.c.InfrastructureRetry.MaxRetries = &wide
	x.open()
	x.tick(0)
}

// ESC-V0-007: a disabled or absent policy holds after the first
// infrastructure session; a native RETRY_EXHAUSTED plan reason is named.
func TestESCV0007_DisabledAbsentAndNativeExhaustion(t *testing.T) {
	for name, tc := range map[string]struct {
		max    int
		reason string
		native bool
	}{
		"disabled": {0, InfraRetryDisabled, false},
		"absent":   {-1, InfraRetryDisabled, false},
		"native":   {3, NativeRetryExhausted, true},
	} {
		x := newIssue502(t, testConfig(t, "exit 0"), tc.max)
		if tc.native {
			x.q.obs.Tickets[0].PlanReason = wire.CodeRetryExhausted
		}
		x.session(func(w *Worker) { x.raise(w, "infrastructure") })
		x.tick(0)
		x.clock = x.clock.Add(time.Hour)
		x.tick(0)
		if e := x.episode(); e == nil || e.State != InfraExhausted || e.Reason != tc.reason || x.d.ledger.Backoff[issue502Key] != nil {
			t.Fatalf("%s: episode %+v backoff %+v", name, e, x.d.ledger.Backoff[issue502Key])
		}
		x.d.Close()
	}
}

func TestESCV0007_CooldownDoublesToCap(t *testing.T) {
	max := 10
	r := &InfrastructureRetry{MaxRetries: &max, CooldownSeconds: 30, MaxCooldownSeconds: 300}
	var got []time.Duration
	for n := 1; n <= 6; n++ {
		got = append(got, r.RetryCooldown(n))
	}
	want := []time.Duration{30 * time.Second, 60 * time.Second, 120 * time.Second, 240 * time.Second, 300 * time.Second, 300 * time.Second}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cooldowns = %v", got)
	}
	m, cd, mx := (&InfrastructureRetry{}).Limits()
	if m != 3 || cd != 30*time.Second || mx != 300*time.Second {
		t.Fatalf("defaults = %d %v %v", m, cd, mx)
	}
	if m, _, _ := (*InfrastructureRetry)(nil).Limits(); m != 0 {
		t.Fatalf("absent policy allows %d retries", m)
	}
	if _, _, mx := (&InfrastructureRetry{CooldownSeconds: 600}).Limits(); mx != 600*time.Second {
		t.Fatalf("cap below cooldown: %v", mx)
	}
}

func TestESCV0007_ConfigBounds(t *testing.T) {
	ok := testConfig(t, "exit 0")
	three := 3
	ok.InfrastructureRetry = &InfrastructureRetry{MaxRetries: &three, CooldownSeconds: 30, MaxCooldownSeconds: 300}
	raw, _ := json.Marshal(ok)
	if _, err := DecodeConfig(raw); err != nil {
		t.Fatalf("valid policy refused: %v", err)
	}
	for name, body := range map[string]string{
		"eleven retries":     `{"maxRetries":11}`,
		"negative retries":   `{"maxRetries":-1}`,
		"cooldown too long":  `{"cooldownSeconds":3601}`,
		"cap below cooldown": `{"cooldownSeconds":60,"maxCooldownSeconds":30}`,
		"cap too long":       `{"maxCooldownSeconds":86401}`,
		"unknown field":      `{"maxRetries":1,"jitter":true}`,
		// Seconds that overflow time.Duration must be refused before conversion.
		"cooldown overflows": `{"cooldownSeconds":9223372037}`,
		"cap overflows":      `{"cooldownSeconds":60,"maxCooldownSeconds":9223372037}`,
	} {
		c := testConfig(t, "exit 0")
		raw, _ := json.Marshal(c)
		raw = append(raw[:len(raw)-1], []byte(`,"infrastructureRetry":`+body+`}`)...)
		if _, err := DecodeConfig(raw); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// ESC-V0-007: a narrower policy holds a waiting retry without refilling or
// discarding its charged count.
func TestESCV0007_ReloadNarrows(t *testing.T) {
	x := newIssue502(t, testConfig(t, "exit 0"), 3)
	defer x.d.Close()
	x.session(func(w *Worker) { x.raise(w, "infrastructure") })
	x.tick(0)
	x.clock = x.clock.Add(10 * time.Second)
	w := x.tick(1)
	x.raise(w, "infrastructure")
	waitEnded(t, x.d)
	x.tick(0)
	if e := x.episode(); e.State != InfraWait || e.Count != 1 {
		t.Fatalf("episode = %+v", e)
	}
	one := 1
	x.c.InfrastructureRetry.MaxRetries = &one
	x.clock = x.clock.Add(10 * time.Second)
	x.tick(0)
	if e := x.episode(); e.State != InfraExhausted || e.Reason != InfraRetryExhausted || e.Count != 1 || e.Pending != nil {
		t.Fatalf("narrowed episode = %+v", e)
	}
}

// ESC-V0-007: an ended session is charged once.
func TestESCV0007_DuplicateSessionCountsOnce(t *testing.T) {
	x := newIssue502(t, testConfig(t, "exit 0"), 3)
	defer x.d.Close()
	tk := &x.q.obs.Tickets[0]
	w := &Worker{ID: "prog.impl.1.n-1", Key: issue502Key, Role: "impl"}
	x.d.infraEnded(tk, w, []string{"r1"}, "fp", x.clock)
	x.d.infraEnded(tk, w, []string{"r1"}, "fp", x.clock)
	if e := x.episode(); len(e.Sessions) != 1 || len(eventsOf(t, x.d, "infra-retry")) != 1 {
		t.Fatalf("duplicate session charged twice: %+v", e)
	}
}

// ESC-V0-007: a reservation that cannot be saved launches nothing and
// leaves the episode as it was.
func TestESCV0007_FailedReservationSaveLaunchesNothing(t *testing.T) {
	x := newIssue502(t, testConfig(t, "exit 0"), 3)
	defer x.d.Close()
	x.session(func(w *Worker) { x.raise(w, "infrastructure") })
	x.tick(0)
	before := *x.episode()
	x.clock = x.clock.Add(10 * time.Second)
	if err := os.Chmod(x.d.dir, 0o500); err != nil {
		t.Fatal(err)
	}
	x.d.Tick(context.Background())
	os.Chmod(x.d.dir, 0o700)
	if x.d.Running() != 0 {
		t.Fatal("retry launched without a saved reservation")
	}
	if e := x.episode(); !reflect.DeepEqual(*e, before) {
		t.Fatalf("episode changed: %+v, was %+v", e, before)
	}
	if w := x.tick(1); x.episode().Pending.Worker != w.ID || x.episode().Count != 1 {
		t.Fatalf("retry after repair = %+v", x.episode())
	}
}

// ESC-V0-007: a restart before spawn reuses the reservation without a new
// charge; a reservation whose worker directory exists is UNKNOWN.
func TestESCV0007_RestartResolvesReservations(t *testing.T) {
	for _, spawned := range []bool{false, true} {
		x := newIssue502(t, testConfig(t, "exit 0"), 3)
		id := "prog.impl.1.old-7"
		x.d.ledger.InfraRetry = map[string]*InfraRetry{issue502Key: {AcceptanceRevision: "1", State: InfraRetrying, Count: 1, Sessions: []string{"prog.impl.1.old-1"}, Requests: []string{"r1"}, Pending: &InfraReservation{Worker: id, Ordinal: 1, Deadline: x.clock}}}
		if err := x.d.ledger.save(x.d.dir); err != nil {
			t.Fatal(err)
		}
		if spawned {
			os.MkdirAll(x.d.workerDir(id), 0o700)
		}
		x.d.Close()
		x.open()
		if !spawned {
			w := x.tick(1)
			if e := x.episode(); w.ID != id || e.Count != 1 || e.State != InfraRetrying {
				t.Fatalf("reservation not reused: worker %s %+v", w.ID, e)
			}
		} else {
			x.tick(0)
			if e := x.episode(); e.State != InfraUnknown || e.Reason != ReservationUnresolved || e.Count != 1 {
				t.Fatalf("ambiguous reservation = %+v", e)
			}
		}
		x.d.Close()
	}
}

// ESC-V0-007: a reservation restored after a restart is held by a policy
// narrowed below its ordinal; it launches nothing and keeps its charge.
func TestESCV0007_NarrowedPolicyHoldsAReservedRetry(t *testing.T) {
	x := newIssue502(t, testConfig(t, "exit 0"), 3)
	id := "prog.impl.1.old-7"
	x.d.ledger.InfraRetry = map[string]*InfraRetry{issue502Key: {AcceptanceRevision: "1", State: InfraRetrying, Count: 2, Sessions: []string{"prog.impl.1.old-1"}, Requests: []string{"r1"}, Pending: &InfraReservation{Worker: id, Ordinal: 2, Deadline: x.clock}}}
	if err := x.d.ledger.save(x.d.dir); err != nil {
		t.Fatal(err)
	}
	x.d.Close()
	zero := 0
	x.c.InfrastructureRetry.MaxRetries = &zero
	x.open()
	defer x.d.Close()
	x.tick(0)
	if e := x.episode(); e.State != InfraExhausted || e.Reason != InfraRetryDisabled || e.Count != 2 || e.Pending != nil {
		t.Fatalf("narrowed reservation = %+v", e)
	}
}

// ESC-V0-007: progress (in a session or outside one) marks the episode
// RECOVERED and resets its debt; an acceptance change starts a new one.
func TestESCV0007_ProgressRecoversAndAcceptanceResets(t *testing.T) {
	x := newIssue502(t, testConfig(t, "exit 0"), 3)
	defer x.d.Close()
	x.session(func(w *Worker) { x.raise(w, "infrastructure") })
	x.tick(0)
	x.clock = x.clock.Add(10 * time.Second)
	x.session(func(w *Worker) { x.q.obs.Tickets[0].Revision = "2" })
	x.tick(1)
	if e := x.episode(); e.State != InfraRecovered || e.Count != 0 || e.Pending != nil {
		t.Fatalf("progress did not recover: %+v", e)
	}
	waitEnded(t, x.d)
	// An operator change while waiting recovers through the fingerprint.
	w := x.tick(1)
	x.raise(w, "infrastructure")
	waitEnded(t, x.d)
	x.tick(0)
	if e := x.episode(); e.State != InfraWait || e.Count != 0 {
		t.Fatalf("episode after recovery = %+v", e)
	}
	x.q.obs.Tickets[0].Revision = "3"
	x.tick(1)
	if e := x.episode(); e.State != InfraRecovered {
		t.Fatalf("outside change did not recover: %+v", e)
	}
	waitEnded(t, x.d)
	x.q.obs.Tickets[0].AcceptanceRevision = "2"
	x.d.ledger.InfraRetry[issue502Key].State = InfraExhausted
	if err := x.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if e := x.episode(); e != nil {
		t.Fatalf("acceptance change kept the episode: %+v", e)
	}
}

func TestESCV0007_LedgerInfraRetryIsStrict(t *testing.T) {
	dir := t.TempDir()
	write := func(s string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	head := `{"profile":"` + StateProfile + `","program":"prog","launchSeq":0,"eventSeq":0,"workers":[],"backoff":{}`
	valid := `"ticket:a:q:t1":{"acceptanceRevision":"1","state":"RETRYING","count":1,"sessions":["w1"],"requests":["r1"],"pending":{"worker":"w2","ordinal":1,"deadline":"2026-10-05T00:00:00Z"}}`
	write(head + `,"escalation":{"ticket:a:q:t1":{"streak":0,"tiers":{"impl":1},"retainTiers":true}},"infraRetry":{` + valid + `}}`)
	l, err := LoadLedger(dir, "prog")
	if err != nil || l.InfraRetry["ticket:a:q:t1"].Pending.Worker != "w2" || !l.Escalation["ticket:a:q:t1"].RetainTiers {
		t.Fatalf("valid ledger refused: %v", err)
	}
	for name, body := range map[string]string{
		"unknown state":       `{"acceptanceRevision":"1","state":"BOGUS","count":0,"sessions":[]}`,
		"count eleven":        `{"acceptanceRevision":"1","state":"READY","count":11,"sessions":[]}`,
		"retrying unreserved": `{"acceptanceRevision":"1","state":"RETRYING","count":1,"sessions":[]}`,
		"pending when ready":  `{"acceptanceRevision":"1","state":"READY","count":1,"sessions":[],"pending":{"worker":"w","ordinal":1,"deadline":"2026-10-05T00:00:00Z"}}`,
		"ordinal mismatch":    `{"acceptanceRevision":"1","state":"RETRYING","count":2,"sessions":[],"pending":{"worker":"w","ordinal":1,"deadline":"2026-10-05T00:00:00Z"}}`,
		"no acceptance":       `{"acceptanceRevision":"","state":"READY","count":0,"sessions":[]}`,
		"duplicate session":   `{"acceptanceRevision":"1","state":"READY","count":0,"sessions":["w","w"]}`,
		"bad reason":          `{"acceptanceRevision":"1","state":"EXHAUSTED","count":0,"sessions":[],"reason":"TIRED"}`,
		"bad fingerprint":     `{"acceptanceRevision":"1","state":"READY","count":0,"sessions":[],"fingerprint":"x"}`,
		"unknown field":       `{"acceptanceRevision":"1","state":"READY","count":0,"sessions":[],"extra":1}`,
		"alias":               `{"AcceptanceRevision":"1","state":"READY","count":0,"sessions":[]}`,
	} {
		write(head + `,"infraRetry":{"ticket:a:q:t1":` + body + `}}`)
		if _, err := LoadLedger(dir, "prog"); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	write(head + `,"infraRetry":{"t1":{"acceptanceRevision":"1","state":"READY","count":0,"sessions":[]}}}`)
	if _, err := LoadLedger(dir, "prog"); err == nil {
		t.Error("bad key accepted")
	}
	write(head + `,"infraRetry":{},"escalation":{"ticket:a:q:t1":{"streak":0,"retainTiers":1}}}`)
	if _, err := LoadLedger(dir, "prog"); err == nil {
		t.Error("non-boolean retainTiers accepted")
	}
}

// ESC-V0-008: unreadable escalation material is UNKNOWN, never progress: a
// changed revision under it neither recovers the episode nor counts as a
// session's progress.
func TestESCV0008_UnknownMaterialIsNotProgress(t *testing.T) {
	x := newIssue502(t, testConfig(t, "exit 0"), 3)
	defer x.d.Close()
	x.session(func(w *Worker) { x.raise(w, "infrastructure") })
	x.tick(0)
	x.q.obs.Tickets[0].Revision, x.q.obs.Tickets[0].EscalationUnknown = "9", true
	x.tick(0)
	if e := x.episode(); e.State != InfraWait {
		t.Fatalf("unknown material recovered the episode: %+v", e)
	}
	x.clock = x.clock.Add(10 * time.Second)
	x.session(nil)
	if err := x.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, e := range eventsOf(t, x.d, "finished") {
		if e.Detail["progress"] == "true" {
			t.Fatalf("unknown material counted as progress: %+v", e)
		}
	}
}

// ESC-V0-007: a non-parked backoff left at an older fingerprint by an
// ordinary failure never recovers a later infrastructure episode; recovery
// compares the episode's own baseline, so the retry stays bounded.
func TestESCV0007_StaleBackoffDoesNotRecoverAnEpisode(t *testing.T) {
	c := testConfig(t, "exit 0")
	c.Backoff.CooldownSeconds = 60
	x := newIssue502(t, c, 3)
	defer x.d.Close()
	x.session(nil)
	x.tick(0) // an ordinary failure: the backoff records fingerprint A and cools down
	if b := x.d.ledger.Backoff[issue502Key]; b == nil || b.NoProgress != 1 || b.Parked {
		t.Fatalf("ordinary failure not counted: %+v", b)
	}
	x.q.obs.Tickets[0].Revision = "2" // an outside change to B
	x.tick(0)
	x.clock = x.clock.Add(61 * time.Second)
	x.session(func(w *Worker) { x.raise(w, "infrastructure") })
	for range 3 {
		x.tick(0)
		if e := x.episode(); e == nil || e.State != InfraWait || e.Fingerprint == "" {
			t.Fatalf("stale backoff fingerprint recovered the episode: %+v", e)
		}
	}
}

// ESC-V0-007: a ticket's first progress token rewraps the episode baseline
// as it does the worker and backoff baselines, so it grants no progress
// credit and leaves a waiting episode's debt in place.
func TestESCV0007_FirstProgressTokenDoesNotRecover(t *testing.T) {
	c := testConfig(t, "exit 0")
	source := filepath.Join(c.WorkRoot, "states.json")
	c.WorkState = &WorkState{Kind: "command", Argv: []string{"/bin/cat", source}}
	state := func(body string) {
		t.Helper()
		if err := os.WriteFile(source, []byte(`{"t1":`+body+`}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	state(`{"state":"work"}`)
	x := newIssue502(t, c, 3)
	defer x.d.Close()
	x.session(func(w *Worker) { x.raise(w, "infrastructure") })
	x.tick(0)
	if e := x.episode(); e == nil || e.State != InfraWait {
		t.Fatalf("episode = %+v", e)
	}
	state(`{"state":"work","progress":"first"}`)
	x.tick(0)
	if e := x.episode(); e.State != InfraWait || e.Fingerprint == "" || x.d.ledger.Progress[issue502Key] == nil {
		t.Fatalf("first progress token recovered the episode: %+v", e)
	}
	state(`{"state":"work","progress":"second"}`)
	x.tick(1)
	if e := x.episode(); e.State != InfraRecovered {
		t.Fatalf("a later token is progress: %+v", e)
	}
}
