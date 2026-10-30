package doccorpus

import (
	"context"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// StableCEMSchema identifies a read-only documentation projection through an
// exact cem/1.0 map. It is a new schema beside corvint-corpus-cem/1, which
// keeps refusing a stable map (CEM-V1-075).
const StableCEMSchema = "corvint-corpus-cem-stable/1"

// StableCEMInputs is the complete input binding the stable verifier reads.
// The repository is also where each documentation anchor is checked.
type StableCEMInputs struct {
	Repository, ExpectedBase, Target, ArtifactRoot string
}

var stableCEMLimits = []string{
	"citation supports the original span at the verified map's base only; generated claim meaning and adequacy are not established",
	"the projection is read-only: it offers no cem cite or other writer argv, and it never encodes, reduces or relabels the stable map",
	"an accepted stable map establishes change and reference integrity only; native authority, criterion adequacy and runner execution stay NOT_OBSERVED",
}

// StableCEMProjection projects documentation claims through a tagged,
// verified stable change view. The map is admitted only when the complete
// stable verifier accepts these exact bytes; the receipt then carries the full
// map digest and spec, the unmodified verifier result (its references, limits
// and NOT_OBSERVED axes) and each claim's original immutable span at the
// verified base. A refusal returns the code corpus-stable-refused and a
// receipt that keeps the complete verifier result and no claim row, so a wrong
// base or edited input never fabricates authority (CEM-V1-075, CEM-V1-076).
func StableCEMProjection(ctx context.Context, a *Artifact, data []byte, claimID string, inputs StableCEMInputs) (map[string]any, error) {
	// A map that does not declare exactly cem/1.0 still goes through the
	// verifier, which refuses it, so every refusal carries the full result.
	declared := "UNSUPPORTED"
	if wire.DeclaresStable(data) {
		declared = wire.StableSpec
	}
	result, _ := verify.Stable(ctx, data, verify.StableOptions{Repository: inputs.Repository, ExpectedBase: inputs.ExpectedBase,
		Target: inputs.Target, ArtifactRoot: inputs.ArtifactRoot})
	receipt := map[string]any{"schema": StableCEMSchema, "artifact_sha256": a.SHA256, "cem_spec": declared, "cem_sha256": Digest(data),
		"cem_verification": result, "authority": "generated-documentation", "native_authority": "NOT_OBSERVED", "mutates": false,
		"limitations": append([]string(nil), stableCEMLimits...)}
	refuse := func(message string) (map[string]any, error) {
		receipt["outcome"], receipt["claims"] = "REFUSED", []any{}
		if digest, err := hashValue(receipt); err == nil {
			receipt["sha256"] = digest
		}
		return receipt, &Error{Code: "corpus-stable-refused", Message: message}
	}
	if declared != wire.StableSpec {
		message := "the stable documentation projection reads only an exact " + wire.StableSpec + " map"
		// A retired cem/0.x map names the recovery line (CEM-V1-210).
		if _, err := wire.ParseStable(data); cemcode.CodeOf(err) == cemcode.UnsupportedSpec {
			message += "; " + err.Error()
		}
		return refuse(message)
	}
	if result.Outcome != "ACCEPT" || result.Accept == nil || !*result.Accept {
		code := ""
		if result.Code != nil {
			code = " with code " + *result.Code
		}
		return refuse("the stable verifier returned " + result.Outcome + " at stage " + result.Stage + code)
	}
	document, err := wire.ParseStable(data)
	if err != nil || result.MapSha256 == nil || *result.MapSha256 != Digest(data) {
		return refuse("the accepted stable result does not bind the exact map bytes")
	}
	rows, omitted, err := projectClaims(ctx, inputs.Repository, a, claimID, document.Change.BaseRevision, document.Change.Evidence)
	if err != nil {
		return nil, err
	}
	receipt["outcome"], receipt["base_revision"], receipt["target_revision"], receipt["claims"] = "PROJECTED", document.Change.BaseRevision, inputs.Target, rows
	// Reference counts state what the verified map names; each reference is
	// integrity-checked only, never task completion or criterion satisfaction.
	receipt["references"] = map[string]any{"criterion_bindings": len(document.Criteria), "runner_receipts": len(document.Receipts),
		"criterion_links": len(document.Links), "artifacts": len(document.Artifacts), "semantics": "REFERENCE_INTEGRITY_ONLY"}
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
