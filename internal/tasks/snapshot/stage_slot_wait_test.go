package snapshot

import (
	"reflect"
	"testing"
	"time"
)

// CTS-V0-008: the shared stage-slot wait doubles from 25ms to a 400ms
// ceiling, sums to exactly two seconds, and then reports its budget spent.
func TestCTSV0008_StageSlotWaitBackoffAndBudget(t *testing.T) {
	ms := time.Millisecond
	want := []time.Duration{25 * ms, 50 * ms, 100 * ms, 200 * ms, 400 * ms, 400 * ms, 400 * ms, 400 * ms, 25 * ms}
	var w StageSlotWait
	var got []time.Duration
	for {
		d, ok := w.Next()
		if !ok {
			break
		}
		got = append(got, d)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pauses %v; want %v", got, want)
	}
	if _, ok := w.Next(); ok {
		t.Fatal("a spent wait paused again")
	}
}
