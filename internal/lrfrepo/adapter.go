// Package lrfrepo issues canonical LRF results from verified repository authority.
package lrfrepo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/publish"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/lrf"
)

const maxOCMBytes = 1 << 20

// Options are the repository-authority inputs accepted by corvint lrf.
type Options struct {
	CEMPath      string
	OCMPath      string
	PatchPath    string
	PatchGiven   bool
	ExpectedBase string
	Target       string
}

// Error is an adapter-owned structural or compatibility failure.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// CodeOf returns an LRF, CEM, or adapter error code.
func CodeOf(err error) string {
	if code := cemcode.CodeOf(err); code != "" {
		return code
	}
	for current := err; current != nil; {
		switch typed := current.(type) {
		case *Error:
			return typed.Code
		case *lrf.Error:
			return typed.Code
		case interface{ Unwrap() error }:
			current = typed.Unwrap()
		default:
			return ""
		}
	}
	return ""
}

func fail(code, format string, args ...any) error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Evaluate is the `corvint lrf` admission. The frozen lrf/0 projection was
// defined over cem/0.x maps, which are retired, and it evaluates no cem/1.0
// map, so Evaluate reads only the map bytes and refuses (CEM-V1-055, CEM-V1-210).
func Evaluate(ctx context.Context, root string, options Options) (lrf.Result, error) {
	if options.CEMPath == "" {
		return lrf.Result{}, fail(cemcode.InvalidArguments, "--cem is required")
	}
	inputRoot, err := publish.OpenRoot(root)
	if err != nil {
		return lrf.Result{}, err
	}
	cemRaw, err := readRootInput(inputRoot, options.CEMPath, wire.MaxMapBytes, cemcode.MapUnavailable)
	if err != nil {
		return lrf.Result{}, err
	}
	// CEM-V1-210: a retired cem/0.x map refuses with unsupported-spec and the
	// recovery line before any patch, OCM or repository input is read.
	if _, err := wire.ParseStable(cemRaw); err != nil {
		return lrf.Result{}, err
	}
	// CEM-V1-055: lrf/0 stays frozen and evaluates no cem/1.0 map; there is no
	// stable LRF result, so a stable map refuses explicitly.
	return lrf.Result{}, cemcode.New(cemcode.UnsupportedSpec, "lrf/0 evaluates no cem/1.0 map; the stable universe verification is its only admission")
}

type verifiedCEM struct {
	base        string
	target      string
	patch       []byte
	patchSource string
	excluded    *string
}

func readRootInput(root *publish.Root, requested string, bound int, code string) ([]byte, error) {
	path := requested
	if filepath.IsAbs(path) {
		resolved := filepath.Clean(path)
		relative, err := filepath.Rel(root.Path(), resolved)
		if err != nil || relative == "." || relative == ".." || filepath.IsAbs(relative) || len(relative) >= 3 && relative[:3] == ".."+string(filepath.Separator) {
			// CEM-V1-955: name the rule, the resolved path and the root.
			return nil, fail(cemcode.InvalidArguments, "input path must remain inside repository root: %s is outside the root %s; copy the file into the repository, or read a map outside it with `corvint cem verify --map ABSOLUTE_PATH`", resolved, root.Path())
		}
		path = filepath.ToSlash(relative)
	}
	if filepath.Clean(filepath.FromSlash(path)) != filepath.FromSlash(path) {
		return nil, fail(cemcode.InvalidArguments, "input path must be normalized")
	}
	return root.ReadBounded(path, bound, code)
}

