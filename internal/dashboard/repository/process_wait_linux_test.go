//go:build linux

package repository

import (
	"context"
	"errors"
	"syscall"
	"testing"
	"time"
)

// V1-1047 review P2: an unreadable process table during cancellation still
// SIGKILLs the pinned group, so a TERM-ignoring leader cannot block the
// wait, and the cleanup stays unproved.
func TestV11047UnreadableProcessTableStillKillsPinnedGroup(t *testing.T) {
	previous := linuxGroupLive
	t.Cleanup(func() { linuxGroupLive = previous })
	linuxGroupLive = func(int) (bool, error) { return true, errors.New("injected /proc failure") }
	signals := recordGroupSignals(t)
	command, containment := startContained(t, "trap '' TERM; while :; do /bin/sleep 1; done")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	result := make(chan containmentOutcome, 1)
	go func() { result <- containment.wait(command, ctx, nil, func() bool { return true }) }()
	select {
	case outcome := <-result:
		if !outcome.timedOutOrCancelled || outcome.cleanupProven {
			t.Fatalf("outcome = %#v: want cancelled with unproved cleanup", outcome)
		}
		sent := signals()
		if len(sent) == 0 || sent[len(sent)-1] != syscall.SIGKILL {
			t.Fatalf("signals %v: want a final group SIGKILL", sent)
		}
	case <-time.After(20 * time.Second):
		containment.force()
		t.Fatal("wait blocked on a TERM-ignoring leader after a /proc failure")
	}
}
