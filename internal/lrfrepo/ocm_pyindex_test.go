package lrfrepo

import (
	"bytes"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"testing"
)

// The naive* helpers are the whole-blob scans pythonBlobIndex replaced. They
// stay here as the equivalence oracle: the index is a performance change only,
// so every answer it gives must be the answer a fresh scan would have given.

// Hoisted so the every-offset oracle pays one compile, not one per offset.
var (
	naivePythonDefPattern       = regexp.MustCompile(`(?m)^\s*(?:async\s+)?def\s+(test_[A-Za-z0-9_]+)\s*\(`)
	naivePythonSignaturePattern = regexp.MustCompile(`(?m)^\s*(?:async\s+)?def\s+test_[A-Za-z0-9_]+\s*\([^\n]*\)\s*(?:->[^:\n]+)?\s*:\s*(?:\r?\n)`)
)

func naiveNearestPythonTest(prefix []byte) string {
	matches := naivePythonDefPattern.FindAllSubmatch(prefix, -1)
	if len(matches) == 0 {
		return ""
	}
	return string(matches[len(matches)-1][1])
}

func naiveCountPythonTest(clean []byte, name string) int {
	pattern := regexp.MustCompile(`(?m)^\s*(?:async\s+)?def\s+` + regexp.QuoteMeta(name) + `\s*\(`)
	return len(pattern.FindAllIndex(clean, -1))
}

func naivePythonFirstStatement(clean []byte, start int) bool {
	matches := naivePythonSignaturePattern.FindAllIndex(clean[:start], -1)
	if len(matches) == 0 {
		return false
	}
	definition := matches[len(matches)-1]
	definitionLine := clean[definition[0]:]
	definitionIndent := leadingIndent(definitionLine)
	anchorLineStart := bytes.LastIndexByte(clean[:start], '\n') + 1
	if leadingIndent(clean[anchorLineStart:]) <= definitionIndent {
		return false
	}
	between := clean[definition[1]:start]
	for _, line := range bytes.Split(between, []byte{'\n'}) {
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) != 0 {
			return false
		}
	}
	return true
}

// pythonIndexCorpus carries the shapes the index has to agree with the scans
// on: plain and async defs, indentation, annotations and defaults, blank lines
// and decorators before a def, names that are prefixes of longer names, a name
// that is not a plain identifier, a def that never opens its parameter list, a
// def inside a string or comment, CRLF endings, and a multi-line signature.
func pythonIndexCorpus() []string {
	return []string{
		"",
		"def test_alpha():\n    \"\"\"alpha claim\"\"\"\n",
		"async def test_alpha():\n    \"\"\"alpha claim\"\"\"\n",
		"\n\n\n    def test_alpha():\n        \"\"\"alpha claim\"\"\"\n",
		"def test_alpha(a: int = 1, *, b: str = \"x\") -> None:\n    \"\"\"alpha claim\"\"\"\n",
		"def test_alpha(\n    a,\n) -> None:\n    \"\"\"alpha claim\"\"\"\n",
		"def test_alpha():\n    pass\ndef test_alphabeta():\n    \"\"\"beta claim\"\"\"\n",
		"def test_a.b(x):\n    \"\"\"odd claim\"\"\"\n",
		"async  def   test_a.b  (x):\n    pass\n",
		"def test_alpha\n",
		"def test_alpha",
		"def ",
		"def",
		"# def test_alpha():\ndef test_beta():\n    \"\"\"beta claim\"\"\"\n",
		"x = \"def test_alpha():\"\ndef test_beta():\n    \"\"\"beta claim\"\"\"\n",
		"@pytest.mark.parametrize(\"a\", [1])\ndef test_alpha(a):\n    \"\"\"alpha claim\"\"\"\n",
		"def test_alpha():\r\n    \"\"\"alpha claim\"\"\"\r\n",
		"def test_alpha():\n    pass\ndef test_alpha():\n    pass\n",
		"def test_alpha():\n\n\n\n    \"\"\"alpha after blanks\"\"\"\n",
		"def test_alpha():\n    x = 1\n    \"\"\"not first\"\"\"\n",
		"class T:\n    def test_alpha(self):\n        \"\"\"method claim\"\"\"\n",
		"def testalpha():\n    pass\ndef test_(x):\n    pass\n",
	}
}

