package corpusbridge

import (
	"context"
	"testing"
)

const claimEvalCases = `{"schema":"corvint-corpus-eval-cases/1","origin":"labelled","cases":[
{"id":"claim-exact","operation":"claims","arguments":{"query":"Checkout step 1."},"expect":{"claims":["native:file:docs/p1.md:paragraph:3"]}},
{"id":"claim-terms","operation":"claims","arguments":{"query":"p2 checkout","limit":1},"expect":{"claims":["native:file:docs/p2.md:paragraph:3"]}},
{"id":"claim-absent","operation":"claims","arguments":{"query":"zebra"},"expect":{"abstain":true}}
]}`

// DCP-V1-313: docs_claims ranks claims by text, so a caller-labelled
// expect.claims case reaches nonzero claim recall through the MCP tool path.
func TestDCPV1313ClaimTextQueryReachesClaimRecall(t *testing.T) {
	root, _ := lazyFixture(t)
	report := evalRun(t, root, claimEvalCases)
	for _, id := range []string{"claim-exact", "claim-terms"} {
		c := evalCase(t, report, id)
		if c.Status != "pass" || c.Rank == nil || *c.Rank != 1 || !same(c.RecallAtK, 1, 1) {
			t.Fatalf("%s = %+v", id, c)
		}
	}
	if c := evalCase(t, report, "claim-absent"); c.Status != "pass" || c.Abstention != "correct" {
		t.Fatalf("claim-absent = %+v", c)
	}
	registry, err := New(root, "corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	// Exactly one of path, paths or query; an empty query is refused.
	for _, arguments := range []string{`{"query":"checkout","path":"docs/p1.md"}`, `{"query":"checkout","paths":["docs/p1.md"]}`, `{"query":""}`, `{}`} {
		if _, _, _, protocol := registry.Call(context.Background(), "corvint.docs_claims", []byte(arguments)); protocol == nil || protocol.Code != "invalid-arguments" {
			t.Fatalf("%s admitted: %v", arguments, protocol)
		}
	}
	// The path form is unchanged.
	structured, _, failure, protocol := registry.Call(context.Background(), "corvint.docs_claims", []byte(`{"path":"docs/p1.md"}`))
	if failure != nil || protocol != nil || len(structured["results"].([]any)) == 0 {
		t.Fatalf("path claims changed: %v %v %v", structured, failure, protocol)
	}
	// The query property is advertised on docs_claims only, and is optional there.
	for _, tool := range registry.Tools() {
		properties := tool.InputSchema["properties"].(map[string]any)
		_, has := properties["query"]
		switch tool.Name {
		case "corvint.docs_claims":
			if !has || len(tool.InputSchema["required"].([]any)) != 0 {
				t.Fatalf("docs_claims schema = %v", tool.InputSchema)
			}
		case "corvint.docs_search", "corvint.docs_concept", "corvint.docs_vocabulary", "corvint.docs_locate", "corvint.docs_navigation": // DCP-V1-650, DCS-V1-597
		default:
			if has {
				t.Fatalf("%s advertises query", tool.Name)
			}
		}
	}
}
