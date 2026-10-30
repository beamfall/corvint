package doccorpus

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// coverageRows returns the docs_coverage metric rows by metric name.
func coverageRows(t *testing.T, a *Artifact) map[string]map[string]any {
	t.Helper()
	rows := map[string]map[string]any{}
	for _, row := range queryResults(t, a, Request{Operation: "coverage"}).Results {
		if m, ok := row.(map[string]any); ok && m["metric"] != nil {
			rows[fmt.Sprint(m["metric"])] = m
		}
	}
	return rows
}

// roundTrip encodes a built artifact and opens it again, as a reader does.
func roundTrip(t *testing.T, a *Artifact) *Artifact {
	t.Helper()
	raw, err := Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	var b Artifact
	if err := decodeBounded(raw, &b, len(raw)); err != nil {
		t.Fatal(err)
	}
	return &b
}

// statusClaims adds one status claim on the fixture flow per supplied detail.
func statusClaims(details ...SuppliedStatusDetails) func(*ProviderRecord) {
	return func(p *ProviderRecord) {
		ev := p.Subjects[0].Evidence
		for i, d := range details {
			id := fmt.Sprintf("adapter:status:%02d", i)
			p.Claims = append(p.Claims, Claim{ID: id, Subject: "adapter:flow", Text: "Flow status " + d.State, Provider: p.ID, Evidence: ev})
			detail := d
			p.Details[id] = RecordDetails{Status: &detail}
		}
	}
}

// DCP-V1-300, DCP-V1-301, DCP-V1-302: every widened state builds, is returned
// by docs_get and counted per state; a qualified proven counts apart from
// plain proven; unknown is a counted state while an absent status adds no row.
func TestDCPV1300WidenedStatusVocabulary(t *testing.T) {
	t.Parallel()
	states := []string{"blocked", "ready", "proven", "partial", "not-demonstrable", "disputed", "obsolete", "not-established", "unknown"}
	details := []SuppliedStatusDetails{}
	for _, s := range states {
		details = append(details, SuppliedStatusDetails{State: s, Reason: "ledger row " + s, Supporting: []string{}})
	}
	details = append(details,
		SuppliedStatusDetails{State: "proven", Qualifier: "provisional", Reason: "provisional proof", Supporting: []string{}},
		SuppliedStatusDetails{State: "proven", Qualifier: "classification-only", Reason: "classified, not demonstrated", Supporting: []string{}})
	a := buildAdoption(t, statusClaims(details...))
	r := queryResults(t, a, Request{Operation: "get", ID: "adapter:flow"})
	got := map[string]int{}
	for _, d := range r.Details {
		if d.Status != nil {
			got[d.Status.State+"/"+d.Status.Qualifier]++
		}
	}
	for _, s := range states {
		if got[s+"/"] != 1 {
			t.Fatalf("docs_get lost status %q: %v", s, got)
		}
	}
	if got["proven/provisional"] != 1 || got["proven/classification-only"] != 1 {
		t.Fatalf("docs_get lost the qualifier: %v", got)
	}
	rows := coverageRows(t, a)
	total := len(details)
	for _, metric := range append(slices.Clone(states), "proven:provisional", "proven:classification-only") {
		m := rows["supplied_status:"+metric]
		if m == nil || m["value"] != 1 || m["denominator"] != total || m["origin"] != "supplied" {
			t.Fatalf("coverage row supplied_status:%s = %v", metric, m)
		}
	}
	// A qualified proven is never counted as plain proven.
	if rows["supplied_status:proven"]["value"] != 1 {
		t.Fatalf("qualified proven counted as plain proven: %v", rows["supplied_status:proven"])
	}
	// The qualifier survives the artifact round trip.
	b := roundTrip(t, a)
	if d := b.Details["adapter:status:09"].Status; d == nil || d.State != "proven" || d.Qualifier != "provisional" {
		t.Fatalf("qualifier lost in the artifact: %+v", d)
	}
	// unknown is a supplied state, distinct from a subject with no status.
	plain := buildAdoption(t, nil)
	out, _ := Encode(queryResults(t, plain, Request{Operation: "coverage"}))
	if bytes.Contains(out, []byte("supplied_status")) {
		t.Fatal("a corpus without statuses gained a status row")
	}
	only := buildAdoption(t, statusClaims(SuppliedStatusDetails{State: "unknown", Reason: "independent verification missing", Supporting: []string{}}))
	if m := coverageRows(t, only)["supplied_status:unknown"]; m == nil || m["value"] != 1 || m["denominator"] != 1 {
		t.Fatalf("unknown status not counted: %v", m)
	}
	// A status still grants no trust.
	for _, c := range a.Claims {
		if strings.HasPrefix(c.ID, "adapter:status:") && c.Evidence.Trust != "generated" {
			t.Fatalf("supplied status raised trust: %+v", c.Evidence)
		}
	}
}

