package frontier

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/wp3codec"
)

// openScenario is one linked obligation over one closing hunk plus one CEM
// unknown hunk: the smallest fixture that emits all three item kinds.
func openScenario() scenario {
	return scenario{
		evidence: []evidenceSpec{closingEvidence},
		hunks: []hunkSpec{
			{name: "u", disposition: CEMUnknown, reason: "no-evidence", newPath: "src/u.go"},
			closingHunk("h"),
		},
		obligations: []obligationSpec{{
			id: "CFTEST-001", disposition: OCMLinked, statement: "the widgetregistry accepts entries",
			hunks: []string{"h"}, claims: []string{"c1"},
		}},
		claims: map[string][]TCQClaimResult{
			"CFTEST-001": {callerReported("CFTEST-001", "c1", "test-not-matched")},
		},
	}
}

// CF-V0-017, CF-V0-018 and CF-V0-019: the complete result has exactly the
// closed top-level, inputs, scope, and item key sets, and is codec bytes plus
// exactly one LF.
func TestCanonicalDocumentShape(t *testing.T) {
	_, encoded := openScenario().run(t)
	if encoded[len(encoded)-1] != '\n' || strings.Count(string(encoded), "\n") != 1 {
		t.Fatal("a complete document is codec bytes plus exactly one LF")
	}
	root, err := wp3codec.Parse(encoded[:len(encoded)-1])
	if err != nil {
		t.Fatalf("document is not canonical codec bytes: %v", err)
	}
	if !equalStrings(root.Keys(), []string{"frontierState", "id", "inputs", "items", "profile", "scope", "universeId"}) {
		t.Fatalf("top-level keys %v are not the closed CF-V0-018 set", root.Keys())
	}
	inputs, _ := root.Lookup("inputs")
	if !equalStrings(inputs.Keys(), []string{"cemSha256", "lrfSha256", "ocmSha256", "policy", "tcqId", "testMode"}) {
		t.Fatalf("inputs keys %v are not the closed CF-V0-018 set", inputs.Keys())
	}
	scope, _ := root.Lookup("scope")
	want := []string{"baseRevision", "excludedPath", "intentBlobOid", "intentPath", "intentSpan",
		"intentSpanSha256", "objectFormat", "patchSha256", "targetRevision"}
	if !equalStrings(scope.Keys(), want) {
		t.Fatalf("scope keys %v are not the closed CF-V0-018 set", scope.Keys())
	}
	span, _ := scope.Lookup("intentSpan")
	for _, key := range []string{"end", "start"} {
		offset, _ := span.Lookup(key)
		if offset.Kind() != wp3codec.KindString {
			t.Fatalf("intent %s must be a canonical decimal string, not a JSON number", key)
		}
	}
	items, _ := root.Lookup("items")
	for _, item := range items.Items() {
		if !equalStrings(item.Keys(), []string{"authorityClass", "id", "kind", "nextAction", "reasons",
			"relatedIds", "resolutionClass", "subjectId"}) {
			t.Fatalf("item keys %v are not the closed CF-V0-017 set", item.Keys())
		}
	}
	if err := VerifyDocument(encoded); err != nil {
		t.Fatalf("own output failed verification: %s", err.Code)
	}
}

