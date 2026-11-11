package contextindex

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

// taskContextCancelStages is every boundary at which compile observes the
// request context (TCP-V0-064), in compile order. "lexical" is inside
// lexicalRows, where V1-0485 observed a cancelled request returning READY.
var taskContextCancelStages = []string{"pair", "mentioned", "subject-slots", "cochange", "lexical-term", "lexical", "ordered", "packet"}

func taskContextReaderStage(stage string) bool {
	return stage == "history-read" || stage == "recency-read"
}

// taskContextReaderHold is how long assertTaskContextJoinHoldsReader keeps
// the held reader from finishing. A join that does not await it returns inside
// the hold; a join that does blocks until the hold ends.
const taskContextReaderHold = 150 * time.Millisecond

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
