package companionrelease

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// Release-readiness refusal classes for one cem/1.0 stable result
// (docs/specs/cem-stable-v1.md CEM-V1-569..571, MIGRATION.json A02).
const (
	StableRefusalSchema    = "stable-result-schema"
	StableRefusalMode      = "stable-result-mode"
	StableRefusalAuthority = "stable-result-authority"
	StableRefusalAdmission = "stable-result-unsupported-admission"
	StableRefusalNotAccept = "stable-result-not-accepted"
	StableRefusalRunner    = "release-runner-incomplete"
	StableRefusalPlatform  = "release-platform-incomplete"
	StableRefusalExternal  = "release-external-incomplete"
)

// StableReleaseRefusal names the class that kept a stable result from
// release readiness.
type StableReleaseRefusal struct{ Class, Detail string }

func (r *StableReleaseRefusal) Error() string { return r.Class + ": " + r.Detail }

func stableRefuse(class, format string, args ...any) error {
	return &StableReleaseRefusal{Class: class, Detail: fmt.Sprintf(format, args...)}
}

// StableReleaseOutcomes is one class of release evidence that the stable
// result never carries: Required names every row the release needs and
// Observed maps a row to its observed outcome. Only PASSED is complete.
type StableReleaseOutcomes struct {
	Required []string
	Observed map[string]string
}

// StableReleaseEvidence is release readiness over one exact cem/1.0 stable
// verification result. Runner, platform and external outcomes are separately
// mandatory native release evidence: reference integrity, accept=true and the
// binary version never stand in for them.
type StableReleaseEvidence struct {
	Result                     []byte
	Runner, Platform, External StableReleaseOutcomes
}

// stableResultMembers is the exact normative member order of
// cem-stable-verification/1 (RESULT.schema.json required, CEM-V1-113).
var stableResultMembers = []string{"profile", "spec", "verificationMode", "outcome", "accept", "stage", "code", "issueCodes", "assurance", "expectedBase", "targetRevision", "mapSha256", "patchSha256", "sidecar", "drift", "hunks", "references", "artifactChecks", "citation", "axes", "runtime", "repositoryEnvelope", "verdictSha256"}

// stableAuthorityAxes stay NOT_OBSERVED in the reference operation; only
// authentication may move, under a consumer trusted-keys file (CEM-V1-084).
var stableAuthorityAxes = []string{"nativeAuthority", "historicalValidity", "currentApplicability", "sourceGitBinding", "sourceInventoryCompleteness", "executionAtCommit", "runnerReceiptSemantics", "runnerExecution", "authentication", "dependencyClosure", "criterionAdequacy", "criterionDiscrimination", "externalInteroperability"}

var stableIntegrityAxes = map[string][]string{
	"changeIntegrity":          {"NOT_CHECKED", "VERIFIED", "FAILED"},
	"referenceIntegrity":       {"NOT_CHECKED", "REFERENCE_INTEGRITY_ONLY", "EMPTY_REFERENCE_SET", "FAILED"},
	"structuralProof":          {"NOT_CHECKED", "NOT_REQUIRED", "PROVEN_DECLARED_FILE_PREDICATES", "FAILED", "UNSUPPORTED_RUNTIME"},
	"coverageValidation":       {"NOT_CHECKED", "ABSENT", "WIRE_VALIDATED_ONLY"},
	"discriminationValidation": {"NOT_CHECKED", "ABSENT", "WIRE_VALIDATED_ONLY"},
}

// ValidateStableReleaseEvidence reads e.Result as the exact cem/1.0 stable
// result and refuses release readiness unless the result is an admitted
// ACCEPT within its authority, runtime and envelope limits and every runner,
// platform and external row is observed PASSED. A refusal is a
// *StableReleaseRefusal.
func ValidateStableReleaseEvidence(e StableReleaseEvidence) error {
	if err := validateStableResult(e.Result); err != nil {
		return err
	}
	return completeReleaseOutcomes(e.Runner, e.Platform, e.External)
}