func pythonIndexFuzzBlob(source *rand.Rand) []byte {
	fragments := []string{
		"def ", "async def ", "async  def\t", "def\n", "xdef ", " def ", "\tdef ",
		"test_a", "test_ab", "test_a.b", "test_", "test", "testa", "_",
		"(", ")", ":", "\n", "\r\n", " ", "\t", "->", "None", ",", "#", "\"\"\"",
		"\"x\"", "'y'", "pass", "@deco", "class T:", "    ", "        ",
		"def test_a():\n", "def test_b(x) -> int:\n", "\n\n",
	}
	body := &strings.Builder{}
	for count := 0; count < 110; count++ {
		body.WriteString(fragments[source.Intn(len(fragments))])
	}
	return []byte(body.String())
}

func assertPythonIndexMatchesScans(t *testing.T, label string, blob []byte) {
	t.Helper()
	index := newPythonBlobIndex(blob)
	clean := index.clean
	if !bytes.Equal(clean, maskComments(blob, ".py")) {
		t.Fatalf("%s: index masked the blob differently", label)
	}
	definitions := pythonTestDef.FindAllSubmatchIndex(clean, -1)
	if len(definitions) != len(index.tests) {
		t.Fatalf("%s: index found %d defs, scan found %d", label, len(index.tests), len(definitions))
	}
	for ordinal, match := range definitions {
		site := index.tests[ordinal]
		if site.end != match[1] || site.name != string(clean[match[2]:match[3]]) {
			t.Fatalf("%s: def %d is %+v, scan says end=%d name=%q", label, ordinal, site, match[1], clean[match[2]:match[3]])
		}
	}
	for offset := 0; offset <= len(clean); offset++ {
		if got, want := index.nearestTest(offset), naiveNearestPythonTest(clean[:offset]); got != want {
			t.Fatalf("%s: nearestTest(%d)=%q, scan says %q", label, offset, got, want)
		}
		if got, want := index.firstStatement(offset), naivePythonFirstStatement(clean, offset); got != want {
			t.Fatalf("%s: firstStatement(%d)=%t, scan says %t", label, offset, got, want)
		}
	}
	names := map[string]bool{"test_a": true, "test_ab": true, "test_a.b": true, "test_": true,
		"test_alpha": true, "test_alphabeta": true, "test_alpha.beta": true, "test_beta": true, "test_absent": true}
	for _, site := range index.tests {
		names[site.name] = true
	}
	for name := range names {
		if got, want := index.testCount(name), naiveCountPythonTest(clean, name); got != want {
			t.Fatalf("%s: testCount(%q)=%d, scan says %d", label, name, got, want)
		}
	}
}

func TestPythonBlobIndexAnswersMatchWholeBlobScans(t *testing.T) {
	for ordinal, body := range pythonIndexCorpus() {
		assertPythonIndexMatchesScans(t, fmt.Sprintf("corpus[%d]", ordinal), []byte(body))
	}
	source := rand.New(rand.NewSource(20260829))
	for round := 0; round < 400; round++ {
		assertPythonIndexMatchesScans(t, fmt.Sprintf("fuzz[%d]", round), pythonIndexFuzzBlob(source))
	}
}

// TestPythonDefNameTallyAloneWouldFlipAVerdict is why pythonBlobIndex.testCount
// keeps a second tier instead of answering every name from the def tally. The
// Python def-prefix rule admits a name the identifier pattern never captures,
// and such a claim VERIFIES today -- answering it from the tally would report
// zero defs and reject a claim the scan accepts.
func TestPythonDefNameTallyAloneWouldFlipAVerdict(t *testing.T) {
	blob := []byte("def test_widget.renderer(x):\n    pass\n")
	name := "test_widget.renderer"
	start := bytes.Index(blob, []byte(name))
	claim := ocmClaim{path: "tests/test_widget.py", selector: "test:" + name,
		span: ocmSpan{start: int64(start), end: int64(start + len(name))}}
	if !claimShapeExtractableIn(claim, blob, newBlobIndexes()) {
		t.Fatal("the non-identifier def claim is no longer extractable; the premise moved")
	}
	index := newPythonBlobIndex(blob)
	if got := index.testCount(name); got != 1 {
		t.Fatalf("testCount(%q)=%d, want 1", name, got)
	}
	if got := index.nameCounts[name]; got != 0 {
		t.Fatalf("the identifier tally holds %q with count %d; it must not, or this test proves nothing", name, got)
	}
}

