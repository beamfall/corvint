//go:build darwin || linux

package dispatch

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestCALV0143_WorkerLogsAreCappedWhileTheWorkerRuns runs a worker that
// writes four segments to each stream and then blocks. One tick truncates
// both live logs in place and keeps each stream's newest bytes, behind a
// marker line, in a rotated segment no larger than the cap. The worker keeps
// appending to the truncated file, and the finished summary reads its final
// message across the two segments.
func TestCALV0143_WorkerLogsAreCappedWhileTheWorkerRuns(t *testing.T) {
	defer func(n int64) { workerLogSegmentBytes = n }(workerLogSegmentBytes)
	workerLogSegmentBytes = 64 << 10
	c := testConfig(t, `i=0
while [ $i -lt 640 ]; do
  printf '{"type":"step_start","pad":"%0400d"}\n' $i
  printf 'stderr line %0400d\n' $i >&2
  i=$((i+1))
done
while [ ! -e go.flag ]; do sleep 0.05; done
printf '{"type":"text","part":{"text":"capped done"}}\n'`)
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if err := d.Tick(ctx); err != nil || d.Running() != 1 {
		t.Fatalf("tick: %v running %d", err, d.Running())
	}
	w := d.ledger.Workers[0]
	dir := d.workerDir(w.ID)
	size := func(name string) int64 {
		st, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			return -1
		}
		return st.Size()
	}
	for i := 0; size("stderr.log") < 640*413 || size("stdout.log") < 640*431; i++ {
		if i == 400 {
			t.Fatalf("worker output did not complete: %d, %d", size("stdout.log"), size("stderr.log"))
		}
		time.Sleep(25 * time.Millisecond)
	}
	if err := d.Tick(ctx); err != nil || d.Running() != 1 {
		t.Fatalf("supervising tick: %v running %d", err, d.Running())
	}
	for _, name := range []string{"stdout.log", "stderr.log"} {
		if got := size(name); got != 0 {
			t.Fatalf("%s holds %d bytes after the cap", name, got)
		}
		rotated, err := os.ReadFile(filepath.Join(dir, name+".1"))
		if err != nil || int64(len(rotated)) > workerLogSegmentBytes || int64(len(rotated)) < workerLogSegmentBytes-1024 {
			t.Fatalf("%s.1 = %d bytes (%v), want at most %d", name, len(rotated), err, workerLogSegmentBytes)
		}
		head, _, _ := strings.Cut(string(rotated), "\n")
		if !strings.HasPrefix(head, "corvint-tasks dispatch: "+name+" reached ") {
			t.Fatalf("%s.1 marker = %q", name, head)
		}
		if !strings.Contains(string(rotated), fmt.Sprintf("%0400d", 639)) {
			t.Fatalf("%s.1 lost the newest line", name)
		}
	}
	if w.LogBytes != 0 {
		t.Fatalf("recorded log bytes %d after the cap", w.LogBytes)
	}
	if err := os.WriteFile(filepath.Join(c.WorkRoot, "go.flag"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	waitEnded(t, d)
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if got := size("stdout.log"); got <= 0 || got > 64 {
		t.Fatalf("the worker's append after truncation left stdout.log at %d bytes", got)
	}
	events, _ := ReadEvents(d.dir, 1000)
	found := false
	for _, e := range events {
		if e.Kind == "finished" {
			found = true
			if !strings.HasSuffix(e.Message, ": capped done") {
				t.Fatalf("finished summary = %q", e.Message)
			}
		}
	}
	if !found {
		t.Fatal("no finished event")
	}
}

// TestCALV0143_CappedOutputCountsAsActivity writes more than a segment into a
// worker's log between two ticks whose recorded size is zero, as after an
// earlier cut. The comparison precedes the cut, so the burst counts as
// activity, and the size left after the cut is what the next tick compares.
func TestCALV0143_CappedOutputCountsAsActivity(t *testing.T) {
	defer func(n int64) { workerLogSegmentBytes = n }(workerLogSegmentBytes)
	workerLogSegmentBytes = 4 << 10
	d, err := Open("prog", testConfig(t, "exit 0"), &fakeQueue{}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	w := &Worker{ID: "burst", PID: -1}
	dir := d.workerDir(w.ID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	burst := strings.Repeat("output line\n", 1024)
	if err := os.WriteFile(filepath.Join(dir, "stdout.log"), []byte(burst), 0o600); err != nil {
		t.Fatal(err)
	}
	if !d.active(w, map[int]proc{}, Host{}) {
		t.Fatal("a burst that was then cut did not count as activity")
	}
	if w.LogBytes != 0 {
		t.Fatalf("recorded log bytes %d, want the size after the cut", w.LogBytes)
	}
	if d.active(w, map[int]proc{}, Host{}) {
		t.Fatal("an unchanged log counted as activity")
	}
}

// TestCALV0144_FinishedWorkerDirsAreRetired fills workers/ with 40 quiet
// finished directories and one that changed within the quiet window. When a
// real worker finishes, only the 32 newest quiet finished directories stay
// beside the recent one and the just-finished worker's own; a directory an
// infrastructure retry names, or whose recorded tree may still run, is kept
// however old it is.
func TestCALV0144_FinishedWorkerDirsAreRetired(t *testing.T) {
	c := testConfig(t, `printf '{"type":"text","part":{"text":"done"}}\n'`)
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	now := time.Now()
	plant := func(name string, at time.Time) {
		dir := d.workerDir(name)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		for _, f := range []string{"stdout.log", "stderr.log"} {
			p := filepath.Join(dir, f)
			if err := os.WriteFile(p, []byte("old\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			os.Chtimes(p, at, at)
		}
		os.Chtimes(dir, at, at)
	}
	for i := 0; i < 40; i++ {
		plant(fmt.Sprintf("old-%02d", i), now.Add(-2*time.Hour-time.Duration(i)*time.Minute))
	}
	plant("recent", now.Add(-5*time.Minute))
	ctx := context.Background()
	if err := d.Tick(ctx); err != nil || d.Running() != 1 {
		t.Fatalf("tick: %v running %d", err, d.Running())
	}
	id := d.ledger.Workers[0].ID
	// A running worker's tick retires nothing: no worker has finished.
	if _, err := os.Stat(d.workerDir("old-39")); err != nil {
		t.Fatalf("retired before any worker finished: %v", err)
	}
	waitEnded(t, d)
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if !has(kinds(t, d), "finished") {
		t.Fatal("the worker did not finish")
	}
	// The finishing tick only marks the removable directories; a pass at
	// least a minute later removes them.
	for i := 0; i < 40; i++ {
		name := fmt.Sprintf("old-%02d", i)
		if _, err := os.Stat(d.workerDir(name)); err != nil {
			t.Fatalf("%s was removed by the marking pass: %v", name, err)
		}
		if marked(d, name) != (i >= maxRetainedWorkerDirs) {
			t.Errorf("%s marked=%v", name, marked(d, name))
		}
	}
	later := func(by time.Duration) {
		d.Now = func() time.Time { return time.Now().Add(by) }
		d.retireWorkerDirs()
	}
	later(2 * time.Minute)
	for i := 0; i < 40; i++ {
		name := fmt.Sprintf("old-%02d", i)
		_, err := os.Stat(d.workerDir(name))
		if kept := err == nil; kept != (i < maxRetainedWorkerDirs) || marked(d, name) {
			t.Errorf("%s kept=%v marked=%v", name, kept, marked(d, name))
		}
	}
	for _, name := range []string{"recent", id} {
		if _, err := os.Stat(filepath.Join(d.workerDir(name), "stdout.log")); err != nil {
			t.Errorf("%s was retired: %v", name, err)
		}
	}
	if _, _, err := readLeader(d.workerDir(id)); err != nil {
		t.Fatalf("launch kept no leader: %v", err)
	}
	// Directories the ledger never recorded, as after a crash between launch
	// and the ledger save, are kept while their recorded tree may run: a live
	// leader, or a session whose leader exited but whose child still runs. A
	// leader that is gone with no member left is retired.
	leader := func(name string, pid int, identity string) {
		plant(name, now.Add(-12*time.Hour))
		if err := os.WriteFile(filepath.Join(d.workerDir(name), workerLeaderName), []byte(fmt.Sprintf("%d %s\n", pid, identity)), 0o600); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(d.workerDir(name), now.Add(-12*time.Hour), now.Add(-12*time.Hour))
	}
	self, err := processIdentity(os.Getpid())
	if err != nil || self == "" {
		t.Fatalf("own identity: %q %v", self, err)
	}
	leader("live-leader", os.Getpid(), self)
	orphan := exec.Command("/bin/sh", "-c", "sleep 60 & exit 0")
	orphan.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := orphan.Start(); err != nil {
		t.Fatal(err)
	}
	defer syscall.Kill(-orphan.Process.Pid, syscall.SIGKILL)
	orphanID, err := processIdentity(orphan.Process.Pid)
	if err != nil || orphanID == "" {
		t.Fatalf("orphan identity: %q %v", orphanID, err)
	}
	if err := orphan.Wait(); err != nil {
		t.Fatal(err)
	}
	leader("live-session", orphan.Process.Pid, orphanID)
	gone := exec.Command("/bin/sh", "-c", "exit 0")
	if err := gone.Run(); err != nil {
		t.Fatal(err)
	}
	leader("gone-leader", gone.Process.Pid, "gone-identity")
	plant("bad-leader", now.Add(-12*time.Hour))
	os.WriteFile(filepath.Join(d.workerDir("bad-leader"), workerLeaderName), []byte("not a leader"), 0o600)
	os.Chtimes(d.workerDir("bad-leader"), now.Add(-12*time.Hour), now.Add(-12*time.Hour))
	// The oldest directory stays while a reserved infrastructure retry names
	// it, since its absence would prove that launch never spawned.
	plant("infra-held", now.Add(-10*time.Hour))
	d.ledger.InfraRetry = map[string]*InfraEpisode{"ticket:a:q:other": {State: InfraReserved, Launch: "infra-held"}}
	later(4 * time.Minute)
	later(6 * time.Minute)
	if _, err := os.Stat(d.workerDir("infra-held")); err != nil {
		t.Fatalf("a directory named by an infrastructure retry was retired: %v", err)
	}
	for _, name := range []string{"live-leader", "live-session", "bad-leader"} {
		if _, err := os.Stat(d.workerDir(name)); err != nil {
			t.Errorf("%s was retired: %v", name, err)
		}
	}
	if _, err := os.Stat(d.workerDir("gone-leader")); !os.IsNotExist(err) {
		t.Errorf("a directory whose tree is gone was kept: %v", err)
	}
	d.ledger.InfraRetry = nil
	later(8 * time.Minute)
	later(10 * time.Minute)
	if _, err := os.Stat(d.workerDir("infra-held")); !os.IsNotExist(err) {
		t.Fatalf("an unprotected oldest directory was kept: %v", err)
	}
}

// TestCALV0144_ProtectedDirsTakeNoRetentionSlot plants 33 quiet finished
// directories whose recorded leader is this live process, newer than one
// finished directory with no leader. The protected directories are kept and
// take none of the 32 retention slots, so the older finished one stays too.
func TestCALV0144_ProtectedDirsTakeNoRetentionSlot(t *testing.T) {
	d, err := Open("prog", testConfig(t, "exit 0"), &fakeQueue{}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	self, err := processIdentity(os.Getpid())
	if err != nil || self == "" {
		t.Fatalf("own identity: %q %v", self, err)
	}
	now := time.Now()
	plant := func(name, leader string, at time.Time) {
		dir := d.workerDir(name)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if leader != "" {
			if err := os.WriteFile(filepath.Join(dir, workerLeaderName), []byte(leader), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		os.Chtimes(dir, at, at)
	}
	for i := 0; i < maxRetainedWorkerDirs+1; i++ {
		plant(fmt.Sprintf("held-%02d", i), fmt.Sprintf("%d %s\n", os.Getpid(), self), now.Add(-2*time.Hour-time.Duration(i)*time.Minute))
	}
	plant("older", "", now.Add(-10*time.Hour))
	d.retireWorkerDirs()
	d.Now = func() time.Time { return now.Add(2 * time.Minute) }
	d.retireWorkerDirs()
	if _, err := os.Stat(d.workerDir("older")); err != nil || marked(d, "older") {
		t.Fatalf("protected directories took the retention slots: %v, marked=%v", err, marked(d, "older"))
	}
	for i := 0; i < maxRetainedWorkerDirs+1; i++ {
		if _, err := os.Stat(d.workerDir(fmt.Sprintf("held-%02d", i))); err != nil {
			t.Fatalf("held-%02d was retired: %v", i, err)
		}
	}
}

// TestCALV0144_IncompleteSessionReadIsRetaken reads a table in which a
// process is gone before its session is read, as when it forked a child the
// table does not show. The table is read once more and the two reads are
// joined: the groups and the sessions of either read alone count. The two
// live processes lead sessions of their own, so each read sees a session the
// other does not. A gap left only by the table reader itself needs no second
// read.
func TestCALV0144_IncompleteSessionReadIsRetaken(t *testing.T) {
	gone := exec.Command("/bin/sh", "-c", "exit 0")
	if err := gone.Run(); err != nil {
		t.Fatal(err)
	}
	leader := func() int {
		c := exec.Command("/bin/sleep", "60")
		c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := c.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { c.Process.Kill(); c.Wait() })
		return c.Process.Pid
	}
	pid, first, second, self := gone.Process.Pid, leader(), leader(), os.Getpid()
	prev := superviseProcs
	defer func() { superviseProcs = prev }()
	reads := 0
	table := func(ppid int, comm string) {
		reads = 0
		superviseProcs = func() (map[int]proc, error) {
			reads++
			if reads == 1 {
				return map[int]proc{
					pid:   {pid: pid, ppid: ppid, pgid: 1_000_001, comm: comm},
					first: {pid: first, ppid: self, pgid: 1_000_002, comm: "sleep"},
				}, nil
			}
			return map[int]proc{second: {pid: second, ppid: self, pgid: 1_000_003, comm: "sleep"}}, nil
		}
	}
	table(1, "sh")
	groups, sessions, ok := liveTrees()
	if !ok || reads != 2 {
		t.Fatalf("after a gone process: ok=%v reads=%d", ok, reads)
	}
	for _, g := range []int{1_000_001, 1_000_002, 1_000_003} {
		if !groups[g] {
			t.Errorf("group %d of one read was dropped", g)
		}
	}
	if !sessions[first] || !sessions[second] {
		t.Errorf("first-read session kept=%v, second-read session kept=%v", sessions[first], sessions[second])
	}
	table(self, "ps")
	if _, _, ok := liveTrees(); !ok || reads != 1 {
		t.Fatalf("after the gone table reader: ok=%v reads=%d", ok, reads)
	}
}

// TestCALV0144_RemovalNeedsAConfirmingPass plants 33 quiet finished
// directories. The first pass marks the oldest and removes nothing, a pass
// under a minute later still keeps it, and a pass after new log activity
// clears the mark. Only a pass at least a minute after a fresh mark, finding
// the same age, removes it.
func TestCALV0144_RemovalNeedsAConfirmingPass(t *testing.T) {
	d, err := Open("prog", testConfig(t, "exit 0"), &fakeQueue{}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	now := time.Now()
	for i := 0; i < maxRetainedWorkerDirs+1; i++ {
		plantQuiet(t, d, fmt.Sprintf("dir-%02d", i), "", now.Add(-2*time.Hour-time.Duration(i)*time.Minute))
	}
	oldest := fmt.Sprintf("dir-%02d", maxRetainedWorkerDirs)
	pass := func(at time.Time) {
		d.Now = func() time.Time { return at }
		d.retireWorkerDirs()
	}
	exists := func() bool { _, err := os.Stat(d.workerDir(oldest)); return err == nil }
	pass(now)
	if !exists() || !marked(d, oldest) {
		t.Fatalf("first pass: exists=%v marked=%v", exists(), marked(d, oldest))
	}
	pass(now.Add(30 * time.Second))
	if !exists() || !marked(d, oldest) {
		t.Fatalf("pass under a minute later: exists=%v marked=%v", exists(), marked(d, oldest))
	}
	// New output after the mark: still the oldest, and outside the quiet window.
	touched := now.Add(-2*time.Hour - time.Duration(maxRetainedWorkerDirs)*time.Minute + 30*time.Second)
	if err := os.Chtimes(filepath.Join(d.workerDir(oldest), "stdout.log"), touched, touched); err != nil {
		t.Fatal(err)
	}
	pass(now.Add(2 * time.Minute))
	if !exists() || marked(d, oldest) {
		t.Fatalf("pass after new activity: exists=%v marked=%v", exists(), marked(d, oldest))
	}
	pass(now.Add(3 * time.Minute))
	if !exists() || !marked(d, oldest) {
		t.Fatalf("re-marking pass: exists=%v marked=%v", exists(), marked(d, oldest))
	}
	pass(now.Add(4 * time.Minute))
	if exists() || marked(d, oldest) {
		t.Fatalf("confirming pass: exists=%v marked=%v", exists(), marked(d, oldest))
	}
	for i := 0; i < maxRetainedWorkerDirs; i++ {
		if _, err := os.Stat(d.workerDir(fmt.Sprintf("dir-%02d", i))); err != nil {
			t.Fatalf("dir-%02d was removed: %v", i, err)
		}
	}
}

// TestCALV0144_LiveMemberOnConfirmingPassKeepsDir marks a directory whose
// recorded leader is gone with no member left, then lets the confirming
// pass's process table show a live member of the leader's process group. The
// directory is kept and its mark is cleared.
func TestCALV0144_LiveMemberOnConfirmingPassKeepsDir(t *testing.T) {
	d, err := Open("prog", testConfig(t, "exit 0"), &fakeQueue{}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	gone := exec.Command("/bin/sh", "-c", "exit 0")
	if err := gone.Run(); err != nil {
		t.Fatal(err)
	}
	leader := gone.Process.Pid
	now := time.Now()
	for i := 0; i < maxRetainedWorkerDirs; i++ {
		plantQuiet(t, d, fmt.Sprintf("dir-%02d", i), "", now.Add(-2*time.Hour-time.Duration(i)*time.Minute))
	}
	plantQuiet(t, d, "tree", fmt.Sprintf("%d gone-identity\n", leader), now.Add(-12*time.Hour))
	self := os.Getpid()
	prev := superviseProcs
	defer func() { superviseProcs = prev }()
	pgid := self
	superviseProcs = func() (map[int]proc, error) {
		return map[int]proc{self: {pid: self, ppid: 1, pgid: pgid, comm: "go"}}, nil
	}
	d.Now = func() time.Time { return now }
	d.retireWorkerDirs()
	if !marked(d, "tree") {
		t.Fatal("a removable directory was not marked")
	}
	pgid = leader // a member of the leader's group is now seen
	d.Now = func() time.Time { return now.Add(2 * time.Minute) }
	d.retireWorkerDirs()
	if _, err := os.Stat(d.workerDir("tree")); err != nil {
		t.Fatalf("a directory with a live member was removed: %v", err)
	}
	if marked(d, "tree") {
		t.Fatal("a live member left the mark in place")
	}
}

// TestCALV0144_ActivityDuringTheConfirmingPassKeepsDir marks a directory
// with no leader file, then lets its logs change while the confirming pass
// reads the process table for another directory's leader, after the pass's
// scan. The fresh check before removal sees the change: the directory is
// kept and its mark is cleared.
func TestCALV0144_ActivityDuringTheConfirmingPassKeepsDir(t *testing.T) {
	d, err := Open("prog", testConfig(t, "exit 0"), &fakeQueue{}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	gone := exec.Command("/bin/sh", "-c", "exit 0")
	if err := gone.Run(); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for i := 0; i < maxRetainedWorkerDirs; i++ {
		leader := ""
		if i == 0 {
			leader = fmt.Sprintf("%d gone-identity\n", gone.Process.Pid)
		}
		plantQuiet(t, d, fmt.Sprintf("dir-%02d", i), leader, now.Add(-2*time.Hour-time.Duration(i)*time.Minute))
	}
	plantQuiet(t, d, "target", "", now.Add(-10*time.Hour))
	d.Now = func() time.Time { return now }
	d.retireWorkerDirs()
	if !marked(d, "target") {
		t.Fatal("a removable directory was not marked")
	}
	prev := superviseProcs
	defer func() { superviseProcs = prev }()
	superviseProcs = func() (map[int]proc, error) {
		at := time.Now()
		if err := os.Chtimes(filepath.Join(d.workerDir("target"), "stdout.log"), at, at); err != nil {
			t.Error(err)
		}
		return prev()
	}
	d.Now = func() time.Time { return now.Add(2 * time.Minute) }
	d.retireWorkerDirs()
	if _, err := os.Stat(d.workerDir("target")); err != nil {
		t.Fatalf("a directory that resumed logging was removed: %v", err)
	}
	if marked(d, "target") {
		t.Fatal("the resumed directory kept its mark")
	}
}

// TestCALV0144_StaleMarkStartsOver lets a mark outlive its maximum age. The
// pass that finds it stale marks the directory again instead of removing it;
// a pass a minute after the fresh mark removes it.
func TestCALV0144_StaleMarkStartsOver(t *testing.T) {
	d, err := Open("prog", testConfig(t, "exit 0"), &fakeQueue{}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	now := time.Now()
	for i := 0; i < maxRetainedWorkerDirs; i++ {
		plantQuiet(t, d, fmt.Sprintf("dir-%02d", i), "", now.Add(-2*time.Hour-time.Duration(i)*time.Minute))
	}
	plantQuiet(t, d, "target", "", now.Add(-10*time.Hour))
	pass := func(at time.Time) {
		d.Now = func() time.Time { return at }
		d.retireWorkerDirs()
	}
	pass(now)
	stale := now.Add(workerRetireMarkMaxAge + time.Minute)
	pass(stale)
	if _, err := os.Stat(d.workerDir("target")); err != nil {
		t.Fatalf("a stale mark was confirmed: %v", err)
	}
	markedAt, _, err := readRetireMark(filepath.Join(d.dir, "workers", workerRetireMarks, "target"))
	if err != nil || !markedAt.Equal(stale) {
		t.Fatalf("stale mark not renewed: %v %v", markedAt, err)
	}
	pass(stale.Add(workerRetireConfirm))
	if _, err := os.Stat(d.workerDir("target")); !os.IsNotExist(err) {
		t.Fatalf("a confirmed directory was kept: %v", err)
	}
}

// TestCALV0144_MarksAreBounded fills the mark directory with maxRetireMarks
// marks of directories that do not exist. The pass prunes them from their
// own directory and, having read a full set, writes no new mark; the next
// pass marks the removable directory.
func TestCALV0144_MarksAreBounded(t *testing.T) {
	d, err := Open("prog", testConfig(t, "exit 0"), &fakeQueue{}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	now := time.Now()
	for i := 0; i < maxRetainedWorkerDirs; i++ {
		plantQuiet(t, d, fmt.Sprintf("dir-%02d", i), "", now.Add(-2*time.Hour-time.Duration(i)*time.Minute))
	}
	plantQuiet(t, d, "target", "", now.Add(-10*time.Hour))
	marks := filepath.Join(d.dir, "workers", workerRetireMarks)
	if err := os.MkdirAll(marks, 0o700); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxRetireMarks; i++ {
		if err := os.WriteFile(filepath.Join(marks, fmt.Sprintf("absent-%04d", i)), []byte("0 0\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	d.Now = func() time.Time { return now }
	d.retireWorkerDirs()
	left, _ := os.ReadDir(marks)
	if len(left) != 0 {
		t.Fatalf("%d marks left after a full set (target marked=%v)", len(left), marked(d, "target"))
	}
	d.retireWorkerDirs()
	if !marked(d, "target") {
		t.Fatal("the pass after the prune did not mark")
	}
}

func plantQuiet(t *testing.T, d *Dispatcher, name, leader string, at time.Time) {
	t.Helper()
	dir := d.workerDir(name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "stdout.log")
	if err := os.WriteFile(p, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if leader != "" {
		if err := os.WriteFile(filepath.Join(dir, workerLeaderName), []byte(leader), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	os.Chtimes(p, at, at)
	os.Chtimes(dir, at, at)
}

func marked(d *Dispatcher, name string) bool {
	_, err := os.Stat(filepath.Join(d.dir, "workers", workerRetireMarks, name))
	return err == nil
}
