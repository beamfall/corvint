package lrfrepo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

// The naive* helpers are the whole-blob scans goBlobIndex replaced. They stay
// here as the equivalence oracle: the index is a performance change only, so
// every answer it gives must be the answer a fresh scan would have given.

// Hoisted so the every-offset oracle pays one compile, not one per offset.
var naiveGoTestPattern = regexp.MustCompile(`\bfunc\s+(Test[A-Z][A-Za-z0-9_]*)\s*\(`)

func naiveNearestGoTest(prefix []byte) string {
	matches := naiveGoTestPattern.FindAllSubmatch(prefix, -1)
	if len(matches) == 0 {
		return ""
	}
	return string(matches[len(matches)-1][1])
}

func naiveCountGoSelector(clean []byte, selector string) int {
	casePattern := regexp.MustCompile(`\b(?:(?:name|testName|test_name)\s*:|[A-Za-z_][A-Za-z0-9_]*\.Run\()\s*"((?:[^"\\]|\\.)*)"`)
	count := 0
	for _, match := range casePattern.FindAllSubmatchIndex(clean, -1) {
		var claimText string
		if err := json.Unmarshal(append(append([]byte{'"'}, clean[match[2]:match[3]]...), '"'), &claimText); err != nil {
			continue
		}
		parent := naiveNearestGoTest(clean[:match[0]])
		if "test:"+parent+"/case:"+selectorFragment(claimText) == selector {
			count++
		}
	}
	return count
}

func naiveGoFuncCount(clean []byte, name string) int {
	return len(regexp.MustCompile(`\bfunc\s+`+regexp.QuoteMeta(name)+`\s*\(`).FindAllIndex(clean, -1))
}

func naiveGoTestFunctions(clean []byte) [][]int {
	return goTestFunction.FindAllSubmatchIndex(clean, -1)
}

// goIndexCorpus carries the shapes the index has to agree with the scans on:
// plain declarations, names that are prefixes of longer names, a `func` that is
// not a declaration, a name that is not a plain identifier, cases with no
// enclosing test, repeated cases, and commented or quoted decoys.
func goIndexCorpus() []string {
	return []string{
		"package p\n",
		"package p\nfunc TestAlpha(t *testing.T) {}\n",
		"package p\nfunc TestAlpha (\nt *testing.T) {}\nfunc TestAlphaBeta(t *testing.T) {}\n",
		"package p\nfunc TestAlpha.Beta(t *testing.T) {}\n",
		"package p\nfunc func TestAlpha(t *testing.T) {}\n",
		"package p\nxfunc TestAlpha(t *testing.T) {}\n",
		"package p\nfunc\tTestAlpha\n\t(t *testing.T) {}\n",
		"package p\nfunc TestAlphafunc TestBeta(t *testing.T) {}\n",
		"package p\n_ = []struct{ name string }{{name: \"orphan case here\"}}\n",
		"package p\nfunc TestAlpha(t *testing.T) {\n_ = []struct{ name string }{{name: \"widget renderer parity\"}, {testName: \"widget renderer parity\"}}\n}\n",
		"package p\nfunc TestAlpha(t *testing.T) {\n// name: \"commented decoy\"\n_ = \"name: \\\"quoted decoy\\\"\"\n_ = []struct{ test_name string }{{test_name: \"real one\"}}\n}\n",
		"package p\nfunc TestAlpha(t *testing.T) {\nt.Run(\"TCP-V0-003 rows carry an action\", func(t *testing.T) {})\n_ = cmd.Run(\"not a test\")\n}\n",
		"package p\nfunc TestAlpha(t *testing.T) {}\nfunc TestAlpha(t *testing.T) {}\n",
		"package p\nfunc Testalpha(t *testing.T) {}\nfunc Test_Alpha(t *testing.T) {}\nfunc Test(t *testing.T) {}\n",
		"package p\n/* func TestAlpha( */\nfunc TestBeta(t *testing.T) {\n_ = []struct{ name string }{{name: \"beta case\"}}\n}\n",
		"func TestAlpha(",
		"func TestAlpha",
		"func ",
		"func",
	}
}

// goIndexFuzzBlob builds pseudo-random Go-ish text out of fragments chosen to
// collide with every boundary the two patterns care about.
func goIndexFuzzBlob(source *rand.Rand) []byte {
	fragments := []string{
		"func ", "func\t", "func\n", "funcy ", "xfunc ", "func func ",
		"TestA", "TestAB", "TestA_", "Testa", "Test", "TestA.B", "A",
		"(", ")", " ", "\t", "\n", "{", "}", ".", "_", ",",
		"name: ", "testName: ", "test_name: ", "name:", "\"widget parity\"", "t.Run(", "tb.Run( ", ".Run(",
		"\"alpha\"", "\"a b c\"", "\"\"", "\"\\\"x\\\"\"", "// ", "/* ", " */",
		"_ = []struct{ name string }{{name: \"case one\"}}", "\"", "\\",
	}
	body := &strings.Builder{}
	for count := 0; count < 120; count++ {
		body.WriteString(fragments[source.Intn(len(fragments))])
	}
	return []byte(body.String())
}

