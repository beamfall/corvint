package intent

// SetPinTreeDirsForTest switches TreeDigest between pinned subdirectory reads
// and per-file InRoot reads, returning the restore.
func SetPinTreeDirsForTest(on bool) func() {
	old := pinTreeDirs
	pinTreeDirs = on
	return func() { pinTreeDirs = old }
}

// SetAfterTreeDirPinForTest runs hook once TreeDigest has pinned a store
// subdirectory, before it reads any record there.
func SetAfterTreeDirPinForTest(hook func(sub string)) func() {
	old := afterTreeDirPin
	afterTreeDirPin = hook
	return func() { afterTreeDirPin = old }
}

// SetBeforeTreeCaptureForTest runs hook when TreeDigest ends phase 1 and
// begins reading record contents.
func SetBeforeTreeCaptureForTest(hook func()) func() {
	old := beforeTreeCapture
	beforeTreeCapture = hook
	return func() { beforeTreeCapture = old }
}
