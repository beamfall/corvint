package doccorpus

import (
	"context"
	"strings"
	"testing"
)

func TestCorpusTypedQueryConformance(t *testing.T) {
	t.Run("DCP-V1-038 DCP-V1-039 typed documented evidence", func(t *testing.T) {
		root, m := adoptionFixture(t, func(p *ProviderRecord) {
			ev := p.Subjects[0].Evidence
			flow := *p.Details["adapter:flow"].Flow
			flow.Sections = append([]FlowSection{}, flow.Sections...)
			for i := range flow.Sections {
				flow.Sections[i].Paragraphs = []string{}
			}
			p.Subjects = append(p.Subjects, Subject{"adapter:flow2", "flow", "Checkout completion", "adapter", ev}, Subject{"adapter:term", "business_term", "purchase checkout", "adapter", ev}, Subject{"adapter:screen", "ui_surface", "Checkout screen", "adapter", ev})
			p.Details["adapter:flow2"] = RecordDetails{Flow: &flow}
			for _, rel := range []Relation{{"adapter:down", "adapter:flow", "adapter:flow2", "depends_on", "adapter", ev}, {"adapter:cycle", "adapter:flow2", "adapter:flow", "depends_on", "adapter", ev}, {"adapter:alias", "adapter:term", "adapter:flow", "related_to", "adapter", ev}, {"adapter:route", "adapter:screen", "adapter:flow", "related_to", "adapter", ev}} {
				p.Relations = append(p.Relations, rel)
			}
		})
		a, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		cases := []Request{{Operation: "concept", Query: "Count"}, {Operation: "claims", Path: "src/value.go"}, {Operation: "flow", ID: "adapter:flow"}, {Operation: "dependencies", ID: "adapter:flow"}, {Operation: "recommend-tests", Path: "src/value.go"}, {Operation: "navigation", ID: "adapter:flow"}, {Operation: "vocabulary", Query: "purchase"}, {Operation: "intent", ID: "adapter:intent"}}
		for _, q := range cases {
			t.Run(q.Operation, func(t *testing.T) {
				r, err := Query(a, q, "fresh", nil)
				if err != nil || len(r.Results) == 0 || r.Envelope == nil || r.Envelope.SourceRevision != m.Repository.Revision || r.Envelope.CorpusRevision != a.SHA256 {
					t.Fatalf("lost typed evidence %s: %+v %v", q.Operation, r, err)
				}
				if len(r.Citations) == 0 || len(r.Envelope.Citations) != len(r.Citations) {
					t.Fatal("page citations lost")
				}
				if q.Operation == "flow" && len(r.Details["adapter:flow"].Flow.Sections) != 8 {
					t.Fatal("eight sections lost")
				}
				if q.Operation == "recommend-tests" && (r.Selection == nil || r.Selection.NarrowingAllowed || r.Selection.State != "full-relevant-suite-required") {
					t.Fatal("unsafe narrowing")
				}
				if q.Operation == "dependencies" && len(r.Results) != 4 {
					t.Fatal("cycle direction or termination lost", r.Results)
				}
			})
		}
		for _, op := range []string{"info", "validate", "search", "get", "trace", "locate", "related", "coverage", "gaps", "journey", "stability", "inventory"} {
			q := Request{Operation: op, ID: "missing", Query: "quokka", Path: "missing.go"}
			r, e := Query(a, q, "stale", nil)
			if e != nil || r.Envelope == nil || r.Envelope.Freshness != "stale" {
				t.Fatal(op, e)
			}
			if r.State == "empty" && r.Meaning != "not documented" {
				t.Fatal("empty claimed nonexistence")
			}
			if r.State == "unavailable" && r.Miss != "capability-absent" {
				t.Fatal("capability miss flattened")
			}
		}
		index, e := BuildQueryIndex(context.Background(), a)
		if e != nil {
			t.Fatal(e)
		}
		copy := *a
		copy.RuntimeIndex = index
		for _, q := range cases {
			r, e := Query(a, q, "fresh", nil)
			if e != nil {
				t.Fatal(e)
			}
			other, e := Query(&copy, q, "fresh", nil)
			if e != nil {
				t.Fatal(e)
			}
			left, _ := Encode(r)
			right, _ := Encode(other)
			if string(left) != string(right) {
				t.Fatal("indexed native parity", q.Operation)
			}
		}
		r, e := Query(a, Request{Operation: "gaps"}, "stale", nil)
		if e != nil {
			t.Fatal(e)
		}
		kinds := map[string]bool{}
		for _, v := range r.Results {
			if g, ok := v.(Gap); ok {
				kinds[g.Kind] = true
			}
		}
		for _, kind := range []string{"no-tests", "missing-asserting-e2e-tests", "missing-manual-cases", "stale-anchors"} {
			if !kinds[kind] {
				t.Fatal("missing named gap", kind)
			}
		}
	})
}

