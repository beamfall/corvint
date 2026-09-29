//go:build unix

package store

import (
	"context"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func poolRunnerIdentity(pid int) (string, error) {
	c := exec.Command("/bin/ps", "-p", strconv.Itoa(pid), "-o", "lstart=")
	raw, e := c.Output()
	if exit, ok := e.(*exec.ExitError); ok && exit.ExitCode() == 1 {
		return "", nil
	}
	return strings.TrimSpace(string(raw)), e
}
func poolGroupLive(pid int) (bool, error) {
	raw, e := exec.Command("/bin/ps", "-axo", "pgid=,stat=").Output()
	if e != nil {
		return true, e
	}
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && f[0] == strconv.Itoa(pid) && !strings.HasPrefix(f[1], "Z") {
			return true, nil
		}
	}
	return false, nil
}

// Trusted commands are confined to an owned process group. Detached processes
// are outside this qualification; uncertain exit/pipe cleanup never admits.
func executePool(ctx context.Context, def *intent.PoolCommand, root string, env []string) (class string, clean bool, digest wire.Digest) {
	out := &cappedOutput{}
	cmd := exec.Command(def.Argv[0], def.Argv[1:]...)
	cmd.Dir = root
	cmd.Env = env
	pipeRead, pipeWrite, e := os.Pipe()
	if e != nil {
		return "SPAWN_FAILED", true, wire.Sum(nil)
	}
	defer pipeRead.Close()
	cmd.Stdout = pipeWrite
	cmd.Stderr = pipeWrite
	cmd.WaitDelay = time.Second
	containGate(cmd)
	out.kill = func() { killGate(cmd) }
	if e := cmd.Start(); e != nil {
		pipeWrite.Close()
		return "SPAWN_FAILED", true, wire.Sum(nil)
	}
	pipeWrite.Close()
	readDone := make(chan struct{})
	go func() { _, _ = io.Copy(out, pipeRead); close(readDone) }()
	stopped := make(chan struct{})
	timer := time.NewTimer(time.Duration(def.TimeoutSeconds.Int()) * time.Second)
	defer timer.Stop()
	reason := make(chan string, 1)
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		select {
		case <-ctx.Done():
			reason <- "INTERRUPTED"
			killGate(cmd)
		case <-timer.C:
			reason <- "TIMEOUT"
			killGate(cmd)
		case <-stopped:
		}
	}()
	err := cmd.Wait()
	close(stopped)
	<-watcherDone
	class = "EXIT_ZERO"
	if err != nil {
		class = "EXIT_NONZERO"
	}
	if err == exec.ErrWaitDelay {
		class = "UNKNOWN"
	}
	select {
	case why := <-reason:
		class = why
	default:
	}

	live, e := poolGroupLive(cmd.Process.Pid)
	if e == nil && live {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	until := time.Now().Add(2 * time.Second)
	for e == nil && live && time.Now().Before(until) {
		time.Sleep(10 * time.Millisecond)
		live, e = poolGroupLive(cmd.Process.Pid)
	}
	clean = e == nil && !live && err != exec.ErrWaitDelay
	select {
	case <-readDone:
	case <-time.After(time.Second):
		pipeRead.Close()
		<-readDone
		clean = false
		class = "UNKNOWN"
	}
	if out.overflow {
		class = "OUTPUT_LIMIT"
	}
	return class, clean, wire.Sum(out.buf.Bytes())
}
