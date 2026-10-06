//go:build darwin || linux

package criterionexperiment

import (
	"testing"

	"github.com/Beamfall/corvint/internal/testsupport"
)

// TestOwnGroupPipeDrainBound: a successful child keeps its output when its reader stalls after
// exit, the old one-second bound fails that run, and a held pipe still fails
// within a finite bound (V1-0391).
func TestOwnGroupPipeDrainBound(t *testing.T) {
	testsupport.CheckPipeDrainBound(t, ownGroup)
}
