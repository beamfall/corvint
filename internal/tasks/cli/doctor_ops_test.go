//go:build darwin || linux

package cli_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// writeDoctorConfig writes a doctor config file and returns its path.
func writeDoctorConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "doctor.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func commonDirOf(t *testing.T, root string) string {
	t.Helper()
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	return repo.CommonDir
}

// TQD-V0-020: the config is closed, bounded and optional; omitted
// thresholds take the documented defaults and a malformed config refuses
// before the store is read.
func TestTQDV0020_DoctorConfig(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	for name, body := range map[string]string{
		"not json":          `{`,
		"wrong profile":     `{"profile":"taskman-doctor-config/9"}`,
		"unknown member":    `{"profile":"taskman-doctor-config/0","extra":"1"}`,
		"unknown threshold": `{"profile":"taskman-doctor-config/0","thresholds":{"lockHeld":"1"}}`,
		"zero threshold":    `{"profile":"taskman-doctor-config/0","thresholds":{"lockHeldSeconds":"0"}}`,
		"number threshold":  `{"profile":"taskman-doctor-config/0","thresholds":{"retryStorm":3}}`,
		"percent over 100":  `{"profile":"taskman-doctor-config/0","thresholds":{"fdPressurePercent":"101"}}`,
		"relative hook":     `{"profile":"taskman-doctor-config/0","notify":{"command":["hook.sh"]}}`,
		"empty hook":        `{"profile":"taskman-doctor-config/0","notify":{"command":[]}}`,
		"interval too long": `{"profile":"taskman-doctor-config/0","notify":{"command":["/bin/true"],"minIntervalSeconds":"604801"}}`,
		"unknown format":    `{"profile":"taskman-doctor-config/0","dispatchers":[{"programDir":"p","format":"gpt"}]}`,
		"no dispatchers":    `{"profile":"taskman-doctor-config/0","dispatchers":[]}`,
		"nine dispatchers":  `{"profile":"taskman-doctor-config/0","dispatchers":[` + strings.TrimSuffix(strings.Repeat(`{"programDir":"p"},`, 9), ",") + `]}`,
	} {
		x := atm(t, root, nil, "doctor", "--config", writeDoctorConfig(t, body))
		if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeMalformed) && !hasCode(x.res, wire.CodeUnsupportedVersion) {
			t.Fatalf("%s accepted: %s", name, x.stdout)
		}
	}
	if x := atm(t, root, nil, "doctor", "--config", filepath.Join(t.TempDir(), "missing.json")); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("missing config accepted: %s", x.stdout)
	}
	// Defaults: 80% file pressure fires at 80 of 100 and not at 79.
	for used, want := range map[uint64]int{79: 0, 80: 1} {
		restore := cli.SetDoctorTestHost(cli.DoctorTestHost{FilesUsed: used, FMax: 100})
		_, item := doctorOK(t, root, "--config", writeDoctorConfig(t, `{"profile":"taskman-doctor-config/0"}`))
		restore()
		if got := len(findingsOf(item, "FD_PRESSURE")); got != want {
			t.Fatalf("default fd threshold at %d: %s", used, wire.Encode(item))
		}
	}
}

// lockHost is a fake host where pid 4242 has run `ticket claim` for 90s
// holding taskman.lock and pid 4243 has run `queue status` for 10s with the
// lock open.
func lockHost(holder int) cli.DoctorTestHost {
	h := cli.DoctorTestHost{
		Procs: []cli.DoctorTestProc{
			{Pid: 4242, Elapsed: 90 * time.Second, Command: "/usr/local/bin/corvint-tasks ticket claim V1-0001"},
			{Pid: 4243, Elapsed: 10 * time.Second, Command: "corvint-tasks queue status"},
			{Pid: 4244, Elapsed: time.Hour, Command: "/usr/bin/vim taskman.lock"},
		},
		Openers: map[string][]int{"taskman.lock": {4242, 4243}},
		FMax:    100,
	}
	if holder != 0 {
		h.Holders = map[string]int{"taskman.lock": holder}
	}
	return h
}

// TQD-V0-021: a queue lock held past the threshold names its holder's pid,
// command and age and how many others wait; the finding clears when the
// lock is released and respects its configured threshold.
func TestTQDV0021_QueueLockHeld(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	for _, holder := range []int{4242, -1} {
		restore := cli.SetDoctorTestHost(lockHost(holder))
		_, item := doctorOK(t, root)
		restore()
		got := findingsOf(item, "QUEUE_LOCK_HELD")
		if len(got) != 1 || field(got[0], "who").Str != "taskman.lock" || field(got[0], "source").Str != "builtin" || field(got[0], "remedy").Str == "" {
			t.Fatalf("holder %d: %s", holder, wire.Encode(item))
		}
		d := field(got[0], "detail").Str
		for _, want := range []string{"pid 4242", "running 90s", "corvint-tasks ticket claim V1-0001", "1 other corvint-tasks process(es)"} {
			if !strings.Contains(d, want) {
				t.Fatalf("holder %d detail %q lacks %q", holder, d, want)
			}
		}
		if len(findingsOf(item, "SLOW_QUEUE_COMMAND")) != 0 {
			t.Fatalf("waiter under the threshold reported: %s", wire.Encode(item))
		}
	}
	// Released: no finding.
	restore := cli.SetDoctorTestHost(lockHost(0))
	_, item := doctorOK(t, root)
	restore()
	if len(findingsOf(item, "QUEUE_LOCK_HELD")) != 0 {
		t.Fatalf("released lock reported: %s", wire.Encode(item))
	}
	// A higher threshold suppresses it.
	cfg := writeDoctorConfig(t, `{"profile":"taskman-doctor-config/0","thresholds":{"lockHeldSeconds":"120"}}`)
	restore = cli.SetDoctorTestHost(lockHost(4242))
	_, item = doctorOK(t, root, "--config", cfg)
	restore()
	if len(findingsOf(item, "QUEUE_LOCK_HELD")) != 0 {
		t.Fatalf("threshold ignored: %s", wire.Encode(item))
	}
	// A lock held by a process that is not a corvint-tasks command is a
	// warning, not a guessed holder.
	h := lockHost(4244)
	h.Openers = map[string][]int{}
	restore = cli.SetDoctorTestHost(h)
	x, item := doctorOK(t, root)
	restore()
	if len(findingsOf(item, "QUEUE_LOCK_HELD")) != 0 || !strings.Contains(strings.Join(x.res.Warnings, "\n"), "not a corvint-tasks command") {
		t.Fatalf("foreign holder: %s", x.stdout)
	}
	// A known foreign holder is never replaced by the longest-waiting
	// corvint-tasks opener (pid 4242 has waited 90 s with it open).
	restore = cli.SetDoctorTestHost(lockHost(4244))
	x, item = doctorOK(t, root)
	restore()
	if len(findingsOf(item, "QUEUE_LOCK_HELD")) != 0 || !strings.Contains(strings.Join(x.res.Warnings, "\n"), "held by pid 4244, which is not a corvint-tasks command") {
		t.Fatalf("known foreign holder with a waiting opener: %s %v", x.stdout, x.res.Warnings)
	}
}

