package doccorpus

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// declarationSubjectOrders runs a search with include:["claims"] over the
// corpus read both ways (the query index derived after decoding, and from the
// blocks the read derived), asserts the two orders are equal, and returns one.
func declarationSubjectOrders(t *testing.T, root string, m Manifest, query string) []string {
	t.Helper()
	encoded, err := Encode(buildOwned(t, root, m))
	if err != nil {
		t.Fatal(err)
	}
	var orders [2][]string
	for n, index := range []bool{false, true} {
		read, err := OpenRead(context.Background(), root, encoded, ReadOptions{Index: index})
		if err != nil {
			t.Fatalf("index=%v: the built corpus should pass the verified read: %v", index, err)
		}
		orders[n] = searchOrders(t, read, Request{Operation: "search", Query: query, Include: []string{"claims"}, Limit: 50})
	}
	if !slices.Equal(orders[0], orders[1]) {
		t.Fatalf("the two reads order differently:\n%v\n%v", orders[0], orders[1])
	}
	return orders[0]
}

// assertClaimsBeforePathOnlyDeclarations checks that every descriptive claim
// is a hit, that no declaration subject matching only through its path is,
// and that every other declaration subject or claim ranks below every
// descriptive claim unless it is, or declares, one of named.
func assertClaimsBeforePathOnlyDeclarations(t *testing.T, order []string, prefix string, descriptive, pathOnly, named []string) {
	t.Helper()
	worst := 0
	for _, id := range descriptive {
		if rankOf(order, id) == 0 {
			t.Fatalf("descriptive claim %s is missing: %v", id, order)
		}
		worst = max(worst, rankOf(order, id))
	}
	for _, id := range pathOnly {
		if rankOf(order, id) != 0 {
			t.Fatalf("%s matches only through its path and should not be a hit: %v", id, order)
		}
	}
	for _, id := range named {
		if rankOf(order, id) == 0 {
			t.Fatalf("%s names the query words and should be a hit: %v", id, order)
		}
	}
	for n, id := range order {
		if strings.HasPrefix(id, prefix) && !slices.Contains(named, strings.TrimSuffix(id, ":declaration")) && n+1 < worst {
			t.Fatalf("declaration %s outranks a descriptive claim: %v", id, order)
		}
	}
}

// DCS-V1-574 (GitHub #1665 re-test of 6edb100, case "validate PLI lightspeed
// payments"): with include:["claims"], a native declaration subject is ranked
// by its symbol name and its claims' text, not by the file path its ID
// spells, so the `call` and `club` declarations, which hold every query word
// only through their path, are not hits and the descriptive claims order
// above every declaration subject.
func TestDCSV1574DeclarationSubjectsRankBySymbolNotPath(t *testing.T) {
	t.Parallel()
	root, m := ownedSourceCorpus(t, declarationClaimSources, []string{"src/lightspeed_payments/validate_pli.rb"}, false, declarationClaimTexts...)
	order := declarationSubjectOrders(t, root, m, "validate PLI lightspeed payments")
	assertClaimsBeforePathOnlyDeclarations(t, order, declarationClaimPrefix,
		[]string{"adapter:owner:claim-1", "adapter:owner:claim-2", "adapter:owner:claim-3"},
		[]string{declarationClaimPrefix + "2:call", declarationClaimPrefix + "4:club"},
		nil)
}

var adminDeclarationSources = map[string]string{
	"src/admin/set_admin_user.rb": "module Admin\n  class SetAdminUser\n    def set_admin_user\n    end\n    def user\n    end\n    def call\n    end\n  end\nend\n",
}

const adminDeclarationPrefix = "native:symbol:src/admin/set_admin_user.rb:"

// DCS-V1-574 (GitHub #1665 re-test of 6edb100, case "set admin user"): the
// `user`, `Admin` and `call` declarations hold the query words only through
// their path and do not outrank the descriptive claim, while `SetAdminUser`
// and `set_admin_user`, whose names spell the query, stay hits.
func TestDCSV1574AdminDeclarationsDoNotCrowdOutTheClaim(t *testing.T) {
	t.Parallel()
	root, m := ownedSourceCorpus(t, adminDeclarationSources, []string{"src/admin/set_admin_user.rb"}, false,
		"Support staff set the admin flag on a user account from the console")
	order := declarationSubjectOrders(t, root, m, "set admin user")
	assertClaimsBeforePathOnlyDeclarations(t, order, adminDeclarationPrefix,
		[]string{"adapter:owner:claim-1"},
		[]string{adminDeclarationPrefix + "7:call"},
		[]string{adminDeclarationPrefix + "2:SetAdminUser", adminDeclarationPrefix + "3:set_admin_user"})
	for _, id := range []string{adminDeclarationPrefix + "1:Admin", adminDeclarationPrefix + "5:user"} {
		if rankOf(order, id) != 0 && rankOf(order, id) < rankOf(order, "adapter:owner:claim-1") {
			t.Fatalf("%s holds one query word by its name and should not outrank the claim: %v", id, order)
		}
	}
}

// DCS-V1-574 parity with DCS-V1-570: a query naming the declared symbol still
// finds its declaration subject in a search with include:["claims"].
func TestDCSV1574DeclarationSubjectRanksWhenTheQueryNamesTheSymbol(t *testing.T) {
	t.Parallel()
	root, m := ownedSourceCorpus(t, declarationClaimSources, []string{"src/lightspeed_payments/validate_pli.rb"}, false, declarationClaimTexts...)
	for query, want := range map[string]string{
		"club":        declarationClaimPrefix + "4:club",
		"call":        declarationClaimPrefix + "2:call",
		"ValidatePli": declarationClaimPrefix + "1:ValidatePli",
	} {
		if order := declarationSubjectOrders(t, root, m, query); rankOf(order, want) != 1 {
			t.Fatalf("search %q: the declaration subject should rank first: %v", query, order)
		}
	}
}

// DCS-V1-574: without include, search ranking is unchanged, so a declaration
// subject still matches through its ID's path terms.
func TestDCSV1574SubjectOnlySearchKeepsIDTerms(t *testing.T) {
	t.Parallel()
	root, m := ownedSourceCorpus(t, declarationClaimSources, []string{"src/lightspeed_payments/validate_pli.rb"}, false, declarationClaimTexts...)
	order := searchOrders(t, buildOwned(t, root, m), Request{Operation: "search", Query: "validate PLI lightspeed payments", Limit: 50})
	if rankOf(order, declarationClaimPrefix+"2:call") == 0 {
		t.Fatalf("a subject-only search should still match a declaration through its ID: %v", order)
	}
}
