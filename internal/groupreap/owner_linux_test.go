//go:build linux

package groupreap

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

type linuxChild struct {
	command     *exec.Cmd
	mu          sync.Mutex
	waitStarted bool
	done        chan struct{}
	waitErr     error
}

func (child *linuxChild) killUncollected() {
	child.mu.Lock()
	defer child.mu.Unlock()
	if !child.waitStarted {
		_ = child.command.Process.Kill()
	}
}

func (child *linuxChild) collect(kill bool) error {
	child.mu.Lock()
	if !child.waitStarted {
		if kill {
			_ = child.command.Process.Kill()
		}
		child.waitStarted = true
		go func() {
			child.waitErr = child.command.Wait()
			close(child.done)
		}()
	}
	child.mu.Unlock()
	select {
	case <-child.done:
		return child.waitErr
	case <-time.After(3 * time.Second):
		return errors.New("direct child collection timed out; ownership retained")
	}
}

func linuxCheckCollected(t *testing.T, child *linuxChild, err error) {
	t.Helper()
	select {
	case <-child.done:
	default:
		t.Errorf("direct child cleanup incomplete: %v", err)
	}
}

type linuxFixture struct {
	owner       *Owner
	leader      *linuxChild
	member      *linuxChild
	input       *os.File
	reapStarted atomic.Bool
}

func linuxHandshake(read *os.File, timeout time.Duration) error {
	if err := read.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	line, err := bufio.NewReader(read).ReadString('\n')
	if err != nil {
		return err
	}
	if line != "ready\n" {
		return fmt.Errorf("malformed handshake %q", line)
	}
	return nil
}

func linuxPipes(t *testing.T) (*os.File, *os.File, *os.File, *os.File) {
	t.Helper()
	input, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { input.Close(); writer.Close() })
	reader, output, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reader.Close(); output.Close() })
	return input, writer, reader, output
}

func linuxStartFixture(t *testing.T, script string, primitives Primitives, collectMember bool) (*linuxFixture, error) {
	t.Helper()
	input, writer, reader, output := linuxPipes(t)
	command := exec.Command("/bin/sh", "-c", script)
	command.Stdin, command.Stdout = input, output
	fixture := &linuxFixture{input: writer, leader: &linuxChild{command: command, done: make(chan struct{})}}
	primitives.Reap = func(*exec.Cmd) error {
		fixture.reapStarted.Store(true)
		err := fixture.leader.collect(false)
		if collectMember && fixture.member != nil {
			memberErr := fixture.member.collect(false)
			select {
			case <-fixture.member.done:
			default:
				return errors.Join(err, memberErr)
			}
		}
		return err
	}
	owner, err := StartWith(command, primitives)
	if err != nil {
		return nil, err
	}
	fixture.owner = owner
	t.Cleanup(func() {
		owner.Stop()
		fixture.leader.killUncollected()
		select {
		case <-owner.Exited():
			linuxCheckCollected(t, fixture.leader, fixture.leader.collect(false))
		case <-time.After(3 * time.Second):
			t.Error("leader exit observation cleanup timed out; ownership retained")
		}
	})
	input.Close()
	output.Close()
	err = linuxHandshake(reader, 250*time.Millisecond)
	reader.Close()
	return fixture, err
}

func (fixture *linuxFixture) addMember(t *testing.T) {
	t.Helper()
	input, _, reader, output := linuxPipes(t)
	command := exec.Command("/bin/sh", "-c", "echo ready; read release")
	command.Stdin, command.Stdout = input, output
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: fixture.leader.command.Process.Pid}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	child := &linuxChild{command: command, done: make(chan struct{})}
	t.Cleanup(func() { fixture.owner.Stop(); linuxCheckCollected(t, child, child.collect(true)) })
	fixture.member = child
	input.Close()
	output.Close()
	if err := linuxHandshake(reader, time.Second); err != nil {
		t.Fatal(err)
	}
	reader.Close()
}

func linuxFinish(t *testing.T, fixture *linuxFixture) Result {
	t.Helper()
	return linuxFinishWithin(fixture, 5*time.Second)
}

func linuxFinishWithin(fixture *linuxFixture, bound time.Duration) Result {
	limit := make(chan struct{})
	timer := time.AfterFunc(bound, func() { close(limit) })
	defer timer.Stop()
	return fixture.owner.Finish(limit)
}

func TestLinuxOwnerReleasesLeaderOnlyAfterNaturalExit(t *testing.T) {
	fixture, err := linuxStartFixture(t, "echo ready; read release; exit 0", Primitives{}, false)
	if err != nil {
		t.Fatal(err)
	}
	fixture.input.Close()
	result := linuxFinish(t, fixture)
	if result.State != Released || result.Err != nil || result.WaitErr != nil || !result.PostReapObserved || result.PostReap != ProbeAbsent {
		t.Fatalf("result = %+v events = %v", result, fixture.owner.Events())
	}
}