// CF-V0-006 and CF-V0-007: the identities are reproduced here from the clause
// formulas, with the preimages written out by hand rather than through the
// package's own codec, so a change to either the preimage or the codec breaks
// this vector.
func TestIdentityVectorsMatchTheClauseFormulas(t *testing.T) {
	document, _ := openScenario().run(t)
	scope := document.Scope
	preimage := `{"baseRevision":"` + scope.BaseRevision +
		`","excludedPath":"` + ExcludedPath +
		`","intent":{"blobOid":"` + scope.IntentBlobOID +
		`","path":"` + scope.IntentPath +
		`","span":{"end":"` + strconv.FormatInt(scope.IntentSpan.End, 10) +
		`","start":"` + strconv.FormatInt(scope.IntentSpan.Start, 10) +
		`"},"spanSha256":"` + scope.IntentSpanSHA256 +
		`"},"objectFormat":"` + scope.ObjectFormat +
		`","patchSha256":"` + scope.PatchSHA256 +
		`","targetRevision":"` + scope.TargetRevision + `"}`
	wantUniverse := universeIDPrefix + domainHash("corvint-frontier-universe/0", preimage)
	if document.UniverseID != wantUniverse {
		t.Fatalf("universe id %s, want %s", document.UniverseID, wantUniverse)
	}
	for _, item := range document.Items {
		itemPreimage := `{"kind":"` + item.Kind + `","subjectId":"` + item.SubjectID +
			`","universeId":"` + document.UniverseID + `"}`
		want := itemIDPrefix + domainHash("corvint-frontier-item/0", itemPreimage)
		if item.ID != want {
			t.Fatalf("item id %s, want %s", item.ID, want)
		}
	}
}

// CF-V0-006 and CF-V0-007: a syntactically valid nested ID cannot be accepted
// merely because the outer document ID was recomputed around the forgery.
func TestVerifyDocumentRebindsNestedIdentities(t *testing.T) {
	document, _ := openScenario().run(t)
	for _, testCase := range []struct {
		name  string
		forge func(*Document)
	}{
		{
			name: "universe id",
			forge: func(candidate *Document) {
				candidate.UniverseID = universeIDPrefix + strings.Repeat("0", 64)
				for index := range candidate.Items {
					candidate.Items[index].ID, _ = ItemID(candidate.Items[index].Kind, candidate.Items[index].SubjectID, candidate.UniverseID)
				}
			},
		},
		{
			name: "item id",
			forge: func(candidate *Document) {
				candidate.Items[0].ID = itemIDPrefix + strings.Repeat("0", 64)
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			candidate := document
			candidate.Items = append([]Item(nil), document.Items...)
			testCase.forge(&candidate)
			candidate.ID, _ = documentID(candidate)
			encoded, err := CanonicalBytes(candidate)
			if err != nil {
				t.Fatal(err)
			}
			if verifyErr := VerifyDocument(encoded); verifyErr == nil || verifyErr.Code != CodeNoncanonical {
				t.Fatalf("verification returned %v, want %s", verifyErr, CodeNoncanonical)
			}
		})
	}
}

// CF-V0-009..012 and CF-V0-015: each item kind/reason fixes its authority;
// another admitted enum value cannot be authenticated by the outer document ID.
func TestVerifyDocumentRejectsForgedItemAuthority(t *testing.T) {
	document, _ := openScenario().run(t)
	for index, authority := range []string{AuthorityCallerReported, AuthorityNone, AuthorityProducerDeclared} {
		candidate := document
		candidate.Items = append([]Item(nil), document.Items...)
		candidate.Items[index].AuthorityClass = authority
		candidate.ID, _ = documentID(candidate)
		encoded, err := CanonicalBytes(candidate)
		if err != nil {
			t.Fatal(err)
		}
		if verifyErr := VerifyDocument(encoded); verifyErr == nil || verifyErr.Code != CodeNoncanonical {
			t.Fatalf("item %d verification returned %v, want %s", index, verifyErr, CodeNoncanonical)
		}
	}
}

