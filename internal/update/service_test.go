package update

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/execfile"
)

// serviceFixture writes a service-control program whose state lives in dir:
// `running` marks a running service (status exits 0, otherwise 3),
// `status-broken` makes status exit 2, `live` holds the live-attempt count
// (decremented on each read when `draining` exists), `fail-new` makes a start
// of build 163 never report running and `stop-fails-new` makes a stop of build
// 163 fail. `start-fails` makes every start exit 1. `slow-stop` (any build) and
// `slow-stop-new` (build 163) hold a count: stop then exits 0 while the
// service stays running for that many further status reads; `broken-after-stop`
// makes status exit 2 after a stop until the next start. Every call is logged
// with the build of the managed binary.
func serviceFixture(t *testing.T, dir string) string {
	t.Helper()
	program := filepath.Join(dir, "service")
	script := `#!/bin/sh
d='` + dir + `'
echo "$1 $("$CORVINT_UPDATE_BINARY" --version 2>/dev/null) $CORVINT_UPDATE_COMPONENT" >> "$d/log"
new() { "$CORVINT_UPDATE_BINARY" --version | grep -q 'build 163'; }
case "$1" in
status)
  if [ -f "$d/status-broken" ]; then exit 2; fi
  if [ -f "$d/stopping" ]; then
    n=$(cat "$d/stopping")
    if [ "$n" -le 0 ]; then rm -f "$d/running" "$d/stopping"; else echo $((n - 1)) > "$d/stopping"; fi
  fi
  [ -f "$d/running" ] || exit 3 ;;
stop)
  if [ -f "$d/stop-fails-new" ] && new; then exit 1; fi
  if [ -f "$d/slow-stop" ]; then cp "$d/slow-stop" "$d/stopping"; exit 0; fi
  if [ -f "$d/slow-stop-new" ] && new; then : > "$d/running"; cp "$d/slow-stop-new" "$d/stopping"; exit 0; fi
  rm -f "$d/running"
  if [ -f "$d/broken-after-stop" ]; then : > "$d/status-broken"; fi ;;
start)
  if [ -f "$d/start-fails" ]; then exit 1; fi
  rm -f "$d/stopping" "$d/status-broken"
  if [ -f "$d/fail-new" ] && new; then exit 0; fi
  : > "$d/running" ;;
live-attempts)
  n=$(cat "$d/live" 2>/dev/null || echo 0)
  echo "$n"
  if [ -f "$d/draining" ] && [ "$n" -gt 0 ]; then echo $((n - 1)) > "$d/live"; fi ;;
*) exit 2 ;;
esac
`
	if err := execfile.WriteFile(program, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return program
}

func serviceEngine(t *testing.T) (engine, string, string) {
	t.Helper()
	e := fixtureEngine(t)
	dir := filepath.Join(filepath.Dir(e.config.BinDir), "svc")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	e.config.ServiceProgram = serviceFixture(t, dir)
	e.config.StartTimeout = 500 * time.Millisecond
	e.config.DrainTimeout = 500 * time.Millisecond
	e.config.HookTimeout = time.Second
	e.servicePoll = 10 * time.Millisecond
	dest := filepath.Join(e.config.BinDir, "corvint")
	writeScript(t, dest, 162)
	releaseClient(t, &e, 163)
	return e, dir, dest
}

func readLog(t *testing.T, dir string) string {
	t.Helper()
	b, _ := os.ReadFile(filepath.Join(dir, "log"))
	return string(b)
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func TestUPDV0011ServiceRestartsAroundSwitch(t *testing.T) {
	e, dir, dest := serviceEngine(t)
	if err := os.WriteFile(filepath.Join(dir, "running"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := e.execute(context.Background(), "apply")
	if err != nil {
		t.Fatal(err)
	}
	if r.Action != "applied" || !r.Switched || r.InstalledBuild != 163 || r.Service == nil || r.Service.Mode != "RESTART" || !r.Service.WasRunning || r.Service.State != "RESUMED" || !exists(filepath.Join(dir, "running")) {
		t.Fatalf("restart: %+v %+v", r, r.Service)
	}
	log := readLog(t, dir)
	stop, start := strings.Index(log, "stop Corvint 1.0.0-rc.1 (build 162)"), strings.Index(log, "start Corvint 1.0.0-rc.1 (build 163)")
	if stop < 0 || start < stop || !strings.Contains(log[start:], "status Corvint 1.0.0-rc.1 (build 163)") || !strings.Contains(log, " core\n") {
		t.Fatalf("service log:\n%s", log)
	}
	if d, _ := digest(dest); d != hash(scriptBinary(163, 0)) {
		t.Fatal("new build not installed")
	}
	// A service that was not running is switched under but not started.
	e, dir, _ = serviceEngine(t)
	r, err = e.execute(context.Background(), "apply")
	if err != nil || r.Service.State != "NOT_RUNNING_BEFORE" || r.Service.WasRunning || !r.Switched || strings.Contains(readLog(t, dir), "start") || exists(filepath.Join(dir, "running")) {
		t.Fatalf("not running before: %+v %+v %v\n%s", r, r.Service, err, readLog(t, dir))
	}
	// The options belong to apply, and the program must be an absolute executable.
	e, _, _ = serviceEngine(t)
	if _, err := e.execute(context.Background(), "check"); err == nil || !strings.Contains(err.Error(), "apply only to apply") {
		t.Fatalf("check with service: %v", err)
	}
	e.config.ServiceProgram = "service"
	if _, err := e.execute(context.Background(), "apply"); err == nil || !strings.Contains(err.Error(), "clean absolute path") {
		t.Fatalf("relative program: %v", err)
	}
	e.config.ServiceProgram = ""
	e.config.HookAttempts = 6
	e.config.PostSwitch = []string{"true"}
	if _, err := e.execute(context.Background(), "apply"); err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("hook attempts bound: %v", err)
	}
}

func TestUPDV0012PostSwitchHooksReportedAndSwitchStands(t *testing.T) {
	e, dir, dest := serviceEngine(t)
	e.config.ServiceProgram = ""
	counter := filepath.Join(dir, "flaky")
	e.config.PostSwitch = []string{
		`echo "ok build $CORVINT_UPDATE_BUILD $CORVINT_UPDATE_COMPONENT"`,
		`n=$(cat '` + counter + `' 2>/dev/null || echo 0); echo $((n + 1)) > '` + counter + `'; echo try $n; [ "$n" -ge 2 ]`,
		`echo broken >&2; exit 7`,
		`sleep 5`,
	}
	e.config.HookTimeout = 200 * time.Millisecond
	r, err := e.execute(context.Background(), "apply")
	if err == nil || !strings.Contains(err.Error(), "the switch to the new build stands") {
		t.Fatalf("hook failure not reported: %v", err)
	}
	if r.Action != "applied" || !r.Switched || r.InstalledBuild != 163 || len(r.Hooks) != 4 {
		t.Fatalf("result: %+v", r)
	}
	if d, _ := digest(dest); d != hash(scriptBinary(163, 0)) {
		t.Fatal("a failed hook reversed the switch")
	}
	want := []struct {
		state    string
		attempts int
		exit     int
		output   string
	}{
		{"SUCCEEDED", 1, 0, "ok build 163 core"},
		{"SUCCEEDED", 3, 0, "try 2"},
		{"FAILED", 3, 7, "broken"},
		{"FAILED", 3, -1, ""},
	}
	for i, w := range want {
		h := r.Hooks[i]
		if h.Command != e.config.PostSwitch[i] || h.State != w.state || h.Attempts != w.attempts || h.ExitStatus != w.exit || !strings.Contains(h.Output, w.output) {
			t.Fatalf("hook %d: %+v", i, h)
		}
	}
	if !strings.Contains(r.Hooks[3].Reason, "timed out") {
		t.Fatalf("timeout reason: %+v", r.Hooks[3])
	}
}

func TestUPDV0013RollbackWhenServiceDoesNotStart(t *testing.T) {
	e, dir, dest := serviceEngine(t)
	before, _ := digest(dest)
	for _, name := range []string{"running", "fail-new"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	marker := filepath.Join(dir, "hook-ran")
	e.config.PostSwitch = []string{": > '" + marker + "'"}
	r, err := e.execute(context.Background(), "apply")
	if err == nil || !strings.Contains(err.Error(), "rolled back to the previous build") {
		t.Fatalf("rollback not reported: %v", err)
	}
	if after, _ := digest(dest); after != before {
		t.Fatal("previous binary not restored")
	}
	if r.Action != "rolled-back" || r.Switched || r.InstalledBuild != 162 || r.Service.State != "ROLLED_BACK" || !strings.Contains(r.Service.Reason, "did not report running") || r.Receipt == "" {
		t.Fatalf("result: %+v %+v", r, r.Service)
	}
	log := readLog(t, dir)
	if !exists(filepath.Join(dir, "running")) || !strings.HasSuffix(strings.TrimSpace(log), "status Corvint 1.0.0-rc.1 (build 162) core") {
		t.Fatalf("service not restored on the previous build:\n%s", log)
	}
	if exists(marker) || r.Hooks != nil {
		t.Fatal("hooks ran after a rolled-back switch")
	}
}

func TestUPDV0014LiveAttemptsAdoptedOrDrained(t *testing.T) {
	for _, c := range []struct {
		component            string
		installed, candidate []string
		want                 string
	}{
		{"core", nil, nil, "RESTART"},
		{"tasks", []string{"a/0", "b/0"}, []string{"a/0", "b/0"}, "ADOPT"},
		{"tasks", []string{"a/0", "b/0"}, []string{"a/0", "b/1"}, "DRAIN"},
		{"tasks", nil, nil, "DRAIN"},
	} {
		if got := serviceMode(c.component, c.installed, c.candidate); got != c.want {
			t.Fatalf("%+v: %s", c, got)
		}
	}
	bin := filepath.Join(t.TempDir(), "corvint-tasks")
	if err := execfile.WriteFile(bin, []byte("#!/bin/sh\necho '{\"items\":[{\"formats\":[\"b/0\",\"a/0\"],\"version\":\"x+build.5\"}],\"profile\":\"taskman-command-result/0\"}'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if got := tasksFormats(context.Background(), bin, ""); strings.Join(got, ",") != "a/0,b/0" {
		t.Fatalf("formats: %v", got)
	}
	if err := execfile.WriteFile(bin, []byte("#!/bin/sh\necho 'not json'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if got := tasksFormats(context.Background(), bin, ""); got != nil {
		t.Fatalf("unreadable formats: %v", got)
	}
	setup := func(t *testing.T, live string, draining bool) (engine, service, string) {
		e, dir, dest := serviceEngine(t)
		e.config.Component = "tasks"
		files := map[string]string{"running": "", "live": live}
		if draining {
			files["draining"] = ""
		}
		for name, body := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		return e, service{program: e.config.ServiceProgram, component: "tasks", binary: dest, poll: e.servicePoll}, dir
	}
	// ADOPT stops at once and leaves the live attempts to the restarted service.
	e, s, dir := setup(t, "2\n", false)
	out, err := e.quiesce(context.Background(), s, "ADOPT")
	if err != nil || out.State != "STOPPED" || out.LiveAttempts == nil || *out.LiveAttempts != 2 || exists(filepath.Join(dir, "running")) {
		t.Fatalf("adopt: %+v %v", out, err)
	}
	// DRAIN waits for zero live attempts before the switch.
	e, s, dir = setup(t, "2\n", true)
	out, err = e.quiesce(context.Background(), s, "DRAIN")
	if err != nil || out.State != "STOPPED" || *out.LiveAttempts != 0 || exists(filepath.Join(dir, "running")) {
		t.Fatalf("drain: %+v %v", out, err)
	}
	// A drain that does not reach zero in the bound resumes the old build and refuses.
	e, s, dir = setup(t, "3\n", false)
	out, err = e.quiesce(context.Background(), s, "DRAIN")
	if err == nil || out.State != "RESUMED_OLD" || *out.LiveAttempts != 3 || !exists(filepath.Join(dir, "running")) || !strings.Contains(out.Reason, "did not drain") {
		t.Fatalf("drain timeout: %+v %v", out, err)
	}
}

// Codex findings 2 and 3 on #1059: a new build that cannot be stopped, or a
// DRAIN-mode Tasks switch, is never rolled back underneath the service.
func TestUPDV0013NoRollbackWithoutStopOrAcrossFormats(t *testing.T) {
	e, dir, dest := serviceEngine(t)
	for _, name := range []string{"running", "fail-new", "stop-fails-new"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	r, err := e.execute(context.Background(), "apply")
	if err == nil || !strings.Contains(err.Error(), "could not be stopped; the new build stays installed") {
		t.Fatalf("stop failure not reported: %v", err)
	}
	if d, _ := digest(dest); d != hash(scriptBinary(163, 0)) {
		t.Fatal("previous binary installed under a service that did not stop")
	}
	if r.Action != "applied" || !r.Switched || r.InstalledBuild != 163 || r.Service.State != "FAILED" || !strings.Contains(r.Service.Reason, "rollback not attempted") {
		t.Fatalf("result: %+v %+v", r, r.Service)
	}
	if strings.Contains(readLog(t, dir), "start Corvint 1.0.0-rc.1 (build 162)") {
		t.Fatalf("old build started:\n%s", readLog(t, dir))
	}
	// DRAIN mode: the service is stopped, the rollback refused.
	e, dir, dest = serviceEngine(t)
	if err := os.WriteFile(filepath.Join(dir, "running"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	writeScript(t, dest, 163)
	s := service{program: e.config.ServiceProgram, component: "tasks", binary: dest, poll: e.servicePoll}
	out := &ServiceOutcome{Mode: "DRAIN", State: "STOPPED", WasRunning: true}
	err = rollbackPermitted(context.Background(), s, out, errors.New("service did not report running"), e.config.StartTimeout)
	if err == nil || !strings.Contains(err.Error(), "automatic rollback refused") || out.State != "FAILED" || !strings.Contains(out.Reason, "formats differ or are unknown") || exists(filepath.Join(dir, "running")) {
		t.Fatalf("drain rollback: %+v %v", out, err)
	}
	for _, mode := range []string{"ADOPT", "RESTART"} {
		out = &ServiceOutcome{Mode: mode, State: "STOPPED", WasRunning: true}
		if err := rollbackPermitted(context.Background(), s, out, errors.New("x"), e.config.StartTimeout); err != nil {
			t.Fatalf("%s rollback refused: %v", mode, err)
		}
	}
}

// Codex finding 1 on #1059: an unknown status refuses before any stop, and
// DRAIN waits for live attempts even when the dispatcher is stopped.
func TestUPDV0014StoppedDispatcherStillDrainsAndUnknownStatusRefuses(t *testing.T) {
	e, dir, dest := serviceEngine(t)
	if err := os.WriteFile(filepath.Join(dir, "status-broken"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := e.execute(context.Background(), "apply")
	if err == nil || !strings.Contains(err.Error(), "service status unknown") || r.Service == nil || r.Service.State != "FAILED" || r.Switched || strings.Contains(readLog(t, dir), "stop") {
		t.Fatalf("unknown status: %+v %v\n%s", r, err, readLog(t, dir))
	}
	if d, _ := digest(dest); d != hash(scriptBinary(162, 0)) {
		t.Fatal("switched under an unknown service status")
	}
	s := service{program: e.config.ServiceProgram, component: "tasks", binary: dest, poll: e.servicePoll}
	// Stopped dispatcher, three live attempts that never drain: refused.
	if err := os.Remove(filepath.Join(dir, "status-broken")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "live"), []byte("3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := e.quiesce(context.Background(), s, "DRAIN")
	if err == nil || out.WasRunning || out.State != "NOT_RUNNING_BEFORE" || out.LiveAttempts == nil || *out.LiveAttempts != 3 || !strings.Contains(out.Reason, "did not drain") || exists(filepath.Join(dir, "running")) {
		t.Fatalf("stopped dispatcher drain: %+v %v", out, err)
	}
	// Stopped dispatcher whose attempts drain: the switch may proceed.
	if err := os.WriteFile(filepath.Join(dir, "draining"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err = e.quiesce(context.Background(), s, "DRAIN")
	if err != nil || out.WasRunning || out.State != "NOT_RUNNING_BEFORE" || *out.LiveAttempts != 0 {
		t.Fatalf("stopped dispatcher drained: %+v %v", out, err)
	}
	// A count that cannot be read refuses as well.
	if err := os.WriteFile(filepath.Join(dir, "live"), []byte("many\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err = e.quiesce(context.Background(), s, "DRAIN"); err == nil || !strings.Contains(out.Reason, "no count") {
		t.Fatalf("unreadable count: %+v %v", out, err)
	}
}
