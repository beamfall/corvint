//go:build !darwin && !linux && !windows

package repository

import (
	"context"
	"io"
	"os"
	"os/exec"
	"time"
)

type childContainment struct{}

func prepareContainment(*exec.Cmd) (*childContainment, bool) { return nil, false }
func (*childContainment) attach(*os.Process) bool            { return false }
func (*childContainment) terminate()                         {}
func (*childContainment) force()                             {}
func (*childContainment) quiescent() bool                    { return false }
func (*childContainment) stopAndProve(time.Duration) bool    { return false }
func (*childContainment) close()                             {}
func (*childContainment) wait(*exec.Cmd, context.Context, io.Reader, func() bool) containmentOutcome {
	return containmentOutcome{cleanupProven: true, groupQuiescent: true, executableStable: true}
}
