package doccorpus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// tkAnchor pins an anchor to the committed bytes of path at rev.
func tkAnchor(t *testing.T, root string, m Manifest, rev, p string, start, end int) Anchor {
	t.Helper()
	cmd := exec.Command("git", "-C", root, "cat-file", "blob", rev+":"+p)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	data, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	excerpt, err := span(data, start, end)
	if err != nil {
		t.Fatal(err)
	}
	return Anchor{Repository: m.Repository.ID, Revision: rev, Path: p, Blob: git(t, root, "rev-parse", rev+":"+p), SHA256: Digest(data), Start: start, End: end, SpanSHA256: Digest(excerpt), Authority: "external-provider", Kind: "declared", Reason: "reviewed test example"}
}

// testKnowledgeFixture adds one test-knowledge provider to the adoption corpus.
// It returns the corpus manifest with and without that provider.
func testKnowledgeFixture(t *testing.T, edit func(root string, m Manifest, p *ProviderRecord), raw func([]byte) []byte) (string, Manifest, Manifest) {
	t.Helper()
	root, m := adoptionFixture(t, nil)
	base := m
	base.Providers = slices.Clone(m.Providers)
	base.Inputs = slices.Clone(m.Inputs)
	base.Scopes = slices.Clone(m.Scopes)
	rev := m.Repository.Revision
	runRevision := ""
	for _, in := range m.Inputs {
		if in.Path == "evidence/run.json" {
			runRevision = in.Revision
		}
	}
	receipt := tkAnchor(t, root, m, runRevision, "evidence/run.json", 1, 1)
	record := ProviderRecord{Schema: TestKnowledgeSchema, ID: "tk", Version: "1", Source: m.Repository, TestKnowledge: &TestKnowledgeSet{Revision: rev, Records: []TestKnowledgeRecord{{
		ID: "tk:count-unit", Kind: "unit", Framework: "go test", Title: "TestCount returns one",
		Test: tkAnchor(t, root, m, rev, "src/value_test.go", 3, 3), Sources: []Anchor{tkAnchor(t, root, m, rev, "src/value.go", 4, 4)},
		Entry: "value.Count", Actor: "caller", Setup: []string{}, Stubs: []TestKnowledgeStub{{Name: "clock", Boundary: "external"}},
		Behavior: "Count returns one", Asserted: []string{"Count() == 1"}, Flows: []string{"adapter:flow"}, Verdict: "bound",
		Docs: TestKnowledgeDocs{State: "consistent"}, Trust: "run+reviewed", Receipt: &receipt, Reviews: []string{"independent review 1"},
	}}}}
	if edit != nil {
		edit(root, m, &record)
	}
	data, err := Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	if raw != nil {
		data = raw(data)
	}
	if err := os.WriteFile(filepath.Join(root, "evidence/tk.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "evidence/tk.json")
	git(t, root, "commit", "-qm", "test knowledge")
	tkRevision := git(t, root, "rev-parse", "HEAD")
	m.Providers = append(m.Providers, Provider{ID: "tk", Kind: "records", Version: "1", Revision: tkRevision, Record: "evidence/tk.json"})
	m.Scopes = append(m.Scopes, Scope{"evidence/tk.json", tkRevision})
	m.Inputs = append(m.Inputs, Input{Path: "evidence/tk.json", Revision: tkRevision, Blob: git(t, root, "rev-parse", tkRevision+":evidence/tk.json"), SHA256: Digest(data), Provider: "tk", Purpose: "provider"})
	return root, m, base
}
func tkRecord(edit func(r *TestKnowledgeRecord)) func(string, Manifest, *ProviderRecord) {
	return func(_ string, _ Manifest, p *ProviderRecord) { edit(&p.TestKnowledge.Records[0]) }
}
func tkBuild(t *testing.T, edit func(string, Manifest, *ProviderRecord), raw func([]byte) []byte) (*Artifact, error) {
	t.Helper()
	root, m, _ := testKnowledgeFixture(t, edit, raw)
	return Build(context.Background(), root, m)
}
func tkRefused(t *testing.T, err error, want string) {
	t.Helper()
	var refused *Error
	if !errors.As(err, &refused) || refused.Code != "corpus-refused" || !strings.Contains(refused.Message, want) {
		t.Fatalf("want corpus-refused %q, got %v", want, err)
	}
}

func TestDCPV1086TestKnowledgeSchemaIsClosed(t *testing.T) {
	t.Parallel()
	a, err := tkBuild(t, nil, nil)
	if err != nil || len(a.TestKnowledge) != 1 || a.TestKnowledge[0].Status != "joined" {
		t.Fatalf("valid record not joined: %+v %v", a, err)
	}
	for name, raw := range map[string]func([]byte) []byte{
		"unknown record member": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"trust":"run+reviewed"`), []byte(`"trust":"run+reviewed","verified":true`), 1)
		},
		"unknown set member": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"test_knowledge":{`), []byte(`"test_knowledge":{"coverage":1,`), 1)
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := tkBuild(t, nil, raw); err == nil || !strings.Contains(err.Error(), "corpus-refused") {
				t.Fatalf("unknown member admitted: %v", err)
			}
		})
	}
	for name, c := range map[string]struct {
		edit func(*TestKnowledgeRecord)
		want string
	}{
		"kind":          {func(r *TestKnowledgeRecord) { r.Kind = "snapshot" }, "unknown test-knowledge kind"},
		"verdict":       {func(r *TestKnowledgeRecord) { r.Verdict = "verified" }, "unknown test-knowledge kind"},
		"trust":         {func(r *TestKnowledgeRecord) { r.Trust = "verified" }, "unknown test-knowledge kind"},
		"docs state":    {func(r *TestKnowledgeRecord) { r.Docs.State = "unknown" }, "unknown test-knowledge kind"},
		"stub boundary": {func(r *TestKnowledgeRecord) { r.Stubs[0].Boundary = "network" }, "invalid test-knowledge stub"},
		"unqualified":   {func(r *TestKnowledgeRecord) { r.ID = "count-unit" }, "provider-qualified"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := tkBuild(t, tkRecord(c.edit), nil)
			tkRefused(t, err, c.want)
		})
	}
	t.Run("duplicate id", func(t *testing.T) {
		_, err := tkBuild(t, func(_ string, _ Manifest, p *ProviderRecord) {
			p.TestKnowledge.Records = append(p.TestKnowledge.Records, p.TestKnowledge.Records[0])
		}, nil)
		tkRefused(t, err, "unique")
	})
	t.Run("other members under the schema", func(t *testing.T) {
		_, err := tkBuild(t, func(_ string, m Manifest, p *ProviderRecord) {
			p.Capabilities = []CapabilityDeclaration{{"subjects", "absent", "none"}}
		}, nil)
		tkRefused(t, err, "only its test_knowledge member")
	})
	t.Run("member under another schema", func(t *testing.T) {
		_, err := tkBuild(t, func(_ string, _ Manifest, p *ProviderRecord) { p.Schema = ProviderSchema }, nil)
		tkRefused(t, err, "explicit provider schema")
	})
	t.Run("schema without member", func(t *testing.T) {
		_, err := tkBuild(t, func(_ string, _ Manifest, p *ProviderRecord) { p.TestKnowledge = nil }, nil)
		tkRefused(t, err, "explicit provider schema")
	})
	t.Run("manifest /1", func(t *testing.T) {
		root, m, _ := testKnowledgeFixture(t, nil, nil)
		var record ProviderRecord
		if err := decode([]byte(git(t, root, "show", "HEAD:evidence/tk.json")), &record); err != nil {
			t.Fatal(err)
		}
		m.Schema = ManifestSchema
		c := &compiler{manifest: m}
		tkRefused(t, c.admitTestKnowledge(Provider{ID: "tk"}, record), "requires manifest /2")
	})
}

