package journal

// SetSegmentPrefetchForTest turns the LGL-V0-813 read-ahead through a
// segment view on or off and returns the restore. Production code never
// calls it; it lets a store test compare the prefetched sealed walk with the
// sequential one over a real segmented store.
func SetSegmentPrefetchForTest(on bool) (restore func()) {
	old := segmentPrefetch
	segmentPrefetch = on
	return func() { segmentPrefetch = old }
}
