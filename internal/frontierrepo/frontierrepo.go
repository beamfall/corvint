// Package frontierrepo binds the Change Frontier V0 seams to real Git
// authority. `internal/frontier` holds `frontier.Verifier` and
// `frontier.TCQRecomputer` as interfaces so its cascade ordering and its
// CF-V0-022 translation stay testable without a repository; this package is
// the one implementation of both over the verified machinery that already
// exists in `internal/lrfrepo`, `internal/cem/...` and `internal/tcq`.
//
// Per CF-V0-027 nothing here may be wired into the agent harness, and per
// CF-V0-026 path acquisition stays outside: an Adapter is handed artifact
// BYTES and a repository root, never artifact paths.
package frontierrepo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/Beamfall/corvint/internal/frontier"
	"github.com/Beamfall/corvint/internal/lrfrepo"
	"github.com/Beamfall/corvint/internal/tcq"
)

// Adapter is one repository root bound to every Frontier V0 seam. A single
// value satisfies `frontier.Verifier`, `frontier.TCQRecomputer` and
// `tcq.UpstreamVerifier`, which is what lets one invocation make exactly ONE
// shared OCM-consuming verification call (CF-V0-002, CF-V0-021 step 5): the
// call Frontier makes at stage 5 is the same call TCQ consumes at stage 7.
//
// The context is held on the value because two of the three seams — TCQ's
// upstream verifier and TCQ's bounded tree reader — are context-free
// interfaces owned by their producer, and widening them here would change a
// frozen upstream contract to suit a consumer.
type Adapter struct {
	ctx  context.Context
	root string
	// shared records the one accepted verification of this invocation. It is
	// a memo, never a bypass: a call with different bytes or different
	// revisions misses it and runs a real verification.
	shared *sharedOutcome
	// pythonClaims is the edge-local Python grammar evidence produced by the
	// verification call TCQ consumed. It is reset for every recomputation.
	pythonClaims []lrfrepo.PythonClaimEvidence
}

// New binds an adapter to one repository root. It performs no Git work: the
// repository boundary is opened inside the shared verification call, where
// CF-V0-021 step 5 puts it.
func New(ctx context.Context, root string) *Adapter {
	return &Adapter{ctx: ctx, root: root}
}

// sharedOutcome is the accepted result of this invocation's single shared
// verification call, keyed by the exact artifact bytes and revisions that
// produced it.
type sharedOutcome struct {
	cemSHA256    string
	ocmSHA256    string
	expectedBase string
	target       string
	resolved     tcq.Resolved
	pythonClaims []lrfrepo.PythonClaimEvidence
}

// Verify is cascade stage 5: one shared OCM-consuming verification call whose
// native order is neither interleaved nor reimplemented here. Every ordering
// decision — repository boundary and alternate denial, expected-base
// resolution and equality, caller-target resolution and equality, base-side
// historical-sidecar check, target-side CEM artifact binding, canonical patch
// derivation and exclusion, then the remaining CEM/OCM map verification —
// lives inside lrfrepo.VerifyUniverse, which runs the sequence this repository
// already ships. Native CEM/OCM precedence therefore wins over every later
// Frontier check, exactly as CF-V0-021 requires.
func (adapter *Adapter) Verify(ctx context.Context, request frontier.VerifyRequest) (frontier.VerifiedUniverse, error) {
	universe, err := lrfrepo.VerifyUniverse(ctx, adapter.root,
		request.CEMBytes, request.OCMBytes, request.ExpectedBase, request.Target)
	if err != nil {
		return frontier.VerifiedUniverse{}, translate(err)
	}
	adapter.shared = &sharedOutcome{
		cemSHA256:    digest(request.CEMBytes),
		ocmSHA256:    digest(request.OCMBytes),
		expectedBase: request.ExpectedBase,
		target:       request.Target,
		resolved: tcq.Resolved{
			BaseRevision:   universe.BaseRevision,
			TargetRevision: universe.TargetRevision,
		},
		pythonClaims: append([]lrfrepo.PythonClaimEvidence(nil), universe.PythonClaims...),
	}
	return frontier.VerifiedUniverse{
		CEM:            universe.CEM,
		Obligations:    obligations(universe.Obligations),
		BaseRevision:   universe.BaseRevision,
		TargetRevision: universe.TargetRevision,
		ObjectFormat:   universe.ObjectFormat,
		PatchSHA256:    universe.PatchSHA256,
		Intent: frontier.IntentScope{
			BlobOID:    universe.Intent.BlobOID,
			Path:       universe.Intent.Path,
			Span:       frontier.Span{Start: universe.Intent.Start, End: universe.Intent.End},
			SpanSHA256: universe.Intent.SpanSHA256,
		},
		LRFRequest: universe.LRFRequest,
	}, nil
}

// obligations carries the verified OCM rows across the seam field for field.
// Nothing is filtered: CF-V0-011 and CF-V0-012 both need the unknown rows, and
// dropping one here would silently remove an obligation from the frontier.
func obligations(rows []lrfrepo.Obligation) []frontier.Obligation {
	projected := make([]frontier.Obligation, 0, len(rows))
	for _, row := range rows {
		projected = append(projected, frontier.Obligation{
			ID:          row.ID,
			Disposition: row.Disposition,
			Reason:      row.Reason,
			HunkIDs:     row.HunkIDs,
			ClaimIDs:    row.ClaimIDs,
		})
	}
	return projected
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