func TestLinuxOwnerHoldsWhenMemberRemainsVisibleAfterReap(t *testing.T) {
	p := Primitives{RetirementMode: ReapAfterSuccessfulSignal, KillGroup: func(int) error { return nil }}
	fixture, err := linuxStartFixture(t, "echo ready; read release; exit 0", p, false)
	if err != nil {
		t.Fatal(err)
	}
	fixture.addMember(t)
	fixture.input.Close()
	result := linuxFinish(t, fixture)
	if result.State != Hold || !result.PostReapObserved || result.PostReap == ProbeAbsent {
		t.Fatalf("surviving member did not force HOLD: %+v", result)
	}
}

func TestLinuxOwnerStopRetiresRunningGroup(t *testing.T) {
	fixture, err := linuxStartFixture(t, "echo ready; read release", Primitives{}, true)
	if err != nil {
		t.Fatal(err)
	}
	fixture.addMember(t)
	fixture.owner.Stop()
	result := linuxFinish(t, fixture)
	if result.State != Released || result.WaitErr == nil || !result.PostReapObserved || result.PostReap != ProbeAbsent {
		t.Fatalf("result = %+v events = %v", result, fixture.owner.Events())
	}
	select {
	case <-fixture.member.done:
	default:
		t.Fatal("RELEASED without completed member collection")
	}
}

func TestLinuxOwnerPreservesKillGroupESRCH(t *testing.T) {
	fixture, err := linuxStartFixture(t, "echo ready; read release; exit 0", Primitives{KillGroup: func(int) error { return syscall.ESRCH }}, false)
	if err != nil {
		t.Fatal(err)
	}
	fixture.addMember(t)
	fixture.input.Close()
	result := linuxFinish(t, fixture)
	if result.State != Hold || !errors.Is(result.Err, syscall.ESRCH) {
		t.Fatalf("result = %+v", result)
	}
	if strings.Contains(strings.Join(fixture.owner.Events(), ","), "reap") {
		t.Fatal("reaped after signal failure")
	}
}

func TestLinuxFixtureFailureCleanup(t *testing.T) {
	t.Run("bad-start", func(t *testing.T) {
		command := exec.Command("/nonexistent-cem-linux-fixture")
		owner, err := StartWith(command, Primitives{})
		if err == nil || owner != nil || command.Process != nil {
			t.Fatalf("bad start = %v, %v", owner, err)
		}
	})
	for _, script := range []string{"read release", "echo malformed; read release"} {
		t.Run(script, func(t *testing.T) {
			var fixture *linuxFixture
			t.Run("owned-handshake-failure", func(t *testing.T) {
				var err error
				fixture, err = linuxStartFixture(t, script, Primitives{}, false)
				if err == nil {
					t.Fatal("invalid handshake accepted")
				}
			})
			if fixture == nil {
				t.Fatal("fixture did not start")
			}
			select {
			case <-fixture.leader.done:
			default:
				t.Fatal("failed handshake left direct leader uncollected")
			}
		})
	}
}

// linuxSignalRecorder wraps the real group signal and records whether any
// real signal followed the start of the reap (PGO-V0-001).
type linuxSignalRecorder struct {
	fixture *linuxFixture
	kills   atomic.Int32
	late    atomic.Int32
}

func (r *linuxSignalRecorder) kill(leader int) error {
	r.kills.Add(1)
	if r.fixture != nil && r.fixture.reapStarted.Load() {
		r.late.Add(1)
	}
	return defaultPrimitives().KillGroup(leader)
}

func (r *linuxSignalRecorder) require(t *testing.T, owner *Owner) {
	t.Helper()
	events := owner.Events()
	if r.kills.Load() != 1 || r.late.Load() != 0 || slices.Index(events, "kill-group") < 0 ||
		(slices.Contains(events, "reap") && slices.Index(events, "kill-group") > slices.Index(events, "reap")) {
		t.Fatalf("kills = %d, post-reap kills = %d, events = %v", r.kills.Load(), r.late.Load(), events)
	}
}

// PGO-V0-006: an exited, unreaped leader and a SIGKILLed member that is still
// an uncollected zombie form a zombie-only group. Signal 0 still succeeds for
// it, yet the /proc proof reports it quiet, the owner reaps and RELEASES, and
// exactly one real group signal precedes the reap.
func TestLinuxOwnerRetiresZombieOnlyGroup(t *testing.T) {
	recorder := &linuxSignalRecorder{}
	type proof struct {
		signal0 error
		probe   Probe
		err     error
	}
	var proofs []proof
	p := Primitives{
		KillGroup: recorder.kill,
		QuietProof: func(leader int) (Probe, error) {
			signal0 := syscall.Kill(-leader, 0)
			probe, err := procGroupQuiet("/proc", leader, os.Getpid())
			proofs = append(proofs, proof{signal0, probe, err})
			return probe, err
		},
	}
	fixture, err := linuxStartFixture(t, "echo ready; read release; exit 0", p, true)
	if err != nil {
		t.Fatal(err)
	}
	recorder.fixture = fixture
	fixture.addMember(t)
	fixture.input.Close()
	result := linuxFinish(t, fixture)
	if result.State != Released || result.Err != nil || result.WaitErr != nil || !result.PostReapObserved || result.PostReap != ProbeAbsent {
		t.Fatalf("result = %+v events = %v proofs = %+v", result, fixture.owner.Events(), proofs)
	}
	last := proofs[len(proofs)-1]
	if last.probe != ProbeQuiet || last.err != nil || last.signal0 != nil {
		t.Fatalf("zombie-only proof = %+v, want quiet while signal 0 still succeeds", proofs)
	}
	events := fixture.owner.Events()
	reap := slices.Index(events, "reap")
	if reap < 1 || events[reap-1] != "probe-quiet" || events[len(events)-1] != "released" {
		t.Fatalf("events = %v", events)
	}
	recorder.require(t, fixture.owner)
	select {
	case <-fixture.member.done:
	default:
		t.Fatal("RELEASED without completed member collection")
	}
}

