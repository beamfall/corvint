//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package main

import (
	"context"
	"errors"
	"os/exec"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
)

const (
	// residueWindow is how long the exited, still-unreaped leader's group is
	// given to become quiet on its own before the run counts as residue.
	residueWindow = 250 * time.Millisecond
	// residueInterval separates the pre-reap residue observations.
	residueInterval = 10 * time.Millisecond
	// retirementWindow bounds the owner's retirement before the reap; a group
	// not proved quiet within it is HOLD, never success.
	retirementWindow = 5 * time.Second
)

// executionHost is the groupreap host for the candidate's process group.
// Its post-reap probe first collects the zombies of members reparented to
// this subreaper (Linux), which signal 0 would otherwise count as live; the
// owner calls it only after the leader reap, so it can never reap the leader.
// The pre-reap quiet proof stays the platform's own. A candidate that keeps
// forking is SIGKILLed again on each pre-reap live observation (PGO-V0-017).
// Tests replace it to record the order of signals and the reap (PGO-V0-019).
var executionHost = func() groupreap.Primitives {
	host := groupreap.HostPrimitives()
	probe := host.ProbeGroup
	host.ProbeGroup = func(leader int) (groupreap.Probe, error) {
		if err := reapExecutionChildren(); err != nil {
			return groupreap.ProbeLive, err
		}
		return probe(leader)
	}
	host.RepeatKillWhileLive = true
	return host
}

// heldGroupQuiet is groupreap.HeldGroupQuiet; tests inject residue.
var heldGroupQuiet = groupreap.HeldGroupQuiet

// ownedExecution runs one candidate as the leader of a process group owned
// through its unreaped leader, so every group signal (the timeout stop, the
// output-limit stop that fires on an output copy goroutine, and the
// retirement SIGKILL) is sent while the leader's PID still pins the group ID
// and never after the reap (V1-1387). Only signal-0 observations follow the
// reap. The original group only is observed: a descendant that leaves it
// (setsid or setpgid) is adopted by the subreaper or init and is outside this
// observation.
type ownedExecution struct {
	mu      sync.Mutex
	owner   *groupreap.Owner
	stopped bool
}

// stop retires the group at once. A stop that arrives before the owner is
// installed (an output copy goroutine can run before StartWith returns) is
// applied as soon as it is.
func (x *ownedExecution) stop() {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.stopped = true
	if x.owner != nil {
		x.owner.Stop()
	}
}

// executionOutcome is the receipt view of one owned run. status is set only
// when the group outcome decides the run's status.
type executionOutcome struct {
	runErr         error
	group, cleanup string
	status, reason string
	unsupported    bool
}

// run starts c, stops its group when ctx ends, and retires the group. The
// group is residue when it holds a live member residueWindow after the
// leader exited; it is then SIGKILLed before the reap. A retirement that is
// not proved within retirementWindow is CLEANUP_ERROR/PROCESS_GROUP_HOLD,
// and a platform without the owner is UNSUPPORTED_EXECUTION_AUTHORITY.
func (x *ownedExecution) run(ctx context.Context, c *exec.Cmd) executionOutcome {
	c.WaitDelay = 250 * time.Millisecond
	owner, err := groupreap.StartWith(c, executionHost())
	if errors.Is(err, groupreap.ErrOwnerUnavailable) {
		return executionOutcome{group: "NOT_OBSERVED", cleanup: "NOT_OBSERVED", status: "UNSUPPORTED_EXECUTION_AUTHORITY", reason: "UNSUPPORTED_EXECUTION_AUTHORITY", unsupported: true}
	}
	if err != nil {
		return executionOutcome{runErr: err, group: "NOT_OBSERVED", cleanup: "NOT_OBSERVED"}
	}
	x.mu.Lock()
	x.owner = owner
	if x.stopped {
		owner.Stop()
	}
	x.mu.Unlock()
	select {
	case <-owner.Exited():
	case <-ctx.Done():
		owner.Stop()
	}
	bound, cancel := context.WithTimeout(context.Background(), retirementWindow)
	defer cancel()
	residue, proofErr := false, error(nil)
	select {
	case <-owner.Exited():
		residue, proofErr = awaitHeldQuiet(c.Process.Pid)
	case <-bound.Done():
	}
	result := owner.FinishBounded(groupreap.RetirementBound{Done: bound.Done()})
	out := executionOutcome{runErr: result.WaitErr}
	switch {
	case result.State != groupreap.Released:
		out.group, out.cleanup, out.status, out.reason = "NOT_EMPTY_OR_NOT_OBSERVED", "NOT_EMPTY_OR_NOT_OBSERVED", "CLEANUP_ERROR", "PROCESS_GROUP_HOLD"
	case proofErr != nil:
		out.group, out.cleanup, out.status, out.reason = "NOT_EMPTY_OR_NOT_OBSERVED", "ERROR", "CLEANUP_ERROR", "PROCESS_GROUP_STATE"
	case residue:
		out.group, out.cleanup, out.status, out.reason = "CLEANED_AFTER_RESIDUE", "CLEANED_AFTER_RESIDUE", "PROCESS_GROUP_RESIDUE", "PROCESS_GROUP_RESIDUE"
	default:
		out.group, out.cleanup = "GROUP_EMPTY_OBSERVED", "GROUP_EMPTY_OBSERVED"
	}
	return out
}

// awaitHeldQuiet polls the pre-reap quiet proof of the exited, unreaped
// leader's group for residueWindow. It reports residue when a member is
// still live at the end and sends no signal other than Darwin's signal 0.
func awaitHeldQuiet(leader int) (residue bool, err error) {
	deadline := time.NewTimer(residueWindow)
	defer deadline.Stop()
	tick := time.NewTicker(residueInterval)
	defer tick.Stop()
	for {
		quiet, err := heldGroupQuiet(leader)
		if err != nil {
			return false, err
		}
		if quiet {
			return false, nil
		}
		select {
		case <-deadline.C:
			return true, nil
		case <-tick.C:
		}
	}
}