// CF-V0-018: scope is verified Git/OCM identity, not arbitrary strings that
// become acceptable when every enclosing identity is recomputed.
func TestVerifyDocumentRejectsNoncanonicalScopeIdentity(t *testing.T) {
	document, _ := openScenario().run(t)
	for _, testCase := range []struct {
		name  string
		forge func(*Scope)
	}{
		{"dash-led revision", func(scope *Scope) { scope.BaseRevision = "-base" }},
		{"traversal intent path", func(scope *Scope) { scope.IntentPath = "docs/../intent.md" }},
		{"object-format width mismatch", func(scope *Scope) { scope.IntentBlobOID = strings.Repeat("0", 64) }},
		{"empty intent span", func(scope *Scope) { scope.IntentSpan.End = scope.IntentSpan.Start }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			candidate := document
			candidate.Items = append([]Item(nil), document.Items...)
			testCase.forge(&candidate.Scope)
			candidate.UniverseID, _ = UniverseID(candidate.Scope)
			for index := range candidate.Items {
				candidate.Items[index].ID, _ = ItemID(candidate.Items[index].Kind, candidate.Items[index].SubjectID, candidate.UniverseID)
			}
			candidate.ID, _ = documentID(candidate)
			encoded, err := CanonicalBytes(candidate)
			if err != nil {
				t.Fatal(err)
			}
			if verifyErr := VerifyDocument(encoded); verifyErr == nil || verifyErr.Code != CodeNoncanonical {
				t.Fatalf("verification returned %v, want %s", verifyErr, CodeNoncanonical)
			}
		})
	}
}

func domainHash(domain, preimage string) string {
	hasher := sha256.New()
	hasher.Write([]byte(domain))
	hasher.Write([]byte{0})
	hasher.Write([]byte(preimage))
	return hex.EncodeToString(hasher.Sum(nil))
}

// CF-V0-006 and CF-V0-007: repairing evidence removes items but keeps the
// universe and the surviving item identities stable, because map, LRF and TCQ
// identities deliberately stay out of the universe preimage.
func TestEvidenceRepairKeepsIdentitiesStable(t *testing.T) {
	before := openScenario()
	before.hunks = append(before.hunks, hunkSpec{
		name: "broken", disposition: CEMSupported, reason: "evidence-backed",
		body: "widgetregistry accepts entries", newPath: "src/broken.go",
		bases: []string{"unrelated"},
	})
	before.evidence = append(before.evidence,
		evidenceSpec{name: "unrelated", path: "docs/other.md", body: "gizmocatalog rules"})

	after := openScenario()
	after.hunks = append(after.hunks, closingHunk("broken"))

	first, _ := before.run(t)
	second, _ := after.run(t)
	if first.UniverseID != second.UniverseID {
		t.Fatal("evidence repair must not change the universe id")
	}
	if len(itemsOfKind(first, KindHunkBasis)) != 2 || len(itemsOfKind(second, KindHunkBasis)) != 1 {
		t.Fatal("the repaired hunk item should disappear and the unresolved one remain")
	}
	if findItem(t, first, KindHunkBasis, hunkRef("u")).ID != findItem(t, second, KindHunkBasis, hunkRef("u")).ID {
		t.Fatal("an unresolved obligation must keep its identity across evidence repair")
	}
}

// CF-V0-006: changing patch or intent identity yields a new universe, and
// therefore new item IDs for the same subjects.
func TestPatchOrIntentChangeYieldsNewIdentities(t *testing.T) {
	baseline, _ := openScenario().run(t)
	cases := []struct {
		name  string
		alter func(*scenario)
	}{
		{"patch identity", func(s *scenario) { s.patchSeed = "different-patch" }},
		{"intent path", func(s *scenario) { s.intentPath = "docs/specs/other-spec.md" }},
		{"intent span", func(s *scenario) { s.intentSpan = Span{Start: 1, End: 900} }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			altered := openScenario()
			testCase.alter(&altered)
			changed, _ := altered.run(t)
			if changed.UniverseID == baseline.UniverseID {
				t.Fatal("a changed patch or intent identity must yield a new universe id")
			}
			if changed.Items[0].ID == baseline.Items[0].ID {
				t.Fatal("a new universe must yield new item ids for the same subject")
			}
		})
	}
}