// The original V1-0668 defect: with signal 0 as the quiet proof, the same
// zombie-only group never looks quiet, so the owner HOLDs without reaping.
func TestLinuxSignalZeroCannotProveZombieOnlyQuiet(t *testing.T) {
	recorder := &linuxSignalRecorder{}
	p := Primitives{KillGroup: recorder.kill, QuietProof: defaultPrimitives().ProbeGroup}
	fixture, err := linuxStartFixture(t, "echo ready; read release; exit 0", p, true)
	if err != nil {
		t.Fatal(err)
	}
	recorder.fixture = fixture
	fixture.addMember(t)
	fixture.input.Close()
	result := linuxFinishWithin(fixture, 300*time.Millisecond)
	if result.State != Hold || result.PostReapObserved || slices.Contains(fixture.owner.Events(), "reap") {
		t.Fatalf("result = %+v events = %v", result, fixture.owner.Events())
	}
	recorder.require(t, fixture.owner)
}

// PGO-V0-006: a member that is still live keeps the proof from reporting
// quiet; the owner HOLDs at the bound before reaping.
func TestLinuxOwnerHoldsWhileLiveMemberRemains(t *testing.T) {
	fixture, err := linuxStartFixture(t, "echo ready; read release; exit 0", Primitives{KillGroup: func(int) error { return nil }}, false)
	if err != nil {
		t.Fatal(err)
	}
	fixture.addMember(t)
	fixture.input.Close()
	result := linuxFinishWithin(fixture, 300*time.Millisecond)
	events := fixture.owner.Events()
	if result.State != Hold || result.PostReapObserved || !slices.Contains(events, "probe-live") || slices.Contains(events, "reap") || fixture.reapStarted.Load() {
		t.Fatalf("live member did not force a pre-reap HOLD: %+v events = %v", result, events)
	}
}

// PGO-V0-006: without the /proc proof, or with a leader identity that does not
// match, the owner HOLDs after its one signal and never reaps.
func TestLinuxOwnerHoldsWhenQuietProofUnavailable(t *testing.T) {
	for name, quiet := range map[string]func(*testing.T) func(int) (Probe, error){
		"proc-missing": func(t *testing.T) func(int) (Probe, error) {
			root := filepath.Join(t.TempDir(), "missing")
			return func(leader int) (Probe, error) { return procGroupQuiet(root, leader, os.Getpid()) }
		},
		"wrong-parent": func(*testing.T) func(int) (Probe, error) {
			return func(leader int) (Probe, error) { return procGroupQuiet("/proc", leader, os.Getppid()) }
		},
	} {
		t.Run(name, func(t *testing.T) {
			recorder := &linuxSignalRecorder{}
			fixture, err := linuxStartFixture(t, "echo ready; read release; exit 0", Primitives{KillGroup: recorder.kill, QuietProof: quiet(t)}, true)
			if err != nil {
				t.Fatal(err)
			}
			recorder.fixture = fixture
			fixture.addMember(t)
			fixture.input.Close()
			result := linuxFinish(t, fixture)
			if result.State != Hold || !errors.Is(result.Err, errProcProof) || slices.Contains(fixture.owner.Events(), "reap") {
				t.Fatalf("result = %+v events = %v", result, fixture.owner.Events())
			}
			recorder.require(t, fixture.owner)
		})
	}
}

// The platform default on Linux is quiet-first with the /proc proof.
func TestLinuxDefaultRetirementUsesProcQuietProof(t *testing.T) {
	recorder := &linuxSignalRecorder{}
	fixture, err := linuxStartFixture(t, "echo ready; read release", Primitives{KillGroup: recorder.kill}, true)
	if err != nil {
		t.Fatal(err)
	}
	recorder.fixture = fixture
	fixture.addMember(t)
	fixture.owner.Stop()
	result := linuxFinish(t, fixture)
	events := fixture.owner.Events()
	reap := slices.Index(events, "reap")
	if result.State != Released || reap < 1 || events[reap-1] != "probe-quiet" {
		t.Fatalf("result = %+v events = %v", result, events)
	}
	recorder.require(t, fixture.owner)
}
