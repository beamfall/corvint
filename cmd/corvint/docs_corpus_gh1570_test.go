package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
)

// DCP-V1-321, DCP-V1-328, DCP-V1-331: corpus gaps takes --kind and corpus
// search takes --kind and --include claims, each with the receipt of the
// matching MCP call; search refuses any other include and get refuses --kind.
func TestDCPV1321CorpusKindAndClaimSearchCLIParity(t *testing.T) {
	root, _ := typedCorpusCLIFixture(t)
	registry, err := corpusbridge.New(root, "corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		args       []string
		tool, json string
	}{
		{[]string{"gaps", "--kind", "no-such-kind"}, "corvint.docs_gaps", `{"kind":"no-such-kind"}`},
		{[]string{"search", "--query", "checkout", "--include", "claims"}, "corvint.docs_search", `{"query":"checkout","include":["claims"]}`},
		{[]string{"search", "--query", "checkout", "--kind", "capability"}, "corvint.docs_search", `{"query":"checkout","kind":"capability"}`},
	} {
		code, out, stderr := corpusCLI(t, root, append([]string{"docs", "corpus"}, append(c.args, "--artifact", "corpus.json")...)...)
		if code != 0 {
			t.Fatal(c.args, code, stderr)
		}
		_, text, failure, protocol := registry.Call(context.Background(), c.tool, []byte(c.json))
		if failure != nil || protocol != nil || text != out {
			t.Fatal("CLI/MCP receipt mismatch", c.args, out, text, failure, protocol)
		}
	}
	code, out, _ := corpusCLI(t, root, "docs", "corpus", "gaps", "--kind", "no-such-kind", "--artifact", "corpus.json")
	var receipt doccorpus.Receipt
	if code != 0 || json.Unmarshal([]byte(out), &receipt) != nil || len(receipt.KindTotals) != 1 {
		t.Fatal("gaps --kind totals", out)
	}
	for _, args := range [][]string{
		{"search", "--query", "checkout", "--include", "sections"},
		{"get", "--id", "proof:claim", "--kind", "capability"},
		{"gaps", "--kind", "a,a"},
	} {
		if code, _, _ := corpusCLI(t, root, append([]string{"docs", "corpus"}, append(args, "--artifact", "corpus.json")...)...); code != 2 {
			t.Fatalf("corpus %v exit %d, want 2", args, code)
		}
	}
}
