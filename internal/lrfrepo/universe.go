package lrfrepo

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/lrf"
)

// This file is the one exported byte-input entry point onto the OCM-consuming
// canonical verifier this package already owns. It exists because Change
// Frontier V0 must hand the SAME bounded immutable CEM and OCM byte copies and
// both independent revisions through ONE shared verification call (CF-V0-002),
// and CF-V0-026 keeps path acquisition outside that call — so a consumer that
// already holds the bytes cannot go through Evaluate, which reads them from
// repository-relative paths and then evaluates LRF itself.
//
// Nothing here re-orders or re-implements the verification: it is exactly the
// sequence Evaluate runs (canonical CEM verification, canonical patch
// derivation, OCM parse, OCM verification, LRF projection), with the two path
// reads replaced by caller-supplied bytes. CF-V0-021 step 5 requires that
// native order to stay intact, so the steps below stay in it.

// IntentScope is the one exact pinned OCM intent scope of a verified universe.
type IntentScope struct {
	Path       string
	BlobOID    string
	Start      int64
	End        int64
	SpanSHA256 string
}

// Obligation is one verified OCM obligation row in OCM order. The rows are
// surfaced alongside the LRF projection because a consumer of the intent
// closure needs the declared disposition and reason, which the LRF request
// deliberately does not carry.
type Obligation struct {
	ID          string
	Disposition string
	Reason      string
	HunkIDs     []string
	ClaimIDs    []string
}

// PythonClaimEvidence is one structurally re-derived Python claim. ClaimID
// identifies every selected edge that rests on the claim; Path and BlobOID pin
// its source, and AnchorProfile preserves the supported profile if frozen
// grammar rejection prevents TCQ from re-deriving it itself.
type PythonClaimEvidence struct {
	ClaimID       string
	Path          string
	BlobOID       string
	AnchorProfile string
	Reason        string
}

// Universe is one accepted verification: the declared universe as the shared
// verifier resolved it, plus the pure post-structural LRF projection derived
// from the same verified inputs and target Git objects. The LRF result itself
// is deliberately absent — a consumer recomputes it rather than accepting one.
type Universe struct {
	CEM            *wire.Map
	BaseRevision   string
	TargetRevision string
	// ObjectFormat is the repository's own format, exactly "sha1" or "sha256".
	ObjectFormat string
	PatchSHA256  string
	OCMSHA256    string
	Intent       IntentScope
	Obligations  []Obligation
	LRFRequest   lrf.Request
	PythonClaims []PythonClaimEvidence
}

// VerifyUniverse runs one shared OCM-consuming canonical verification over
// caller-supplied artifact bytes and returns the accepted universe.
//
// Both revisions are required and independent: an inferred revision authority
// is refused by the canonical verifier itself, which is why neither is
// defaulted here.
func VerifyUniverse(ctx context.Context, root string, cemRaw, ocmRaw []byte, expectedBase, target string) (*Universe, error) {
	repository, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		return nil, err
	}
	return VerifyUniverseWithRepository(ctx, repository, cemRaw, ocmRaw, expectedBase, target)
}

// VerifyUniverseWithRepository executes the same canonical verification using
// one caller-owned repository scope. It does not accept a precomputed result or
// an authority grant; the protected adapter supplies its separately budgeted
// immutable reader. Ordinary VerifyUniverse callers retain their uncached Open.
func VerifyUniverseWithRepository(ctx context.Context, repository *gitauth.Repository, cemRaw, ocmRaw []byte, expectedBase, target string) (*Universe, error) {
	if repository == nil {
		return nil, cemcode.New(cemcode.InvalidArguments, "universe repository is required")
	}
	defer repository.BeginObjectSession()()
	document, err := wire.ParseMap(cemRaw)
	if err != nil {
		return nil, err
	}
	if document.Spec != wire.Spec02 {
		return nil, cemcode.New(cemcode.UnsupportedSpec, "universe verification accepts canonical cem/0.2 only")
	}
	options := Options{ExpectedBase: expectedBase, Target: target}
	verified, err := verifyCanonicalCEM(ctx, repository, document, cemRaw, options)
	if err != nil {
		return nil, err
	}
	parsed, err := parseOCM(ocmRaw)
	if err != nil {
		return nil, err
	}
	ocm, pythonClaims, err := verifyUniverseOCM(ctx, repository, document, cemRaw, verified, parsed, ocmRaw, options)
	if err != nil {
		return nil, err
	}
	request, err := project(ctx, repository, document, cemRaw, verified, ocm)
	if err != nil {
		return nil, err
	}
	format, err := objectFormat(ctx, repository)
	if err != nil {
		return nil, err
	}
	return &Universe{
		CEM:            document,
		BaseRevision:   verified.base,
		TargetRevision: verified.target,
		ObjectFormat:   format,
		PatchSHA256:    sha256Hex(verified.patch),
		OCMSHA256:      ocm.mapSHA256,
		Intent:         intentScope(ocm.intent),
		Obligations:    obligationRows(parsed.obligations),
		LRFRequest:     request,
		PythonClaims:   pythonClaims,
	}, nil
}

