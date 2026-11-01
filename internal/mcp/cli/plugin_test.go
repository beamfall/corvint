package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// MCPV0-951: serverInfo.version and --version carry the calling binary's
// build identity; the default stays serverVersion.
func TestMCPV0951ServerInfoCarriesTheBuildVersion(t *testing.T) {
	stamped := "1.0.0-rc.3 (build 774, commit " + strings.Repeat("a", 40) + ")"
	var stdout, stderr bytes.Buffer
	if exit := runVersioned(context.Background(), stamped, []string{"--version"}, bytes.NewReader(nil), &stdout, &stderr); exit != 0 || stdout.String() != "corvint-mcp "+stamped+"\n" {
		t.Fatalf("--version exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
	root := filepath.Clean(t.TempDir())
	gitInit(t, root)
	input := `{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}` + "\n"
	for _, version := range []string{stamped, serverVersion} {
		stdout.Reset()
		if exit := runVersioned(context.Background(), version, []string{"--root", root, "--protocol-version", "2025-11-25"}, strings.NewReader(input), &stdout, &stderr); exit != 0 {
			t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
		}
		if !strings.Contains(stdout.String(), `"serverInfo":{"description":"Local read-only Corvint context and repository evidence server.","name":"corvint-mcp","version":"`+version+`"}`) {
			t.Fatalf("initialize for %q: %s", version, stdout.String())
		}
	}
}

// MCPV0-951: the plugin declaration is read once, bounded, and kept malformed
// rather than guessed at; no declaration is nil.
func TestMCPV0951DeclaredPluginIsBounded(t *testing.T) {
	lookup := func(values map[string]string) func(string) (string, bool) {
		return func(key string) (string, bool) { value, ok := values[key]; return value, ok }
	}
	if declaredPlugin(lookup(nil)) != nil {
		t.Fatal("no declaration should be nil")
	}
	good := declaredPlugin(lookup(map[string]string{pluginVersionVariable: "claude-code/0.3.8", pluginToolsVariable: "corvint.help,corvint.status"}))
	if good == nil || good.malformed || good.version != "claude-code/0.3.8" || !reflect.DeepEqual(good.tools, []string{"corvint.help", "corvint.status"}) {
		t.Fatalf("good=%#v", good)
	}
	for _, values := range []map[string]string{
		{pluginVersionVariable: "0.3.8", pluginToolsVariable: "corvint.help"},
		{pluginVersionVariable: "claude-code/0.3.8"},
		{pluginVersionVariable: "claude-code/0.3.8", pluginToolsVariable: "corvint.help,,corvint.status"},
		{pluginVersionVariable: "claude-code/0.3.8", pluginToolsVariable: "other.tool"},
		{pluginVersionVariable: "claude-code/0.3.8\n", pluginToolsVariable: "corvint.help"},
		{pluginVersionVariable: "claude-code/0.3.8", pluginToolsVariable: strings.Repeat("corvint.help,", maxPluginTools) + "corvint.help"},
	} {
		if declaration := declaredPlugin(lookup(values)); declaration == nil || !declaration.malformed || declaration.version != "" {
			t.Fatalf("%q => %#v", values, declaration)
		}
	}
}

// MCPV0-951: corvint.status reports plugin/binary compatibility when the
// launcher declares a plugin: compatible when every expected tool is listed,
// else the missing tools; the status receipt is unchanged and an undeclared
// launch adds nothing.
func TestMCPV0951StatusReportsPluginCompatibility(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	gitInit(t, root)
	status := map[string]any{"method": "tools/call", "params": map[string]any{"name": "corvint.status", "arguments": map[string]any{}}}
	session := func(tools string, declare bool) (map[string]any, map[string]any) {
		t.Helper()
		if declare {
			t.Setenv(pluginVersionVariable, "claude-code/0.3.8")
			t.Setenv(pluginToolsVariable, tools)
		} else {
			if err := os.Unsetenv(pluginVersionVariable); err != nil {
				t.Fatal(err)
			}
		}
		exit, responses, stderr := legacySession(t, []string{"--root", root}, status)
		if exit != 0 {
			t.Fatalf("exit=%d stderr=%q", exit, stderr)
		}
		result, _ := responses[1]["result"].(map[string]any)
		meta, _ := result["_meta"].(map[string]any)
		compatibility, _ := meta[pluginCompatibilityMeta].(map[string]any)
		return result, compatibility
	}
	result, compatibility := session("corvint.help,corvint.status", true)
	if compatibility["compatible"] != true || !reflect.DeepEqual(compatibility["missingTools"], []any{}) || compatibility["plugin"] != "claude-code/0.3.8" || compatibility["server"] != "corvint-mcp "+serverVersion || compatibility["profile"] != pluginCompatibilityProfile {
		t.Fatalf("compatible=%#v", compatibility)
	}
	content, _ := result["content"].([]any)
	if last, _ := content[len(content)-1].(map[string]any); last["text"] != "corvint plugin compatibility: plugin claude-code/0.3.8 with corvint-mcp "+serverVersion+": compatible" {
		t.Fatalf("text=%#v", content)
	}
	declaredStructured := result["structuredContent"]
	_, compatibility = session("corvint.help,corvint.test_validity,corvint.docs_draft", true)
	if compatibility["compatible"] != false || !reflect.DeepEqual(compatibility["missingTools"], []any{"corvint.docs_draft", "corvint.test_validity"}) {
		t.Fatalf("incompatible=%#v", compatibility)
	}
	_, compatibility = session("not-a-tool", true)
	if compatibility["compatible"] != false || compatibility["reason"] != "malformed-declaration" {
		t.Fatalf("malformed=%#v", compatibility)
	}
	result, compatibility = session("", false)
	if compatibility != nil || len(result["content"].([]any)) != 1 {
		t.Fatalf("undeclared result=%#v", result)
	}
	if !reflect.DeepEqual(result["structuredContent"], declaredStructured) {
		t.Fatal("the declaration changed the status receipt")
	}
}