// DCP-V1-300: a state or qualifier outside the closed sets is refused.
func TestDCPV1300StatusOutsideVocabularyRefused(t *testing.T) {
	t.Parallel()
	for name, d := range map[string]SuppliedStatusDetails{
		"unknown state":         {State: "verified", Reason: "r", Supporting: []string{}},
		"empty state":           {State: "", Reason: "r", Supporting: []string{}},
		"case-folded state":     {State: "Proven", Reason: "r", Supporting: []string{}},
		"unknown qualifier":     {State: "proven", Qualifier: "tentative", Reason: "r", Supporting: []string{}},
		"qualifier as a state":  {State: "provisional", Reason: "r", Supporting: []string{}},
		"state with whitespace": {State: "proven ", Reason: "r", Supporting: []string{}},
	} {
		t.Run(name, func(t *testing.T) {
			root, m := adoptionFixture(t, statusClaims(d))
			if _, err := Build(context.Background(), root, m); err == nil || !strings.Contains(err.Error(), "invalid supplied status") {
				t.Fatalf("got %v, want invalid supplied status", err)
			}
		})
	}
	var d RecordDetails
	if err := decode([]byte(`{"status":{"state":"proven","reason":"r","supporting":[],"grade":"A"}}`), &d); err == nil {
		t.Fatal("unknown status member admitted")
	}
}

// fullRun is a supplied test run carrying every optional ledger field.
func fullRun(p *ProviderRecord) {
	suppliedTestRun(p)
	retries := 2
	x := p.Details["adapter:run:1"].TestRun
	x.Retries = &retries
	x.FlakeVerdict = "flaky"
	x.Cleanup = "partial"
	x.ControlOutcome = "failed"
	x.Validity = []SuppliedRunValidity{{Axis: "revision", Status: "passed"}, {Axis: "environment", Status: "unknown"}, {Axis: "isolation", Status: "failed"}, {Axis: "oracle", Status: "passed"}, {Axis: "determinism", Status: "not-applicable"}}
}

