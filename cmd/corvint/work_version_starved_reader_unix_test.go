//go:build darwin || linux

package main

import (
	"testing"

	"github.com/Beamfall/corvint/internal/testsupport"
)

// TestWorkContainVersionPipeDrainBound: a successful child keeps its output when its reader stalls after
// exit, the old one-second bound fails that run, and a held pipe still fails
// within a finite bound (V1-0391).
func TestWorkContainVersionPipeDrainBound(t *testing.T) {
	testsupport.CheckPipeDrainBound(t, workContainVersion)
}
