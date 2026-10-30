package snapshot

import "time"

// A native writer holds descriptor-less staging slots under its writer lock
// for the whole of its publish, so a reader that does not hold the lock and
// sees such slots with no stage descriptor and no descriptor temp waits for
// the writer before returning the MALFORMED "unassigned stage slot" that a
// killed writer's orphan earns (CTS-V0-008). The journal audit and the
// archive export share this backoff and budget. The budget does not follow
// DefaultPatience: test binaries zero that, and the race is real in them too.
const (
	StageSlotPatience     = 2 * time.Second
	StageSlotPause        = 25 * time.Millisecond
	StageSlotPauseCeiling = 400 * time.Millisecond
)

// StageSlotWait is one reader's CTS-V0-008 wait. Its zero value is ready.
type StageSlotWait struct{ waited, pause time.Duration }

// Next returns the next pause, doubling from StageSlotPause up to
// StageSlotPauseCeiling and trimmed so the pauses sum to exactly
// StageSlotPatience, or false once that budget is spent.
func (w *StageSlotWait) Next() (time.Duration, bool) {
	if w.waited >= StageSlotPatience {
		return 0, false
	}
	if w.pause == 0 {
		w.pause = StageSlotPause
	}
	step := min(w.pause, StageSlotPatience-w.waited)
	w.waited += step
	w.pause = min(2*w.pause, StageSlotPauseCeiling)
	return step, true
}
