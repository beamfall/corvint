package wire

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// HunkDigestProfile prefixes the bytes HunkDigest hashes.
const HunkDigestProfile = "cem-hunk-state/1"

// HunkDigest is the expected-old digest a guarded replacement of one hunk's
// citations binds to: the map's base revision and patch digest, the hunk ID,
// its disposition and reason, and its basis in order. Evidence IDs are content
// identities, so the digest changes whenever any earlier citation of the hunk,
// or the base or patch it was cited against, changes (CEM-V1-694).
func HunkDigest(baseRevision, patchSha256, hunkID, disposition, reason string, basis []Basis) string {
	var text strings.Builder
	for _, field := range []string{HunkDigestProfile, baseRevision, patchSha256, hunkID, disposition, reason} {
		text.WriteString(field)
		text.WriteByte('\n')
	}
	for _, entry := range basis {
		text.WriteString(entry.EvidenceID)
		text.WriteByte('\t')
		text.WriteString(entry.Relation)
		text.WriteByte('\n')
	}
	digest := sha256.Sum256([]byte(text.String()))
	return hex.EncodeToString(digest[:])
}

// DocumentHunkDigest is HunkDigest of the hunk at index of document.
func DocumentHunkDigest(document *Map, index int) string {
	hunk := document.Hunks[index]
	return HunkDigest(document.BaseRevision, document.PatchSha256, hunk.ID, hunk.Disposition, hunk.Reason, hunk.Basis)
}