func project(ctx context.Context, repository *gitauth.Repository, document *wire.Map, cemRaw []byte, verified *verifiedCEM, ocm *verifiedOCM) (lrf.Request, error) {
	parsed, err := patch.Parse(verified.patch)
	if err != nil {
		return lrf.Request{}, err
	}
	mapped := make(map[string]wire.Hunk, len(document.Hunks))
	for _, hunk := range document.Hunks {
		mapped[hunk.ID] = hunk
	}
	hunks := make([]lrf.Hunk, 0, len(parsed.Hunks))
	for ordinal, parsedHunk := range parsed.Hunks {
		mappedHunk := mapped[parsedHunk.ID]
		added := make([]byte, 0)
		for _, line := range parsedHunk.Body {
			if line.Prefix == '+' {
				added = append(added, line.NewPayload()...)
			}
		}
		hunks = append(hunks, lrf.Hunk{
			ID: parsedHunk.ID, OldPath: parsedHunk.OldPath, NewPath: parsedHunk.NewPath,
			Added: added, Disposition: mappedHunk.Disposition, Basis: mappedHunk.Basis, Ordinal: ordinal,
		})
	}
	evidence := make([]lrf.Evidence, 0, len(document.Evidence))
	for _, record := range document.Evidence {
		entry, exists, err := repository.LookupTreeEntry(ctx, verified.base, record.Path)
		if err != nil {
			return lrf.Request{}, err
		}
		if !exists || entry.OID != record.BlobOid || entry.Type != "blob" || entry.Mode != "100644" && entry.Mode != "100755" {
			return lrf.Request{}, cemcode.New(cemcode.EvidenceUnavailable, "verified evidence is no longer available")
		}
		blob, err := repository.BlobBytes(ctx, entry.OID)
		if err != nil {
			return lrf.Request{}, err
		}
		if record.Span.Start < 0 || record.Span.End > int64(len(blob)) || record.Span.Start >= record.Span.End {
			return lrf.Request{}, cemcode.New(cemcode.EvidenceUnavailable, "verified evidence span is unavailable")
		}
		evidence = append(evidence, lrf.Evidence{ID: record.ID, Path: record.Path, Span: append([]byte(nil), blob[record.Span.Start:record.Span.End]...)})
	}
	context := lrf.Context{
		CEMSpec: document.Spec, CEMMapSHA256: sha256Hex(cemRaw), PatchSource: verified.patchSource,
		BaseRevision: verified.base, PatchSHA256: sha256Hex(verified.patch), ExcludedPath: verified.excluded,
	}
	if verified.target != "" {
		context.TargetRevision = stringPointer(verified.target)
	}
	obligations := []lrf.Obligation(nil)
	if ocm != nil {
		context.OCMSpec = stringPointer(ocm.spec)
		context.OCMMapSHA256 = stringPointer(ocm.mapSHA256)
		context.IntentPath = stringPointer(ocm.intent.path)
		context.IntentBlobOID = stringPointer(ocm.intent.blobOID)
		context.IntentStart = intPointer(ocm.intent.start)
		context.IntentEnd = intPointer(ocm.intent.end)
		context.IntentSpanSHA256 = stringPointer(ocm.intent.spanSHA256)
		obligations = append(obligations, ocm.obligations...)
	}
	return lrf.Request{Context: context, Hunks: hunks, Evidence: evidence, Obligations: obligations}, nil
}

func sha256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func stringPointer(value string) *string { return &value }
func intPointer(value int64) *int64      { return &value }

// StableUniverseOptions are the explicit authority inputs of one stable
// universe verification. Every field is required: there is no default
// repository, revision or artifact root, and no Git-only fallback when the
// artifact root is missing (CEM-V1-052).
type StableUniverseOptions struct {
	// Root is the absolute, lexically clean repository root.
	Root         string
	ExpectedBase string
	Target       string
	// ArtifactRoot is the absolute directory holding the map's referenced
	// artifact bytes.
	ArtifactRoot string
}

// StableUniverse is one accepted read-only verified cem/1.0 change projection
// with its verified OCM obligations (CEM-V1-051). It is the C05 entry point
// that the later stable consumers (TCQ/frontier, obligations, dashboards) build
// on.
//
// It deliberately retains the complete stable identity: Spec is always
// cem/1.0, MapSHA256 is the digest of the exact original map bytes (never an
// encoder digest), and Verification is the full accepted stable verifier
// result including its artifact checks and NOT_OBSERVED authority axes. It
// never carries a legacy *wire.Map, so no 0.x writer or reduced serializer can
// consume it (CEM-V1-054).
type StableUniverse struct {
	Spec           string
	MapSHA256      string
	Verification   verify.StableResult
	BaseRevision   string
	TargetRevision string
	// ObjectFormat is the repository's own format, exactly "sha1" or "sha256".
	ObjectFormat string
	PatchSHA256  string
	OCMSHA256    string
	Intent       IntentScope
	// Obligations are every verified OCM row in OCM order; unknown rows stay
	// unknown with their reason (CEM-V1-053).
	Obligations []Obligation
	// LRFRequest is the stable projection admitted by lrf.ValidateStableRequest.
	// lrf/0 is frozen and lrf.Evaluate refuses it; a consumer needs its own
	// accepted stable LRF profile before it may evaluate this request.
	LRFRequest   lrf.Request
	PythonClaims []PythonClaimEvidence
	original     []byte
}

// OriginalBytes returns a copy of the exact authenticated stable map bytes.
func (u *StableUniverse) OriginalBytes() []byte { return append([]byte(nil), u.original...) }

