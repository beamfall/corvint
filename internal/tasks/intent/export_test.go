package intent

// SetPinTreeDirsForTest switches TreeDigest between pinned subdirectory reads
// and per-file InRoot reads, returning the restore.
func SetPinTreeDirsForTest(on bool) func() {
	old := pinTreeDirs
	pinTreeDirs = on
	return func() { pinTreeDirs = old }
}
