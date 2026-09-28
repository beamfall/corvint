package doccorpus

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func adoptionFixture(t *testing.T, edit func(*ProviderRecord)) (string, Manifest) {
	t.Helper()
	root, m := providerFixture(t, func(p *ProviderRecord) {
		p.Schema = AdoptionProviderSchema
		ev := p.Subjects[0].Evidence
		p.Details = map[string]RecordDetails{}
		add := func(id, kind string, d RecordDetails) {
			p.Subjects = append(p.Subjects, Subject{ID: id, Kind: kind, Name: id, Provider: p.ID, Evidence: ev})
			p.Details[id] = d
		}
		sections := []FlowSection{}
		for _, name := range flowSections {
			sections = append(sections, FlowSection{Name: name, Paragraphs: []string{}})
		}
		sections[0].Paragraphs = []string{"adapter:paragraph"}
		add("adapter:flow", "flow", RecordDetails{Flow: &FlowDetails{Sections: sections, Actors: []string{"operator"}, Entrypoints: []string{"adapter:count"}, Variations: []string{}}})
		add("adapter:paragraph", "paragraph", RecordDetails{Paragraph: &ParagraphDetails{Flow: "adapter:flow", Section: "readme", StableID: "adapter:stable"}})
		add("adapter:coverage", "coverage_definition", RecordDetails{Coverage: &CoverageDetails{Definition: "supplied claims with documented outcome", Denominator: []string{"adapter:claim"}, Numerator: []string{"adapter:claim"}, Rule: "explicit-membership"}})
		add("adapter:ticket", "ticket", RecordDetails{Ticket: &TicketDetails{ExternalID: "APP-100", Status: "closed", History: []TicketHistory{{At: "2026-09-28T00:00:00Z", Revision: p.Source.Revision, Summary: "Historical disposition"}}}})
		add("adapter:intent", "intent_comparison", RecordDetails{IntentComparison: &IntentComparisonDetails{ReportedIntent: []string{"adapter:paragraph"}, ObservedClaims: []string{"adapter:claim"}, Missing: []string{}, Conflicts: []string{}}})
		p.Details["adapter:claim"] = RecordDetails{ClaimKind: "behavior"}
		p.Relations = append(p.Relations, Relation{ID: "adapter:test-join", From: "adapter:flow", To: "native:symbol:src/value_test.go:3:TestCount", Type: "asserts", Provider: p.ID, Evidence: ev})
		p.Details["adapter:test-join"] = RecordDetails{TestLink: &TestLinkDetails{Role: "asserting", JoinConfidence: "exact", File: "src/value_test.go", Title: "TestCount", Project: "go"}}
		p.RestrictedFindings = []RestrictedFinding{{ID: "restricted-canary-identity", Severity: "high", LocalDetailPath: "private/restricted-canary-report.txt", Detail: "restricted-canary-exploit-detail"}}
		if edit != nil {
			edit(p)
		}
	})
	m.Schema = ManifestSchemaV2
	return root, m
}
func TestCorpusAdoptionTypedRecords(t *testing.T) {
	t.Run("DCP-V1-034 typed joins and DCP-V1-035 restricted projections", func(t *testing.T) {
		root, m := adoptionFixture(t, nil)
		a, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := Encode(a)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "restricted-canary") {
			t.Fatal("restricted input escaped")
		}
		if len(a.RestrictedSummaries) != 1 || a.RestrictedSummaries[0].BySeverity["high"] != 1 {
			t.Fatal("missing severity summary")
		}
		if a.Details["adapter:claim"].ClaimKind != "behavior" || len(a.Details["adapter:flow"].Flow.Sections) != 8 {
			t.Fatal("typed details dropped")
		}
		if _, err = Open(context.Background(), root, raw); err != nil {
			t.Fatal(err)
		}
		for _, op := range []string{"info", "inventory", "get", "trace", "search", "coverage", "gaps", "related"} {
			r, err := Query(a, Request{Operation: op, ID: "adapter:test-join", Query: "Count", Limit: 2}, "fresh", nil)
			if err != nil {
				t.Fatal(err)
			}
			out, _ := Encode(r)
			if strings.Contains(string(out), "restricted-canary") {
				t.Fatal("restricted projection")
			}
		}
		r, err := Query(a, Request{Operation: "get", ID: "adapter:test-join"}, "fresh", nil)
		if err != nil || len(r.Results) != 1 || r.Details["adapter:test-join"].TestLink == nil {
			t.Fatal("typed edge not retrievable", err)
		}
		seen := map[string]bool{}
		offset := 0
		for {
			r, err := Query(a, Request{Operation: "inventory", Limit: 2, Offset: offset}, "fresh", nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, x := range r.Results {
				id := recordID(x)
				if id == "" || seen[id] {
					t.Fatal("page duplicate or lost identity")
				}
				seen[id] = true
			}
			if r.NextOffset == nil {
				break
			}
			offset = *r.NextOffset
		}
		if len(seen) != len(a.Subjects)+len(a.Claims)+len(a.Relations)+len(a.Journeys)+len(a.Observations) {
			t.Fatal("pagination lost records")
		}
		before := a.SHA256
		if err := os.WriteFile(filepath.Join(root, "src/value.go"), []byte("changed\n"), 0600); err != nil {
			t.Fatal(err)
		}
		state, _, err := Freshness(context.Background(), root, a)
		if err != nil || state != "stale" || a.SHA256 != before {
			t.Fatal("freshness mutated artifact or ignored dirty source", err)
		}
	})
}
func TestCorpusAdoptionRefusals(t *testing.T) {
	t.Run("DCP-V1-034 closed typed vocabulary and DCP-V1-035 no restricted references", func(t *testing.T) {
		cases := map[string]func(*ProviderRecord){
			"missing section": func(p *ProviderRecord) {
				p.Details["adapter:flow"].Flow.Sections = p.Details["adapter:flow"].Flow.Sections[:7]
			},
			"missing paragraph membership": func(p *ProviderRecord) { p.Details["adapter:flow"].Flow.Sections[0].Paragraphs = nil },
			"retirement cycle":             func(p *ProviderRecord) { p.Details["adapter:paragraph"].Paragraph.RetiredTo = "adapter:paragraph" },
			"invalid join confidence":      func(p *ProviderRecord) { p.Details["adapter:test-join"].TestLink.JoinConfidence = "proven" },
			"wrong test file":              func(p *ProviderRecord) { p.Details["adapter:test-join"].TestLink.File = "wrong.go" },
			"missing denominator":          func(p *ProviderRecord) { p.Details["adapter:coverage"].Coverage.Denominator = []string{"absent"} },
			"invalid history":              func(p *ProviderRecord) { p.Details["adapter:ticket"].Ticket.History[0].Revision = "HEAD" },
			"missing intent": func(p *ProviderRecord) {
				p.Details["adapter:intent"].IntentComparison.ObservedClaims = []string{"absent"}
			},
			"restricted body":     func(p *ProviderRecord) { p.Claims[0].Text = "prefix restricted-canary-exploit-detail suffix" },
			"restricted identity": func(p *ProviderRecord) { p.Subjects[0].Name = "restricted-canary-identity" },
			"restricted path": func(p *ProviderRecord) {
				p.Details["adapter:ticket"].Ticket.History[0].Summary = "private/restricted-canary-report.txt"
			},
		}
		for name, edit := range cases {
			t.Run(name, func(t *testing.T) {
				root, m := adoptionFixture(t, edit)
				_, err := Build(context.Background(), root, m)
				if err == nil {
					t.Fatal("invalid import accepted")
				}
				if strings.Contains(err.Error(), "restricted-canary") {
					t.Fatal("diagnostic leaks restricted detail")
				}
			})
		}
	})
}

