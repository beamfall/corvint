//go:build darwin || linux

package gitrun

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/groupreap"
)

// PGO-V0-012: the stable runner's owned children retire through the Owner.
// The single group signal precedes the reap that ends identity ownership, no
// signal follows it, and an unobserved exit or failed quiet proof is a HOLD.

// signalLedger wraps the Owner's real kill and reap primitives and records
// their order. KillGroup mirrors the platform default exactly.
type signalLedger struct {
	mu     sync.Mutex
	events []string
	leader int
}

func (r *signalLedger) note(event string, leader int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	r.leader = leader
}

func (r *signalLedger) recorded() ([]string, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.events), r.leader
}

func (r *signalLedger) primitives(groupreap.Primitives) groupreap.Primitives {
	return groupreap.Primitives{
		KillGroup: func(leader int) error {
			r.note("kill-group", leader)
			err := syscall.Kill(-leader, syscall.SIGKILL)
			if runtime.GOOS != "linux" && (err == syscall.EPERM || err == syscall.ESRCH) {
				return nil
			}
			return err
		},
		Reap: func(command *exec.Cmd) error {
			r.note("reap", command.Process.Pid)
			return command.Wait()
		},
	}
}

func (r *signalLedger) with(extra func(*groupreap.Primitives)) func(int, bool) groupreap.Primitives {
	return func(int, bool) groupreap.Primitives {
		p := r.primitives(groupreap.Primitives{})
		if extra != nil {
			extra(&p)
		}
		return p
	}
}

// requireKillsThenReap fails unless the ledger saw one or more group
// signals, then the reap, then nothing. A signal after the first is the
// PGO-V0-017 repeat on a pre-reap live observation; none may follow the reap.
func (r *signalLedger) requireKillsThenReap(t *testing.T) {
	t.Helper()
	got, _ := r.recorded()
	n := len(got)
	if n < 2 || got[n-1] != "reap" || slices.ContainsFunc(got[:n-1], func(e string) bool { return e != "kill-group" }) {
		t.Fatalf("owner primitives = %v, want kill-group (repeated only before the reap), reap", got)
	}
}

// requireEvents fails unless the ledger saw exactly want.
func (r *signalLedger) requireEvents(t *testing.T, want ...string) {
	t.Helper()
	if got, _ := r.recorded(); !slices.Equal(got, want) {
		t.Fatalf("owner primitives = %v, want %v", got, want)
	}
}

// lingering starts a descendant that outlives a normal exit unless retired
// and records its PID in dir/descendant.
func lingering(dir, tail string) string {
	return "/bin/sleep 30 >/dev/null 2>&1 & echo $! > " + filepath.Join(dir, "descendant") + "; " + tail
}

