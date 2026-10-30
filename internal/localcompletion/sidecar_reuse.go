package localcompletion

import (
	"context"
	"path/filepath"
	"regexp"

	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// The closed refusal reasons of stable sidecar-only reuse (CEM-V1-207). Each
// leaves the check unqualified, so it runs again on the current target.
const (
	reuseContentChanged     = "sidecar-content-changed"
	reuseIDsNotFull         = "sidecar-ids-not-full"
	reuseSidecarUnavailable = "sidecar-unavailable"
	reuseRejected           = "sidecar-rejected"
	reuseUnverifiable       = "sidecar-unverifiable"
	reuseBindingMismatch    = "sidecar-binding-mismatch"
)

var fullObjectID = regexp.MustCompile(`^[0-9a-f]{40}$`)

// checkQualifies reports whether a check's latest observation qualifies on
// the current target; checkQualification also returns the closed sidecar-reuse
// refusal (CEM-V1-207), empty unless the plan allowed reuse and it was refused.
func (repo *repository) checkQualifies(ctx context.Context, saved *state, check Check, snap snapshot) bool {
	qualified, _ := repo.checkQualification(ctx, saved, check, snap)
	return qualified
}

// stableSidecarReuse decides whether a passing observation made on another
// target may stand for the current one (CEM-V1-205..208). The only admitted
// difference is a committed cem/1.0 sidecar that the stable verifier ACCEPTs
// over its exact bytes, with the absolute repository root as artifact root and
// the plan's base and the current target as full object IDs. It returns ""
// on a grant; otherwise a closed reason and, for a verifier result, its code.
func (repo *repository) stableSidecarReuse(ctx context.Context, saved *state, observed *observation, snap snapshot) (reason, code string) {
	if observed.ContentDigest != snap.content {
		return reuseContentChanged, ""
	}
	if !fullObjectID.MatchString(saved.Plan.Base) || !fullObjectID.MatchString(snap.target) {
		return reuseIDsNotFull, ""
	}
	mapPath := wire.ExcludedCEMPath
	if saved.Sealed != nil && snap.target == saved.Sealed.Binding.EvidenceTarget {
		mapPath = saved.Sealed.Binding.MapPath
	}
	raw, err := readFile(filepath.Join(repo.auth.Root, filepath.FromSlash(mapPath)), wire.MaxMapBytes)
	if err != nil {
		return reuseSidecarUnavailable, ""
	}
	result, exit := verify.Stable(ctx, raw, verify.StableOptions{Repository: repo.auth.Root, ExpectedBase: saved.Plan.Base, Target: snap.target, ArtifactRoot: repo.auth.Root})
	if result.Code != nil {
		code = *result.Code
	} else if len(result.IssueCodes) > 0 {
		code = result.IssueCodes[0]
	}
	accepted := exit == 0 && result.Outcome == "ACCEPT" && result.Accept != nil && *result.Accept
	if !accepted {
		if result.Outcome == "REJECT" {
			return reuseRejected, code
		}
		return reuseUnverifiable, code
	}
	// The verdict must name exactly these bytes, committed at this target, for
	// this base: the verifier proves the committed sidecar equals them (EXACT).
	if result.Sidecar != "EXACT" || result.MapSha256 == nil || *result.MapSha256 != digest(raw) ||
		result.ExpectedBase == nil || *result.ExpectedBase != saved.Plan.Base ||
		result.TargetRevision == nil || *result.TargetRevision != snap.target {
		return reuseBindingMismatch, ""
	}
	return "", ""
}
