package frontier

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/wp3codec"
)

// The stable frontier view (V1-1304). No stable LRF or frontier profile is
// accepted (CEM-V1-126) and frontier/0 was deleted with the cem/0.x wire
// (CEM-V1-217), so a cem/1.0 change map yields this view: the stable
// verifier's ACCEPT over the exact map, the accepted map's unknown hunks, the
// verified OCM obligations carried field for field, and the recomputed tcq-stable/0 identity, under a frontier
// state that is always NOT_PRODUCED and never EMPTY or closed.
const (
	StableViewProfile = "frontier-stable-view/0"
	// StableFrontierState is the only state the view may carry.
	StableFrontierState = "NOT_PRODUCED"
	// StableViewExitCode is 1: the view is a valid result that closes nothing,
	// so it can never take the CF-V0-004 EMPTY exit 0.
	StableViewExitCode = 1
	stableTCQProfile   = "tcq-stable/0"
	stableVerification = "ACCEPT"
	stableSidecar      = "EXACT"
)

// StableViewRequest is one cem/1.0 invocation. The artifact root is the
// explicit directory holding the map's referenced artifacts and has no default.
type StableViewRequest struct {
	CEMBytes     []byte
	OCMBytes     []byte
	ExpectedBase string
	Target       string
	ArtifactRoot string

	Command     []byte
	Observation []byte
	JUnitReport []byte
}

// StableViewer is the repository-backed seam onto the shipped stable route
// (frontierrepo.RecomputeStable). It is an interface so the
// admission order and translation stay testable without a repository.
type StableViewer interface {
	StableView(ctx context.Context, request StableViewRequest) (StableView, error)
}

// StableView is the verified cem/1.0 upstream as the frontier command reports
// it. Every value comes from the accepted stable verification; nothing is
// derived from the caller beyond the bytes it supplied.
type StableView struct {
	FrontierState  string
	Spec           string
	Verification   string
	Sidecar        string
	MapSHA256      string
	PatchSHA256    string
	OCMSHA256      string
	BaseRevision   string
	TargetRevision string
	ObjectFormat   string
	TCQProfile     string
	TCQID          string
	TestMode       TestMode
	// UnknownHunks are the accepted map's hunks whose disposition is unknown,
	// in map order. A supported or mechanical hunk was accepted by the stable
	// verifier and is not listed; nothing listed here is closed.
	UnknownHunks []StableHunk
	Obligations  []Obligation
}

// StableHunk is one unknown hunk of the accepted map.
type StableHunk struct {
	ID     string
	Path   string
	Reason string
}

// ExitCode is always 1: NOT_PRODUCED is never EMPTY.
func (StableView) ExitCode() int { return StableViewExitCode }

// DeclaresStableCEM reports whether the bytes are a readable map declaring
// cem/1.0. It selects the route only; admission is the stable verifier's.
func DeclaresStableCEM(data []byte) bool { return wire.DeclaresStable(data) }

// ComputeStable admits a cem/1.0 map through the stable viewer and returns the
// view plus its canonical bytes. Its order mirrors the CF-V0-021 cascade: call
// shape, inherited raw ceilings, admitted profiles, revisions, then the one
// stable verification. Every failure is operational with a closed code and no
// partial view; a refused map or a wrong base never yields a narrowed result.
func ComputeStable(ctx context.Context, viewer StableViewer, request StableViewRequest) (StableView, []byte, error) {
	view, encoded, err := computeStable(ctx, viewer, request)
	if err != nil {
		return StableView{}, nil, err
	}
	return view, encoded, nil
}

