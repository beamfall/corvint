//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

// beamfall-shadow-wrapper is intentionally tiny: it gives run.sh a portable
// process-group signal and reaping boundary around `go run`.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
)

const (
	// unsupportedExitCode reports a host without the execution authority:
	// the Linux subreaper, or the unreaped-leader process-group owner.
	unsupportedExitCode = 125
	// holdExitCode reports a group whose retirement was not proved within
	// retirementWindow (groupreap HOLD); it is never success.
	holdExitCode = 124
	// stopGrace is how long the group has after SIGTERM before SIGKILL.
	stopGrace = time.Second
	// retirementWindow bounds the owner's retirement before the reap.
	retirementWindow = 5 * time.Second
	// quietInterval separates pre-reap quiet observations.
	quietInterval = 10 * time.Millisecond
)

func main() {
	code := run(os.Args[1:], launchHooks{})
	switch code {
	case unsupportedExitCode:
		fmt.Fprintln(os.Stderr, "beamfall-shadow-wrapper: UNSUPPORTED_EXECUTION_AUTHORITY")
	case holdExitCode:
		fmt.Fprintln(os.Stderr, "beamfall-shadow-wrapper: PROCESS_GROUP_HOLD")
	}
	os.Exit(code)
}

type launchHooks struct {
	afterSignalInstall func()
	beforeStart        func()
	afterStart         func()
}

// wrapperHost is the groupreap host for the wrapped group. Its post-reap
// probe first collects zombies of members reparented to this subreaper
// (Linux); the owner calls it only after the leader reap, so it cannot reap
// the leader. `go run` forks, so the group SIGKILL repeats on each pre-reap
// live observation (PGO-V0-017). Tests replace it to record the order.
var wrapperHost = func() groupreap.Primitives {
	host := groupreap.HostPrimitives()
	probe := host.ProbeGroup
	host.ProbeGroup = func(leader int) (groupreap.Probe, error) {
		reapWrapperChildren()
		return probe(leader)
	}
	host.RepeatKillWhileLive = true
	return host
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
	if !establishWrapperAuthority() || !groupreap.OwnerAvailable() {
		return unsupportedExitCode
	}
	command := exec.Command(args[1], args[2:]...)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	if hooks.beforeStart != nil {
		hooks.beforeStart()
	}
	if pendingSignal(signals) {
		return 128
	}
	owner, err := groupreap.StartWith(command, wrapperHost())
	if errors.Is(err, groupreap.ErrOwnerUnavailable) {
		return unsupportedExitCode
	}
	if err != nil {
		return 1
	}
	if hooks.afterStart != nil {
		hooks.afterStart()
	}
	if pendingSignal(signals) {
		return retire(owner, command.Process.Pid, true)
	}
	select {
	case <-owner.Exited():
		return retire(owner, command.Process.Pid, false)
	case <-signals:
		return retire(owner, command.Process.Pid, true)
	}
}

func pendingSignal(signals <-chan os.Signal) bool {
	select {
	case <-signals:
		return true
	case <-time.After(20 * time.Millisecond):
		return false
	}
}

// retire ends the owned group. Every group signal is sent while the leader is
// held unreaped, so none can name a reused group ID (V1-1387): SIGTERM to the
// group, up to stopGrace for the leader to exit and the group to go quiet,
// SIGKILL to whatever remains, the reap, then signal-0 observations only. An
// unproved retirement is holdExitCode; an interrupted run is 128.
func retire(owner *groupreap.Owner, leader int, interrupted bool) int {
	_ = owner.Signal(syscall.SIGTERM)
	grace := time.NewTimer(stopGrace)
	defer grace.Stop()
	select {
	case <-owner.Exited():
		awaitQuiet(leader, grace.C)
	case <-grace.C:
		owner.Stop()
	}
	bound, cancel := context.WithTimeout(context.Background(), retirementWindow)
	defer cancel()
	result := owner.FinishBounded(groupreap.RetirementBound{Done: bound.Done()})
	switch {
	case result.State != groupreap.Released:
		return holdExitCode
	case interrupted:
		return 128
	case result.WaitErr != nil:
		return 1
	}
	return 0
}

// awaitQuiet polls the exited, unreaped leader's group until it holds no
// live member or done closes. It sends no signal other than Darwin's signal 0.
func awaitQuiet(leader int, done <-chan time.Time) {
	tick := time.NewTicker(quietInterval)
	defer tick.Stop()
	for {
		if quiet, err := groupreap.HeldGroupQuiet(leader); quiet || err != nil {
			return
		}
		select {
		case <-done:
			return
		case <-tick.C:
		}
	}
}
