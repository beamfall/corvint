package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
)

// The provider fixture pins source before recording declarations. Expected
// tools and arguments below are independent of the production operation map.
func typedCorpusCLIFixture(t *testing.T) (string, doccorpus.Manifest) {
	t.Helper()
	root := taskContextRepository(t)
	cemWrite(t, root, "proof.go", "package cache\nfunc Checkout() {}\n")
	cemGit(t, root, "add", "proof.go")
	cemGit(t, root, "commit", "-qm", "typed source")
	rev := cemGit(t, root, "rev-parse", "HEAD")
	m, err := doccorpus.Inventory(context.Background(), root, rev, "proof.go", "2026-09-30T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	m.Schema = doccorpus.ManifestSchemaV2
	in := m.Inputs[0]
	ev := doccorpus.Evidence{Derivation: "declared", Trust: "generated", State: "supported", Freshness: "fresh", Anchors: []doccorpus.Anchor{{Repository: m.Repository.ID, Revision: rev, Path: in.Path, Blob: in.Blob, SHA256: in.SHA256, Start: 2, End: 2, SpanSHA256: doccorpus.Digest([]byte("func Checkout() {}\n")), Symbol: "Checkout", Authority: "external-provider", Kind: "declared", Reason: "bounded synthetic typed declaration"}}, Limitations: []string{"provider declarations are not authenticated observations"}}
	p := doccorpus.ProviderRecord{Schema: doccorpus.AdoptionProviderSchema, ID: "proof", Version: "1", Source: m.Repository, Subjects: []doccorpus.Subject{}, Claims: []doccorpus.Claim{{ID: "proof:claim", Subject: "proof:flow", Text: "Checkout is declared", Provider: "proof", Evidence: ev}}, Relations: []doccorpus.Relation{}, Journeys: []doccorpus.Journey{{ID: "proof:journey", Subject: "proof:flow", Provider: "proof", Status: "non_ui", Preconditions: []string{}, Cleanup: "not-run", Steps: []doccorpus.Step{}, Evidence: ev}}, Observations: []doccorpus.ObservationLink{}, Details: map[string]doccorpus.RecordDetails{"proof:claim": {ClaimKind: "behavior"}}, Capabilities: []doccorpus.CapabilityDeclaration{{Name: "subjects", State: "present", Reason: "declared inventory"}, {Name: "claims", State: "present", Reason: "declared claims"}, {Name: "relations", State: "present", Reason: "declared relations"}, {Name: "journeys", State: "present", Reason: "explicit non-UI disposition"}}}
	add := func(id, kind, name string, detail doccorpus.RecordDetails) {
		p.Subjects = append(p.Subjects, doccorpus.Subject{ID: id, Kind: kind, Name: name, Provider: "proof", Evidence: ev})
		if !reflect.DeepEqual(detail, doccorpus.RecordDetails{}) {
			p.Details[id] = detail
		}
	}
	sections := []doccorpus.FlowSection{}
	for _, name := range []string{"readme", "entry-points", "flow-diagram", "functional-overview", "technical-deep-dive", "entities", "exceptions", "related-flows"} {
		sections = append(sections, doccorpus.FlowSection{Name: name, Paragraphs: []string{}})
	}
	sections[0].Paragraphs = []string{"proof:paragraph"}
	add("proof:flow", "flow", "Checkout", doccorpus.RecordDetails{Flow: &doccorpus.FlowDetails{Sections: sections, Actors: []string{"operator"}, Entrypoints: []string{"proof:screen"}, Variations: []string{}}})
	add("proof:paragraph", "paragraph", "Checkout rationale", doccorpus.RecordDetails{Paragraph: &doccorpus.ParagraphDetails{Flow: "proof:flow", Section: "readme", StableID: "proof:stable"}})
	emptySections := append([]doccorpus.FlowSection{}, sections...)
	emptySections[0].Paragraphs = []string{}
	add("proof:upstream", "flow", "Authorize", doccorpus.RecordDetails{Flow: &doccorpus.FlowDetails{Sections: emptySections, Actors: []string{}, Entrypoints: []string{}, Variations: []string{}}})
	add("proof:downstream", "flow", "Complete", doccorpus.RecordDetails{Flow: &doccorpus.FlowDetails{Sections: emptySections, Actors: []string{}, Entrypoints: []string{}, Variations: []string{}}})
	add("proof:screen", "ui_surface", "Checkout screen", doccorpus.RecordDetails{})
	add("proof:term", "business_term", "purchase", doccorpus.RecordDetails{})
	add("proof:test", "test", "TestCheckout", doccorpus.RecordDetails{})
	p.Details["proof:route"] = doccorpus.RecordDetails{TestLink: &doccorpus.TestLinkDetails{Role: "navigating", JoinConfidence: "exact", File: "proof.go", Title: "TestCheckout", Project: "declared"}}
	add("proof:coverage", "coverage_definition", "Checkout coverage", doccorpus.RecordDetails{Coverage: &doccorpus.CoverageDetails{Definition: "explicit claim membership", Denominator: []string{"proof:claim"}, Numerator: []string{"proof:claim"}, Rule: "explicit-membership"}})
	add("proof:ticket", "ticket", "Ticket", doccorpus.RecordDetails{Ticket: &doccorpus.TicketDetails{ExternalID: "PROOF-1", Status: "closed", History: []doccorpus.TicketHistory{{At: "2026-09-30T00:00:00Z", Revision: rev, Summary: "Recorded intent"}}}})
	add("proof:intent", "intent_comparison", "Intent", doccorpus.RecordDetails{IntentComparison: &doccorpus.IntentComparisonDetails{ReportedIntent: []string{"proof:paragraph"}, ObservedClaims: []string{"proof:claim"}, Missing: []string{}, Conflicts: []string{}}})
	for _, r := range []doccorpus.Relation{{ID: "proof:up", From: "proof:upstream", To: "proof:flow", Type: "depends_on", Provider: "proof", Evidence: ev}, {ID: "proof:down", From: "proof:flow", To: "proof:downstream", Type: "depends_on", Provider: "proof", Evidence: ev}, {ID: "proof:alias", From: "proof:term", To: "proof:flow", Type: "related_to", Provider: "proof", Evidence: ev}, {ID: "proof:screen-join", From: "proof:screen", To: "proof:flow", Type: "related_to", Provider: "proof", Evidence: ev}, {ID: "proof:route", From: "proof:flow", To: "proof:test", Type: "navigates", Provider: "proof", Evidence: ev}} {
		p.Relations = append(p.Relations, r)
	}
	raw, err := doccorpus.Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	cemWrite(t, root, "provider.json", string(raw))
	cemGit(t, root, "add", "provider.json")
	cemGit(t, root, "commit", "-qm", "typed provider")
	pr := cemGit(t, root, "rev-parse", "HEAD")
	m.Providers = append(m.Providers, doccorpus.Provider{ID: "proof", Kind: "records", Version: "1", Revision: pr, Record: "provider.json"})
	m.Scopes = append(m.Scopes, doccorpus.Scope{Path: "provider.json", Revision: pr})
	m.Inputs = append(m.Inputs, doccorpus.Input{Path: "provider.json", Revision: pr, Blob: cemGit(t, root, "rev-parse", pr+":provider.json"), SHA256: doccorpus.Digest(raw), Provider: "proof", Purpose: "provider"})
	a, err := doccorpus.Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = doccorpus.Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	cemWrite(t, root, "corpus.json", string(raw))
	return root, m
}

func TestCorpusTypedCLIMCPEndToEnd(t *testing.T) {
	t.Run("DCP-V1-038 symbol selectors preserve exact claims", func(t *testing.T) {
		root, _ := typedCorpusCLIFixture(t)
		registry, err := corpusbridge.New(root, "corpus.json")
		if err != nil {
			t.Fatal(err)
		}
		for _, selector := range []string{"Checkout", "UndocumentedSymbol"} {
			code, out, stderr := corpusCLI(t, root, "docs", "corpus", "claims", "--artifact", "corpus.json", "--path", selector)
			if code != 0 {
				t.Fatal(code, stderr)
			}
			var receipt, other doccorpus.Receipt
			if err := json.Unmarshal([]byte(out), &receipt); err != nil {
				t.Fatal(err)
			}
			args, _ := json.Marshal(map[string]string{"path": selector})
			_, text, failure, protocol := registry.Call(context.Background(), "corvint.docs_claims", args)
			if failure != nil || protocol != nil {
				t.Fatal(failure, protocol)
			}
			if err := json.Unmarshal([]byte(text), &other); err != nil || !reflect.DeepEqual(receipt, other) {
				t.Fatal("symbol CLI/MCP complete receipt mismatch", err)
			}
			if selector == "UndocumentedSymbol" {
				if len(receipt.Results) != 0 || receipt.Meaning != "not documented" || receipt.Miss != "no-match" {
					t.Fatal("undocumented symbol lost typed miss", receipt)
				}
				continue
			}
			found := false
			for _, result := range receipt.Results {
				data, _ := json.Marshal(result)
				var claim doccorpus.Claim
				if err := json.Unmarshal(data, &claim); err != nil {
					t.Fatal(err)
				}
				found = found || claim.ID == "proof:claim"
			}
			anchored := false
			for _, citation := range receipt.Citations {
				anchored = anchored || citation.Symbol == "Checkout" && citation.Path == "proof.go"
			}
			if !found || !anchored {
				t.Fatal("symbol selector lost exact claim or anchor", receipt)
			}
		}
	})
	t.Run("DCP-V1-038 DCP-V1-039 real typed claims parity", func(t *testing.T) {
		root, m := typedCorpusCLIFixture(t)
		code, out, stderr := corpusCLI(t, root, "docs", "corpus", "claims", "--artifact", "corpus.json", "--path", "proof.go")
		if code != 0 {
			t.Fatal(code, stderr)
		}
		var receipt doccorpus.Receipt
		if err := json.Unmarshal([]byte(out), &receipt); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, v := range receipt.Results {
			b, _ := json.Marshal(v)
			if strings.Contains(string(b), `"id":"proof:claim"`) {
				found = true
			}
		}
		if !found || receipt.Envelope == nil || receipt.Envelope.ContentStatus != "generated" || receipt.Envelope.SourceRevision != m.Repository.Revision || len(receipt.Citations) == 0 || receipt.Citations[0].Path != "proof.go" {
			t.Fatal("exact declared claim/trust/source anchor lost", out)
		}
		registry, err := corpusbridge.New(root, "corpus.json")
		if err != nil {
			t.Fatal(err)
		}
		_, text, failure, protocol := registry.Call(context.Background(), "corvint.docs_claims", []byte(`{"path":"proof.go"}`))
		if failure != nil || protocol != nil {
			t.Fatal(failure, protocol)
		}
		var other doccorpus.Receipt
		if err := json.Unmarshal([]byte(text), &other); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(receipt, other) {
			t.Fatal("complete CLI/MCP receipt mismatch", out, text)
		}
		if _, err := os.Stat(filepath.Join(root, "provider.json")); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("DCP-V1-038 DCP-V1-039 independent complete operation oracle", func(t *testing.T) {
		root, m := typedCorpusCLIFixture(t)
		registry, err := corpusbridge.New(root, "corpus.json")
		if err != nil {
			t.Fatal(err)
		}
		cases := []struct{ op, tool, arg, value, wantID string }{
			{"info", "corvint.docs_info", "", "", ""},
			{"validate", "corvint.docs_validate", "", "", ""},
			{"search", "corvint.docs_search", "query", "Checkout", "proof:flow"},
			{"get", "corvint.docs_get", "id", "proof:flow", "proof:flow"},
			{"trace", "corvint.docs_trace", "id", "proof:flow", "proof:up"},
			{"locate", "corvint.docs_locate", "path", "proof.go", "proof:flow"},
			{"related", "corvint.docs_find_related", "id", "proof:flow", "proof:route"},
			{"coverage", "corvint.docs_coverage", "", "", ""},
			{"gaps", "corvint.docs_gaps", "", "", ""},
			{"journey", "corvint.docs_get_journey", "id", "proof:flow", "proof:journey"},
			{"stability", "corvint.docs_get_stability", "id", "proof:flow", ""},
			{"inventory", "corvint.docs_inventory", "", "", "proof:claim"},
			{"concept", "corvint.docs_concept", "query", "Checkout", "proof:flow"},
			{"claims", "corvint.docs_claims", "path", "proof.go", "proof:claim"},
			{"flow", "corvint.docs_flow", "id", "proof:flow", "proof:paragraph"},
			{"dependencies", "corvint.docs_dependencies", "id", "proof:flow", ""},
			{"recommend-tests", "corvint.docs_recommend_tests", "path", "proof.go", "proof:flow"},
			{"navigation", "corvint.docs_navigation", "id", "proof:flow", "proof:screen"},
			{"vocabulary", "corvint.docs_vocabulary", "query", "purchase", "proof:alias"},
			{"intent", "corvint.docs_intent", "id", "proof:intent", "proof:intent"},
		}
		advertised := map[string]bool{}
		for _, tool := range registry.Tools() {
			advertised[tool.Name] = true
		}
		if len(advertised) != 19 {
			t.Fatal("independent expected tool count", advertised)
		}
		for _, tc := range cases {
			t.Run(tc.op, func(t *testing.T) {
				args := []string{"docs", "corpus", tc.op, "--artifact", "corpus.json"}
				input := map[string]any{}
				if tc.arg != "" {
					args = append(args, "--"+tc.arg, tc.value)
					input[tc.arg] = tc.value
				}
				code, out, stderr := corpusCLI(t, root, args...)
				if code != 0 {
					t.Fatal(code, stderr)
				}
				var receipt doccorpus.Receipt
				if err := json.Unmarshal([]byte(out), &receipt); err != nil {
					t.Fatal(err)
				}
				if receipt.Operation != tc.op || receipt.Envelope == nil || receipt.Envelope.ContentStatus != "generated" || receipt.Envelope.SourceRevision != m.Repository.Revision || receipt.Envelope.CorpusRevision != receipt.ArtifactSHA256 || receipt.Envelope.Freshness != receipt.Freshness || receipt.Envelope.Retirement.State != "unknown" || !reflect.DeepEqual(receipt.Citations, receipt.Envelope.Citations) || len(receipt.Envelope.Limitations) == 0 {
					t.Fatal("uniform trust envelope lost", out)
				}
				encoded, _ := json.Marshal(input)
				_, text, failure, protocol := registry.Call(context.Background(), tc.tool, encoded)
				if tc.op == "stability" {
					if advertised[tc.tool] || protocol == nil || protocol.Code != "unsupported-tool" || receipt.State != "unavailable" || receipt.Miss != "capability-absent" {
						t.Fatal("absent stability advertised or flattened", out, protocol)
					}
					return
				}
				if !advertised[tc.tool] || failure != nil || protocol != nil {
					t.Fatal("expected operation unavailable", tc.tool, failure, protocol)
				}
				var other doccorpus.Receipt
				if err := json.Unmarshal([]byte(text), &other); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(receipt, other) {
					t.Fatal("full native receipt parity lost", out, text)
				}
				results, _ := json.Marshal(receipt.Results)
				if tc.wantID != "" && !strings.Contains(string(results), `"id":"`+tc.wantID+`"`) {
					t.Fatal("expected record absent", tc.wantID, string(results))
				}
				switch tc.op {
				case "flow":
					if d := receipt.Details["proof:flow"].Flow; d == nil || len(d.Sections) != 8 || !reflect.DeepEqual(d.Entrypoints, []string{"proof:screen"}) || !reflect.DeepEqual(d.Sections[0].Paragraphs, []string{"proof:paragraph"}) {
						t.Fatal("typed flow fields lost", receipt.Details)
					}
				case "dependencies":
					if len(receipt.Results) != 2 || !strings.Contains(string(results), `"direction":"upstream"`) || !strings.Contains(string(results), `"direction":"downstream"`) || !strings.Contains(string(results), `"id":"proof:up"`) || !strings.Contains(string(results), `"id":"proof:down"`) {
						t.Fatal("directed dependencies lost", string(results))
					}
				case "recommend-tests":
					if receipt.Selection == nil || receipt.Selection.NarrowingAllowed || receipt.Selection.State != "full-relevant-suite-required" || len(receipt.Selection.MissingEvidence) == 0 {
						t.Fatal("unsafe test narrowing", out)
					}
				case "navigation":
					if len(receipt.Citations) == 0 || !strings.Contains(string(results), `"id":"proof:route"`) {
						t.Fatal("screen/route evidence lost", out)
					}
				case "intent":
					if d := receipt.Details["proof:intent"].IntentComparison; d == nil || !reflect.DeepEqual(d.ReportedIntent, []string{"proof:paragraph"}) || !reflect.DeepEqual(d.ObservedClaims, []string{"proof:claim"}) {
						t.Fatal("intent and observation conflated", out)
					}
				case "coverage":
					if !strings.Contains(string(results), `"denominator":1`) || !strings.Contains(string(results), `"definition":"explicit claim membership"`) {
						t.Fatal("named coverage denominator lost", string(results))
					}
				}
			})
		}
		for _, op := range []struct{ op, tool, arg string }{{"claims", "corvint.docs_claims", "path"}, {"concept", "corvint.docs_concept", "query"}, {"intent", "corvint.docs_intent", "id"}} {
			t.Run(op.op+" missing", func(t *testing.T) {
				code, out, stderr := corpusCLI(t, root, "docs", "corpus", op.op, "--artifact", "corpus.json", "--"+op.arg, "not-documented")
				if code != 0 {
					t.Fatal(stderr)
				}
				var r doccorpus.Receipt
				json.Unmarshal([]byte(out), &r)
				if r.State != "empty" || r.Meaning != "not documented" || len(r.Results) != 0 {
					t.Fatal("missing documentation became nonexistence", out)
				}
				b, _ := json.Marshal(map[string]string{op.arg: "not-documented"})
				_, text, failure, protocol := registry.Call(context.Background(), op.tool, b)
				if failure != nil || protocol != nil || text != out {
					t.Fatal("typed miss parity", out, text, failure, protocol)
				}
			})
		}
	})
}
