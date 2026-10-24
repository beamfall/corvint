package doccorpus

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// DCP-V1-322: supplied free text carries a per-field untrusted origin with its
// byte cap on both reader tails; compiler-extracted native text carries none.
func TestDCPV1322SuppliedTextOrigins(t *testing.T) {
	t.Parallel()
	want := TextOrigin{Origin: "untrusted", MaxBytes: 64 << 10}
	v2 := buildAdoption(t, suppliedHomes)
	root, m := providerFixture(t, nil)
	v1, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	if v1.Schema == v2.Schema {
		t.Fatal("fixtures must cover both reader tails")
	}
	for _, a := range []*Artifact{v1, v2} {
		r := queryResults(t, a, Request{Operation: "get", ID: "adapter:count"})
		if r.TextOrigins["adapter:count"]["name"] != want || r.TextOrigins["adapter:claim"]["text"] != want {
			t.Fatalf("schema %s: supplied text not marked: %+v", a.Schema, r.TextOrigins)
		}
		native := queryResults(t, a, Request{Operation: "get", ID: residualTestSubject})
		if native.TextOrigins != nil {
			t.Fatalf("schema %s: native text marked: %+v", a.Schema, native.TextOrigins)
		}
		out, _ := Encode(native)
		if strings.Contains(string(out), "text_origins") {
			t.Fatal("a native receipt changed its bytes")
		}
	}
	gaps := queryResults(t, v2, Request{Operation: "gaps", ID: "adapter:flow"})
	if gaps.TextOrigins["adapter:gap:flow"]["reason"] != want {
		t.Fatalf("supplied gap reason not marked: %+v", gaps.TextOrigins)
	}
}

// DCP-V1-323: association is a validity axis; a run whose association is
// not passed is served with a limitation that it does not witness its claim.
func TestDCPV1323AssociationAxisWithholdsWitness(t *testing.T) {
	t.Parallel()
	for status, limited := range map[string]bool{"passed": false, "failed": true, "unknown": true, "not-applicable": true} {
		a := buildAdoption(t, func(p *ProviderRecord) {
			fullRun(p)
			x := p.Details["adapter:run:1"].TestRun
			x.Validity = append(x.Validity, SuppliedRunValidity{Axis: "association", Status: status})
		})
		r := queryResults(t, a, Request{Operation: "get", ID: residualTestSubject})
		want := "test run adapter:run:1 association is " + status + ": the run does not witness its claim"
		if slices.Contains(r.Limitations, want) != limited {
			t.Fatalf("association %s: limitations %v", status, r.Limitations)
		}
		if got := r.Details["adapter:run:1"].TestRun.Validity; got[len(got)-1].Axis != "association" {
			t.Fatalf("association axis lost: %+v", got)
		}
	}
}

func detailClaim(id string, d RecordDetails) func(*ProviderRecord) {
	return func(p *ProviderRecord) {
		p.Claims = append(p.Claims, Claim{ID: id, Subject: "adapter:flow", Text: id + " supplied detail", Provider: p.ID, Evidence: p.Subjects[0].Evidence})
		p.Details[id] = d
	}
}

func ledger(scope, status string) *CoverageLedgerDetails {
	n := 3
	return &CoverageLedgerDetails{Scope: scope, LedgerStatus: status, BrowserEvidence: "playwright-trace", Journeys: &n, OrderedSteps: &n, Verification: "partial", Limitations: []string{"two journeys unrecorded"}}
}

func completeness(p *ProviderRecord, coverage, audit bool) *FlowCompletenessDetails {
	return &FlowCompletenessDetails{TargetStatus: "documented", CoverageComplete: CompletenessFlag{&coverage, "ledger rows reviewed"}, AuditComplete: CompletenessFlag{&audit, "audit pending"}, Revision: p.Source.Revision, Path: "docs/flows/count.md"}
}

func unresolved() *UnresolvedConceptDetails {
	return &UnresolvedConceptDetails{Concept: "bulk count export", Unplaced: []string{"ui-location", "flow"}, Reason: "anchored in source, not yet placed"}
}