func TestCorpusTypedMultiFileCoverageAndSelectors(t *testing.T) {
	t.Run("DCP-V1-038 separate selectors and explicit cross-file coverage", func(t *testing.T) {
		members := []string{"native:file:src/value.go:excerpt", "native:file:src/value_test.go:excerpt"}
		root, manifest := adoptionFixture(t, func(p *ProviderRecord) {
			p.Details["adapter:coverage"] = RecordDetails{Coverage: &CoverageDetails{
				Definition: "claims across two pinned source files", Rule: "explicit-membership",
				Denominator: members, Numerator: members[:1],
			}}
		})
		artifact, err := Build(context.Background(), root, manifest)
		if err != nil {
			t.Fatal(err)
		}
		index, err := BuildQueryIndex(context.Background(), artifact)
		if err != nil {
			t.Fatal(err)
		}
		indexed := *artifact
		indexed.RuntimeIndex = index
		seen := map[string]bool{}
		for i, path := range []string{"src/value.go", "src/value_test.go"} {
			request := Request{Operation: "claims", Path: path}
			receipt, err := Query(artifact, request, "fresh", nil)
			if err != nil || len(receipt.Results) == 0 {
				t.Fatal("file selector lost claims", path, err)
			}
			other, err := Query(&indexed, request, "fresh", nil)
			if err != nil {
				t.Fatal(err)
			}
			left, _ := Encode(receipt)
			right, _ := Encode(other)
			if string(left) != string(right) {
				t.Fatal("precomputed index changed complete receipt", path)
			}
			found := false
			for _, result := range receipt.Results {
				claim := result.(Claim)
				if seen[claim.ID] {
					t.Fatal("distinct paths selected the same claim", path, claim.ID)
				}
				seen[claim.ID] = true
				found = found || claim.ID == members[i]
				if len(claim.Evidence.Anchors) == 0 {
					t.Fatal("claim lost file evidence", claim.ID)
				}
				for _, anchor := range claim.Evidence.Anchors {
					if anchor.Path != path {
						t.Fatal("selector returned a different file", path, anchor)
					}
				}
			}
			if !found {
				t.Fatal("selector lost exact native excerpt", path, members[i])
			}
			recommendation, err := Query(artifact, Request{Operation: "recommend-tests", Path: path}, "fresh", nil)
			if err != nil || recommendation.Selection == nil || recommendation.Selection.NarrowingAllowed || recommendation.Selection.State != "full-relevant-suite-required" {
				t.Fatal("cross-file evidence allowed unsafe narrowing", path, err, recommendation.Selection)
			}
		}
		get, err := Query(artifact, Request{Operation: "get", ID: "adapter:coverage"}, "fresh", nil)
		if err != nil {
			t.Fatal(err)
		}
		detail := get.Details["adapter:coverage"].Coverage
		if detail == nil || len(detail.Denominator) != 2 || detail.Denominator[0] != members[0] || detail.Denominator[1] != members[1] || len(detail.Numerator) != 1 || detail.Numerator[0] != members[0] {
			t.Fatal("cross-file coverage lost membership identities", detail)
		}
		coverage, err := Query(artifact, Request{Operation: "coverage"}, "fresh", nil)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, result := range coverage.Results {
			row, ok := result.(map[string]any)
			if !ok || row["metric"] != "adapter:coverage" {
				continue
			}
			found = true
			if row["value"] != 1 || row["denominator"] != 2 || row["defined"] != true || row["definition"] != "claims across two pinned source files" || row["rule"] != "explicit-membership" || row["revision"] != manifest.Repository.Revision || len(row["limitations"].([]string)) == 0 {
				t.Fatal("coverage changed its named denominator or limits", row)
			}
		}
		if !found {
			t.Fatal("cross-file coverage metric missing")
		}
	})
}