// Document re-parses the exact authenticated bytes into a fresh full stable
// document, so a consumer that edits its copy cannot alter this universe.
func (u *StableUniverse) Document() (*wire.StableMap, error) { return wire.ParseStable(u.original) }

// StableRefusal is a stable verifier result that did not accept. It retains
// the full result and exit status as evidence; it is never an accept.
type StableRefusal struct {
	Result verify.StableResult
	Exit   int
}

// StableAdmissionRefusal is a refusal after the stable verifier accepted: an
// inexact sidecar or a failed OCM/LRF admission. It keeps the accepted
// verifier result as evidence of the step reached; it is never an accept
// (V1-2034). Its message is the admission error's, unchanged, so callers that
// print or trim it see the same text as before.
type StableAdmissionRefusal struct {
	Result verify.StableResult
	Err    error
}

func (e *StableAdmissionRefusal) Error() string {
	return e.Err.Error()
}

// Unwrap exposes the admission refusal's code to CodeOf.
func (e *StableAdmissionRefusal) Unwrap() error { return e.Err }

// Code is the verifier refusal code, or the first REJECT issue code.
func (e *StableRefusal) Code() string {
	if e.Result.Code != nil {
		return *e.Result.Code
	}
	if len(e.Result.IssueCodes) > 0 {
		return e.Result.IssueCodes[0]
	}
	return "stable-verification-failed"
}

func (e *StableRefusal) Error() string {
	return e.Code() + ": stable verification " + e.Result.Outcome + " at stage " + e.Result.Stage + e.limitSuffix()
}

// limitSuffix names the bound behind a resource-limit refusal and its value,
// or nothing (OCM-V0-026; V1-2170).
func (e *StableRefusal) limitSuffix() string {
	if e.Result.LimitCause == "" {
		return ""
	}
	return " (" + e.Result.LimitCause + ")"
}

// Unwrap exposes the refusal code to CodeOf as an adapter error.
func (e *StableRefusal) Unwrap() error {
	return &Error{Code: e.Code(), Message: "stable verification " + e.Result.Outcome + " at stage " + e.Result.Stage}
}

// VerifyStableUniverse is the explicit cem/1.0 OCM/LRF admission. In order it:
//
//  1. admits only a map that declares cem/1.0 (anything else is
//     unsupported-spec; a stable map is never reduced to a 0.x reading);
//  2. runs the complete stable verifier over the exact original bytes with
//     the explicit base, target and artifact root, and refuses with the full
//     result unless it accepted with the exact committed sidecar;
//  3. opens the repository under the stable envelope and retains that
//     boundary for the whole OCM phase;
//  4. re-derives the canonical patch at the accepted revisions and requires
//     its digest to equal the accepted stable patch digest;
//  5. verifies the OCM against the exact original map digest, the same target
//     and patch, the pinned intent scope, its claims and its obligations;
//  6. projects the stable LRF request and admits it through
//     lrf.ValidateStableRequest;
//  7. revalidates the retained boundary and runs the stable verifier again,
//     requiring a result identical to the first, so the repository and
//     artifacts it authenticated are still the ones the OCM phase read.
//
// Every refusal returns no universe, so nothing downstream can derive a closed
// or empty frontier from a stale, swapped or incomplete input (CEM-V1-056).
func VerifyStableUniverse(ctx context.Context, cemRaw, ocmRaw []byte, options StableUniverseOptions) (*StableUniverse, error) {
	if !wire.DeclaresStable(cemRaw) {
		// A retired cem/0.x map keeps the decoder's refusal and fix line
		// (CEM-V1-210).
		if _, err := wire.ParseStable(cemRaw); cemcode.CodeOf(err) == cemcode.UnsupportedSpec {
			return nil, err
		}
		return nil, cemcode.New(cemcode.UnsupportedSpec, "stable universe verification accepts cem/1.0 only")
	}
	result, err := acceptedStable(ctx, cemRaw, options)
	if err != nil {
		return nil, err
	}
	if result.Sidecar != "EXACT" {
		return nil, &StableAdmissionRefusal{Result: result, Err: cemcode.New(cemcode.ExcludedArtifactMismatch, "stable OCM/LRF admission requires the exact committed sidecar at the target; sidecar is %s", result.Sidecar)}
	}
	universe, err := stableUniversePhase(ctx, cemRaw, ocmRaw, options, result)
	if err != nil {
		return nil, &StableAdmissionRefusal{Result: result, Err: err}
	}
	again, err := acceptedStable(ctx, cemRaw, options)
	if err != nil {
		return nil, err
	}
	if !sameStableResult(result, again) {
		return nil, &StableAdmissionRefusal{Result: result, Err: cemcode.New(cemcode.RepositoryObjectUnavailable, "stable verification changed during OCM/LRF admission")}
	}
	return universe, nil
}