// DCP-V1-324..326: the three closed detail shapes build and round-trip with
// the provider trust ceiling; unknown values, members and unanchored
// unresolved concepts are refused.
func TestDCPV1324To326CoverageDetailShapes(t *testing.T) {
	t.Parallel()
	a := buildAdoption(t, func(p *ProviderRecord) {
		detailClaim("adapter:ledger", RecordDetails{CoverageLedger: ledger("ui", "partial")})(p)
		detailClaim("adapter:complete", RecordDetails{FlowCompleteness: completeness(p, true, false)})(p)
		detailClaim("adapter:concept", RecordDetails{UnresolvedConcept: unresolved()})(p)
	})
	for _, b := range []*Artifact{a, roundTrip(t, a)} {
		r := queryResults(t, b, Request{Operation: "get", ID: "adapter:flow"})
		if x := r.Details["adapter:ledger"].CoverageLedger; x == nil || x.LedgerStatus != "partial" || *x.Journeys != 3 {
			t.Fatalf("ledger lost: %+v", x)
		}
		if x := r.Details["adapter:complete"].FlowCompleteness; x == nil || !*x.CoverageComplete.Value || *x.AuditComplete.Value {
			t.Fatalf("completeness lost: %+v", x)
		}
		if x := r.Details["adapter:concept"].UnresolvedConcept; x == nil || len(x.Unplaced) != 2 {
			t.Fatalf("unresolved concept lost: %+v", x)
		}
	}
	for _, c := range a.Claims {
		if strings.HasPrefix(c.ID, "adapter:") && c.Provider == "adapter" && c.Evidence.Trust != "generated" {
			t.Fatalf("supplied detail raised trust: %+v", c.Evidence)
		}
	}
	tooMany := 1<<20 + 1
	for name, tc := range map[string]struct {
		edit func(*ProviderRecord)
		want string
	}{
		"ledger scope":  {detailClaim("adapter:x", RecordDetails{CoverageLedger: ledger("browser", "partial")}), "invalid supplied coverage ledger"},
		"ledger status": {detailClaim("adapter:x", RecordDetails{CoverageLedger: ledger("ui", "proven")}), "invalid supplied coverage ledger"},
		"ledger count": {func(p *ProviderRecord) {
			l := ledger("ui", "confirmed")
			l.Journeys = &tooMany
			detailClaim("adapter:x", RecordDetails{CoverageLedger: l})(p)
		}, "invalid supplied coverage ledger"},
		"completeness oid": {func(p *ProviderRecord) {
			c := completeness(p, true, true)
			c.Revision = "main"
			detailClaim("adapter:x", RecordDetails{FlowCompleteness: c})(p)
		}, "invalid supplied flow completeness"},
		"completeness reason": {func(p *ProviderRecord) {
			c := completeness(p, true, true)
			c.AuditComplete.Reason = ""
			detailClaim("adapter:x", RecordDetails{FlowCompleteness: c})(p)
		}, "invalid supplied flow completeness"},
		// Codex P2: an omitted or null completeness value is refused, never read as false.
		"completeness value omitted": {func(p *ProviderRecord) {
			c := completeness(p, true, true)
			c.AuditComplete.Value = nil
			detailClaim("adapter:x", RecordDetails{FlowCompleteness: c})(p)
		}, "invalid supplied flow completeness"},
		"concept target": {func(p *ProviderRecord) {
			u := unresolved()
			u.Unplaced = []string{"page"}
			detailClaim("adapter:x", RecordDetails{UnresolvedConcept: u})(p)
		}, "invalid supplied unresolved concept"},
		"concept on subject": {func(p *ProviderRecord) { p.Details["adapter:count"] = RecordDetails{UnresolvedConcept: unresolved()} }, "invalid supplied unresolved concept"},
	} {
		t.Run(name, func(t *testing.T) {
			root, m := adoptionFixture(t, tc.edit)
			if _, err := Build(context.Background(), root, m); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
		})
	}
	var d RecordDetails
	for _, raw := range []string{
		`{"coverage_ledger":{"scope":"ui","ledger_status":"confirmed","owner":"x"}}`,
		`{"flow_completeness":{"target_status":"t","coverage_complete":{"value":true,"reason":"r","by":"x"},"audit_complete":{"value":true,"reason":"r"},"revision":"r","path":"p"}}`,
		`{"unresolved_concept":{"concept":"c","unplaced":["flow"],"reason":"r","domain":"x"}}`,
	} {
		if err := decode([]byte(raw), &d); err == nil {
			t.Fatalf("unknown member admitted: %s", raw)
		}
	}
	// Codex P2: an omitted or null value decodes to nil and fails validation.
	for _, raw := range []string{
		`{"flow_completeness":{"target_status":"documented","coverage_complete":{"reason":"r"},"audit_complete":{"value":true,"reason":"r"},"revision":"` + strings.Repeat("a", 40) + `","path":"docs/a.md"}}`,
		`{"flow_completeness":{"target_status":"documented","coverage_complete":{"value":null,"reason":"r"},"audit_complete":{"value":true,"reason":"r"},"revision":"` + strings.Repeat("a", 40) + `","path":"docs/a.md"}}`,
	} {
		var nullable RecordDetails
		if err := decode([]byte(raw), &nullable); err != nil {
			t.Fatalf("decode %s: %v", raw, err)
		}
		if err := validateRecordDetails("claim", nullable, Relation{}); err == nil || !strings.Contains(err.Error(), "invalid supplied flow completeness") {
			t.Fatalf("absent completeness value admitted: %s: %v", raw, err)
		}
	}
	// An explicit empty limitation list is refused; encoding would drop it.
	empty := ledger("ui", "confirmed")
	empty.Limitations = []string{}
	if err := validateRecordDetails("claim", RecordDetails{CoverageLedger: empty}, Relation{}); err == nil {
		t.Fatal("empty ledger limitations admitted")
	}
	root, m := adoptionFixture(t, func(p *ProviderRecord) {
		detailClaim("adapter:concept", RecordDetails{UnresolvedConcept: unresolved()})(p)
		c := &p.Claims[len(p.Claims)-1]
		c.Evidence.Anchors = []Anchor{}
		c.Evidence.State, c.Evidence.Unknown = "unknown", "source location not yet found"
	})
	if _, err := Build(context.Background(), root, m); err == nil || !strings.Contains(err.Error(), "no anchor") {
		t.Fatalf("unanchored unresolved concept: got %v", err)
	}
}

