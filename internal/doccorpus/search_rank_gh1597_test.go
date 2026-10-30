package doccorpus

import (
	"context"
	"fmt"
	"slices"
	"testing"
)

// searchOrders returns the docs_search order of the scanning reader and of the
// indexed reader for one request, failing when they differ (DCS-V1-403).
func searchOrders(t *testing.T, a *Artifact, q Request) []string {
	t.Helper()
	index, err := BuildQueryIndex(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	indexed := *a
	indexed.RuntimeIndex = index
	var orders [2][]string
	for n, b := range []*Artifact{a, &indexed} {
		for _, v := range queryResults(t, b, q).Results {
			orders[n] = append(orders[n], recordID(v))
		}
	}
	if !slices.Equal(orders[0], orders[1]) {
		t.Fatalf("index/scan parity lost for %+v: %v vs %v", q, orders[0], orders[1])
	}
	return orders[0]
}

func rankOf(order []string, id string) int {
	return slices.Index(order, id) + 1
}

// tee sheet corpus for GitHub #1597 row 1: the file that implements the
// behavior is a module subject whose path names "cancel" and whose claim
// states the behavior; a documentation paragraph subject repeats the query
// words in a prose name; ten filler use cases keep the idf realistic.
func teeSheetCorpus(p *ProviderRecord) {
	ev := p.Subjects[0].Evidence
	p.Subjects = append(p.Subjects,
		Subject{ID: p.ID + ":zz-file-cancel", Kind: "module", Name: "src/teesheet/cancel_booking.ts", Provider: p.ID, Evidence: ev},
		Subject{ID: p.ID + ":aa-paragraph", Kind: "document", Name: "When a member cancels a booking the tee sheet shows the slot as free again and the occupancy counter for that day is updated by the nightly job", Provider: p.ID, Evidence: ev},
		Subject{ID: p.ID + ":aa-tally", Kind: "use_case", Name: "Tally engine", Provider: p.ID, Evidence: ev},
		Subject{ID: p.ID + ":aa-tally-module", Kind: "module", Name: "Tally engine", Provider: p.ID, Evidence: ev},
	)
	p.Claims = append(p.Claims, Claim{ID: p.ID + ":zz-file-cancel:claim", Subject: p.ID + ":zz-file-cancel", Text: "Cancel refreshes occupancy once per tee sheet", Provider: p.ID, Evidence: ev})
	for i := range 10 {
		p.Subjects = append(p.Subjects, Subject{ID: fmt.Sprintf("%s:filler-%02d", p.ID, i), Kind: "use_case", Name: fmt.Sprintf("member profile screen %02d", i), Provider: p.ID, Evidence: ev})
	}
}

// DCS-V1-400, DCS-V1-401 (GitHub #1597 row 1): file lookup needs no kind
// filter. A module subject's anchor path segments are name terms, and a prose
// subject name weighs as text, so the implementing file outranks the
// documentation paragraph that repeats the query words.
func TestDCSV1400ModulePathSegmentsAreNameTerms(t *testing.T) {
	t.Parallel()
	a := buildAdoption(t, teeSheetCorpus)
	query := Request{Operation: "search", Query: "cancel refreshes occupancy once per tee sheet"}
	order := searchOrders(t, a, query)
	if order[0] != "adapter:zz-file-cancel" {
		t.Fatalf("implementing file should rank first without a kind filter: %v", order)
	}
	if rankOf(order, "adapter:aa-paragraph") < 2 {
		t.Fatalf("prose paragraph subject should still match as text: %v", order)
	}
	kinded := query
	kinded.Kinds = []string{"module"}
	if got := searchOrders(t, a, kinded); len(got) == 0 || got[0] != "adapter:zz-file-cancel" {
		t.Fatalf("kind filter order changed: %v", got)
	}
	// Every fixture subject is anchored at src/value.go. A module subject
	// whose name is not that path matches the path's segments as name terms;
	// a use case with the same name and anchor does not.
	value := searchOrders(t, a, Request{Operation: "search", Query: "value"})
	if !slices.Contains(value, "adapter:aa-tally-module") || slices.Contains(value, "adapter:aa-tally") {
		t.Fatalf("anchor path segments should match module subjects only: %v", value)
	}
}

// DCS-V1-401: a name of at most twelve distinct terms keeps name weight, so
// a short title still outranks a claim-text-only match; only a longer, prose
// name weighs as text.
func TestDCSV1401ProseNameThreshold(t *testing.T) {
	t.Parallel()
	a := buildAdoption(t, func(p *ProviderRecord) {
		ev := p.Subjects[0].Evidence
		p.Subjects = append(p.Subjects,
			Subject{ID: p.ID + ":zz-title", Kind: "use_case", Name: "alpha bravo charlie delta echo foxtrot golf hotel india juliet kilo lima", Provider: p.ID, Evidence: ev},
			Subject{ID: p.ID + ":zz-prose", Kind: "use_case", Name: "alpha bravo charlie delta echo foxtrot golf hotel india juliet kilo lima mike", Provider: p.ID, Evidence: ev},
			Subject{ID: p.ID + ":aa-claimed", Kind: "use_case", Name: "unrelated", Provider: p.ID, Evidence: ev},
			Subject{ID: p.ID + ":aa-claimed-2", Kind: "use_case", Name: "unrelated two", Provider: p.ID, Evidence: ev},
		)
		p.Claims = append(p.Claims, Claim{ID: p.ID + ":aa-claimed:claim", Subject: p.ID + ":aa-claimed", Text: "lima appears here", Provider: p.ID, Evidence: ev})
	})
	order := searchOrders(t, a, Request{Operation: "search", Query: "lima"})
	if len(order) != 3 || order[0] != "adapter:zz-title" || order[1] != "adapter:aa-claimed" || order[2] != "adapter:zz-prose" {
		t.Fatalf("twelve-term name should weigh as a name and a thirteen-term name as text: %v", order)
	}
}

// DCS-V1-402 (review P2): an exact case-insensitive name match ranks first
// even when tokenization gives it no query term. "SignIn" splits into "sign"
// and "in", so it shares no term with the query "signin", while the helper's
// name holds every query term.
func TestDCSV1402ExactMatchPrecedesTermOverlap(t *testing.T) {
	t.Parallel()
	a := buildAdoption(t, func(p *ProviderRecord) {
		ev := p.Subjects[0].Evidence
		p.Subjects = append(p.Subjects,
			Subject{ID: p.ID + ":zz-target", Kind: "use_case", Name: "SignIn", Provider: p.ID, Evidence: ev},
			Subject{ID: p.ID + ":aa-helper", Kind: "use_case", Name: "signin helper", Provider: p.ID, Evidence: ev},
		)
		p.Claims = append(p.Claims, Claim{ID: p.ID + ":aa-claim", Subject: p.ID + ":aa-helper", Text: "signin opens the session", Provider: p.ID, Evidence: ev})
	})
	for _, include := range [][]string{nil, {"claims"}} {
		order := searchOrders(t, a, Request{Operation: "search", Query: "signin", Include: include})
		if len(order) < 2 || order[0] != "adapter:zz-target" {
			t.Fatalf("exact name match should rank first (include %v): %v", include, order)
		}
	}
}

// claimBurialCorpus for GitHub #1597 row 2: every query word is common
// across claims, though only one claim holds them all, while a handful of
// subjects name one query word, so under raw field weights every such
// subject outranked the claim.
func claimBurialCorpus(p *ProviderRecord) {
	ev := p.Subjects[0].Evidence
	for i := range 6 {
		p.Subjects = append(p.Subjects, Subject{ID: fmt.Sprintf("%s:refund-%02d", p.ID, i), Kind: "use_case", Name: fmt.Sprintf("Refund desk %02d", i), Provider: p.ID, Evidence: ev})
	}
	for i := range 30 {
		p.Subjects = append(p.Subjects, Subject{ID: fmt.Sprintf("%s:filler-%02d", p.ID, i), Kind: "use_case", Name: fmt.Sprintf("ledger page %02d", i), Provider: p.ID, Evidence: ev})
	}
	patterns := []string{"refund line %d keeps the receipt", "voids line %d before the receipt", "refund line %d voids nothing"}
	for j := range 60 {
		holder := fmt.Sprintf("%s:filler-%02d", p.ID, j%2)
		p.Claims = append(p.Claims, Claim{ID: fmt.Sprintf("%s:claim-%02d", holder, j), Subject: holder, Text: fmt.Sprintf(patterns[j%3], j), Provider: p.ID, Evidence: ev})
	}
	p.Claims = append(p.Claims, Claim{ID: p.ID + ":zz-target", Subject: p.ID + ":filler-00", Text: "A refund issues one credit note and voids the receipt", Provider: p.ID, Evidence: ev})
}

// DCS-V1-402 (GitHub #1597 row 2): with include:["claims"] each family's
// score is normalized by its largest possible partial score, so the claim
// that holds every query term outranks subjects naming one of them; without
// include the subject order is unchanged, and docs corpus claims --query
// still ranks the claim (DCS-V1-403).
func TestDCSV1402ClaimFamilyNormalizedMerge(t *testing.T) {
	t.Parallel()
	a := buildAdoption(t, claimBurialCorpus)
	query := Request{Operation: "search", Query: "refund voids the receipt", Include: []string{"claims"}}
	order := searchOrders(t, a, query)
	if len(order) == 0 || order[0] != "adapter:zz-target" {
		t.Fatalf("claim holding every query term should rank first: %v", order[:min(len(order), 8)])
	}
	plain := query
	plain.Include = nil
	subjects := searchOrders(t, a, plain)
	merged := []string{}
	for _, id := range order {
		if slices.Contains(subjects, id) {
			merged = append(merged, id)
		}
	}
	if !slices.Equal(merged, subjects) {
		t.Fatalf("include changed the relative subject order:\n%v\n%v", merged, subjects)
	}
	claims := queryResults(t, a, Request{Operation: "claims", Query: "refund voids the receipt"})
	if len(claims.Results) == 0 || recordID(claims.Results[0]) != "adapter:zz-target" {
		t.Fatalf("docs corpus claims --query lost the claim: %v", claims.Results)
	}
}
