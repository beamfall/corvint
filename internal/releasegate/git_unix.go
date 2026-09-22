//go:build !windows

package releasegate

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

func runGitProcess(ctx context.Context, identity gitIdentity, root string, limit int, input []byte, args ...string) ([]byte, error) {
	commandArgs := []string{"--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false", "-c", "core.excludesFile=", "-c", "credential.helper=", "-c", "submodule.recurse=false", "-C", root}
	commandArgs = append(commandArgs, args...)
	command := exec.Command(identity.path, commandArgs...)
	command.Env = sanitizedGitEnvironment()
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Stdin = bytes.NewReader(input)
	stdout, stderr := &limitBuffer{limit: limit}, &limitBuffer{limit: 64 << 10}
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer stdoutRead.Close()
	stderrRead, stderrWrite, err := os.Pipe()
	if err != nil {
		_ = stdoutWrite.Close()
		return nil, err
	}
	defer stderrRead.Close()
	command.Stdout, command.Stderr = stdoutWrite, stderrWrite
	if err := command.Start(); err != nil {
		_ = stdoutWrite.Close()
		_ = stderrWrite.Close()
		return nil, errors.New("sanitized Git command failed to start")
	}
	var copied sync.WaitGroup
	copied.Add(2)
	go func() { _, _ = io.Copy(stdout, stdoutRead); copied.Done() }()
	go func() { _, _ = io.Copy(stderr, stderrRead); copied.Done() }()
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	var runErr error
	select {
	case err := <-done:
		// Git may exit while a hostile descendant retains its process group.
		// Kill that group on every terminal path; a non-existent group is benign.
		runErr = err
	case <-ctx.Done():
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		<-done
		runErr = ctx.Err()
	}
	_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	_ = stdoutWrite.Close()
	_ = stderrWrite.Close()
	copied.Wait()
	if !processGroupGone(command.Process.Pid) {
		return nil, errors.New("sanitized Git descendant did not disappear")
	}
	if runErr != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("sanitized Git command failed")
	}
	if stdout.exceeded || stderr.exceeded {
		return nil, errors.New("Git output exceeds release gate bound")
	}
	return stdout.Bytes(), nil
}

func processGroupGone(pgid int) bool {
	deadline := time.Now().Add(time.Second)
	for {
		err := syscall.Kill(-pgid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return true
		}
		if err != nil && !errors.Is(err, syscall.EPERM) {
			return false
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}