// DCP-V1-327: docs_flow, docs_gaps and docs_coverage serve the three details;
// docs_gaps selects them by subject and detail kind as claims, not gaps, and
// they change no other coverage metric.
func TestDCPV1327CoverageDetailsServed(t *testing.T) {
	t.Parallel()
	edit := func(p *ProviderRecord) {
		detailClaim("adapter:ledger", RecordDetails{CoverageLedger: ledger("ui", "partial")})(p)
		detailClaim("adapter:complete", RecordDetails{FlowCompleteness: completeness(p, true, false)})(p)
		detailClaim("adapter:concept", RecordDetails{UnresolvedConcept: unresolved()})(p)
	}
	a := buildAdoption(t, edit)
	ids := func(r Receipt) []string {
		out := []string{}
		for _, v := range r.Results {
			if c, ok := v.(Claim); ok {
				out = append(out, c.ID)
			}
		}
		return out
	}
	flow := queryResults(t, a, Request{Operation: "flow", ID: "adapter:flow"})
	for _, id := range []string{"adapter:ledger", "adapter:complete", "adapter:concept"} {
		if !slices.Contains(ids(flow), id) || flow.Details[id] == (RecordDetails{}) {
			t.Fatalf("docs_flow lost %s: %v", id, ids(flow))
		}
	}
	gaps := queryResults(t, a, Request{Operation: "gaps", ID: "adapter:flow"})
	if got := ids(gaps); !slices.Equal(slices.Sorted(slices.Values(got)), []string{"adapter:complete", "adapter:concept", "adapter:ledger"}) || gaps.Details["adapter:concept"].UnresolvedConcept == nil {
		t.Fatalf("docs_gaps details: %v", got)
	}
	only := queryResults(t, a, Request{Operation: "gaps", Kinds: []string{"unresolved_concept"}})
	if got := ids(only); !slices.Equal(got, []string{"adapter:concept"}) || only.KindTotals["unresolved_concept"] != 1 {
		t.Fatalf("docs_gaps kind filter: %v %v", got, only.KindTotals)
	}
	if other := queryResults(t, a, Request{Operation: "gaps", ID: "adapter:count"}); len(ids(other)) != 0 {
		t.Fatalf("details served for another subject: %v", ids(other))
	}
	rows := coverageRows(t, a)
	for metric, want := range map[string][2]int{
		"supplied_coverage_ledger:partial":      {1, 1},
		"supplied_flow_coverage_complete:true":  {1, 1},
		"supplied_flow_audit_complete:false":    {1, 1},
		"supplied_unresolved_concepts:unplaced": {1, 1},
	} {
		if m := rows[metric]; m == nil || m["value"] != want[0] || m["denominator"] != want[1] || m["origin"] != "supplied" {
			t.Fatalf("coverage %s = %v", metric, m)
		}
	}
	plain := coverageRows(t, buildAdoption(t, nil))
	if rows["subjects_with_claims"]["value"] != plain["subjects_with_claims"]["value"] {
		t.Fatalf("details changed subjects_with_claims: %v vs %v", rows["subjects_with_claims"], plain["subjects_with_claims"])
	}
}

