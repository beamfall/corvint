package criterionexperiment

import (
	"context"
	"fmt"
	"github.com/Beamfall/corvint/internal/cem/cemcode"

	"github.com/Beamfall/corvint/internal/cem/verify"
	cw "github.com/Beamfall/corvint/internal/cem/wire"
)

// StableProfile is the explicit cem/1.0 input mode (CEM-V1-136). Its request,
// plan, receipt and summary carry this schema; the frozen Profile admitted
// cem/0.2 only, which is retired, so it now admits no map (CEM-V1-210), and a
// plan never changes mode between phases.
const StableProfile = "cem-criterion-experiments-stable/0"

// StableRefusal is a stable verifier result that did not accept, kept whole.
type StableRefusal struct {
	Result verify.StableResult
	Exit   int
}

func (e *StableRefusal) Error() string {
	code := "stable-verification-failed"
	if e.Result.Code != nil {
		code = *e.Result.Code
	} else if len(e.Result.IssueCodes) > 0 {
		code = e.Result.IssueCodes[0]
	}
	return "stable CEM " + e.Result.Outcome + " at stage " + e.Result.Stage + ": " + code
}

// PlanStableExperiment, RunStable and VerifyStable are the stable-mode
// phases. Each re-admits the plan's map through the stable verifier with the
// explicit artifact root; the root is an invocation input, never plan data.
func PlanStableExperiment(ctx context.Context, repoPath string, r Request, out, artifactRoot string, tasks TasksVerifierConfig) (Plan, error) {
	return planExperiment(ctx, repoPath, r, out, artifactRoot, tasks)
}

func RunStable(ctx context.Context, repoPath, planPath, approval, out, artifactRoot string, experimental, trusted bool, tasks TasksVerifierConfig) (Receipt, error) {
	return run(ctx, repoPath, planPath, approval, out, artifactRoot, experimental, trusted, tasks)
}

func VerifyStable(ctx context.Context, repoPath, planPath, receiptPath, artifactRoot string, live bool, tasks TasksVerifierConfig) (Summary, error) {
	return verifyExperiment(ctx, repoPath, planPath, receiptPath, artifactRoot, live, tasks)
}

// admitCEM selects the CEM admission by the request schema alone. The frozen
// profile admitted only cem/0.2, which is retired: a 0.x map refuses with the
// retired-spec fix and a cem/1.0 map names the stable profile (CEM-V1-210). The
// stable profile admits only through validateStableCEM.
func admitCEM(ctx context.Context, repoPath string, r Request, raw []byte, artifactRoot string) error {
	switch r.Schema {
	case Profile:
		if artifactRoot != "" {
			return fmt.Errorf("an artifact root is admitted only with %s", StableProfile)
		}
		if _, e := cw.ParseStable(raw); e != nil {
			return e
		}
		return cemcode.New(cemcode.UnsupportedSpec, "%s admitted only cem/0.2, which is retired; a cem/1.0 map is admitted by %s", Profile, StableProfile)
	case StableProfile:
		return validateStableCEM(ctx, repoPath, r, raw, artifactRoot)
	}
	return fmt.Errorf("unsupported request identity or bounds")
}

// validateStableCEM admits a cem/1.0 map only when the shipped stable
// verifier accepts the exact bytes at the request base and target with the
// explicit artifact root and the exact committed sidecar, and every criterion
// hunk is a hunk of that accepted change (CEM-V1-137). The map's declared
// criterion links are not read: criterion authority stays the native Tasks
// capture and verification.
func validateStableCEM(ctx context.Context, repoPath string, r Request, raw []byte, artifactRoot string) error {
	if !cw.DeclaresStable(raw) {
		// A retired cem/0.x map refuses with unsupported-spec and the
		// recovery line (CEM-V1-210).
		if _, e := cw.ParseStable(raw); e != nil {
			return e
		}
		return fmt.Errorf("only cem/1.0 admitted by %s", StableProfile)
	}
	result, exit := verify.Stable(ctx, raw, verify.StableOptions{Repository: repoPath, ExpectedBase: r.Base, Target: r.Target, ArtifactRoot: artifactRoot})
	if exit != 0 || result.Accept == nil || !*result.Accept || result.Outcome != "ACCEPT" ||
		result.TargetRevision == nil || result.PatchSha256 == nil || result.MapSha256 == nil {
		return &StableRefusal{Result: result, Exit: exit}
	}
	if result.Sidecar != "EXACT" {
		return fmt.Errorf("stable CEM requires the exact committed sidecar at the target")
	}
	d, e := cw.ParseStable(raw)
	if e != nil || *result.MapSha256 != Digest(raw) || d.Change.BaseRevision != r.Base || *result.TargetRevision != r.Target ||
		*result.PatchSha256 != d.Change.PatchSha256 {
		return fmt.Errorf("stable CEM not bound to the exact map, base, target and patch")
	}
	ids := map[string]bool{}
	for _, h := range d.Change.Hunks {
		ids[h.ID] = true
	}
	for _, c := range r.Criteria {
		for _, h := range c.Hunks {
			if !ids[h] {
				return fmt.Errorf("criterion references unknown CEM hunk")
			}
		}
	}
	return nil
}
