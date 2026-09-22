package frontier

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/lrf"
)

// Request is one Frontier invocation (CF-V0-001). It carries immutable byte
// copies rather than paths: CF-V0-026 keeps path acquisition outside the wire
// and the verifier, so a command wrapper reads files through its own hardened
// reader and hands the bytes here.
type Request struct {
	// CEMBytes and OCMBytes are the exact bounded raw copies that will be
	// verified and then digested into `inputs` (CF-V0-002, CF-V0-019).
	CEMBytes []byte
	OCMBytes []byte
	// ExpectedBase and Target are the two independent revision inputs. Neither
	// may be inferred: CF-V0-001 fails inferred revision authority.
	ExpectedBase string
	Target       string

	// The optional dynamic test bundle is the all-or-none CF-V0-003 tuple.
	Command     []byte
	Observation []byte
	JUnitReport []byte

	// Verifier is the shared OCM-consuming CEM 0.2 verifier whose native
	// precedence CF-V0-021 puts ahead of every later Frontier check.
	Verifier Verifier
	// TCQ recomputes Test Claim Qualification from the verified inputs. A
	// caller-supplied TCQ result is never accepted as authority or as a
	// shortcut (CF-V0-002), which is why this is a recomputer, not a document.
	TCQ TCQRecomputer
}

// VerifyRequest is what Frontier hands the shared verifier: the same bounded
// immutable byte copies and both independent revisions (CF-V0-002).
type VerifyRequest struct {
	CEMBytes     []byte
	OCMBytes     []byte
	ExpectedBase string
	Target       string
}

// VerifiedUniverse is the declared universe after one shared OCM-consuming
// verification call. Frontier never interleaves or reimplements the steps
// inside that call (CF-V0-021 step 5); it consumes the accepted result.
type VerifiedUniverse struct {
	CEM *wire.Map
	// Obligations retain the frozen intent requirement order, which is the
	// CF-V0-020 sort order for both intent item kinds.
	Obligations    []Obligation
	BaseRevision   string
	TargetRevision string
	// ObjectFormat is exactly `sha1` or `sha256` (CF-V0-006).
	ObjectFormat string
	PatchSHA256  string
	Intent       IntentScope
	// LRFRequest is the pure post-structural projection the verifier derived
	// from the same verified inputs and target Git objects. Frontier evaluates
	// it itself so the LRF result is recomputed, never accepted (CF-V0-002).
	LRFRequest lrf.Request
}

// IntentScope is the one exact pinned OCM intent scope (CF-V0-001).
type IntentScope struct {
	BlobOID    string
	Path       string
	Span       Span
	SpanSHA256 string
}

// Verifier is the seam onto the shared OCM-consuming CEM 0.2 verifier. It is
// an interface so the cascade's ordering and translation are testable without
// a repository, and so the Git-backed adapter can land separately without
// changing this package's contract.
type Verifier interface {
	Verify(ctx context.Context, request VerifyRequest) (VerifiedUniverse, error)
}

// Compute runs the CF-V0-021 validation cascade and returns one complete
// Frontier document plus its exact canonical bytes. Every failure is
// operational: it emits no frontier result, never an open item.
func Compute(ctx context.Context, request Request) (Document, []byte, error) {
	document, encoded, err := compute(ctx, request)
	if err != nil {
		return Document{}, nil, err
	}
	return document, encoded, nil
}

