package journal

import "sync/atomic"

// SetSegmentPrefetchForTest turns the LGL-V0-813 read-ahead through a
// segment view on or off and returns the restore. Production code never
// calls it; it lets a store test compare the prefetched sealed walk with the
// sequential one over a real segmented store. The switch is process-wide, so
// a caller must not run in parallel with other tests.
func SetSegmentPrefetchForTest(on bool) (restore func()) {
	old := segmentPrefetch.Swap(on)
	return func() { segmentPrefetch.Store(old) }
}

// segmentLoadFault, when set, runs before a segment view reads a segment and
// fails the load with its error; only SetSegmentLoadFaultForTest sets it.
var segmentLoadFault atomic.Pointer[func(name string) error]

// SetSegmentLoadFaultForTest makes every segment load first call fault,
// failing the load with any error it returns (an injected EIO), and returns
// the restore. Production code never calls it. The fault is process-wide,
// so a caller must not run in parallel with other tests.
func SetSegmentLoadFaultForTest(fault func(name string) error) (restore func()) {
	old := segmentLoadFault.Swap(&fault)
	return func() { segmentLoadFault.Store(old) }
}
