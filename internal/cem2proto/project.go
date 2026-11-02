package cem2proto

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// Blob is one tree entry and its bytes at the base revision.
type Blob struct {
	Mode, OID string
	Data      []byte
}

// Source supplies the Git-derived facts a projection needs. Blobs omits a
// path that does not resolve at the base.
type Source interface {
	Patch(ctx context.Context, base, target string) ([]byte, error)
	Blobs(ctx context.Context, base string, paths []string) (map[string]Blob, error)
}

// Project rebuilds the canonical cem/1.0 map that header and body describe
// for the externally supplied target (CEM2-V0-005). Every derived member is
// recomputed from Git; the body supplies only claims and spans. The result is
// an ordinary cem/1.0 map: its native verification remains authoritative.
func Project(ctx context.Context, src Source, target string, header, body []byte) ([]byte, error) {
	h, err := DecodeHeader(header)
	if err != nil {
		return nil, err
	}
	if Sha256Hex(body) != h.BodySha256 {
		return nil, fail(CodeBodySha256Mismatch, "body bytes do not match bodySha256")
	}
	b, err := DecodeBody(body)
	if err != nil {
		return nil, err
	}
	patchBytes, err := src.Patch(ctx, h.Base, target)
	if err != nil {
		return nil, err
	}
	if Sha256Hex(patchBytes) != h.PatchSha256 {
		return nil, fail(CodePatchSha256Mismatch, "canonical patch does not match patchSha256")
	}
	parsed, err := patch.Parse(patchBytes)
	if err != nil {
		return nil, err
	}
	if len(parsed.Hunks) != len(b.Hunks) {
		return nil, fail(CodeHunkCountMismatch, "patch has %d hunks; body has %d", len(parsed.Hunks), len(b.Hunks))
	}
	evidence, err := resolveEvidence(ctx, src, h.Base, b)
	if err != nil {
		return nil, err
	}
	m := &wire.StableMap{
		Change: wire.StableChange{
			Spec:         wire.StableSpec,
			BaseRevision: h.Base,
			PatchSha256:  h.PatchSha256,
			ExcludedPath: wire.ExcludedCEMPath,
			Evidence:     evidence,
		},
		Criteria:   []wire.StableCriterion{},
		Receipts:   []wire.StableReceipt{},
		Links:      []wire.StableLink{},
		Artifacts:  []wire.StableArtifact{},
		Provenance: b.Provenance,
	}
	if h.Role != wire.StableRoleChange {
		m.Role = h.Role
	}
	for i, ph := range parsed.Hunks {
		row := b.Hunks[i]
		reason := Reasons[row.Reason]
		hunk := wire.Hunk{ID: ph.ID, Path: ph.DisplayPath, OldRange: ph.OldRange, NewRange: ph.NewRange, Disposition: disposition(reason), Reason: reason, Basis: []wire.Basis{}}
		for _, basis := range row.Basis {
			hunk.Basis = append(hunk.Basis, wire.Basis{EvidenceID: evidence[basis.Evidence].ID, Relation: Relations[basis.Relation]})
		}
		m.Change.Hunks = append(m.Change.Hunks, hunk)
	}
	if m.Change.Hunks == nil {
		m.Change.Hunks = []wire.Hunk{}
	}
	out, err := wire.EncodeStable(m)
	if err != nil {
		// CEM2-V0-024: the expanded map is held to the cem/1.0 byte bound,
		// which EncodeStable enforces by reparsing its own output.
		if cemcode.CodeOf(err) == cemcode.MapUnavailable {
			return nil, fail(CodeBoundExceeded, "projection exceeds %d bytes: %v", wire.MaxMapBytes, err)
		}
		return nil, fail(CodeMalformed, "projection is not a valid cem/1.0 map: %v", err)
	}
	return out, nil
}

// resolveEvidence derives each blob OID, span digest and identity from the
// base tree, and compares pinned digests when a row carries them.
func resolveEvidence(ctx context.Context, src Source, base string, b Body) ([]wire.Evidence, error) {
	blobs, err := src.Blobs(ctx, base, b.Paths)
	if err != nil {
		return nil, err
	}
	out := make([]wire.Evidence, 0, len(b.Evidence))
	for _, row := range b.Evidence {
		path := b.Paths[row.Path]
		blob, ok := blobs[path]
		if !ok || (blob.Mode != "100644" && blob.Mode != "100755") {
			return nil, fail(CodeEvidenceUnresolved, "evidence path %q is not a regular blob at the base", path)
		}
		if row.Start < 0 || row.Start >= row.End || row.End > int64(len(blob.Data)) {
			return nil, fail(CodeEvidenceUnresolved, "evidence span [%d,%d) outside %q", row.Start, row.End, path)
		}
		digest := sha256.Sum256(blob.Data[row.Start:row.End])
		spanSha := hex.EncodeToString(digest[:])
		if row.Pinned() && (row.BlobOid != blob.OID || row.SpanSha256 != spanSha) {
			return nil, fail(CodePinnedMismatch, "pinned digests for %q differ from the base", path)
		}
		span := wire.Span{Start: row.Start, End: row.End}
		out = append(out, wire.Evidence{ID: wire.EvidenceIdentity(blob.OID, path, span, spanSha), BlobOid: blob.OID, Path: path, Span: span, SpanSha256: spanSha})
	}
	return out, nil
}

// Equal reports whether a projection reproduced the original bytes.
func Equal(original, projected []byte) bool { return bytes.Equal(original, projected) }