// completeReleaseOutcomes refuses the first runner, platform or external
// class with a required row that is not observed PASSED.
func completeReleaseOutcomes(runner, platform, external StableReleaseOutcomes) error {
	for _, class := range []struct {
		code string
		o    StableReleaseOutcomes
	}{{StableRefusalRunner, runner}, {StableRefusalPlatform, platform}, {StableRefusalExternal, external}} {
		if err := completeStableOutcomes(class.code, class.o); err != nil {
			return err
		}
	}
	return nil
}

// validateStableResult refuses raw unless it is an exact admitted stable
// ACCEPT within its authority, runtime and envelope limits.
func validateStableResult(raw []byte) error {
	view, err := readStableResult(raw)
	if err != nil {
		return err
	}
	if err := closedStableResult(view); err != nil {
		return err
	}
	if view.Profile != "cem-stable-verification/1" || view.Spec != wire.StableSpec || view.VerificationMode != "canonical-and-reference-integrity" {
		return stableRefuse(StableRefusalMode, "result is %s %s %s, not cem-stable-verification/1 %s canonical-and-reference-integrity", view.Profile, view.Spec, view.VerificationMode, wire.StableSpec)
	}
	for _, axis := range stableAuthorityAxes {
		value := view.Axes[axis]
		if value == "NOT_OBSERVED" || axis == "authentication" && (value == verify.AuthenticationUntrusted || value == verify.AuthenticationVerified || value == verify.AuthenticationFailed) {
			continue
		}
		return stableRefuse(StableRefusalAuthority, "axis %s is %q; the stable operation never observes it", axis, value)
	}
	if view.Outcome == "UNSUPPORTED" {
		return stableRefuse(StableRefusalAdmission, "result is UNSUPPORTED at stage %s", view.Stage)
	}
	switch view.Runtime.StructuralQualification {
	case "NOT_REQUIRED":
	case "QUALIFIED_EXACT_TUPLE":
		if !verify.StructuralTupleAdmitted(view.Runtime.GoVersion) {
			return stableRefuse(StableRefusalAdmission, "structural runtime %s is not an admitted QUALIFIED_EXACT_TUPLE", view.Runtime.GoVersion)
		}
	default:
		return stableRefuse(StableRefusalAdmission, "structural runtime %s is %s", view.Runtime.GoVersion, view.Runtime.StructuralQualification)
	}
	if view.RepositoryEnvelope != "canonical-repository-bounded/1" {
		return stableRefuse(StableRefusalAdmission, "repository envelope %q is not admitted", view.RepositoryEnvelope)
	}
	if view.Axes["structuralProof"] == "UNSUPPORTED_RUNTIME" {
		return stableRefuse(StableRefusalAdmission, "structural proof is UNSUPPORTED_RUNTIME")
	}
	if view.Outcome != "ACCEPT" || view.Accept == nil || !*view.Accept || view.Stage != "complete" || view.Code != nil || len(view.IssueCodes) != 0 || view.Assurance == nil || view.Axes["changeIntegrity"] != "VERIFIED" || view.Axes["authentication"] == verify.AuthenticationFailed {
		return stableRefuse(StableRefusalNotAccept, "result is %s at stage %s", view.Outcome, view.Stage)
	}
	switch view.Axes["referenceIntegrity"] {
	case "REFERENCE_INTEGRITY_ONLY", "EMPTY_REFERENCE_SET":
	default:
		return stableRefuse(StableRefusalNotAccept, "reference integrity is %s", view.Axes["referenceIntegrity"])
	}
	// An ACCEPT reached the repository and verification stages, so its sidecar
	// and structural proof must say so, and a qualified tuple is only ever
	// reported for a proven structural hunk.
	if view.Sidecar != "ABSENT" && view.Sidecar != "EXACT" {
		return stableRefuse(StableRefusalNotAccept, "an ACCEPT reports sidecar %s", view.Sidecar)
	}
	proof := view.Axes["structuralProof"]
	if proof != "NOT_REQUIRED" && proof != "PROVEN_DECLARED_FILE_PREDICATES" || view.Runtime.StructuralQualification == "QUALIFIED_EXACT_TUPLE" && proof != "PROVEN_DECLARED_FILE_PREDICATES" {
		return stableRefuse(StableRefusalNotAccept, "an ACCEPT reports structural proof %s under %s", proof, view.Runtime.StructuralQualification)
	}
	return nil
}

