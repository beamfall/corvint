//go:build linux

package repository

import (
	"context"
	"io"
	"os/exec"
)

func (containment *childContainment) wait(command *exec.Cmd, childContext context.Context, stdin io.Reader, verifyExecutable func() bool) containmentOutcome {
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	var waitErr error
	timedOutOrCancelled := false
	select {
	case waitErr = <-done:
	case <-childContext.Done():
		timedOutOrCancelled = true
	}
	executableStable := true
	if !timedOutOrCancelled {
		executableStable = verifyExecutable()
	}

	// A leader may exit successfully while leaving descendants behind. Record
	// that fact before cleanup: successful cleanup is proof that residue was
	// removed, not permission to accept the command's output.
	residueAfterLeader := !timedOutOrCancelled && !containment.quiescent()
	cleanupRequired := timedOutOrCancelled || residueAfterLeader
	cleanupProven := true
	if cleanupRequired {
		closeReader(stdin)
		cleanupProven = containment.stopAndProve(cleanupDeadline)
		if timedOutOrCancelled {
			waitErr = <-done
			executableStable = verifyExecutable()
		}
	}
	groupQuiescent := containment.quiescent()
	return containmentOutcome{
		waitErr: waitErr, timedOutOrCancelled: timedOutOrCancelled,
		residueAfterLeader: residueAfterLeader, cleanupProven: cleanupProven,
		groupQuiescent: groupQuiescent, executableStable: executableStable,
	}
}
