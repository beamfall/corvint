package docviews

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/doccompiler"
)

func TestCompileProducesOneTruthCorpusAndFiveReferenceOnlyViews(t *testing.T) {
	source := testSource(t, []doccompiler.Clause{
		testSupportedClaim("C-SUPPORTED"),
		testConflictedClaim("C-CONFLICTED", "The accepted contract and implementation disagree."),
		testUnknownClaim("C-UNKNOWN", "The fallback behavior is bounded."),
	})
	options := testOptions([]string{"C-SUPPORTED", "C-CONFLICTED", "C-UNKNOWN"})
	beforeSource := deepCopySource(source)
	beforeOptions := deepCopyOptions(t, options)

	first, err := Compile(source, options)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Compile(source, options)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("equal inputs produced different bundles")
	}
	if !bytes.Equal(source.Plan, beforeSource.Plan) || !bytes.Equal(source.Patch, beforeSource.Patch) || !reflect.DeepEqual(options, beforeOptions) {
		t.Fatal("Compile mutated caller-owned input")
	}
	if len(first.Truth.Claims) != 3 || len(first.Views) != 5 {
		t.Fatalf("bundle sizes = claims %d, views %d", len(first.Truth.Claims), len(first.Views))
	}
	for index, view := range first.Views {
		if view.Audience != audienceOrder[index] || view.TruthSHA256 != first.TruthSHA256 || view.DisclosureScope != first.Truth.DisclosureScope || view.DisclosurePolicySHA256 != first.Truth.DisclosurePolicySHA256 {
			t.Fatalf("view[%d] does not bind the shared truth: %+v", index, view)
		}
	}
	if first.Truth.Revision != source.Index.CommitRevision || first.Truth.PlanSHA256 != sha256Hex(source.Plan) {
		t.Fatalf("truth does not bind the admitted plan: revision %s plan %s", first.Truth.Revision, first.Truth.PlanSHA256)
	}
	report := Verify(source, first)
	if report.Status != "PASS" || len(report.Divergences) != 0 {
		t.Fatalf("verification = %+v", report)
	}
	encoded, digest, err := CanonicalBundle(source, first)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) == 0 || encoded[len(encoded)-1] != '\n' || bytes.HasSuffix(encoded, []byte("\n\n")) || !digestValid(digest) {
		t.Fatalf("noncanonical bytes or digest: %q %s", encoded, digest)
	}
	if bytes.Count(encoded, []byte("The accepted contract and implementation disagree.")) != 1 {
		t.Fatal("claim truth was duplicated into an audience view")
	}
	for _, forbidden := range [][]byte{[]byte(`"text":"The accepted`), []byte(`"state":"CONFLICTED"`)} {
		if bytes.Count(encoded, forbidden) != 1 {
			t.Fatalf("truth field occurrence count changed for %q", forbidden)
		}
	}

	const expectedTruthSHA256 = "ef757ad4092f942fe8496122ddc9bc3b6c13eea260f89867787858f769d22523"
	const expectedBundleSHA256 = "d042866141f8c4f29c06797e6c330579ea74e11faeb6decf42be91a39639eae5"
	if first.TruthSHA256 != expectedTruthSHA256 || digest != expectedBundleSHA256 {
		t.Fatalf("fixed vector changed: truth=%s bundle=%s", first.TruthSHA256, digest)
	}
}

func TestTruthDigestAndBundleHashHDCV0041BytesWithoutHTMLOrLineSeparatorEscapes(t *testing.T) {
	const text = "a<b&c\u2028d"
	claim := testSupportedClaim("C-ESCAPE")
	claim.Text = text
	source := testSource(t, []doccompiler.Clause{claim})
	bundle, err := Compile(source, testOptions([]string{"C-ESCAPE"}))
	if err != nil {
		t.Fatal(err)
	}
	truthBytes, err := doccompiler.CanonicalJSONUnbounded(bundle.Truth)
	if err != nil {
		t.Fatal(err)
	}
	literal := []byte(`"text":"` + text + `"`)
	if !bytes.Contains(truthBytes, literal) {
		t.Fatalf("digest input does not carry %q unescaped: %s", literal, truthBytes)
	}
	digest := sha256.Sum256(append(append([]byte(nil), truthDigestDomain...), truthBytes[:len(truthBytes)-1]...))
	if bundle.TruthSHA256 != hex.EncodeToString(digest[:]) {
		t.Fatalf("truth digest %s does not hash the LF-stripped HDCV0-041 bytes", bundle.TruthSHA256)
	}
	encoded, bundleSHA256, err := CanonicalBundle(source, bundle)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, literal) || doccompiler.VerifyCanonicalJSON(encoded) != nil {
		t.Fatalf("bundle bytes are not HDCV0-041 canonical with %q unescaped", literal)
	}
	const expectedTruthSHA256 = "7dc4f73b86d42bb9a556a4d34900e0707fad897e8a765ffaef57d2af509b22fe"
	const expectedBundleSHA256 = "0212f7c1a9977c2cbaf6eebbe9800646327438a4a056927d92029338d14f04b2"
	if bundle.TruthSHA256 != expectedTruthSHA256 || bundleSHA256 != expectedBundleSHA256 {
		t.Fatalf("fixed vector changed: truth=%s bundle=%s", bundle.TruthSHA256, bundleSHA256)
	}
}

