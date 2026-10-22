package doccorpus

import (
	"fmt"
	"strings"
	"testing"
)

// statusLedger extends migrationLedger with a second run, more readiness
// claims, a qualified status and a status outside any mapping.
func statusLedger(p *ProviderRecord) {
	migrationLedger(p)
	ev := p.Subjects[0].Evidence
	p.Subjects = append(p.Subjects, Subject{ID: "adapter:unit:next:1", Kind: "inventory", Name: "unit next:1", Provider: p.ID, Evidence: ev})
	p.Relations = append(p.Relations, Relation{ID: "adapter:map:4", From: "adapter:unit:new:1", To: "adapter:unit:next:1", Type: "migration_mapping", Provider: p.ID, Evidence: ev})
	p.Details["adapter:map:4"] = RecordDetails{MigrationMapping: &MigrationMappingDetails{Outcome: "renamed", Basis: "symbol", Run: "migration-run-2"}}
	status := func(id, subject string, d SuppliedStatusDetails) {
		p.Claims = append(p.Claims, Claim{ID: id, Subject: subject, Text: subject + " readiness", Provider: p.ID, Evidence: ev})
		d.Supporting = []string{}
		p.Details[id] = RecordDetails{Status: &d}
	}
	status("adapter:ready:new:2", "adapter:unit:new:2", SuppliedStatusDetails{State: "unknown", Reason: "ambiguous origin"})
	status("adapter:ready:next:1", "adapter:unit:next:1", SuppliedStatusDetails{State: "unknown", Reason: "second run"})
	status("adapter:ready:new:3", "adapter:unit:new:3", SuppliedStatusDetails{State: "proven", Qualifier: "provisional", Reason: "provisional proof"})
	status("adapter:status:flow", "adapter:flow", SuppliedStatusDetails{State: "blocked", Reason: "outside the ledger"})
}

func claimIDs(r Receipt) string {
	out := []string{}
	for _, v := range r.Results {
		out = append(out, recordID(v))
	}
	return fmt.Sprint(out)
}

