package affected

// SetGraphCacheHook stages a replacement race at a graph cache stage
// (AFP-V0-390, AFP-V0-392) and returns the restore.
func SetGraphCacheHook(hook func(stage string)) func() {
	saved := graphCacheHook
	graphCacheHook = hook
	return func() { graphCacheHook = saved }
}

// SetReadSource replaces the disk read behind Source.Read and returns the
// restore (AFP-V0-392).
func SetReadSource(read func(root, relative string) ([]byte, error)) func() {
	saved := readSourceAt
	readSourceAt = read
	return func() { readSourceAt = saved }
}
