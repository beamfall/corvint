//go:build unix

package store

import (
	"context"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestTOLV0027_PreflightStatusStopsAtFirstEntry: the post-gate status read
// settles on its first byte and stops the command, rather than reading
// every untracked entry to the end. The command here writes one entry and
// then would run for ten minutes; the read must return dirty at once.
func TestTOLV0027_PreflightStatusStopsAtFirstEntry(t *testing.T) {
	type answer struct {
		dirty bool
		err   error
	}
	done := make(chan answer, 1)
	c := exec.Command("sh", "-c", `printf '?? untracked\0'; exec /bin/sleep 600`)
	go func() {
		dirty, err := writesAnything(context.Background(), c)
		done <- answer{dirty, err}
	}()
	select {
	case a := <-done:
		if !a.dirty || a.err != nil {
			t.Fatalf("status with an entry: dirty=%v err=%v", a.dirty, a.err)
		}
	case <-time.After(20 * time.Second):
		if c.Process != nil {
			_ = c.Process.Kill()
		}
		t.Fatal("the status read did not stop at the first entry")
	}
	// A command that writes nothing is stopped when its context ends.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	started := time.Now()
	if dirty, err := writesAnything(ctx, exec.Command("/bin/sleep", "600")); dirty || err == nil || time.Since(started) > 20*time.Second {
		t.Fatalf("silent command past its context: dirty=%v err=%v after %s", dirty, err, time.Since(started))
	}
	if dirty, err := writesAnything(context.Background(), exec.Command("true")); dirty || err != nil {
		t.Fatalf("empty status: dirty=%v err=%v", dirty, err)
	}
	if _, err := writesAnything(context.Background(), exec.Command("false")); err == nil {
		t.Fatal("a failed status read was not an error")
	}
}

// TestTOLV0027_PreflightBoundedCancelDuringRetireSignalsBeforeReap: a
// cancel that lands while retire runs, with a slow cancel callback, still
// signals the process group only before the command is reaped. Once the
// leader is reaped its pid, the group's id, can be reused, so a later kill
// could reach an unrelated group.
func TestTOLV0027_PreflightBoundedCancelDuringRetireSignalsBeforeReap(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	var events []string
	boundedEvent = func(event string) {
		switch event {
		case "retire":
			cancel()
		case "cancel":
			time.Sleep(300 * time.Millisecond)
		}
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	}
	t.Cleanup(func() { boundedEvent = nil })
	if dirty, err := writesAnything(ctx, exec.Command("true")); dirty {
		t.Fatalf("empty status: dirty=%v err=%v", dirty, err)
	}
	// Wait for the cancel callback's own signal, wherever it lands.
	var seen []string
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		mu.Lock()
		seen = slices.Clone(events)
		mu.Unlock()
		if strings.Count(strings.Join(seen, " "), "signal") >= 2 {
			break
		}
	}
	reaped := slices.Index(seen, "reaped")
	if reaped < 0 || !slices.Contains(seen[:reaped], "cancel") || slices.Contains(seen[reaped+1:], "signal") ||
		strings.Count(strings.Join(seen[:reaped], " "), "signal") != 2 {
		t.Fatalf("bounded call events %v: every signal must precede the reap", seen)
	}
}