func compute(ctx context.Context, request Request) (Document, []byte, *Error) {
	// Stage 1: library call shape, immutable-byte types, and optional
	// test-bundle completeness.
	mode, err := validateCallShape(request)
	if err != nil {
		return Document{}, nil, err
	}
	// Stage 2: inherited raw byte ceilings, before any Git work.
	if err := validateRawCeilings(request); err != nil {
		return Document{}, nil, err
	}
	// Stage 3: canonical parsing in shared OCM-consuming order, then the
	// Frontier admitted-profile rejection. This runs before stage 4 so a CEM
	// 0.1 input fails `unsupported-frontier-context` even when a revision is
	// also missing.
	if err := validateAdmittedProfiles(request); err != nil {
		return Document{}, nil, err
	}
	// Stage 4: missing expected base, then missing target.
	if err := validateRevisions(request); err != nil {
		return Document{}, nil, err
	}
	if err := checkContext(ctx); err != nil {
		return Document{}, nil, err
	}
	// Stage 5: one shared OCM-consuming verification call.
	universe, verifyErr := request.Verifier.Verify(ctx, VerifyRequest{
		CEMBytes:     request.CEMBytes,
		OCMBytes:     request.OCMBytes,
		ExpectedBase: request.ExpectedBase,
		Target:       request.Target,
	})
	if verifyErr != nil {
		return Document{}, nil, translateVerifier(verifyErr)
	}
	if err := validateUniverse(universe); err != nil {
		return Document{}, nil, err
	}
	// Stage 6: LRF recomputation and complete normal result.
	lrfResult, err := recomputeLRF(ctx, universe)
	if err != nil {
		return Document{}, nil, err
	}
	// Stage 7: static or dynamic TCQ recomputation and complete normal result.
	tcqResult, err := recomputeTCQ(ctx, request, universe, mode)
	if err != nil {
		return Document{}, nil, err
	}
	// Stage 8: universe and item derivation.
	document, err := deriveDocument(request, universe, lrfResult, tcqResult, mode)
	if err != nil {
		return Document{}, nil, err
	}
	// Stage 9: Frontier bounds, canonical encoding, state invariant, and ID.
	return seal(document)
}

