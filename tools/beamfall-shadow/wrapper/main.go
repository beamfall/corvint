//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

// beamfall-shadow-wrapper is intentionally tiny: it gives run.sh a portable
// process-group signal and reaping boundary around `go run`.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	code := run(os.Args[1:], launchHooks{})
	if code == 125 {
		fmt.Fprintln(os.Stderr, "beamfall-shadow-wrapper: UNSUPPORTED_EXECUTION_AUTHORITY")
	}
	os.Exit(code)
}

type launchHooks struct {
	afterSignalInstall func()
	beforeStart        func()
	afterStart         func()
}

func run(args []string, hooks launchHooks) int {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGHUP, syscall.SIGTERM)
	defer signal.Stop(signals)
	if hooks.afterSignalInstall != nil {
		hooks.afterSignalInstall()
	}
	if pendingSignal(signals) {
		return 128
	}
	if len(args) < 2 || args[0] != "--" {
		return 2
	}
	if !establishWrapperAuthority() {
		return 125
	}
	command := exec.Command(args[1], args[2:]...)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if hooks.beforeStart != nil {
		hooks.beforeStart()
	}
	if pendingSignal(signals) {
		return 128
	}
	if err := command.Start(); err != nil {
		return 1
	}
	if hooks.afterStart != nil {
		hooks.afterStart()
	}
	if pendingSignal(signals) {
		interrupt(command, nil)
		return 128
	}
	result := make(chan error, 1)
	go func() { result <- command.Wait() }()
	select {
	case err := <-result:
		reap(command.Process.Pid)
		if err != nil {
			return 1
		}
	case signal := <-signals:
		_ = signal
		interrupt(command, result)
		return 128
	}
	return 0
}

func pendingSignal(signals <-chan os.Signal) bool {
	select {
	case <-signals:
		return true
	case <-time.After(20 * time.Millisecond):
		return false
	}
}

func interrupt(command *exec.Cmd, waited <-chan error) {
	if command.Process == nil {
		return
	}
	_ = syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
	if waited == nil {
		result := make(chan error, 1)
		go func() { result <- command.Wait() }()
		waited = result
	}
	select {
	case <-waited:
	case <-time.After(time.Second):
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		<-waited
	}
	reap(command.Process.Pid)
}

func reap(pid int) {
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return
	}
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		reapWrapperChildren()
		if errors.Is(syscall.Kill(-pid, 0), syscall.ESRCH) {
			return
		}
	}
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}
