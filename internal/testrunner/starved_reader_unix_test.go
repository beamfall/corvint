//go:build darwin || linux

package testrunner

import (
	"os/exec"
	"testing"

	"github.com/Beamfall/corvint/internal/testsupport"
)

// TestContainPhasePipeDrainBound: a successful child keeps its output when its reader stalls after
// exit, the old one-second bound fails that run, and a held pipe still fails
// within a finite bound (V1-0391).
func TestContainPhasePipeDrainBound(t *testing.T) {
	testsupport.CheckPipeDrainBound(t, func(c *exec.Cmd) { containPhase(c, false) })
}