func TestCompileAcceptsEmptyTruthWithoutInventingClaims(t *testing.T) {
	source := testSource(t, nil)
	options := testOptions(nil)
	bundle, err := Compile(source, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Truth.Claims) != 0 || Verify(source, bundle).Status != "PASS" {
		t.Fatalf("empty bundle = %+v", bundle)
	}
	for _, view := range bundle.Views {
		if len(view.ClaimIDs) != 0 {
			t.Fatalf("empty truth acquired an audience claim: %+v", view)
		}
	}
}

func TestCompilePreservesARepeatedPlanAnchor(t *testing.T) {
	claim := testSupportedClaim("C1")
	claim.Anchors = append(claim.Anchors, claim.Anchors[0])
	source := testSource(t, []doccompiler.Clause{claim})
	bundle, err := Compile(source, testOptions([]string{"C1"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Truth.Claims[0].Evidence) != 2 || Verify(source, bundle).Status != "PASS" {
		t.Fatalf("repeated anchor bundle = %+v", bundle.Truth.Claims[0].Evidence)
	}
}

// TestCompileKeepsDisqualifiedAnchorsAsReviewContext binds a verified plan
// whose clauses retain HDCV0-023 disqualified anchors: a mutable anchor demoted
// to UNKNOWN, and a mutable anchor beside a qualifying one on SUPPORTED. The
// profile copies them without re-qualifying them and keeps the admitted state
// (CATN-V0-014, decision 0268).
func TestCompileKeepsDisqualifiedAnchorsAsReviewContext(t *testing.T) {
	demoted := testSupportedClaim("C-DEMOTED")
	demoted.Anchors[0].Blob = "mutable"
	supported := testSupportedClaim("C-SUPPORTED")
	supported.Anchors = append(supported.Anchors, demoted.Anchors[0])
	source := testSource(t, []doccompiler.Clause{demoted, supported})
	bundle, err := Compile(source, testOptions([]string{"C-DEMOTED", "C-SUPPORTED"}))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct{ state, frontier string }{
		"C-DEMOTED":   {doccompiler.ClauseUnknown, doccompiler.FrontierNoQualifyingSource},
		"C-SUPPORTED": {doccompiler.ClauseSupported, ""},
	}
	for _, claim := range bundle.Truth.Claims {
		mutable := 0
		for _, evidence := range claim.Evidence {
			if evidence.Blob == "mutable" {
				mutable++
			}
		}
		if claim.State != want[claim.ID].state || claim.Frontier != want[claim.ID].frontier || mutable != 1 {
			t.Errorf("claim %s = %s %q with %d mutable anchors, want %+v keeping one", claim.ID, claim.State, claim.Frontier, mutable, want[claim.ID])
		}
	}
	if report := Verify(source, bundle); report.Status != "PASS" {
		t.Fatalf("verification = %+v", report)
	}
}

func TestCompileRejectsInvalidPlanAndProjectionInputs(t *testing.T) {
	baseSource := testSource(t, []doccompiler.Clause{testSupportedClaim("C1"), testSupportedClaim("C2")})
	baseOptions := testOptions([]string{"C1", "C2"})
	tests := []struct {
		name     string
		mutate   func(*SourcePlan, *CompileOptions)
		wantCode string
	}{
		{"tampered-plan", func(source *SourcePlan, _ *CompileOptions) {
			source.Plan = bytes.Replace(source.Plan, []byte("The behavior is evidence-bound."), []byte("tampered"), 1)
		}, "INVALID_PLAN"},
		{"tampered-patch", func(source *SourcePlan, _ *CompileOptions) { source.Patch = append(source.Patch, '+') }, "INVALID_PLAN"},
		{"missing-index", func(source *SourcePlan, _ *CompileOptions) { source.Index = nil }, "INVALID_PLAN"},
		{"patch-plan-bytes", func(source *SourcePlan, _ *CompileOptions) {
			source.Plan = mustCanonical(t, doccompiler.PatchPlan{Profile: doccompiler.ExperimentalPatchPlanProfile})
		}, "INVALID_PLAN"},
		{"missing-overlay", func(_ *SourcePlan, options *CompileOptions) { options.Overlays = options.Overlays[:1] }, "INVALID_INPUT"},
		{"duplicate-overlay", func(_ *SourcePlan, options *CompileOptions) { options.Overlays[1] = options.Overlays[0] }, "INVALID_INPUT"},
		{"missing-audience", func(_ *SourcePlan, options *CompileOptions) { options.Recipes = options.Recipes[:4] }, "INVALID_INPUT"},
		{"duplicate-audience", func(_ *SourcePlan, options *CompileOptions) {
			options.Recipes[1].Audience = options.Recipes[0].Audience
		}, "INVALID_INPUT"},
		{"missing-claim", func(_ *SourcePlan, options *CompileOptions) { options.Recipes[0].ClaimIDs = []string{"C1"} }, "INVALID_INPUT"},
		{"duplicate-claim", func(_ *SourcePlan, options *CompileOptions) {
			options.Recipes[0].ClaimIDs = []string{"C1", "C1"}
		}, "INVALID_INPUT"},
		{"unknown-claim", func(_ *SourcePlan, options *CompileOptions) {
			options.Recipes[0].ClaimIDs = []string{"C1", "C3"}
		}, "INVALID_INPUT"},
		{"unknown-currency", func(_ *SourcePlan, options *CompileOptions) { options.Overlays[0].Currency = "FRESH" }, "INVALID_INPUT"},
		{"missing-stale-reason", func(_ *SourcePlan, options *CompileOptions) { options.Overlays[1].CurrencyReason = "" }, "INVALID_INPUT"},
		{"invalid-policy", func(_ *SourcePlan, options *CompileOptions) { options.DisclosurePolicySHA256 = "mutable" }, "INVALID_INPUT"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := deepCopySource(baseSource)
			options := deepCopyOptions(t, baseOptions)
			test.mutate(&source, &options)
			_, err := Compile(source, options)
			if errorCode(err) != test.wantCode {
				t.Fatalf("error = %v, want %s", err, test.wantCode)
			}
		})
	}
}

func TestCompileEnforcesLimitationBoundsAtEdges(t *testing.T) {
	source := testSource(t, []doccompiler.Clause{testSupportedClaim("C1")})
	for _, count := range []int{maxLimitationsPerClaim - 1, maxLimitationsPerClaim, maxLimitationsPerClaim + 1} {
		t.Run(fmt.Sprintf("count-%d", count), func(t *testing.T) {
			options := testOptions([]string{"C1"})
			options.Overlays[0].Limitations = make([]string, count)
			for index := range options.Overlays[0].Limitations {
				options.Overlays[0].Limitations[index] = limitationID(index)
			}
			_, err := Compile(source, options)
			if count <= maxLimitationsPerClaim && err != nil {
				t.Fatal(err)
			}
			if count > maxLimitationsPerClaim && errorCode(err) != "LIMIT_EXCEEDED" {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

// testIndex pins the anchor sources and tracks them, so a new views page is
// provably absent.
func testIndex() *contextindex.Index {
	sources := map[string]string{
		"docs/intent.md":      "# Intent\nThe contract requires a bound.\n",
		"internal/current.go": "package current\n\nfunc Bound() int { return 0 }\n",
	}
	index := &contextindex.Index{CommitRevision: strings.Repeat("b", 40), Sources: map[string]contextindex.Source{}, Tracked: map[string]struct{}{}}
	for path, text := range sources {
		digest := sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(text), text)))
		index.Sources[path] = contextindex.Source{Path: path, BlobHash: hex.EncodeToString(digest[:]), Data: []byte(text), Mode: "100644"}
		index.Tracked[path] = struct{}{}
	}
	return index
}

// testSource compiles the clauses into an admitted plan whose one operation
// creates a views page naming every clause.
func testSource(t testing.TB, clauses []doccompiler.Clause) SourcePlan {
	t.Helper()
	index := testIndex()
	request := doccompiler.AdmittedPlanRequest{Clauses: clauses}
	if len(clauses) > 0 {
		document := doccompiler.PlanDocument{ID: "op-views", Target: "docs/views.md", Reason: "the views page carries every claim"}
		for _, clause := range clauses {
			document.ClauseIDs = append(document.ClauseIDs, clause.ID)
		}
		request.Documents = []doccompiler.PlanDocument{document}
	}
	plan, patch, err := doccompiler.CompileAdmittedPlan(index, request)
	if err != nil {
		t.Fatal(err)
	}
	return SourcePlan{Index: index, Plan: plan, Patch: patch}
}

func testAnchor(index *contextindex.Index, path string, line int, authority string) doccompiler.Anchor {
	source := index.Sources[path]
	span := strings.SplitAfter(string(source.Data), "\n")[line-1]
	digest := sha256.Sum256([]byte(span))
	return doccompiler.Anchor{
		Path: path, Blob: source.BlobHash, StartLine: line, EndLine: line,
		SpanSHA256: hex.EncodeToString(digest[:]), Authority: authority, Reason: "states the claim",
	}
}

func testSupportedClaim(id string) doccompiler.Clause {
	anchor := testAnchor(testIndex(), "internal/current.go", 3, doccompiler.AuthorityPinnedSource)
	return doccompiler.Clause{ID: id, State: doccompiler.ClauseSupported, Kind: doccompiler.KindDescriptive, Text: "The behavior is evidence-bound.", Anchors: []doccompiler.Anchor{anchor}}
}

func testConflictedClaim(id, text string) doccompiler.Clause {
	index := testIndex()
	anchors := []doccompiler.Anchor{
		testAnchor(index, "docs/intent.md", 2, doccompiler.AuthorityAcceptedIntent),
		testAnchor(index, "internal/current.go", 3, doccompiler.AuthorityPinnedSource),
	}
	return doccompiler.Clause{ID: id, State: doccompiler.ClauseConflicted, Kind: doccompiler.KindPrescriptive, Text: text, Anchors: anchors}
}

func testUnknownClaim(id, text string) doccompiler.Clause {
	return doccompiler.Clause{ID: id, State: doccompiler.ClauseUnknown, Kind: doccompiler.KindDescriptive, Text: text, Frontier: doccompiler.FrontierNoQualifyingSource}
}

// forgeSource re-encodes an edited plan canonically while keeping its patch.
func forgeSource(t testing.TB, source SourcePlan, edit func(*doccompiler.AdmittedPlan)) SourcePlan {
	t.Helper()
	var plan doccompiler.AdmittedPlan
	if err := json.Unmarshal(source.Plan, &plan); err != nil {
		t.Fatal(err)
	}
	edit(&plan)
	return SourcePlan{Index: source.Index, Plan: mustCanonical(t, plan), Patch: source.Patch}
}

func mustCanonical(t testing.TB, value any) []byte {
	t.Helper()
	encoded, err := doccompiler.CanonicalJSON(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func testOptions(ids []string) CompileOptions {
	overlays := make([]ClaimOverlay, 0, len(ids))
	for index, id := range ids {
		overlay := ClaimOverlay{ClaimID: id, Currency: CurrencyCurrent, ReviewState: ReviewVerified, Limitations: []string{}}
		if index == 1 {
			overlay.Currency = CurrencyStale
			overlay.CurrencyReason = "A newer source revision exists."
			overlay.Limitations = []string{"Owner resolution is pending."}
		}
		if index == 2 {
			overlay.Currency = CurrencyUnknown
			overlay.CurrencyReason = "Currency has not been observed."
			overlay.ReviewState = ReviewGenerated
		}
		overlays = append(overlays, overlay)
	}
	details := []Detail{DetailFull, DetailStandard, DetailFull, DetailFull, DetailBrief}
	recipes := make([]ProjectionRecipe, 0, len(audienceOrder))
	for index, audience := range audienceOrder {
		order := append([]string(nil), ids...)
		if index%2 == 1 {
			for left, right := 0, len(order)-1; left < right; left, right = left+1, right-1 {
				order[left], order[right] = order[right], order[left]
			}
		}
		recipes = append(recipes, ProjectionRecipe{Audience: audience, ClaimIDs: order, Detail: details[index]})
	}
	return CompileOptions{
		DisclosurePolicySHA256: hexString("d"), DisclosureScope: "publishable/default",
		Overlays: overlays, Recipes: recipes,
	}
}

func deepCopySource(value SourcePlan) SourcePlan {
	return SourcePlan{Index: value.Index, Plan: bytes.Clone(value.Plan), Patch: bytes.Clone(value.Patch)}
}

func deepCopyOptions(t testing.TB, value CompileOptions) CompileOptions {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var result CompileOptions
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func errorCode(err error) string {
	if problem, ok := err.(*Error); ok {
		return problem.Code
	}
	return ""
}

func hexString(character string) string {
	return string(bytes.Repeat([]byte(character), 64))
}

func limitationID(index int) string {
	const digits = "0123456789abcdef"
	return "limitation-" + string([]byte{digits[(index>>4)&15], digits[index&15]})
}
