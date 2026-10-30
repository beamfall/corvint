package intent

import "github.com/Beamfall/corvint/internal/tasks/wire"

// Publication values (TM-V0-007).
const (
	Published   = "PUBLISHED"
	Unpublished = "UNPUBLISHED"
	Diverged    = "DIVERGED"
)

// Publication derives the TM-V0-007 publication status of one intent file
// from three digests: the working-tree file, the HEAD blob and the journal's
// latest post digest for that path. It is a pure function; the journal and
// HEAD readers that supply the inputs arrive with TCP-02.
//
//	PUBLISHED   HEAD blob = file = journal post
//	UNPUBLISHED file = journal, HEAD differs (or no HEAD blob)
//	DIVERGED    file ≠ journal
func Publication(file, headBlob, journalPost wire.Digest) string {
	if file != journalPost {
		return Diverged
	}
	if headBlob == file {
		return Published
	}
	return Unpublished
}

// Divergence applies the TM-V0-007 guard: the file's current digest must
// equal the journal's latest post digest for the path, or the pre digest of
// a receipt that is still redo-pending. pendingPre is nil when no receipt is
// pending. It returns nil or INTENT_DIVERGED.
func Divergence(path string, file wire.Digest, latestPost wire.Digest, pendingPre *wire.Digest) error {
	if file == latestPost {
		return nil
	}
	if pendingPre != nil && file == *pendingPre {
		return nil
	}
	return wire.Errorf(wire.CodeIntentDiverged, path, "file digest %s is neither the journal's latest post digest nor a redo-pending pre digest", file)
}
