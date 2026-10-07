//go:build darwin || linux

package dispatch

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestDetachedRunHelper is not a test: run with CORVINT_TEST_DETACHED_HELPER
// it is a detached run supervisor stand-in. It leads its own session, starts
// a command in its own process group inside that session, writes
// "supervisor command" pids to the named file and sleeps.
func TestDetachedRunHelper(t *testing.T) {
	out := os.Getenv("CORVINT_TEST_DETACHED_HELPER")
	if out == "" {
		t.Skip("helper process only")
	}
	if _, err := syscall.Setsid(); err != nil {
		os.Exit(3)
	}
	cmd := exec.Command("sleep", "300")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		os.Exit(4)
	}
	_ = os.WriteFile(out+".tmp", []byte(strconv.Itoa(os.Getpid())+" "+strconv.Itoa(cmd.Process.Pid)), 0o600)
	_ = os.Rename(out+".tmp", out)
	time.Sleep(300 * time.Second)
	os.Exit(0)
}

// runQueue is a fakeQueue that also reads detached run records.
type runQueue struct {
	*fakeQueue
	mu   sync.Mutex
	runs map[string][]DetachedRun
}

func (q *runQueue) AttemptRuns(id string) ([]DetachedRun, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return append([]DetachedRun(nil), q.runs[id]...), nil
}

func (q *runQueue) AttemptRun(id, run string) (*DetachedRun, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, r := range q.runs[id] {
		if r.RunID == run {
			return &r, nil
		}
	}
	return nil, nil
}

func (q *runQueue) set(id string, runs ...DetachedRun) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.runs[id] = runs
}

const testRunID = "0123456789abcdef"

// detachedScript: the first session starts a supervisor stand-in, then
// exits once the test creates <worker>.go; a relaunched session records its
// prompt and run environment and exits. Later ordinary sessions start no
// stand-in, so none can outlive the test's cleanup.
func detachedScript(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return `if [ -n "$CORVINT_DISPATCH_RUN_ID" ]; then
  printf '%s\n' "$1" > "$CORVINT_DISPATCH_WORKER.prompt"
  env | grep '^CORVINT_DISPATCH_RUN_' | sort > "$CORVINT_DISPATCH_WORKER.env"
  exit 0
fi
if mkdir .helper-once 2>/dev/null; then
  CORVINT_TEST_DETACHED_HELPER="$PWD/$CORVINT_DISPATCH_WORKER.sup" '` + exe + `' -test.run='^TestDetachedRunHelper$' >/dev/null 2>&1 &
fi
while [ ! -f "$CORVINT_DISPATCH_WORKER.go" ]; do sleep 0.05; done
exit 0`
}

// detachedRig launches one worker on t1, lets it claim, waits for its
// supervisor stand-in and ticks once more so the supervisor is recorded in
// the worker's tree, as the launcher's child is today.
type detachedRig struct {
	c         *Config
	q         *runQueue
	d         *Dispatcher
	w         Worker
	sup, cmd  int
	supID     string
	attemptID string
}

func newDetachedRig(t *testing.T) *detachedRig {
	t.Helper()
	c := testConfig(t, detachedScript(t))
	c.Backoff.CooldownSeconds = 3600
	c.Roles[0].Prompt = "work on {ticketLocal}; {detachedRun}"
	q := &runQueue{fakeQueue: &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}, runs: map[string][]DetachedRun{}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	r := &detachedRig{c: c, q: q, d: d, attemptID: "claim"}
	t.Cleanup(func() {
		if r.d != nil {
			r.d.Close()
		}
	})
	// Every session and supervisor stand-in the test started is stopped,
	// including sessions launched after a hand-off.
	t.Cleanup(func() {
		for _, w := range r.d.ledger.Workers {
			for _, m := range w.Members {
				syscall.Kill(m.PID, syscall.SIGKILL)
			}
		}
		sups, _ := filepath.Glob(filepath.Join(c.WorkRoot, "*.sup"))
		for _, p := range sups {
			raw, _ := os.ReadFile(p)
			for _, f := range strings.Fields(string(raw)) {
				if pid := atoi(f); pid > 1 {
					syscall.Kill(pid, syscall.SIGKILL)
				}
			}
		}
	})
	tick(t, d)
	if d.Running() != 1 {
		t.Fatalf("running %d", d.Running())
	}
	r.w = *d.ledger.Workers[0]
	t.Cleanup(func() { syscall.Kill(r.w.PID, syscall.SIGKILL) })
	for i := 0; i < 500 && r.sup == 0; i++ {
		raw, _ := os.ReadFile(filepath.Join(c.WorkRoot, r.w.ID+".sup"))
		if f := strings.Fields(string(raw)); len(f) == 2 {
			r.sup, r.cmd = atoi(f[0]), atoi(f[1])
		}
		time.Sleep(20 * time.Millisecond)
	}
	if r.sup == 0 {
		t.Fatal("supervisor stand-in did not start")
	}
	t.Cleanup(func() { syscall.Kill(r.cmd, syscall.SIGKILL); syscall.Kill(r.sup, syscall.SIGKILL) })
	id, err := identityOf(r.sup)
	if err != nil || id == "" {
		t.Fatalf("supervisor identity %q %v", id, err)
	}
	r.supID = id
	q.obs.Attempts = []Attempt{{ID: r.attemptID, Ticket: r.w.Ticket, Phase: "RUNNING", Generation: "1", Holder: r.w.ID, Live: true, LeaseExpires: time.Now().Add(time.Hour)}}
	tick(t, d)
	if !memberOf(d.ledger.Workers[0], r.sup) {
		t.Fatalf("the supervisor stand-in is not in the worker tree: %+v", d.ledger.Workers[0].Members)
	}
	return r
}