func shardAdoptionFixture(t *testing.T, edit func(*ProviderRecord, *ProviderRecord)) (string, Manifest) {
	t.Helper()
	root, m := adoptionFixture(t, nil)
	var record ProviderRecord
	raw, err := os.ReadFile(filepath.Join(root, "evidence/provider.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = decodeBounded(raw, &record, MaxProviderBytes); err != nil {
		t.Fatal(err)
	}
	last := ProviderRecord{Schema: record.Schema, ID: record.ID, Version: record.Version, Source: record.Source, RestrictedFindings: record.RestrictedFindings, Capabilities: record.Capabilities}
	record.RestrictedFindings = nil
	if edit != nil {
		edit(&record, &last)
	}
	paths := []string{"shards/first.json", "shards/last.json"}
	if err = os.Mkdir(filepath.Join(root, "shards"), 0700); err != nil {
		t.Fatal(err)
	}
	for i, r := range []ProviderRecord{record, last} {
		raw, err := Encode(r)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(root, paths[i]), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	git(t, root, "add", "shards")
	git(t, root, "commit", "-qm", "adoption shards")
	rev := git(t, root, "rev-parse", "HEAD")
	filtered := m.Inputs[:0]
	for _, in := range m.Inputs {
		if in.Path != "evidence/provider.json" {
			filtered = append(filtered, in)
		}
	}
	m.Inputs = filtered
	scopes := m.Scopes[:0]
	for _, s := range m.Scopes {
		if s.Path != "evidence/provider.json" {
			scopes = append(scopes, s)
		}
	}
	m.Scopes = append(scopes, Scope{Path: "shards", Revision: rev})
	for _, path := range paths {
		raw, _ := os.ReadFile(filepath.Join(root, path))
		m.Inputs = append(m.Inputs, Input{Path: path, Revision: rev, Blob: git(t, root, "rev-parse", rev+":"+path), SHA256: Digest(raw), Provider: "adapter", Purpose: "provider"})
	}
	for i := range m.Providers {
		if m.Providers[i].ID == "adapter" {
			m.Providers[i].Record = ""
			m.Providers[i].Shards = paths
			m.Providers[i].Revision = rev
		}
	}
	return root, m
}
func TestCorpusAdoptionShards(t *testing.T) {
	t.Run("DCP-V1-033 shard closure and DCP-V1-035 late restricted input", func(t *testing.T) {
		for _, name := range []string{"valid", "late restricted", "duplicate", "identity drift", "missing shard", "legacy"} {
			t.Run(name, func(t *testing.T) {
				root, m := shardAdoptionFixture(t, func(first, last *ProviderRecord) {
					switch name {
					case "late restricted":
						first.Subjects[0].Name = "restricted-canary-identity"
					case "duplicate":
						last.Subjects = first.Subjects[:1]
					case "identity drift":
						last.Version = "2"
					}
				})
				if name == "missing shard" {
					m.Inputs = m.Inputs[:len(m.Inputs)-1]
				}
				if name == "legacy" {
					m.Schema = ManifestSchema
				}
				a, err := Build(context.Background(), root, m)
				if name != "valid" {
					if err == nil {
						t.Fatal("invalid shards admitted")
					}
					if strings.Contains(err.Error(), "restricted-canary") {
						t.Fatal("restricted diagnostic")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				raw, err := Encode(a)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = Open(context.Background(), root, raw); err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(raw), "restricted-canary") || len(a.RestrictedSummaries) != 1 {
					t.Fatal("cross shard restrictions lost")
				}
			})
		}
	})
}

func TestCorpusAdoptionReviewRegressions(t *testing.T) {
	t.Run("DCP-V1-035 live restricted path is redacted", func(t *testing.T) {
		root, m := adoptionFixture(t, func(p *ProviderRecord) { p.RestrictedFindings[0].LocalDetailPath = "src/restricted-canary-report.txt" })
		a, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := Encode(a)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(root, "src/restricted-canary-report.txt"), []byte("local restricted detail\n"), 0600); err != nil {
			t.Fatal(err)
		}
		impact := Impact(a, []string{"src/restricted-canary-report.txt"}, "stale")
		projected, err := Encode(impact)
		if err != nil || strings.Contains(string(projected), "restricted-canary") {
			t.Fatal("impact leaked restricted path", err)
		}
		for _, tracked := range []bool{false, true} {
			if tracked {
				git(t, root, "add", "src/restricted-canary-report.txt")
				git(t, root, "commit", "-qm", "local report")
			}
			r, err := ReadQuery(context.Background(), root, raw, Request{Operation: "info"})
			if err != nil {
				t.Fatal(err)
			}
			out, err := Encode(r)
			if err != nil {
				t.Fatal(err)
			}
			if r.Freshness != "stale" || strings.Contains(string(out), "restricted-canary") {
				t.Fatal("live restricted path escaped or stale state lost")
			}
		}
	})
	t.Run("DCP-V1-033 generated v2 cannot become source", func(t *testing.T) {
		root, m := fixture(t)
		path := "src/copied.json"
		if err := os.WriteFile(filepath.Join(root, path), []byte(`{"schema":"corvint-evidence-corpus/2"}`), 0600); err != nil {
			t.Fatal(err)
		}
		git(t, root, "add", path)
		git(t, root, "commit", "-qm", "generated artifact under ordinary name")
		rev := git(t, root, "rev-parse", "HEAD")
		m, err := Inventory(context.Background(), root, rev, "src", m.BuiltAt)
		if err != nil {
			t.Fatal(err)
		}
		m.Schema = ManifestSchemaV2
		if _, err = Build(context.Background(), root, m); err == nil {
			t.Fatal("generated corpus became source")
		}
	})
	t.Run("DCP-V1-036 oversized individual record refuses admission", func(t *testing.T) {
		root, m := adoptionFixture(t, func(p *ProviderRecord) {
			a := p.Claims[0].Evidence.Anchors[0]
			a.Reason = strings.Repeat("x", 64<<10)
			p.Claims[0].Evidence.Anchors = nil
			for i := 0; i < 64; i++ {
				p.Claims[0].Evidence.Anchors = append(p.Claims[0].Evidence.Anchors, a)
			}
		})
		if _, err := Build(context.Background(), root, m); err == nil {
			t.Fatal("unreadable individual record admitted")
		}
	})
}
