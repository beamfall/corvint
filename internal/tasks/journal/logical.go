package journal

import (
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Logical returns src with the receipts and request afterimages sealed into
// segments readable by their logical paths (JSG-V0-012), for a reader that
// names one file instead of walking the journal. A receipts/ or requests/
// read src reports absent consults the head src holds; when that head binds
// a manifest, the segment it binds serves the path once the manifest's and
// the segment's digests check. A manifest the head named but a later seal
// replaced is a moved observation.
func Logical(src Source) Source { return &logicalSource{inner: src} }

type logicalSource struct {
	inner Source
	view  *segmentView
}

func (l *logicalSource) List(p string, max int) (Listing, error) { return l.inner.List(p, max) }

func (l *logicalSource) Read(p string, max int) ([]byte, error) {
	raw, err := l.inner.Read(p, max)
	if !os.IsNotExist(err) || !(strings.HasPrefix(p, "receipts/") || strings.HasPrefix(p, "requests/")) {
		return raw, err
	}
	if l.view == nil {
		view, ok, verr := l.open()
		if verr != nil {
			return nil, verr
		}
		if !ok {
			return nil, err
		}
		l.view = view
	}
	return l.view.Read(p, max)
}

// open reads the head and, when it is segmented, the manifest it binds.
func (l *logicalSource) open() (*segmentView, bool, error) {
	raw, err := l.inner.Read("head.json", wire.MaxJournalHeadBytes)
	if err != nil {
		return nil, false, err
	}
	head, err := snapshot.DecodeHead(raw)
	if err != nil {
		return nil, false, err
	}
	if head.Segments == nil {
		return nil, false, nil
	}
	name := snapshot.SegmentManifestName(head.Segments.Through.Uint64())
	m, err := l.inner.Read(name, snapshot.MaxSegmentManifestBytes)
	if os.IsNotExist(err) {
		return nil, false, wire.Errorf(wire.CodeSnapshotMoved, name, "a later seal replaced the manifest the head named")
	}
	if err != nil {
		return nil, false, err
	}
	if wire.Sum(m) != head.Segments.ManifestSha256 {
		return nil, false, wire.Errorf(wire.CodeSnapshotMoved, name, "segment manifest differs from the digest the head binds")
	}
	manifest, err := snapshot.DecodeSegmentManifest(m, 1)
	if err != nil {
		return nil, false, err
	}
	if len(manifest.Segments) == 0 || manifest.Through != head.Segments.Through.Uint64() || manifest.QueueID != head.QueueID {
		return nil, false, wire.Errorf(wire.CodeJournalForked, name, "segment manifest differs from the head")
	}
	return newSegmentView(l.inner, manifest), true, nil
}