// validateCallShape checks the library boundary and the CF-V0-003 all-or-none
// dynamic tuple. Every partial combination is `invalid-frontier-input`.
func validateCallShape(request Request) (TestMode, *Error) {
	if request.Verifier == nil || request.TCQ == nil {
		return "", fail(CodeInvalidInput, "verifier and tcq recomputer are required")
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

// validateRawCeilings applies the inherited raw byte ceilings at cascade stage
// 2 and reports them with the upstream producers' own codes, because the
// ceiling is theirs and CF-V0-022 passes an admitted upstream code through
// unchanged.
func validateRawCeilings(request Request) *Error {
	if len(request.CEMBytes) > maxCEMRawBytes {
		return fail("map-unavailable", "cem raw bytes exceed the inherited ceiling")
	}
	if len(request.OCMBytes) > maxOCMRawBytes {
		return fail("map-too-large", "ocm raw bytes exceed the inherited ceiling")
	}
	return nil
}

// validateAdmittedProfiles is the Frontier-owned compatibility rejection. CEM
// 0.1, OCM bound to CEM 0.1, or an unknown profile is
// `unsupported-frontier-context` — never a silently narrowed result.
func validateAdmittedProfiles(request Request) *Error {
	if err := declaredProfile(request.CEMBytes, AdmittedCEMSpec); err != nil {
		return err
	}
	return validateOCMProfile(request.OCMBytes)
}

// declaredProfile reads only the top-level `spec` member. A document that does
// not parse at all is left to the shared verifier's native precedence, which
// CF-V0-021 puts ahead of every Frontier check; only a readable document
// declaring a profile outside the admitted set is Frontier's to reject.
func declaredProfile(data []byte, admitted string) *Error {
	parsed, err := wire.Parse(data)
	if err != nil || parsed.Kind != wire.KindObject || parsed.Obj == nil {
		return nil
	}
	return admittedMember(parsed, "spec", admitted)
}

func validateOCMProfile(data []byte) *Error {
	parsed, err := wire.Parse(data)
	if err != nil || parsed.Kind != wire.KindObject || parsed.Obj == nil {
		return nil
	}
	if err := admittedMember(parsed, "spec", AdmittedOCMSpec); err != nil {
		return err
	}
	// OCM binds the CEM it was produced against; an OCM bound to CEM 0.1 is
	// rejected here even though its own profile is admitted (CF-V0-001).
	bound, found := parsed.Obj.Get("cem")
	if !found || bound.Kind != wire.KindObject || bound.Obj == nil {
		return nil
	}
	return admittedMember(bound, "spec", AdmittedCEMSpec)
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

// validateRevisions is cascade stage 4. Both codes are the inherited ones, so
// a caller sees the same failure Frontier's upstream would have produced.
func validateRevisions(request Request) *Error {
	if request.ExpectedBase == "" {
		return fail("expected-base-required", "an independent expected base is required")
	}
	if request.Target == "" {
		return fail("target-required", "an independent caller target is required")
	}
	return nil
}

// validateUniverse checks the values Frontier itself binds into identity.
// `objectFormat` is exactly sha1|sha256 (CF-V0-006); an unadmitted value would
// otherwise silently produce a universe ID no independent consumer could
// reproduce.
func validateUniverse(universe VerifiedUniverse) *Error {
	if universe.CEM == nil {
		return fail(CodeInternalError, "verifier returned no cem map")
	}
	if universe.ObjectFormat != "sha1" && universe.ObjectFormat != "sha256" {
		return fail(CodeUnsupportedContext, "object format is not admitted")
	}
	return nil
}

// recomputeLRF runs cascade stage 6. A complete LRF bound document is a normal
// return value, not an error, and CF-V0-022 maps it to
// `frontier-resource-exhausted` rather than passing `relevance-bound-exceeded`
// through as an operational code.
func recomputeLRF(ctx context.Context, universe VerifiedUniverse) (lrf.Result, *Error) {
	if err := checkContext(ctx); err != nil {
		return lrf.Result{}, err
	}
	result, err := lrf.Evaluate(universe.LRFRequest)
	if err != nil {
		return lrf.Result{}, translateLRF(err)
	}
	if result.BoundExceeded() {
		return lrf.Result{}, fail(CodeResourceExhausted, "lrf returned an aggregate bound document")
	}
	return result, nil
}

// recomputeTCQ runs cascade stage 7. Both modes always bind the recomputed TCQ
// ID (CF-V0-003).
func recomputeTCQ(ctx context.Context, request Request, universe VerifiedUniverse, mode TestMode) (TCQResult, *Error) {
	if err := checkContext(ctx); err != nil {
		return TCQResult{}, err
	}
	result, err := request.TCQ.Recompute(TCQRequest{
		CEMBytes:       request.CEMBytes,
		OCMBytes:       request.OCMBytes,
		BaseRevision:   universe.BaseRevision,
		TargetRevision: universe.TargetRevision,
		Mode:           mode,
		Command:        request.Command,
		Observation:    request.Observation,
		JUnitReport:    request.JUnitReport,
	})
	if err != nil {
		return TCQResult{}, translateTCQ(err)
	}
	if result.ID == "" {
		return TCQResult{}, fail(CodeInternalError, "tcq returned no identity")
	}
	return result, nil
}

// deriveDocument runs cascade stage 8: the universe ID, the item projection,
// and the state law's input half.
func deriveDocument(request Request, universe VerifiedUniverse, lrfResult lrf.Result, tcqResult TCQResult, mode TestMode) (Document, *Error) {
	scope := Scope{
		BaseRevision:     universe.BaseRevision,
		IntentBlobOID:    universe.Intent.BlobOID,
		IntentPath:       universe.Intent.Path,
		IntentSpan:       universe.Intent.Span,
		IntentSpanSHA256: universe.Intent.SpanSHA256,
		ObjectFormat:     universe.ObjectFormat,
		PatchSHA256:      universe.PatchSHA256,
		TargetRevision:   universe.TargetRevision,
	}
	universeID, err := UniverseID(scope)
	if err != nil {
		return Document{}, fail(CodeNoncanonical, "universe identity is not derivable")
	}
	items, projectErr := projectItems(universeID, universe.CEM, universe.Obligations, lrfResult, tcqResult)
	if projectErr != nil {
		return Document{}, projectErr
	}
	state := StateOpen
	if len(items) == 0 {
		state = StateEmpty
	}
	return Document{
		FrontierState: state,
		Inputs: Inputs{
			CEMSHA256: digest(request.CEMBytes),
			LRFSHA256: digest(lrfResult.CanonicalBytes()),
			OCMSHA256: digest(request.OCMBytes),
			TCQID:     tcqResult.ID,
			TestMode:  mode,
		},
		Items:      items,
		Scope:      scope,
		UniverseID: universeID,
	}, nil
}

// digest binds exactly the bytes CF-V0-019 names: CEM and OCM hash their
// verified bounded raw copies, LRF hashes its complete canonical result bytes.
func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
