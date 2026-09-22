package doccorpus

import (
	"bytes"
	"context"
	"fmt"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// CEMProjection preserves the immutable original span and carries corpus
// provenance beside the frozen wire. It never cites generated prose as intent.
func CEMProjection(ctx context.Context, root string, a *Artifact, data []byte, claimID string) (map[string]any, error) {
	cem, err := wire.ParseMap(data)
	if err != nil {
		return nil, err
	}
	auth, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		return nil, err
	}
	release := auth.BeginObjectSession()
	defer release()
	rows := []any{}
	for _, claim := range a.Claims {
		if claimID != "" && claim.ID != claimID {
			continue
		}
		for _, anchor := range claim.Evidence.Anchors {
			entry, ok, err := auth.LookupTreeEntry(ctx, cem.BaseRevision, anchor.Path)
			if err != nil {
				return nil, err
			}
			if !ok || entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") || entry.OID != anchor.Blob {
				return nil, fail("documentation citation differs at CEM base")
			}
			raw, err := auth.BlobBytes(ctx, entry.OID)
			if err != nil {
				return nil, err
			}
			excerpt, err := span(raw, anchor.Start, anchor.End)
			if err != nil {
				return nil, err
			}
			if Digest(excerpt) != anchor.SpanSHA256 {
				return nil, fail("CEM documentation span mismatch")
			}
			lines := bytes.SplitAfter(raw, []byte{'\n'})
			start := int64(0)
			for _, line := range lines[:anchor.Start-1] {
				start += int64(len(line))
			}
			extent := wire.Span{Start: start, End: start + int64(len(excerpt))}
			id := wire.EvidenceIdentity(anchor.Blob, anchor.Path, extent, anchor.SpanSHA256)
			bound := false
			for _, e := range cem.Evidence {
				if e.ID == id {
					bound = true
				}
			}
			rows = append(rows, map[string]any{"claim_id": claim.ID, "evidence_id": id, "bound_in_cem": bound, "source": anchor, "derivation": claim.Evidence.Derivation, "trust": claim.Evidence.Trust, "cite": []string{"cem", "cite", "--evidence-path", anchor.Path, "--bytes", fmt.Sprintf("%d:%d", extent.Start, extent.End)}, "limitations": []string{"citation supports the original span only; generated claim meaning and adequacy are not established"}})
		}
	}
	if len(rows) == 0 {
		return nil, fail("no documentation claim found for CEM projection")
	}
	receipt := map[string]any{"schema": "corvint-corpus-cem/1", "artifact_sha256": a.SHA256, "cem_sha256": Digest(data), "base_revision": cem.BaseRevision, "claims": rows, "authority": "generated-documentation", "mutates": false}
	digest, err := hashValue(receipt)
	if err != nil {
		return nil, err
	}
	receipt["sha256"] = digest
	return receipt, nil
}
