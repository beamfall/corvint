package groupreap

import (
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

type fakeTable struct {
	rows     map[int]procRow
	env      map[int]bool
	reused   map[int]procRow // identity read after the snapshot sees a reused pid
	immortal map[int]bool
	signals  []int
}

func (f *fakeTable) primitives() retirePrimitives {
	return retirePrimitives{
		rows: func() ([]procRow, error) {
			out := []procRow{}
			for _, r := range f.rows {
				out = append(out, r)
			}
			return out, nil
		},
		identity: func(pid int) (procRow, bool, error) {
			if r, ok := f.reused[pid]; ok {
				return r, true, nil
			}
			r, ok := f.rows[pid]
			return r, ok, nil
		},
		environ: func(pid int, _ string) (bool, error) { return f.env[pid], nil },
		signal: func(pid int, sig syscall.Signal) error {
			f.signals = append(f.signals, pid)
			if sig == sigKill && !f.immortal[pid] {
				delete(f.rows, pid)
			}
			return nil
		},
		sleep: func(time.Duration) {},
	}
}

// TestRetirerProvesOwnershipBeforeSignalling binds TRE-V0-030's identity
// rules with an injected process table: unrelated and reused identities are
// never signalled; foreign-uid and surviving owned processes stay unretired.
func TestRetirerProvesOwnershipBeforeSignalling(t *testing.T) {
	now := time.Now().Unix()
	uid := os.Getuid()
	f := &fakeTable{rows: map[int]procRow{
		100: {pid: 100, ppid: 1, uid: uid, sec: now},        // leader
		101: {pid: 101, ppid: 100, uid: uid, sec: now},      // ancestry
		102: {pid: 102, ppid: 101, uid: uid + 1, sec: now},  // foreign uid descendant
		103: {pid: 103, ppid: 1, uid: uid, sec: now},        // detached, token
		104: {pid: 104, ppid: 1, uid: uid, sec: now},        // unrelated, no token
		105: {pid: 105, ppid: 1, uid: uid, sec: now},        // token, but pid reused before signal
		106: {pid: 106, ppid: 1, uid: uid, sec: now - 3600}, // token, started before the phase
		107: {pid: 107, ppid: 1, uid: uid, sec: now, zombie: true},
		108: {pid: 108, ppid: 1, uid: uid, sec: now}, // token, survives SIGKILL
	}, env: map[int]bool{103: true, 105: true, 106: true, 107: true, 108: true}, reused: map[int]procRow{105: {pid: 105, ppid: 1, uid: uid, sec: now, usec: 7}}, immortal: map[int]bool{108: true}}
	r, err := newRetirer(f.primitives())
	if err != nil {
		t.Fatal(err)
	}
	r.Leader(100)
	r.Retire()
	got := r.Result()
	signalled := map[int]bool{}
	for _, pid := range f.signals {
		signalled[pid] = true
	}
	for _, pid := range []int{102, 104, 105, 106, 107} {
		if signalled[pid] {
			t.Fatalf("signalled unproven identity %d: %v", pid, f.signals)
		}
	}
	for _, pid := range []int{100, 101, 103, 108} {
		if !signalled[pid] {
			t.Fatalf("owned identity %d not signalled: %v", pid, f.signals)
		}
	}
	owners := map[int]string{}
	for _, p := range got.Retired {
		owners[p.PID] = p.Owner
	}
	if owners[101] != OwnerAncestry || owners[103] != OwnerToken || len(got.Unretired) != 2 || got.Clean() {
		t.Fatalf("%+v", got)
	}
	for _, p := range got.Unretired {
		if p.PID != 102 && p.PID != 108 {
			t.Fatalf("unexpected unretired %+v", p)
		}
	}
}

func TestRetirerReportsUnconvergedForkStorm(t *testing.T) {
	now := time.Now().Unix()
	next := 200
	f := &fakeTable{rows: map[int]procRow{}, env: map[int]bool{}}
	p := f.primitives()
	rows := p.rows
	p.rows = func() ([]procRow, error) {
		// Each snapshot shows one new owned process.
		next++
		f.rows[next] = procRow{pid: next, ppid: 1, uid: os.Getuid(), sec: now}
		f.env[next] = true
		return rows()
	}
	r, err := newRetirer(p)
	if err != nil {
		t.Fatal(err)
	}
	r.Retire()
	if got := r.Result(); got.Clean() || len(got.Problems) != 1 {
		t.Fatalf("%+v", got)
	}
}

// TestRetirerKeepsReadFailuresAsUncertainty binds TRE-V0-030's failure path:
// unreadable identities or owner tokens never count as retired, and a failed
// later snapshot still kills processes the first snapshot stopped.
func TestRetirerKeepsReadFailuresAsUncertainty(t *testing.T) {
	now := time.Now().Unix()
	uid := os.Getuid()
	f := &fakeTable{rows: map[int]procRow{
		103: {pid: 103, ppid: 1, uid: uid, sec: now}, // token; stopped, then killed
		104: {pid: 104, ppid: 1, uid: uid, sec: now}, // token unreadable
		105: {pid: 105, ppid: 1, uid: uid, sec: now}, // token; identity unreadable after kill
	}, env: map[int]bool{103: true, 105: true}}
	p := f.primitives()
	rows, identity, environ := p.rows, p.identity, p.environ
	snapshots := 0
	p.rows = func() ([]procRow, error) {
		if snapshots++; snapshots > 1 {
			return nil, errors.New("snapshot refused")
		}
		return rows()
	}
	killed := map[int]bool{}
	signal := p.signal
	p.signal = func(pid int, sig syscall.Signal) error {
		if sig == sigKill {
			killed[pid] = true
		}
		if pid == 105 && sig == sigKill {
			f.immortal[105] = true
		}
		return signal(pid, sig)
	}
	f.immortal = map[int]bool{}
	p.identity = func(pid int) (procRow, bool, error) {
		if pid == 105 && killed[105] {
			return procRow{}, false, errors.New("identity refused")
		}
		return identity(pid)
	}
	p.environ = func(pid int, entry string) (bool, error) {
		if pid == 104 {
			return false, errors.New("procargs refused")
		}
		return environ(pid, entry)
	}
	r, err := newRetirer(p)
	if err != nil {
		t.Fatal(err)
	}
	r.Retire()
	got := r.Result()
	if !killed[103] || killed[104] {
		t.Fatalf("stopped process not killed after snapshot failure, or unproven signalled: %v", f.signals)
	}
	if got.Clean() || len(got.Retired) != 1 || got.Retired[0].PID != 103 {
		t.Fatalf("%+v", got)
	}
	if len(got.Unretired) != 1 || got.Unretired[0].PID != 105 || got.Unretired[0].Detail != "identity unreadable" {
		t.Fatalf("unreadable identity certified as retired: %+v", got)
	}
	problems := strings.Join(got.Problems, "; ")
	if !strings.Contains(problems, "owner token unreadable for pid 104") || !strings.Contains(problems, "snapshot refused") {
		t.Fatalf("read failures not retained: %v", got.Problems)
	}
}
