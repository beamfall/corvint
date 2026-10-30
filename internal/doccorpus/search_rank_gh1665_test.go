package doccorpus

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"
)

// claimOrder returns the docs_claims order for a query.
func claimOrder(t *testing.T, a *Artifact, query string) []string {
	t.Helper()
	var order []string
	for _, v := range queryResults(t, a, Request{Operation: "claims", Query: query}).Results {
		order = append(order, recordID(v))
	}
	return order
}

// DCX-V1-475 (GitHub #1665 rows 1 and 4): a subject whose name field holds
// the query's words in order outranks partial overlaps. The identifier
// logout_redirect_url, also written LogoutRedirectURL, finds the subject that
// quotes it in a prose name ahead of one named lsid_error_redirect_url, and
// the phrase "on deck" finds the controller file whose path segment is
// on_deck ahead of short names that only hold "deck".
func TestDCXV1475PhraseAndIdentifierMatchesRankFirst(t *testing.T) {
	t.Parallel()
	a := buildAdoption(t, func(p *ProviderRecord) {
		ev := p.Subjects[0].Evidence
		p.Subjects = append(p.Subjects,
			Subject{ID: p.ID + ":aa-lsid", Kind: "use_case", Name: "lsid_error_redirect_url setting", Provider: p.ID, Evidence: ev},
			Subject{ID: p.ID + ":zz-logout", Kind: "document", Name: "The sign out page sends the member to the logout_redirect_url configured for the club when it is present and to the home page when it is blank", Provider: p.ID, Evidence: ev},
			Subject{ID: p.ID + ":zz-on-deck", Kind: "module", Name: "app/controllers/admin/on_deck_controller.rb", Provider: p.ID, Evidence: ev},
			Subject{ID: p.ID + ":aa-deck", Kind: "use_case", Name: "Deck", Provider: p.ID, Evidence: ev},
			Subject{ID: p.ID + ":ab-deck", Kind: "data_entity", Name: "deck slot", Provider: p.ID, Evidence: ev},
			Subject{ID: p.ID + ":ac-deck", Kind: "use_case", Name: "deck on hold", Provider: p.ID, Evidence: ev},
			Subject{ID: p.ID + ":zz-as-is", Kind: "module", Name: "app/as_is.rb", Provider: p.ID, Evidence: ev},
		)
		for i := range 8 {
			p.Subjects = append(p.Subjects, Subject{ID: fmt.Sprintf("%s:filler-%02d", p.ID, i), Kind: "use_case", Name: fmt.Sprintf("redirect url %02d", i), Provider: p.ID, Evidence: ev})
		}
	})
	for _, query := range []string{"logout_redirect_url", "LogoutRedirectURL"} {
		order := searchOrders(t, a, Request{Operation: "search", Query: query})
		if len(order) == 0 || order[0] != "adapter:zz-logout" {
			t.Fatalf("%s: the subject quoting the whole identifier should rank first: %v", query, order)
		}
		if rankOf(order, "adapter:aa-lsid") < 2 {
			t.Fatalf("%s: the partial overlap should still match: %v", query, order)
		}
	}
	order := searchOrders(t, a, Request{Operation: "search", Query: "on deck"})
	if len(order) == 0 || order[0] != "adapter:zz-on-deck" {
		t.Fatalf("the on_deck path segment should rank first for the phrase: %v", order)
	}
	// A phrase of stop words scores no term, and still finds the subject
	// whose path holds it (Codex review of #1669).
	if got := searchOrders(t, a, Request{Operation: "search", Query: "as is"}); len(got) == 0 || got[0] != "adapter:zz-as-is" {
		t.Fatalf("a stop-word phrase should find app/as_is.rb on its own: %v", got)
	}
	if got := searchOrders(t, a, Request{Operation: "search", Query: "as is", Kinds: []string{"use_case"}}); slices.Contains(got, "adapter:zz-as-is") {
		t.Fatalf("a phrase-only candidate should still obey the kind filter: %v", got)
	}
	// Order matters, and a prose name only matches a one-identifier query.
	if got := searchOrders(t, a, Request{Operation: "search", Query: "deck on"}); len(got) > 0 && got[0] == "adapter:zz-on-deck" {
		t.Fatalf("a reversed phrase should not phrase-match: %v", got)
	}
	prose := Subject{ID: "adapter:zz-logout", Kind: "document", Name: "The sign out page sends the member to the logout_redirect_url configured for the club when it is present and to the home page when it is blank"}
	if phraseMatch(prose, []string{"sends", "the", "member"}, false) || !phraseMatch(prose, []string{"logout", "redirect", "url"}, true) {
		t.Fatal("a prose name should match only a one-identifier query")
	}
	for text, want := range map[string][]string{
		"LogoutRedirectURL":             {"logout", "redirect", "url"},
		"logout_redirect_url":           {"logout", "redirect", "url"},
		"app/controllers/on_deck.rb":    {"app", "controllers", "on", "deck", "rb"},
		"HTTPServer2Ready":              {"http", "server2", "ready"},
		"  ":                            nil,
		"claim:2004 on-deck_Controller": {"claim", "2004", "on", "deck", "controller"},
	} {
		if got := identifierWords(text); !slices.Equal(got, want) {
			t.Errorf("identifierWords(%q) = %v, want %v", text, got, want)
		}
	}
}

