package tcq

import (
	"sort"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// ocmClaim is the subset of one OCM claim row TCQ re-derives against. Per
// TCQ-V0-004 none of it is trusted: the selector and span are re-run through the
// extractor, and the language, anchor kind, and profile label are never read.
type ocmClaim struct {
	id       string
	path     string
	blobOID  string
	selector string
	start    int64
	end      int64
}

// selectedEdge is one `(obligationId, claimId)` reference in a `linked` row.
type selectedEdge struct {
	obligationID string
	claim        ocmClaim
}

type ocmDocument struct {
	targetRevision string
	sha256         string
	edges          []selectedEdge
}

// readOCM derives the exact selected set of TCQ-V0-003: obligations in OCM
// order, every claim ID in each `linked` row, claim IDs sorted within an
// obligation. A claim referenced by two obligations produces two edges.
func readOCM(raw []byte) (ocmDocument, error) {
	if err := preflightJSON(raw, ocmBounds); err != nil {
		return ocmDocument{}, err
	}
	value, err := wire.Parse(raw)
	if err != nil || value.Kind != wire.KindObject {
		return ocmDocument{}, fail(CodeInvalidTCQ)
	}
	if string(canonicalJSON(value)) != string(raw) {
		return ocmDocument{}, fail(CodeNoncanonicalMap)
	}
	if spec, ok := value.Obj.Get("spec"); !ok || spec.Kind != wire.KindString || spec.Str != OCMSpec {
		return ocmDocument{}, fail(CodeUnsupportedOCMProfile)
	}
	claims, err := readOCMClaims(value.Obj)
	if err != nil {
		return ocmDocument{}, err
	}
	edges, err := readSelectedEdges(value.Obj, claims)
	if err != nil {
		return ocmDocument{}, err
	}
	target, ok := value.Obj.Get("targetRevision")
	if !ok || target.Kind != wire.KindString {
		return ocmDocument{}, fail(CodeInvalidTCQ)
	}
	return ocmDocument{targetRevision: target.Str, sha256: sha256Hex(raw), edges: edges}, nil
}

// readOCMClaims indexes the claim rows. TCQ-V0-004 admits only extractor
// `corvint-test-claim/1`; an unknown extractor is an operational failure, not a
// per-claim abstention, so it is checked across every row.
func readOCMClaims(document *wire.Object) (map[string]ocmClaim, error) {
	value, ok := document.Get("claims")
	if !ok || value.Kind != wire.KindArray {
		return nil, fail(CodeInvalidTCQ)
	}
	claims := make(map[string]ocmClaim, len(value.Arr))
	for _, item := range value.Arr {
		if item.Kind != wire.KindObject {
			return nil, fail(CodeInvalidTCQ)
		}
		extractor, ok := item.Obj.Get("extractor")
		if !ok || extractor.Kind != wire.KindString || extractor.Str != ClaimExtractor {
			return nil, fail(CodeUnsupportedClaimExtractor)
		}
		claim, err := readOCMClaim(item.Obj)
		if err != nil {
			return nil, err
		}
		claims[claim.id] = claim
	}
	return claims, nil
}

func readOCMClaim(object *wire.Object) (ocmClaim, error) {
	identity, err := stringField(object, "id", CodeInvalidTCQ)
	if err != nil {
		return ocmClaim{}, err
	}
	path, err := stringField(object, "path", CodeInvalidTCQ)
	if err != nil {
		return ocmClaim{}, err
	}
	blob, err := stringField(object, "blobOid", CodeInvalidTCQ)
	if err != nil {
		return ocmClaim{}, err
	}
	selector, err := stringField(object, "selector", CodeInvalidTCQ)
	if err != nil {
		return ocmClaim{}, err
	}
	span, ok := object.Get("span")
	if !ok || span.Kind != wire.KindObject {
		return ocmClaim{}, fail(CodeInvalidTCQ)
	}
	start, err := intField(span.Obj, "start", 0, wire.MaxWireInteger, CodeInvalidTCQ)
	if err != nil {
		return ocmClaim{}, err
	}
	end, err := intField(span.Obj, "end", 0, wire.MaxWireInteger, CodeInvalidTCQ)
	if err != nil {
		return ocmClaim{}, err
	}
	return ocmClaim{id: identity, path: path, blobOID: blob, selector: selector, start: start, end: end}, nil
}

func readSelectedEdges(document *wire.Object, claims map[string]ocmClaim) ([]selectedEdge, error) {
	value, ok := document.Get("obligations")
	if !ok || value.Kind != wire.KindArray {
		return nil, fail(CodeInvalidTCQ)
	}
	var edges []selectedEdge
	for _, item := range value.Arr {
		if item.Kind != wire.KindObject {
			return nil, fail(CodeInvalidTCQ)
		}
		if disposition, ok := item.Obj.Get("disposition"); !ok || disposition.Kind != wire.KindString || disposition.Str != "linked" {
			continue
		}
		obligation, err := stringField(item.Obj, "id", CodeInvalidTCQ)
		if err != nil {
			return nil, err
		}
		selected, err := obligationEdges(item.Obj, obligation, claims)
		if err != nil {
			return nil, err
		}
		edges = append(edges, selected...)
	}
	if len(edges) > maxSelectedEdges {
		return nil, fail(CodeResourceExhausted)
	}
	return edges, nil
}

func obligationEdges(object *wire.Object, obligation string, claims map[string]ocmClaim) ([]selectedEdge, error) {
	value, ok := object.Get("claimIds")
	if !ok || value.Kind != wire.KindArray {
		return nil, fail(CodeInvalidTCQ)
	}
	identifiers := make([]string, 0, len(value.Arr))
	for _, item := range value.Arr {
		if item.Kind != wire.KindString {
			return nil, fail(CodeInvalidTCQ)
		}
		identifiers = append(identifiers, item.Str)
	}
	sort.Strings(identifiers)
	edges := make([]selectedEdge, 0, len(identifiers))
	for _, identifier := range identifiers {
		claim, ok := claims[identifier]
		if !ok {
			return nil, fail(CodeInvalidTCQ)
		}
		edges = append(edges, selectedEdge{obligationID: obligation, claim: claim})
	}
	return edges, nil
}