// TQD-V0-022: a short-lived corvint-tasks command with a queue lock open
// past the threshold is SLOW_QUEUE_COMMAND; long-running verbs, processes
// without this queue's lock open and other programs never are.
func TestTQDV0022_SlowQueueCommand(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	h := cli.DoctorTestHost{
		Procs: []cli.DoctorTestProc{
			{Pid: 5000, Elapsed: 75 * time.Second, Command: "corvint-tasks queue status"},
			{Pid: 5001, Elapsed: 2 * time.Hour, Command: "corvint-tasks run --program p"},
			{Pid: 5002, Elapsed: 2 * time.Hour, Command: "corvint-tasks gate run --ticket V1-0001"},
			{Pid: 5003, Elapsed: 2 * time.Hour, Command: "corvint-tasks ticket show V1-0001"},
			{Pid: 5004, Elapsed: 2 * time.Hour, Command: "/bin/cat taskman.prepare.lock"},
		},
		Openers: map[string][]int{"taskman.prepare.lock": {5000, 5001, 5002, 5004}},
		FMax:    100,
	}
	restore := cli.SetDoctorTestHost(h)
	_, item := doctorOK(t, root)
	restore()
	got := findingsOf(item, "SLOW_QUEUE_COMMAND")
	if len(got) != 1 || field(got[0], "who").Str != "pid 5000" || !strings.Contains(field(got[0], "detail").Str, "corvint-tasks queue running 75s") {
		t.Fatalf("slow command: %s", wire.Encode(item))
	}
	cfg := writeDoctorConfig(t, `{"profile":"taskman-doctor-config/0","thresholds":{"slowCommandSeconds":"76"}}`)
	restore = cli.SetDoctorTestHost(h)
	_, item = doctorOK(t, root, "--config", cfg)
	restore()
	if len(findingsOf(item, "SLOW_QUEUE_COMMAND")) != 0 {
		t.Fatalf("threshold ignored: %s", wire.Encode(item))
	}
	h.Openers = map[string][]int{}
	restore = cli.SetDoctorTestHost(h)
	_, item = doctorOK(t, root)
	restore()
	if len(findingsOf(item, "SLOW_QUEUE_COMMAND")) != 0 {
		t.Fatalf("closed lock reported: %s", wire.Encode(item))
	}
}

// TestDoctorLockHelperProcess is a subprocess for the real-probe test, run
// through a link named corvint-tasks: it opens the lock file named by
// DOCTOR_LOCK_HELPER, flocks it when DOCTOR_LOCK_HELPER_FLOCK is set,
// reports ready and waits for stdin to close.
func TestDoctorLockHelperProcess(t *testing.T) {
	path := os.Getenv("DOCTOR_LOCK_HELPER")
	if path == "" {
		t.Skip("helper process only")
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		os.Exit(2)
	}
	if os.Getenv("DOCTOR_LOCK_HELPER_FLOCK") != "" {
		if syscall.Flock(int(f.Fd()), syscall.LOCK_EX) != nil {
			os.Exit(3)
		}
	}
	// f is unused after Flock, so without the KeepAlive below a GC could
	// finalize it, closing the descriptor and releasing the lock while the
	// helper still runs (V1-1780). Forcing collection and letting finalizers
	// run before "ready" makes that regression fail every run.
	runtime.GC()
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	os.Stdout.WriteString("ready\n")
	_, _ = io.Copy(io.Discard, os.Stdin)
	runtime.KeepAlive(f)
	os.Exit(0)
}

