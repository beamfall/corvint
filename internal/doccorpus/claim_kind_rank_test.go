package doccorpus

import (
	"encoding/json"
	"testing"
)

// claimKindCorpus holds, for one query, a behavioural claim that holds two of
// its three words, a supplied test-run claim whose text repeats the spec name
// and so holds all three, and an adversarial claim shaped like a run record
// (":run:" ID, "passed: spec/..." text) that carries no typed test-run
// detail. GitHub #2014.
func claimKindCorpus(t *testing.T) *Artifact {
	t.Helper()
	return buildAdoption(t, func(p *ProviderRecord) {
		ev := p.Subjects[0].Evidence
		subject := p.Subjects[0].ID
		p.Claims = append(p.Claims,
			Claim{ID: "adapter:behaviour", Subject: subject, Text: "the confirmation mailer sends one email per reservation", Provider: p.ID, Evidence: ev},
			Claim{ID: "adapter:run:a742", Subject: subject, Text: "passed: spec/mailers/booking_confirmation_mailer_spec.rb", Provider: p.ID, Evidence: ev},
			Claim{ID: "adapter:run:c588", Subject: subject, Text: "passed: spec/mailers/booking_confirmation_mailer_spec.rb retry", Provider: p.ID, Evidence: ev},
			Claim{ID: "adapter:run:shaped", Subject: subject, Text: "passed: spec/mailers/booking_confirmation_mailer_spec.rb shaped", Provider: p.ID, Evidence: ev},
		)
		p.Details["adapter:behaviour"] = RecordDetails{ClaimKind: "behavior"}
		for _, id := range []string{"adapter:run:a742", "adapter:run:c588"} {
			p.Details[id] = RecordDetails{TestRun: &SuppliedTestRunDetails{Outcome: "passed", Run: "run-" + id[len(id)-4:], Origin: "supplied"}}
		}
	})
}

// receiptMap is r as its wire JSON object.
func receiptMap(t *testing.T, r Receipt) map[string]any {
	t.Helper()
	raw, err := Encode(r)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// DCS-V1-593 (GitHub #2014): a claims query ranks every claim whose typed
// details make it a supplied test run after every other claim, even when the
// run text repeats the spec name and so holds more query words. A claim only
// shaped like a run record, without the typed detail, keeps its rank: the
// kind is read from evidence, never guessed from an ID or text.
func TestDCSV1593ClaimsQueryRanksTestRunsAfterOtherClaims(t *testing.T) {
	t.Parallel()
	a := claimKindCorpus(t)
	r := pageOf(t, a, Request{Operation: "claims", Query: "booking confirmation mailer", Limit: 10})
	ranks := r.ResultRanks
	behaviour, shaped := ranks["adapter:behaviour"], ranks["adapter:run:shaped"]
	if behaviour.Rank == 0 || shaped.Rank == 0 {
		t.Fatalf("claims missing from the ranked page: %v", ranks)
	}
	for _, run := range []string{"adapter:run:a742", "adapter:run:c588"} {
		got := ranks[run]
		if got.Rank == 0 || got.Rank <= behaviour.Rank || got.Group <= behaviour.Group {
			t.Fatalf("test run %s should rank and group after the behavioural claim: run %+v behaviour %+v", run, got, behaviour)
		}
		if got.Terms != 3 || got.Tier != "terms" {
			t.Fatalf("demotion should keep the run's served terms and tier: %+v", got)
		}
	}
	if shaped.Rank != 1 || shaped.Terms != 3 {
		t.Fatalf("a run-shaped claim without a typed test-run detail must keep its place: %+v", shaped)
	}
	// The page order is the rank order.
	for n, v := range r.Results {
		if got := ranks[recordID(v)].Rank; got != n+1 {
			t.Fatalf("result %d is %s at rank %d", n, recordID(v), got)
		}
	}
}

// DCS-V1-591 (GitHub #2014): every visible claim of a claims receipt carries
// its claim kind, read from its typed details; a claim without one is
// "unspecified", whatever its ID or text look like.
func TestDCSV1591ClaimsReceiptLabelsClaimKinds(t *testing.T) {
	t.Parallel()
	a := claimKindCorpus(t)
	for _, q := range []Request{{Operation: "claims", Query: "booking confirmation mailer", Limit: 10}, {Operation: "claims", Path: "src/value.go", Limit: 20}} {
		r := pageOf(t, a, q)
		m := receiptMap(t, r)
		kinds, _ := m["claim_kinds"].(map[string]any)
		want := map[string]string{"adapter:behaviour": "behavior", "adapter:run:a742": "test_run", "adapter:run:c588": "test_run", "adapter:run:shaped": "unspecified"}
		if q.Path != "" {
			want["adapter:claim"] = "behavior"
		}
		for id, kind := range want {
			if kinds[id] != kind {
				t.Fatalf("%+v: claim_kinds[%s] = %v, want %s (all: %v)", q, id, kinds[id], kind, kinds)
			}
		}
		if len(kinds) != len(claimResultIDs(r)) {
			t.Fatalf("%+v: every visible claim and only those should be labelled: %v", q, kinds)
		}
	}
	// Other operations keep their bytes.
	if m := receiptMap(t, pageOf(t, a, Request{Operation: "search", Query: "booking", Include: []string{"claims"}, Limit: 10})); m["claim_kinds"] != nil {
		t.Fatalf("search gained claim_kinds: %v", m["claim_kinds"])
	}
}

// DCS-V1-592 (GitHub #2014): a claims query takes a claim kind filter, which
// keeps only the named kinds, on every claims form.
func TestDCSV1592ClaimsKindFilter(t *testing.T) {
	t.Parallel()
	a := claimKindCorpus(t)
	for _, q := range []Request{{Operation: "claims", Query: "booking confirmation mailer", Kinds: []string{"test_run"}, Limit: 10}, {Operation: "claims", Path: "src/value.go", Kinds: []string{"test_run"}, Limit: 20}} {
		r, err := Query(a, q, "fresh", nil)
		if err != nil {
			t.Fatalf("%+v: %v", q, err)
		}
		got := claimResultIDs(r)
		if len(got) != 2 || got[0] != "adapter:run:a742" && got[1] != "adapter:run:a742" {
			t.Fatalf("%+v: kind filter kept %v", q, got)
		}
	}
	for _, kinds := range [][]string{{"run"}, {"test_run", "test_run"}, {}} {
		if _, err := Query(a, Request{Operation: "claims", Query: "booking", Kinds: kinds}, "fresh", nil); err == nil {
			t.Fatalf("claims kind filter %v admitted", kinds)
		}
	}
}

// claimResultIDs lists the IDs of r's claim results in order.
func claimResultIDs(r Receipt) []string {
	ids := []string{}
	for _, v := range r.Results {
		if c, ok := v.(Claim); ok {
			ids = append(ids, c.ID)
		}
	}
	return ids
}