// descendantPID waits until the script recorded its descendant.
func descendantPID(t *testing.T, dir string) int {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		data, err := os.ReadFile(filepath.Join(dir, "descendant"))
		if pid, convErr := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && convErr == nil && pid > 0 {
			return pid
		}
		if time.Now().After(deadline) {
			t.Fatal("descendant PID was not recorded")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// requireDescendantGone observes with signal 0 only. A reused PID can only
// make the descendant look present, so the check fails closed; the test never
// signals the bare PID.
func requireDescendantGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
		if time.Now().After(deadline) {
			t.Fatalf("descendant %d was not observed absent after retirement", pid)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestPGOV0012StableDescendantRetiresWithoutPostReapSignal covers a normal
// exit and both interruptions (caller cancellation and per-operation
// timeout) on the stable owned runner with a real lingering descendant.
func TestPGOV0012StableDescendantRetiresWithoutPostReapSignal(t *testing.T) {
	cases := []struct {
		name      string
		tail      string
		interrupt func(l *lifecycle, cancel context.CancelFunc)
		want      string
	}{
		{name: "normal-exit", tail: "echo ok"},
		{name: "cancel", tail: "exec /bin/sleep 30", interrupt: func(_ *lifecycle, cancel context.CancelFunc) { cancel() }, want: cemcode.GitCancelled},
		{name: "timeout", tail: "exec /bin/sleep 30", interrupt: func(l *lifecycle, _ context.CancelFunc) { l.advance(DefaultPerOpTimeout) }, want: cemcode.GitTimeout},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ledger := &signalLedger{}
			l := newLifecycle(4, ledger.with(nil))
			options := lifecycleShell()
			options.Dir = t.TempDir()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			interrupted := make(chan int, 1)
			if c.interrupt != nil {
				go func() {
					pid := descendantPID(t, options.Dir)
					c.interrupt(l, cancel)
					interrupted <- pid
				}()
			}
			out, err := Run(ctx, l.budget, options, "-c", lingering(options.Dir, c.tail))
			var pid int
			if c.interrupt == nil {
				if err != nil || string(out) != "ok\n" {
					t.Fatalf("out=%q err=%v", out, err)
				}
				pid = descendantPID(t, options.Dir)
			} else {
				requireCode(t, err, c.want)
				pid = <-interrupted
			}
			ledger.requireKillsThenReap(t)
			if l.budget.Held() || l.count("released") != 1 || l.count("hold") != 0 {
				t.Fatalf("events = %v held=%v", l.names(), l.budget.Held())
			}
			requireDescendantGone(t, pid)
		})
	}
}

// TestPGOV0012StableFailedQuietProofHoldsWithoutReapOrSecondSignal: a failed
// pre-reap quiet proof after the single group signal is a HOLD. The leader
// stays unreaped and owned, nothing signals again, and no later operation
// starts.
func TestPGOV0012StableFailedQuietProofHoldsWithoutReapOrSecondSignal(t *testing.T) {
	ledger := &signalLedger{}
	l := newLifecycle(4, ledger.with(func(p *groupreap.Primitives) {
		p.QuietProof = func(int) (groupreap.Probe, error) {
			return groupreap.ProbeLive, errors.New("injected quiet proof failure")
		}
	}))
	options := lifecycleShell()
	options.Dir = t.TempDir()
	_, err := Run(context.Background(), l.budget, options, "-c", lingering(options.Dir, "echo ok"))
	requireCode(t, err, ProcessContainment)
	_, leader := ledger.recorded()
	t.Cleanup(func() { reapHeldLeader(t, leader, false) })
	ledger.requireEvents(t, "kill-group")
	if !l.budget.Held() || l.count("hold") != 1 || l.count("released") != 0 {
		t.Fatalf("events = %v held=%v", l.names(), l.budget.Held())
	}
	_, err = Run(context.Background(), l.budget, options, "-c", "echo never")
	requireCode(t, err, ProcessContainment)
	ledger.requireEvents(t, "kill-group")
	requireDescendantGone(t, descendantPID(t, options.Dir))
}

// TestPGOV0012StableUnobservedExitHoldsWithoutAnySignal: without the
// unreaped-leader exit observation the group ID is not provably owned, so no
// group signal and no reap follow; the result is a HOLD, never a release.
func TestPGOV0012StableUnobservedExitHoldsWithoutAnySignal(t *testing.T) {
	ledger := &signalLedger{}
	l := newLifecycle(4, ledger.with(func(p *groupreap.Primitives) {
		p.WaitExit = func(leader int) error {
			ledger.note("exit-unobserved", leader)
			return errors.New("injected exit observation failure")
		}
	}))
	_, err := Run(context.Background(), l.budget, lifecycleShell(), "-c", "exec /bin/sleep 30")
	requireCode(t, err, ProcessContainment)
	_, leader := ledger.recorded()
	t.Cleanup(func() { reapHeldLeader(t, leader, true) })
	ledger.requireEvents(t, "exit-unobserved")
	if !l.budget.Held() || l.count("hold") != 1 || l.count("released") != 0 {
		t.Fatalf("events = %v held=%v", l.names(), l.budget.Held())
	}
}

// reapHeldLeader is test-owned cleanup of a HOLD. The leader is this
// process's own unreaped child, so its PID is pinned until the Wait4 below;
// it is first removed from the exit-kill registry so nothing can name its
// group after the reap.
func reapHeldLeader(t *testing.T, leader int, running bool) {
	t.Helper()
	if leader <= 0 {
		return
	}
	groupreap.ReleaseLive(leader)
	if running {
		_ = syscall.Kill(leader, syscall.SIGKILL)
	}
	var status syscall.WaitStatus
	if _, err := syscall.Wait4(leader, &status, 0, nil); err != nil {
		t.Errorf("reap held leader %d: %v", leader, err)
	}
}

// repeatHost is a fake host for PGO-V0-017. The first pre-reap quiet proof
// sees a live member, as one forked alongside the group SIGKILL can be on
// Darwin. A later proof is quiet only once the group was signalled again;
// without the repeat it fails, which HOLDs. The leader exits on its own and
// is reaped for real; the post-reap probe reports absence.
func (r *signalLedger) repeatHost() groupreap.Primitives {
	var kills, proofs atomic.Int32
	return groupreap.Primitives{
		KillGroup: func(leader int) error {
			r.note("kill-group", leader)
			kills.Add(1)
			return nil
		},
		QuietProof: func(int) (groupreap.Probe, error) {
			switch {
			case proofs.Add(1) == 1:
				return groupreap.ProbeLive, nil
			case kills.Load() < 2:
				return groupreap.ProbeLive, errors.New("live member after a single group signal")
			}
			return groupreap.ProbeQuiet, nil
		},
		ProbeGroup: func(int) (groupreap.Probe, error) { return groupreap.ProbeAbsent, nil },
		Reap: func(command *exec.Cmd) error {
			r.note("reap", command.Process.Pid)
			return command.Wait()
		},
	}
}

// TestPGOV0017StableRepeatsGroupKillWhileMemberLive: the stable runner opts
// its owned children into RepeatKillWhileLive, so a member still live after
// the first group SIGKILL is signalled again before the reap. With the opt-in
// removed the fake host's second proof fails and the runner HOLDs.
func TestPGOV0017StableRepeatsGroupKillWhileMemberLive(t *testing.T) {
	ledger := &signalLedger{}
	l := newLifecycle(4, func(int, bool) groupreap.Primitives { return ledger.repeatHost() })
	out, err := Run(context.Background(), l.budget, lifecycleShell(), "-c", "echo ok")
	if err != nil || string(out) != "ok\n" {
		t.Fatalf("out=%q err=%v events=%v", out, err, l.names())
	}
	ledger.requireEvents(t, "kill-group", "kill-group", "reap")
	if l.budget.Held() || l.count("released") != 1 {
		t.Fatalf("events = %v held=%v", l.names(), l.budget.Held())
	}
}