// TQD-V0-021, TQD-V0-022 with the real host probes: a corvint-tasks
// process holding taskman.lock and another with taskman.prepare.lock open
// are reported once both outlive one-second thresholds, and neither is
// reported after they exit.
func TestTQDV0021_RealProbes(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	common := commonDirOf(t, root)
	bin := filepath.Join(t.TempDir(), "corvint-tasks")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(self, bin); err != nil {
		t.Fatal(err)
	}
	start := func(lock string, flock bool) (*exec.Cmd, io.WriteCloser) {
		t.Helper()
		c := exec.Command(bin, "-test.run=^TestDoctorLockHelperProcess$")
		c.Env = append(os.Environ(), "DOCTOR_LOCK_HELPER="+filepath.Join(common, lock))
		if flock {
			c.Env = append(c.Env, "DOCTOR_LOCK_HELPER_FLOCK=1")
		}
		in, err := c.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		out, err := c.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := c.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = in.Close(); _ = c.Process.Kill(); _ = c.Wait() })
		if line, err := bufio.NewReader(out).ReadString('\n'); err != nil || line != "ready\n" {
			t.Fatalf("helper: %q %v", line, err)
		}
		return c, in
	}
	holder, holderIn := start("taskman.lock", true)
	slow, slowIn := start("taskman.prepare.lock", false)
	time.Sleep(2100 * time.Millisecond) // ps reports whole seconds
	cfg := writeDoctorConfig(t, `{"profile":"taskman-doctor-config/0","thresholds":{"lockHeldSeconds":"1","slowCommandSeconds":"1"}}`)
	defer cli.UseRealDoctorProbes()()
	// A probe past its 5 s bound on a loaded host is a warning and no
	// finding (TQD-V0-021); retry a bounded number of times.
	x, item := doctorOK(t, root, "--config", cfg)
	for try := 1; try < 4 && strings.Contains(fmt.Sprint(x.res.Warnings), "probe incomplete"); try++ {
		t.Logf("try %d: %v", try, x.res.Warnings)
		x, item = doctorOK(t, root, "--config", cfg)
	}
	if strings.Contains(fmt.Sprint(x.res.Warnings), "probe incomplete") {
		// An incomplete probe must not invent a finding; the positive
		// case is then unobservable on this host.
		if n := len(findingsOf(item, "QUEUE_LOCK_HELD")) + len(findingsOf(item, "SLOW_QUEUE_COMMAND")); n != 0 {
			t.Fatalf("incomplete probe reported %d findings: %s", n, wire.Encode(item))
		}
		t.Skipf("lock opener probe incomplete on every try (host load): %v", x.res.Warnings)
	}
	held := findingsOf(item, "QUEUE_LOCK_HELD")
	if len(held) != 1 || field(held[0], "who").Str != "taskman.lock" || !strings.Contains(field(held[0], "detail").Str, "pid "+strconv.Itoa(holder.Process.Pid)) {
		t.Fatalf("held: %s warnings %v", wire.Encode(item), x.res.Warnings)
	}
	slowF := findingsOf(item, "SLOW_QUEUE_COMMAND")
	if len(slowF) != 1 || field(slowF[0], "who").Str != "pid "+strconv.Itoa(slow.Process.Pid) {
		t.Fatalf("slow: %s warnings %v", wire.Encode(item), x.res.Warnings)
	}
	_ = holderIn.Close()
	_ = slowIn.Close()
	_ = holder.Wait()
	_ = slow.Wait()
	_, item = doctorOK(t, root, "--config", cfg)
	if len(findingsOf(item, "QUEUE_LOCK_HELD"))+len(findingsOf(item, "SLOW_QUEUE_COMMAND")) != 0 {
		t.Fatalf("exited processes reported: %s", wire.Encode(item))
	}
	if used, max, err := cli.RealDoctorFileTable(); err != nil || max == 0 || used > max {
		t.Fatalf("file table: %d/%d %v", used, max, err)
	}
}

// TQD-V0-023: with ready tickets, idle workers and no completion for the
// window, NO_COMPLETIONS is reported; an empty queue, a running session or
// a recent completion clears it.
func TestTQDV0023_NoCompletions(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	at := func(d time.Duration) wire.Value {
		t.Helper()
		restore := cli.SetDoctorClock(func() time.Time { return time.Now().Add(d) })
		defer restore()
		_, item := doctorOK(t, root)
		return item
	}
	if got := findingsOf(at(3*time.Hour), "NO_COMPLETIONS"); len(got) != 0 {
		t.Fatal("empty queue reported")
	}
	a := planTicket(t, root, "ready-a", "P1", `["src/"]`)
	if got := findingsOf(at(time.Hour), "NO_COMPLETIONS"); len(got) != 0 {
		t.Fatal("reported inside the window")
	}
	item := at(3 * time.Hour)
	got := findingsOf(item, "NO_COMPLETIONS")
	if len(got) != 1 || !strings.Contains(field(got[0], "detail").Str, "1 ticket(s) are ready") || !strings.Contains(field(got[0], "detail").Str, "earliest scanned receipt") {
		t.Fatalf("no completions: %s", wire.Encode(item))
	}
	// A configured window.
	cfg := writeDoctorConfig(t, `{"profile":"taskman-doctor-config/0","thresholds":{"noCompletionsSeconds":"14400"}}`)
	restore := cli.SetDoctorClock(func() time.Time { return time.Now().Add(3 * time.Hour) })
	_, item = doctorOK(t, root, "--config", cfg)
	restore()
	if len(findingsOf(item, "NO_COMPLETIONS")) != 0 {
		t.Fatalf("window ignored: %s", wire.Encode(item))
	}
	// A running session is not an idle worker.
	c := handoffCLI(t, root, "claim", a, "--holder", "w", "--stage", "implement", "--request-id", "claim-a")
	if c.res.Outcome != wire.OutcomeOK {
		t.Fatalf("claim: %s", c.stdout)
	}
	planTicket(t, root, "ready-b", "P1", `["docs/"]`)
	if got := findingsOf(at(3*time.Hour), "NO_COMPLETIONS"); len(got) != 0 {
		t.Fatal("running session reported idle")
	}
	attempt, g := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	if r := handoffCLI(t, root, "release", "--attempt", attempt, "--generation", g, "--reason", wire.CodeHandoff, "--evidence", "no-change", "--request-id", "release-a"); r.res.Outcome != wire.OutcomeOK {
		t.Fatalf("release: %s", r.stdout)
	}
	if x := atm(t, root, nil, "ticket", "complete-manual", "--target", a, "--expected-revision", obligationRevision(t, root, a), "--request-id", "done-a", "--payload", `{"evidence":[],"reason":"manual"}`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("complete: %s", x.stdout)
	}
	if got := findingsOf(at(time.Hour), "NO_COMPLETIONS"); len(got) != 0 {
		t.Fatal("recent completion reported")
	}
	item = at(3 * time.Hour)
	if got := findingsOf(item, "NO_COMPLETIONS"); len(got) != 1 || !strings.Contains(field(got[0], "detail").Str, "the last completion") {
		t.Fatalf("after completion: %s", wire.Encode(item))
	}
}

