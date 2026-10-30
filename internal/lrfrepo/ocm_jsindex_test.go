package lrfrepo

import (
	"bytes"
	"fmt"
	"github.com/Beamfall/corvint/internal/testsupport"
	"math/rand"
	"regexp"
	"strings"
	"testing"
)

// naiveCountJSSelector is the whole-blob walk jsBlobIndex replaced. It stays
// here as the equivalence oracle: the index is a performance change only.
var naiveJSCallPattern = regexp.MustCompile(`\b(?:it|test)(?:\.(?:concurrent|only|skip|todo))?\s*\(\s*`)

func naiveCountJSSelector(clean []byte, selector string) int {
	prefix := naiveJSCallPattern
	count := 0
	for _, match := range prefix.FindAllIndex(clean, -1) {
		start := match[1]
		if start >= len(clean) || clean[start] != '"' && clean[start] != '\'' && clean[start] != '`' {
			continue
		}
		quote := clean[start]
		end := start + 1
		for end < len(clean) {
			if clean[end] == '\\' {
				end += 2
				continue
			}
			if clean[end] == quote {
				break
			}
			end++
		}
		if end >= len(clean) {
			continue
		}
		claimText, ok := decodeJSString(clean[start+1:end], quote)
		if ok && "test:"+selectorFragment(claimText) == selector {
			count++
		}
	}
	return count
}

func jsIndexCorpus() []string {
	return []string{
		"",
		"it(\"renders the widget list\", () => {});\n",
		"test('renders the widget list', () => {});\n",
		"it(`renders the widget list`, () => {});\n",
		"it(`renders ${count} widgets`, () => {});\n",
		"it.only(\"renders alpha\", () => {});\nit.skip(\"renders beta\", () => {});\n",
		"it.concurrent (\n  \"renders gamma\", () => {});\n",
		"it.todo(\"renders delta\");\n",
		"xit(\"renders epsilon\", () => {});\nfit(\"renders zeta\", () => {});\n",
		"// it(\"commented decoy\", () => {});\nit(\"renders eta\", () => {});\n",
		"/* it(\"block decoy\") */ it(\"renders theta\", () => {});\n",
		"const s = \"it(\\\"quoted decoy\\\")\";\nit(\"renders iota\", () => {});\n",
		"it(\"renders \\\"escaped\\\" widget\", () => {});\n",
		"it('renders \\'escaped\\' widget', () => {});\n",
		"it(\"renders alpha\", () => {});\nit(\"renders alpha\", () => {});\n",
		"it(\"renders widget\", () => {});\nit(\"render widgets\", () => {});\n",
		"it(\"unterminated\n",
		"it(",
		"it(\"\", () => {});\n",
		"describe(\"outer\", () => { it(\"renders kappa\", () => {}); });\n",
		"unit(\"not a test\", () => {});\nlatest(\"not a test\", () => {});\n",
	}
}

func jsIndexFuzzBlob(source *rand.Rand) []byte {
	fragments := []string{
		"it(", "test(", "it .only(", "it.skip (", "test.concurrent(", "unit(", "xit(",
		"\"renders alpha\"", "'renders beta'", "`renders gamma`", "`renders ${x}`",
		"\"\"", "\"a\"", "\"renders alpha\"", "\\", "\\\"", "'", "\"", "`",
		"(", ")", ",", " ", "\n", "\t", "// ", "/* ", " */", "=>", "{}", ";",
	}
	body := &strings.Builder{}
	for count := 0; count < 90; count++ {
		body.WriteString(fragments[source.Intn(len(fragments))])
	}
	return []byte(body.String())
}

func assertJSIndexMatchesScans(t *testing.T, label string, blob []byte) {
	t.Helper()
	index := newJSBlobIndex(blob)
	// jsClaimCandidates used to mask with the claim path's own suffix. Every JS
	// suffix must mask identically, or collapsing them onto the index's one
	// masked buffer would change what the candidates see.
	for _, suffix := range []string{".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs"} {
		if !bytes.Equal(index.clean, maskComments(blob, suffix)) {
			t.Fatalf("%s: masking under %q differs from the index's buffer", label, suffix)
		}
	}
	selectors := map[string]bool{"test:absent-selector": true, "test:unnamed": true}
	for selector := range index.selectorCounts {
		selectors[selector] = true
	}
	for _, pattern := range jsTestCalls {
		for _, match := range pattern.FindAllSubmatchIndex(index.clean, -1) {
			if text, ok := decodeJSString(index.clean[match[2]:match[3]], index.clean[match[2]-1]); ok {
				selectors["test:"+selectorFragment(text)] = true
			}
		}
	}
	for selector := range selectors {
		if got, want := index.selectorCount(selector), naiveCountJSSelector(index.clean, selector); got != want {
			t.Fatalf("%s: selectorCount(%q)=%d, scan says %d", label, selector, got, want)
		}
	}
}

func TestJSBlobIndexAnswersMatchWholeBlobScans(t *testing.T) {
	for ordinal, body := range jsIndexCorpus() {
		assertJSIndexMatchesScans(t, fmt.Sprintf("corpus[%d]", ordinal), []byte(body))
	}
	source := rand.New(rand.NewSource(20260829))
	for round := 0; round < 400; round++ {
		assertJSIndexMatchesScans(t, fmt.Sprintf("fuzz[%d]", round), jsIndexFuzzBlob(source))
	}
}