func assertGoIndexMatchesScans(t *testing.T, label string, blob []byte) {
	t.Helper()
	index := newGoBlobIndex(blob)
	clean := index.clean
	if !bytes.Equal(clean, maskComments(blob, ".go")) {
		t.Fatalf("%s: index masked the blob differently", label)
	}
	functions := naiveGoTestFunctions(clean)
	if len(functions) != len(index.tests) {
		t.Fatalf("%s: index found %d test declarations, scan found %d", label, len(index.tests), len(functions))
	}
	for ordinal, match := range functions {
		site := index.tests[ordinal]
		if site.nameStart != match[2] || site.nameEnd != match[3] || site.end != match[1] || site.name != string(clean[match[2]:match[3]]) {
			t.Fatalf("%s: declaration %d is %+v, scan says %v", label, ordinal, site, match)
		}
	}
	for offset := 0; offset <= len(clean); offset++ {
		if got, want := index.nearestTest(offset), naiveNearestGoTest(clean[:offset]); got != want {
			t.Fatalf("%s: nearestTest(%d)=%q, scan says %q", label, offset, got, want)
		}
	}
	decoded := 0
	for _, match := range goTableCase.FindAllSubmatchIndex(clean, -1) {
		var claimText string
		if err := json.Unmarshal(append(append([]byte{'"'}, clean[match[2]:match[3]]...), '"'), &claimText); err != nil {
			continue
		}
		if decoded >= len(index.cases) {
			t.Fatalf("%s: index holds %d cases, scan found more", label, len(index.cases))
		}
		site := index.cases[decoded]
		want := goCaseSite{start: match[2], end: match[3], parent: naiveNearestGoTest(clean[:match[0]])}
		want.selector = "test:" + want.parent + "/case:" + selectorFragment(claimText)
		if site != want {
			t.Fatalf("%s: case %d is %+v, scan says %+v", label, decoded, site, want)
		}
		decoded++
	}
	if decoded != len(index.cases) {
		t.Fatalf("%s: index holds %d cases, scan decoded %d", label, len(index.cases), decoded)
	}
	selectors := map[string]bool{"test:TestAlpha/case:absent": true}
	for _, site := range index.cases {
		selectors[site.selector] = true
	}
	for selector := range selectors {
		if got, want := index.caseCount(selector), naiveCountGoSelector(clean, selector); got != want {
			t.Fatalf("%s: caseCount(%q)=%d, scan says %d", label, selector, got, want)
		}
	}
	names := map[string]bool{"TestA": true, "TestAB": true, "TestA_": true, "TestA.B": true, "TestAlpha": true, "TestAlphaBeta": true, "TestAlpha.Beta": true, "TestAfunc": true, "TestZ": true}
	for _, site := range index.tests {
		names[site.name] = true
	}
	for name := range names {
		if got, want := index.funcCount(name), naiveGoFuncCount(clean, name); got != want {
			t.Fatalf("%s: funcCount(%q)=%d, scan says %d", label, name, got, want)
		}
	}
}

func TestGoBlobIndexAnswersMatchWholeBlobScans(t *testing.T) {
	for ordinal, body := range goIndexCorpus() {
		assertGoIndexMatchesScans(t, fmt.Sprintf("corpus[%d]", ordinal), []byte(body))
	}
	source := rand.New(rand.NewSource(20260829))
	for round := 0; round < 400; round++ {
		assertGoIndexMatchesScans(t, fmt.Sprintf("fuzz[%d]", round), goIndexFuzzBlob(source))
	}
}

// legalCeilingBlob is the worst artifact the OCM bounds admit: maxClaims table
// cases in a single blob of maxOCMBlob bytes, with the padding placed FIRST so
// every claim sits behind the whole blob. Before the index that shape cost
// ~40s of scanning PER CLAIM.
func legalCeilingBlob(t *testing.T) []byte {
	t.Helper()
	claims := &strings.Builder{}
	for index := 0; index < maxClaims; index++ {
		fmt.Fprintf(claims, "\nfunc TestClaim%s(t *testing.T) {\n\t_ = []struct{ name string }{{name: %q}}\n}\n",
			ceilingSuffix(index), fmt.Sprintf("OBL-%04d widget renderer parity", index))
	}
	body := &strings.Builder{}
	body.WriteString("package tests\n\nimport \"testing\"\n")
	for filler := 0; body.Len()+claims.Len() < maxOCMBlob-64; filler++ {
		fmt.Fprintf(body, "\nfunc padding%d() int { return %d }\n", filler, filler)
	}
	body.WriteString(claims.String())
	blob := []byte(body.String())
	if len(blob) > maxOCMBlob {
		t.Fatalf("ceiling blob is %d bytes, over the %d-byte OCM ceiling", len(blob), maxOCMBlob)
	}
	return blob
}

