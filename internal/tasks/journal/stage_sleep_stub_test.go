package journal

import (
	"testing"
	"time"
)

// This helper stays out of the darwin||linux-tagged stage_slot_wait_test.go
// because the untagged projection_order_test.go also calls it, and the
// Windows cross-vet type-checks that file.

// stubStageSleep replaces the stage wait's sleep for one test, recording
// each pause and running then (if set) in place of sleeping.
func stubStageSleep(t *testing.T, then func(n int)) *[]time.Duration {
	t.Helper()
	pauses := &[]time.Duration{}
	saved := stageSleep
	stageSleep = func(d time.Duration) {
		*pauses = append(*pauses, d)
		if then != nil {
			then(len(*pauses))
		}
	}
	t.Cleanup(func() { stageSleep = saved })
	return pauses
}
