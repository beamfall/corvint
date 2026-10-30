package extevidence

import (
	"context"

	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// StableObligationsSchema names the cem/1.0 obligation sidecar (CEM-V1-130).
// It is the named successor of the retired external-frontier-obligations/0
// sidecar (CEM-V1-217): a cem/1.0 map is projected only under this distinct
// schema and identity prefix.
const StableObligationsSchema = "external-frontier-obligations-stable/0"

const stableObligationsNote = "reference-only: each association proves identity, integrity, and freshness of an external reference; none states that an external record justifies the hunk; declared criterion links are listed apart from the associations and close no obligation; the stable verifier's accept binds the map, patch and artifacts only"

// StableObligationsOptions is the explicit stable verification context. None
// of it has a default: the verifier refuses a missing repository, base,
// target or artifact root.
type StableObligationsOptions struct {
	Repository, ExpectedBase, Target, ArtifactRoot string
	// AllowAbsentSidecar admits an accepted map that is not committed at the
	// target (sidecar ABSENT) and records that in binding.sidecar; MISMATCH
	// and UNSUPPORTED_KIND still refuse (CEM-V1-639).
	AllowAbsentSidecar bool
}

// StableObligationsRefusal is a stable verifier result that did not accept,
// kept whole as evidence; the sidecar is never produced beside it.
type StableObligationsRefusal struct {
	Result verify.StableResult
	Exit   int
}

func (e *StableObligationsRefusal) code() string {
	if e.Result.Code != nil {
		return *e.Result.Code
	}
	if len(e.Result.IssueCodes) > 0 {
		return e.Result.IssueCodes[0]
	}
	return "invalid-obligations-input"
}

func (e *StableObligationsRefusal) Error() string {
	return e.code() + ": stable verification " + e.Result.Outcome + " at stage " + e.Result.Stage
}

// Unwrap exposes the verifier code as a kernel error for the CLI envelope.
func (e *StableObligationsRefusal) Unwrap() error {
	return &gokernel.Error{Code: e.code(), Message: "stable verification " + e.Result.Outcome + " at stage " + e.Result.Stage}
}

// StableObligations composes the external-frontier-obligations-stable/0
// sidecar (CEM-V1-130..CEM-V1-134). In order it:
//
//  1. refuses a retired cem/0.x map with unsupported-spec, an invocation
//     without the stable context as the retired frozen route, and any other
//     bytes that do not declare cem/1.0;
//  2. runs the shipped stable verifier over those exact bytes with the
//     explicit repository, base, target and artifact root, and refuses with
//     the full result unless it accepted with the exact committed sidecar;
//  3. requires the accepted map digest to equal the raw digest, the map base
//     to equal the requested base and the accepted patch to equal the map's;
//  4. joins every stable hunk to the impact receipt exactly as the frozen
//     sidecar does, keeping every unknown association;
//  5. lists the map's declared criterion links in their own member, never as
//     associations and never as closure.
func StableObligations(ctx context.Context, cem, impact []byte, limit int, options StableObligationsOptions) (map[string]any, error) {
	if err := refuseRetiredCEM(cem); err != nil {
		return nil, err
	}
	if options.Repository == "" && options.ExpectedBase == "" && options.Target == "" && options.ArtifactRoot == "" {
		return nil, &gokernel.Error{Code: "invalid-obligations-input", Message: "external-frontier-obligations/0 is retired; a cem/1.0 map is read only by the stable sidecar (--repository, --expected-base, --target and --stable-artifact-root)"}
	}
	if !wire.DeclaresStable(cem) {
		return nil, &gokernel.Error{Code: "invalid-obligations-input", Message: "stable cem input is not a cem/1.0 document"}
	}
	result, exit := verify.Stable(ctx, cem, verify.StableOptions{
		Repository: options.Repository, ExpectedBase: options.ExpectedBase, Target: options.Target, ArtifactRoot: options.ArtifactRoot,
	})
	if exit != 0 || result.Accept == nil || !*result.Accept || result.Outcome != "ACCEPT" ||
		result.TargetRevision == nil || result.PatchSha256 == nil || result.MapSha256 == nil {
		return nil, &StableObligationsRefusal{Result: result, Exit: exit}
	}
	absent := options.AllowAbsentSidecar && result.Sidecar == "ABSENT"
	if result.Sidecar != "EXACT" && !absent {
		fix := ""
		if result.Sidecar == "ABSENT" {
			fix = "; fix: commit the map at " + wire.ExcludedCEMPath + " in the target, or pass --allow-absent-sidecar to bind the uncommitted map by digest"
		}
		return nil, &gokernel.Error{Code: "invalid-obligations-input", Message: "stable obligations require the exact committed sidecar at the target; sidecar is " + result.Sidecar + fix}
	}
	document, err := wire.ParseStable(cem)
	if err != nil || *result.MapSha256 != digest(cem) || document.Change.BaseRevision != options.ExpectedBase ||
		*result.PatchSha256 != document.Change.PatchSha256 {
		return nil, &gokernel.Error{Code: "invalid-obligations-input", Message: "the accepted stable result does not bind the exact map bytes, base and patch"}
	}
	joined, err := joinImpact(impact, document.Change.Hunks, limit)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"schema":       StableObligationsSchema,
		"authority":    Authority,
		"state":        joined.state,
		"state_reason": joined.reason,
		"binding": map[string]any{
			"cem_spec": wire.StableSpec, "cem_sha256": digest(cem), "patch_sha256": *result.PatchSha256,
			"base_revision": document.Change.BaseRevision, "target_revision": *result.TargetRevision,
			"excluded_path": wire.ExcludedCEMPath, "verification": result.Outcome,
			"impact_sha256": digest(impact), "impact_tool": "impact",
		},
		"providers":             joined.providers,
		"hunks":                 joined.hunks,
		"criteria":              stableCriterionLinks(document),
		"note":                  stableObligationsNote,
		"untrusted_text_fields": anyStrings(obligationsUntrusted),
	}
	// The member appears only for an admitted absent sidecar, so an exact
	// sidecar keeps its bytes and identity (CEM-V1-639).
	if absent {
		out["binding"].(map[string]any)["sidecar"] = "ABSENT"
	}
	encoded, err := gokernel.CanonicalJSON(out)
	if err != nil {
		return nil, err
	}
	out["id"] = "obligations-stable:sha256:" + digest(encoded)
	return out, nil
}

// stableCriterionLinks lists each declared criterion link by identity and
// the hunks, evidence and runner receipts it names. A link is a declaration
// the verifier checked for graph integrity; it states no satisfaction.
func stableCriterionLinks(document *wire.StableMap) []any {
	tickets := map[string]string{}
	for _, criterion := range document.Criteria {
		tickets[criterion.ID] = criterion.TicketID
	}
	out := make([]any, 0, len(document.Links))
	for _, link := range document.Links {
		out = append(out, map[string]any{
			"criterion": link.CriterionID, "ticket": tickets[link.CriterionID],
			"hunks": anyStrings(link.HunkIDs), "evidence": anyStrings(link.EvidenceIDs),
			"runner_receipts": anyStrings(link.RunnerReceiptSha256s), "satisfaction": "NOT_OBSERVED",
		})
	}
	return out
}
