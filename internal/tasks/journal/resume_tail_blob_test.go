//go:build darwin || linux

package journal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestLGLV0009_TailRemovedBaseBlobKeepsTheResume classifies V1-2622 as not a
// bug. Evidence and pinned blobs are content-addressed (evidence/<sha256>,
// pinned/<sha256>.json), so a legitimate tail write can only remove a base
// blob, never give its path other bytes. A sealed base's re-hash skips a blob
// the walked tail removed, so the resume holds; a blob rewritten in place,
// which no write can produce, still declines it.
func TestLGLV0009_TailRemovedBaseBlobKeepsTheResume(t *testing.T) {
	for _, tc := range []struct {
		name    string
		removed bool
	}{{"tail-removed", true}, {"rewritten-in-place", false}} {
		t.Run(tc.name, func(t *testing.T) {
			repo, r, base, blob := precheckStore(t, 0)
			base.segments = base.segments[:len(base.segments)-1]
			res, _, err := r.AuditForResume(base, "")
			if err != nil || res.Mode != ModeWriter {
				t.Fatalf("resumed audit: %+v %v", res, err)
			}
			// The fixture is unsegmented: an empty binding stands in for the
			// sealed prefix, so the sealed-base blob checks are what runs.
			base.sealedBase = true
			head := *res.Head
			head.Segments = &snapshot.SegmentBinding{}
			res.Head = &head
			full := filepath.Join(repo.StateDir, filepath.FromSlash(blob))
			if tc.removed {
				l := res.chain.canonical[blob]
				l.seq, l.digest = wire.SizeOf(base.seq+1), nil
				res.chain.canonical[blob] = l
				if err := os.Remove(full); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(full, []byte("other bytes"), 0o600); err != nil {
				t.Fatal(err)
			}
			s := newNativeRead(r.Source.(Native))
			decline := base.held(s, res, profileLimits, true)
			if err := s.close(); err != nil {
				t.Fatal(err)
			}
			if tc.removed && decline != nil {
				t.Fatalf("a tail-removed base blob declined the resume: %v", decline)
			}
			if !tc.removed && decline == nil {
				t.Fatal("a base blob rewritten in place kept the resume")
			}
		})
	}
}