// verifyUniverseOCM preserves the canonical verifier's binding, intent,
// claims, and obligation order while moving Python grammar uncertainty to the
// Frontier/TCQ edge that owns it (TCQ-V0-047). Unlike the lrf OCM leg, this
// surface must not turn one unsupported Python edge into a command refusal.
func verifyUniverseOCM(ctx context.Context, repository *gitauth.Repository, cem *wire.Map, cemRaw []byte, verified *verifiedCEM, document *ocmDocument, raw []byte, options Options) (*verifiedOCM, []PythonClaimEvidence, error) {
	if err := verifyOCMBinding(ctx, repository, cem, cemRaw, verified, document, options); err != nil {
		return nil, nil, err
	}
	if err := refuseDeclaredIntentForm(document); err != nil {
		return nil, nil, err
	}
	intentContext, err := verifyOCMIntent(ctx, repository, cem, document)
	if err != nil {
		return nil, nil, err
	}
	anchors, claimPaths, pythonClaims, err := verifyUniverseClaims(intentContext.reader, document.target, document.claims)
	if err != nil {
		return nil, nil, err
	}
	obligations, err := verifyObligations(document, cem, intentContext.requirements, intentContext.scope, intentContext.statements, anchors, claimPaths)
	if err != nil {
		return nil, nil, err
	}
	return &verifiedOCM{
		spec: document.spec, mapSHA256: sha256Hex(raw), intent: document.intent,
		obligations: obligations,
	}, pythonClaims, nil
}

// verifyUniverseClaims verifies claim identity and structural re-extraction but
// does not ask pythonsyntax.SourceSyntaxValid to stand in for python-ast/1.
// Each structurally valid Python claim carries source evidence so a later
// frozen-grammar rejection can retain its supported anchor profile and become
// an edge-local abstention.
func verifyUniverseClaims(reader *ocmBlobReader, target string, claims []ocmClaim) (map[string][]byte, map[string]string, []PythonClaimEvidence, error) {
	anchors := make(map[string][]byte, len(claims))
	paths := make(map[string]string, len(claims))
	blobs := newBlobIndexes()
	var pythonClaims []PythonClaimEvidence
	for _, claim := range claims {
		blob, err := reader.blob(target, claim.path, claim.blobOID)
		if err != nil {
			return nil, nil, nil, err
		}
		if claim.span.start < 0 || claim.span.end > int64(len(blob)) || claim.span.start >= claim.span.end {
			return nil, nil, nil, fail("claim-not-reextractable", "claim cannot be re-extracted at target")
		}
		anchor := blob[claim.span.start:claim.span.end]
		if sha256Hex(anchor) != claim.spanSHA256 || !claimShapeExtractableIn(claim, blob, blobs) {
			return nil, nil, nil, fail("claim-not-reextractable", "claim cannot be re-extracted at target")
		}
		anchors[claim.id] = append([]byte(nil), anchor...)
		paths[claim.id] = claim.path
		// Case-sensitive on purpose: internal/tcq.analyzeBlob dispatches to the
		// Python analyzer on a case-sensitive ".py" suffix, so a ".PY" path is
		// never given Python treatment and fails as unsupported-anchor-profile.
		// Matching case-insensitively here would relabel that as
		// unsupported-python-grammar -- naming a grammar failure that never
		// happened (AGENTS.md invariant 2). The dispatcher decides; this only
		// reports what it decided.
		if filepath.Ext(claim.path) == ".py" {
			pythonClaims = append(pythonClaims, PythonClaimEvidence{
				ClaimID: claim.id, Path: claim.path, BlobOID: claim.blobOID,
				AnchorProfile: pythonAnchorProfile(claim.selector),
				Reason:        "unsupported-python-grammar",
			})
		}
	}
	return anchors, paths, pythonClaims, nil
}

func pythonAnchorProfile(selector string) string {
	if strings.HasSuffix(selector, "#doc") {
		return "python-docstring/1"
	}
	return "python-test-name/1"
}

// objectFormat reports the repository's own object format. Canonical patch
// derivation has already loaded it in every accepted path; the guard is here
// so the value can never be reported as an empty string a consumer would then
// bind into an identity.
func objectFormat(ctx context.Context, repository *gitauth.Repository) (string, error) {
	if repository.ObjectFormat == "" {
		if err := repository.LoadObjectFormat(ctx); err != nil {
			return "", err
		}
	}
	return repository.ObjectFormat, nil
}

func intentScope(intent ocmIntent) IntentScope {
	return IntentScope{
		Path:       intent.path,
		BlobOID:    intent.blobOID,
		Start:      intent.start,
		End:        intent.end,
		SpanSHA256: intent.spanSHA256,
	}
}

// obligationRows copies the verified rows in OCM order. The reference slices
// are copied rather than aliased so a consumer cannot sort or truncate the
// verified document in place.
func obligationRows(rows []ocmObligation) []Obligation {
	obligations := make([]Obligation, 0, len(rows))
	for _, row := range rows {
		obligations = append(obligations, Obligation{
			ID:          row.id,
			Disposition: row.disposition,
			Reason:      row.reason,
			HunkIDs:     append([]string(nil), row.hunkIDs...),
			ClaimIDs:    append([]string(nil), row.claimIDs...),
		})
	}
	return obligations
}
