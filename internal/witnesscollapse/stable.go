package witnesscollapse

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/Beamfall/corvint/internal/cem/cemcode"

	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// StableProfile identifies a collapse report over an exact cem/1.0 map. It is
// a new profile, so no experimental-v0 report byte is reinterpreted
// (CEM-V1-073).
const StableProfile = "corvint-correlated-witness-collapse/experimental-stable-v0"

// StableInputs is the complete input binding of one stable collapse: the same
// repository, expected base, target commit and artifact root the stable
// verifier reads.
type StableInputs struct {
	Repository, ExpectedBase, Target, ArtifactRoot string
}

// StableBinding links a stable collapse report to its full map and artifacts.
// Verification is the complete, unmodified stable verifier result: the full
// map digest, every original hunk with its caller coverage and discrimination
// witness states, every criterion, runner, link and artifact reference, the
// artifact checks and the NOT_OBSERVED axes. Collapsing a presentation never
// removes it (CEM-V1-073).
type StableBinding struct {
	Verification          verify.StableResult `json:"verification"`
	CriterionSatisfaction string              `json:"criterion_satisfaction"`
	Limits                []string            `json:"limits"`
}

var stableCollapseLimits = []string{
	"the collapse is a presentation over the accepted map; the original graph and witness evidence stay in verification",
	"caller coverage, discrimination and mutation witness states are caller metadata and are never criterion satisfaction",
	"an accepted stable map establishes change and reference integrity only; every NOT_OBSERVED verifier axis stays NOT_OBSERVED",
}

// CompileStable admits an exact cem/1.0 map only through the complete stable
// verifier, then collapses declared common causes over its private mechanics
// view. A refusal returns the code stable-verification-refused together with a
// report that carries only the full verifier result, so the refused evidence is
// retained and no collapse is produced from it (CEM-V1-074).
func CompileStable(ctx context.Context, rawCEM []byte, attributions []Attribution, inputs StableInputs) (Report, error) {
	if !wire.DeclaresStable(rawCEM) {
		// A retired cem/0.x map refuses with the closed code and its fix
		// line (CEM-V1-210); any other non-stable input stays not-stable.
		if _, err := wire.ParseStable(rawCEM); cemcode.CodeOf(err) == cemcode.UnsupportedSpec {
			return Report{}, &Error{Code: cemcode.UnsupportedSpec, Message: cemcode.MessageOf(err)}
		}
		return Report{}, failure("not-stable", "the map does not declare exactly %s", wire.StableSpec)
	}
	result, _ := verify.Stable(ctx, rawCEM, verify.StableOptions{Repository: inputs.Repository, ExpectedBase: inputs.ExpectedBase,
		Target: inputs.Target, ArtifactRoot: inputs.ArtifactRoot})
	binding := &StableBinding{Verification: result, CriterionSatisfaction: "NOT_OBSERVED",
		Limits: append([]string(nil), stableCollapseLimits...)}
	refused := Report{Profile: StableProfile, DeliveryStage: DeliveryStage, Claim: Claim, Assurance: Assurance, Stable: binding}
	if result.Outcome != "ACCEPT" || result.Accept == nil || !*result.Accept {
		code := ""
		if result.Code != nil {
			code = " with code " + *result.Code
		}
		return refused, failure("stable-verification-refused", "the stable verifier returned %s at stage %s%s", result.Outcome, result.Stage, code)
	}
	digest := sha256.Sum256(rawCEM)
	document, err := wire.ParseStable(rawCEM)
	if err != nil || result.MapSha256 == nil || *result.MapSha256 != hex.EncodeToString(digest[:]) {
		return refused, failure("stable-verification-refused", "the accepted stable result does not bind the exact map bytes")
	}
	// The mechanics view supplies evidence and basis relations to the
	// collapse only; it is never serialized or handed to a legacy writer.
	change := wire.Map(document.Change)
	report, err := compileDocument(rawCEM, &change, attributions)
	if err != nil {
		return Report{}, err
	}
	report.Profile, report.CEM.Spec, report.Stable = StableProfile, wire.StableSpec, binding
	return report, nil
}

// VerifyStable recompiles a stable report from the same exact bytes and
// inputs and requires structural equality. Evidence drift, a changed artifact
// or an unsupported runtime refuses instead of yielding an equivalent report.
func VerifyStable(ctx context.Context, rawCEM []byte, report Report, inputs StableInputs) error {
	if report.Profile != StableProfile {
		return failure("not-stable", "the report does not carry %s", StableProfile)
	}
	attributions := make([]Attribution, 0)
	for _, cause := range report.Causes {
		for _, member := range cause.Members {
			attributions = append(attributions, Attribution{
				WitnessEvidenceID: member.WitnessEvidenceID, Kind: cause.Kind,
				CauseSubjectSHA256:    cause.SubjectSHA256,
				AttestationEvidenceID: member.AttestationEvidenceID,
			})
		}
	}
	recompiled, err := CompileStable(ctx, rawCEM, attributions, inputs)
	if err != nil {
		return err
	}
	// The verifier result holds interface-typed values that a decoded report
	// carries as generic JSON, so equality is over the key-sorted JSON value of
	// each report, not over Go types.
	got, err := stableJSONValue(report)
	if err != nil {
		return err
	}
	want, err := stableJSONValue(recompiled)
	if err != nil {
		return err
	}
	if !bytes.Equal(got, want) {
		return failure("report-mismatch", "report differs from deterministic recompilation")
	}
	return nil
}

func stableJSONValue(report Report) ([]byte, error) {
	document, _, err := Canonical(report)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, failure("report-encode-failed", "report cannot be decoded")
	}
	normalized, err := json.Marshal(value)
	if err != nil {
		return nil, failure("report-encode-failed", "report cannot be encoded")
	}
	return normalized, nil
}
