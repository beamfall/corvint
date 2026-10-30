package contextindex

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// taskContextCancelStages is every boundary at which compile observes the
// request context (TCP-V0-064), in compile order. "lexical" is inside
// lexicalRows, where V1-0485 observed a cancelled request returning READY.
var taskContextCancelStages = []string{"pair", "mentioned", "subject-slots", "cochange", "lexical-term", "lexical", "ordered", "packet"}

// TestTaskContextCancellationNeverReturnsAPacket covers TCP-V0-064 (V1-0485):
// a request context cancelled at any compile stage, including inside the
// lexical fill after every Git read has finished, returns the cancellation
// refusal and never a packet; the history and recency readers are joined
// before TaskContext returns, observed directly by their finish hooks and,
// deterministically, by a held-reader case that a join which does not await
// them fails (V1-1362); and an uncancelled request afterwards returns the
// same packet as before the cancellations.
func TestTaskContextCancellationNeverReturnsAPacket(t *testing.T) {
	const task, subject = "Reviewer asks whether `Split` handles empty keys in cache/demux.go", "cache/demux.go"
	for _, recency := range []string{"off", "on"} {
		t.Run("recency-"+recency, func(t *testing.T) {
			t.Setenv("CORVINT_CONTEXT_RECENCY", recency)
			index := taskContextFixture(t)
			baseline := taskContextPacketJSON(t, index, task, subject)
			// The subject starts the history reader; recency adds its own.
			readers := []string{"history-read"}
			if recency == "on" {
				readers = append(readers, "recency-read")
			}
			for _, stage := range taskContextCancelStages {
				t.Run(stage, func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					var cancelledAt time.Time
					var finished atomic.Int32
					setTaskContextStage(t, func(at string) {
						if taskContextReaderStage(at) {
							finished.Add(1)
							return
						}
						if at == stage && cancelledAt.IsZero() {
							cancelledAt = time.Now()
							cancel()
						}
					})
					packet, err := TaskContext(ctx, index, task, subject, 20)
					returned := time.Now()
					if got := finished.Load(); got != int32(len(readers)) {
						t.Fatalf("cancelled at %s: %d of %d readers had finished when TaskContext returned", stage, got, len(readers))
					}
					if cancelledAt.IsZero() {
						t.Fatalf("stage %s was never reached", stage)
					}
					var refusal *Error
					if packet != nil || !errors.As(err, &refusal) || refusal.Cause != context.Canceled {
						t.Fatalf("cancelled at %s: packet state=%v err=%v, want the cancellation refusal and no packet", stage, packet["state"], err)
					}
					assertTaskContextReadersJoined(t)
					t.Logf("cancelled at %s: returned %s after cancellation", stage, returned.Sub(cancelledAt))
				})
			}
			for _, reader := range readers {
				t.Run("held-"+reader, func(t *testing.T) {
					assertTaskContextJoinHoldsReader(t, index, task, subject, reader, len(readers))
				})
			}
			setTaskContextStage(t, nil)
			if again := taskContextPacketJSON(t, index, task, subject); again != baseline {
				t.Fatalf("uncancelled packet changed after cancellations:\n%s\n%s", baseline, again)
			}
		})
	}
}

// TestTaskContextPreCancelledRequestReturnsCancellation: a context cancelled
// before the call (the early case) refuses the same way, without a packet,
// for both the subject and the retrieval shape.
func TestTaskContextPreCancelledRequestReturnsCancellation(t *testing.T) {
	index := taskContextFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, subject := range []string{"cache/demux.go", ""} {
		packet, err := TaskContext(ctx, index, "Does `Split` handle empty keys?", subject, 20)
		var refusal *Error
		if packet != nil || !errors.As(err, &refusal) || refusal.Cause != context.Canceled {
			t.Fatalf("subject %q: packet=%v err=%v", subject, packet != nil, err)
		}
	}
	assertTaskContextReadersJoined(t)
}