// TQD-V0-024: FD_PRESSURE reports the system file table at or above the
// configured percentage and clears below it.
func TestTQDV0024_FDPressure(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	restore := cli.SetDoctorTestHost(cli.DoctorTestHost{FilesUsed: 9000, FMax: 10000})
	_, item := doctorOK(t, root)
	got := findingsOf(item, "FD_PRESSURE")
	if len(got) != 1 || field(got[0], "who").Str != "system" || !strings.Contains(field(got[0], "detail").Str, "9000 of 10000") {
		restore()
		t.Fatalf("fd pressure: %s", wire.Encode(item))
	}
	_, item = doctorOK(t, root, "--config", writeDoctorConfig(t, `{"profile":"taskman-doctor-config/0","thresholds":{"fdPressurePercent":"95"}}`))
	restore()
	if len(findingsOf(item, "FD_PRESSURE")) != 0 {
		t.Fatalf("threshold ignored: %s", wire.Encode(item))
	}
	_, item = doctorOK(t, root)
	if len(findingsOf(item, "FD_PRESSURE")) != 0 {
		t.Fatalf("idle host reported: %s", wire.Encode(item))
	}
}

// TestTQDV0021_ProbeParsers covers the neutral parsers of the Linux and
// darwin probes.
func TestTQDV0021_ProbeParsers(t *testing.T) {
	locks := "1: FLOCK  ADVISORY  WRITE 4242 fd:01:1234 0 EOF\n" +
		"1: -> FLOCK  ADVISORY  WRITE 4243 fd:01:1234 0 EOF\n" +
		"2: POSIX  ADVISORY  WRITE 99 fd:01:1234 0 EOF\n" +
		"3: FLOCK  ADVISORY  WRITE 77 fd:01:9999 0 EOF\n"
	if got := cli.ParseDoctorProcLocksForTest(locks, 0xfd, 1, 1234); len(got) != 1 || got[0] != 4242 {
		t.Fatalf("proc locks: %v", got)
	}
	if got := cli.ParseDoctorLsofForTest("p10\nf3\ni55\np11\ni55\ni56\n"); len(got[55]) != 2 || len(got[56]) != 1 || got[56][0] != 11 {
		t.Fatalf("lsof: %v", got)
	}
	if used, max, err := cli.ParseDoctorFileNrForTest("2048\t48\t10000\n"); err != nil || used != 2000 || max != 10000 {
		t.Fatalf("file-nr: %d %d %v", used, max, err)
	}
	if _, _, err := cli.ParseDoctorFileNrForTest("1 2"); err == nil {
		t.Fatal("malformed file-nr accepted")
	}
	for in, want := range map[string]time.Duration{"00:05": 5 * time.Second, "01:02:03": time.Hour + 2*time.Minute + 3*time.Second, "2-00:00:01": 48*time.Hour + time.Second} {
		if got, err := cli.ParseDoctorElapsedForTest(in); err != nil || got != want {
			t.Fatalf("elapsed %s: %v %v", in, got, err)
		}
	}
	if _, err := cli.ParseDoctorElapsedForTest("5"); err == nil {
		t.Fatal("malformed elapsed accepted")
	}
}

