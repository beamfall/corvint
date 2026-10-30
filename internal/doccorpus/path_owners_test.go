package doccorpus

import (
	"context"
	"slices"
	"testing"
)

// ownerCorpus adds, around the native subjects anchored at src/value_test.go,
// a flow that documents that file, an asserting test link from that flow, and
// three subjects that must not become owners: one related_to the file, one
// the file documents (outgoing), and one that documents only the owner (two
// hops). GitHub #2012.
func ownerCorpus(t *testing.T) *Artifact {
	t.Helper()
	return buildAdoption(t, func(p *ProviderRecord) {
		ev := p.Subjects[0].Evidence
		module := "native:file:src/value_test.go"
		for _, id := range []string{"adapter:owner", "adapter:neighbour", "adapter:documented", "adapter:grand-owner"} {
			p.Subjects = append(p.Subjects, Subject{ID: id, Kind: "flow", Name: id, Provider: p.ID, Evidence: ev})
			p.Details[id] = bareFlow(p.Details["adapter:flow"].Flow)
		}
		p.Subjects = append(p.Subjects, Subject{ID: "adapter:owner-test", Kind: "test", Name: "owner test", Provider: p.ID, Evidence: ev})
		p.Relations = append(p.Relations,
			Relation{ID: "adapter:owns", From: "adapter:owner", To: module, Type: "documents", Provider: p.ID, Evidence: ev},
			Relation{ID: "adapter:owner-asserts", From: "adapter:owner", To: "adapter:owner-test", Type: "asserts", Provider: p.ID, Evidence: ev},
			Relation{ID: "adapter:near", From: "adapter:neighbour", To: module, Type: "related_to", Provider: p.ID, Evidence: ev},
			Relation{ID: "adapter:outgoing", From: module, To: "adapter:documented", Type: "documents", Provider: p.ID, Evidence: ev},
			Relation{ID: "adapter:second-hop", From: "adapter:grand-owner", To: "adapter:owner", Type: "documents", Provider: p.ID, Evidence: ev},
		)
		p.Details["adapter:owner-asserts"] = RecordDetails{TestLink: &TestLinkDetails{Role: "asserting", JoinConfidence: "declared", File: "tests/owner_test.go", Title: "TestOwner", Project: "go"}}
	})
}

// bareFlow is the typed detail of a flow with flow's section names and no
// paragraphs, actors, entry points or variations.
func bareFlow(flow *FlowDetails) RecordDetails {
	sections := []FlowSection{}
	for _, s := range flow.Sections {
		sections = append(sections, FlowSection{Name: s.Name, Paragraphs: []string{}})
	}
	return RecordDetails{Flow: &FlowDetails{Sections: sections, Actors: []string{}, Entrypoints: []string{}, Variations: []string{}}}
}

// DCS-V1-594 (GitHub #2012): locate on a path also returns each subject that
// documents a subject anchored there, through one incoming documents
// relation, with that relation as its justification. Other relation types,
// outgoing documents relations and a second hop add nothing; the indexed and
// scanned forms agree.
func TestDCSV1594LocateReturnsDocumentingOwners(t *testing.T) {
	t.Parallel()
	a := ownerCorpus(t)
	scanned := pageOf(t, a, Request{Operation: "locate", Path: "src/value_test.go", Limit: 100})
	indexed := *a
	index, err := BuildQueryIndex(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	indexed.RuntimeIndex = index
	for name, r := range map[string]Receipt{"scan": scanned, "index": pageOf(t, &indexed, Request{Operation: "locate", Path: "src/value_test.go", Limit: 100})} {
		ids := resultIDs(r)
		for _, want := range []string{"native:file:src/value_test.go", "adapter:owner", "adapter:owns"} {
			if !slices.Contains(ids, want) {
				t.Fatalf("%s: locate lacks %s: %v", name, want, ids)
			}
		}
		for _, absent := range []string{"adapter:neighbour", "adapter:near", "adapter:documented", "adapter:outgoing", "adapter:grand-owner", "adapter:second-hop", "adapter:owner-asserts"} {
			if slices.Contains(ids, absent) {
				t.Fatalf("%s: locate followed more than one incoming documents hop: %s in %v", name, absent, ids)
			}
		}
		seen := map[string]bool{}
		for _, id := range ids {
			if seen[id] {
				t.Fatalf("%s: duplicate %s in %v", name, id, ids)
			}
			seen[id] = true
		}
	}
	if !slices.Equal(slices.Sorted(slices.Values(resultIDs(scanned))), slices.Sorted(slices.Values(resultIDs(pageOf(t, &indexed, Request{Operation: "locate", Path: "src/value_test.go", Limit: 100}))))) {
		t.Fatal("indexed and scanned locate disagree")
	}
	// A path no subject documents keeps its results unchanged.
	plain := pageOf(t, buildAdoption(t, nil), Request{Operation: "locate", Path: "src/value_test.go", Limit: 100})
	for _, id := range resultIDs(plain) {
		if id == "adapter:owns" || id == "adapter:owner" {
			t.Fatalf("owner without documents relation: %v", resultIDs(plain))
		}
	}
}

// DCS-V1-595 (GitHub #2012): recommend-tests on a path also returns the
// documenting owners with their documents relation, and the TestLink
// relations of those owners, while the selection stays the full relevant
// suite: an owner's test link is evidence, never a narrowing.
func TestDCSV1595RecommendTestsFollowsOwnerTestLinks(t *testing.T) {
	t.Parallel()
	a := ownerCorpus(t)
	r := pageOf(t, a, Request{Operation: "recommend-tests", Path: "src/value_test.go", Limit: 100})
	ids := resultIDs(r)
	for _, want := range []string{"adapter:owner", "adapter:owns", "adapter:owner-asserts"} {
		if !slices.Contains(ids, want) {
			t.Fatalf("recommend-tests lacks %s: %v", want, ids)
		}
	}
	for _, absent := range []string{"adapter:neighbour", "adapter:near", "adapter:documented", "adapter:grand-owner", "adapter:second-hop"} {
		if slices.Contains(ids, absent) {
			t.Fatalf("recommend-tests followed more than one incoming documents hop: %s in %v", absent, ids)
		}
	}
	if r.Selection == nil || r.Selection.State != "full-relevant-suite-required" || r.Selection.NarrowingAllowed {
		t.Fatalf("an owner's test link must not narrow the selection: %+v", r.Selection)
	}
	if r.Details["adapter:owner-asserts"].TestLink == nil {
		t.Fatalf("owner test link lost its TestLink detail: %+v", r.Details["adapter:owner-asserts"])
	}
}
