package doccompiler

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

func admissionFixture() *contextindex.Index {
	sources := map[string]string{
		"docs/spec.md":               "# Spec\nPlans never write the repository.\n",
		"internal/plan/plan.go":      "package plan\n\nfunc Plan() error { return nil }\n",
		"internal/plan/plan_test.go": "package plan\n\nfunc TestPlan(t *testing.T) {}\n",
		"internal/plan/large.go":     "package plan\n// " + strings.Repeat("x", maxAnchorSpan-17) + "\n// y\n",
		"internal/plan/corrupt.go":   "package plan\n",
	}
	index := &contextindex.Index{CommitRevision: strings.Repeat("b", 40), Sources: map[string]contextindex.Source{}}
	for path, text := range sources {
		digest := sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(text), text)))
		index.Sources[path] = contextindex.Source{Path: path, BlobHash: hex.EncodeToString(digest[:]), Data: []byte(text), Mode: "100644"}
	}
	corrupt := index.Sources["internal/plan/corrupt.go"]
	corrupt.Data = []byte("package tampered\n")
	index.Sources["internal/plan/corrupt.go"] = corrupt
	return index
}

func anchorAt(index *contextindex.Index, path string, start, end int, authority string) Anchor {
	source := index.Sources[path]
	lines := strings.SplitAfter(string(source.Data), "\n")
	digest := sha256.Sum256([]byte(strings.Join(lines[start-1:end], "")))
	return Anchor{Path: path, Blob: source.BlobHash, StartLine: start, EndLine: end, SpanSHA256: hex.EncodeToString(digest[:]), Authority: authority, Reason: "states the clause"}
}

func admitOne(t *testing.T, index *contextindex.Index, clause Clause) Clause {
	t.Helper()
	admitted, err := AdmitClauses(index, []Clause{clause})
	if err != nil {
		t.Fatalf("AdmitClauses(%s) = %v", clause.ID, err)
	}
	return admitted[0]
}

func TestClauseAdmissionRefusesMalformedClausesHDCV0023(t *testing.T) {
	index := admissionFixture()
	valid := Clause{ID: "c1", State: ClauseUnknown, Kind: KindDescriptive, Text: "Plans are bounded.", Frontier: FrontierResolverUndecided}
	mutate := func(change func(*Clause)) Clause { clause := valid; change(&clause); return clause }
	refused := []struct {
		name    string
		clauses []Clause
		code    string
	}{
		{"empty ID", []Clause{mutate(func(c *Clause) { c.ID = "" })}, "invalid-clause"},
		{"whitespace ID", []Clause{mutate(func(c *Clause) { c.ID = "c 1" })}, "invalid-clause"},
		{"format-character ID", []Clause{mutate(func(c *Clause) { c.ID = "c\u200b1" })}, "invalid-clause"},
		{"oversized ID", []Clause{mutate(func(c *Clause) { c.ID = strings.Repeat("c", maxClauseIDBytes+1) })}, "invalid-clause"},
		{"empty text", []Clause{mutate(func(c *Clause) { c.Text = " " })}, "invalid-clause"},
		{"open state", []Clause{mutate(func(c *Clause) { c.State = "LIKELY" })}, "invalid-clause"},
		{"open kind", []Clause{mutate(func(c *Clause) { c.Kind = "NORMATIVE" })}, "invalid-clause"},
		{"open scope", []Clause{mutate(func(c *Clause) { c.Scope = "UNIVERSAL" })}, "invalid-clause"},
		{"UNKNOWN without frontier", []Clause{mutate(func(c *Clause) { c.Frontier = "" })}, "invalid-clause"},
		{"UNKNOWN open frontier", []Clause{mutate(func(c *Clause) { c.Frontier = "MODEL_UNSURE" })}, "invalid-clause"},
		{"SUPPORTED with frontier", []Clause{mutate(func(c *Clause) { c.State = ClauseSupported })}, "invalid-clause"},
		{"duplicate ID", []Clause{valid, valid}, "duplicate-clause"},
		{"clause limit+1", make([]Clause, maxClauses+1), "clause-limit-exceeded"},
		{"anchor limit+1", []Clause{mutate(func(c *Clause) { c.Anchors = make([]Anchor, maxAnchors+1) })}, "clause-limit-exceeded"},
	}
	for _, test := range refused {
		if _, err := AdmitClauses(index, test.clauses); errorCode(err) != test.code {
			t.Errorf("%s: code = %q, want %q", test.name, errorCode(err), test.code)
		}
	}
	admitted := admitOne(t, index, valid)
	if admitted.Scope != ScopeGeneral || admitted.State != ClauseUnknown || admitted.Frontier != FrontierResolverUndecided {
		t.Fatalf("undeclared scope or UNKNOWN frontier not preserved: %+v", admitted)
	}
}