// DCP-V1-318: claims with a state lists the supplied status claims in that
// state, unqualified unless a qualifier is named, optionally only those whose
// subject a migration_mapping of one run names; each result carries its
// subject, anchors and status detail, the listing pages, and the unqualified
// total equals the docs_coverage supplied_status row.
func TestDCPV1318StatusClaimListing(t *testing.T) {
	t.Parallel()
	a := buildAdoption(t, statusLedger)
	for _, c := range []struct {
		q    Request
		want string
	}{
		{Request{State: "unknown"}, "[adapter:ready:new:1 adapter:ready:new:2 adapter:ready:next:1]"},
		{Request{State: "unknown", Run: "migration-run-1"}, "[adapter:ready:new:1 adapter:ready:new:2]"},
		{Request{State: "unknown", Run: "migration-run-2"}, "[adapter:ready:new:1 adapter:ready:next:1]"},
		{Request{State: "unknown", Run: "migration-run-3"}, "[]"},
		{Request{State: "proven"}, "[]"},
		{Request{State: "proven", Qualifier: "provisional"}, "[adapter:ready:new:3]"},
		{Request{State: "proven", Qualifier: "provisional", Run: "migration-run-1"}, "[adapter:ready:new:3]"},
		{Request{State: "blocked"}, "[adapter:status:flow]"},
		{Request{State: "blocked", Run: "migration-run-1"}, "[]"},
	} {
		c.q.Operation = "claims"
		r := queryResults(t, a, c.q)
		if got := claimIDs(r); got != c.want {
			t.Fatalf("%+v = %s, want %s", c.q, got, c.want)
		}
		for _, v := range r.Results {
			claim := v.(Claim)
			d := r.Details[claim.ID].Status
			if claim.Subject == "" || len(claim.Evidence.Anchors) == 0 || d == nil || d.State != c.q.State || d.Qualifier != c.q.Qualifier {
				t.Fatalf("%+v: result lacks subject, anchors or status: %+v %+v", c.q, claim, d)
			}
		}
		if c.want != "[]" && len(r.Citations) == 0 {
			t.Fatalf("%+v: no citations", c.q)
		}
		if c.want == "[]" && (r.State != "empty" || r.Miss != "no-match") {
			t.Fatalf("%+v: empty listing state = %s/%s", c.q, r.State, r.Miss)
		}
	}
	if m := coverageRows(t, a)["supplied_status:unknown"]; m == nil || m["value"] != 3 {
		t.Fatalf("coverage row disagrees with the listing: %v", m)
	}
	first, err := Query(a, Request{Operation: "claims", State: "unknown", Limit: 2}, "fresh", nil)
	if err != nil || claimIDs(first) != "[adapter:ready:new:1 adapter:ready:new:2]" || first.NextOffset == nil || *first.NextOffset != 2 {
		t.Fatalf("first page = %s %v %v", claimIDs(first), first.NextOffset, err)
	}
	next, err := Query(a, Request{Operation: "claims", State: "unknown", Limit: 2, Offset: 2}, "fresh", nil)
	if err != nil || claimIDs(next) != "[adapter:ready:next:1]" || next.NextOffset != nil {
		t.Fatalf("second page = %s %v %v", claimIDs(next), next.NextOffset, err)
	}
	for _, q := range []Request{
		{Operation: "claims", State: "unknown", Path: "src/value.go"},
		{Operation: "claims", State: "unknown", Query: "readiness"},
		{Operation: "claims", Qualifier: "provisional"},
		{Operation: "claims", Run: "migration-run-1"},
		{Operation: "claims", State: "done"},
		{Operation: "claims", State: "proven", Qualifier: "final"},
		{Operation: "claims", State: "unknown", Run: strings.Repeat("r", 1025)},
		{Operation: "get", ID: "adapter:flow", State: "unknown"},
	} {
		if _, err := Query(a, q, "fresh", nil); err == nil {
			t.Fatalf("%+v admitted", q)
		}
	}
}

// DCP-V1-319: intent without an id lists every intent comparison subject in
// ID order with its detail, paged, and no ticket or other subject.
func TestDCPV1319IntentListing(t *testing.T) {
	t.Parallel()
	a := buildAdoption(t, func(p *ProviderRecord) {
		p.Subjects = append(p.Subjects, Subject{ID: "adapter:intent:2", Kind: "intent_comparison", Name: "second intent", Provider: p.ID, Evidence: p.Subjects[0].Evidence})
		p.Details["adapter:intent:2"] = RecordDetails{IntentComparison: &IntentComparisonDetails{ReportedIntent: []string{"adapter:paragraph"}, ObservedClaims: []string{"adapter:claim"}, Missing: []string{}, Conflicts: []string{}}}
	})
	r := queryResults(t, a, Request{Operation: "intent"})
	if got := claimIDs(r); got != "[adapter:intent adapter:intent:2]" {
		t.Fatalf("intent listing = %s", got)
	}
	for _, id := range []string{"adapter:intent", "adapter:intent:2"} {
		if r.Details[id].IntentComparison == nil {
			t.Fatalf("%s lost its comparison: %+v", id, r.Details)
		}
	}
	page, err := Query(a, Request{Operation: "intent", Limit: 1}, "fresh", nil)
	if err != nil || claimIDs(page) != "[adapter:intent]" || page.NextOffset == nil || *page.NextOffset != 1 {
		t.Fatalf("intent page = %s %v %v", claimIDs(page), page.NextOffset, err)
	}
	// One subject's comparisons are unchanged.
	if got := claimIDs(queryResults(t, a, Request{Operation: "intent", ID: "adapter:intent"})); got != "[adapter:intent]" {
		t.Fatalf("intent by id = %s", got)
	}
}