func TestCorpusRetirementBindings(t *testing.T) {
	t.Run("DCP-V1-039 age distance and unknown provenance", func(t *testing.T) {
		a := &Artifact{Schema: SchemaV2, SHA256: strings.Repeat("a", 64), Manifest: Manifest{Repository: Repository{Revision: strings.Repeat("b", 40)}, BuiltAt: "2026-09-19T00:00:00Z"}, Capabilities: []Capability{{Name: "subjects", State: "present"}}}
		age := int64(10)
		distance := 3
		maxDistance := 2
		p := &RetirementPolicy{EvaluatedAt: "2026-09-20T00:00:00Z", MaxAgeSeconds: &age, MaxMergeDistance: &maxDistance, MergeDistance: &distance, SourceRevision: a.Manifest.Repository.Revision, TargetRevision: strings.Repeat("c", 40)}
		r, e := Query(a, Request{Operation: "search", Query: "missing", Retirement: p}, "unknown", nil)
		if e != nil || r.Envelope.Retirement.State != "retired-by-policy" || r.Miss != "no-match" || r.Meaning != "not documented" {
			t.Fatal(r, e)
		}
		for _, edit := range []func(*RetirementPolicy){func(p *RetirementPolicy) { p.SourceRevision = strings.Repeat("d", 40) }, func(p *RetirementPolicy) { p.EvaluatedAt = "2026-09-18T00:00:00Z" }, func(p *RetirementPolicy) { n := -1; p.MergeDistance = &n }, func(p *RetirementPolicy) { p.TargetRevision = p.SourceRevision }} {
			bad := *p
			edit(&bad)
			if _, e := Query(a, Request{Operation: "search", Query: "missing", Retirement: &bad}, "fresh", nil); e == nil {
				t.Fatal("unbound retirement admitted")
			}
		}
		noPolicy, e := Query(a, Request{Operation: "search", Query: "missing"}, "fresh", nil)
		if e != nil || noPolicy.Envelope.Retirement.State != "unknown" {
			t.Fatal("invented retirement measurement")
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, e := QueryContext(ctx, a, Request{Operation: "search", Query: "missing"}, "fresh", nil); e == nil {
			t.Fatal("cancelled query ran")
		}
	})
}

func TestCorpusRetirementBounds(t *testing.T) {
	t.Run("DCP-V1-039 bounded retirement declarations and unknown measurements", func(t *testing.T) {
		a := &Artifact{Schema: SchemaV2, SHA256: strings.Repeat("a", 64), Manifest: Manifest{Repository: Repository{Revision: strings.Repeat("b", 40)}, BuiltAt: "2026-09-19T00:00:00Z"}, Capabilities: []Capability{{Name: "subjects", State: "present"}}}
		for _, body := range []string{
			`{"evaluated_at":"2026-09-20T00:00:00Z","max_age_seconds":3155760001}`,
			`{"evaluated_at":"2026-09-20T00:00:00Z","max_age_seconds":-1}`,
			`{"evaluated_at":"2026-09-20T00:00:00Z","max_merge_distance":1000001}`,
			`{"evaluated_at":"2026-09-20T00:00:00Z","max_merge_distance":-1}`,
			`{"evaluated_at":"2026-09-20T00:00:00Z","max_merge_distance":1000000,"merge_distance":1000001,"source_revision":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","target_revision":"cccccccccccccccccccccccccccccccccccccccc"}`,
			`{"evaluated_at":"2026-09-20T00:00:00Z","max_age_seconds":9223372036854775808}`,
		} {
			p, err := ParseRetirement([]byte(body))
			if err == nil {
				_, err = Query(a, Request{Operation: "search", Query: "none", Retirement: p}, "fresh", nil)
			}
			if err == nil {
				t.Fatal("out-of-bound retirement accepted", body)
			}
		}
		maxAge := int64(3155760000)
		maxDistance := 1000000
		p := &RetirementPolicy{EvaluatedAt: "2026-09-20T00:00:00Z", MaxAgeSeconds: &maxAge, MaxMergeDistance: &maxDistance}
		r, err := Query(a, Request{Operation: "search", Query: "none", Retirement: p}, "fresh", nil)
		if err != nil || r.Envelope.Retirement.State != "unknown" || r.Envelope.Retirement.MergeDistance != nil {
			t.Fatal("missing distance became observed", err)
		}
		distance := 1000000
		p.MergeDistance = &distance
		p.SourceRevision = a.Manifest.Repository.Revision
		p.TargetRevision = strings.Repeat("c", 40)
		r, err = Query(a, Request{Operation: "search", Query: "none", Retirement: p}, "fresh", nil)
		if err != nil || r.Envelope.Retirement.State != "within-declared-policy" {
			t.Fatal("legal upper bound refused", err)
		}
		distance = 0
		p.TargetRevision = p.SourceRevision
		r, err = Query(a, Request{Operation: "search", Query: "none", Retirement: p}, "fresh", nil)
		if err != nil || *r.Envelope.Retirement.MergeDistance != 0 {
			t.Fatal("same revision zero distance refused", err)
		}
	})
}

func TestCorpusNavigationSelectorEvidence(t *testing.T) {
	t.Run("DCP-V1-038 exact declared navigation selector and anchor", func(t *testing.T) {
		root, m := behaviorFixture(t, nil)
		a, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		r, err := Query(a, Request{Operation: "navigation", ID: "summary"}, "fresh", nil)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, v := range r.Results {
			if s, ok := v.(NavigationSelector); ok {
				if s.ID != "project:count:count-visible" || s.Flow != "summary" || s.Test != "project:count" || s.Locator != "#count" || s.Annotation.Path != "src/view.ts" || s.Annotation.Revision != m.Repository.Revision || s.Trust != "attributed-declaration" {
					t.Fatal("selector/anchor authority changed", s)
				}
				found = true
			}
		}
		if !found {
			t.Fatal("declared selector missing", r.Results)
		}
	})
}

func TestCorpusIndexedParityEdgeCases(t *testing.T) {
	t.Run("DCP-V1-038 DCP-V1-040 exact short names self edges and pages", func(t *testing.T) {
		a := &Artifact{Schema: SchemaV2, Subjects: []Subject{{ID: "s", Name: "X", Kind: "flow"}}, Relations: []Relation{{ID: "self", From: "s", To: "s", Type: "depends_on"}}, Capabilities: []Capability{{Name: "subjects", State: "present"}, {Name: "relations", State: "present"}}, Details: map[string]RecordDetails{}}
		x, e := BuildQueryIndex(context.Background(), a)
		if e != nil {
			t.Fatal(e)
		}
		indexed := *a
		indexed.RuntimeIndex = x
		for _, q := range []Request{{Operation: "search", Query: "X"}, {Operation: "search", Query: "x"}, {Operation: "trace", ID: "s"}} {
			native, e := Query(a, q, "fresh", nil)
			if e != nil {
				t.Fatal(e)
			}
			other, e := Query(&indexed, q, "fresh", nil)
			if e != nil {
				t.Fatal(e)
			}
			left, _ := Encode(native)
			right, _ := Encode(other)
			if string(left) != string(right) {
				t.Fatalf("indexed/native edge parity %s: %s vs %s", q.Operation, left, right)
			}
		}
		exhausted, e := Query(a, Request{Operation: "search", Query: "X", Offset: 100}, "fresh", nil)
		if e != nil || exhausted.Miss != "page-exhausted" || exhausted.Meaning != "page exhausted" {
			t.Fatal("documented page called undocumented", exhausted, e)
		}
	})
}

// cancellationProbe deterministically cancels after execution has entered its
// traversal. This detects missing in-loop checks without timing-sensitive sleeps.
type cancellationProbe struct {
	context.Context
	checks int
	after  int
}

func (c *cancellationProbe) Err() error {
	c.checks++
	if c.checks >= c.after {
		return context.Canceled
	}
	return nil
}
func TestCorpusTypedCancellationDuringExecution(t *testing.T) {
	t.Run("DCP-V1-042 bounded joins and cancellable active traversal", func(t *testing.T) {
		a := &Artifact{Schema: SchemaV2, Subjects: []Subject{}, Relations: []Relation{}, Capabilities: []Capability{{Name: "subjects", State: "present"}, {Name: "relations", State: "present"}, {Name: "gaps", State: "present"}}, Details: map[string]RecordDetails{}}
		for i := 0; i < 1000; i++ {
			id := strings.Repeat("x", i%5+1) + string(rune(0x1000+i))
			a.Subjects = append(a.Subjects, Subject{ID: id, Name: "checkout", Kind: "flow"})
			a.Relations = append(a.Relations, Relation{ID: "r" + id, From: a.Subjects[0].ID, To: id, Type: "depends_on"})
		}
		index, e := BuildQueryIndex(context.Background(), a)
		if e != nil {
			t.Fatal(e)
		}
		a.RuntimeIndex = index
		for _, q := range []Request{{Operation: "concept", Query: "checkout"}, {Operation: "intent", ID: a.Subjects[0].ID}, {Operation: "gaps"}, {Operation: "dependencies", ID: a.Subjects[0].ID}} {
			ctx := &cancellationProbe{Context: context.Background(), after: 20}
			if _, e := QueryContext(ctx, a, q, "fresh", nil); e != context.Canceled || ctx.checks < 20 {
				t.Fatal("active query ignored cancellation", q.Operation, e, ctx.checks)
			}
		}
	})
}
