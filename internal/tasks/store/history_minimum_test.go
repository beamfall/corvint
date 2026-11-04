package store

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

// fatalRecorder records the first Fatalf and stops its goroutine, as testing.T does.
type fatalRecorder struct {
	testing.TB
	message string
}

func (r *fatalRecorder) Fatalf(format string, args ...any) {
	r.message = fmt.Sprintf(format, args...)
	runtime.Goexit()
}

// V1-3561: a receipt count below the history fixture's own setup is refused before any build,
// naming the minimum, instead of failing later as "head 51, want 20".
func TestHistoryStoreRefusesReceiptsBelowItsSetup(t *testing.T) {
	recorder := &fatalRecorder{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		historyStore(recorder, 20)
	}()
	<-done
	want := "20 receipts is below its setup's own 51 (init plus 50 ticket creates); pass at least 51"
	if !strings.Contains(recorder.message, want) {
		t.Fatalf("refusal %q lacks %q", recorder.message, want)
	}
}