func completeStableOutcomes(class string, o StableReleaseOutcomes) error {
	if len(o.Required) == 0 {
		return stableRefuse(class, "no required rows are declared")
	}
	seen := map[string]bool{}
	for _, name := range o.Required {
		if name == "" || seen[name] {
			return stableRefuse(class, "required row %q is empty or repeated", name)
		}
		seen[name] = true
		outcome, ok := o.Observed[name]
		if !ok {
			return stableRefuse(class, "required row %s is not observed", name)
		}
		if outcome != "PASSED" {
			return stableRefuse(class, "required row %s is %s", name, outcome)
		}
	}
	return nil
}

// readStableResult enforces the closed cem-stable-verification/1 shape: the
// strict CEM JSON profile with signed result integers, the exact member names
// in normative order (CEM-V1-113), no unknown nested member, a typed reading
// that loses nothing, and a verdictSha256 that the supplied members recompute
// (CEM-V1-066).
func readStableResult(raw []byte) (verify.StableResult, error) {
	var view verify.StableResult
	raw = bytes.TrimSuffix(raw, []byte("\n"))
	if len(raw) == 0 || len(raw) > installedTotalOutputLimit || !utf8.Valid(raw) {
		return view, stableRefuse(StableRefusalSchema, "result exceeds UTF-8/size bounds")
	}
	parsed, err := wire.ParseSigned(raw)
	if err != nil {
		return view, stableRefuse(StableRefusalSchema, "%v", err)
	}
	if parsed.Kind != wire.KindObject {
		return view, stableRefuse(StableRefusalSchema, "result is not a JSON object")
	}
	names, err := topLevelNames(raw)
	if err != nil {
		return view, stableRefuse(StableRefusalSchema, "%v", err)
	}
	if fmt.Sprint(names) != fmt.Sprint(stableResultMembers) {
		return view, stableRefuse(StableRefusalSchema, "members %v differ from the normative cem-stable-verification/1 members", names)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&view); err != nil {
		return view, stableRefuse(StableRefusalSchema, "%v", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return view, stableRefuse(StableRefusalSchema, "%v", err)
	}
	// The verdict is recomputed from the supplied value, never from the typed
	// view: typed decoding reads null as 0 or "" and would rebuild the digest
	// of a different result.
	if stableVerdictDigest(parsed) != view.VerdictSha256 {
		return view, stableRefuse(StableRefusalSchema, "verdictSha256 does not bind the result")
	}
	type plainResult verify.StableResult
	typed, err := json.Marshal(plainResult(view))
	if err != nil {
		return view, stableRefuse(StableRefusalSchema, "%v", err)
	}
	reread, err := wire.ParseSigned(typed)
	if err != nil {
		return view, stableRefuse(StableRefusalSchema, "%v", err)
	}
	if !bytes.Equal(wire.CanonicalValue(withoutMembers(parsed, "verdictSha256")), wire.CanonicalValue(withoutMembers(reread, "verdictSha256"))) {
		return view, stableRefuse(StableRefusalSchema, "a member has a null or type the typed result cannot hold")
	}
	return view, nil
}

// stableVerdictDigest is the CEM-V1-066 verdict: the SHA-256 of the canonical
// JSON value of the result without runtime, repositoryEnvelope and
// verdictSha256.
func stableVerdictDigest(result wire.Value) string {
	sum := sha256.Sum256(wire.CanonicalValue(withoutMembers(result, "runtime", "repositoryEnvelope", "verdictSha256")))
	return hex.EncodeToString(sum[:])
}

func withoutMembers(object wire.Value, names ...string) wire.Value {
	out := &wire.Object{Values: map[string]wire.Value{}}
	for _, key := range object.Obj.Keys {
		drop := false
		for _, name := range names {
			drop = drop || key == name
		}
		if !drop {
			out.Keys = append(out.Keys, key)
			out.Values[key] = object.Obj.Values[key]
		}
	}
	return wire.Value{Kind: wire.KindObject, Obj: out}
}

// closedStableResult closes every enumeration release readiness reads.
func closedStableResult(v verify.StableResult) error {
	member := func(name, value string, allowed ...string) error {
		for _, a := range allowed {
			if value == a {
				return nil
			}
		}
		return stableRefuse(StableRefusalSchema, "%s %q is outside the closed set", name, value)
	}
	checks := []error{
		member("outcome", v.Outcome, "ACCEPT", "REJECT", "UNSUPPORTED"),
		member("stage", v.Stage, "arguments", "input", "wire", "references", "authority-arguments", "runtime", "repository", "binding", "verification", "drift", "artifacts", "complete"),
		member("sidecar", v.Sidecar, "NOT_CHECKED", "ABSENT", "EXACT", "MISMATCH", "UNSUPPORTED_KIND"),
		member("runtime.structuralQualification", v.Runtime.StructuralQualification, "NOT_REQUIRED", "EXPERIMENTAL_TUPLE_PENDING_FULL_CORPUS", "QUALIFIED_EXACT_TUPLE", "UNSUPPORTED"),
	}
	if v.Assurance != nil {
		checks = append(checks, member("assurance", *v.Assurance, "structural-only", "canonical"))
	}
	if v.IssueCodes == nil {
		checks = append(checks, stableRefuse(StableRefusalSchema, "issueCodes is not an array"))
	}
	if v.Runtime.GoVersion == "" || len(v.RepositoryEnvelope) == 0 || len(v.RepositoryEnvelope) > 128 {
		checks = append(checks, stableRefuse(StableRefusalSchema, "runtime goVersion or repositoryEnvelope is outside its bounds"))
	}
	references := make([]string, 0, len(v.References))
	for name, value := range v.References {
		references = append(references, name)
		if _, ok := value.([]any); !ok {
			checks = append(checks, stableRefuse(StableRefusalSchema, "references.%s is not an array", name))
		}
	}
	sort.Strings(references)
	if fmt.Sprint(references) != "[artifacts criterionBindings criterionLinks runnerReceipts]" {
		checks = append(checks, stableRefuse(StableRefusalSchema, "references %v differ from the four reference arrays", references))
	}
	axes := make([]string, 0, len(v.Axes))
	for name := range v.Axes {
		axes = append(axes, name)
	}
	sort.Strings(axes)
	want := append(append([]string(nil), stableAuthorityAxes...), "changeIntegrity", "referenceIntegrity", "structuralProof", "coverageValidation", "discriminationValidation")
	sort.Strings(want)
	if fmt.Sprint(axes) != fmt.Sprint(want) {
		checks = append(checks, stableRefuse(StableRefusalSchema, "axes %v differ from the closed axis set", axes))
	}
	for name, allowed := range stableIntegrityAxes {
		checks = append(checks, member("axes."+name, v.Axes[name], allowed...))
	}
	for _, err := range checks {
		if err != nil {
			return err
		}
	}
	return nil
}

func topLevelNames(raw []byte) ([]string, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return nil, fmt.Errorf("result is not a JSON object")
	}
	var names []string
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		name, _ := token.(string)
		names = append(names, name)
		var skip json.RawMessage
		if err := decoder.Decode(&skip); err != nil {
			return nil, err
		}
	}
	if _, err := decoder.Token(); err != nil && err != io.EOF {
		return nil, err
	}
	return names, nil
}
