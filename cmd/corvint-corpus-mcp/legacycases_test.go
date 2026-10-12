package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

// DCP-V1-077: corvint-corpus-mcp lists corvint.docs_search_cases only with
// --legacy-cases, decodes its arguments strictly, and returns the CLI's exact
// search-cases bytes framed as untrusted data, never step text.
func TestDCPV1077CorpusMCPSearchCases(t *testing.T) {
	root, _ := corpusServerFixture(t, "# Evidence\n\nOriginal source.\n")
	export := "ID,Title,Section Hierarchy,Suite,Steps\nC7,Manager approves request,Invoices > Refund approvals,Billing,Open the STEPQUEUE\n"
	if err := os.WriteFile(filepath.Join(root, "cases.csv"), []byte(export), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		args    []string
		ok, set bool
	}{
		{[]string{"--root", root, "--artifact", "corpus.json"}, true, false},
		{[]string{"--root", root, "--artifact", "corpus.json", "--legacy-cases"}, true, true},
		{[]string{"--root", root, "--artifact", "corpus.json", "--legacy-cases", "--legacy-cases"}, false, false},
		{[]string{"--legacy-cases", "--root", root, "--artifact", "corpus.json"}, false, false},
	} {
		_, set, ok := extractLegacyCasesFlag(c.args)
		if ok != c.ok || set != c.set {
			t.Errorf("extractLegacyCasesFlag(%q) = %v %v", c.args, set, ok)
		}
	}
	registry, err := corpusbridge.New(root, "corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	listed := func(h *toolHandler) bool {
		result, rpc := h.Handle(context.Background(), protocol.Request{Method: "tools/list", Params: map[string]any{}}, nil)
		if rpc != nil {
			t.Fatal(rpc)
		}
		raw, _ := json.Marshal(result["tools"])
		return bytes.Contains(raw, []byte(legacyCasesTool))
	}
	if listed(&toolHandler{registry: registry}) {
		t.Fatal("search_cases listed without --legacy-cases")
	}
	h := &toolHandler{registry: registry, legacyCases: &legacyCaseSearcher{root: root}}
	if !listed(h) {
		t.Fatal("search_cases not listed")
	}
	call := func(arguments map[string]any) (map[string]any, *protocol.RPCError) {
		return h.call(context.Background(), map[string]any{"name": legacyCasesTool, "arguments": arguments})
	}
	result, rpc := call(map[string]any{"format": "csv", "input": "cases.csv", "query": "refund", "limit": 3})
	if rpc != nil || result["isError"] != false {
		t.Fatalf("%v %v", rpc, result)
	}
	want, searchErr := doccorpus.SearchLegacyCases("csv", "cases.csv", []byte(export), "refund", 3)
	if searchErr != nil {
		t.Fatal(searchErr)
	}
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	framed, _ := repoenvelope.Frame(string(want))
	if text != framed || strings.Contains(text, "STEPQUEUE") || result["structuredContent"].(map[string]any)["state"] != "found" {
		t.Fatalf("search_cases output differs from the CLI bytes: %s", text)
	}
	for _, bad := range []map[string]any{
		{"format": "csv", "input": "cases.csv"},
		{"format": "csv", "input": "cases.csv", "query": "x", "extra": 1},
		{"format": "csv", "input": "cases.csv", "query": "x", "limit": 0},
		{"format": "csv", "input": "cases.csv", "query": nil},
	} {
		if _, rpc := call(bad); rpc == nil {
			t.Errorf("%v admitted", bad)
		}
	}
	for _, refused := range []map[string]any{
		{"format": "csv", "input": "../cases.csv", "query": "x"},
		{"format": "xlsx", "input": "cases.csv", "query": "x"},
	} {
		if result, rpc := call(refused); rpc != nil || result["isError"] != true {
			t.Errorf("%v not refused: %v %v", refused, rpc, result)
		}
	}
}
