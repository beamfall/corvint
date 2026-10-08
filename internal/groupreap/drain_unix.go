//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package groupreap

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// Drain starts command and returns the wait that reaps it. Unlike Wait, it
// owns the output pipes so the group is swept only after they drain: a
// descendant that still holds a pipe delay after the leader exits is reported
// as incomplete capture (exec.ErrWaitDelay, its pipe closed), not killed into
// apparent completion. ctx ending stops the leader at once (Stop) and then
// sweeps the group as soon as the leader's exit is observed. Where waitid is
// available every group signal is sent while the leader is held unreaped.
//
// Drain starts command as StartLive does: a group it leads is recorded from
// the start until after the drain and the sweep, and released just before the
// reap, so KillLive retires a descendant that still holds a pipe (AHI-048). A
// start after KillLive is refused with ErrExiting.
//
// Stdout and Stderr must be distinct writers (or nil, or *os.File, which are
// passed through untracked as os/exec does). A command without its own new
// process group (Setpgid with Pgid 0) is started and waited plainly.
func Drain(ctx context.Context, command *exec.Cmd, delay time.Duration) (func() error, error) {
	attributes := command.SysProcAttr
	if attributes == nil || !attributes.Setpgid || attributes.Pgid != 0 {
		if err := liveGroups.start(command); err != nil {
			return nil, err
		}
		return command.Wait, nil
	}
	var pipes []*os.File
	copies := make(chan error, 2)
	copiers := 0
	writeEnds := []*os.File{}
	closeAll := func(files []*os.File) {
		for _, file := range files {
			_ = file.Close()
		}
	}
	for _, stream := range []*io.Writer{&command.Stdout, &command.Stderr} {
		if _, isFile := (*stream).(*os.File); *stream == nil || isFile {
			continue
		}
		read, write, err := os.Pipe()
		if err != nil {
			closeAll(pipes)
			closeAll(writeEnds)
			return nil, err
		}
		pipes, writeEnds = append(pipes, read), append(writeEnds, write)
		destination := *stream
		*stream = write
		copiers++
		go func() {
			_, err := io.Copy(destination, read)
			_ = read.Close()
			copies <- err
		}()
	}
	err := liveGroups.start(command)
	closeAll(writeEnds)
	if err != nil {
		closeAll(pipes)
		for ; copiers > 0; copiers-- {
			<-copies
		}
		return nil, err
	}
	processID := command.Process.Pid
	drained := make(chan error, 1)
	go func() {
		var first error
		for range copiers {
			if err := <-copies; first == nil {
				first = err
			}
		}
		drained <- first
	}()
	return func() error {
		exited := make(chan error, 1)
		go func() { exited <- observeExit(processID) }()
		var observed error
		select {
		case observed = <-exited:
		case <-ctx.Done():
			// The leader may already be reaped by a foreign reaper, so stop
			// only the leader here; the group is swept below once the exit
			// observation proves the leader is held unreaped (V1-0652).
			_ = Stop(command)
			observed = <-exited
		}
		var waitErr error
		if observed != nil {
			// The leader cannot be held unreaped: release its record, reap
			// it now and send no group signal after it, except the legacy
			// one below.
			liveGroups.release(processID)
			waitErr = command.Wait()
		}
		timedOut, copyErr := false, error(nil)
		timer := time.NewTimer(delay)
		defer timer.Stop()
		cancelled := ctx.Done()
		if observed != nil {
			cancelled = nil
		}
	drain:
		for {
			select {
			case copyErr = <-drained:
				break drain
			case <-timer.C:
				timedOut = true
				break drain
			case <-cancelled:
				_ = signalGroup(-processID, syscall.SIGKILL)
				cancelled = nil
			}
		}
		if observed == nil || errors.Is(observed, errors.ErrUnsupported) {
			_ = signalGroup(-processID, syscall.SIGKILL)
		}
		if timedOut {
			closeAll(pipes)
			<-drained
		}
		if observed == nil {
			liveGroups.release(processID)
			waitErr = command.Wait()
		}
		switch {
		case waitErr != nil:
			return waitErr
		case timedOut:
			return exec.ErrWaitDelay
		}
		return copyErr
	}, nil
}