func TestDCPV1087TestKnowledgeFieldRules(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		edit func(*TestKnowledgeRecord)
		want string
	}{
		"partial without gap":      {func(r *TestKnowledgeRecord) { r.Verdict = "partial" }, "gap is required"},
		"gap without partial":      {func(r *TestKnowledgeRecord) { r.Gap = "does not cover zero" }, "gap is required"},
		"contradicts without":      {func(r *TestKnowledgeRecord) { r.Docs.State = "contradicts" }, "docs claim"},
		"claim without contradict": {func(r *TestKnowledgeRecord) { r.Docs.Claim = "adapter:claim" }, "docs claim"},
		"run without receipt":      {func(r *TestKnowledgeRecord) { r.Receipt = nil }, "receipt is required"},
		"read-only with receipt": {func(r *TestKnowledgeRecord) {
			r.Trust = "read-only"
			r.Reviews = []string{}
		}, "receipt is required"},
		"reviewed without reviews":  {func(r *TestKnowledgeRecord) { r.Reviews = []string{} }, "reviews are required"},
		"unreviewed with reviews":   {func(r *TestKnowledgeRecord) { r.Trust = "run-unreviewed" }, "reviews are required"},
		"secret-shaped behavior":    {func(r *TestKnowledgeRecord) { r.Behavior = "uses sk0123456789abcdef0123456789abcdef" }, "secret-shaped"},
		"secret-shaped asserted":    {func(r *TestKnowledgeRecord) { r.Asserted = []string{"sk0123456789abcdef0123456789abcdef"} }, "secret-shaped"},
		"empty framework":           {func(r *TestKnowledgeRecord) { r.Framework = "" }, "secret-shaped"},
		"no sources":                {func(r *TestKnowledgeRecord) { r.Sources = nil }, "list"},
		"duplicate flow":            {func(r *TestKnowledgeRecord) { r.Flows = []string{"adapter:flow", "adapter:flow"} }, "duplicate test-knowledge flow"},
		"oversized framework label": {func(r *TestKnowledgeRecord) { r.Framework = strings.Repeat("x", 129) }, "secret-shaped"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := tkBuild(t, tkRecord(c.edit), nil)
			tkRefused(t, err, c.want)
		})
	}
	t.Run("partial with gap and contradicts with claim admitted", func(t *testing.T) {
		a, err := tkBuild(t, tkRecord(func(r *TestKnowledgeRecord) {
			r.Verdict, r.Gap = "partial", "zero value is not asserted"
			r.Docs = TestKnowledgeDocs{State: "contradicts", Claim: "adapter:claim"}
		}), nil)
		if err != nil || a.TestKnowledge[0].Record.Gap == "" || !a.TestKnowledge[0].CountsTowardCoverage {
			t.Fatalf("partial record lost: %v", err)
		}
	})
}