// jsCeilingBlob is the worst artifact the OCM bounds admit for JS: maxClaims
// test calls in a single blob of maxOCMBlob bytes, padding FIRST. distinct
// controls whether the cases derive distinct selectors -- colliding selectors
// are rejected as ambiguous, but only AFTER the selector tally is consulted, so
// both shapes exercise the same scan.
func jsCeilingBlob(t *testing.T, distinct bool) []byte {
	t.Helper()
	cases := &strings.Builder{}
	for index := 0; index < maxClaims; index++ {
		name := "OBL-0000 widget renderer parity"
		if distinct {
			name = fmt.Sprintf("renders widget %s in priority order", ceilingSuffix(index))
		}
		fmt.Fprintf(cases, "it(%q, () => { expect(1).toBe(1); });\n", name)
	}
	body := &strings.Builder{}
	for filler := 0; body.Len()+cases.Len() < maxOCMBlob-64; filler++ {
		fmt.Fprintf(body, "function padding%d() { return %d; }\n", filler, filler)
	}
	body.WriteString(cases.String())
	blob := []byte(body.String())
	if len(blob) > maxOCMBlob {
		t.Fatalf("ceiling blob is %d bytes, over the %d-byte OCM ceiling", len(blob), maxOCMBlob)
	}
	return blob
}

// TestJSClaimEnumerationHoldsTheLegalCeiling pins the COMPLEXITY, not a
// duration, for the same reason the Go and Python ceilings do: one derivation
// per blob however many claims cite it, and a whole ceiling that costs a small
// multiple of ONE derivation rather than one per claim.
func TestJSClaimEnumerationHoldsTheLegalCeiling(t *testing.T) {
	testsupport.SkipTimingUnderLoad(t, "host load exceeds core count; the 8x derivation ceiling is unreliable under load")
	blob := jsCeilingBlob(t, true)
	oid := strings.Repeat("c", 40)
	var claims []ocmClaim
	var err error
	derivation, elapsed := fastestCeilingSamples(
		func() { newJSBlobIndex(blob) },
		func() { claims, err = enumerateClaims("tests/claims.test.js", oid, blob) },
	)
	if err != nil {
		t.Fatalf("enumerate: %v", err)
	}
	if len(claims) != maxClaims {
		t.Fatalf("enumerated %d claims, want the %d-claim ceiling", len(claims), maxClaims)
	}

	cache := newBlobIndexes()
	for _, claim := range claims {
		if !claimShapeExtractableIn(claim, blob, cache) {
			t.Fatalf("claim %s is not re-extractable at the ceiling", claim.selector)
		}
	}
	if len(cache.jsBlobs) != 1 {
		t.Fatalf("verifying one blob derived %d JS indexes, want 1", len(cache.jsBlobs))
	}
	if first, second := cache.jsIndex(oid, blob), cache.jsIndex(oid, blob); first != second {
		t.Fatal("the cache re-derived a blob it had already derived")
	}
	if ceiling := 8 * derivation; elapsed > ceiling {
		t.Fatalf("enumerating %d claims took %v, over %v (8 blob derivations at %v) -- per-claim work is scanning the blob again",
			len(claims), elapsed, ceiling, derivation)
	}
	t.Logf("js ceiling: %d claims in a %d-byte blob, derivation=%v enumerate=%v", len(claims), len(blob), derivation, elapsed)
}

func TestJSRegexLiteralKeepsCommentsMasked(t *testing.T) {
	t.Run("OCM-V0-005 a commented-out test after a regex literal holding a quote is no claim", func(t *testing.T) {
		blob := []byte("const q = /['\"]/;\n\n// it(\"commented RGX-V0-001 case\", () => {});\n\nit(\"real RGX-V0-002 case\", () => {});\n")
		claims, err := enumerateClaims("tests/regex.test.js", "oid", blob)
		if err != nil || len(claims) != 1 || !strings.Contains(claims[0].selector, "real") {
			t.Fatalf("claims = %+v, err = %v; want only the real case", claims, err)
		}
	})
	t.Run("OCM-V0-005 division is not read as a regex literal", func(t *testing.T) {
		blob := []byte("const half = total / 2; // it(\"commented RGX-V0-003 case\", () => {});\n\nit(\"real RGX-V0-004 case\", () => {});\n")
		claims, err := enumerateClaims("tests/division.test.js", "oid", blob)
		if err != nil || len(claims) != 1 || !strings.Contains(claims[0].selector, "real") {
			t.Fatalf("claims = %+v, err = %v; want only the real case", claims, err)
		}
	})
}

func TestJSTemplateExpressionKeepsCommentsMasked(t *testing.T) {
	t.Run("OCM-V0-005 a backtick inside a template expression does not close the template", func(t *testing.T) {
		blob := []byte("const label = `${\"`\"}`;\n\n// it(\"commented TPL-V0-001 case\", () => {});\n\nit(\"real TPL-V0-002 case\", () => {});\n")
		claims, err := enumerateClaims("tests/template.test.js", "oid", blob)
		if err != nil || len(claims) != 1 || !strings.Contains(claims[0].selector, "real") {
			t.Fatalf("claims = %+v, err = %v; want only the real case", claims, err)
		}
	})
}
