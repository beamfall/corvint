package behaviorfalsify

import (
	"context"
	"errors"
	"io/fs"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

// TestStartProcessRetriesTextBusy pins the bounded retry of a staged executable whose exec failed
// ETXTBSY because a concurrent fork briefly held its write descriptor: the start is retried until
// it starts, other start failures are not retried, and the retries are bounded. It swaps package
// seams, so it does not run in parallel.
func TestStartProcessRetriesTextBusy(t *testing.T) {
	savedRun, savedRetries := runProcessGroup, textBusyRetries
	t.Cleanup(func() { runProcessGroup, textBusyRetries = savedRun, savedRetries })
	textBusyRetries = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	busy := procgroup.Observation{Err: &fs.PathError{Op: "fork/exec", Path: "/staged", Err: syscall.ETXTBSY}}
	script := func(results ...procgroup.Observation) *int {
		calls := 0
		runProcessGroup = func(context.Context, procgroup.Spec) procgroup.Observation {
			result := results[min(calls, len(results)-1)]
			calls++
			return result
		}
		return &calls
	}

	calls := script(busy, busy, procgroup.Observation{Started: true})
	if got := startProcess(context.Background(), procgroup.Spec{}); !got.Started || *calls != 3 {
		t.Fatalf("text busy twice then start: started=%v calls=%d", got.Started, *calls)
	}
	calls = script(procgroup.Observation{Err: &fs.PathError{Op: "fork/exec", Path: "/staged", Err: syscall.ENOENT}})
	if got := startProcess(context.Background(), procgroup.Spec{}); got.Started || *calls != 1 {
		t.Fatalf("other start failure: started=%v calls=%d, want one call", got.Started, *calls)
	}
	calls = script(busy)
	if got := startProcess(context.Background(), procgroup.Spec{}); !errors.Is(got.Err, syscall.ETXTBSY) || *calls != 4 {
		t.Fatalf("persistent text busy: err=%v calls=%d, want 4 bounded calls", got.Err, *calls)
	}
}
