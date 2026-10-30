package doccorpus

import (
	"fmt"
	"slices"
	"testing"
)

// leftoversCorpus reproduces the GitHub #1665 re-test leftovers: a flow whose
// name a question holds among rarer question words, a manual case whose name
// holds both forms of one of those words, identifier files a shorter feature
// name ties, and a controller file whose own declarations phrase-match it.
func leftoversCorpus(t *testing.T) *Artifact {
	t.Helper()
	return buildAdoption(t, func(p *ProviderRecord) {
		ev := p.Subjects[0].Evidence
		add := func(id, kind, name string) {
			p.Subjects = append(p.Subjects, Subject{ID: p.ID + ":lo-" + id, Kind: kind, Name: name, Provider: p.ID, Evidence: ev})
		}
		add("flow", "use_case", "Reservation Cancellation")
		add("calendar", "use_case", "Reservation calendar")
		add("fee", "business_term", "Cancellation fee")
		add("breaks", "document", "Breaks between sessions")
		add("pos-file", "module", "src/app/services/pos_sync.rb")
		add("pos-feature", "capability", "POS sync dashboard")
		add("deck-file", "module", "src/app/controllers/club/on_deck_controller.rb")
		add("symbol:src/app/controllers/club/on_deck_controller.rb:1:OnDeckController", "symbol", "OnDeckController")
		add("symbol:src/app/controllers/club/on_deck_controller.rb:2:index", "symbol", "index")
		add("counter", "use_case", "Occupancy Counter")
		add("conditions", "business_term", "conditions")
		add("refund", "business_term", "refund")
		for i := range 10 {
			add(fmt.Sprintf("filler-%02d", i), "use_case", fmt.Sprintf("member profile screen %02d", i))
		}
	})
}

// DCS-V1-540: a query word counts once, by the best of the terms it yields,
// so the plural "breaks" (which also yields "break") no longer outweighs two
// subject words, and served terms count held query words.
func TestDCSV1540QueryWordCountsOnce(t *testing.T) {
	t.Parallel()
	a := leftoversCorpus(t)
	q := Request{Operation: "search", Query: "breaks reservation cancellation", Limit: 10}
	order := searchOrders(t, a, q)
	if rankOf(order, "adapter:lo-flow") != 1 || rankOf(order, "adapter:lo-breaks") <= 1 {
		t.Fatalf("the subject holding two query words should outrank one holding both forms of one word: %v", order)
	}
	ranks := pageOf(t, a, q).ResultRanks
	if ranks["adapter:lo-flow"].Terms != 2 || ranks["adapter:lo-breaks"].Terms != 1 || ranks["adapter:lo-flow"].Tier != "terms" {
		t.Fatalf("terms should count held query words: %v", ranks)
	}
	groups := queryGroups("What breaks, Cancellations", []string{"break", "breaks", "cancellation", "cancellations"})
	if !slices.EqualFunc(groups, [][]string{{"break", "breaks"}, {"cancellation", "cancellations"}}, slices.Equal) {
		t.Fatalf("groups: %v", groups)
	}
	// The grouping, and so the order, does not depend on word order: a word
	// and its plural in either order are one group (Codex review, P2).
	terms := []string{"condition", "conditions", "refund"}
	for _, query := range []string{"conditions condition refund", "condition conditions refund"} {
		if got := queryGroups(query, terms); !slices.EqualFunc(got, [][]string{{"condition", "conditions"}, {"refund"}}, slices.Equal) {
			t.Fatalf("%q groups: %v", query, got)
		}
	}
	first := searchOrders(t, a, Request{Operation: "search", Query: "condition conditions refund", Limit: 10})
	second := searchOrders(t, a, Request{Operation: "search", Query: "conditions condition refund", Limit: 10})
	if !slices.Equal(first, second) {
		t.Fatalf("word order changed the ranking: %v vs %v", first, second)
	}
}

// DCS-V1-541: a query of one identifier ranks a subject one of whose name or
// path tokens spells it, with or without the token's extension, before a
// shorter name that only holds its words as a phrase. A spaced query is not
// an identifier.
func TestDCSV1541IdentifierTokenTier(t *testing.T) {
	t.Parallel()
	a := leftoversCorpus(t)
	for _, query := range []string{"pos_sync", "PosSync", "pos_sync.rb"} {
		q := Request{Operation: "search", Query: query, Limit: 10}
		order := searchOrders(t, a, q)
		ranks := pageOf(t, a, q).ResultRanks
		if order[0] != "adapter:lo-pos-file" || ranks["adapter:lo-pos-file"].Tier != "identifier" {
			t.Fatalf("%s: the file spelling the identifier should rank first: %v %v", query, order, ranks)
		}
	}
	ranks := pageOf(t, a, Request{Operation: "search", Query: "pos_sync", Limit: 10}).ResultRanks
	if ranks["adapter:lo-pos-feature"].Tier != "phrase" || ranks["adapter:lo-pos-feature"].Group <= ranks["adapter:lo-pos-file"].Group {
		t.Fatalf("the feature only holds the words as a phrase: %v", ranks)
	}
	spaced := pageOf(t, a, Request{Operation: "search", Query: "pos sync", Limit: 10}).ResultRanks
	if spaced["adapter:lo-pos-file"].Tier != "path-phrase" {
		t.Fatalf("a spaced query is not an identifier: %v", spaced)
	}
}

