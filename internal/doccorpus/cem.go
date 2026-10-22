package doccorpus

import (
	"bytes"
	"context"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// RefuseCEMProjection is all that is left of `docs corpus cem`: it projected a
// cem/0.x map only, and cem/0.x is retired (CEM-V1-210). A 0.x map refuses with
// the retired-spec fix, a cem/1.0 map with the stable operation (CEM-V1-163),
// and any other coded parse refusal keeps its code (V1-1305).
func RefuseCEMProjection(data []byte) error {
	if _, err := wire.ParseStable(data); err != nil {
		if code := cemcode.CodeOf(err); code != "" {
			return &Error{Code: code, Message: cemcode.MessageOf(err)}
		}
		return err
	}
	return &Error{Code: cemcode.UnsupportedSpec, Message: "docs corpus cem projected retired cem/0.x maps only; project a cem/1.0 map with `corvint docs corpus cem-stable`, which also requires --expected-base, --target and --stable-artifact-root"}
}

// projectClaims checks each selected claim anchor at the map's base commit and
// binds it to the map's evidence by identity. A stable projection never offers
// a writer (CEM-V1-076).
func projectClaims(ctx context.Context, root string, a *Artifact, claimID, base string, evidence []wire.Evidence) ([]any, int, error) {
	auth, err := gitauth.Open(root, newGitBudget(ctx))
	if err != nil {
		return nil, 0, err
	}
	release := auth.BeginObjectSession()
	defer release()
	// The selected anchors are resolved and read in batches (DCP-V1-216); the
	// loop below keeps its per-anchor refusals and order.
	paths := []string{}
	for _, claim := range a.Claims {
		if claimID == "" || claim.ID == claimID {
			for _, anchor := range claim.Evidence.Anchors {
				paths = append(paths, anchor.Path)
			}
		}
	}
	batched := newPathPrefetch(base)
	if err := batched.resolve(ctx, auth, paths); err != nil {
		return nil, 0, err
	}
	if err := batched.read(ctx, auth, batched.regularOIDs(paths), gitauth.MaxBlobBytes, maxPinnedInputBytes); err != nil {
		return nil, 0, err
	}
	rows := []any{}
	omitted, rowBytes := 0, 0
	for _, claim := range a.Claims {
		if claimID != "" && claim.ID != claimID {
			continue
		}
		for _, anchor := range claim.Evidence.Anchors {
			entry, ok, err := batched.lookup(ctx, auth, base, anchor.Path)
			if err != nil {
				return nil, 0, err
			}
			if !ok || entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") || entry.OID != anchor.Blob {
				return nil, 0, fail("documentation citation differs at CEM base")
			}
			raw, err := batched.blob(ctx, auth, entry.OID)
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
