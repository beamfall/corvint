package store

// SetSourceContainmentScanBoundForTest lowers the SCC-V0-003 patch-id scan
// bound and returns the restore function.
func SetSourceContainmentScanBoundForTest(n int) func() {
	prev := sourceContainmentScanBound
	sourceContainmentScanBound = n
	return func() { sourceContainmentScanBound = prev }
}
