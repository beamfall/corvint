package doccorpus_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
)

// DCP-V1-174: corvint.docs_flow admits section (one name or a list) and
// include, carries them to the flow query, and refuses malformed filters.
func TestDCPV1174DocsFlowToolFilters(t *testing.T) {
	t.Parallel()
	root, _, a := doccorpus.EightSectionFlowForTest(t)
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
	for _, tool := range r.Tools() {
		props := tool.InputSchema["properties"].(map[string]any)
		if (props["section"] != nil) != (tool.Name == "corvint.docs_flow") || (props["include"] != nil) != (tool.Name == "corvint.docs_flow" || tool.Name == "corvint.docs_search") {
			// DCP-V1-328: docs_search admits include:["claims"] only.
			t.Fatalf("filter schema on %s", tool.Name)
		}
	}
	call := func(name, args string) map[string]any {
		t.Helper()
		result, _, failure, perr := r.Call(context.Background(), name, []byte(args))
		if perr != nil || failure != nil {
			t.Fatalf("%s %s: %v %v", name, args, perr, failure)
		}
		return result
	}
	for args, want := range map[string]int{
		`{"id":"adapter:flow","limit":100}`:                                                        1 + 2 + 1 + 65,
		`{"id":"adapter:flow","limit":100,"section":"entry-points"}`:                               1 + 8,
		`{"id":"adapter:flow","limit":100,"section":["readme","entities"]}`:                        1 + 9 + 8,
		`{"id":"adapter:flow","limit":100,"include":["claims","relations"]}`:                       1 + 2 + 1,
		`{"id":"adapter:flow","limit":100,"section":"entities","include":["paragraphs","claims"]}`: 1 + 2 + 8,
	} {
		result := call("corvint.docs_flow", args)
		if got := len(result["results"].([]any)); got != want {
			t.Fatalf("%s: %d results, want %d", args, got, want)
		}
		if _, ok := result["flow_selection"]; ok == (args == `{"id":"adapter:flow","limit":100}`) {
			t.Fatalf("%s: flow_selection presence %v", args, ok)
		}
	}
	miss := call("corvint.docs_flow", `{"id":"adapter:flow","section":"B"}`)
	selection, _ := miss["flow_selection"].(map[string]any)
	if miss["miss"] != "none-found" || selection == nil || len(selection["sections"].([]any)) != 8 {
		t.Fatalf("unknown section: %v", miss)
	}
	for name, args := range map[string]string{
		"corvint.docs_flow": `{"id":"adapter:flow","include":["subjects"]}`,
		"corvint.docs_get":  `{"id":"adapter:flow","section":"readme"}`,
	} {
		if _, _, _, perr := r.Call(context.Background(), name, []byte(args)); perr == nil || perr.Code != "invalid-arguments" {
			t.Fatalf("%s %s admitted: %v", name, args, perr)
		}
	}
	for _, args := range []string{`{"id":"adapter:flow","section":7}`, `{"id":"adapter:flow","section":[]}`, `{"id":"adapter:flow","section":["readme","readme"]}`, `{"id":"adapter:flow","section":""}`, `{"id":"adapter:flow","include":[]}`, `{"id":"adapter:flow","include":null}`, `{"id":"adapter:flow","section":"readme","include":["claims"]}`} {
		if _, _, _, perr := r.Call(context.Background(), "corvint.docs_flow", []byte(args)); perr == nil || perr.Code != "invalid-arguments" {
			t.Fatalf("%s admitted: %v", args, perr)
		}
	}
}