func memberOf(w *Worker, pid int) bool {
	for _, m := range w.Members {
		if m.PID == pid {
			return true
		}
	}
	return false
}

func (r *detachedRig) run(state string, exit *int) DetachedRun {
	run := DetachedRun{RunID: testRunID, AttemptID: r.attemptID, Generation: "1", State: state, SupervisorPID: r.sup, SupervisorIdentity: r.supID, TimeoutSeconds: 600, LaunchedAt: time.Now().UTC().Truncate(time.Second), ExitStatus: exit, Output: "/runs/" + testRunID + "/output.log"}
	if state == RunFinished {
		run.Result = "/runs/" + testRunID + "/result.json"
	}
	return run
}

// endWorker lets the worker leader exit and ticks until it is accounted.
func (r *detachedRig) endWorker(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(r.c.WorkRoot, r.w.ID+".go"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	waitEnded(t, r.d)
	for i := 0; i < 20 && r.d.worker(r.w.ID) != nil; i++ {
		tick(t, r.d)
	}
	if r.d.worker(r.w.ID) != nil {
		t.Fatalf("worker %s still recorded", r.w.ID)
	}
}

func (r *detachedRig) reopen(t *testing.T) {
	t.Helper()
	if err := r.d.Close(); err != nil {
		t.Fatal(err)
	}
	d, err := Open("prog", r.c, r.q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	r.d = d
}

func alive(pid int, identity string) bool {
	id, _ := identityOf(pid)
	return id != "" && id == identity
}

// eventsWith returns the events of kind whose detail key has value.
func eventsWith(t *testing.T, d *Dispatcher, kind, key, value string) []Event {
	t.Helper()
	events, err := ReadEvents(d.dir, 1000)
	if err != nil {
		t.Fatal(err)
	}
	var out []Event
	for _, e := range events {
		if e.Kind == kind && e.Detail[key] == value {
			out = append(out, e)
		}
	}
	return out
}

func markerFiles(t *testing.T, d *Dispatcher) []string {
	t.Helper()
	entries, _ := os.ReadDir(d.detachedDir())
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

func (r *detachedRig) waitRelaunched(t *testing.T) Event {
	t.Helper()
	launched := eventsWith(t, r.d, "launched", "detachedRun", testRunID)
	if len(launched) != 1 {
		t.Fatalf("relaunches %d, want 1; events %v", len(launched), kinds(t, r.d))
	}
	succ := launched[0].Worker
	var prompt, env []byte
	for i := 0; i < 250 && len(env) == 0; i++ {
		prompt, _ = os.ReadFile(filepath.Join(r.c.WorkRoot, succ+".prompt"))
		env, _ = os.ReadFile(filepath.Join(r.c.WorkRoot, succ+".env"))
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(string(prompt), "detached run "+testRunID+" of attempt claim finished with exit status 7") || !strings.Contains(string(prompt), "/runs/"+testRunID+"/result.json") {
		t.Fatalf("relaunch prompt %q", prompt)
	}
	for _, want := range []string{"CORVINT_DISPATCH_RUN_ID=" + testRunID, "CORVINT_DISPATCH_RUN_ATTEMPT=claim", "CORVINT_DISPATCH_RUN_EXIT=7", "CORVINT_DISPATCH_RUN_RESULT=/runs/" + testRunID + "/result.json", "CORVINT_DISPATCH_RUN_OUTPUT=/runs/" + testRunID + "/output.log"} {
		if !strings.Contains(string(env), want+"\n") {
			t.Fatalf("relaunch env %q lacks %s", env, want)
		}
	}
	if launched[0].Role != r.w.Role || launched[0].Ticket != r.w.Ticket || launched[0].Detail["runExit"] != "7" {
		t.Fatalf("relaunch event %+v", launched[0])
	}
	return launched[0]
}

// CAL-V0-145, CAL-V0-146, CAL-V0-148, CAL-V0-149: the worker exits while its
// detached run continues; the supervisor is not stopped, the hand-off waits
// for the run, and once it finishes the attempt is handed off and one
// session of the same role is launched with the run's outcome.
func TestCALV0145_WorkerExitsWhileDetachedRunContinues(t *testing.T) {
	r := newDetachedRig(t)
	r.q.set(r.attemptID, r.run(RunRunning, nil))
	r.endWorker(t)
	if !alive(r.sup, r.supID) {
		t.Fatal("the dispatcher stopped the detached run supervisor")
	}
	if id, _ := identityOf(r.cmd); id == "" {
		t.Fatal("the dispatcher stopped the detached run's command")
	}
	if len(r.q.released) != 0 || len(r.q.reaped) != 0 {
		t.Fatalf("hand-off not deferred: released %v reaped %v", r.q.released, r.q.reaped)
	}
	if len(eventsWith(t, r.d, "handoff", "handoff", "DEFERRED")) != 1 || len(eventsWith(t, r.d, "finished", "detachedRun", testRunID)) != 1 {
		t.Fatalf("events %v", kinds(t, r.d))
	}
	if has(kinds(t, r.d), "cooldown") || has(kinds(t, r.d), "killing") {
		t.Fatalf("a deferred session was stopped or cooled down: %v", kinds(t, r.d))
	}
	// While it runs, ticks keep deferring and launch nothing.
	tick(t, r.d)
	tick(t, r.d)
	if len(r.q.released) != 0 || r.d.Running() != 0 || r.d.idleEligible() {
		t.Fatalf("released %v running %d idle %v", r.q.released, r.d.Running(), r.d.idleEligible())
	}
	exit := 7
	r.q.set(r.attemptID, r.run(RunFinished, &exit))
	tick(t, r.d)
	if len(r.q.released) != 1 || r.q.released[0] != r.attemptID || r.q.evidence[0] != "dispatch:"+r.w.ID {
		t.Fatalf("released %v evidence %v", r.q.released, r.q.evidence)
	}
	r.waitRelaunched(t)
	for i := 0; i < 10; i++ {
		waitEnded(t, r.d)
		tick(t, r.d)
	}
	if len(eventsWith(t, r.d, "launched", "detachedRun", testRunID)) != 1 || len(r.q.released) != 1 {
		t.Fatalf("relaunched more than once: %v", kinds(t, r.d))
	}
	if m := markerFiles(t, r.d); len(m) != 0 || len(r.d.detached) != 0 {
		t.Fatalf("markers kept after the successor ended: %v", m)
	}
}

// CAL-V0-147, CAL-V0-149: a restarted dispatcher resumes the deferral from
// its marker, hands off once and relaunches once, also across a restart
// after the relaunch.
func TestCALV0147_DeferralAndRelaunchSurviveRestart(t *testing.T) {
	r := newDetachedRig(t)
	r.q.set(r.attemptID, r.run(RunRunning, nil))
	r.endWorker(t)
	if m := markerFiles(t, r.d); len(m) != 1 || m[0] != testRunID+".json" {
		t.Fatalf("markers %v", m)
	}
	r.reopen(t)
	tick(t, r.d)
	if len(r.q.released) != 0 || r.d.Running() != 0 {
		t.Fatalf("restart lost the deferral: released %v running %d", r.q.released, r.d.Running())
	}
	exit := 7
	r.q.set(r.attemptID, r.run(RunFinished, &exit))
	r.reopen(t)
	tick(t, r.d)
	if len(r.q.released) != 1 {
		t.Fatalf("released %v", r.q.released)
	}
	r.waitRelaunched(t)
	r.reopen(t)
	for i := 0; i < 10; i++ {
		waitEnded(t, r.d)
		tick(t, r.d)
	}
	if n := len(eventsWith(t, r.d, "launched", "detachedRun", testRunID)); n != 1 || len(r.q.released) != 1 {
		t.Fatalf("relaunches %d released %v after restart", n, r.q.released)
	}
}

// CAL-V0-145, CAL-V0-151: a record whose supervisor identity does not match
// exempts nothing, defers nothing and launches nothing.
func TestCALV0151_ForgedRecordNeitherExemptsNorRelaunches(t *testing.T) {
	r := newDetachedRig(t)
	forged := r.run(RunRunning, nil)
	forged.SupervisorIdentity = "forged"
	r.q.set(r.attemptID, forged)
	r.endWorker(t)
	if !gone(r.sup) {
		t.Fatal("a forged record exempted the process from the stopped tree")
	}
	if len(r.q.released) != 1 || len(eventsWith(t, r.d, "handoff", "handoff", "DEFERRED")) != 0 || len(markerFiles(t, r.d)) != 0 {
		t.Fatalf("released %v markers %v events %v", r.q.released, markerFiles(t, r.d), kinds(t, r.d))
	}
	exit := 0
	finished := r.run(RunFinished, &exit)
	finished.SupervisorIdentity = "forged"
	r.q.set(r.attemptID, finished)
	tick(t, r.d)
	if len(eventsWith(t, r.d, "launched", "detachedRun", testRunID)) != 0 {
		t.Fatal("a forged record launched a session")
	}
}

// CAL-V0-145: only a live session leader other than the worker leader can
// be a detached run supervisor.
func TestCALV0145_SupervisorMustBeAnotherSessionLeader(t *testing.T) {
	r := newDetachedRig(t)
	leaderID, _ := identityOf(r.w.PID)
	cmdID, _ := identityOf(r.cmd)
	cases := map[string]struct {
		pid int
		id  string
		ok  bool
	}{
		"supervisor":         {r.sup, r.supID, true},
		"worker leader":      {r.w.PID, leaderID, false},
		"not session leader": {r.cmd, cmdID, false},
		"wrong identity":     {r.sup, r.supID + "x", false},
		"dispatcher":         {os.Getpid(), "", false},
	}
	for name, c := range cases {
		if got := detachedSupervisor(c.pid, c.id, r.w.PID); got != c.ok {
			t.Errorf("%s: detachedSupervisor = %v", name, got)
		}
	}
	os.WriteFile(filepath.Join(r.c.WorkRoot, r.w.ID+".go"), nil, 0o600)
}

// CAL-V0-145, CAL-V0-151: a record naming a session leader that encloses
// the dispatcher spares nothing of the worker; the worker's own run
// supervisor is still spared.
func TestCALV0151_EnclosingSessionLeaderSparesNoWorker(t *testing.T) {
	r := newDetachedRig(t)
	outer, err := getsid(os.Getpid())
	if err != nil || outer <= 1 || outer == os.Getpid() {
		t.Skipf("no enclosing session leader: %d %v", outer, err)
	}
	outerID, err := identityOf(outer)
	if err != nil || outerID == "" {
		t.Skipf("enclosing session leader unreadable: %v", err)
	}
	supID, _ := identityOf(r.sup)
	procs, err := observeProcs()
	if err != nil {
		t.Fatal(err)
	}
	w := r.w
	w.Members = append([]Proc(nil), r.w.Members...)
	if err := refreshTree(&w, procs, map[int]string{outer: outerID, r.sup: supID}); err != nil {
		t.Fatal(err)
	}
	in := map[int]bool{}
	for _, m := range w.Members {
		in[m.PID] = true
	}
	if !in[r.w.PID] || in[r.sup] || in[r.cmd] {
		t.Fatalf("members %v: leader %d kept, supervisor %d and its command %d spared", w.Members, r.w.PID, r.sup, r.cmd)
	}
}

// CAL-V0-146, CAL-V0-148: a supervisor lost while deferring ends the
// deferral: the attempt is handed off, and no session is relaunched.
func TestCALV0148_SupervisorLostHandsOffWithoutRelaunch(t *testing.T) {
	r := newDetachedRig(t)
	r.q.set(r.attemptID, r.run(RunRunning, nil))
	r.endWorker(t)
	syscall.Kill(r.cmd, syscall.SIGKILL)
	syscall.Kill(r.sup, syscall.SIGKILL)
	if !gone(r.sup) {
		t.Fatal("supervisor stand-in survived SIGKILL")
	}
	tick(t, r.d)
	if len(r.q.released) != 1 || len(eventsWith(t, r.d, "alert", "runState", "SUPERVISOR_LOST")) != 1 {
		t.Fatalf("released %v events %v", r.q.released, kinds(t, r.d))
	}
	tick(t, r.d)
	if len(eventsWith(t, r.d, "launched", "detachedRun", testRunID)) != 0 || len(markerFiles(t, r.d)) != 0 {
		t.Fatalf("relaunched or kept the marker after the supervisor was lost: %v", markerFiles(t, r.d))
	}
}

// CAL-V0-146: the deferral is bounded by the run's own timeout plus the
// settle bound; past it the hand-off proceeds although the record still
// says RUNNING.
func TestCALV0146_DeferralIsBounded(t *testing.T) {
	r := newDetachedRig(t)
	r.q.set(r.attemptID, r.run(RunRunning, nil))
	r.endWorker(t)
	if len(r.q.released) != 0 {
		t.Fatalf("released %v", r.q.released)
	}
	r.d.Now = func() time.Time { return time.Now().Add(600*time.Second + detachedSettle + time.Second) }
	tick(t, r.d)
	if len(r.q.released) != 1 || len(eventsWith(t, r.d, "alert", "runState", "OVERDUE")) != 1 || len(markerFiles(t, r.d)) != 0 {
		t.Fatalf("released %v markers %v events %v", r.q.released, markerFiles(t, r.d), kinds(t, r.d))
	}
}

// CAL-V0-147: a marker that does not decode is reported and removed at Open
// and defers or launches nothing.
func TestCALV0147_MalformedMarkerIsRemoved(t *testing.T) {
	c := testConfig(t, "exit 0")
	dir := filepath.Join(ProgramDir(c, "prog"), detachedDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, testRunID+".json"), []byte(`{"profile":"taskman-dispatch-detached-run/0","runId":"`+testRunID+`","extra":1}`), 0o600)
	os.WriteFile(filepath.Join(dir, ".tmp-1"), nil, 0o600)
	// Another build's marker version is held unread, not removed (CAL-V0-131).
	const newer = "00000000000000ff"
	os.WriteFile(filepath.Join(dir, newer+".json"), []byte(`{"profile":"taskman-dispatch-detached-run/1","runId":"`+newer+`","zzNewerMember":1}`), 0o600)
	views, bad, err := DetachedMarkers(ProgramDir(c, "prog"))
	if err != nil || len(views) != 0 || len(bad) != 2 || wire.CodeOf(bad[newer+".json"]) != wire.CodeUnsupportedVersion {
		t.Fatalf("status view %v bad %v err %v", views, bad, err)
	}
	d, err := Open("prog", c, &fakeQueue{}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if m := markerFiles(t, d); len(m) != 1 || m[0] != newer+".json" || len(d.detached) != 0 || !d.detachedHeld[newer] || !has(kinds(t, d), "alert") {
		t.Fatalf("markers %v events %v", m, kinds(t, d))
	}
}

// CAL-V0-149: {detachedRun} renders only into a role prompt; a host reads
// the outcome from the CORVINT_DISPATCH_RUN_* environment instead.
func TestCALV0149_DetachedRunPlaceholderIsPromptOnly(t *testing.T) {
	for name, mut := range map[string]func(*Config){
		"argv": func(c *Config) { h := c.Hosts["sh"]; h.Argv = append(h.Argv, "{detachedRun}"); c.Hosts["sh"] = h },
		"env": func(c *Config) {
			h := c.Hosts["sh"]
			h.Env = map[string]string{"RUN": "{detachedRun}"}
			c.Hosts["sh"] = h
		},
		"activityPaths": func(c *Config) {
			h := c.Hosts["sh"]
			h.ActivityPaths = []string{"/tmp/{detachedRun}"}
			c.Hosts["sh"] = h
		},
	} {
		c := testConfig(t, "true")
		mut(c)
		raw, _ := json.Marshal(c)
		if _, err := DecodeConfig(raw); err == nil {
			t.Fatalf("{detachedRun} in host %s admitted", name)
		}
	}
	c := testConfig(t, "true")
	c.Roles[0].Prompt = "work {detachedRun}"
	raw, _ := json.Marshal(c)
	if _, err := DecodeConfig(raw); err != nil {
		t.Fatalf("{detachedRun} in a role prompt refused: %v", err)
	}
}
