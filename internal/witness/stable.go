package witness

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/contextindex"
)

// StableProfile identifies a report whose witness source is an exact cem/1.0
// map. It is a new profile, so no corvint-witness/0 byte is reinterpreted
// (CEM-V1-072).
const StableProfile = "corvint-witness-stable/0"

// StableBindingProfile identifies the stable witness-source binding.
const StableBindingProfile = "corvint-witness-stable-binding/1"

// StableBinding records how an exact cem/1.0 map was admitted as a witness
// source. Verification is the complete stable verifier result, unmodified: its
// map digest, spec, sidecar, references and NOT_OBSERVED axes are retained
// whether the map was admitted or refused (CEM-V1-070, CEM-V1-071).
type StableBinding struct {
	Profile               string               `json:"profile"`
	Verification          *verify.StableResult `json:"verification"`
	CriterionSatisfaction string               `json:"criterionSatisfaction"`
	Limits                []string             `json:"limits"`
}

var stableWitnessLimits = []string{
	"an accepted stable map establishes change and reference integrity only; criterion satisfaction, runner execution and native authority stay NOT_OBSERVED",
	"caller-supplied coverage and discrimination witness states are retained in the verifier result as caller metadata and never close an obligation",
}

// readStableSource admits an exact cem/1.0 map only through the complete
// stable verifier over the same repository, base and head commit the report
// reads. Any outcome but ACCEPT keeps the full result and applies no citation.
func readStableSource(ctx context.Context, source Source, raw []byte, index *contextindex.Index, options Options) (Source, *wire.Map) {
	binding := &StableBinding{Profile: StableBindingProfile, CriterionSatisfaction: "NOT_OBSERVED",
		Limits: append([]string(nil), stableWitnessLimits...)}
	source.Stable = binding
	// ParseStable may return a complete document beside a graph error; its
	// counts are kept for the refusal, while admission below still requires
	// parseErr == nil.
	document, parseErr := wire.ParseStable(raw)
	if document != nil {
		source.Hunks = len(document.Change.Hunks)
		for _, hunk := range document.Change.Hunks {
			source.Basis += len(hunk.Basis)
		}
	}
	if options.StableArtifacts == "" {
		source.Status = "UNSUPPORTED"
		source.Detail = "the map declares " + wire.StableSpec + "; it is applied only through the stable verifier with an artifact root, so its citations are not applied"
		return source, nil
	}
	result, _ := verify.Stable(ctx, raw, verify.StableOptions{Repository: index.Root, ExpectedBase: options.Base,
		Target: index.CommitRevision, ArtifactRoot: options.StableArtifacts})
	binding.Verification = &result
	if result.Outcome != "ACCEPT" || result.Accept == nil || !*result.Accept {
		source.Status, source.Detail = stableRefusalStatus(result), "the stable verifier returned "+result.Outcome+" at stage "+result.Stage+stableCode(result)+"; its citations are not applied"
		return source, nil
	}
	digest := sha256.Sum256(raw)
	if parseErr != nil || result.MapSha256 == nil || *result.MapSha256 != hex.EncodeToString(digest[:]) {
		source.Status, source.Detail = "INVALID", "the accepted stable result does not bind the exact map bytes this report read; its citations are not applied"
		return source, nil
	}
	if document.Change.BaseRevision != options.Base {
		source.Status, source.Detail = "UNBOUND", "the map is bound to base "+document.Change.BaseRevision+", not the requested base; its citations are not applied"
		return source, nil
	}
	// The mechanics view is private to this report: it supplies exact basis
	// relations to attachWitnesses and is never serialized or handed to a
	// legacy writer.
	change := wire.Map(document.Change)
	source.Status = "BOUND"
	return source, &change
}

func stableRefusalStatus(result verify.StableResult) string {
	switch {
	case result.Outcome == "UNSUPPORTED":
		return "UNSUPPORTED"
	case result.Stage == "binding":
		return "UNBOUND"
	}
	return "INVALID"
}

func stableCode(result verify.StableResult) string {
	if result.Code == nil {
		return ""
	}
	return " with code " + *result.Code
}
