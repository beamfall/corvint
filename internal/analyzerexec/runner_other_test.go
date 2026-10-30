//go:build !darwin && !linux

package analyzerexec

import (
	"context"
	"testing"
)

func TestUnavailableOtherHostRetainsCleanupNotRun(t *testing.T) {
	result, err := Run(context.Background(), Plan{})
	if !Is(err, Unsupported) || result.Started || result.CleanupState != CleanupNotRun || !result.ValidCleanupObservation() {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}
