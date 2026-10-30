//go:build darwin || linux

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
)

// fakeProc is one incarnation in a fakeHost.
type fakeProc struct {
	p      groupreap.Process
	reaped bool
}

// fakeHost is a process table with pid reuse under the test's control. A
// SIGKILL ends and reaps the incarnation it reaches; signals are recorded as
// "pid/start/signal".
type fakeHost struct {
	procs      map[int]*fakeProc
	signals    []string
	tableErr   error
	openErr    error
	signalErr  map[incarnation]error
	beforeOpen func(pid int)
	afterTable func(n int)
	tables     int
}

func newFakeHost() *fakeHost {
	return &fakeHost{procs: map[int]*fakeProc{}, signalErr: map[incarnation]error{}}
}

func (h *fakeHost) spawn(pid, ppid, pgid int, start int64) {
	h.procs[pid] = &fakeProc{p: groupreap.Process{PID: pid, PPID: ppid, PGID: pgid, Start: start}}
}

func (h *fakeHost) exit(pid int) {
	if p := h.procs[pid]; p != nil {
		p.reaped = true
		delete(h.procs, pid)
	}
}

func (h *fakeHost) host() reapHost {
	return reapHost{
		table: func() (map[int]groupreap.Process, error) {
			h.tables++
			if h.tableErr != nil {
				return nil, h.tableErr
			}
			out := map[int]groupreap.Process{}
			for pid, p := range h.procs {
				out[pid] = p.p
			}
			if h.afterTable != nil {
				h.afterTable(h.tables)
			}
			return out, nil
		},
		open: func(pid int) (pin, bool, error) {
			if h.beforeOpen != nil {
				h.beforeOpen(pid)
			}
			if h.openErr != nil {
				return nil, false, h.openErr
			}
			p := h.procs[pid]
			if p == nil {
				return nil, true, nil
			}
			return &fakePin{h: h, p: p}, false, nil
		},
	}
}

func (h *fakeHost) signalled(pid int, start int64) []string {
	var out []string
	for _, s := range h.signals {
		if strings.HasPrefix(s, fmt.Sprintf("%d/%d/", pid, start)) {
			out = append(out, s)
		}
	}
	return out
}

type fakePin struct {
	h *fakeHost
	p *fakeProc
}

func (f *fakePin) pinned() (bool, error) { return !f.p.reaped, nil }

func (f *fakePin) signal(sig syscall.Signal) (bool, error) {
	if f.p.reaped {
		return true, nil
	}
	if err := f.h.signalErr[incarnationOf(f.p.p)]; err != nil {
		return false, err
	}
	f.h.signals = append(f.h.signals, fmt.Sprintf("%d/%d/%v", f.p.p.PID, f.p.p.Start, sig))
	switch sig {
	case syscall.SIGKILL:
		f.h.exit(f.p.p.PID)
	case syscall.SIGSTOP:
		f.p.p.State = groupreap.StateStopped
	}
	return false, nil
}

func (f *fakePin) close() {}

// The fake test binary is pid 1000.
const fakeParent = 1000

func fakeTracker(h *fakeHost) *reapTracker {
	return newReapTracker(h.host(), fakeParent, 0, nil, 999)
}

// ATR-V0-310 (review finding 5): a worker pid that is reused by a later
// worker of the same binary is attributed again under its new incarnation
// and retired; the reaped incarnation is forgotten. A tracker that kept the
// first incarnation of a pid forever skipped the later worker at every
// sample and at the end.
func TestATRV0310_ReusedWorkerPIDIsTrackedByIncarnation(t *testing.T) {
	h := newFakeHost()
	r := fakeTracker(h)
	h.spawn(100, fakeParent, 100, 1)
	if err := r.sample(); err != nil || r.owned[100] == nil {
		t.Fatalf("first worker not attributed: %v %v", err, r.owned)
	}
	h.exit(100)
	h.spawn(100, fakeParent, 100, 2) // a later worker of the binary under the same pid
	h.spawn(101, 100, 100, 3)        // and its member
	if err := r.sample(); err != nil {
		t.Fatal(err)
	}
	if o := r.owned[100]; o == nil || o.p.Start != 2 || r.owned[101] == nil {
		t.Fatalf("the reused pid's new worker is not tracked: %+v", r.owned)
	}
	rep := r.retire()
	if rep.err != nil || !slices.Equal(rep.killed, []int{100, 101}) || len(rep.unproven)+len(rep.stuck) != 0 {
		t.Fatalf("report %s, want killed [100 101]", rep)
	}
	if len(h.signalled(100, 1)) != 0 || len(h.signalled(100, 2)) != 2 || len(h.signalled(101, 3)) != 1 {
		t.Fatalf("signals %v", h.signals)
	}
}

