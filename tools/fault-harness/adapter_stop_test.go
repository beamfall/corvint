// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

// TestGoAdapterOverflowIsFailedMeasurement drives the real Go adapter through
// command: output past the capture bound must come back marked, so runFault
// records ERROR and never NO_KILL.
func TestGoAdapterOverflowIsFailedMeasurement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh and head")
	}
	if _, err := exec.LookPath("head"); err != nil {
		t.Skip("head not on PATH")
	}
	a := goAdapter{goTest: []string{"sh", "-c", "head -c 34000000 /dev/zero; exit 1", "go-test"}, timeout: time.Minute}
	ctx := context.Background()
	_, err := a.Run(ctx, t.TempDir(), nil)
	if !errors.Is(err, errOutputOverflow) || !stopped(ctx, err) {
		t.Fatalf("Run error = %v, want an output overflow the harness reads as stopped", err)
	}
}

// TestStoppedKeepsEveryHarnessMark pins that each harness failure, wrapped as
// the adapter wraps it, still reads as a failed measurement.
func TestStoppedKeepsEveryHarnessMark(t *testing.T) {
	ctx := context.Background()
	for _, mark := range []error{errWallTimeout, errCleanupUnproved, errOutputOverflow} {
		wrapped := errors.Join(errors.New("go test stopped by the harness"), mark)
		if !stopped(ctx, wrapped) {
			t.Errorf("stopped(%v) = false, want true", wrapped)
		}
	}
}
