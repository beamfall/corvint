package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/corpusindex"
	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
)

// DCP-V1-313: corpus claims ranks claims by --query text with the same
// receipt as docs_claims, and takes exactly one of --path or --query; the
// companion request parser admits the same shape.
func TestDCPV1313CorpusClaimsQueryCLIParity(t *testing.T) {
	root, _ := typedCorpusCLIFixture(t)
	code, out, stderr := corpusCLI(t, root, "docs", "corpus", "claims", "--artifact", "corpus.json", "--query", "checkout declared")
	if code != 0 {
		t.Fatal(code, stderr)
	}
	var receipt doccorpus.Receipt
	if err := json.Unmarshal([]byte(out), &receipt); err != nil {
		t.Fatal(err)
	}
	// Both claims hold "checkout"; the one holding both terms ranks first.
	if len(receipt.Results) != 2 {
		t.Fatal("claim text query did not return the matching claims", out)
	}
	if b, _ := json.Marshal(receipt.Results[0]); !strings.Contains(string(b), `"id":"proof:claim"`) {
		t.Fatal("claim text query ranked", out)
	}
	registry, err := corpusbridge.New(root, "corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	_, text, failure, protocol := registry.Call(context.Background(), "corvint.docs_claims", []byte(`{"query":"checkout declared"}`))
	if failure != nil || protocol != nil || text != out {
		t.Fatal("CLI/MCP claim query receipt mismatch", out, text, failure, protocol)
	}
	for _, args := range [][]string{{"--query", "checkout", "--path", "proof.go"}, {}} {
		if code, _, _ := corpusCLI(t, root, append([]string{"docs", "corpus", "claims", "--artifact", "corpus.json"}, args...)...); code != 2 {
			t.Fatalf("corpus claims %v exit %d, want 2", args, code)
		}
	}
	if q, err := corpusindex.ParseRequest([]byte(`{"operation":"claims","query":"checkout","limit":20}`)); err != nil || q.Query != "checkout" {
		t.Fatal("companion refused a claim text query", err)
	}
	if _, err := corpusindex.ParseRequest([]byte(`{"operation":"claims","query":"checkout","path":"proof.go","limit":20}`)); err == nil {
		t.Fatal("companion admitted both path and query")
	}
}