func TestDCPV1088TestKnowledgeAnchorsArePinned(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		edit func(*TestKnowledgeRecord)
		want string
	}{
		"unpinned source":  {func(r *TestKnowledgeRecord) { r.Sources[0].Revision = r.Receipt.Revision }, "not pinned to the set revision"},
		"symbol":           {func(r *TestKnowledgeRecord) { r.Test.Symbol = "TestCount" }, "anchor identity"},
		"repository":       {func(r *TestKnowledgeRecord) { r.Test.Repository = r.Test.Blob }, "anchor identity"},
		"authority":        {func(r *TestKnowledgeRecord) { r.Sources[0].Authority = "syntax" }, "anchor identity"},
		"evidence kind":    {func(r *TestKnowledgeRecord) { r.Sources[0].Kind = "verified" }, "anchor identity"},
		"blob":             {func(r *TestKnowledgeRecord) { r.Test.Blob = "not-a-blob" }, "anchor pin"},
		"digest":           {func(r *TestKnowledgeRecord) { r.Test.SHA256 = "" }, "anchor pin"},
		"span":             {func(r *TestKnowledgeRecord) { r.Test.End = 0 }, "anchor pin"},
		"receipt path":     {func(r *TestKnowledgeRecord) { r.Receipt.Path = "../run.json" }, "anchor pin"},
		"receipt reason":   {func(r *TestKnowledgeRecord) { r.Receipt.Reason = "" }, "anchor pin"},
		"forged same blob": {func(r *TestKnowledgeRecord) { r.Sources[0].SpanSHA256 = r.Test.SpanSHA256 }, "binding mismatch"},
		"forged blob at corpus revision": {func(r *TestKnowledgeRecord) {
			r.Sources[0].Blob = r.Test.Blob
		}, "binding mismatch"},
		"forged receipt": {func(r *TestKnowledgeRecord) { r.Receipt.SHA256 = r.Test.SHA256 }, "binding mismatch"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := tkBuild(t, tkRecord(c.edit), nil)
			tkRefused(t, err, c.want)
		})
	}
}