// ATR-V0-310 (review finding 2): a process sampled as a member of an owned
// group that exits, and whose pid is reused by an unrelated process before
// the reaper opens its handle, is not attributed: the row that decides
// membership is read after the handle is opened and belongs to the handle's
// process. The replacement is never signalled.
func TestATRV0310_MembershipIsReadFromThePinnedProcess(t *testing.T) {
	h := newFakeHost()
	r := fakeTracker(h)
	h.spawn(100, fakeParent, 100, 1)
	h.spawn(200, 100, 100, 5)
	h.beforeOpen = func(pid int) {
		if pid == 200 && h.procs[200].p.Start == 5 {
			h.exit(200)
			h.spawn(200, 1, 777, 6) // unrelated, in its own group
		}
	}
	if err := r.sample(); err != nil {
		t.Fatal(err)
	}
	if r.owned[100] == nil || r.owned[200] != nil {
		t.Fatalf("owned %+v: the replacement of a sampled member was attributed", r.owned)
	}
	h.beforeOpen = nil
	rep := r.retire()
	if len(h.signalled(200, 6)) != 0 {
		t.Fatalf("the replacement process was signalled: %v", h.signals)
	}
	if rep.err != nil || !slices.Equal(rep.killed, []int{100}) {
		t.Fatalf("report %s, want killed [100]", rep)
	}
	// A member whose handle is released between the second read and the
	// check is not attributed either: its row may be another process's.
	h2 := newFakeHost()
	r2 := fakeTracker(h2)
	h2.spawn(100, fakeParent, 100, 1)
	h2.spawn(200, 100, 100, 5)
	if err := r2.sample(); err != nil || r2.owned[200] == nil {
		t.Fatalf("setup: %v %+v", err, r2.owned)
	}
	h2.spawn(201, 100, 100, 7)
	// The second sample's reads are the third and fourth: 201 is released
	// just after the fourth, the read its membership is taken from.
	h2.afterTable = func(n int) {
		if n == 4 {
			h2.exit(201)
		}
	}
	if err := r2.sample(); err != nil {
		t.Fatal(err)
	}
	if r2.owned[201] != nil {
		t.Fatalf("a member whose handle was released after the read was attributed: %+v", r2.owned)
	}
}

// ATR-V0-310 (review finding 1): no signal goes to a group or pid by number.
// An owned leader is stopped first and killed last through its handle, its
// members through theirs; once the leader and its members are gone, a group
// under the same number, which may belong to anyone, is reported and never
// signalled.
func TestATRV0310_NoSignalWithoutAPinnedOwner(t *testing.T) {
	h := newFakeHost()
	r := fakeTracker(h)
	h.spawn(100, fakeParent, 100, 1)
	h.spawn(200, 100, 100, 2)
	if err := r.sample(); err != nil {
		t.Fatal(err)
	}
	rep := r.retire()
	want := []string{fmt.Sprintf("100/1/%v", syscall.SIGSTOP), fmt.Sprintf("200/2/%v", syscall.SIGKILL), fmt.Sprintf("100/1/%v", syscall.SIGKILL)}
	if rep.err != nil || !slices.Equal(h.signals, want) {
		t.Fatalf("signals %v, want %v (report %s)", h.signals, want, rep)
	}
	// The group number is released and reused by an unrelated group.
	h3 := newFakeHost()
	r3 := fakeTracker(h3)
	h3.spawn(100, fakeParent, 100, 1)
	h3.spawn(200, 100, 100, 2)
	if err := r3.sample(); err != nil {
		t.Fatal(err)
	}
	h3.exit(100)
	h3.exit(200)
	h3.spawn(100, 1, 100, 9) // a stranger leading a group under the same number
	h3.spawn(301, 100, 100, 10)
	rep = r3.retire()
	if len(h3.signals) != 0 {
		t.Fatalf("signalled %v: a reused group number was taken for the owned group", h3.signals)
	}
	if !slices.Equal(rep.unproven, []int{100, 301}) || rep.clean() {
		t.Fatalf("report %s, want the processes under the reused number reported, not signalled", rep)
	}
}