func TestClauseAdmissionDisqualifiesAnchorsHDCV0023(t *testing.T) {
	index := admissionFixture()
	good := anchorAt(index, "internal/plan/plan.go", 3, 3, AuthorityPinnedSource)
	supported := func(anchors ...Anchor) Clause {
		return Clause{ID: "c1", State: ClauseSupported, Kind: KindDescriptive, Scope: ScopeObserved, Text: "Plan returns nil.", Anchors: anchors}
	}
	with := func(change func(*Anchor)) Anchor { anchor := good; change(&anchor); return anchor }
	if admitted := admitOne(t, index, supported(good)); admitted.State != ClauseSupported || admitted.Frontier != "" {
		t.Fatalf("qualifying anchor not admitted: %+v", admitted)
	}
	atLimit := anchorAt(index, "internal/plan/large.go", 1, 2, AuthorityPinnedSource)
	if admitted := admitOne(t, index, supported(atLimit)); admitted.State != ClauseSupported {
		t.Fatalf("8 KiB span refused: %+v", admitted)
	}
	disqualified := []struct {
		name     string
		anchor   Anchor
		frontier string
	}{
		{"missing reason", with(func(a *Anchor) { a.Reason = "" }), FrontierNoQualifyingSource},
		{"path-only", Anchor{Path: good.Path, Authority: AuthorityPinnedSource, Reason: "cited"}, FrontierNoQualifyingSource},
		{"line-only", Anchor{StartLine: 3, EndLine: 3, Authority: AuthorityPinnedSource, Reason: "cited"}, FrontierNoQualifyingSource},
		{"stale blob", with(func(a *Anchor) { a.Blob = strings.Repeat("d", 40) }), FrontierStaleAnchor},
		{"blob in another object format", with(func(a *Anchor) { a.Blob = strings.Repeat("d", 64) }), FrontierNoQualifyingSource},
		{"mutable blob", with(func(a *Anchor) { a.Blob = "HEAD" }), FrontierNoQualifyingSource},
		{"unpinned path", with(func(a *Anchor) { a.Path = "internal/plan/invented.go" }), FrontierNoQualifyingSource},
		{"index bytes differ from blob", anchorAt(index, "internal/plan/corrupt.go", 1, 1, AuthorityPinnedSource), FrontierNoQualifyingSource},
		{"out of range", with(func(a *Anchor) { a.StartLine, a.EndLine = 4, 4 }), FrontierNoQualifyingSource},
		{"reversed range", with(func(a *Anchor) { a.StartLine, a.EndLine = 3, 2 }), FrontierNoQualifyingSource},
		{"oversized span", anchorAt(index, "internal/plan/large.go", 1, 3, AuthorityPinnedSource), FrontierNoQualifyingSource},
		{"hash mismatch", with(func(a *Anchor) { a.SpanSHA256 = strings.Repeat("0", 64) }), FrontierNoQualifyingSource},
	}
	for _, test := range disqualified {
		admitted := admitOne(t, index, supported(test.anchor))
		if admitted.State != ClauseUnknown || admitted.Frontier != test.frontier || len(admitted.Anchors) != 1 {
			t.Errorf("%s: admitted %+v, want UNKNOWN %s keeping the anchor", test.name, admitted, test.frontier)
		}
	}
	conflicted := supported(good, good)
	conflicted.State = ClauseConflicted
	if admitted := admitOne(t, index, conflicted); admitted.State != ClauseUnknown {
		t.Fatalf("CONFLICTED on one location admitted: %+v", admitted)
	}
	conflicted.Anchors[1] = anchorAt(index, "internal/plan/plan.go", 1, 1, AuthorityPinnedSource)
	if admitted := admitOne(t, index, conflicted); admitted.State != ClauseConflicted || len(admitted.Anchors) != 2 {
		t.Fatalf("CONFLICTED on two distinct qualifying locations refused: %+v", admitted)
	}
}

func TestClauseAuthorityIsArtifactClassSpecificHDCV0024(t *testing.T) {
	index := admissionFixture()
	intent := anchorAt(index, "docs/spec.md", 2, 2, AuthorityAcceptedIntent)
	source := anchorAt(index, "internal/plan/plan.go", 3, 3, AuthorityPinnedSource)
	clause := func(state, kind string, anchors ...Anchor) Clause {
		return Clause{ID: "c1", State: state, Kind: kind, Text: "Plans never write the repository.", Anchors: anchors}
	}
	cases := []struct {
		name   string
		clause Clause
		want   string
	}{
		{"prescriptive on intent", clause(ClauseSupported, KindPrescriptive, intent), ClauseSupported},
		{"prescriptive on source", clause(ClauseSupported, KindPrescriptive, source), ClauseUnknown},
		{"descriptive on intent", clause(ClauseSupported, KindDescriptive, intent), ClauseUnknown},
		{"descriptive on source", clause(ClauseSupported, KindDescriptive, source), ClauseSupported},
		{"model output authority", clause(ClauseSupported, KindDescriptive, withAuthority(source, "MODEL_OUTPUT")), ClauseUnknown},
		{"generated documentation authority", clause(ClauseSupported, KindPrescriptive, withAuthority(intent, "GENERATED_DOCUMENTATION")), ClauseUnknown},
		{"intent/code drift as prescriptive", clause(ClauseConflicted, KindPrescriptive, intent, source), ClauseConflicted},
		{"intent/code drift as descriptive", clause(ClauseConflicted, KindDescriptive, intent, source), ClauseConflicted},
		{"code-only conflict on prescriptive", clause(ClauseConflicted, KindPrescriptive, source, anchorAt(index, "internal/plan/plan.go", 1, 1, AuthorityPinnedSource)), ClauseUnknown},
	}
	for _, test := range cases {
		admitted := admitOne(t, index, test.clause)
		if admitted.State != test.want || len(admitted.Anchors) != len(test.clause.Anchors) {
			t.Errorf("%s: admitted %+v, want %s with every anchor retained", test.name, admitted, test.want)
		}
	}
}