// DCP-V1-328: docs_search with include:["claims"] ranks claim hits with the
// subject hits and names each hit's kind; without it the answer is unchanged.
func TestDCPV1328SearchIncludesClaims(t *testing.T) {
	t.Parallel()
	a := buildAdoption(t, nil)
	index, err := BuildQueryIndex(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	indexed := *a
	indexed.RuntimeIndex = index
	for _, b := range []*Artifact{a, &indexed} {
		plain := queryResults(t, b, Request{Operation: "search", Query: "provider-declared capability"})
		if plain.ResultKinds != nil {
			t.Fatalf("result kinds without include: %v", plain.ResultKinds)
		}
		for _, v := range plain.Results {
			if _, ok := v.(Claim); ok {
				t.Fatal("claim hit without include")
			}
		}
		r := queryResults(t, b, Request{Operation: "search", Query: "provider-declared capability", Include: []string{"claims"}})
		if r.ResultKinds["adapter:claim"] != "claim" || r.ResultKinds["adapter:count"] != "subject" {
			t.Fatalf("result kinds: %v", r.ResultKinds)
		}
		// DCS-V1-402: the claim holding every query term is the better match
		// and outranks the subject whose name holds only one of them.
		if recordID(r.Results[0]) != "adapter:claim" {
			t.Fatalf("claim holding every query term should rank first: %v", recordID(r.Results[0]))
		}
		// DCS-V1-402: when both families match every term, the subject leads.
		named := queryResults(t, b, Request{Operation: "search", Query: "count capability", Include: []string{"claims"}})
		if recordID(named.Results[0]) != "adapter:count" {
			t.Fatalf("subject name hit should rank first on a tie: %v", recordID(named.Results[0]))
		}
		// An exact claim text outranks every partial match.
		exact := queryResults(t, b, Request{Operation: "search", Query: "Count is a provider-declared capability", Include: []string{"claims"}})
		if recordID(exact.Results[0]) != "adapter:claim" {
			t.Fatalf("exact claim text should rank first: %v", recordID(exact.Results[0]))
		}
	}
	for _, q := range []Request{
		{Operation: "search", Query: "count", Include: []string{"relations"}},
		{Operation: "search", Query: "count", Include: []string{"claims", "relations"}},
		{Operation: "search", Query: "count", Include: []string{"claims"}, Sections: []string{"readme"}},
		{Operation: "get", ID: "adapter:count", Include: []string{"claims"}},
	} {
		if _, err := Query(a, q, "fresh", nil); err == nil {
			t.Fatalf("admitted %+v", q)
		}
	}
}

// DCP-V1-329: alternate ids are a closed list of exact strings, unique within
// the provider; a duplicate or an alias equal to a record id is refused.
func TestDCPV1329AlternateIDsAdmission(t *testing.T) {
	t.Parallel()
	alias := func(f func(p *ProviderRecord)) func(*ProviderRecord) { return f }
	a := buildAdoption(t, alias(func(p *ProviderRecord) { p.Subjects[0].AlternateIDs = []string{"legacy:count", "COUNT-1"} }))
	if got := roundTrip(t, a).Subjects; !slices.ContainsFunc(got, func(s Subject) bool { return slices.Equal(s.AlternateIDs, []string{"legacy:count", "COUNT-1"}) }) {
		t.Fatal("alternate ids lost on round trip")
	}
	many := make([]string, MaxAlternateIDs+1)
	for i := range many {
		many[i] = "alias:" + string(rune('a'+i%26)) + strings.Repeat("x", i)
	}
	// An explicit empty list is refused; encoding would drop it.
	if err := validateAlternateIDs([]Subject{{ID: "s", AlternateIDs: []string{}}}, map[string]bool{"s": true}); err == nil {
		t.Fatal("empty alternate id list admitted")
	}
	for name, edit := range map[string]func(*ProviderRecord){
		"empty entry": func(p *ProviderRecord) { p.Subjects[0].AlternateIDs = []string{""} },
		"repeated":    func(p *ProviderRecord) { p.Subjects[0].AlternateIDs = []string{"a", "a"} },
		"over bound":  func(p *ProviderRecord) { p.Subjects[0].AlternateIDs = many },
		"duplicate": func(p *ProviderRecord) {
			p.Subjects[0].AlternateIDs = []string{"a"}
			p.Subjects[1].AlternateIDs = []string{"a"}
		},
		"equals record id": func(p *ProviderRecord) { p.Subjects[0].AlternateIDs = []string{"adapter:flow"} },
	} {
		t.Run(name, func(t *testing.T) {
			root, m := adoptionFixture(t, edit)
			if _, err := Build(context.Background(), root, m); err == nil || !strings.Contains(err.Error(), "alternate id") {
				t.Fatalf("got %v, want an alternate id refusal", err)
			}
		})
	}
}

// DCP-V1-330: docs_get, docs_flow and docs_gaps resolve an alternate id
// exactly and record the resolution; an exact record id wins, and an
// ambiguous or unknown id keeps the existing typed miss.
func TestDCPV1330AlternateIDResolution(t *testing.T) {
	t.Parallel()
	a := buildAdoption(t, func(p *ProviderRecord) {
		suppliedHomes(p)
		for i := range p.Subjects {
			if p.Subjects[i].ID == "adapter:flow" {
				p.Subjects[i].AlternateIDs = []string{"FLOW-7"}
			}
		}
	})
	index, err := BuildQueryIndex(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	indexed := *a
	indexed.RuntimeIndex = index
	for _, b := range []*Artifact{a, roundTrip(t, a), &indexed} {
		for _, op := range []string{"get", "flow", "gaps"} {
			r := queryResults(t, b, Request{Operation: op, ID: "FLOW-7"})
			exact := queryResults(t, b, Request{Operation: op, ID: "adapter:flow"})
			if r.AliasResolution == nil || *r.AliasResolution != (AliasResolution{AlternateID: "FLOW-7", ID: "adapter:flow"}) || len(r.Results) == 0 || len(r.Results) != len(exact.Results) {
				t.Fatalf("%s by alias: %+v (%d vs %d results)", op, r.AliasResolution, len(r.Results), len(exact.Results))
			}
			if exact.AliasResolution != nil {
				t.Fatalf("%s by id recorded a resolution", op)
			}
		}
		miss := queryResults(t, b, Request{Operation: "get", ID: "adapter:none"}).Miss
		if r := queryResults(t, b, Request{Operation: "get", ID: "FLOW-8"}); miss == "" || r.Miss != miss || r.AliasResolution != nil {
			t.Fatalf("unknown alias: %s (want %s) %+v", r.Miss, miss, r.AliasResolution)
		}
	}
	ambiguous := &Artifact{Subjects: []Subject{{ID: "p:a", AlternateIDs: []string{"X"}}, {ID: "q:b", AlternateIDs: []string{"X"}}, {ID: "Y"}, {ID: "q:y", AlternateIDs: []string{"Y"}}}}
	if _, ok := resolveAlternateID(ambiguous, "X"); ok {
		t.Fatal("ambiguous alternate id resolved")
	}
	if _, ok := resolveAlternateID(ambiguous, "Y"); ok {
		t.Fatal("alternate id outranked an exact record id")
	}
	if id, ok := resolveAlternateID(ambiguous, "Z"); ok || id != "" {
		t.Fatal("unknown alternate id resolved")
	}
}

// rankingCorpus adds one flow named for login, whose ID sorts last, and five
// manual cases that mention login only in claim text; one also mentions a
// rare term.
func rankingCorpus(p *ProviderRecord) {
	ev := p.Subjects[0].Evidence
	p.Subjects = append(p.Subjects, Subject{ID: p.ID + ":zz-flow-sign-in", Kind: "use_case", Name: "Login flow", Provider: p.ID, Evidence: ev})
	for i := range 5 {
		id := p.ID + ":aa-case-" + string(rune('1'+i))
		text := "step one opens the login page"
		if i == 3 {
			text += " and exports the audit ledger"
		}
		p.Subjects = append(p.Subjects, Subject{ID: id, Kind: "manual_case", Name: "case " + id, Provider: p.ID, Evidence: ev})
		p.Claims = append(p.Claims, Claim{ID: id + ":step", Subject: id, Text: text, Provider: p.ID, Evidence: ev})
	}
}

// DCP-V1-331: docs_search ranks a name match above claim-text-only matches,
// weights rarer terms higher, breaks only equal scores by ID, keeps index and
// scan parity, and filters by subject kind.
func TestDCPV1331SearchRanking(t *testing.T) {
	t.Parallel()
	a := buildAdoption(t, rankingCorpus)
	index, err := BuildQueryIndex(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	indexed := *a
	indexed.RuntimeIndex = index
	order := func(b *Artifact, q Request) []string {
		out := []string{}
		for _, v := range queryResults(t, b, q).Results {
			out = append(out, recordID(v))
		}
		return out
	}
	login := Request{Operation: "search", Query: "login"}
	scan, fast := order(a, login), order(&indexed, login)
	if !slices.Equal(scan, fast) || !slices.Equal(scan, order(a, login)) {
		t.Fatalf("index/scan or repeat parity lost: %v vs %v", scan, fast)
	}
	if len(scan) != 6 || scan[0] != "adapter:zz-flow-sign-in" || !slices.IsSorted(scan[1:]) {
		t.Fatalf("name match should lead, then equal scores by ID: %v", scan)
	}
	rare := order(&indexed, Request{Operation: "search", Query: "login audit"})
	if len(rare) < 2 || rare[0] != "adapter:aa-case-4" {
		t.Fatalf("rarer term should weigh more: %v", rare)
	}
	if got := order(a, Request{Operation: "search", Query: "login", Kinds: []string{"manual_case"}}); len(got) != 5 || slices.Contains(got, "adapter:zz-flow-sign-in") {
		t.Fatalf("kind filter: %v", got)
	}
	withClaims := queryResults(t, &indexed, Request{Operation: "search", Query: "login", Kinds: []string{"use_case"}, Include: []string{"claims"}})
	if len(withClaims.Results) != 1 || recordID(withClaims.Results[0]) != "adapter:zz-flow-sign-in" {
		t.Fatalf("kind filter with claims: %v", withClaims.Results)
	}
	for _, q := range []Request{
		{Operation: "search", Query: "login", Kinds: []string{}},
		{Operation: "get", ID: "adapter:count", Kinds: []string{"flow"}},
	} {
		if _, err := Query(a, q, "fresh", nil); err == nil {
			t.Fatalf("admitted %+v", q)
		}
	}
}

// DCP-V1-331 (Codex P3): with include:["claims"] the exact bonus is shared
// across families, so an exact subject name match outranks a partial claim
// match whose rare claim term scores higher than the common subject term.
func TestDCPV1331ExactBonusAcrossFamilies(t *testing.T) {
	t.Parallel()
	a := buildAdoption(t, func(p *ProviderRecord) {
		ev := p.Subjects[0].Evidence
		p.Subjects = append(p.Subjects, Subject{ID: p.ID + ":zz-login", Kind: "use_case", Name: "Login", Provider: p.ID, Evidence: ev})
		for i := range 40 {
			id := fmt.Sprintf("%s:login-helper-%02d", p.ID, i)
			p.Subjects = append(p.Subjects, Subject{ID: id, Kind: "use_case", Name: "login helper " + id, Provider: p.ID, Evidence: ev})
		}
		for i := range 30 {
			text := fmt.Sprintf("unrelated step %02d", i)
			if i == 0 {
				text = "the login page opens"
			}
			p.Claims = append(p.Claims, Claim{ID: fmt.Sprintf("%s:aa-claim-%02d", p.ID, i), Subject: p.ID + ":zz-login", Text: text, Provider: p.ID, Evidence: ev})
		}
	})
	index, err := BuildQueryIndex(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	indexed := *a
	indexed.RuntimeIndex = index
	for _, b := range []*Artifact{a, &indexed} {
		first := func(query string) string {
			r := queryResults(t, b, Request{Operation: "search", Query: query, Include: []string{"claims"}})
			if len(r.Results) == 0 {
				return ""
			}
			return recordID(r.Results[0])
		}
		if got := first("login"); got != "adapter:zz-login" {
			t.Fatalf("exact subject should outrank a partial claim, got %s first", got)
		}
		if got := first("the login page opens"); got != "adapter:aa-claim-00" {
			t.Fatalf("exact claim should outrank partial subjects, got %s first", got)
		}
	}
}

// DCP-V1-329, DCP-V1-330 (Codex P2): an alias equal to a join record id is
// refused at build, and a reader given such an alias still serves the join
// record rather than redirecting through the alias.
func TestDCPV1329AliasCannotShadowJoinRecords(t *testing.T) {
	t.Parallel()
	shadow := func(p *ProviderRecord) { p.Subjects[0].AlternateIDs = []string{"audit:v1"} }
	if _, _, err := auditBuild(t, shadow, one(t, nil), nil); err == nil || !strings.Contains(err.Error(), "alternate id") {
		t.Fatalf("alias equal to a claim-audit id: got %v, want an alternate id refusal", err)
	}
	_, a, err := auditBuild(t, nil, one(t, nil), nil)
	if err != nil {
		t.Fatal(err)
	}
	a.Subjects[0].AlternateIDs = []string{"audit:v1"}
	index, err := BuildQueryIndex(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	indexed := *a
	indexed.RuntimeIndex = index
	for _, b := range []*Artifact{a, &indexed} {
		r := queryResults(t, b, Request{Operation: "get", ID: "audit:v1"})
		if r.AliasResolution != nil || len(r.Results) == 0 {
			t.Fatalf("alias shadowed the claim-audit record: %+v", r.AliasResolution)
		}
		if _, ok := r.Results[0].(ClaimAuditJoin); !ok {
			t.Fatalf("get audit:v1 served %T, want the claim-audit join", r.Results[0])
		}
	}
}