// notifyHook writes a hook that counts its runs, saves its stdin and then
// runs tail; it returns the hook path and the run counter.
func notifyHook(t *testing.T, tail string) (string, func() int, func() []byte) {
	t.Helper()
	dir := t.TempDir()
	hook := filepath.Join(dir, "hook.sh")
	body := "#!/bin/sh\necho run >> '" + dir + "/runs'\ncat > '" + dir + "/stdin'\n" + tail + "\n"
	if err := os.WriteFile(hook, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	runs := func() int {
		raw, _ := os.ReadFile(filepath.Join(dir, "runs"))
		return bytes.Count(raw, []byte("run\n"))
	}
	stdin := func() []byte { raw, _ := os.ReadFile(filepath.Join(dir, "stdin")); return raw }
	return hook, runs, stdin
}

// TQD-V0-026: the hook runs on --refresh once per change of the finding
// set, with the summary on stdin; an identical set never runs it again, a
// plain doctor never runs it, and minIntervalSeconds spaces runs.
func TestTQDV0026_NotifyOncePerChange(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	hook, runs, stdin := notifyHook(t, "exit 0")
	cfg := writeDoctorConfig(t, `{"profile":"taskman-doctor-config/0","notify":{"command":["`+hook+`","arg"]}}`)
	refresh := func(h cli.DoctorTestHost) wire.Value {
		t.Helper()
		restore := cli.SetDoctorTestHost(h)
		defer restore()
		_, item := doctorOK(t, root, "--refresh", "--config", cfg)
		return item
	}
	idle := cli.DoctorTestHost{FMax: 100}
	pressure := cli.DoctorTestHost{FilesUsed: 99, FMax: 100}
	refresh(idle)
	if runs() != 0 {
		t.Fatal("empty first set notified")
	}
	refresh(pressure)
	if runs() != 1 {
		t.Fatalf("runs %d after the first change", runs())
	}
	var summary map[string]any
	if err := json.Unmarshal(stdin(), &summary); err != nil || summary["profile"] != "taskman-doctor/0" || !bytes.Contains(stdin(), []byte(`"FD_PRESSURE"`)) {
		t.Fatalf("hook stdin: %s %v", stdin(), err)
	}
	// A refresh without the notify config keeps the notify history: the
	// next configured refresh of the same set does not run the hook again.
	restore := cli.SetDoctorTestHost(pressure)
	doctorOK(t, root, "--refresh")
	restore()
	refresh(pressure)
	restore = cli.SetDoctorTestHost(cli.DoctorTestHost{FilesUsed: 1, FMax: 100})
	doctorOK(t, root, "--config", cfg)
	restore()
	if runs() != 1 {
		t.Fatalf("runs %d after an identical set and a plain doctor", runs())
	}
	both := lockHost(4242)
	both.FilesUsed = 99
	item := refresh(both)
	if runs() != 2 || len(findingsOf(item, "NOTIFY_FAILED")) != 0 {
		t.Fatalf("runs %d after a changed set: %s", runs(), wire.Encode(item))
	}
	refresh(idle)
	if runs() != 3 {
		t.Fatalf("runs %d after the findings cleared", runs())
	}
	// A minimum interval holds a change until it has passed.
	spaced := writeDoctorConfig(t, `{"profile":"taskman-doctor-config/0","notify":{"command":["`+hook+`"],"minIntervalSeconds":"3600"}}`)
	restore = cli.SetDoctorTestHost(pressure)
	doctorOK(t, root, "--refresh", "--config", spaced)
	restore()
	if runs() != 3 {
		t.Fatalf("runs %d inside the minimum interval", runs())
	}
	restoreClock := cli.SetDoctorClock(func() time.Time { return time.Now().Add(2 * time.Hour) })
	restore = cli.SetDoctorTestHost(pressure)
	doctorOK(t, root, "--refresh", "--config", spaced)
	restore()
	restoreClock()
	if runs() != 4 {
		t.Fatalf("runs %d after the minimum interval", runs())
	}
}

// TQD-V0-026: a failing, slow or noisy hook becomes NOTIFY_FAILED and never
// fails the doctor; the failure is re-emitted by a plain doctor and the
// hook is not retried for the same set.
func TestTQDV0026_NotifyFailureIsAFinding(t *testing.T) {
	// The default 10 s hook timeout: a fresh script's first exec can take
	// seconds on a loaded host.
	for name, c := range map[string]struct{ tail, want string }{
		"exit":    {"exit 3", "notify hook failed"},
		"timeout": {"exec /bin/sleep 60", "timed out"},
		"output":  {"/usr/bin/head -c 70000 /dev/zero", "output larger than"},
	} {
		root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
		hook, runs, _ := notifyHook(t, c.tail)
		cfg := writeDoctorConfig(t, `{"profile":"taskman-doctor-config/0","notify":{"command":["`+hook+`"]}}`)
		restore := cli.SetDoctorTestHost(cli.DoctorTestHost{FilesUsed: 99, FMax: 100})
		_, item := doctorOK(t, root, "--refresh", "--config", cfg)
		got := findingsOf(item, "NOTIFY_FAILED")
		if len(got) != 1 || field(got[0], "who").Str != "notify" || !strings.Contains(field(got[0], "detail").Str, c.want) || len(findingsOf(item, "FD_PRESSURE")) != 1 {
			restore()
			t.Fatalf("%s: %s", name, wire.Encode(item))
		}
		doctorOK(t, root, "--refresh")
		_, item = doctorOK(t, root, "--config", cfg)
		if len(findingsOf(item, "NOTIFY_FAILED")) != 1 {
			restore()
			t.Fatalf("%s: a plain doctor after an unconfigured refresh lost the failure: %s", name, wire.Encode(item))
		}
		_, item = doctorOK(t, root, "--refresh", "--config", cfg)
		restore()
		if runs() != 1 || len(findingsOf(item, "NOTIFY_FAILED")) != 1 {
			t.Fatalf("%s: runs %d after an identical set: %s", name, runs(), wire.Encode(item))
		}
		if _, item = doctorOK(t, root); len(findingsOf(item, "NOTIFY_FAILED")) != 0 {
			t.Fatalf("%s: failure reported without the config", name)
		}
	}
}

// writeWorkerLog writes a worker's stdout.log under programDir.
func writeWorkerLog(t *testing.T, programDir, worker string, lines ...string) string {
	t.Helper()
	dir := filepath.Join(programDir, "workers", worker)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "stdout.log")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TQD-V0-027, TQD-V0-028, TQD-V0-029: retry storms, host errors and early
// refusals come only from recognised host lines of recent worker logs,
// with counts, the worker id and the cited line range.
func TestTQDV0027_HostSessionFindings(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	prog := filepath.Join(t.TempDir(), "codex-prog")
	start := `{"type":"turn.started"}`
	done := `{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1}}`
	fail := `{"type":"item.completed","item":{"id":"i","type":"command_execution","exit_code":1,"status":"failed"}}`
	ok := `{"type":"item.completed","item":{"id":"i","type":"command_execution","exit_code":0,"status":"completed"}}`
	errLine := `{"type":"error","message":"stream disconnected"}`
	stormy := writeWorkerLog(t, prog, "w1", start, done, start, start, start, "plain text", errLine, errLine, errLine)
	refused := writeWorkerLog(t, prog, "w2", start, fail, fail, fail, ok, fail, done)
	writeWorkerLog(t, prog, "w3", "turn.started", `{"type":"turn.started","Type":"x"}`, "error", `{"type":"thread.started"}`, "", "{")
	old := writeWorkerLog(t, prog, "w4", start, start, start, start)
	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}
	cfg := writeDoctorConfig(t, `{"profile":"taskman-doctor-config/0","dispatchers":[{"programDir":"`+prog+`","format":"codex"}]}`)
	_, item := doctorOK(t, root, "--config", cfg)
	check := func(kind, who, want string) {
		t.Helper()
		var hit []wire.Value
		for _, f := range findingsOf(item, kind) {
			if field(f, "who").Str == who {
				hit = append(hit, f)
			}
		}
		if len(hit) != 1 || field(hit[0], "detail").Str != want {
			t.Fatalf("%s %s: want %q in %s", kind, who, want, wire.Encode(item))
		}
	}
	check("HOST_RETRY_STORM", "w1", "3 turn starts with no completed turn between them (codex) in "+stormy+" lines 3-5")
	check("HOST_ERRORS", "w1", "3 host error events (codex) in "+stormy+" lines 7-9")
	check("EARLY_REFUSAL", "w2", "3 failed tool calls before the first successful one (codex) in "+refused+" lines 2-4")
	if n := len(findingsOf(item, "HOST_RETRY_STORM")) + len(findingsOf(item, "HOST_ERRORS")) + len(findingsOf(item, "EARLY_REFUSAL")); n != 3 {
		t.Fatalf("unrecognised or old lines reported: %s", wire.Encode(item))
	}
	// Thresholds are configurable; no declared format reads nothing.
	cfg = writeDoctorConfig(t, `{"profile":"taskman-doctor-config/0","thresholds":{"retryStorm":"4","hostErrors":"4","earlyRefusal":"4"},"dispatchers":[{"programDir":"`+prog+`","format":"codex"}]}`)
	_, item = doctorOK(t, root, "--config", cfg)
	if n := len(findingsOf(item, "HOST_RETRY_STORM")) + len(findingsOf(item, "HOST_ERRORS")) + len(findingsOf(item, "EARLY_REFUSAL")); n != 0 {
		t.Fatalf("thresholds ignored: %s", wire.Encode(item))
	}
	cfg = writeDoctorConfig(t, `{"profile":"taskman-doctor-config/0","dispatchers":[{"programDir":"`+prog+`"}]}`)
	_, item = doctorOK(t, root, "--config", cfg)
	if n := len(findingsOf(item, "HOST_RETRY_STORM")) + len(findingsOf(item, "HOST_ERRORS")) + len(findingsOf(item, "EARLY_REFUSAL")); n != 0 {
		t.Fatalf("undeclared format read: %s", wire.Encode(item))
	}
	// The same storm read as Claude Code output is not recognised.
	cfg = writeDoctorConfig(t, `{"profile":"taskman-doctor-config/0","dispatchers":[{"programDir":"`+prog+`","format":"claude-code"}]}`)
	_, item = doctorOK(t, root, "--config", cfg)
	if n := len(findingsOf(item, "HOST_RETRY_STORM")) + len(findingsOf(item, "HOST_ERRORS")) + len(findingsOf(item, "EARLY_REFUSAL")); n != 0 {
		t.Fatalf("foreign format read: %s", wire.Encode(item))
	}
}

