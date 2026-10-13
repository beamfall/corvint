package tcq

import (
	"regexp"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// The stable TCQ route (CEM-V1-120..CEM-V1-124). The tcq/0 route over cem/0.2
// is retired (CEM-V1-217); a cem/1.0 map is evaluated only under this distinct
// profile and identity domain, so no stable result can ever be read as, or
// collide with, a historical tcq/0 document.
const (
	StableProfile = "tcq-stable/0"
	StableCEMSpec = wire.StableSpec

	stableTCQPrefix = "tcq-stable:sha256:"
	domainTCQStable = "corvint-tcq-stable/0"
)

var stableDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// StableBinding is what the shipped stable verifier accepted for one
// invocation: the resolved revisions plus the exact map and patch digests.
type StableBinding struct {
	BaseRevision   string
	TargetRevision string
	MapSha256      string
	PatchSha256    string
}

// StableUpstreamVerifier admits a cem/1.0 map and its OCM only through the
// stable verifier's ACCEPT with an explicit artifact root. Any refusal is
// returned as-is and TCQ emits nothing (CEM-V1-121).
type StableUpstreamVerifier interface {
	VerifyStableOCM(cemRaw, ocmRaw []byte, expectedBase, target string) (StableBinding, error)
}

// EvaluateStable computes one tcq-stable/0 result. It runs the same TCQ-V0-042
// stages the retired tcq/0 route ran over the same bounded copies, except that
// stage 3 admits cem/1.0 only and stage 7 is the stable verifier. Claims, units, issues and
// relations are computed by the unchanged tcq/0 machinery; only the envelope
// differs: the stable profile, the bound cem/1.0 map and patch digests, and an
// identity hashed in the stable domain (CEM-V1-122).
func EvaluateStable(repository Repository, verifier StableUpstreamVerifier, request Request) (Result, error) {
	inputs, err := copyInputs(request)
	if err != nil {
		return Result{}, err
	}
	document, err := parseArtifactsWith(inputs, checkStableCEMProfile)
	if err != nil {
		return Result{}, err
	}
	if request.ExpectedBase == "" {
		return Result{}, fail(CodeExpectedBaseRequired)
	}
	if request.Target == "" {
		return Result{}, fail(CodeTargetRequired)
	}
	binding, err := verifier.VerifyStableOCM(inputs.cem, inputs.ocm, request.ExpectedBase, request.Target)
	if err != nil {
		return Result{}, err
	}
	if binding.MapSha256 != sha256Hex(inputs.cem) || !stableDigestPattern.MatchString(binding.PatchSha256) {
		return Result{}, fail(CodeInvalidTCQ)
	}
	if err := checkStableOCMBinding(inputs.ocm, binding); err != nil {
		return Result{}, err
	}
	resolved := Resolved{BaseRevision: binding.BaseRevision, TargetRevision: binding.TargetRevision}
	if err := bindRevisions(document, resolved); err != nil {
		return Result{}, err
	}
	dynamic, err := verifyDynamic(repository, inputs, document, resolved)
	if err != nil {
		return Result{}, err
	}
	frozen, err := assemble(repository, document, dynamic, resolved, nil)
	if err != nil {
		return Result{}, err
	}
	return stableResult(frozen, binding, inputs.expected)
}

// checkStableCEMProfile is the stable stage-3 admission: only a bounded
// object declaring exactly cem/1.0. A 0.x map is never admitted here and a
// cem/1.0 map is never admitted by checkCEMProfile.
func checkStableCEMProfile(raw []byte) error {
	if err := preflightJSON(raw, cemBounds); err != nil {
		return err
	}
	if !wire.DeclaresStable(raw) {
		return fail(CodeUnsupportedCEMProfile)
	}
	return nil
}

// checkStableOCMBinding requires the OCM to bind the exact accepted map and
// patch digests, independently of the verifier seam that already checked it.
func checkStableOCMBinding(ocmRaw []byte, binding StableBinding) error {
	value, err := wire.Parse(ocmRaw)
	if err != nil || value.Kind != wire.KindObject {
		return fail(CodeInvalidTCQ)
	}
	cem, ok := value.Obj.Get("cem")
	if !ok || cem.Kind != wire.KindObject {
		return fail(CodeInvalidTCQ)
	}
	mapDigest, okMap := cem.Obj.Get("mapSha256")
	patchDigest, okPatch := cem.Obj.Get("patchSha256")
	if !okMap || !okPatch || mapDigest.Kind != wire.KindString || patchDigest.Kind != wire.KindString ||
		mapDigest.Str != binding.MapSha256 || patchDigest.Str != binding.PatchSha256 {
		return fail(CodeInvalidTCQ)
	}
	return nil
}

// stableResult re-envelopes the frozen tcq/0 computation under tcq-stable/0.
// The intermediate tcq/0 bytes are never returned.
func stableResult(frozen Result, binding StableBinding, expected []byte) (Result, error) {
	value, err := wire.Parse(frozen.raw)
	if err != nil || value.Kind != wire.KindObject {
		return Result{}, fail(CodeInvalidTCQ)
	}
	body := withoutMember(value, "id")
	body.Obj.Values["profile"] = jsonString(StableProfile)
	body.Obj.Keys = append(body.Obj.Keys, "cem")
	body.Obj.Values["cem"] = jsonObject(
		member{"mapSha256", jsonString(binding.MapSha256)},
		member{"patchSha256", jsonString(binding.PatchSha256)},
		member{"spec", jsonString(StableCEMSpec)},
		member{"verification", jsonString("ACCEPT")},
	)
	identity := stableTCQPrefix + domainHash(domainTCQStable, canonicalValue(body))
	body.Obj.Keys = append(body.Obj.Keys, "id")
	body.Obj.Values["id"] = jsonString(identity)
	raw := canonicalJSON(body)
	if len(raw) > maxResultBytes {
		return Result{}, fail(CodeResourceExhausted)
	}
	if expected != nil && string(expected) != string(raw) {
		return Result{}, fail(CodeInvalidTCQ)
	}
	return Result{raw: raw, id: identity, claims: frozen.claims, environment: frozen.environment}, nil
}
