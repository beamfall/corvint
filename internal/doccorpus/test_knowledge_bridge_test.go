package doccorpus_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
)

// DCP-V1-091 (V1-1873): test-knowledge rows and their trust labels cross the
// corpus MCP bridge byte-identical to the ReadQuery receipt for the same query.
func TestDCPV1091TestKnowledgeAcrossCorpusBridge(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root, m, _ := doccorpus.TestKnowledgeFixtureForTest(t, nil, nil)
	a, err := doccorpus.Build(ctx, root, m)
	if err != nil {
		t.Fatal(err)
	}
	data, err := doccorpus.Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "corpus.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	r, e := corpusbridge.New(root, "corpus.json")
	if e != nil {
		t.Fatal(e.Code)
	}
	for _, c := range []struct {
		tool, args string
		query      doccorpus.Request
	}{
		{"corvint.docs_get", `{"id":"tk:count-unit"}`, doccorpus.Request{Operation: "get", ID: "tk:count-unit", Limit: 20}},
		{"corvint.docs_flow", `{"id":"adapter:flow"}`, doccorpus.Request{Operation: "flow", ID: "adapter:flow", Limit: 20}},
		{"corvint.docs_claims", `{"path":"src/value.go"}`, doccorpus.Request{Operation: "claims", Path: "src/value.go", Limit: 20}},
		{"corvint.docs_locate", `{"path":"src/value_test.go"}`, doccorpus.Request{Operation: "locate", Path: "src/value_test.go", Limit: 20}},
		{"corvint.docs_recommend_tests", `{"path":"src/value.go"}`, doccorpus.Request{Operation: "recommend-tests", Path: "src/value.go", Limit: 20}},
	} {
		want, err := doccorpus.ReadQuery(ctx, root, data, c.query)
		if err != nil {
			t.Fatal(err)
		}
		wantText, err := doccorpus.Encode(want)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(wantText), `"trust_level":"run+reviewed"`) {
			t.Fatalf("%s: ReadQuery returned no labelled test-knowledge row; parity is vacuous", c.tool)
		}
		_, text, failure, perr := r.Call(ctx, c.tool, []byte(c.args))
		if failure != nil || perr != nil || text != string(wantText) {
			t.Fatalf("%s: failure=%v protocol=%v byte-identical=%v", c.tool, failure, perr, text == string(wantText))
		}
	}
}
