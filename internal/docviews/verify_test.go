package docviews

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccompiler"
)

func TestVerifyRejectsEveryCrossViewDivergenceClass(t *testing.T) {
	plan := testSource(t, []doccompiler.Clause{testSupportedClaim("C1"), testConflictedClaim("C2", "Sources disagree.")})
	base, err := Compile(plan, testOptions([]string{"C1", "C2"}))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*Bundle)
		code   string
	}{
		{"bundle-profile", func(bundle *Bundle) { bundle.Profile = "other" }, "INVALID_PROFILE"},
		{"truth-state", func(bundle *Bundle) { bundle.Truth.Claims[1].State = doccompiler.ClauseSupported }, "TRUTH_DIGEST_MISMATCH"},
		{"truth-currency", func(bundle *Bundle) {
			bundle.Truth.Claims[1].Currency = CurrencyCurrent
			bundle.Truth.Claims[1].CurrencyReason = ""
		}, "TRUTH_DIGEST_MISMATCH"},
		{"truth-limitation", func(bundle *Bundle) { bundle.Truth.Claims[1].Limitations = nil }, "TRUTH_DIGEST_MISMATCH"},
		{"truth-evidence", func(bundle *Bundle) { bundle.Truth.Claims[1].Evidence = bundle.Truth.Claims[1].Evidence[:1] }, "TRUTH_DIGEST_MISMATCH"},
		{"view-truth", func(bundle *Bundle) { bundle.Views[0].TruthSHA256 = hexString("0") }, "TRUTH_DIGEST_MISMATCH"},
		{"view-scope", func(bundle *Bundle) { bundle.Views[1].DisclosureScope = "publishable/other" }, "POLICY_SCOPE_MISMATCH"},
		{"missing-claim", func(bundle *Bundle) { bundle.Views[2].ClaimIDs = bundle.Views[2].ClaimIDs[:1] }, "MISSING_CLAIM"},
		{"duplicate-claim", func(bundle *Bundle) { bundle.Views[2].ClaimIDs[1] = bundle.Views[2].ClaimIDs[0] }, "DUPLICATE_CLAIM"},
		{"unknown-claim", func(bundle *Bundle) { bundle.Views[3].ClaimIDs[0] = "C-UNKNOWN" }, "UNKNOWN_CLAIM"},
		{"missing-audience", func(bundle *Bundle) { bundle.Views = bundle.Views[:4] }, "MISSING_AUDIENCE"},
		{"duplicate-audience", func(bundle *Bundle) { bundle.Views[1].Audience = bundle.Views[0].Audience }, "DUPLICATE_AUDIENCE"},
		{"invalid-detail", func(bundle *Bundle) { bundle.Views[4].Detail = "HIDDEN" }, "INVALID_DETAIL"},
		{"view-order", func(bundle *Bundle) { bundle.Views[0], bundle.Views[1] = bundle.Views[1], bundle.Views[0] }, "NONCANONICAL_ORDER"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bundle := deepCopyBundle(t, base)
			test.mutate(&bundle)
			report := Verify(plan, bundle)
			if report.Status != "FAIL" || !hasDivergence(report, test.code) {
				t.Fatalf("report = %+v, want %s", report, test.code)
			}
			if _, _, err := CanonicalBundle(plan, bundle); errorCode(err) != "NON_EQUIVALENT" {
				t.Fatalf("canonical error = %v", err)
			}
		})
	}
}

// CATN-V0-008: divergences for distinct invalid audiences are sorted by the
// audience spelling and deduplicated, whatever order the views arrive in.
func TestVerifySortsAndDeduplicatesInvalidAudienceDivergences(t *testing.T) {
	plan := testSource(t, []doccompiler.Clause{testSupportedClaim("C1")})
	base, err := Compile(plan, testOptions([]string{"C1"}))
	if err != nil {
		t.Fatal(err)
	}
	bundle := deepCopyBundle(t, base)
	bundle.Views[0].Audience, bundle.Views[1].Audience, bundle.Views[2].Audience = "zz-invalid", "aa-invalid", "zz-invalid"
	report := Verify(plan, bundle)
	for index := 1; index < len(report.Divergences); index++ {
		left, right := report.Divergences[index-1], report.Divergences[index]
		if left == right {
			t.Fatalf("duplicate divergence %+v in %+v", right, report.Divergences)
		}
		if audienceRank(left.Audience) == audienceRank(right.Audience) && left.Audience > right.Audience {
			t.Fatalf("divergence %+v sorts after %+v: %+v", left, right, report.Divergences)
		}
	}
}

