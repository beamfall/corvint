package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
)

func outcomeHandler(t *testing.T, root, log string) *toolHandler {
	t.Helper()
	registry, e := corpusbridge.New(root, "corpus.json")
	if e != nil {
		t.Fatal(e)
	}
	handler := &toolHandler{registry: registry}
	if log != "" {
		listed := []string{}
		for _, tool := range handler.listedTools() {
			listed = append(listed, tool.Name)
		}
		var err error
		if handler.outcomes, err = openOutcomeLog(log, registry.ArtifactSHA256(), listed, io.Discard); err != nil {
			t.Fatal(err)
		}
	}
	return handler
}

func withoutMeta(result map[string]any) map[string]any {
	copied := map[string]any{}
	for k, v := range result {
		if k != "_meta" {
			copied[k] = v
		}
	}
	return copied
}

func TestDCPV1285OutcomeLogAggregatesWithoutRequestText(t *testing.T) {
	root, _ := corpusServerFixture(t, "# Zebracorn alpha\n\nThe zebracorn-alpha claim holds.\n")
	if _, path, ok := extractOutcomeLogFlag([]string{"--root", root, "--artifact", "corpus.json"}); !ok || path != "" {
		t.Fatalf("outcome log is not off by default: %q %v", path, ok)
	}
	log := filepath.Join(t.TempDir(), "outcomes.json")
	plain, logged := outcomeHandler(t, root, ""), outcomeHandler(t, root, log)
	secrets := []string{"zebracorn-alpha", "qqqnomatchqqq", "missing-secret-path.md", "secret-argument-value", "corvint.secret_tool_name"}
	calls := []map[string]any{
		{"name": "corvint.docs_search", "arguments": map[string]any{"query": "zebracorn-alpha"}},
		{"name": "corvint.docs_search", "arguments": map[string]any{"query": "qqqnomatchqqq"}},
		{"name": "corvint.docs_get", "arguments": map[string]any{"path": "source.md"}},
		{"name": "corvint.docs_get", "arguments": map[string]any{"path": "missing-secret-path.md"}},
		{"name": "corvint.docs_search", "arguments": map[string]any{"query": "x", "bogus": "secret-argument-value"}},
		{"name": "corvint.secret_tool_name", "arguments": map[string]any{}},
		{"name": "corvint.docs_info"},
	}
	for _, params := range calls {
		want, wantErr := plain.Handle(context.Background(), protocol.Request{Method: "tools/call", Params: params}, nil)
		got, gotErr := logged.Handle(context.Background(), protocol.Request{Method: "tools/call", Params: params}, nil)
		if !reflect.DeepEqual(withoutMeta(want), withoutMeta(got)) || !reflect.DeepEqual(wantErr, gotErr) {
			t.Fatalf("enabling the outcome log changed a result for %v:\n%#v %v\n%#v %v", params, want, wantErr, got, gotErr)
		}
	}
	raw, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range secrets {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatalf("outcome log contains request text %q:\n%s", secret, raw)
		}
	}
	data, err := decodeOutcomes(raw)
	if err != nil {
		t.Fatalf("%v\n%s", err, raw)
	}
	sum := 0
	for _, tool := range data.Tools {
		sum += tool.Calls
	}
	search := data.Tools["corvint.docs_search"]
	if sum != len(calls) || search == nil || search.Calls != 3 || search.Outcomes["answered"] != 1 || search.Outcomes["refused"] != 1 || search.Outcomes["answered"]+search.Outcomes["no-match"]+search.Outcomes["refused"] != 3 {
		t.Fatalf("per-tool counts do not sum to %d:\n%s", len(calls), raw)
	}
	if data.Tools[unlistedTool] == nil || data.Tools[unlistedTool].Outcomes["refused"] != 1 || data.ArtifactSHA256 != logged.registry.ArtifactSHA256() || data.WindowStart == "" || data.WindowEnd == "" {
		t.Fatalf("unlisted tool, corpus identity or window missing:\n%s", raw)
	}
	// A restart continues the same artifact's log; a foreign artifact is refused.
	reopened := outcomeHandler(t, root, log)
	reopened.Handle(context.Background(), protocol.Request{Method: "tools/call", Params: calls[0]}, nil)
	if raw, _ = os.ReadFile(log); !strings.Contains(string(raw), `"calls": 4`) {
		t.Fatalf("restart did not continue the log:\n%s", raw)
	}
	if _, err := openOutcomeLog(log, "sha256:other", nil, io.Discard); err == nil {
		t.Fatal("a foreign artifact's outcome log was continued")
	}
}

func TestDCPV1286OutcomeSummaryFloor(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "outcomes.json")
	l, err := openOutcomeLog(log, "sha256:corpus", []string{"corvint.docs_search", "corvint.docs_get"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		outcome := "answered"
		if i >= 8 {
			outcome = "no-match"
		}
		l.record("corvint.docs_search", outcome, 0)
	}
	floor := func(body string) string {
		path := filepath.Join(dir, "floor.json")
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	summary := func(args ...string) (int, string) {
		var out bytes.Buffer
		code := runOutcomeSummary(append([]string{"--outcome-summary"}, args...), &out, io.Discard)
		return code, out.String()
	}
	if code, out := summary(log); code != 0 || !strings.Contains(out, `"answered_share": 0.8`) || !strings.Contains(out, `"calls": 10`) {
		t.Fatalf("summary: %d %s", code, out)
	}
	if code, out := summary(log, "--floor", floor(`{"answered_share":0.85,"tolerance":0.1}`)); code != 0 {
		t.Fatalf("share within tolerance failed: %d %s", code, out)
	}
	if code, out := summary(log, "--floor", floor(`{"answered_share":0.95,"tolerance":0.1}`)); code != 1 || !strings.Contains(out, `"overall"`) {
		t.Fatalf("share below floor minus tolerance passed: %d %s", code, out)
	}
	if code, out := summary(log, "--floor", floor(`{"answered_share":0.5,"tolerance":0,"tools":{"corvint.docs_get":0.5}}`)); code != 1 || !strings.Contains(out, `"corvint.docs_get"`) {
		t.Fatalf("a floored tool without calls passed: %d %s", code, out)
	}
	for _, bad := range []string{`{"answered_share":2,"tolerance":0}`, `{"answered_share":0.5,"tolerance":0,"extra":1}`, `not json`} {
		if code, _ := summary(log, "--floor", floor(bad)); code != 2 {
			t.Fatalf("malformed floor %s accepted: %d", bad, code)
		}
	}
	tampered := filepath.Join(dir, "tampered.json")
	raw, _ := os.ReadFile(log)
	var doc map[string]any
	_ = json.Unmarshal(raw, &doc)
	doc["tools"].(map[string]any)["corvint.docs_search"].(map[string]any)["calls"] = 11
	raw, _ = json.Marshal(doc)
	_ = os.WriteFile(tampered, raw, 0600)
	if code, _ := summary(tampered); code != 2 {
		t.Fatalf("inconsistent counters accepted: %d", code)
	}
	if code, _ := summary(filepath.Join(dir, "absent.json")); code != 2 {
		t.Fatalf("missing log accepted: %d", code)
	}
}
