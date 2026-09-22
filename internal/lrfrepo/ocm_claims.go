package lrfrepo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Claim ENUMERATION, as distinct from the verification the read path already
// does. Verifying asks whether one recorded claim is still extractable;
// enumerating asks what claims a test blob offers, which is what `ocm link`
// needs in order to resolve a human-written selector.

var (
	goTestFunction = regexp.MustCompile(`\bfunc\s+(Test[A-Z][A-Za-z0-9_]*)\s*\(`)
	// A Go case anchor is a table entry's name field or the first argument
	// of an identifier's `.Run(` call (`t.Run("...")`), TCQ-V0-018 as amended.
	goTableCase = regexp.MustCompile(`\b(?:(?:name|testName|test_name)\s*:|[A-Za-z_][A-Za-z0-9_]*\.Run\()\s*"((?:[^"\\]|\\.)*)"`)
	// RE2 has no backreferences, so the three quote styles are separate patterns
	// rather than one with a \1 for the opening quote.
	jsTestCalls = []*regexp.Regexp{
		regexp.MustCompile(`\b(?:it|test)(?:\.(?:concurrent|only|skip|todo))?\s*\(\s*"((?:\\.|[^"\\])*)"`),
		regexp.MustCompile(`\b(?:it|test)(?:\.(?:concurrent|only|skip|todo))?\s*\(\s*'((?:\\.|[^'\\])*)'`),
		regexp.MustCompile("\\b(?:it|test)(?:\\.(?:concurrent|only|skip|todo))?\\s*\\(\\s*`((?:\\\\.|[^`\\\\])*)`"),
	}
)

// enumerateClaims returns every claim a test blob offers, ordered by claim id
// exactly as the oracle orders them. A .py blob yields nothing here: GPK-V0-037
// requires the typed Python refusal rather than an approximate extraction.
func enumerateClaims(path, blobOID string, data []byte) ([]ocmClaim, error) {
	suffix := strings.ToLower(filepath.Ext(path))
	blobs := newBlobIndexes()
	var claims []ocmClaim
	switch suffix {
	case ".go":
		claims = goClaimCandidates(path, blobOID, data, blobs.goIndex(blobOID, data))
	case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs":
		claims = jsClaimCandidates(path, blobOID, data, blobs.jsIndex(blobOID, data))
	case ".py":
		return nil, fail("unsupported-ocm-python-claims", "native Go OCM cannot exactly verify Python claim syntax")
	default:
		return nil, nil
	}
	// Only keep what the verifier would accept, so link can never record a claim
	// that the very next verify would reject.
	kept := claims[:0]
	for _, claim := range claims {
		if claimExtractableIn(claim, data, blobs) {
			kept = append(kept, claim)
		}
	}
	claims = kept
	if len(claims) > maxClaims {
		return nil, &Error{Code: "too-many-claims", Message: "test blob exceeds the claim limit"}
	}
	sort.Slice(claims, func(i, j int) bool { return claims[i].id < claims[j].id })
	for index := 1; index < len(claims); index++ {
		if claims[index].id == claims[index-1].id {
			return nil, &Error{Code: "duplicate-claim", Message: "extractor produced duplicate claims"}
		}
	}
	return claims, nil
}

func newClaim(path, blobOID, selector string, data []byte, start, end int) ocmClaim {
	digest := sha256.Sum256(data[start:end])
	claim := ocmClaim{
		extractor: claimExtractor, path: path, blobOID: blobOID, selector: selector,
		span:       ocmSpan{start: int64(start), end: int64(end)},
		spanSHA256: hex.EncodeToString(digest[:]),
	}
	claim.id = claimIdentity(claim)
	return claim
}

func goClaimCandidates(path, blobOID string, data []byte, index *goBlobIndex) []ocmClaim {
	claims := make([]ocmClaim, 0, len(index.tests)+len(index.cases))
	for _, site := range index.tests {
		claims = append(claims, newClaim(path, blobOID, "test:"+site.name, data, site.nameStart, site.nameEnd))
	}
	for _, site := range index.cases {
		if site.parent == "" {
			continue
		}
		claims = append(claims, newClaim(path, blobOID, site.selector, data, site.start, site.end))
	}
	return claims
}

func jsClaimCandidates(path, blobOID string, data []byte, index *jsBlobIndex) []ocmClaim {
	clean := index.clean
	var claims []ocmClaim
	for _, pattern := range jsTestCalls {
		for _, match := range pattern.FindAllSubmatchIndex(clean, -1) {
			var claimText string
			if err := json.Unmarshal(append(append([]byte{'"'}, clean[match[2]:match[3]]...), '"'), &claimText); err != nil {
				continue
			}
			claims = append(claims, newClaim(path, blobOID, "test:"+selectorFragment(claimText), data, match[2], match[3]))
		}
	}
	sort.Slice(claims, func(i, j int) bool { return claims[i].span.start < claims[j].span.start })
	return claims
}
