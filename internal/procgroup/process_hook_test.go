//go:build darwin || linux

package procgroup

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestBeforeStopFailureCannotBypassGroupCleanup(t *testing.T) {
	for _, mode := range []string{"snapshot-error", "panic", "deadline", "success"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			r := Run(context.Background(), Spec{Dir: t.TempDir(), Argv: []string{"/bin/sh", "-c", "printf ready; sleep 30 & wait"}, Env: []string{"PATH=/usr/bin:/bin"}, Timeout: 80 * time.Millisecond, ShutdownTimeout: 400 * time.Millisecond, BeforeStop: func(ctx context.Context, pid int) error {
				calls++
				if pid <= 0 {
					t.Error("missing leader")
				}
				switch mode {
				case "snapshot-error":
					return errors.New("snapshot unavailable")
				case "panic":
					panic("private detail")
				case "deadline":
					<-ctx.Done()
					return nil
				}
				return nil
			}})
			if calls != 1 || !r.TimedOut || !r.WaitCompleted || !r.PipesDrained || !r.OwnedProcessGroupCleanup || string(r.Stdout) != "ready" {
				t.Fatalf("cleanup lost: calls=%d %+v", calls, r)
			}
			if mode != "success" && (r.Err == nil || !strings.Contains(r.Err.Error(), "process-before-stop-failed")) {
				t.Fatalf("hook failure lost: %+v", r)
			}
			if r.Err != nil && strings.Contains(r.Err.Error(), "private detail") {
				t.Fatal("panic leaked detail")
			}
		})
	}
}
func TestBeforeStopRacedExitStillRunsBeforeReap(t *testing.T) {
	calls := 0
	r := Run(context.Background(), Spec{Dir: t.TempDir(), Argv: []string{"/bin/sh", "-c", "exit 23"}, Env: []string{"PATH=/usr/bin:/bin"}, Timeout: time.Second, ShutdownTimeout: time.Second, BeforeStop: func(ctx context.Context, pid int) error { calls++; return nil }})
	if calls != 1 || r.ExitStatus != 23 || r.Err != nil || !r.WaitCompleted || !r.OwnedProcessGroupCleanup {
		t.Fatalf("raced exit lost: calls=%d %+v", calls, r)
	}
}
