package main

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/repoenvelope"
)

// DCP-V1-761, DCP-V1-762: GH1996-common-only reaches both MCP receipt views,
// including framed text, while preserving the legacy hit outcome and records.
func TestDCPV1761SearchQualityOnMCPWire(t *testing.T) {
	t.Parallel()
	root, _ := corpusServerFixture(t, "# Evidence\n\nOriginal source.\n")
	for _, view := range []string{"compact", "full"} {
		input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}},"name":"corvint.docs_search","arguments":{"query":"Original zxqmissing","receipt":"` + view + `"}}}` + "\n"
		var stdout, stderr bytes.Buffer
		if exit := run(context.Background(), []string{"--root", root, "--artifact", "corpus.json"}, strings.NewReader(input), &stdout, &stderr); exit != 0 {
			t.Fatalf("%s: exit=%d stderr=%q", view, exit, stderr.String())
		}
		var response struct {
			Result struct {
				IsError    bool           `json:"isError"`
				Structured map[string]any `json:"structuredContent"`
				Content    []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"result"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		r := response.Result
		q, _ := r.Structured["search_quality"].(map[string]any)
		results, _ := r.Structured["results"].([]any)
		if r.IsError || q["schema"] != "corvint-corpus-search-quality/1" || q["outcome"] != "weak" || q["reason"] != "all-rare-query-terms-unmatched" || len(results) == 0 || r.Structured["miss"] != "" {
			t.Fatalf("%s: weak explanation or retained hits missing: %s", view, stdout.String())
		}
		if len(r.Content) != 1 || !strings.HasPrefix(r.Content[0].Text, repoenvelope.Prefix) || !strings.HasSuffix(r.Content[0].Text, repoenvelope.Suffix) {
			t.Fatalf("%s: repository framing missing", view)
		}
		payload := strings.TrimSuffix(strings.TrimPrefix(r.Content[0].Text, repoenvelope.Prefix), repoenvelope.Suffix)
		var framed map[string]any
		if err := json.Unmarshal([]byte(payload), &framed); err != nil || !reflect.DeepEqual(framed, r.Structured) {
			t.Fatalf("%s: framed receipt differs: %v", view, err)
		}
	}
}
