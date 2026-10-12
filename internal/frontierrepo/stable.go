package frontierrepo

import (
	"context"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/frontier"
	"github.com/Beamfall/corvint/internal/lrfrepo"
	"github.com/Beamfall/corvint/internal/tcq"
)

// StableFrontierState is the frontier state of every stable upstream. frontier/0
// is frozen, admits cem/0.2 only and binds an lrf/0 digest, and no stable LRF
// or frontier profile is accepted, so the stable route produces no frontier
// document and claims no frontier state at all (CEM-V1-126). It is never an
// EMPTY or closed frontier.
const StableFrontierState = "NOT_PRODUCED"

// StableRequest is one explicit cem/1.0 upstream invocation. Every artifact is
// caller-supplied bytes; the artifact root is the explicit directory holding
// the map's referenced artifact files and has no default.
type StableRequest struct {
	CEMBytes     []byte
	OCMBytes     []byte
	ExpectedBase string
	Target       string
	ArtifactRoot string

	// The TCQ dynamic tuple, all-or-none exactly as in tcq.Request.
	Command     []byte
	Observation []byte
	JUnitReport []byte
}

// StableUpstream is the verified cem/1.0 closure-chain input: the stable
// OCM/LRF universe, its obligations carried field for field (unknown rows
// included), and a recomputed tcq-stable/0 result over the same accepted map.
type StableUpstream struct {
	Universe      *lrfrepo.StableUniverse
	Obligations   []frontier.Obligation
	TCQ           tcq.Result
	TCQClaims     []frontier.TCQClaimResult
	FrontierState string
}

// RecomputeStable is the C06 stable route (CEM-V1-125). In order it:
//
//  1. admits the map and OCM only through lrfrepo.VerifyStableUniverse, which
//     requires the shipped stable verifier's ACCEPT with the exact committed
//     sidecar, the explicit base, target and artifact root;
//  2. opens the target repository under the stable envelope for the TCQ phase;
//  3. recomputes TCQ under tcq-stable/0, whose upstream seam returns the
//     accepted binding only for exactly the bytes and revisions step 1 verified
//     and otherwise runs a real stable verification;
//  4. revalidates the retained boundary.
//
// Any refusal returns no upstream. The stable TCQ result is never rebound into
// a tcq/0 identity and no frontier/0 document is computed from it.
func RecomputeStable(ctx context.Context, root string, request StableRequest) (*StableUpstream, error) {
	options := lrfrepo.StableUniverseOptions{
		Root: root, ExpectedBase: request.ExpectedBase, Target: request.Target, ArtifactRoot: request.ArtifactRoot,
	}
	universe, err := lrfrepo.VerifyStableUniverse(ctx, request.CEMBytes, request.OCMBytes, options)
	if err != nil {
		return nil, translate(err)
	}
	repository, boundary, err := gitauth.OpenStable(root, gitrun.NewDefaultBudget(), nil)
	if err != nil {
		return nil, translate(err)
	}
	defer boundary.Close()
	if err := boundary.Validate(); err != nil {
		return nil, translate(err)
	}
	release := repository.BeginObjectSession()
	defer release()
	verifier := &stableVerifier{ctx: ctx, options: options, accepted: universe,
		cemSHA256: digest(request.CEMBytes), ocmSHA256: digest(request.OCMBytes)}
	reader := treeReader{adapter: &Adapter{ctx: ctx, root: root}, repository: repository}
	result, err := tcq.EvaluateStable(reader, verifier, tcq.Request{
		CEM:          request.CEMBytes,
		OCM:          request.OCMBytes,
		ExpectedBase: universe.BaseRevision,
		Target:       universe.TargetRevision,
		Command:      request.Command,
		Observation:  request.Observation,
		Report:       request.JUnitReport,
	})
	if err != nil {
		return nil, translate(err)
	}
	if err := boundary.Validate(); err != nil {
		return nil, translate(err)
	}
	return &StableUpstream{
		Universe:      universe,
		Obligations:   obligations(universe.Obligations),
		TCQ:           result,
		TCQClaims:     claims(result.Claims()),
		FrontierState: StableFrontierState,
	}, nil
}

// stableVerifier implements tcq.StableUpstreamVerifier over one accepted
// stable universe. Its memo hit requires the exact CEM and OCM bytes by digest
// and both revisions naming the accepted commits; anything else runs a real
// stable verification, so bytes the first call never saw cannot pass.
type stableVerifier struct {
	ctx                  context.Context
	options              lrfrepo.StableUniverseOptions
	accepted             *lrfrepo.StableUniverse
	cemSHA256, ocmSHA256 string
}

func (verifier *stableVerifier) VerifyStableOCM(cemRaw, ocmRaw []byte, expectedBase, target string) (tcq.StableBinding, error) {
	universe := verifier.accepted
	hit := verifier.cemSHA256 == digest(cemRaw) && verifier.ocmSHA256 == digest(ocmRaw) &&
		namesRevision(expectedBase, verifier.options.ExpectedBase, universe.BaseRevision) &&
		namesRevision(target, verifier.options.Target, universe.TargetRevision)
	if !hit {
		options := verifier.options
		options.ExpectedBase, options.Target = expectedBase, target
		fresh, err := lrfrepo.VerifyStableUniverse(verifier.ctx, cemRaw, ocmRaw, options)
		if err != nil {
			return tcq.StableBinding{}, err
		}
		universe = fresh
	}
	return tcq.StableBinding{
		BaseRevision:   universe.BaseRevision,
		TargetRevision: universe.TargetRevision,
		MapSha256:      universe.MapSHA256,
		PatchSha256:    universe.PatchSHA256,
	}, nil
}
