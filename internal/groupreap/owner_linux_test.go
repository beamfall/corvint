//go:build linux

package groupreap

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
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
	owner  *Owner
	leader *linuxChild
	member *linuxChild
	input  *os.File
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
	limit := make(chan struct{})
	timer := time.AfterFunc(5*time.Second, func() { close(limit) })
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
	fixture, err := linuxStartFixture(t, "echo ready; read release; exit 0", Primitives{KillGroup: func(int) error { return nil }}, false)
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
