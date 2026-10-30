package doccorpus

import (
	"context"
	"testing"
)

// DCP-V1-650, DCP-V1-651 (V1-3070, GitHub #1972): locate with a query ranks
// only module and symbol subjects, labels the receipt locate, and lists each
// visible result's line anchors; a kind filter narrows to one of the two.
func TestDCPV1650LocateByQueryReturnsAnchoredSourceSubjects(t *testing.T) {
	t.Parallel()
	root, m := ownedSourceCorpus(t, tsMemberSources, []string{"src/readme.md"}, false)
	a := buildOwned(t, root, m)
	r := queryResults(t, a, Request{Operation: "locate", Query: "Cart"})
	if r.Operation != "locate" || len(r.Results) == 0 {
		t.Fatalf("locate query receipt: %+v", r)
	}
	symbol := false
	for _, v := range r.Results {
		s, ok := v.(Subject)
		if !ok || s.Kind != "module" && s.Kind != "symbol" {
			t.Fatalf("locate query returned a non-source result: %+v", v)
		}
		if _, ok := r.ResultRanks[s.ID]; !ok {
			t.Fatalf("result %s has no rank: %v", s.ID, r.ResultRanks)
		}
		locations, ok := r.ResultLocations[s.ID]
		if !ok {
			t.Fatalf("result %s has no locations: %v", s.ID, r.ResultLocations)
		}
		for _, l := range locations {
			if l.Path == "" || l.Start < 1 || l.End < l.Start || l.Repository == "" {
				t.Fatalf("result %s location %+v is not a line anchor", s.ID, l)
			}
		}
		if s.ID == "native:symbol:src/cart.ts:1:Cart" {
			symbol = true
			if len(locations) != 1 || locations[0].Path != "src/cart.ts" || locations[0].Start != 1 || locations[0].Symbol != "Cart" {
				t.Fatalf("Cart locations = %+v", locations)
			}
		}
	}
	if !symbol || recordID(r.Results[0]) != "native:symbol:src/cart.ts:1:Cart" {
		t.Fatalf("the Cart declaration should rank first: %v", r.Results)
	}
	// The same query through search keeps every subject kind and no locations.
	if s := queryResults(t, a, Request{Operation: "search", Query: "Cart"}); s.ResultLocations != nil {
		t.Fatalf("search carries locations: %v", s.ResultLocations)
	}
	narrowed := queryResults(t, a, Request{Operation: "locate", Query: "Cart", Kinds: []string{"module"}})
	for _, v := range narrowed.Results {
		if v.(Subject).Kind != "module" {
			t.Fatalf("module filter returned %+v", v)
		}
	}
	for name, request := range map[string]Request{
		"foreign kind":   {Operation: "locate", Query: "Cart", Kinds: []string{"file"}},
		"repository":     {Operation: "locate", Query: "Cart", Repository: a.Manifest.Repository.ID},
		"kind on a path": {Operation: "locate", Path: "src/cart.ts", Kinds: []string{"symbol"}},
	} {
		if _, err := QueryContext(context.Background(), a, request, "fresh", nil); err == nil {
			t.Fatalf("%s admitted", name)
		}
	}
	// The path form is unchanged: no ranks, no locations, even beside a query.
	for _, query := range []string{"", "Cart"} {
		if p := queryResults(t, a, Request{Operation: "locate", Path: "src/cart.ts", Query: query}); len(p.Results) == 0 || p.ResultLocations != nil || p.ResultRanks != nil {
			t.Fatalf("path locate changed: %+v", p)
		}
	}
}
