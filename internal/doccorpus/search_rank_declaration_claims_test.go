package doccorpus

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// declarationClaimSources reproduces the GitHub #1665 re-test of b63dd0d:
// a records provider owns a Ruby file whose path spells the query words, so
// the native provider derives declaration claims (`def call`, `def club`,
// `def pli`) whose IDs, and only their IDs, hold every query word.
var declarationClaimSources = map[string]string{
	"src/lightspeed_payments/validate_pli.rb": "class ValidatePli\n  def call\n  end\n  def club\n  end\n  def pli\n  end\nend\n",
}

var declarationClaimTexts = []string{
	"Lightspeed payments are validated before the batch posts",
	"The PLI check validates each Lightspeed payment",
	"An invalid PLI amount blocks Lightspeed payments",
}

func declarationClaimsCorpus(t *testing.T) *Artifact {
	t.Helper()
	root, m := ownedSourceCorpus(t, declarationClaimSources, []string{"src/lightspeed_payments/validate_pli.rb"}, false, declarationClaimTexts...)
	a := buildOwned(t, root, m)
	encoded, err := Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	read, err := OpenRead(context.Background(), root, encoded, ReadOptions{})
	if err != nil {
		t.Fatalf("the built corpus should pass the verified read: %v", err)
	}
	return read
}

const declarationClaimPrefix = "native:symbol:src/lightspeed_payments/validate_pli.rb:"

// DCS-V1-570: a native declaration claim is ranked by its text and the
// symbol it declares, not by the file path its ID spells, so in both the
// claims operation and docs_search with include:["claims"] the descriptive
// claims holding the query words order above every declaration claim, and a
// declaration matching only through its path is not a hit.
func TestDCSV1570DeclarationClaimsRankBySymbolNotPath(t *testing.T) {
	t.Parallel()
	a := declarationClaimsCorpus(t)
	if got := len(subjectsWithPrefix(a, declarationClaimPrefix)); got != 4 {
		t.Fatalf("the fixture should derive four declarations: %d", got)
	}
	query := "validate PLI lightspeed payments"
	descriptive := []string{"adapter:owner:claim-1", "adapter:owner:claim-2", "adapter:owner:claim-3"}
	order := claimOrder(t, a, query)
	if len(order) < 3 || !sameSet(order[:3], descriptive) {
		t.Fatalf("claims: the descriptive claims should order first: %v", order)
	}
	for _, id := range []string{declarationClaimPrefix + "2:call:declaration", declarationClaimPrefix + "4:club:declaration"} {
		if slices.Contains(order, id) {
			t.Fatalf("claims: %s matches only through its path: %v", id, order)
		}
	}
	search := searchOrders(t, a, Request{Operation: "search", Query: query, Include: []string{"claims"}, Limit: 50})
	worst := 0
	for _, id := range descriptive {
		if rankOf(search, id) == 0 {
			t.Fatalf("search: %s is missing: %v", id, search)
		}
		worst = max(worst, rankOf(search, id))
	}
	for _, id := range search {
		if strings.HasSuffix(id, ":declaration") && rankOf(search, id) < worst {
			t.Fatalf("search: declaration claim %s outranks a descriptive claim: %v", id, search)
		}
		if id == declarationClaimPrefix+"2:call:declaration" || id == declarationClaimPrefix+"4:club:declaration" {
			t.Fatalf("search: %s matches only through its path: %v", id, search)
		}
	}
}

// DCS-V1-570: a query naming the declared symbol still ranks its declaration
// claim first, by the symbol name and the declaration line.
func TestDCSV1570DeclarationClaimRanksWhenTheQueryNamesTheSymbol(t *testing.T) {
	t.Parallel()
	a := declarationClaimsCorpus(t)
	for query, want := range map[string]string{
		"club":        declarationClaimPrefix + "4:club:declaration",
		"call":        declarationClaimPrefix + "2:call:declaration",
		"ValidatePli": declarationClaimPrefix + "1:ValidatePli:declaration",
	} {
		if order := claimOrder(t, a, query); len(order) == 0 || order[0] != want {
			t.Fatalf("claims %q: the declaration claim should rank first: %v", query, order)
		}
		if order := searchOrders(t, a, Request{Operation: "search", Query: query, Include: []string{"claims"}, Limit: 50}); rankOf(order, want) == 0 {
			t.Fatalf("search %q: the declaration claim should be a hit: %v", query, order)
		}
	}
}

// DCS-V1-570 (Codex review): only a `kind: native` provider's declaration
// claims rank by their symbol. A records provider may supply a claim of the
// same shape, its subject's ID plus ":declaration" with one anchor naming a
// symbol, and that claim keeps its ID terms, both when the index is derived
// from the decoded artifact and from the blocks the read derived.
func TestDCSV1570RecordsDeclarationShapedClaimKeepsItsIDTerms(t *testing.T) {
	t.Parallel()
	const id = "adapter:payments:declaration"
	root, m := ownedSourceCorpusWith(t, declarationClaimSources, []string{"src/lightspeed_payments/validate_pli.rb"}, false, func(r *ProviderRecord) {
		ev := r.Subjects[0].Evidence
		ev.Anchors = slices.Clone(ev.Anchors[:1])
		ev.Anchors[0].Symbol = "ValidatePli"
		r.Subjects = append(r.Subjects, Subject{"adapter:payments", "capability", "Settlement", "adapter", r.Subjects[0].Evidence, nil})
		r.Claims = append(r.Claims, Claim{id, "adapter:payments", "class ValidatePli", "adapter", ev})
	})
	a := buildOwned(t, root, m)
	encoded, err := Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range []bool{false, true} {
		read, err := OpenRead(context.Background(), root, encoded, ReadOptions{Index: index})
		if err != nil {
			t.Fatalf("index=%v: the built corpus should pass the verified read: %v", index, err)
		}
		if order := claimOrder(t, read, "payments"); !slices.Contains(order, id) {
			t.Fatalf("index=%v: claims: the records claim should match through its ID: %v", index, order)
		}
		if order := searchOrders(t, read, Request{Operation: "search", Query: "payments", Include: []string{"claims"}, Limit: 50}); rankOf(order, id) == 0 {
			t.Fatalf("index=%v: search: the records claim should match through its ID: %v", index, order)
		}
	}
}

func sameSet(got, want []string) bool {
	return len(got) == len(want) && !slices.ContainsFunc(want, func(id string) bool { return !slices.Contains(got, id) })
}
