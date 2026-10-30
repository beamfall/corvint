//go:build darwin || linux

package cli_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestDSGV0012_RollingUpgradeWithRealExec covers GitHub #1815 end to end on
// real builds: three corvint-tasks builds that differ only in their version
// string. A dispatcher started on build A hands over to B with `dispatch
// upgrade --apply`; the process ID and the lock stay, the session A launched
// keeps running on A, the next session launches on B (its tools named by
// CORVINT_TASKS_BIN), and the A session's exit code is observed by B after B
// owns the queue. A second upgrade in a row moves B to C the same way. A
// third request names a build that dies as soon as it is executed: the
// dispatcher process ends mid-upgrade, and a restart reports the interrupted
// upgrade with the live session, its lease and its record unchanged.
func TestDSGV0012_RollingUpgradeWithRealExec(t *testing.T) {
	if testing.Short() {
		t.Skip("builds three corvint-tasks executables")
	}
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	work, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var builds [3]string
	var errs [3]error
	var wg sync.WaitGroup
	for i, n := range []string{"A", "B", "C"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out := filepath.Join(work, "build-"+n, "corvint-tasks")
			c := exec.Command("go", "build", "-ldflags", "-X main.build=upgrade-"+n, "-o", out, "../../../cmd/corvint-tasks")
			if b, err := c.CombinedOutput(); err != nil {
				errs[i] = fmt.Errorf("build %s: %v %s", n, err, b)
			}
			builds[i] = out
		}()
	}
	wg.Wait()
	if err := errors.Join(errs[:]...); err != nil {
		t.Fatal(err)
	}
	a, b, c := builds[0], builds[1], builds[2]
	sha := map[string]string{}
	for _, p := range builds {
		if sha[p], err = dispatch.BuildDigest(p); err != nil {
			t.Fatal(err)
		}
	}
	if sha[a] == sha[b] || sha[b] == sha[c] {
		t.Fatal("the builds are not distinct")
	}

	// Each session claims its ticket with the tools CORVINT_TASKS_BIN names,
	// records that name, and exits 7 once released.
	release := filepath.Join(work, "release")
	if err := os.MkdirAll(release, 0o700); err != nil {
		t.Fatal(err)
	}
	host := filepath.Join(work, "session.sh")
	fixture.Write(t, host, []byte(`#!/bin/sh
R=`+release+`
"$CORVINT_TASKS_BIN" claim "$1" --holder "$2" --stage implement --request-id "claim-$3" > "$R/$3.claim" 2>&1
echo "$CORVINT_TASKS_BIN" > "$R/$3.bin"
while [ ! -e "$R/$3.go" ] && [ ! -e "$R/all" ]; do sleep 0.1; done
exit 7
`))
	if err := os.Chmod(host, 0o700); err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(work, "state")
	progDir := filepath.Join(stateDir, "prog")
	cfg := map[string]any{
		"profile": "taskman-dispatch/0", "stateDir": stateDir, "workRoot": root,
		"tickSeconds": 1, "globalCap": 3, "killGraceSeconds": 1,
		"hosts":   map[string]any{"sh": map[string]any{"argv": []string{host, "{ticket}", "{holder}", "{worker}"}}},
		"roles":   []any{map[string]any{"name": "impl", "host": "sh", "cap": 3, "match": map[string]any{"planSelected": true}, "prompt": "implement {ticketLocal}", "idleSeconds": 600, "wallSeconds": 900}},
		"backoff": map[string]any{"cooldownSeconds": 3600, "parkAfter": 3},
		"heal":    map[string]any{"handoff": true, "reap": true},
	}
	raw, _ := json.Marshal(cfg)
	configPath := filepath.Join(work, "dispatch.json")
	fixture.Write(t, configPath, raw)
	base := []string{"--program", "prog", "--config", configPath}

	var procs []*exec.Cmd
	start := func(build string) *exec.Cmd {
		t.Helper()
		log, err := os.Create(filepath.Join(work, fmt.Sprintf("dispatcher-%d.log", len(procs))))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { log.Close() })
		cmd := exec.Command(build, append([]string{"dispatch"}, base...)...)
		cmd.Dir, cmd.Stdout, cmd.Stderr = root, log, log
		cmd.Env = append(os.Environ(), "CORVINT_TASKS_BIN=")
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		procs = append(procs, cmd)
		return cmd
	}
	t.Cleanup(func() {
		fixture.Write(t, filepath.Join(release, "all"), nil)
		for _, p := range procs {
			if p.ProcessState == nil {
				_ = p.Process.Signal(syscall.SIGTERM)
				_ = p.Wait()
			}
		}
		// Sessions adopted by a restarted dispatcher are not its children;
		// give them a moment to see the release before the directory goes.
		time.Sleep(300 * time.Millisecond)
	})
	waitEvent := func(match func(dispatch.Event) bool, what string) dispatch.Event {
		t.Helper()
		deadline := time.Now().Add(60 * time.Second)
		for time.Now().Before(deadline) {
			events, _ := dispatch.ReadEvents(progDir, 10000)
			for _, e := range events {
				if match(e) {
					return e
				}
			}
			time.Sleep(50 * time.Millisecond)
		}
		logs, _ := filepath.Glob(filepath.Join(work, "dispatcher-*.log"))
		var all []byte
		for _, l := range logs {
			b, _ := os.ReadFile(l)
			all = append(all, b...)
		}
		t.Fatalf("no %s event:\n%s", what, all)
		return dispatch.Event{}
	}
	launched := map[string]bool{}
	nextLaunch := func(ticket string) string {
		t.Helper()
		e := waitEvent(func(e dispatch.Event) bool { return e.Kind == "launched" && e.Ticket == ticket && !launched[e.Worker] }, "launched "+ticket)
		launched[e.Worker] = true
		return e.Worker
	}
	apply := func(build, want string) {
		t.Helper()
		r := atm(t, root, nil, append([]string{"dispatch", "upgrade", "--apply", build}, base...)...)
		if r.res.Outcome != wire.OutcomeOK || field(r.res.Items[0], "verdict").Str != want {
			t.Fatalf("apply %s: %s %s", build, r.stdout, r.stderr)
		}
	}
	lockPID := func() int {
		raw, _ := os.ReadFile(filepath.Join(progDir, "lock"))
		pid, _ := strconv.Atoi(strings.Fields(string(raw) + " 0")[0])
		return pid
	}
	binOf := func(worker string) string {
		t.Helper()
		for i := 0; i < 600; i++ {
			if raw, err := os.ReadFile(filepath.Join(release, worker+".bin")); err == nil && len(raw) > 0 {
				return strings.TrimSpace(string(raw))
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("session %s recorded no CORVINT_TASKS_BIN", worker)
		return ""
	}

	t1 := planTicket(t, root, "upgrade-one", "P1", `["one/"]`)
	d := start(a)
	w1 := nextLaunch(t1)
	if got := dispatch.WorkerBuild(progDir, w1); got.Sha256 != sha[a] || binOf(w1) != a {
		t.Fatalf("session 1 build %+v tools %s, want %s", got, binOf(w1), a)
	}

	// Upgrade 1: A -> B with session 1 live.
	apply(b, "REQUESTED")
	up := waitEvent(func(e dispatch.Event) bool { return e.Kind == "upgraded" && e.Detail["toSha256"] == sha[b] }, "upgraded to B")
	if up.Detail["fromSha256"] != sha[a] || up.Detail["oldBuildWorkers"] != "1" || up.Detail["windowEndsBy"] == "" {
		t.Fatalf("upgraded A->B: %+v", up.Detail)
	}
	if lockPID() != d.Process.Pid {
		t.Fatalf("lock holder %d, want the same process %d", lockPID(), d.Process.Pid)
	}
	t2 := planTicket(t, root, "upgrade-two", "P1", `["two/"]`)
	w2 := nextLaunch(t2)
	if got := dispatch.WorkerBuild(progDir, w2); got.Sha256 != sha[b] || binOf(w2) != b {
		t.Fatalf("session 2 build %+v tools %s, want %s", got, binOf(w2), b)
	}
	if got := dispatch.WorkerBuild(progDir, w1); got.Sha256 != sha[a] {
		t.Fatalf("session 1 build changed: %+v", got)
	}
	// The A session completes after B owns the queue; B observes its exit.
	fixture.Write(t, filepath.Join(release, w1+".go"), nil)
	fin := waitEvent(func(e dispatch.Event) bool { return e.Kind == "finished" && e.Worker == w1 }, "finished session 1")
	if fin.Detail["exitCode"] != "7" {
		t.Fatalf("session 1 exit after the handover: %+v", fin.Detail)
	}
	waitEvent(func(e dispatch.Event) bool { return e.Kind == "upgrade-settled" }, "upgrade-settled")

	// Upgrade 2, right after: B -> C with session 2 live.
	apply(c, "REQUESTED")
	up = waitEvent(func(e dispatch.Event) bool { return e.Kind == "upgraded" && e.Detail["toSha256"] == sha[c] }, "upgraded to C")
	if up.Detail["fromSha256"] != sha[b] || up.Detail["oldBuildWorkers"] != "1" || lockPID() != d.Process.Pid {
		t.Fatalf("upgraded B->C: %+v lock %d", up.Detail, lockPID())
	}
	fixture.Write(t, filepath.Join(release, w2+".go"), nil)
	fin = waitEvent(func(e dispatch.Event) bool { return e.Kind == "finished" && e.Worker == w2 }, "finished session 2")
	if fin.Detail["exitCode"] != "7" {
		t.Fatalf("session 2 exit after two handovers: %+v", fin.Detail)
	}

	// Upgrade 3 dies mid-exec: the wrapper answers the status probe with
	// build B but exits as soon as the dispatcher executes it.
	t3 := planTicket(t, root, "upgrade-three", "P1", `["three/"]`)
	w3 := nextLaunch(t3)
	binOf(w3)
	crash := filepath.Join(work, "crash", "corvint-tasks")
	fixture.Write(t, crash, []byte("#!/bin/sh\nif [ \"$1 $2\" = \"dispatch status\" ]; then exec "+b+" \"$@\"; fi\nexit 3\n"))
	if err := os.Chmod(crash, 0o700); err != nil {
		t.Fatal(err)
	}
	plan := func() string {
		t.Helper()
		r := atm(t, root, nil, "plan", "preview")
		for _, e := range field(r.res.Items[0], "entries").Arr {
			if field(e, "ticketId").Str == t3 {
				return field(e, "state").Str
			}
		}
		return "ABSENT"
	}
	leaseBefore := plan()
	if leaseBefore == "ABSENT" || leaseBefore == "SELECTED" {
		claim, _ := os.ReadFile(filepath.Join(release, w3+".claim"))
		t.Fatalf("session 3 holds no lease (%s): %s", leaseBefore, claim)
	}
	t.Logf("session 3 lease state %s", leaseBefore)
	apply(crash, "REQUESTED")
	err = d.Wait()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 3 {
		t.Fatalf("the crashing build ended the dispatcher with %v", err)
	}
	if st, err := dispatch.ReadUpgradeState(progDir); err != nil || st.Phase != dispatch.UpgradeExec {
		t.Fatalf("state after the crash: %+v %v", st, err)
	}
	leader := ""
	if l, err := dispatch.LoadLedger(progDir, "prog"); err == nil && len(l.Workers) == 1 && l.Workers[0].ID == w3 {
		leader = l.Workers[0].LeaderIdentity
		if live, _ := supervisor.ProcessIdentity(l.Workers[0].PID); !supervisor.SameProcess(live, leader) {
			t.Fatal("session 3 died with the dispatcher")
		}
	} else {
		t.Fatalf("ledger after the crash: %v", err)
	}
	start(c)
	waitEvent(func(e dispatch.Event) bool { return e.Kind == "alert" && e.Detail["code"] == "UPGRADE_INTERRUPTED" }, "UPGRADE_INTERRUPTED")
	waitEvent(func(e dispatch.Event) bool { return e.Kind == "adopted" && e.Worker == w3 }, "adopted session 3")
	if st, err := dispatch.ReadUpgradeState(progDir); err != nil || st.Phase == dispatch.UpgradeExec {
		t.Fatalf("state after the restart: %+v %v", st, err)
	}
	if l, err := dispatch.LoadLedger(progDir, "prog"); err != nil || len(l.Workers) != 1 || l.Workers[0].LeaderIdentity != leader {
		t.Fatalf("restart lost session 3: %v", err)
	}
	if got := plan(); got != leaseBefore {
		t.Fatalf("session 3 lease state %s, want %s", got, leaseBefore)
	}
}
