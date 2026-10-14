//go:build darwin || linux

package procgroup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// loadSlack absorbs scheduling delay on a loaded host; it is not part of the bound.
const loadSlack = 2 * time.Second

// V1-1368: a slow but healthy /bin/ps binds the leader; a hung one still fails
// closed with the unavailable outcome inside the settle window.
func TestV11368SnapshotBoundUnderLoad(t *testing.T) {
	t.Run("a snapshot slower than one second binds the leader", func(t *testing.T) {
		row := fmt.Sprintf("%d 1 Wed Oct 14 00:00:00 2026 S\\n", os.Getpid())
		o := &descendantObserver{snap: func(ctx context.Context) (map[int]ObservedProcess, error) {
			return descendantSnapshotCommand(exec.CommandContext(ctx, "/bin/sh", "-c", "/bin/sleep 1.2; printf '"+row+"'"))
		}}
		started, err := startObserver(context.Background(), o, os.Getpid())
		if err != nil {
			t.Fatalf("slow snapshot refused: %v", err)
		}
		if _, err := started.finish(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("an unavailable first snapshot is retried inside the settle window", func(t *testing.T) {
		var calls atomic.Int32
		o := &descendantObserver{snap: func(ctx context.Context) (map[int]ObservedProcess, error) {
			switch n := calls.Add(1); {
			case n < 3:
				return descendantSnapshotCommand(exec.CommandContext(ctx, "/bin/sh", "-c", "exit 1"))
			case n == 3:
				return descendantSnapshot(ctx)
			}
			return map[int]ObservedProcess{}, nil
		}}
		started, err := startObserver(context.Background(), o, os.Getpid())
		if err != nil || calls.Load() < 3 {
			t.Fatalf("first snapshot not retried: calls=%d %v", calls.Load(), err)
		}
		if _, err := started.finish(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("a hung ps fails startup as unavailable within the settle window", func(t *testing.T) {
		o := &descendantObserver{snap: func(ctx context.Context) (map[int]ObservedProcess, error) {
			return descendantSnapshotCommand(exec.CommandContext(ctx, "/bin/sleep", "60"))
		}}
		began := time.Now()
		started, err := startObserver(context.Background(), o, os.Getpid())
		waited := time.Since(began)
		if started != nil || err == nil || err.Error() != "descendant snapshot unavailable" {
			t.Fatalf("hung snapshot accepted: %v %v", started, err)
		}
		if waited < descendantSettle || waited > descendantSettle+loadSlack {
			t.Fatalf("startup refusal outside the bound: %v", waited)
		}
	})
	t.Run("a cancelled start stops retrying after the attempt in flight", func(t *testing.T) {
		var calls atomic.Int32
		o := &descendantObserver{snap: func(ctx context.Context) (map[int]ObservedProcess, error) {
			calls.Add(1)
			return descendantSnapshotCommand(exec.CommandContext(ctx, "/bin/sh", "-c", "exit 1"))
		}}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		started, err := startObserver(ctx, o, os.Getpid())
		if started != nil || err == nil || err.Error() != "descendant snapshot unavailable" || calls.Load() != 1 {
			t.Fatalf("cancelled start not refused after one attempt: calls=%d %v %v", calls.Load(), started, err)
		}
	})
	t.Run("a hung ps in the final sweep reports unavailable, not an empty set", func(t *testing.T) {
		done := make(chan struct{})
		close(done)
		owned := ObservedProcess{PID: os.Getpid(), ParentPID: os.Getppid(), Start: "owned", State: "S"}
		o := descendantObserver{root: os.Getppid(), known: map[int]ObservedProcess{owned.PID: owned}, stop: make(chan struct{}), done: done, seen: time.Now(), snap: func(ctx context.Context) (map[int]ObservedProcess, error) {
			return descendantSnapshotCommand(exec.CommandContext(ctx, "/bin/sleep", "60"))
		}}
		began := time.Now()
		report, err := o.finish()
		waited := time.Since(began)
		if err == nil || report.Absent || len(report.Failures) != 1 || !strings.Contains(report.Failures[0], "descendant snapshot unavailable") {
			t.Fatalf("hung final sweep passed: %v %+v", err, report)
		}
		if waited < descendantSettle || waited > descendantSettle+loadSlack {
			t.Fatalf("final sweep outside the settle bound: %v", waited)
		}
	})
}