func TestDCPV1089TestKnowledgeJoin(t *testing.T) {
	t.Parallel()
	t.Run("joined to claims, flows and sources without rewriting records", func(t *testing.T) {
		root, m, base := testKnowledgeFixture(t, tkRecord(func(r *TestKnowledgeRecord) {
			r.Flows = []string{"adapter:flow", "adapter:missing"}
		}), nil)
		a, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		without, err := Build(context.Background(), root, base)
		if err != nil {
			t.Fatal(err)
		}
		j := a.TestKnowledge[0]
		if j.Status != "joined" || !slices.Contains(j.Claims, "adapter:claim") || !slices.Equal(j.Flows, []string{"adapter:flow"}) || !slices.Equal(j.UnresolvedFlows, []string{"adapter:missing"}) || !slices.Equal(j.Paths, []string{"src/value.go", "src/value_test.go"}) {
			t.Fatalf("join lost targets: %+v", j)
		}
		for name, pair := range map[string][2]any{"subjects": {a.Subjects, without.Subjects}, "claims": {a.Claims, without.Claims}, "relations": {a.Relations, without.Relations}, "journeys": {a.Journeys, without.Journeys}, "observations": {a.Observations, without.Observations}, "details": {a.Details, without.Details}} {
			x, _ := Encode(pair[0])
			y, _ := Encode(pair[1])
			if !bytes.Equal(x, y) {
				t.Fatalf("%s rewritten by the test-knowledge join", name)
			}
		}
	})
	t.Run("stale blob is reported, not joined", func(t *testing.T) {
		a, err := tkBuild(t, func(root string, m Manifest, p *ProviderRecord) {
			if err := os.WriteFile(filepath.Join(root, "src/value.go"), []byte("package value\n\n// Count returns two.\nfunc Count() int { return 2 }\n"), 0600); err != nil {
				t.Fatal(err)
			}
			git(t, root, "commit", "-qam", "later source")
			later := git(t, root, "rev-parse", "HEAD")
			r := &p.TestKnowledge.Records[0]
			p.TestKnowledge.Revision = later
			r.Test = tkAnchor(t, root, m, later, "src/value_test.go", 3, 3)
			r.Sources = []Anchor{tkAnchor(t, root, m, later, "src/value.go", 4, 4)}
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		j := a.TestKnowledge[0]
		if j.Status != "stale" || len(j.Claims) != 0 || len(j.Flows) != 0 || j.CountsTowardCoverage || j.Selection != "excluded" || !slices.Contains(j.Reasons, "stale: set revision is not the corpus revision") {
			t.Fatalf("stale record joined: %+v", j)
		}
	})
	t.Run("claimed revision other than the corpus revision never joins", func(t *testing.T) {
		// Same blobs as the corpus revision, so only the claimed revision differs.
		for name, other := range map[string]func(root string, m Manifest, p *ProviderRecord) string{
			"nonexistent": func(string, Manifest, *ProviderRecord) string { return strings.Repeat("1", 40) },
			"other commit": func(_ string, _ Manifest, p *ProviderRecord) string {
				return p.TestKnowledge.Records[0].Receipt.Revision
			},
		} {
			t.Run(name, func(t *testing.T) {
				root, m, _ := testKnowledgeFixture(t, func(root string, m Manifest, p *ProviderRecord) {
					rev := other(root, m, p)
					r := &p.TestKnowledge.Records[0]
					p.TestKnowledge.Revision, r.Test.Revision, r.Sources[0].Revision = rev, rev, rev
				}, nil)
				a, err := Build(context.Background(), root, m)
				if err != nil {
					t.Fatal(err)
				}
				j := a.TestKnowledge[0]
				if j.Status != "stale" || len(j.Paths) != 0 || j.CountsTowardCoverage || j.Selection != "excluded" {
					t.Fatalf("record at a claimed revision joined: %+v", j)
				}
				r, err := Query(a, Request{Operation: "claims", Path: "src/value.go"}, "fresh", nil)
				if err != nil || slices.ContainsFunc(r.Results, func(v any) bool { _, ok := v.(TestKnowledgeJoin); return ok }) {
					t.Fatalf("stale record cited: %+v %v", r.Results, err)
				}
			})
		}
	})
	t.Run("undeclared path and receipt are unanchored", func(t *testing.T) {
		for name, edit := range map[string]func(*TestKnowledgeRecord){
			"source":  func(r *TestKnowledgeRecord) { r.Sources[0].Path = "src/missing.go" },
			"receipt": func(r *TestKnowledgeRecord) { r.Receipt.Revision = r.Test.Revision },
		} {
			t.Run(name, func(t *testing.T) {
				a, err := tkBuild(t, tkRecord(edit), nil)
				if err != nil {
					t.Fatal(err)
				}
				if j := a.TestKnowledge[0]; j.Status != "unanchored" || len(j.Claims) != 0 || j.Selection != "excluded" || j.CountsTowardCoverage {
					t.Fatalf("unanchored record joined: %+v", j)
				}
			})
		}
	})
}

func TestDCPV1090TestKnowledgeTrustRules(t *testing.T) {
	t.Parallel()
	cases := []struct {
		trust, verdict, selection string
		counts                    bool
	}{
		{"run+reviewed", "bound", "candidate", true},
		{"run+reviewed", "partial", "candidate", true},
		{"run+reviewed", "unrelated", "excluded", false},
		{"run+reviewed", "defect", "blocks", false},
		{"run-unreviewed", "bound", "excluded", false},
		{"read-only", "bound", "excluded", false},
		{"legacy-reference", "bound", "excluded", false},
		{"read-only", "defect", "blocks", false},
		{"legacy-reference", "defect", "blocks", false},
	}
	for _, c := range cases {
		t.Run(c.trust+" "+c.verdict, func(t *testing.T) {
			root, m, base := testKnowledgeFixture(t, tkRecord(func(r *TestKnowledgeRecord) {
				r.Trust, r.Verdict = c.trust, c.verdict
				if c.verdict == "partial" {
					r.Gap = "boundary value unasserted"
				}
				if c.trust != "run+reviewed" {
					r.Reviews = []string{}
				}
				if c.trust == "read-only" || c.trust == "legacy-reference" {
					r.Receipt = nil
				}
			}), nil)
			a, err := Build(context.Background(), root, m)
			if err != nil {
				t.Fatal(err)
			}
			j := a.TestKnowledge[0]
			if j.TrustLevel != c.trust || j.CountsTowardCoverage != c.counts || j.Selection != c.selection || !slices.Contains(j.Limitations, testKnowledgeLimitation) {
				t.Fatalf("trust rule broken: %+v", j)
			}
			without, err := Build(context.Background(), root, base)
			if err != nil {
				t.Fatal(err)
			}
			x, _ := Encode(coverageMetrics(without)[:3])
			y, _ := Encode(coverageMetrics(a)[:3])
			if !bytes.Equal(x, y) {
				t.Fatal("test knowledge changed verified-journey or declaration metrics")
			}
			for i := range a.Journeys {
				if a.Journeys[i].Status != without.Journeys[i].Status {
					t.Fatal("test knowledge marked a journey verified")
				}
			}
		})
	}
}

func TestDCPV1091TestKnowledgeQueryExposure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	open := func(t *testing.T, edit func(string, Manifest, *ProviderRecord)) (string, []byte) {
		t.Helper()
		root, m, _ := testKnowledgeFixture(t, edit, nil)
		a, err := Build(ctx, root, m)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := Encode(a)
		if err != nil {
			t.Fatal(err)
		}
		return root, raw
	}
	// ReadQuery is the call the corpus MCP bridge and CLI make for every tool.
	read := func(t *testing.T, root string, raw []byte, q Request) Receipt {
		t.Helper()
		r, err := ReadQuery(ctx, root, raw, q)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	joins := func(r Receipt) []TestKnowledgeJoin {
		out := []TestKnowledgeJoin{}
		for _, v := range r.Results {
			if j, ok := v.(TestKnowledgeJoin); ok {
				out = append(out, j)
			}
		}
		return out
	}
	root, raw := open(t, nil)
	for _, q := range []Request{{Operation: "get", ID: "tk:count-unit"}, {Operation: "flow", ID: "adapter:flow"}, {Operation: "claims", Path: "src/value.go"}, {Operation: "locate", Path: "src/value_test.go"}, {Operation: "recommend-tests", Path: "src/value.go"}, {Operation: "inventory", Limit: 256}} {
		t.Run(q.Operation, func(t *testing.T) {
			t.Parallel()
			r := read(t, root, raw, q)
			got := joins(r)
			if len(got) != 1 || got[0].TrustLevel != "run+reviewed" || got[0].Selection != "candidate" {
				t.Fatalf("%s did not return the labelled join: %+v", q.Operation, r.Results)
			}
			if !slices.ContainsFunc(r.Citations, func(a Anchor) bool { return a.Path == "src/value_test.go" }) {
				t.Fatal("test anchor citation missing")
			}
			if q.Operation == "recommend-tests" && (r.Selection.State != "full-relevant-suite-required" || r.Selection.NarrowingAllowed || len(r.Selection.BlockedBy) != 0) {
				t.Fatalf("selection narrowed: %+v", r.Selection)
			}
		})
	}
	t.Run("coverage", func(t *testing.T) {
		t.Parallel()
		r := read(t, root, raw, Request{Operation: "coverage", Limit: 256})
		rows := map[string]map[string]any{}
		for _, v := range r.Results {
			if row, ok := v.(map[string]any); ok {
				key, _ := row["metric"].(string)
				if level, ok := row["trust_level"].(string); ok {
					key += ":" + level
				}
				rows[key] = row
			}
		}
		flows := rows["flows_with_reviewed_test_knowledge"]
		if flows == nil || flows["value"] != 1 || flows["denominator"] != 1 {
			t.Fatalf("reviewed flow coverage missing: %+v", rows)
		}
		if rows["test_knowledge_trust_level:run+reviewed"]["counts_toward_coverage"] != 1 || rows["test_knowledge_trust_level:read-only"]["joined"] != 0 {
			t.Fatalf("trust rows wrong: %+v", rows)
		}
	})
	t.Run("gaps and no-tests", func(t *testing.T) {
		t.Parallel()
		for _, counts := range []bool{true, false} {
			a := &Artifact{RuntimeIndex: &QueryIndex{}, Subjects: []Subject{{ID: "f", Kind: "flow"}}, TestKnowledge: []TestKnowledgeJoin{{ID: "tk:a", Status: "joined", Flows: []string{"f"}, CountsTowardCoverage: counts}}}
			gaps, err := typedGaps(context.Background(), a, "fresh")
			if err != nil {
				t.Fatal(err)
			}
			if slices.ContainsFunc(gaps, func(g Gap) bool { return g.Kind == "no-tests" }) == counts {
				t.Fatalf("no-tests gap does not follow the counted rule (counts=%v): %+v", counts, gaps)
			}
		}
		unreviewedRoot, unreviewed := open(t, tkRecord(func(r *TestKnowledgeRecord) {
			r.Trust, r.Reviews = "run-unreviewed", []string{}
			r.Flows = []string{"adapter:flow", "adapter:missing"}
		}))
		gaps := read(t, unreviewedRoot, unreviewed, Request{Operation: "gaps", Limit: 256})
		kinds := map[string]bool{}
		for _, v := range gaps.Results {
			g := v.(Gap)
			kinds[g.Subject+" "+g.Kind] = true
		}
		if !kinds["tk:count-unit unresolved-test-knowledge-flow"] {
			t.Fatalf("unresolved flow not reported: %v", kinds)
		}
		if got := joins(read(t, unreviewedRoot, unreviewed, Request{Operation: "recommend-tests", Path: "src/value.go"})); len(got) != 0 {
			t.Fatal("run-unreviewed record selected")
		}
		if got := joins(read(t, unreviewedRoot, unreviewed, Request{Operation: "claims", Path: "src/value.go"})); len(got) != 1 || got[0].TrustLevel != "run-unreviewed" || got[0].CountsTowardCoverage {
			t.Fatal("claim lookup lost the labelled unreviewed record")
		}
		staleRoot, stale := open(t, tkRecord(func(r *TestKnowledgeRecord) { r.Sources[0].Path = "src/missing.go" }))
		gaps = read(t, staleRoot, stale, Request{Operation: "gaps", ID: "tk:count-unit"})
		if len(gaps.Results) != 1 || gaps.Results[0].(Gap).Kind != "unanchored-test-knowledge" {
			t.Fatalf("unanchored record not reported: %+v", gaps.Results)
		}
		if got := joins(read(t, staleRoot, stale, Request{Operation: "flow", ID: "adapter:flow"})); len(got) != 0 {
			t.Fatal("unjoined record exposed as flow proof")
		}
		if got := joins(read(t, staleRoot, stale, Request{Operation: "get", ID: "tk:count-unit"})); len(got) != 1 || got[0].Status != "unanchored" {
			t.Fatal("unjoined record not retrievable by id")
		}
	})
	t.Run("defect blocks selection", func(t *testing.T) {
		t.Parallel()
		defectRoot, defect := open(t, func(_ string, _ Manifest, p *ProviderRecord) {
			second := p.TestKnowledge.Records[0]
			second.ID, second.Verdict, second.Trust, second.Receipt, second.Reviews = "tk:count-defect", "defect", "read-only", nil, []string{}
			p.TestKnowledge.Records = append(p.TestKnowledge.Records, second)
		})
		r := read(t, defectRoot, defect, Request{Operation: "recommend-tests", Path: "src/value.go"})
		if len(joins(r)) != 0 || !slices.Equal(r.Selection.BlockedBy, []string{"tk:count-defect"}) || !slices.Contains(r.Selection.MissingEvidence, "test-knowledge defect blocks selection") || r.Selection.State != "full-relevant-suite-required" || r.Selection.NarrowingAllowed {
			t.Fatalf("defect did not block selection: %+v %+v", r.Results, r.Selection)
		}
	})
	t.Run("worst-case rows stay within the receipt bound", func(t *testing.T) {
		t.Parallel()
		// 4096 joined defects with 1 KiB IDs would exceed the 4 MiB receipt bound if any one
		// coverage row or the blocked_by list named every ID.
		wideRoot, wide := open(t, func(_ string, _ Manifest, p *ProviderRecord) {
			r := p.TestKnowledge.Records[0]
			r.Verdict, r.Trust, r.Receipt, r.Reviews = "defect", "read-only", nil, []string{}
			p.TestKnowledge.Records = nil
			for i := range MaxTestKnowledge {
				r.ID = fmt.Sprintf("tk:%04d%s", i, strings.Repeat("x", 1017))
				p.TestKnowledge.Records = append(p.TestKnowledge.Records, r)
			}
		})
		recommend := read(t, wideRoot, wide, Request{Operation: "recommend-tests", Path: "src/value.go"})
		r := recommend
		s := r.Selection
		if len(s.BlockedBy) != MaxBlockedBy || s.BlockedByCount != MaxTestKnowledge || !s.BlockedByTruncated || !strings.HasPrefix(s.BlockedBy[0], "tk:0000") || len(joins(r)) != 0 {
			t.Fatalf("blocked_by not bounded explicitly: %d %d %v", len(s.BlockedBy), s.BlockedByCount, s.BlockedByTruncated)
		}
		// Pagination halves an oversized page, so the row must be present, not merely absent.
		r = read(t, wideRoot, wide, Request{Operation: "coverage", Limit: MaxResults})
		found := false
		for _, v := range r.Results {
			if row, ok := v.(map[string]any); ok && row["trust_level"] == "read-only" {
				found = row["joined"] == MaxTestKnowledge && row["records"] == nil
			}
		}
		if !found || r.Omitted != 0 {
			t.Fatalf("read-only trust row withheld or not bounded to counts: omitted %d", r.Omitted)
		}
		// The bound is checked on the two receipts read above; a read is deterministic, so
		// reading them again would only repeat the work (V1-2740).
		for operation, receipt := range map[string]Receipt{"recommend-tests": recommend, "coverage": r} {
			if out, err := Encode(receipt); err != nil || len(out) > MaxBytes {
				t.Fatalf("%s receipt exceeds the bound: %d %v", operation, len(out), err)
			}
		}
	})
}

// TestDCPV1091TestKnowledgeOnlyCorpus pins that a corpus whose only provider is a
// test-knowledge record still answers the operations its joins serve.
func TestDCPV1091TestKnowledgeOnlyCorpus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root, m, _ := testKnowledgeFixture(t, func(_ string, _ Manifest, p *ProviderRecord) {
		p.TestKnowledge.Records[0].Flows = []string{}
	}, nil)
	only := m
	only.Providers, only.Inputs, only.Scopes = nil, nil, nil
	for _, p := range m.Providers {
		if p.ID == "tk" {
			only.Providers = append(only.Providers, p)
		}
	}
	for _, in := range m.Inputs {
		if in.Purpose != "provider" || in.Provider == "tk" {
			in.Provider = "tk"
			only.Inputs = append(only.Inputs, in)
		}
	}
	for _, s := range m.Scopes {
		if s.Path != "evidence/provider.json" {
			only.Scopes = append(only.Scopes, s)
		}
	}
	a, err := Build(ctx, root, only)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Subjects)+len(a.Claims)+len(a.Relations) != 0 || len(a.TestKnowledge) != 1 || a.TestKnowledge[0].Status != "joined" {
		t.Fatalf("fixture is not test-knowledge only: %d subjects %+v", len(a.Subjects), a.TestKnowledge)
	}
	raw, err := Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []Request{{Operation: "get", ID: "tk:count-unit"}, {Operation: "locate", Path: "src/value_test.go"}, {Operation: "claims", Path: "src/value.go"}, {Operation: "recommend-tests", Path: "src/value.go"}} {
		t.Run(q.Operation, func(t *testing.T) {
			r, err := ReadQuery(ctx, root, raw, q)
			if err != nil || r.State != "ready" || r.Miss != "" || len(r.Results) != 1 || !slices.ContainsFunc(r.Limitations, func(l string) bool { return strings.HasSuffix(l, "answered from test-knowledge records only") }) {
				t.Fatalf("%s refused on a test-knowledge-only corpus: %+v %v", q.Operation, r, err)
			}
			if j, ok := r.Results[0].(TestKnowledgeJoin); !ok || j.TrustLevel != "run+reviewed" {
				t.Fatalf("%s lost the labelled join: %+v", q.Operation, r.Results)
			}
		})
	}
	if r, err := ReadQuery(ctx, root, raw, Request{Operation: "search", Query: "Count"}); err != nil || r.Miss != "capability-absent" {
		t.Fatalf("an operation test knowledge does not serve was admitted: %+v %v", r, err)
	}
}

