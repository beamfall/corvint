package contextindex

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"strings"
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
// before TaskContext returns; and an uncancelled request afterwards returns
// the same packet as before the cancellations.
func TestTaskContextCancellationNeverReturnsAPacket(t *testing.T) {
	const task, subject = "Reviewer asks whether `Split` handles empty keys in cache/demux.go", "cache/demux.go"
	for _, recency := range []string{"off", "on"} {
		t.Run("recency-"+recency, func(t *testing.T) {
			t.Setenv("CORVINT_CONTEXT_RECENCY", recency)
			index := taskContextFixture(t)
			baseline := taskContextPacketJSON(t, index, task, subject)
			for _, stage := range taskContextCancelStages {
				t.Run(stage, func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					var cancelledAt time.Time
					setTaskContextStage(t, func(at string) {
						if at == stage && cancelledAt.IsZero() {
							cancelledAt = time.Now()
							cancel()
						}
					})
					packet, err := TaskContext(ctx, index, task, subject, 20)
					returned := time.Now()
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

// assertTaskContextReadersJoined fails if a history or recency reader started
// by the call is still running: TaskContext joins both before it returns, so
// none may remain once it has.
func assertTaskContextReadersJoined(t *testing.T) {
	t.Helper()
	buffer := make([]byte, 1<<20)
	stacks := string(buffer[:runtime.Stack(buffer, true)])
	for _, reader := range []string{"(*taskContextCompiler).startHistory.func", "startContextRecency.func"} {
		if strings.Contains(stacks, reader) {
			t.Fatalf("%s still running after TaskContext returned", reader)
		}
	}
}
