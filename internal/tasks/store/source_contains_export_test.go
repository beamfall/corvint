package store

import "time"

// SetSourceContainmentScanBoundForTest lowers the SCC-V0-003 patch-id scan
// bound and returns the restore function.
func SetSourceContainmentScanBoundForTest(n int) func() {
	prev := sourceContainmentScanBound
	sourceContainmentScanBound = n
	return func() { sourceContainmentScanBound = prev }
}

// SetSourceContainmentTimeoutForTest sets the SCC-V0-003 containment
// deadline and returns the restore function.
func SetSourceContainmentTimeoutForTest(d time.Duration) func() {
	prev := sourceContainmentTimeout
	sourceContainmentTimeout = d
	return func() { sourceContainmentTimeout = prev }
}
