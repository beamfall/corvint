package main

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Beamfall/corvint/internal/corpusindex"
	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
)

// statusListingFixture adds a two-unit migration ledger of run-1 and readiness
// claims on the new unit and the flow.
func statusListingFixture(t *testing.T) string {
	t.Helper()
	root, _ := typedCorpusCLIFixtureWith(t, func(p *doccorpus.ProviderRecord, ev doccorpus.Evidence) {
		for _, id := range []string{"proof:unit:old", "proof:unit:new"} {
			p.Subjects = append(p.Subjects, doccorpus.Subject{ID: id, Kind: "inventory", Name: id, Provider: "proof", Evidence: ev})
		}
		p.Relations = append(p.Relations, doccorpus.Relation{ID: "proof:map", From: "proof:unit:old", To: "proof:unit:new", Type: "migration_mapping", Provider: "proof", Evidence: ev})
		p.Details["proof:map"] = doccorpus.RecordDetails{MigrationMapping: &doccorpus.MigrationMappingDetails{Outcome: "moved", Basis: "path", Run: "run-1"}}
		for _, c := range [][2]string{{"proof:ready:new", "proof:unit:new"}, {"proof:ready:flow", "proof:flow"}} {
			p.Claims = append(p.Claims, doccorpus.Claim{ID: c[0], Subject: c[1], Text: c[1] + " readiness", Provider: "proof", Evidence: ev})
			p.Details[c[0]] = doccorpus.RecordDetails{Status: &doccorpus.SuppliedStatusDetails{State: "unknown", Reason: "not verified", Supporting: []string{}}}
		}
	})
	return root
}

func receiptIDs(t *testing.T, out string) string {
	t.Helper()
	var r struct {
		Results []struct {
			ID string `json:"id"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, v := range r.Results {
		ids = append(ids, v.ID)
	}
	return fmt.Sprint(ids)
}

// DCP-V1-318, DCP-V1-319: corpus claims --state [--qualifier] [--run] and
// corpus intent without --id give the docs_claims and docs_intent receipts;
// malformed forms exit 2 or are invalid-arguments, and the companion request
// parser admits the same shapes.
func TestDCPV1318StatusAndIntentListingCLIParity(t *testing.T) {
	root := statusListingFixture(t)
	registry, err := corpusbridge.New(root, "corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		tool, args, want string
		cli              []string
	}{
		{"corvint.docs_claims", `{"state":"unknown"}`, "[proof:ready:flow proof:ready:new]", []string{"claims", "--state", "unknown"}},
		{"corvint.docs_claims", `{"state":"unknown","run":"run-1"}`, "[proof:ready:new]", []string{"claims", "--state", "unknown", "--run", "run-1"}},
		{"corvint.docs_claims", `{"state":"unknown","qualifier":"provisional"}`, "[]", []string{"claims", "--state", "unknown", "--qualifier", "provisional"}},
		{"corvint.docs_claims", `{"state":"unknown","limit":1,"offset":1}`, "[proof:ready:new]", []string{"claims", "--state", "unknown", "--limit", "1", "--offset", "1"}},
		{"corvint.docs_intent", `{}`, "[proof:intent]", []string{"intent"}},
	} {
		code, out, stderr := corpusCLI(t, root, append([]string{"docs", "corpus", c.cli[0], "--artifact", "corpus.json"}, c.cli[1:]...)...)
		if code != 0 {
			t.Fatal(c.cli, code, stderr)
		}
		if got := receiptIDs(t, out); got != c.want {
			t.Fatalf("%v = %s, want %s", c.cli, got, c.want)
		}
		_, text, failure, protocol := registry.Call(context.Background(), c.tool, []byte(c.args))
		if failure != nil || protocol != nil || text != out {
			t.Fatal("CLI/MCP receipt mismatch", c.args, out, text, failure, protocol)
		}
	}
	for _, args := range [][]string{
		{"claims", "--state", "unknown", "--path", "proof.go"},
		{"claims", "--state", "unknown", "--query", "readiness"},
		{"claims", "--run", "run-1"},
		{"claims", "--qualifier", "provisional", "--path", "proof.go"},
		{"claims", "--state", "done"},
		{"claims", "--state=", "--path", "proof.go"},
		{"intent", "--id="},
		{"locate", "--path", "proof.go", "--state", "unknown"},
	} {
		if code, _, _ := corpusCLI(t, root, append([]string{"docs", "corpus", args[0], "--artifact", "corpus.json"}, args[1:]...)...); code != 2 {
			t.Fatalf("corpus %v exit %d, want 2", args, code)
		}
	}
	for _, c := range [][2]string{
		{"corvint.docs_claims", `{"state":"unknown","path":"proof.go"}`},
		{"corvint.docs_claims", `{"state":"unknown","paths":["proof.go"]}`},
		{"corvint.docs_claims", `{"state":"unknown","query":"readiness"}`},
		{"corvint.docs_claims", `{"run":"run-1"}`},
		{"corvint.docs_claims", `{"qualifier":"provisional","path":"proof.go"}`},
		{"corvint.docs_claims", `{"state":"done"}`},
		{"corvint.docs_claims", `{"state":"unknown","run":""}`},
		{"corvint.docs_claims", `{"state":"unknown","qualifier":""}`},
		{"corvint.docs_intent", `{"id":""}`},
		{"corvint.docs_locate", `{"path":"proof.go","state":"unknown"}`},
	} {
		if _, _, _, protocol := registry.Call(context.Background(), c[0], []byte(c[1])); protocol == nil || protocol.Code != "invalid-arguments" {
			t.Fatalf("%s %s admitted: %v", c[0], c[1], protocol)
		}
	}
	for _, tool := range registry.Tools() {
		properties := tool.InputSchema["properties"].(map[string]any)
		_, state := properties["state"]
		switch tool.Name {
		case "corvint.docs_claims":
			if !state || properties["qualifier"] == nil || properties["run"] == nil {
				t.Fatalf("docs_claims schema = %v", tool.InputSchema)
			}
		case "corvint.docs_intent":
			if properties["id"] == nil || len(tool.InputSchema["required"].([]any)) != 0 {
				t.Fatalf("docs_intent schema = %v", tool.InputSchema)
			}
		default:
			if state {
				t.Fatalf("%s advertises state", tool.Name)
			}
		}
	}
	for raw, ok := range map[string]bool{
		`{"operation":"claims","state":"unknown","run":"run-1","limit":20}`:            true,
		`{"operation":"claims","state":"proven","qualifier":"provisional","limit":20}`: true,
		`{"operation":"intent","limit":20}`:                                            true,
		`{"operation":"claims","state":"unknown","path":"proof.go","limit":20}`:        false,
		`{"operation":"claims","state":"unknown","query":"x","limit":20}`:              false,
		`{"operation":"claims","run":"run-1","limit":20}`:                              false,
		`{"operation":"claims","state":"done","limit":20}`:                             false,
		`{"operation":"get","id":"proof:flow","state":"unknown","limit":20}`:           false,
		`{"operation":"claims","state":"unknown","run":"","limit":20}`:                 false,
		`{"operation":"claims","state":"unknown","qualifier":"","limit":20}`:           false,
		`{"operation":"claims","path":"docs/a.md","state":"","limit":20}`:              false,
		`{"operation":"intent","query":"","id":"","path":"","limit":20}`:               true,
	} {
		if _, err := corpusindex.ParseRequest([]byte(raw)); (err == nil) != ok {
			t.Fatalf("companion %s: err=%v, want ok=%v", raw, err, ok)
		}
	}
}