// TQD-V0-027: past 256 recent sessions the doctor reads the first 256 by
// worker name, whatever order the directory lists them in.
func TestTQDV0027_WorkerLimitInNameOrder(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	prog := filepath.Join(t.TempDir(), "many-prog")
	errLine := `{"type":"error","message":"stream disconnected"}`
	quiet := `{"type":"turn.started"}`
	// Created in reverse name order, so a directory listing in creation
	// order would put the last-named worker first.
	writeWorkerLog(t, prog, "w999", errLine, errLine, errLine)
	for i := 255; i >= 1; i-- {
		writeWorkerLog(t, prog, fmt.Sprintf("w%03d", i), quiet)
	}
	writeWorkerLog(t, prog, "w000", errLine, errLine, errLine)
	cfg := writeDoctorConfig(t, `{"profile":"taskman-doctor-config/0","dispatchers":[{"programDir":"`+prog+`","format":"codex"}]}`)
	x, item := doctorOK(t, root, "--config", cfg)
	got := findingsOf(item, "HOST_ERRORS")
	if len(got) != 1 || field(got[0], "who").Str != "w000" {
		t.Fatalf("want only w000 read past the limit: %s", wire.Encode(item))
	}
	if !strings.Contains(strings.Join(x.res.Warnings, "\n"), "more than 256 recent worker sessions") {
		t.Fatalf("limit not warned: %s", x.stdout)
	}
}