func computeStable(ctx context.Context, viewer StableViewer, request StableViewRequest) (StableView, []byte, *Error) {
	mode, err := validateStableCallShape(viewer, request)
	if err != nil {
		return StableView{}, nil, err
	}
	if err := validateRawCeilings(request); err != nil {
		return StableView{}, nil, err
	}
	if err := validateStableProfiles(request); err != nil {
		return StableView{}, nil, err
	}
	if err := validateRevisions(request); err != nil {
		return StableView{}, nil, err
	}
	if request.ArtifactRoot == "" {
		return StableView{}, nil, fail(CodeInvalidInput, "a cem/1.0 map requires an explicit artifact root")
	}
	if err := checkContext(ctx); err != nil {
		return StableView{}, nil, err
	}
	view, viewErr := viewer.StableView(ctx, request)
	if viewErr != nil {
		return StableView{}, nil, translateStable(viewErr)
	}
	if err := checkStableView(view, request, mode); err != nil {
		return StableView{}, nil, err
	}
	encoded, encodeErr := StableViewBytes(view)
	if encodeErr != nil {
		return StableView{}, nil, encodeErr
	}
	if len(encoded) > MaxOutputBytes {
		return StableView{}, nil, fail(CodeResourceExhausted, "output exceeds the frontier byte ceiling")
	}
	return view, encoded, nil
}

func validateStableCallShape(viewer StableViewer, request StableViewRequest) (TestMode, *Error) {
	if viewer == nil {
		return "", fail(CodeInvalidInput, "stable viewer is required")
	}
	if len(request.CEMBytes) == 0 || len(request.OCMBytes) == 0 {
		return "", fail(CodeInvalidInput, "cem and ocm bytes are required")
	}
	present := 0
	for _, part := range [][]byte{request.Command, request.Observation, request.JUnitReport} {
		if part != nil {
			present++
		}
	}
	switch present {
	case 0:
		return TestModeStatic, nil
	case 3:
		return TestModeDynamicCallerReported, nil
	}
	return "", fail(CodeInvalidInput, "dynamic test input is all-or-none")
}

// validateRawCeilings applies the inherited raw byte ceilings and reports them
// with the upstream producers' own codes (CF-V0-022 passes them through).
func validateRawCeilings(request StableViewRequest) *Error {
	if len(request.CEMBytes) > maxCEMRawBytes {
		return fail("map-unavailable", "cem raw bytes exceed the inherited ceiling")
	}
	if len(request.OCMBytes) > maxOCMRawBytes {
		return fail("map-too-large", "ocm raw bytes exceed the inherited ceiling")
	}
	return nil
}

// validateRevisions reports a missing base or target with the inherited codes.
func validateRevisions(request StableViewRequest) *Error {
	if request.ExpectedBase == "" {
		return fail("expected-base-required", "an independent expected base is required")
	}
	if request.Target == "" {
		return fail("target-required", "an independent caller target is required")
	}
	return nil
}