// DCP-V1-303, DCP-V1-305: a run with every ledger field builds, round-trips
// through the artifact and docs_get, and is counted by flake verdict and
// cleanup; a zero retry count is kept, and no field raises trust.
func TestDCPV1303TestRunLedgerFieldsRoundTrip(t *testing.T) {
	t.Parallel()
	a := buildAdoption(t, func(p *ProviderRecord) {
		fullRun(p)
		p.Claims[len(p.Claims)-1].Evidence.Trust = "reviewed"
		p.Claims[len(p.Claims)-1].Evidence.Derivation = "imported"
		ev := p.Subjects[0].Evidence
		zero := 0
		p.Claims = append(p.Claims, Claim{ID: "adapter:run:2", Subject: residualTestSubject, Text: "TestCount passed in run 42", Provider: p.ID, Evidence: ev})
		p.Details["adapter:run:2"] = RecordDetails{TestRun: &SuppliedTestRunDetails{Outcome: "passed", Run: "ci-run-42", Origin: "supplied", Retries: &zero, FlakeVerdict: "stable", Cleanup: "passed"}}
	})
	for _, b := range []*Artifact{a, roundTrip(t, a)} {
		r := queryResults(t, b, Request{Operation: "get", ID: residualTestSubject})
		x := r.Details["adapter:run:1"].TestRun
		if x == nil || x.Retries == nil || *x.Retries != 2 || x.FlakeVerdict != "flaky" || x.Cleanup != "partial" || x.ControlOutcome != "failed" || len(x.Validity) != 5 || x.Validity[2] != (SuppliedRunValidity{Axis: "isolation", Status: "failed"}) {
			t.Fatalf("docs_get lost the run ledger: %+v", x)
		}
		if y := r.Details["adapter:run:2"].TestRun; y == nil || y.Retries == nil || *y.Retries != 0 || y.Validity != nil {
			t.Fatalf("zero retries or absent validity not kept: %+v", y)
		}
	}
	for _, c := range a.Claims {
		if c.ID == "adapter:run:1" && (c.Evidence.Trust != "generated" || c.Evidence.ReportedTrust != "reviewed") {
			t.Fatalf("supplied run ledger raised trust: %+v", c.Evidence)
		}
	}
	rows := coverageRows(t, a)
	for metric, want := range map[string][2]int{
		"supplied_test_run_flake:flaky":     {1, 2},
		"supplied_test_run_flake:stable":    {1, 2},
		"supplied_test_run_cleanup:partial": {1, 2},
		"supplied_test_run_cleanup:passed":  {1, 2},
		"supplied_test_run:passed":          {2, 2},
	} {
		if m := rows[metric]; m == nil || m["value"] != want[0] || m["denominator"] != want[1] || m["origin"] != "supplied" {
			t.Fatalf("coverage %s = %v, want %v", metric, m, want)
		}
	}
	// A run without the optional fields adds no flake or cleanup row.
	plainRun := buildAdoption(t, suppliedTestRun)
	out, _ := Encode(queryResults(t, plainRun, Request{Operation: "coverage"}))
	if bytes.Contains(out, []byte("supplied_test_run_flake")) || bytes.Contains(out, []byte("supplied_test_run_cleanup")) {
		t.Fatal("a run without ledger fields gained flake or cleanup rows")
	}
	raw, _ := Encode(plainRun)
	if bytes.Contains(raw, []byte(`"retries"`)) || bytes.Contains(raw, []byte(`"validity"`)) {
		t.Fatal("a run without ledger fields changed its artifact bytes")
	}
}

// DCP-V1-303: an unknown ledger value or member is refused.
func TestDCPV1303TestRunLedgerUnknownValuesRefused(t *testing.T) {
	t.Parallel()
	edit := func(f func(*SuppliedTestRunDetails)) func(*ProviderRecord) {
		return func(p *ProviderRecord) {
			fullRun(p)
			f(p.Details["adapter:run:1"].TestRun)
		}
	}
	negative, over := -1, maxSuppliedRunRetries+1
	for name, f := range map[string]func(*ProviderRecord){
		"negative retries":        edit(func(x *SuppliedTestRunDetails) { x.Retries = &negative }),
		"retries over bound":      edit(func(x *SuppliedTestRunDetails) { x.Retries = &over }),
		"unknown flake verdict":   edit(func(x *SuppliedTestRunDetails) { x.FlakeVerdict = "intermittent" }),
		"unknown cleanup":         edit(func(x *SuppliedTestRunDetails) { x.Cleanup = "skipped" }),
		"unknown control outcome": edit(func(x *SuppliedTestRunDetails) { x.ControlOutcome = "flaky" }),
		"unknown validity axis":   edit(func(x *SuppliedTestRunDetails) { x.Validity[0].Axis = "coverage" }),
		"unknown validity status": edit(func(x *SuppliedTestRunDetails) { x.Validity[0].Status = "valid" }),
		"repeated validity axis":  edit(func(x *SuppliedTestRunDetails) { x.Validity[1].Axis = "revision" }),
	} {
		t.Run(name, func(t *testing.T) {
			root, m := adoptionFixture(t, f)
			if _, err := Build(context.Background(), root, m); err == nil || !strings.Contains(err.Error(), "invalid supplied test run ledger field") {
				t.Fatalf("got %v, want the ledger refusal", err)
			}
		})
	}
	var d RecordDetails
	if err := decode([]byte(`{"test_run":{"outcome":"passed","run":"r1","origin":"supplied","attempts":2}}`), &d); err == nil {
		t.Fatal("unknown test_run member admitted")
	}
	// An explicit empty validity list is refused; an absent one is admitted.
	if err := decode([]byte(`{"test_run":{"outcome":"passed","run":"r1","origin":"supplied","validity":[]}}`), &d); err != nil {
		t.Fatal(err)
	}
	if err := validateRecordDetails("claim", d, Relation{}); err == nil || !strings.Contains(err.Error(), "invalid supplied test run ledger field") {
		t.Fatalf("empty validity list: got %v", err)
	}
}

