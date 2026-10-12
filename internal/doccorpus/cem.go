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
	rows, omitted, err := projectClaims(ctx, root, a, claimID, cem.BaseRevision, cem.Evidence, true)
	if err != nil {
		return nil, err
	}
	receipt := map[string]any{"schema": "corvint-corpus-cem/1", "artifact_sha256": a.SHA256, "cem_sha256": Digest(data), "base_revision": cem.BaseRevision, "claims": rows, "authority": "generated-documentation", "mutates": false}
	if a.Schema == SchemaV2 {
		receipt["omitted"] = omitted
	}
	digest, err := hashValue(receipt)
	if err != nil {
		return nil, err
	}
	receipt["sha256"] = digest
	return receipt, nil
}

// projectClaims checks each selected claim anchor at the map's base commit and
// binds it to the map's evidence by identity. cite adds the 0.x `cem cite`
// writer argv; a stable projection never offers a writer (CEM-V1-076).
func projectClaims(ctx context.Context, root string, a *Artifact, claimID, base string, evidence []wire.Evidence, cite bool) ([]any, int, error) {
	auth, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		return nil, 0, err
	}
	release := auth.BeginObjectSession()
	defer release()
	rows := []any{}
	omitted, rowBytes := 0, 0
	for _, claim := range a.Claims {
		if claimID != "" && claim.ID != claimID {
			continue
		}
		for _, anchor := range claim.Evidence.Anchors {
			entry, ok, err := auth.LookupTreeEntry(ctx, base, anchor.Path)
			if err != nil {
				return nil, 0, err
			}
			if !ok || entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") || entry.OID != anchor.Blob {
				return nil, 0, fail("documentation citation differs at CEM base")
			}
			raw, err := auth.BlobBytes(ctx, entry.OID)
			if err != nil {
				return nil, 0, err
			}
			excerpt, err := span(raw, anchor.Start, anchor.End)
			if err != nil {
				return nil, 0, err
			}
			if Digest(excerpt) != anchor.SpanSHA256 {
				return nil, 0, fail("CEM documentation span mismatch")
			}
			lines := bytes.SplitAfter(raw, []byte{'\n'})
			start := int64(0)
			for _, line := range lines[:anchor.Start-1] {
				start += int64(len(line))
			}
			extent := wire.Span{Start: start, End: start + int64(len(excerpt))}
			id := wire.EvidenceIdentity(anchor.Blob, anchor.Path, extent, anchor.SpanSHA256)
			bound := false
			for _, e := range evidence {
				if e.ID == id {
					bound = true
				}
			}
			row := map[string]any{"claim_id": claim.ID, "evidence_id": id, "bound_in_cem": bound, "source": anchor, "derivation": claim.Evidence.Derivation, "trust": claim.Evidence.Trust, "limitations": []string{"citation supports the original span only; generated claim meaning and adequacy are not established"}}
			if cite {
				row["cite"] = []string{"cem", "cite", "--evidence-path", anchor.Path, "--bytes", fmt.Sprintf("%d:%d", extent.Start, extent.End)}
			}
			if a.Schema == SchemaV2 {
				raw, err := Encode(row)
				if err != nil {
					return nil, 0, err
				}
				if len(rows) >= MaxResults || rowBytes+len(raw) > 1<<20 {
					omitted++
					continue
				}
				rowBytes += len(raw)
			}
			rows = append(rows, row)
		}
	}
	if len(rows) == 0 && omitted == 0 {
		return nil, 0, fail("no documentation claim found for CEM projection")
	}
	return rows, omitted, nil
}