func admittedMember(object wire.Value, key, admitted string) *Error {
	member, found := object.Obj.Get(key)
	if !found || member.Kind != wire.KindString {
		return nil
	}
	if member.Str != admitted {
		return fail(CodeUnsupportedContext, "bound profile is not admitted")
	}
	return nil
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

var lowerHex64 = regexp.MustCompile(`^[0-9a-f]{64}$`)

// validateStableProfiles admits a map declaring exactly cem/1.0 and an OCM of
// the admitted profile bound to cem/1.0. Anything else is
// `unsupported-frontier-context`, never a reduced reading.
func validateStableProfiles(request StableViewRequest) *Error {
	if !wire.DeclaresStable(request.CEMBytes) {
		return fail(CodeUnsupportedContext, "stable route admits cem/1.0 only")
	}
	parsed, err := wire.Parse(request.OCMBytes)
	if err != nil || parsed.Kind != wire.KindObject || parsed.Obj == nil {
		return nil
	}
	if err := admittedMember(parsed, "spec", AdmittedOCMSpec); err != nil {
		return err
	}
	bound, found := parsed.Obj.Get("cem")
	if !found || bound.Kind != wire.KindObject || bound.Obj == nil {
		return nil
	}
	return admittedMember(bound, "spec", wire.StableSpec)
}

// stableVerifierCodes is the closed allowlist of public refusal codes the
// shipped stable verifier declares: every `code` and `issueCodes` member of
// protocol/cem-1.0/stable/RESULT.schema.json (a StableRefusal surfaces one of
// them), plus the stable verifier's own fallback and process codes. Any code
// outside it, or outside the frozen verifier and TCQ allowlists, is
// `frontier-internal-error` exactly as CF-V0-022 treats an unadmitted code.
// TestStableVerifierCodesCoverTheResultSchema pins it to the schema.
var stableVerifierCodes = closedSet(
	"artifact-changed-during-verification", "artifact-digest-mismatch", "artifact-resource-limit",
	"artifact-role-closure", "artifact-unavailable", "artifacts-required", "base-revision-mismatch",
	"diff-header", "diff-metadata-mismatch", "duplicate-file-group", "duplicate-hunk-id",
	"duplicate-new-path", "duplicate-old-path", "duplicate-reference", "empty-input",
	"evidence-drift", "evidence-unavailable", "excluded-artifact-mismatch", "excluded-path-not-file",
	"expected-base-required", "fabricated-evidence-id", "file-header", "git-diff-failed",
	"git-read-failed", "git-timeout", "hunk-header", "hunk-order", "hunk-payload", "hunk-range",
	"invalid-arguments", "invalid-criterion-identity", "invalid-excluded-path", "invalid-field",
	"invalid-json", "map-unavailable", "metadata", "metadata-repeat", "missing-field", "new-cursor",
	"newline-marker", "patch", "patch-context", "patch-digest-mismatch", "patch-hunk-count",
	"patch-top-level", "path", "process-cleanup-unobserved", "reference-coherence",
	"repository-object-unavailable", "short-hunk", "stable-verification-failed",
	"surplus-hunk-payload", "target-required", "too-many-lines", "unchanged-hunk", "unknown-field",
	"unproven-mechanical", "unresolved-reference", "unsupported-object-alternates",
	"unsupported-patch-inventory", "unsupported-process-containment",
	"unsupported-repository-envelope", "unsupported-resource-limit", "unsupported-spec",
	"unsupported-structural-runtime", "unsupported-tree-mode", "verification-timeout", "zero-hunks",
)

// translateStable maps one stable-route failure through closed allowlists.
func translateStable(err error) *Error {
	var owned *Error
	if errors.As(err, &owned) {
		return owned
	}
	if interrupted := translateInterruption(err); interrupted != nil {
		return interrupted
	}
	code := upstreamCode(err)
	switch {
	case code == "tcq-resource-exhausted":
		return fail(CodeResourceExhausted, "tcq resource exhaustion")
	case stableVerifierCodes[code], verifierCodes[code], tcqPassthroughCodes[code]:
		return fail(code, "inherited stable code")
	}
	return fail(CodeInternalError, "unadmitted stable outcome")
}

// checkStableView re-checks what the view claims against the request, so a
// viewer that returned a different map, OCM, mode or a non-ACCEPT binding can
// never surface as a stable view.
func checkStableView(view StableView, request StableViewRequest, mode TestMode) *Error {
	switch {
	case view.FrontierState != StableFrontierState, view.Spec != wire.StableSpec,
		view.Verification != stableVerification, view.Sidecar != stableSidecar,
		view.TCQProfile != stableTCQProfile, view.TestMode != mode:
		return fail(CodeInternalError, "stable view is not an accepted stable binding")
	case view.MapSHA256 != digest(request.CEMBytes), view.OCMSHA256 != digest(request.OCMBytes):
		return fail(CodeInternalError, "stable view binds other input bytes")
	case !lowerHex64.MatchString(view.PatchSHA256), !strings.HasPrefix(view.TCQID, "tcq-stable:sha256:"):
		return fail(CodeInternalError, "stable view digest is malformed")
	case view.ObjectFormat != "sha1" && view.ObjectFormat != "sha256":
		return fail(CodeUnsupportedContext, "object format is not admitted")
	}
	for _, hunk := range view.UnknownHunks {
		if hunk.ID == "" || hunk.Reason == "" {
			return fail(CodeInternalError, "stable view carries an incomplete unknown hunk")
		}
	}
	for _, obligation := range view.Obligations {
		if obligation.Disposition != OCMLinked && obligation.Disposition != OCMUnknown {
			return fail(CodeInternalError, "stable view carries an unadmitted obligation disposition")
		}
	}
	return nil
}

// StableViewBytes is the canonical view encoding plus exactly one LF.
func StableViewBytes(view StableView) ([]byte, *Error) {
	obligations := make([]wp3codec.Value, 0, len(view.Obligations))
	for _, obligation := range view.Obligations {
		obligations = append(obligations, wp3codec.Object(
			wp3codec.Member{Key: "claimIds", Value: wp3codec.Strings(nonNil(obligation.ClaimIDs))},
			wp3codec.Member{Key: "disposition", Value: wp3codec.String(obligation.Disposition)},
			wp3codec.Member{Key: "hunkIds", Value: wp3codec.Strings(nonNil(obligation.HunkIDs))},
			wp3codec.Member{Key: "id", Value: wp3codec.String(obligation.ID)},
			wp3codec.Member{Key: "reason", Value: wp3codec.String(obligation.Reason)},
		))
	}
	hunks := make([]wp3codec.Value, 0, len(view.UnknownHunks))
	for _, hunk := range view.UnknownHunks {
		hunks = append(hunks, wp3codec.Object(
			wp3codec.Member{Key: "id", Value: wp3codec.String(hunk.ID)},
			wp3codec.Member{Key: "path", Value: wp3codec.String(hunk.Path)},
			wp3codec.Member{Key: "reason", Value: wp3codec.String(hunk.Reason)},
		))
	}
	encoded, err := wp3codec.Encode(wp3codec.Object(
		wp3codec.Member{Key: "cem", Value: wp3codec.Object(
			wp3codec.Member{Key: "mapSha256", Value: wp3codec.String(view.MapSHA256)},
			wp3codec.Member{Key: "patchSha256", Value: wp3codec.String(view.PatchSHA256)},
			wp3codec.Member{Key: "sidecar", Value: wp3codec.String(view.Sidecar)},
			wp3codec.Member{Key: "spec", Value: wp3codec.String(view.Spec)},
			wp3codec.Member{Key: "verification", Value: wp3codec.String(view.Verification)},
		)},
		wp3codec.Member{Key: "frontierState", Value: wp3codec.String(view.FrontierState)},
		wp3codec.Member{Key: "obligations", Value: wp3codec.Array(obligations...)},
		wp3codec.Member{Key: "ocmSha256", Value: wp3codec.String(view.OCMSHA256)},
		wp3codec.Member{Key: "profile", Value: wp3codec.String(StableViewProfile)},
		wp3codec.Member{Key: "scope", Value: wp3codec.Object(
			wp3codec.Member{Key: "baseRevision", Value: wp3codec.String(view.BaseRevision)},
			wp3codec.Member{Key: "objectFormat", Value: wp3codec.String(view.ObjectFormat)},
			wp3codec.Member{Key: "targetRevision", Value: wp3codec.String(view.TargetRevision)},
		)},
		wp3codec.Member{Key: "tcq", Value: wp3codec.Object(
			wp3codec.Member{Key: "id", Value: wp3codec.String(view.TCQID)},
			wp3codec.Member{Key: "profile", Value: wp3codec.String(view.TCQProfile)},
			wp3codec.Member{Key: "testMode", Value: wp3codec.String(string(view.TestMode))},
		)},
		wp3codec.Member{Key: "unknownHunks", Value: wp3codec.Array(hunks...)},
	))
	if err != nil {
		return nil, fail(CodeNoncanonical, "stable view is not codec-encodable")
	}
	return append(encoded, '\n'), nil
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// RenderStableHuman renders the same view for a person, stating that no
// frontier was produced and that nothing it lists is closed.
func RenderStableHuman(view StableView) string {
	var out strings.Builder
	out.WriteString(StableViewProfile + " " + view.FrontierState + " cem=" + view.Spec +
		" verification=" + view.Verification + " sidecar=" + view.Sidecar + "\n")
	out.WriteString("tcq " + view.TCQProfile + " " + view.TCQID + " testMode=" + string(view.TestMode) + "\n")
	for _, hunk := range view.UnknownHunks {
		out.WriteString("unknown-hunk " + hunk.ID + " " + hunk.Path + " reason=" + hunk.Reason + "\n")
	}
	for _, obligation := range view.Obligations {
		line := "obligation " + obligation.ID + " " + obligation.Disposition
		if obligation.Reason != "" {
			line += " reason=" + obligation.Reason
		}
		out.WriteString(line + "\n")
	}
	out.WriteString(stableBoundary)
	return out.String()
}

const stableBoundary = "no frontier document was produced: frontier/0 is retired and no stable " +
	"LRF or frontier profile is accepted, so this view closes nothing and is never an empty frontier\n"