// DCP-V1-304: a supplied gap that targets a run claim builds and is returned
// by docs_gaps for that claim and for the claim's subject; a gap whose target
// is neither a subject nor a claim is still refused.
func TestDCPV1304SuppliedGapTargetsRunClaim(t *testing.T) {
	t.Parallel()
	runGap := func(p *ProviderRecord) {
		suppliedTestRun(p)
		p.Gaps = []SuppliedGap{{ID: "adapter:gap:run", Subject: "adapter:run:1", Kind: "flaky", Reason: "run 41 passed only on retry", Provider: p.ID, Evidence: p.Subjects[0].Evidence}}
	}
	a := buildAdoption(t, runGap)
	for _, indexed := range []bool{false, true} {
		b := *a
		if indexed {
			x, err := BuildQueryIndex(context.Background(), a)
			if err != nil {
				t.Fatal(err)
			}
			b.RuntimeIndex = x
		}
		for _, id := range []string{"adapter:run:1", residualTestSubject, "adapter:gap:run"} {
			_, supplied := gapsFor(t, &b, id)
			if len(supplied) != 1 || supplied[0].ID != "adapter:gap:run" || supplied[0].Subject != "adapter:run:1" {
				t.Fatalf("indexed=%v: docs_gaps %s lost the run gap: %+v", indexed, id, supplied)
			}
		}
		if _, supplied := gapsFor(t, &b, "adapter:flow"); len(supplied) != 0 {
			t.Fatalf("indexed=%v: run gap leaked to an unrelated subject: %+v", indexed, supplied)
		}
	}
	root, m := adoptionFixture(t, func(p *ProviderRecord) {
		runGap(p)
		p.Gaps[0].Subject = "ci-run-41"
	})
	if _, err := Build(context.Background(), root, m); err == nil || !strings.Contains(err.Error(), "supplied gap subject missing") {
		t.Fatalf("a gap naming a bare run identifier was admitted: %v", err)
	}
}

// migrationLedger declares three baseline and three target inventory units,
// the mappings moved, removed and ambiguous between them, and a readiness
// status on the moved unit's target (GitHub #1491).
func migrationLedger(p *ProviderRecord) {
	ev := p.Subjects[0].Evidence
	for _, id := range []string{"old:1", "old:2", "old:3", "new:1", "new:2", "new:3"} {
		p.Subjects = append(p.Subjects, Subject{ID: "adapter:unit:" + id, Kind: "inventory", Name: "unit " + id, Provider: p.ID, Evidence: ev})
	}
	mapping := func(id, from, to, outcome, basis string) {
		p.Relations = append(p.Relations, Relation{ID: "adapter:map:" + id, From: "adapter:unit:" + from, To: "adapter:unit:" + to, Type: "migration_mapping", Provider: p.ID, Evidence: ev})
		p.Details["adapter:map:"+id] = RecordDetails{MigrationMapping: &MigrationMappingDetails{Outcome: outcome, Basis: basis, Run: "migration-run-1"}}
	}
	mapping("1", "old:1", "new:1", "moved", "region_hash")
	mapping("2", "old:2", "old:2", "removed", "stable_id")
	mapping("3a", "old:3", "new:2", "ambiguous", "region_hash")
	mapping("3b", "old:3", "new:3", "ambiguous", "region_hash")
	p.Claims = append(p.Claims, Claim{ID: "adapter:ready:new:1", Subject: "adapter:unit:new:1", Text: "new:1 readiness is unknown", Provider: p.ID, Evidence: ev})
	p.Details["adapter:ready:new:1"] = RecordDetails{Status: &SuppliedStatusDetails{State: "unknown", Reason: "independent verification missing", Supporting: []string{"adapter:map:1"}}}
}