// ceilingSuffix is bijective base-26, so maxClaims declarations never collide.
func ceilingSuffix(index int) string {
	letters := []rune{}
	for {
		letters = append([]rune{rune('A' + index%26)}, letters...)
		index = index/26 - 1
		if index < 0 {
			return string(letters)
		}
	}
}

// TestGoClaimVerificationHoldsTheLegalCeiling pins the COMPLEXITY, not a
// duration: a wall-clock bound is fragile under -race and fleet load, so the
// assertions are structural (one derivation per blob, however many claims cite
// it) and relative (the whole ceiling costs a small multiple of ONE derivation,
// not one per claim). Both hold whatever the machine is doing. The pre-index
// scan cost this ceiling ~40s per claim, ~5h46m in total.
func TestGoClaimVerificationHoldsTheLegalCeiling(t *testing.T) {
	blob := legalCeilingBlob(t)
	oid := strings.Repeat("a", 40)
	claims, err := enumerateClaims("tests/claims_test.go", oid, blob)
	if err != nil {
		t.Fatalf("enumerate: %v", err)
	}
	if len(claims) != maxClaims {
		t.Fatalf("enumerated %d claims, want the %d-claim ceiling", len(claims), maxClaims)
	}
	cache := newBlobIndexes()
	derivation, elapsed := fastestCeilingSamples(
		func() { newGoBlobIndex(blob) },
		func() {
			for _, claim := range claims {
				if !claimShapeExtractableIn(claim, blob, cache) {
					t.Fatalf("claim %s is not re-extractable at the ceiling", claim.selector)
				}
			}
		},
	)

	if len(cache.goBlobs) != 1 {
		t.Fatalf("verifying one blob derived %d indexes, want 1", len(cache.goBlobs))
	}
	if first, second := cache.goIndex(oid, blob), cache.goIndex(oid, blob); first != second {
		t.Fatal("the cache re-derived a blob it had already derived")
	}
	if ceiling := 8 * derivation; elapsed > ceiling {
		t.Fatalf("verifying %d claims took %v, over %v (8 blob derivations at %v) -- per-claim work is scanning the blob again",
			len(claims), elapsed, ceiling, derivation)
	}
	t.Logf("legal ceiling: %d claims in a %d-byte blob, derivation=%v verify=%v, budget=%v",
		len(claims), len(blob), derivation, elapsed, gitrun.DefaultTotalBudget)
}

func TestGoRunLiteralIsACaseAnchor(t *testing.T) {
	blob := []byte("package p\nfunc TestRows(t *testing.T) {\n\tt.Run(\"TCP-V0-003 rows carry an action\", func(t *testing.T) {})\n\t_ = exec.Run(\"other\")\n}\n")
	index := newGoBlobIndex(blob)
	selectors := make([]string, 0, len(index.cases))
	for _, site := range index.cases {
		selectors = append(selectors, site.selector)
	}
	want := []string{"test:TestRows/case:" + selectorFragment("TCP-V0-003 rows carry an action"), "test:TestRows/case:" + selectorFragment("other")}
	if len(selectors) != 2 || selectors[0] != want[0] || selectors[1] != want[1] {
		t.Fatalf("cases = %v, want %v", selectors, want)
	}
	if index.caseCount(want[0]) != 1 || index.nearestTest(index.cases[0].start) != "TestRows" {
		t.Fatalf("the run literal must be a unique case of its parent test")
	}
}

func TestGoRawStringBackslashKeepsCommentsMasked(t *testing.T) {
	t.Run("OCM-V0-005 a commented-out test after a raw string ending in a backslash is no claim", func(t *testing.T) {
		blob := []byte("package p\n\nvar sep = `\\`\n\n// func TestCommentedRowsCarryAction(t *testing.T) {}\n\nfunc TestRealRowsCarryAction(t *testing.T) {}\n")
		claims, err := enumerateClaims("p_test.go", "oid", blob)
		if err != nil || len(claims) != 1 || claims[0].selector != "test:TestRealRowsCarryAction" {
			t.Fatalf("claims = %+v, err = %v; want only test:TestRealRowsCarryAction", claims, err)
		}
	})
}