// writeLedger writes a dispatcher state.json with the given health.
func writeLedger(t *testing.T, programDir string, h *dispatch.WorkStateHealth) {
	t.Helper()
	if err := os.MkdirAll(programDir, 0o700); err != nil {
		t.Fatal(err)
	}
	l := dispatch.Ledger{Profile: dispatch.StateProfile, Program: filepath.Base(programDir), Workers: []*dispatch.Worker{}, Backoff: map[string]*dispatch.BackoffState{}, WorkState: h}
	raw, err := json.Marshal(&l)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(programDir, "state.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

// TQD-V0-031: the doctor reports each configured dispatcher's work-state
// reader health and a failing reader as WORK_STATE_FAILING.
func TestTQDV0031_WorkStateHealth(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	prog := filepath.Join(t.TempDir(), "prog")
	cfg := writeDoctorConfig(t, `{"profile":"taskman-doctor-config/0","dispatchers":[{"programDir":"`+prog+`"}]}`)
	health := func() (wire.Value, wire.Value, run) {
		t.Helper()
		x, item := doctorOK(t, root, "--config", cfg)
		d := field(item, "dispatchers")
		if len(d.Arr) != 1 || field(d.Arr[0], "programDir").Str != prog {
			t.Fatalf("dispatchers: %s", wire.Encode(item))
		}
		return field(d.Arr[0], "workState"), item, x
	}
	if ws, _, _ := health(); field(ws, "health").Str != "NOT_OBSERVED" {
		t.Fatalf("absent state: %s", wire.Encode(ws))
	}
	since := time.Date(2026, 10, 17, 1, 2, 3, 0, time.UTC)
	writeLedger(t, prog, &dispatch.WorkStateHealth{OK: false, Since: since, LastError: "reader exited 2", FailedTicks: 3, Escalated: true})
	ws, item, _ := health()
	if field(ws, "health").Str != "FAILING" || field(ws, "ok").Bool || field(ws, "since").Str != "2026-10-17T01:02:03Z" || field(ws, "lastError").Str != "reader exited 2" || field(ws, "failedTicks").Str != "3" || !field(ws, "escalated").Bool {
		t.Fatalf("failing: %s", wire.Encode(ws))
	}
	got := findingsOf(item, "WORK_STATE_FAILING")
	if len(got) != 1 || field(got[0], "who").Str != prog || field(got[0], "firstSeen").Str != "2026-10-17T01:02:03Z" {
		t.Fatalf("finding: %s", wire.Encode(item))
	}
	writeLedger(t, prog, &dispatch.WorkStateHealth{OK: true, Since: since})
	ws, item, _ = health()
	if field(ws, "health").Str != "OK" || !field(ws, "ok").Bool || field(ws, "lastError").Str != "NONE" || field(ws, "failedTicks").Str != "0" || len(findingsOf(item, "WORK_STATE_FAILING")) != 0 {
		t.Fatalf("ok: %s", wire.Encode(item))
	}
	if err := os.WriteFile(filepath.Join(prog, "state.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	ws, _, x := health()
	if field(ws, "health").Str != "UNREADABLE" || !strings.Contains(strings.Join(x.res.Warnings, "\n"), "state unreadable") {
		t.Fatalf("unreadable: %s", x.stdout)
	}
	// A FIFO in place of state.json is refused without blocking on it.
	state := filepath.Join(prog, "state.json")
	if err := os.Remove(state); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(state, 0o600); err != nil {
		t.Fatal(err)
	}
	ws, _, x = health()
	if field(ws, "health").Str != "UNREADABLE" || !strings.Contains(strings.Join(x.res.Warnings, "\n"), "not a regular file") {
		t.Fatalf("fifo: %s", x.stdout)
	}
	if _, item := doctorOK(t, root); field(item, "dispatchers").Kind != 0 && len(field(item, "dispatchers").Arr) != 0 {
		t.Fatalf("dispatchers without a config: %s", wire.Encode(item))
	}
}

// TQD-V0-032: a reopen ends the NO_PROGRESS_HANDOFF streak; hand-offs
// before it never count toward the reopened ticket's streak.
func TestTQDV0032_ReopenResetsNoProgressStreak(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	id := planTicket(t, root, "reopened", "P1", `["src/"]`)
	loopCLIHandoffs(t, root, id, 3)
	if _, item := doctorOK(t, root); len(findingsOf(item, "NO_PROGRESS_HANDOFF")) != 1 {
		t.Fatalf("streak: %s", wire.Encode(item))
	}
	if x := atm(t, root, nil, "ticket", "complete-manual", "--target", id, "--expected-revision", obligationRevision(t, root, id), "--request-id", "done", "--payload", `{"evidence":[],"reason":"manual"}`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("complete: %s", x.stdout)
	}
	if x := atm(t, root, nil, "ticket", "reopen", "--target", id, "--expected-revision", obligationRevision(t, root, id), "--request-id", "reopen", "--role", "OWNER", "--payload", `{"reason":"regressed"}`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("reopen: %s", x.stdout)
	}
	if _, item := doctorOK(t, root); len(findingsOf(item, "NO_PROGRESS_HANDOFF")) != 0 {
		t.Fatalf("streak survived the reopen: %s", wire.Encode(item))
	}
	for i := 0; i < 3; i++ {
		n := strconv.Itoa(i)
		c := handoffCLI(t, root, "claim", id, "--holder", "r"+n, "--stage", "implement", "--request-id", "rc-"+n)
		if c.res.Outcome != wire.OutcomeOK {
			t.Fatalf("claim: %s", c.stdout)
		}
		a, g := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
		if r := handoffCLI(t, root, "release", "--attempt", a, "--generation", g, "--reason", wire.CodeHandoff, "--evidence", "no-change", "--request-id", "rr-"+n); r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("release: %s", r.stdout)
		}
		_, item := doctorOK(t, root)
		if got := len(findingsOf(item, "NO_PROGRESS_HANDOFF")); (i < 2 && got != 0) || (i == 2 && (got != 1 || len(field(findingsOf(item, "NO_PROGRESS_HANDOFF")[0], "evidenceSeqs").Arr) != 3)) {
			t.Fatalf("after %d new hand-offs: %s", i+1, wire.Encode(item))
		}
	}
}

// TQD-V0-033 measurement: with CORVINT_DOCTOR_SCAN_COST=N, N claim and
// release cycles build a synthetic journal and the doctor's scan time is
// logged. It is opt-in because building the journal takes minutes.
func TestTQDV0033_ScanCostMeasurement(t *testing.T) {
	n, err := strconv.Atoi(os.Getenv("CORVINT_DOCTOR_SCAN_COST"))
	if err != nil || n < 1 {
		t.Skip("set CORVINT_DOCTOR_SCAN_COST=N to measure")
	}
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	built := time.Now()
	for i := 0; i < n; i += 50 {
		id := planTicket(t, root, "cost-"+strconv.Itoa(i), "P1", `["src/`+strconv.Itoa(i)+`/"]`)
		for j := i; j < min(i+50, n); j++ {
			k := strconv.Itoa(j)
			c := handoffCLI(t, root, "claim", id, "--holder", "w"+k, "--stage", "implement", "--request-id", "cost-claim-"+k)
			if c.res.Outcome != wire.OutcomeOK {
				t.Fatalf("claim %d: %s", j, c.stdout)
			}
			a, g := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
			if r := handoffCLI(t, root, "release", "--attempt", a, "--generation", g, "--reason", wire.CodeHandoff, "--evidence", "no-change", "--request-id", "cost-release-"+k); r.res.Outcome != wire.OutcomeOK {
				t.Fatalf("release %d: %s", j, r.stdout)
			}
		}
	}
	t.Logf("built %d cycles in %v", n, time.Since(built))
	// Interleaved rounds; CPU time (user+system) is less sensitive to host
	// load than wall time.
	cpu := func() time.Duration {
		var ru syscall.Rusage
		_ = syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
		return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
	}
	var wall, used [2]time.Duration
	var items [2]wire.Value
	restoreClock := cli.SetDoctorClock(func() time.Time { return built })
	defer restoreClock()
	for i := 0; i < 10; i++ {
		mode := i % 2
		restore := func() {}
		if mode == 0 {
			restore = cli.SetDoctorPerReadJournal()
		}
		start, c0 := time.Now(), cpu()
		_, items[mode] = doctorOK(t, root)
		w, c := time.Since(start), cpu()-c0
		restore()
		if i < 2 || w < wall[mode] {
			wall[mode] = w
		}
		if i < 2 || c < used[mode] {
			used[mode] = c
		}
	}
	old, item := items[0], items[1]
	t.Logf("doctor best of 5 (wall / cpu): per-read sessions %v / %v, one session %v / %v; scan %s", wall[0], used[0], wall[1], used[1], wire.Encode(field(item, "scan")))
	assertDoctorModesEqual(t, old, item)
}

// assertDoctorModesEqual requires the complete findings and scan values of
// a per-read run and a one-session run to be identical.
func assertDoctorModesEqual(t *testing.T, perRead, session wire.Value) {
	t.Helper()
	for _, name := range []string{"findings", "scan"} {
		if a, b := wire.Encode(field(perRead, name)), wire.Encode(field(session, name)); !bytes.Equal(a, b) {
			t.Fatalf("%s differ between read modes:\n%s\n%s", name, a, b)
		}
	}
}

// TQD-V0-033: with the observation clock frozen, the one-session journal
// read and per-read sessions produce identical findings and scan values.
func TestTQDV0033_ReadModesEquivalent(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	id := planTicket(t, root, "modes", "P1", `["src/"]`)
	loopCLIHandoffs(t, root, id, 3)
	frozen := time.Now().UTC()
	restoreClock := cli.SetDoctorClock(func() time.Time { return frozen })
	defer restoreClock()
	restore := cli.SetDoctorPerReadJournal()
	_, perRead := doctorOK(t, root)
	restore()
	_, session := doctorOK(t, root)
	if len(field(session, "findings").Arr) == 0 || field(field(session, "scan"), "receipts").Str == "0" {
		t.Fatalf("no findings or receipts to compare: %s", wire.Encode(session))
	}
	assertDoctorModesEqual(t, perRead, session)
}

// TQD-V0-821: when the stat device differs from the one /proc/locks prints
// (btrfs), the same-inode FLOCK lines are candidates on any device; a real
// holder is then named only through a descriptor on the same file.
func TestTQDV0821LockHolderMatchesByOpenDescriptor(t *testing.T) {
	locks := "1: FLOCK  ADVISORY  WRITE 4242 00:2f:1234 0 EOF\n" +
		"2: POSIX  ADVISORY  WRITE 5151 00:2f:1234 0 EOF\n" +
		"3: FLOCK  ADVISORY  WRITE 6262 00:30:91234 0 EOF\n" +
		"4: FLOCK  ADVISORY  WRITE 7373 08:01:1234 0 EOF\n"
	if got := cli.ParseDoctorProcLocksForTest(locks, 0, 0x48, 1234); len(got) != 0 {
		t.Fatalf("exact match on the stat device 00:48 = %v, want none", got)
	}
	if got := cli.ParseDoctorProcLocksByInodeForTest(locks, 1234); len(got) != 2 || got[0] != 4242 || got[1] != 7373 {
		t.Fatalf("same-inode FLOCK candidates = %v, want [4242 7373]", got)
	}
	// A real flock on this host's temporary filesystem, held by this
	// process, is named whatever device /proc/locks prints.
	path := filepath.Join(t.TempDir(), "taskman.lock")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	held, pid, err := cli.RealDoctorLockHolder(path)
	if runtime.GOOS != "linux" {
		return
	}
	if err != nil || !held || pid != os.Getpid() {
		t.Fatalf("held=%v pid=%d err=%v, want this process %d", held, pid, err, os.Getpid())
	}
}

// TQD-V0-821, Codex review of linux-w2: an open descriptor on the lock file is
// not ownership. A pid that holds a same-inode flock on another filesystem's
// file while merely having this queue's file open is not the holder; only a
// descriptor on this very file whose fdinfo lists the FLOCK is.
func TestTQDV0821OpenDescriptorWithoutLockIsNotOwnership(t *testing.T) {
	const dev, ino, otherDev = 0x48, 1234, 0x31
	lock := "pos:\t0\nflags:\t02100001\nmnt_id:\t120\nino:\t1234\nlock:\t1: FLOCK  ADVISORY  WRITE 4242 00:2f:1234 0 EOF\n"
	open := "pos:\t0\nflags:\t02100000\nmnt_id:\t120\nino:\t1234\n"
	for name, tc := range map[string]struct {
		fds  []cli.DoctorFdForTest
		want bool
	}{
		"owning descriptor on this file": {[]cli.DoctorFdForTest{{Dev: dev, Ino: ino, Fdinfo: lock}}, true},
		"this file merely open, the lock on another filesystem's same-inode file": {[]cli.DoctorFdForTest{
			{Dev: dev, Ino: ino, Fdinfo: open},
			{Dev: otherDev, Ino: ino, Fdinfo: lock},
		}, false},
		"this file merely open":         {[]cli.DoctorFdForTest{{Dev: dev, Ino: ino, Fdinfo: open}}, false},
		"a lock line for another inode": {[]cli.DoctorFdForTest{{Dev: dev, Ino: ino, Fdinfo: strings.Replace(lock, "00:2f:1234", "00:2f:91234", 1)}}, false},
	} {
		if got := cli.DoctorOwnsFlockForTest(tc.fds, dev, ino); got != tc.want {
			t.Errorf("%s: owns = %v, want %v", name, got, tc.want)
		}
	}
	// A real second descriptor on a flocked file does not own the lock: only
	// the locking descriptor's fdinfo lists it.
	path := filepath.Join(t.TempDir(), "taskman.lock")
	locker, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer locker.Close()
	reader, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if err := syscall.Flock(int(locker.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "linux" {
		return
	}
	read := func(f *os.File) string {
		raw, err := os.ReadFile(fmt.Sprintf("/proc/self/fdinfo/%d", f.Fd()))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		t.Fatal(err)
	}
	only := []cli.DoctorFdForTest{{Dev: uint64(st.Dev), Ino: uint64(st.Ino), Fdinfo: read(reader)}}
	both := append(only, cli.DoctorFdForTest{Dev: uint64(st.Dev), Ino: uint64(st.Ino), Fdinfo: read(locker)})
	if cli.DoctorOwnsFlockForTest(only, uint64(st.Dev), uint64(st.Ino)) || !cli.DoctorOwnsFlockForTest(both, uint64(st.Dev), uint64(st.Ino)) {
		t.Fatalf("real fdinfo: reader alone owns=%v, with the locker owns=%v; want false, true", cli.DoctorOwnsFlockForTest(only, uint64(st.Dev), uint64(st.Ino)), cli.DoctorOwnsFlockForTest(both, uint64(st.Dev), uint64(st.Ino)))
	}
}