// DCS-V1-542 (V1-2549): a phrase in a module or file subject's path orders
// before a phrase in another subject's name, which orders before a phrase
// held only by an ID, so "on deck" ranks the controller file before its own
// declarations.
func TestDCSV1542PathPhraseBeforeDeclarations(t *testing.T) {
	t.Parallel()
	a := leftoversCorpus(t)
	q := Request{Operation: "search", Query: "on deck", Limit: 10}
	order := searchOrders(t, a, q)
	class := "adapter:lo-symbol:src/app/controllers/club/on_deck_controller.rb:1:OnDeckController"
	index := "adapter:lo-symbol:src/app/controllers/club/on_deck_controller.rb:2:index"
	if rankOf(order, "adapter:lo-deck-file") != 1 || rankOf(order, class) != 2 || rankOf(order, index) != 3 {
		t.Fatalf("file, then the name phrase, then the ID phrase: %v", order)
	}
	ranks := pageOf(t, a, q).ResultRanks
	if ranks["adapter:lo-deck-file"].Tier != "path-phrase" || ranks[class].Tier != "phrase" || ranks[index].Tier != "id-phrase" {
		t.Fatalf("tiers: %v", ranks)
	}
}

// DCS-V1-543: a question ranks a subject whose name of two or more words it
// holds before subjects matching its terms alone; a statement holding a name
// does not, and an identifier starting with a question word is no question.
func TestDCSV1543QuestionNamesASubject(t *testing.T) {
	t.Parallel()
	a := leftoversCorpus(t)
	for _, query := range []string{"What breaks in Reservation Cancellation, and under what conditions?", "how does reservation cancellation refund"} {
		q := Request{Operation: "search", Query: query, Limit: 10}
		order := searchOrders(t, a, q)
		ranks := pageOf(t, a, q).ResultRanks
		if order[0] != "adapter:lo-flow" || ranks["adapter:lo-flow"].Tier != "named" || ranks["adapter:lo-flow"].Terms != 2 {
			t.Fatalf("%q: the named flow should rank first: %v %v", query, order, ranks)
		}
	}
	statement := pageOf(t, a, Request{Operation: "search", Query: "occupancy counter counts booked players", Limit: 10}).ResultRanks
	if statement["adapter:lo-counter"].Tier != "terms" {
		t.Fatalf("a statement names no subject: %v", statement)
	}
	for query, want := range map[string]bool{"is_admin": false, "Is admin set": true, "admin set?": true, "whatever": false} {
		if questionShaped(query) != want {
			t.Fatalf("questionShaped(%q) = %v", query, !want)
		}
	}
	if !mayBeNamedIn("OnDeckController", "what is the on deck controller") || mayBeNamedIn("On Deck board", "what is the on deck controller") || !mayBeNamedIn("Café menu", "x") {
		t.Fatal("prefilter")
	}
}

// DCS-V1-544: tiers split groups even at equal scores, every served tier is
// one of the named values, and a merged claim carries its held word count.
func TestDCSV1544ServedTiersAndTerms(t *testing.T) {
	t.Parallel()
	a := leftoversCorpus(t)
	served := []string{"exact", "identifier", "path-phrase", "phrase", "id-phrase", "named", "terms"}
	for _, query := range []string{"on deck", "pos_sync", "What breaks in Reservation Cancellation?", "Reservation calendar"} {
		r := pageOf(t, a, Request{Operation: "search", Query: query, Include: []string{"claims"}, Limit: 50})
		byGroup := map[int]string{}
		for id, rank := range r.ResultRanks {
			if !slices.Contains(served, rank.Tier) {
				t.Fatalf("%q %s: tier %q", query, id, rank.Tier)
			}
			if tier, ok := byGroup[rank.Group]; ok && tier != rank.Tier {
				t.Fatalf("%q: group %d holds tiers %s and %s", query, rank.Group, tier, rank.Tier)
			}
			byGroup[rank.Group] = rank.Tier
		}
	}
	if got := tierNames[:]; !slices.Equal(got, served) {
		t.Fatalf("tier names %v", got)
	}
}