// pythonCeilingBlob is the worst artifact the OCM bounds admit for Python:
// maxClaims docstring claims in a single blob of maxOCMBlob bytes, with the
// padding placed FIRST so every claim sits behind the whole blob.
func pythonCeilingBlob(t *testing.T) ([]byte, []ocmClaim) {
	t.Helper()
	tests := &strings.Builder{}
	for index := 0; index < maxClaims; index++ {
		fmt.Fprintf(tests, "def test_widget_%04d():\n    \"\"\"OBL-%04d widget renderer parity\"\"\"\n\n", index, index)
	}
	// The padding is code, not comments: maskComments blanks a comment to
	// SPACES, and a leading-space line would give the signature scan a bogus
	// indent to compare the docstring against.
	body := &strings.Builder{}
	for filler := 0; body.Len()+tests.Len() < maxOCMBlob-64; filler++ {
		fmt.Fprintf(body, "PADDING_%d = %d\n", filler, filler)
	}
	body.WriteString(tests.String())
	blob := []byte(body.String())
	if len(blob) > maxOCMBlob {
		t.Fatalf("ceiling blob is %d bytes, over the %d-byte OCM ceiling", len(blob), maxOCMBlob)
	}
	text := string(blob)
	claims := make([]ocmClaim, 0, maxClaims)
	for index := 0; index < maxClaims; index++ {
		anchor := fmt.Sprintf("\"\"\"OBL-%04d widget renderer parity\"\"\"", index)
		at := strings.Index(text, anchor)
		if at < 0 {
			t.Fatalf("anchor %d is missing from the ceiling blob", index)
		}
		claims = append(claims, ocmClaim{
			path: "tests/test_widget.py", blobOID: strings.Repeat("b", 40),
			selector: fmt.Sprintf("test:test_widget_%04d#doc", index),
			span:     ocmSpan{start: int64(at), end: int64(at + len(anchor))},
		})
	}
	return blob, claims
}

// TestPythonClaimVerificationHoldsTheLegalCeiling pins the COMPLEXITY, not a
// duration: a wall-clock bound is fragile under -race and fleet load, so the
// assertions are structural (one derivation per blob, however many claims cite
// it) and relative (the whole ceiling costs a small multiple of ONE derivation,
// not one per claim). Both hold whatever the machine is doing.
func TestPythonClaimVerificationHoldsTheLegalCeiling(t *testing.T) {
	blob, claims := pythonCeilingBlob(t)
	cache := newBlobIndexes()
	derivation, elapsed := fastestCeilingSamples(
		func() { newPythonBlobIndex(blob) },
		func() {
			for _, claim := range claims {
				if !claimShapeExtractableIn(claim, blob, cache) {
					t.Fatalf("claim %s is not re-extractable at the ceiling", claim.selector)
				}
			}
		},
	)

	if len(cache.pythonBlobs) != 1 {
		t.Fatalf("verifying one blob derived %d Python indexes, want 1", len(cache.pythonBlobs))
	}
	if first, second := cache.pythonIndex(claims[0].blobOID, blob), cache.pythonIndex(claims[0].blobOID, blob); first != second {
		t.Fatal("the cache re-derived a blob it had already derived")
	}
	if ceiling := 8 * derivation; elapsed > ceiling {
		t.Fatalf("verifying %d claims took %v, over %v (8 blob derivations at %v) -- per-claim work is scanning the blob again",
			len(claims), elapsed, ceiling, derivation)
	}
	t.Logf("python ceiling: %d claims in a %d-byte blob, derivation=%v verify=%v", len(claims), len(blob), derivation, elapsed)
}