func TestVerifyRejectsRehashedTruthForgeryAgainstSourcePlan(t *testing.T) {
	plan := testSource(t, []doccompiler.Clause{testSupportedClaim("C1"), testSupportedClaim("C2")})
	bundle, err := Compile(plan, testOptions([]string{"C1", "C2"}))
	if err != nil {
		t.Fatal(err)
	}

	bundle.Truth.Claims[0].Text = "A forged replacement that is internally self-consistent."
	bundle.TruthSHA256, err = truthDigest(bundle.Truth)
	if err != nil {
		t.Fatal(err)
	}
	for index := range bundle.Views {
		bundle.Views[index].TruthSHA256 = bundle.TruthSHA256
	}

	report := Verify(plan, bundle)
	if report.Status != "FAIL" || !hasDivergence(report, "PLAN_CLAIM_MISMATCH") || hasDivergence(report, "TRUTH_DIGEST_MISMATCH") {
		t.Fatalf("rehashed truth forgery was not isolated to source-plan mismatch: %+v", report)
	}
	if _, _, err := CanonicalBundle(plan, bundle); errorCode(err) != "NON_EQUIVALENT" {
		t.Fatalf("canonical error = %v", err)
	}
}

// CATN-V0-014: a CONFLICTED claim needs two anchors at distinct (path, blob,
// start_line, end_line); compilation and verification both refuse fewer.
func TestConflictedClaimRequiresTwoDistinctEvidenceLocations(t *testing.T) {
	plan := testSource(t, []doccompiler.Clause{testConflictedClaim("C1", "Sources disagree.")})
	refused := map[string]func([]doccompiler.Anchor) []doccompiler.Anchor{
		"zero-anchors": func([]doccompiler.Anchor) []doccompiler.Anchor { return []doccompiler.Anchor{} },
		"one-anchor":   func(anchors []doccompiler.Anchor) []doccompiler.Anchor { return anchors[:1] },
		"same-location": func(anchors []doccompiler.Anchor) []doccompiler.Anchor {
			return []doccompiler.Anchor{anchors[0], anchors[0]}
		},
	}
	for name, edit := range refused {
		t.Run(name, func(t *testing.T) {
			forged := forgeSource(t, plan, func(value *doccompiler.AdmittedPlan) { value.Clauses[0].Anchors = edit(value.Clauses[0].Anchors) })
			if _, err := Compile(forged, testOptions([]string{"C1"})); errorCode(err) != "INVALID_PLAN" {
				t.Fatalf("compile error = %v, want INVALID_PLAN", err)
			}
		})
	}

	bundle, err := Compile(plan, testOptions([]string{"C1"}))
	if err != nil {
		t.Fatal(err)
	}
	bundle.Truth.Claims[0].Evidence = bundle.Truth.Claims[0].Evidence[:1]
	if bundle.TruthSHA256, err = truthDigest(bundle.Truth); err != nil {
		t.Fatal(err)
	}
	for index := range bundle.Views {
		bundle.Views[index].TruthSHA256 = bundle.TruthSHA256
	}
	if report := Verify(plan, bundle); report.Status != "FAIL" || !hasDivergence(report, "INVALID_TRUTH") {
		t.Fatalf("single-anchor CONFLICTED truth report = %+v, want INVALID_TRUTH", report)
	}
}