// acceptedStable runs the stable verifier and returns its result only when it
// accepted; any other result is a StableRefusal that keeps it whole. A map the
// cem/1.0 protocol's fixed 1,024 operations refuse as
// unsupported-resource-limit is verified once more under the opt-in scaled
// bound, and that result stands (DCW-V0-373).
func acceptedStable(ctx context.Context, cemRaw []byte, options StableUniverseOptions) (verify.StableResult, error) {
	stableOptions := verify.StableOptions{
		Repository: options.Root, ExpectedBase: options.ExpectedBase,
		Target: options.Target, ArtifactRoot: options.ArtifactRoot,
	}
	result, exit := verify.Stable(ctx, cemRaw, stableOptions)
	if result.Outcome == "UNSUPPORTED" && result.Code != nil && *result.Code == "unsupported-resource-limit" {
		stableOptions.ScaledOperations = true
		result, exit = verify.Stable(ctx, cemRaw, stableOptions)
	}
	if exit != 0 || result.Accept == nil || !*result.Accept || result.Outcome != "ACCEPT" ||
		result.TargetRevision == nil || result.PatchSha256 == nil || result.MapSha256 == nil {
		return result, &StableRefusal{Result: result, Exit: exit}
	}
	return result, nil
}

// afterStableUniversePhase runs between the OCM phase and the terminal
// revalidation. It is a test seam only and does nothing in production.
var afterStableUniversePhase = func() {}

// stableUniversePhase verifies the OCM and projects the request inside one
// retained stable repository boundary, which it revalidates before returning.
func stableUniversePhase(ctx context.Context, cemRaw, ocmRaw []byte, options StableUniverseOptions, result verify.StableResult) (*StableUniverse, error) {
	document, err := wire.ParseStable(cemRaw)
	if err != nil {
		return nil, err
	}
	base, target := document.Change.BaseRevision, *result.TargetRevision
	if base != options.ExpectedBase || *result.MapSha256 != document.StableOriginalDigest() {
		return nil, cemcode.New(cemcode.InvalidArguments, "stable verification binding differs from the requested base or map")
	}
	repository, boundary, err := gitauth.OpenStable(options.Root, gitrun.NewDefaultBudget(), nil)
	if err != nil {
		return nil, err
	}
	defer boundary.Close()
	if err := boundary.Validate(); err != nil {
		return nil, err
	}
	release := repository.BeginObjectSession()
	defer release()
	patchBytes, err := repository.CanonicalDiff(ctx, base, target)
	if err != nil {
		return nil, err
	}
	if sha256Hex(patchBytes) != *result.PatchSha256 || sha256Hex(patchBytes) != document.Change.PatchSha256 {
		return nil, cemcode.New(cemcode.PatchDigestMismatch, "re-derived stable patch does not match the accepted patchSha256")
	}
	// Private mechanics view for the shared OCM and projection steps; it is
	// never returned and its spec stays cem/1.0.
	change := wire.Map(document.Change)
	excluded := wire.ExcludedCEMPath
	verified := &verifiedCEM{base: base, target: target, patch: patchBytes, patchSource: "canonical-derived", excluded: &excluded}
	parsed, err := parseOCM(ocmRaw)
	if err != nil {
		return nil, err
	}
	ocm, pythonClaims, err := verifyUniverseOCM(ctx, repository, &change, cemRaw, verified, parsed, ocmRaw, Options{ExpectedBase: base, Target: target})
	if err != nil {
		return nil, err
	}
	request, err := project(ctx, repository, &change, cemRaw, verified, ocm)
	if err != nil {
		return nil, err
	}
	if err := lrf.ValidateStableRequest(request); err != nil {
		return nil, err
	}
	format, err := objectFormat(ctx, repository)
	if err != nil {
		return nil, err
	}
	afterStableUniversePhase()
	if err := boundary.Validate(); err != nil {
		return nil, err
	}
	return &StableUniverse{
		Spec:           document.Change.Spec,
		MapSHA256:      document.StableOriginalDigest(),
		Verification:   result,
		BaseRevision:   base,
		TargetRevision: target,
		ObjectFormat:   format,
		PatchSHA256:    sha256Hex(patchBytes),
		OCMSHA256:      ocm.mapSHA256,
		Intent:         intentScope(ocm.intent),
		Obligations:    obligationRows(parsed.obligations),
		LRFRequest:     request,
		PythonClaims:   pythonClaims,
		original:       document.OriginalBytes(),
	}, nil
}