// DCX-V1-476 (GitHub #1665 row 2): claim text matches regular inflections
// of a query term, so "set admin user" finds "Setting an admin ..." and
// "revoke admin user" finds "Revoking ...", through docs_claims and through
// docs_search with include:["claims"].
func TestDCXV1476ClaimTextMatchesInflections(t *testing.T) {
	t.Parallel()
	a := buildAdoption(t, func(p *ProviderRecord) {
		ev := p.Subjects[0].Evidence
		subject := p.Subjects[0].ID
		p.Claims = append(p.Claims,
			Claim{ID: p.ID + ":zz-setting", Subject: subject, Text: "Setting an admin creates a manager employment", Provider: p.ID, Evidence: ev},
			Claim{ID: p.ID + ":zz-revoking", Subject: subject, Text: "Revoking an admin removes the manager employment", Provider: p.ID, Evidence: ev},
		)
		for i := range 6 {
			p.Claims = append(p.Claims, Claim{ID: fmt.Sprintf("%s:aa-admin-%02d", p.ID, i), Subject: subject, Text: fmt.Sprintf("admin badge %02d", i), Provider: p.ID, Evidence: ev})
		}
	})
	for query, want := range map[string]string{"set admin user": "adapter:zz-setting", "revoke admin user": "adapter:zz-revoking"} {
		if order := claimOrder(t, a, query); len(order) == 0 || order[0] != want {
			t.Fatalf("docs_claims %q: %s should rank first: %v", query, want, order)
		}
		order := searchOrders(t, a, Request{Operation: "search", Query: query, Include: []string{"claims"}})
		at := rankOf(order, want)
		if at == 0 || slices.ContainsFunc(order[:at-1], func(id string) bool { return strings.HasPrefix(id, "adapter:aa-admin-") }) {
			t.Fatalf("docs_search %q: %s should outrank the admin-only claims: %v", query, want, order)
		}
	}
	for term, want := range map[string][]string{
		"set":    {"sets", "setes", "seted", "seting", "setted", "setting"},
		"revoke": {"revokes", "revokees", "revokeed", "revokeing", "revoked", "revoking"},
		"ab":     nil,
		"sync2":  nil,
	} {
		if got := inflections(term); !slices.Equal(got, want) {
			t.Errorf("inflections(%q) = %v, want %v", term, got, want)
		}
	}
}

// DCX-V1-477 (GitHub #1665 row 3): ties no longer order by ID string. Claims
// holding as many query terms order by the summed idf of the terms they hold,
// then the shorter text, then natural ID order (claim:209 before claim:2004);
// docs_search ties order by the shorter name, then natural ID order.
func TestDCXV1477MeaningfulTieBreaks(t *testing.T) {
	t.Parallel()
	a := buildAdoption(t, func(p *ProviderRecord) {
		ev := p.Subjects[0].Evidence
		subject := p.Subjects[0].ID
		for _, n := range []int{2004, 2052, 209, 211} {
			p.Claims = append(p.Claims, Claim{ID: fmt.Sprintf("%s:claim-%d", p.ID, n), Subject: subject, Text: "tee time booking", Provider: p.ID, Evidence: ev})
		}
		p.Claims = append(p.Claims,
			Claim{ID: p.ID + ":claim-1", Subject: subject, Text: "tee time booking for a guest", Provider: p.ID, Evidence: ev},
			Claim{ID: p.ID + ":claim-9000", Subject: subject, Text: "a waitlist", Provider: p.ID, Evidence: ev},
		)
		for i := range 4 {
			p.Claims = append(p.Claims, Claim{ID: fmt.Sprintf("%s:claim-%d", p.ID, 3000+i), Subject: subject, Text: "a tee", Provider: p.ID, Evidence: ev})
		}
		p.Subjects = append(p.Subjects,
			Subject{ID: p.ID + ":s10", Kind: "use_case", Name: "marshal sheet view", Provider: p.ID, Evidence: ev},
			Subject{ID: p.ID + ":s9", Kind: "use_case", Name: "marshal sheet", Provider: p.ID, Evidence: ev},
			Subject{ID: p.ID + ":s100", Kind: "use_case", Name: "marshal sheet", Provider: p.ID, Evidence: ev},
		)
	})
	order := claimOrder(t, a, "booking")
	if want := []string{"adapter:claim-209", "adapter:claim-211", "adapter:claim-2004", "adapter:claim-2052", "adapter:claim-1"}; !slices.Equal(order, want) {
		t.Fatalf("equal claims should order by natural ID after the shorter text: %v, want %v", order, want)
	}
	// "waitlist" is held by one claim and "tee" by nine, so the rare term
	// orders first although both claims hold one query term.
	if order := claimOrder(t, a, "tee waitlist"); len(order) == 0 || order[0] != "adapter:claim-9000" {
		t.Fatalf("the rarer term should break the tie: %v", order)
	}
	subjects := searchOrders(t, a, Request{Operation: "search", Query: "marshal"})
	if want := []string{"adapter:s9", "adapter:s100", "adapter:s10"}; !slices.Equal(subjects, want) {
		t.Fatalf("equal subjects should order by the shorter name, then natural ID: %v, want %v", subjects, want)
	}
	ids := []string{"claim:2004", "claim:209", "claim:2088", "claim:211", "claim:0209", "claim:20", "claim:a", "claim:"}
	sort.Slice(ids, func(i, j int) bool { return naturalLess(ids[i], ids[j]) })
	if want := []string{"claim:", "claim:20", "claim:0209", "claim:209", "claim:211", "claim:2004", "claim:2088", "claim:a"}; !slices.Equal(ids, want) {
		t.Fatalf("natural order: %v, want %v", ids, want)
	}
}