func TestDCPV1092TestKnowledgeDeterministicAndBounded(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root, m, _ := testKnowledgeFixture(t, nil, nil)
	a, err := Build(ctx, root, m)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Build(ctx, root, m)
	if err != nil || a.SHA256 != b.SHA256 {
		t.Fatal("test-knowledge join is not deterministic", err)
	}
	raw, err := Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	if opened, err := Open(ctx, root, raw); err != nil || opened.SHA256 != a.SHA256 {
		t.Fatal("artifact with test knowledge does not re-derive", err)
	}
	t.Run("per-provider bound", func(t *testing.T) {
		_, err := tkBuild(t, func(_ string, _ Manifest, p *ProviderRecord) {
			r := p.TestKnowledge.Records[0]
			p.TestKnowledge.Records = nil
			for i := 0; i <= MaxTestKnowledge; i++ {
				r.ID = "tk:" + strconv.Itoa(i)
				p.TestKnowledge.Records = append(p.TestKnowledge.Records, r)
			}
		}, nil)
		tkRefused(t, err, "record bound")
	})
	t.Run("aggregate bound", func(t *testing.T) {
		c := &compiler{manifest: m, artifact: &Artifact{}, sources: nil}
		set := &TestKnowledgeSet{Revision: m.Repository.Revision, Records: make([]TestKnowledgeRecord, MaxTestKnowledge/2+1)}
		for i := range set.Records {
			set.Records[i] = a.TestKnowledge[0].Record
			set.Records[i].ID = "tk:" + strconv.Itoa(i)
		}
		other := &TestKnowledgeSet{Revision: set.Revision, Records: slices.Clone(set.Records)}
		for i := range other.Records {
			other.Records[i].ID = "tk2:" + strconv.Itoa(i)
		}
		tkRefused(t, c.joinTestKnowledge([]providerKnowledge{{"tk", set}, {"tk2", other}}), "aggregate test-knowledge bound")
	})
	t.Run("readable record bound", func(t *testing.T) {
		big := &Artifact{TestKnowledge: []TestKnowledgeJoin{{ID: "tk:big", Record: TestKnowledgeRecord{Behavior: strings.Repeat("x", 2<<20)}}}}
		tkRefused(t, validateReadableRecords(big), "read bound")
	})
}