func TestVerifyRejectsCanonicalBundleOver64MiB(t *testing.T) {
	// The source plan stays inside the HDCV0-041 8 MiB bound; the size comes
	// from 64 maximal limitations shared by every in-memory overlay, so the
	// hostile fixture has a small persistent footprint while its canonical
	// representation is unambiguously larger than 64 MiB.
	sharedLimitations := make([]string, maxLimitationsPerClaim)
	for index := range sharedLimitations {
		sharedLimitations[index] = fmt.Sprintf("%02d", index) + strings.Repeat("x", maxLimitationBytes-2)
	}
	claims := make([]doccompiler.Clause, 0, 260)
	ids := make([]string, 0, 260)
	for index := 0; index < 260; index++ {
		id := fmt.Sprintf("C-%04d", index)
		ids = append(ids, id)
		claims = append(claims, testUnknownClaim(id, "The bound is unobserved."))
	}
	plan := testSource(t, claims)
	options := testOptions(ids)
	for index := range options.Overlays {
		options.Overlays[index].Limitations = sharedLimitations
	}
	admitted, err := verifiedPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	truthClaims, err := claimsFromPlan(admitted, options.Overlays)
	if err != nil {
		t.Fatal(err)
	}
	truth := TruthCorpus{
		Claims: truthClaims, DisclosurePolicySHA256: options.DisclosurePolicySHA256,
		DisclosureScope: options.DisclosureScope, PlanSHA256: sha256Hex(plan.Plan),
		Profile: TruthProfile, Revision: admitted.SourceIdentity.Revision,
	}
	truthSHA256, err := truthDigest(truth)
	if err != nil {
		t.Fatal(err)
	}
	views, err := compileViews(truth, truthSHA256, options.Recipes)
	if err != nil {
		t.Fatal(err)
	}
	bundle := Bundle{Profile: BundleProfile, Truth: truth, TruthSHA256: truthSHA256, Views: views}

	report := Verify(plan, bundle)
	if report.Status != "FAIL" || !hasDivergence(report, "LIMIT_EXCEEDED") {
		t.Fatalf("oversized canonical bundle passed verification: %+v", report)
	}
	assertCanonicalFailure := func(want string) {
		t.Helper()
		encoded, digest, err := CanonicalBundle(plan, bundle)
		if errorCode(err) != want {
			t.Fatalf("canonical error = %v, want %s", err, want)
		}
		if encoded != nil || digest != "" {
			t.Fatalf("canonical failure returned partial output: %d bytes, digest %q", len(encoded), digest)
		}
	}
	assertCanonicalFailure("LIMIT_EXCEEDED")

	bundle.Profile = "other"
	assertCanonicalFailure("NON_EQUIVALENT")
	bundle.Profile = BundleProfile
	bundle.TruthSHA256 = hexString("0")
	assertCanonicalFailure("NON_EQUIVALENT")
}

func TestProductionPackageImportsRemainPure(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate package")
	}
	root := filepath.Dir(filename)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]struct{}{
		"bytes": {}, "crypto/sha256": {}, "encoding/hex": {}, "encoding/json": {}, "fmt": {},
		"regexp": {}, "sort": {}, "strconv": {}, "strings": {}, "unicode/utf8": {},
		"github.com/Beamfall/corvint/internal/contextindex": {}, "github.com/Beamfall/corvint/internal/doccompiler": {},
	}
	files := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" || len(entry.Name()) >= 8 && entry.Name()[len(entry.Name())-8:] == "_test.go" {
			continue
		}
		file, err := parser.ParseFile(files, filepath.Join(root, entry.Name()), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imported := range file.Imports {
			path := imported.Path.Value[1 : len(imported.Path.Value)-1]
			if _, exists := allowed[path]; !exists {
				t.Fatalf("production package acquired impure or unreviewed import %q in %s", path, entry.Name())
			}
		}
	}
}

func deepCopyBundle(t testing.TB, value Bundle) Bundle {
	t.Helper()
	raw, err := doccompiler.CanonicalJSONUnbounded(value)
	if err != nil {
		t.Fatal(err)
	}
	var result Bundle
	if err := jsonUnmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func jsonUnmarshal(raw []byte, target any) error {
	return json.Unmarshal(raw, target)
}

func hasDivergence(report VerificationReport, code string) bool {
	for _, divergence := range report.Divergences {
		if divergence.Code == code {
			return true
		}
	}
	return false
}
