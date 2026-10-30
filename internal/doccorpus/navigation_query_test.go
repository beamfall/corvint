package doccorpus

import (
	"slices"
	"testing"
)

// navigationCorpus holds two flows with navigation evidence (a navigates and
// an asserts relation, each with a TestLink detail) and a ui_surface whose
// name repeats a flow's words but which no flow links to. GitHub #2013.
func navigationCorpus(t *testing.T) *Artifact {
	t.Helper()
	return buildAdoption(t, func(p *ProviderRecord) {
		ev := p.Subjects[0].Evidence
		add := func(id, kind, name string) {
			p.Subjects = append(p.Subjects, Subject{ID: id, Kind: kind, Name: name, Provider: p.ID, Evidence: ev})
		}
		add("adapter:checkout", "flow", "Checkout payment")
		add("adapter:settings", "flow", "Admin settings")
		for _, id := range []string{"adapter:checkout", "adapter:settings"} {
			p.Details[id] = bareFlow(p.Details["adapter:flow"].Flow)
		}
		add("adapter:cart-page", "test", "Cart page test")
		add("adapter:settings-test", "test", "settings spec")
		add("adapter:billing-screen", "ui_surface", "Billing checkout screen")
		p.Relations = append(p.Relations,
			Relation{ID: "adapter:checkout-nav", From: "adapter:checkout", To: "adapter:cart-page", Type: "navigates", Provider: p.ID, Evidence: ev},
			Relation{ID: "adapter:settings-asserts", From: "adapter:settings", To: "adapter:settings-test", Type: "asserts", Provider: p.ID, Evidence: ev},
		)
		p.Details["adapter:checkout-nav"] = RecordDetails{TestLink: &TestLinkDetails{Role: "navigating", JoinConfidence: "declared", File: "tests/checkout.spec.ts", Title: "pays with a saved card", Project: "web"}}
		p.Details["adapter:settings-asserts"] = RecordDetails{TestLink: &TestLinkDetails{Role: "asserting", JoinConfidence: "declared", File: "tests/admin.spec.ts", Title: "saves the theme", Project: "web"}}
	})
}

// DCS-V1-596 (GitHub #2013): navigation takes a query instead of an id. It
// selects each flow whose id, name, or linked test file or title holds every
// query word, and returns that flow's navigation records with its typed
// asserts and navigates relations and their TestLink details. A subject that
// is not a flow is never selected by itself, and a query without a
// searchable word selects nothing.
func TestDCSV1596NavigationQuery(t *testing.T) {
	t.Parallel()
	a := navigationCorpus(t)
	cases := []struct {
		query          string
		want, excluded []string
	}{
		{"checkout.spec", []string{"adapter:checkout", "adapter:checkout-nav", "adapter:cart-page"}, []string{"adapter:settings", "adapter:billing-screen"}},
		{"admin settings", []string{"adapter:settings", "adapter:settings-asserts", "adapter:settings-test"}, []string{"adapter:checkout", "adapter:checkout-nav"}},
		{"saves theme", []string{"adapter:settings", "adapter:settings-asserts"}, []string{"adapter:checkout"}},
		{"Checkout payments", []string{"adapter:checkout", "adapter:checkout-nav"}, []string{"adapter:settings"}},
	}
	for _, c := range cases {
		r := pageOf(t, a, Request{Operation: "navigation", Query: c.query, Limit: 100})
		ids := resultIDs(r)
		for _, id := range c.want {
			if !slices.Contains(ids, id) {
				t.Fatalf("%q: lacks %s: %v", c.query, id, ids)
			}
		}
		for _, id := range c.excluded {
			if slices.Contains(ids, id) {
				t.Fatalf("%q: selected %s: %v", c.query, id, ids)
			}
		}
		for _, id := range ids {
			if (id == "adapter:checkout-nav" || id == "adapter:settings-asserts") && r.Details[id].TestLink == nil {
				t.Fatalf("%q: %s lost its TestLink detail", c.query, id)
			}
		}
	}
	// Every word must be held by one flow; a ui_surface alone is no flow.
	for _, q := range []string{"checkout admin", "billing screen", "a", "--"} {
		if r := pageOf(t, a, Request{Operation: "navigation", Query: q, Limit: 100}); len(r.Results) != 0 {
			t.Fatalf("%q: selected %v", q, resultIDs(r))
		}
	}
	// The query form pages like the id form.
	first := pageOf(t, a, Request{Operation: "navigation", Query: "admin settings", Limit: 1})
	if len(first.Results) != 1 || first.NextOffset == nil {
		t.Fatalf("query form should page: %v next %v", resultIDs(first), first.NextOffset)
	}
	if _, err := Query(a, Request{Operation: "navigation", ID: "adapter:checkout", Query: "checkout", Limit: 10}, "fresh", nil); err == nil {
		t.Fatal("navigation admitted an id and a query together")
	}
}

// DCS-V1-596 (review repair): when the query selects two flows that share one
// behavior-contract test, each selected flow gets that test's selectors, and
// the id form still returns only its own flow's selectors.
func TestDCSV1596NavigationQuerySharedBehaviorContractTest(t *testing.T) {
	t.Parallel()
	a := navigationCorpus(t)
	a.BehaviorContracts = append(a.BehaviorContracts, BehaviorReport{Registry: BehaviorRegistry{Tests: []BehaviorTest{{
		ID:         "contract:shared",
		Flows:      []string{"adapter:checkout", "adapter:settings"},
		Assertions: []BehaviorAssertion{{ID: "visible", Locator: "#main"}},
	}}}})
	selectorFlows := func(r Receipt) []string {
		flows := []string{}
		for _, v := range r.Results {
			if s, ok := v.(NavigationSelector); ok {
				flows = append(flows, s.Flow)
			}
		}
		slices.Sort(flows)
		return flows
	}
	both := pageOf(t, a, Request{Operation: "navigation", Query: "spec", Limit: 100})
	if ids := resultIDs(both); !slices.Contains(ids, "adapter:checkout") || !slices.Contains(ids, "adapter:settings") {
		t.Fatalf("query should select both flows: %v", ids)
	}
	if got := selectorFlows(both); !slices.Equal(got, []string{"adapter:checkout", "adapter:settings"}) {
		t.Fatalf("each selected flow needs the shared test's selector, got flows %v", got)
	}
	for _, id := range []string{"adapter:checkout", "adapter:settings"} {
		one := pageOf(t, a, Request{Operation: "navigation", ID: id, Limit: 100})
		if got := selectorFlows(one); !slices.Equal(got, []string{id}) {
			t.Fatalf("id form %s: selector flows %v", id, got)
		}
	}
}