// DCP-V1-306, DCP-V1-307: a migration ledger builds; a unit read returns its
// mapping chain with typed details and its readiness, with or without the
// query index, and import_parity counts the mappings.
func TestDCPV1306MigrationMappingLedger(t *testing.T) {
	t.Parallel()
	a := buildAdoption(t, migrationLedger)
	parity := map[string]int{}
	for _, p := range a.ImportParity {
		parity[p.SourceKind] = p.Admitted
	}
	if parity["relation/migration_mapping"] != 4 || parity["subject/inventory"] < 6 {
		t.Fatalf("import_parity lacks the ledger: %v", parity)
	}
	for _, indexed := range []bool{false, true} {
		b := *a
		if indexed {
			x, err := BuildQueryIndex(context.Background(), a)
			if err != nil {
				t.Fatal(err)
			}
			b.RuntimeIndex = x
		}
		ids := func(id string) (Receipt, []string) {
			r := queryResults(t, &b, Request{Operation: "get", ID: id})
			out := []string{}
			for _, v := range r.Results {
				out = append(out, recordID(v))
			}
			return r, out
		}
		r, got := ids("adapter:unit:new:1")
		if fmt.Sprint(got) != "[adapter:unit:new:1 adapter:ready:new:1 adapter:map:1]" {
			t.Fatalf("indexed=%v: unit read = %v", indexed, got)
		}
		if d := r.Details["adapter:map:1"].MigrationMapping; d == nil || d.Outcome != "moved" || d.Basis != "region_hash" || d.Run != "migration-run-1" {
			t.Fatalf("indexed=%v: mapping detail lost: %+v", indexed, r.Details)
		}
		if d := r.Details["adapter:ready:new:1"].Status; d == nil || d.State != "unknown" {
			t.Fatalf("indexed=%v: readiness lost: %+v", indexed, r.Details)
		}
		if _, got := ids("adapter:unit:old:2"); fmt.Sprint(got) != "[adapter:unit:old:2 adapter:map:2]" {
			t.Fatalf("indexed=%v: removed unit read = %v", indexed, got)
		}
		// The chain spans both ambiguous candidates from either end.
		if _, got := ids("adapter:unit:new:3"); fmt.Sprint(got) != "[adapter:unit:new:3 adapter:map:3a adapter:map:3b]" {
			t.Fatalf("indexed=%v: ambiguous chain = %v", indexed, got)
		}
	}
	// A later migration extends the chain transitively.
	chained := buildAdoption(t, func(p *ProviderRecord) {
		migrationLedger(p)
		p.Subjects = append(p.Subjects, Subject{ID: "adapter:unit:next:1", Kind: "inventory", Name: "unit next:1", Provider: p.ID, Evidence: p.Subjects[0].Evidence})
		p.Relations = append(p.Relations, Relation{ID: "adapter:map:4", From: "adapter:unit:new:1", To: "adapter:unit:next:1", Type: "migration_mapping", Provider: p.ID, Evidence: p.Subjects[0].Evidence})
		p.Details["adapter:map:4"] = RecordDetails{MigrationMapping: &MigrationMappingDetails{Outcome: "renamed", Basis: "symbol"}}
	})
	r := queryResults(t, chained, Request{Operation: "get", ID: "adapter:unit:old:1"})
	got := []string{}
	for _, v := range r.Results {
		got = append(got, recordID(v))
	}
	if fmt.Sprint(got) != "[adapter:unit:old:1 adapter:map:1 adapter:map:4]" {
		t.Fatalf("chain across two migrations = %v", got)
	}
	// A read of a subject outside the ledger is unchanged.
	results := func(a *Artifact) string {
		out := []string{}
		for _, v := range queryResults(t, a, Request{Operation: "get", ID: "adapter:flow"}).Results {
			out = append(out, recordID(v))
		}
		return fmt.Sprint(out)
	}
	if before, after := results(buildAdoption(t, nil)), results(a); before != after {
		t.Fatalf("a migration ledger changed an unrelated get: %s -> %s", before, after)
	}
}

