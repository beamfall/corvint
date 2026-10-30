package cli

import "time"

// SetObligationWorklistNow pins the worklist's review-age clock and returns
// the restore func.
func SetObligationWorklistNow(now time.Time) func() {
	was := obligationWorklistNow
	obligationWorklistNow = func() time.Time { return now }
	return func() { obligationWorklistNow = was }
}