// CF-V0-020 and the "reordered caller arrays and two fresh processes" matrix
// row: caller array order can never reach the output, and recomputation is
// byte-identical.
func TestReorderedCallerArraysAreByteIdentical(t *testing.T) {
	ordered := openScenario()
	ordered.obligations[0].claims = []string{"c1", "c2"}
	ordered.claims["CFTEST-001"] = []TCQClaimResult{
		callerReported("CFTEST-001", "c1", "test-not-matched"),
		callerReported("CFTEST-001", "c2", "empty-body"),
	}
	reordered := openScenario()
	reordered.obligations[0].claims = []string{"c2", "c1"}
	reordered.claims["CFTEST-001"] = []TCQClaimResult{
		callerReported("CFTEST-001", "c2", "empty-body"),
		callerReported("CFTEST-001", "c1", "test-not-matched"),
	}

	_, first := ordered.run(t)
	_, second := reordered.run(t)
	_, again := ordered.run(t)
	if string(first) != string(second) {
		t.Fatal("reordered caller arrays must produce byte-identical output")
	}
	if string(first) != string(again) {
		t.Fatal("two fresh computations must produce byte-identical output")
	}
}

// CF-V0-018 and the "OPEN with empty items, EMPTY with items, or any extra
// stop-decision field" matrix row: all three are `noncanonical-frontier`.
func TestVerifyDocumentRejectsNoncanonicalDocuments(t *testing.T) {
	_, open := openScenario().run(t)
	_, empty := scenario{
		hunks: []hunkSpec{{name: "ws", disposition: CEMMechanical, reason: "whitespace-only", newPath: "src/ws.go"}},
	}.run(t)

	cases := []struct {
		name string
		data []byte
	}{
		{"extra stop-decision field", []byte(strings.Replace(string(open),
			`,"universeId"`, `,"stopDecision":"ALLOW","universeId"`, 1))},
		{"empty state with items", []byte(strings.Replace(string(open),
			`"frontierState":"OPEN"`, `"frontierState":"EMPTY"`, 1))},
		{"open state with no items", []byte(strings.Replace(string(empty),
			`"frontierState":"EMPTY"`, `"frontierState":"OPEN"`, 1))},
		{"mutated identity", []byte(strings.Replace(string(open),
			documentIDPrefix, documentIDPrefix+"0", 1))},
		{"missing terminal lf", open[:len(open)-1]},
		{"numeric intent offset", []byte(strings.Replace(string(open),
			`"intentSpan":{"end":"512"`, `"intentSpan":{"end":512`, 1))},
		{"intent offset wrapping int64 onto the bound value", []byte(strings.Replace(string(open),
			`"intentSpan":{"end":"512"`, `"intentSpan":{"end":"18446744073709552128"`, 1))},
		{"unsorted keys", []byte(strings.Replace(string(open),
			`{"frontierState"`, `{"zzz":"x","frontierState"`, 1))},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := VerifyDocument(testCase.data)
			if err == nil || err.Code != CodeNoncanonical {
				t.Fatalf("verification returned %v, want %s", err, CodeNoncanonical)
			}
		})
	}
}

