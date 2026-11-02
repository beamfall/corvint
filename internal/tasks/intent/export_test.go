package intent

import (
	"sort"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
)

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

// SetBeforeBoundedReadForTest observes every path readBounded is asked to
// read, returning the restore.
func SetBeforeBoundedReadForTest(hook func(path string)) func() {
	old := beforeBoundedRead
	beforeBoundedRead = hook
	return func() { beforeBoundedRead = old }
}

// SetReencode replaces the index admission's re-encoding until the returned
// function restores it (TCX-V0-845).
func SetReencode(f func(*ticket.Record) []byte) func() {
	saved := reencode
	reencode = f
	return func() { reencode = saved }
}

// IndexedTicketPaths lists the ticket paths an index holds, in byte order.
func IndexedTicketPaths(idx *ClosedIndex) []string {
	out := make([]string, 0, len(idx.entries))
	for rel := range idx.entries {
		out = append(out, rel)
	}
	sort.Strings(out)
	return out
}