// DCP-V1-306: an unknown outcome or basis, a wrong-sided mapping, a missing
// detail and a mapping that touches a non-inventory subject are refused.
func TestDCPV1306MigrationMappingRefusals(t *testing.T) {
	t.Parallel()
	edit := func(f func(*ProviderRecord)) func(*ProviderRecord) {
		return func(p *ProviderRecord) {
			migrationLedger(p)
			f(p)
		}
	}
	detail := func(p *ProviderRecord, id string) *MigrationMappingDetails {
		return p.Details["adapter:map:"+id].MigrationMapping
	}
	relation := func(p *ProviderRecord, id string) *Relation {
		for i := range p.Relations {
			if p.Relations[i].ID == "adapter:map:"+id {
				return &p.Relations[i]
			}
		}
		panic(id)
	}
	for name, c := range map[string]struct {
		edit func(*ProviderRecord)
		want string
	}{
		"unknown outcome":           {edit(func(p *ProviderRecord) { detail(p, "1").Outcome = "relocated" }), "invalid migration mapping"},
		"unknown basis":             {edit(func(p *ProviderRecord) { detail(p, "1").Basis = "guess" }), "invalid migration mapping"},
		"oversized run":             {edit(func(p *ProviderRecord) { detail(p, "1").Run = strings.Repeat("r", 1025) }), "invalid migration mapping"},
		"one-sided moved":           {edit(func(p *ProviderRecord) { relation(p, "1").To = "adapter:unit:old:1" }), "invalid migration mapping"},
		"two-sided removed":         {edit(func(p *ProviderRecord) { relation(p, "2").To = "adapter:unit:new:2" }), "invalid migration mapping"},
		"missing detail":            {edit(func(p *ProviderRecord) { delete(p.Details, "adapter:map:1") }), "migration mapping requires typed details"},
		"mapping detail on a claim": {edit(func(p *ProviderRecord) { p.Details["adapter:claim"] = p.Details["adapter:map:1"] }), "invalid migration mapping"},
		"target is not inventory":   {edit(func(p *ProviderRecord) { relation(p, "1").To = "adapter:flow" }), "migration mapping must join inventory subjects"},
		"source is not inventory":   {edit(func(p *ProviderRecord) { relation(p, "1").From = "adapter:count" }), "migration mapping must join inventory subjects"},
	} {
		t.Run(name, func(t *testing.T) {
			root, m := adoptionFixture(t, c.edit)
			if _, err := Build(context.Background(), root, m); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want %q", err, c.want)
			}
		})
	}
	root, m := providerFixture(t, func(p *ProviderRecord) {
		p.Subjects = append(p.Subjects, Subject{ID: "adapter:unit:a", Kind: "inventory", Name: "a", Provider: p.ID, Evidence: p.Subjects[0].Evidence})
		p.Relations = append(p.Relations, Relation{ID: "adapter:map", From: "adapter:unit:a", To: "adapter:unit:a", Type: "migration_mapping", Provider: p.ID, Evidence: p.Subjects[0].Evidence})
	})
	if _, err := Build(context.Background(), root, m); err == nil {
		t.Fatal("a non-adoption provider declared a migration mapping")
	}
}
