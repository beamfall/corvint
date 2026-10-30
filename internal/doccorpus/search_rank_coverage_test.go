package doccorpus

import (
	"testing"
)

// coverageCorpus reproduces the GitHub #1665 re-test of a681846 (V1-2575):
// short names holding a few rare query words in the name field outscore a
// file whose prose name holds every query word, so the file ranked 7th.
func coverageCorpus(t *testing.T) *Artifact {
	t.Helper()
	return buildAdoption(t, func(p *ProviderRecord) {
		ev := p.Subjects[0].Evidence
		add := func(id, kind, name string) {
			p.Subjects = append(p.Subjects, Subject{ID: p.ID + ":cov-" + id, Kind: kind, Name: name, Provider: p.ID, Evidence: ev})
		}
		add("action", "symbol", "lsid redirect url")
		add("template", "document", "lsid redirect")
		add("helper", "symbol", "redirect url")
		add("expected", "document", "The admin sessions controller logout action builds the redirect url from the LSID token and the session state of every signed in member before it returns")
		add("partial", "document", "The admin sessions controller logout action clears the session state of every signed in member before it returns to the start page")
		for _, w := range []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot"} {
			add("filler-"+w, "use_case", "member "+w+" screen")
		}
	})
}

// DCS-V1-547: inside tier terms a result holding more query words orders
// before one holding fewer, whatever their per-word scores, and a change of
// held word count starts a new group.
func TestDCSV1547TermsTierOrdersByCoverage(t *testing.T) {
	t.Parallel()
	a := coverageCorpus(t)
	q := Request{Operation: "search", Query: "admin logout redirect url LSID", Limit: 10}
	order := searchOrders(t, a, q)
	ranks := pageOf(t, a, q).ResultRanks
	expected := ranks["adapter:cov-expected"]
	if order[0] != "adapter:cov-expected" || expected.Tier != "terms" || expected.Terms != 5 {
		t.Fatalf("the result holding all five words should rank first: %v %v", order, ranks)
	}
	action := ranks["adapter:cov-action"]
	if action.Terms != 3 || action.Score <= expected.Score || action.Group <= expected.Group {
		t.Fatalf("the three-word name scores higher but holds fewer words, so it orders after: %v", ranks)
	}
	if partial := ranks["adapter:cov-partial"]; partial.Terms != 2 || rankOf(order, "adapter:cov-partial") < rankOf(order, "adapter:cov-action") {
		t.Fatalf("two words after three: %v %v", order, ranks)
	}
	// A long description: the file holding ten of its eleven words ranks
	// above results holding fewer.
	long := Request{Operation: "search", Query: "admin sessions controller logout action builds redirect url from lsid cookie", Limit: 10}
	if order := searchOrders(t, a, long); order[0] != "adapter:cov-expected" {
		t.Fatalf("the result holding ten of eleven words should rank first: %v", order)
	}
	for id, rank := range pageOf(t, a, long).ResultRanks {
		for other, r := range pageOf(t, a, long).ResultRanks {
			if rank.Group == r.Group && (rank.Terms != r.Terms || rank.Score != r.Score) {
				t.Fatalf("group %d holds %s (%v) and %s (%v)", rank.Group, id, rank, other, r)
			}
		}
	}
}

// DCS-V1-548 (V1-2568): a question names a one-word subject by a capitalized
// word that is not the question's first, so "How does Waitlist work?" ranks
// the Waitlist flow in tier named before a rarer question word's match. A
// lower-case word, the first word and a statement name nothing.
func TestDCSV1548QuestionNamesAOneWordSubject(t *testing.T) {
	t.Parallel()
	a := buildAdoption(t, func(p *ProviderRecord) {
		ev := p.Subjects[0].Evidence
		add := func(id, kind, name string) {
			p.Subjects = append(p.Subjects, Subject{ID: p.ID + ":one-" + id, Kind: kind, Name: name, Provider: p.ID, Evidence: ev})
		}
		add("waitlist", "use_case", "Waitlist")
		add("work", "document", "Work schedule")
		for _, w := range []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot"} {
			add("filler-"+w, "use_case", "waitlist "+w+" screen")
		}
	})
	q := Request{Operation: "search", Query: "How does Waitlist work?", Limit: 10}
	order := searchOrders(t, a, q)
	ranks := pageOf(t, a, q).ResultRanks
	if order[0] != "adapter:one-waitlist" || ranks["adapter:one-waitlist"].Tier != "named" {
		t.Fatalf("the capitalized one-word name should rank first in tier named: %v %v", order, ranks)
	}
	for _, query := range []string{"how does waitlist work?", "Waitlist: how does it work?", "Waitlist work schedule"} {
		if tier := pageOf(t, a, Request{Operation: "search", Query: query, Limit: 10}).ResultRanks["adapter:one-waitlist"].Tier; tier == "named" {
			t.Fatalf("%q should not name the one-word subject: tier %s", query, tier)
		}
	}
	for query, want := range map[string]bool{"How does Waitlist work?": true, "What is the WaitlistNotifier?": false, "What does Does mean?": false, "What breaks under what conditions?": false} {
		if got := capitalizedWords(query)["waitlist"] || capitalizedWords(query)["does"] || capitalizedWords(query)["conditions"]; got != want {
			t.Fatalf("capitalizedWords(%q) = %v", query, capitalizedWords(query))
		}
	}
}
