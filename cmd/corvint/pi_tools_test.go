package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func piToolTest(t *testing.T, operation string, input map[string]any) map[string]any {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"hostVersion": piHostVersion, "input": input})
	return piToolResult(context.Background(), operation, strings.NewReader(string(raw)))
}

func TestPiToolContextExpansion(t *testing.T) {
	t.Run("AHI-025 native context packet expands immutable source without files", func(t *testing.T) {
		root := queryCLIRepository(t)
		t.Chdir(root)
		before := repositoryBytesDigest(t, root)
		result := piToolTest(t, "context", map[string]any{"task": "internal/parser/token.go"})
		if result["ok"] != true {
			t.Fatalf("context: %v", result)
		}
		packet := result["packet"].(map[string]any)
		var source map[string]any
		if err := json.Unmarshal([]byte(packet["json"].(string)), &source); err != nil {
			t.Fatal(err)
		}
		rows := source["results"].([]any)
		if len(rows) == 0 {
			t.Fatal("missing evidence")
		}
		input := map[string]any{"packet": packet["json"], "packetSha256": packet["sha256"], "commit": packet["commit"], "result": 0, "evidence": 0, "lines": "1:1"}
		expanded := piToolTest(t, "expand", input)
		if expanded["ok"] != true || !strings.Contains(expanded["context"].(string), "selector_complete\":true") {
			_, reason := executeSourceViewBytes(context.Background(), sourceViewOptions{root: root, commit: packet["commit"].(string), digest: packet["sha256"].(string), result: 0, evidence: 0, lines: "1:1", maxBytes: 2048}, map[string]any{}, []byte(packet["json"].(string)))
			t.Fatalf("expand: %v; reason=%v; packet=%s", expanded, reason, packet["json"])
		}
		if repositoryBytesDigest(t, root) != before {
			t.Fatal("read tools mutated repository")
		}
		input["packetSha256"] = strings.Repeat("0", 64)
		if result := piToolTest(t, "expand", input); result["fault"] != "source-unavailable" {
			t.Fatalf("forged digest: %v", result)
		}
		input["packetSha256"] = packet["sha256"]
		input["result"] = -1
		if result := piToolTest(t, "expand", input); result["fault"] != "invalid-input" {
			t.Fatalf("negative index: %v", result)
		}
	})
}

func TestPiToolRecord(t *testing.T) {
	t.Run("AHI-025 explicit record uses core admission and does not echo task", func(t *testing.T) {
		root := newRecordFixtureAt(t, filepath.Join(t.TempDir(), "repository"))
		t.Chdir(root)
		input := map[string]any{"task": "explicit Pi fixture outcome", "changedPaths": []string{"internal/example/value.go"}, "verification": []string{"go test ./..."}, "outcome": "passed"}
		result := piToolTest(t, "record", input)
		if result["ok"] != true || result["mutation"] != "recorded" {
			t.Fatalf("record: %v", result)
		}
		if strings.Contains(result["context"].(string), input["task"].(string)) {
			t.Fatal("task echoed")
		}
		if err := os.WriteFile(filepath.Join(root, "internal/example/value.go"), []byte("dirty"), 0600); err != nil {
			t.Fatal(err)
		}
		if result := piToolTest(t, "record", input); result["fault"] != "record-unavailable" {
			t.Fatalf("dirty record: %v", result)
		}
	})
}

func TestPiToolClosedInput(t *testing.T) {
	t.Run("AHI-025 explicit operations reject caller root identity and duplicate keys", func(t *testing.T) {
		for _, raw := range []string{`null`, `{"hostVersion":"0.85.1","input":{"task":"x"},"root":"/"}`, `{"hostVersion":"0.85.1","input":{"task":"x","task":"y"}}`, `{"hostVersion":"0.85.1","input":{"task":null}}`} {
			if result := piToolResult(context.Background(), "context", strings.NewReader(raw)); result["fault"] != "invalid-input" {
				t.Fatalf("accepted %s: %v", raw, result)
			}
		}
	})
}