func withAuthority(anchor Anchor, authority string) Anchor {
	anchor.Authority = authority
	return anchor
}

func TestTestAndReceiptAnchorsCannotSupportGeneralClausesHDCV0025(t *testing.T) {
	index := admissionFixture()
	testAnchor := anchorAt(index, "internal/plan/plan_test.go", 3, 3, AuthorityPinnedTest)
	receipt := withAuthority(testAnchor, AuthorityExecutionReceipt)
	source := anchorAt(index, "internal/plan/plan.go", 3, 3, AuthorityPinnedSource)
	clause := func(scope string, anchors ...Anchor) Clause {
		return Clause{ID: "c1", State: ClauseSupported, Kind: KindDescriptive, Scope: scope, Text: "Plan never fails.", Anchors: anchors}
	}
	cases := []struct {
		name   string
		clause Clause
		want   string
	}{
		{"undeclared scope on test", clause("", testAnchor), ClauseUnknown},
		{"general on test and receipt", clause(ScopeGeneral, testAnchor, receipt), ClauseUnknown},
		{"observed on test", clause(ScopeObserved, testAnchor), ClauseSupported},
		{"observed on receipt", clause(ScopeObserved, receipt), ClauseSupported},
		{"general on source", clause(ScopeGeneral, testAnchor, source), ClauseSupported},
	}
	for _, test := range cases {
		if admitted := admitOne(t, index, test.clause); admitted.State != test.want {
			t.Errorf("%s: admitted %+v, want %s", test.name, admitted, test.want)
		}
	}
}

func TestRenderedProseIsAdmittedClauseTextAndTemplatesOnlyHDCV0026(t *testing.T) {
	index := admissionFixture()
	clauses := []Clause{
		{ID: "c2", State: ClauseSupported, Kind: KindDescriptive, Scope: ScopeObserved, Text: "Plan returns `nil` <b>now</b>.", Anchors: []Anchor{anchorAt(index, "internal/plan/plan.go", 3, 3, AuthorityPinnedSource)}},
		{ID: "c1", State: ClauseSupported, Kind: KindDescriptive, Text: "Plan is fast.", Anchors: []Anchor{anchorAt(index, "internal/plan/plan_test.go", 3, 3, AuthorityPinnedTest)}},
	}
	rendered, err := RenderAdmittedProse(index, clauses)
	if err != nil {
		t.Fatal(err)
	}
	blob := index.Sources["internal/plan/plan.go"].BlobHash
	want := "<!-- corvint-hdc-prose-templates/0 -->\n## Evidence and uncertainty\n" +
		"\n- Clause `\"c1\"` is **UNKNOWN** (DESCRIPTIVE, GENERAL, frontier NO_QUALIFYING_SOURCE): `\"Plan is fast.\"`\n" +
		"  - Anchor `\"internal/plan/plan_test.go\"` lines 3-3, blob `\"" + index.Sources["internal/plan/plan_test.go"].BlobHash + "\"`, authority `\"PINNED_TEST\"`\n" +
		"\n- Clause `\"c2\"` is **SUPPORTED** (DESCRIPTIVE, OBSERVED): ``\"Plan returns `nil` <b>now</b>.\"``\n" +
		"  - Anchor `\"internal/plan/plan.go\"` lines 3-3, blob `\"" + blob + "\"`, authority `\"PINNED_SOURCE\"`\n"
	if string(rendered) != want {
		t.Fatalf("rendered prose =\n%s\nwant\n%s", rendered, want)
	}
	if err := VerifyAdmittedProse(index, clauses, rendered); err != nil {
		t.Fatalf("exact rendering refused: %v", err)
	}
	candidates := map[string]string{
		"connective sentence": string(rendered) + "\nTherefore the plan is production ready.\n",
		"rewritten clause":    strings.Replace(string(rendered), "Plan is fast.", "Plan is always fast.", 1),
		"upgraded state":      strings.Replace(string(rendered), "**UNKNOWN**", "**SUPPORTED**", 1),
	}
	for name, candidate := range candidates {
		if code := errorCode(VerifyAdmittedProse(index, clauses, []byte(candidate))); code != "unadmitted-prose" {
			t.Errorf("%s: code = %q, want unadmitted-prose", name, code)
		}
	}
}