// ATR-V0-310 (review finding 4): a failed process-table read, a handle that
// cannot be opened or a signal refused with EPERM leaves cleanup unproven and
// fails the check; none is taken for "gone".
func TestATRV0310_UnobservableCleanupFails(t *testing.T) {
	h := newFakeHost()
	h.tableErr = errors.New("injected table failure")
	if rep := retireOwnChildrenOf(h); rep.clean() || rep.err == nil {
		t.Fatalf("a failed table read gave a clean report: %s", rep)
	}
	h = newFakeHost()
	h.spawn(100, fakeParent, 100, 1)
	h.openErr = syscall.EACCES
	if rep := retireOwnChildrenOf(h); rep.clean() || !errors.Is(rep.err, syscall.EACCES) {
		t.Fatalf("a failed open gave %s", rep)
	}
	h = newFakeHost()
	h.spawn(100, fakeParent, 100, 1)
	h.spawn(200, 100, 100, 2)
	h.signalErr[incarnation{200, 2}] = syscall.EPERM
	rep := retireOwnChildrenOf(h)
	if rep.clean() || !errors.Is(rep.err, syscall.EPERM) || slices.Contains(rep.killed, 200) {
		t.Fatalf("an EPERM signal gave %s", rep)
	}
	// The dead-man reaper keeps a failed sample and fails at end of file.
	h = newFakeHost()
	h.openErr = errPinUnsupported
	var calls atomic.Int32
	host := h.host()
	table := host.table
	host.table = func() (map[int]groupreap.Process, error) {
		if calls.Add(1) == 1 {
			return nil, errors.New("injected sample failure")
		}
		return table()
	}
	in, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	var out bytes.Buffer
	done := make(chan int, 1)
	go func() { done <- runWorkerReaper(host, fmt.Sprintf("%d:0", fakeParent), in, &out, time.Millisecond) }()
	for i := 0; i < 2000 && calls.Load() < 2; i++ {
		time.Sleep(time.Millisecond)
	}
	w.Close()
	if code := <-done; code != 1 || !strings.Contains(out.String(), "injected sample failure") {
		t.Fatalf("reaper exit %d after a failed sample:\n%s", code, out.String())
	}
}

func retireOwnChildrenOf(h *fakeHost) reapReport {
	r := fakeTracker(h)
	defer r.close()
	return r.retire()
}

// ATR-V0-310 (review finding 6): a reaper that does not end after its pipe
// closes, here with a child in its group as a stalled inventory command
// would leave, is killed with its whole group while its leader is unreaped,
// and is reaped before the shutdown returns; the shutdown fails. A shutdown
// that killed only the reaper left the child running.
func TestATRV0310_StalledReaperIsRetiredWithItsGroup(t *testing.T) {
	record := filepath.Join(t.TempDir(), "child")
	w, owner, _, err := startWorkerReaper(reaperStallEnv + "=" + record)
	if err != nil {
		t.Fatal(err)
	}
	var child incarnation
	for i := 0; ; i++ {
		raw, err := os.ReadFile(record)
		if err == nil {
			if _, err := fmt.Sscan(string(raw), &child.pid, &child.start); err != nil || child.pid <= 0 {
				t.Fatalf("child record %q: %v", raw, err)
			}
			break
		}
		if i > 6000 {
			t.Fatal("the stalled reaper never started its child")
		}
		time.Sleep(10 * time.Millisecond)
	}
	err = stopWorkerReaper(w, owner, time.Second)
	if err == nil || !strings.Contains(err.Error(), "killed with its group") {
		t.Fatalf("stopping a stalled reaper: %v", err)
	}
	if owner.State() != groupreap.Released {
		t.Fatalf("the reaper was not reaped: %v", owner.State())
	}
	for i := 0; ; i++ {
		table, err := groupreap.ReadProcessTable()
		if err != nil {
			t.Fatal(err)
		}
		if p, ok := table[child.pid]; !ok || p.Start != child.start || p.State == groupreap.StateZombie {
			break
		}
		if i > 500 {
			t.Fatalf("the stalled reaper's child %d outlived its forced shutdown", child.pid)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