// CF-V0-023: every Frontier-owned count fails closed at limit plus one, with no
// partial result, and the limit itself still produces a complete result.
func TestFrontierLimits(t *testing.T) {
	relatedScenario := func(count int) scenario {
		built := scenario{claims: map[string][]TCQClaimResult{
			"CFTEST-001": {callerReported("CFTEST-001", "c1", "test-not-matched")},
		}}
		names := []string{}
		for index := 0; index < count; index++ {
			name := "h" + strconv.Itoa(index)
			built.hunks = append(built.hunks, hunkSpec{
				name: name, disposition: CEMUnknown, reason: "no-evidence", newPath: "src/" + name + ".go",
			})
			names = append(names, name)
		}
		built.obligations = []obligationSpec{{
			id: "CFTEST-001", disposition: OCMLinked, statement: "the gizmocatalog rejects duplicates",
			hunks: names, claims: []string{"c1"},
		}}
		return built
	}
	obligationScenario := func(count int) scenario {
		built := scenario{
			evidence: []evidenceSpec{closingEvidence},
			hunks:    []hunkSpec{closingHunk("h")},
			claims:   map[string][]TCQClaimResult{},
		}
		for index := 0; index < count; index++ {
			id := "CFTEST-" + pad(index)
			built.obligations = append(built.obligations, obligationSpec{
				id: id, disposition: OCMLinked, statement: "the gizmocatalog rejects duplicates",
				hunks: []string{"h"}, claims: []string{"c1"},
			})
			built.claims[id] = []TCQClaimResult{callerReported(id, "c1", "test-not-matched")}
		}
		return built
	}
	hunkScenario := func(count int) scenario {
		built := scenario{}
		for index := 0; index < count; index++ {
			name := "h" + strconv.Itoa(index)
			built.hunks = append(built.hunks, hunkSpec{
				name: name, disposition: CEMUnknown, reason: "no-evidence", newPath: "src/" + name + ".go",
			})
		}
		return built
	}

	cases := []struct {
		name    string
		at      scenario
		beyond  scenario
		atItems int
	}{
		{"related ids", relatedScenario(MaxRelatedIDsPerItem), relatedScenario(MaxRelatedIDsPerItem + 1), 0},
		{"intent change items", obligationScenario(MaxIntentChangeItems), obligationScenario(MaxIntentChangeItems + 1), 0},
		{"hunk items", hunkScenario(MaxHunkItems), hunkScenario(MaxHunkItems + 1), MaxHunkItems},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, _, err := testCase.at.compute(t); err != nil {
				t.Fatalf("the limit itself must produce a complete result, got %q", CodeOf(err))
			}
			_, encoded, err := testCase.beyond.compute(t)
			if CodeOf(err) != CodeResourceExhausted {
				t.Fatalf("code %q, want %q", CodeOf(err), CodeResourceExhausted)
			}
			if encoded != nil {
				t.Fatal("exhaustion must leave no partial frontier result")
			}
		})
	}
}

func pad(value int) string {
	text := strconv.Itoa(value)
	for len(text) < 3 {
		text = "0" + text
	}
	return text
}

// CF-V0-026 and CF-V0-025: JSON and human are two renderings of one
// computation, and the human rendering carries the assertion boundary.
func TestHumanAndJSONRenderOneComputation(t *testing.T) {
	document, encoded := openScenario().run(t)
	rendered, err := RenderJSON(document)
	if err != nil {
		t.Fatalf("json rendering failed: %v", err)
	}
	if string(rendered) != string(encoded) {
		t.Fatal("RenderJSON must reproduce the sealed bytes exactly")
	}
	human := RenderHuman(document)
	if !strings.Contains(human, assertionBoundary) {
		t.Fatal("the human rendering must carry the CF-V0-025 assertion boundary")
	}
	for _, item := range document.Items {
		if !strings.Contains(human, item.SubjectID) || !strings.Contains(human, item.NextAction) {
			t.Fatal("the human rendering must cover every item of the same computation")
		}
	}
	if strings.Contains(human, "stop") {
		t.Fatal("no rendering may present a stop decision")
	}
}

// CF-V0-004: exit 0 means valid empty and exit 1 means valid open; operational
// failure exits 2 with no document at all.
func TestExitCodeLaw(t *testing.T) {
	open, _ := openScenario().run(t)
	if open.ExitCode() != 1 {
		t.Fatalf("open exit %d, want 1", open.ExitCode())
	}
	empty, _ := scenario{
		hunks: []hunkSpec{{name: "ws", disposition: CEMMechanical, reason: "line-ending-only", newPath: "src/ws.go"}},
	}.run(t)
	if empty.ExitCode() != 0 {
		t.Fatalf("empty exit %d, want 0", empty.ExitCode())
	}
	request := baseRequest(t)
	request.CEMBytes = []byte(`{"spec":"cem/0.1"}`)
	if _, _, err := Compute(context.Background(), request); err == nil || ErrorExitCode != 2 {
		t.Fatal("operational failure exits 2 with no frontier json")
	}
}