func taskContextPacketJSON(t *testing.T, index *Index, task, subject string) string {
	t.Helper()
	packet, err := TaskContext(context.Background(), index, task, subject, 20)
	if err != nil || packet["state"] != "READY" {
		t.Fatalf("uncancelled packet: state=%v err=%v", packet["state"], err)
	}
	raw, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func setTaskContextStage(t *testing.T, hook func(string)) {
	t.Helper()
	previous := taskContextStage
	taskContextStage = hook
	t.Cleanup(func() { taskContextStage = previous })
}

func taskContextReaderStage(stage string) bool {
	return stage == "history-read" || stage == "recency-read"
}

// taskContextReaderHold is how long assertTaskContextJoinHoldsReader keeps
// the held reader from finishing. A join that does not await it returns inside
// the hold; a join that does blocks until the hold ends.
const taskContextReaderHold = 150 * time.Millisecond

// assertTaskContextJoinHoldsReader cancels at the first boundary and holds one
// reader in its finish hook, before its ready channel closes, while the other
// finishes freely. TaskContext must not return while it is held: the join
// awaits it on the cancelled path that skips compile's own waits. Holding one
// reader at a time keeps a join that awaits only the other from passing. The
// order is forced, not sampled: a join that stops awaiting the reader fails
// here unless the host stalls the returning call for the whole hold (V1-1362).
func assertTaskContextJoinHoldsReader(t *testing.T, index *Index, task, subject, reader string, readers int) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	release := make(chan struct{})
	var held, entered atomic.Int32
	setTaskContextStage(t, func(at string) {
		if taskContextReaderStage(at) {
			entered.Add(1)
		}
		if at == reader {
			held.Add(1)
			<-release
			return
		}
		if at == taskContextCancelStages[0] {
			cancel()
		}
	})
	type result struct {
		packet map[string]any
		err    error
	}
	done := make(chan result, 1)
	go func() {
		packet, err := TaskContext(ctx, index, task, subject, 20)
		done <- result{packet, err}
	}()
	returnedWhileHeld := func() {
		// A broken join leaves readers running; let each pass its hook read of
		// taskContextStage before the cleanup resets it.
		close(release)
		for deadline := time.Now().Add(5 * time.Second); entered.Load() < int32(readers) && time.Now().Before(deadline); {
			time.Sleep(time.Millisecond)
		}
		t.Fatalf("TaskContext returned while %s was held in its finish hook (reached %d times)", reader, held.Load())
	}
	select {
	case <-done:
		returnedWhileHeld()
	case <-time.After(taskContextReaderHold):
	}
	// select picks at random when both are ready; a return already made
	// during the hold must still fail.
	select {
	case <-done:
		returnedWhileHeld()
	default:
	}
	close(release)
	got := <-done
	var refusal *Error
	if got.packet != nil || !errors.As(got.err, &refusal) || refusal.Cause != context.Canceled {
		t.Fatalf("held %s: packet=%v err=%v, want the cancellation refusal and no packet", reader, got.packet != nil, got.err)
	}
	if n := held.Load(); n != 1 {
		t.Fatalf("%s reached its finish hook %d times, want 1", reader, n)
	}
	assertTaskContextReadersJoined(t)
}

// assertTaskContextReadersJoined is the secondary leak guard: no history or
// recency reader goroutine started by the call may survive it. A reader that
// has closed its ready channel can still show its frame for a moment while it
// exits (hosted run 38476173623), so the check waits a bounded time for the
// frame to go; the finish-hook checks above carry the join guarantee itself.
func assertTaskContextReadersJoined(t *testing.T) {
	t.Helper()
	const grace = 2 * time.Second
	buffer := make([]byte, 1<<20)
	deadline := time.Now().Add(grace)
	for {
		stacks := string(buffer[:runtime.Stack(buffer, true)])
		running := ""
		for _, reader := range []string{"(*taskContextCompiler).startHistory.func", "startContextRecency.func"} {
			if strings.Contains(stacks, reader) {
				running = reader
				break
			}
		}
		if running == "" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s still running %s after TaskContext returned", running, grace)
		}
		time.Sleep(time.Millisecond)
	}
}
