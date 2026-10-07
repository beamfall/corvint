package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// multiRootSession runs one corvint-mcp process over the given argv and
// requests and returns each response by its JSON-RPC id.
func multiRootSession(t *testing.T, arguments []string, requests ...map[string]any) map[string]map[string]any {
	t.Helper()
	var input bytes.Buffer
	for _, request := range requests {
		request["jsonrpc"] = "2.0"
		params, _ := request["params"].(map[string]any)
		if params == nil {
			params = map[string]any{}
			request["params"] = params
		}
		if _, present := params["_meta"]; !present {
			params["_meta"] = map[string]any{
				"io.modelcontextprotocol/protocolVersion":    "2026-07-28",
				"io.modelcontextprotocol/clientCapabilities": map[string]any{},
			}
		}
		raw, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		input.Write(append(raw, '\n'))
	}
	var stdout, stderr bytes.Buffer
	if exit := run(context.Background(), arguments, &input, &stdout, &stderr); exit != 0 || stderr.Len() != 0 {
		t.Fatalf("%q exit=%d stderr=%q", arguments, exit, stderr.String())
	}
	responses := map[string]map[string]any{}
	for _, line := range bytes.Split(bytes.TrimSpace(stdout.Bytes()), []byte("\n")) {
		var response map[string]any
		if err := json.Unmarshal(line, &response); err != nil {
			t.Fatalf("%v: %s", err, line)
		}
		responses[fmt.Sprint(response["id"])] = response
	}
	if len(responses) != len(requests) {
		t.Fatalf("responses=%d requests=%d", len(responses), len(requests))
	}
	return responses
}

func toolCall(id int, name string, arguments map[string]any) map[string]any {
	return map[string]any{"id": id, "method": "tools/call", "params": map[string]any{"name": name, "arguments": arguments}}
}

func headRevision(t *testing.T, root string) string {
	t.Helper()
	output, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(output))
}

func statusRevision(t *testing.T, response map[string]any) string {
	t.Helper()
	result, _ := response["result"].(map[string]any)
	structured, _ := result["structuredContent"].(map[string]any)
	repository, _ := structured["repository"].(map[string]any)
	revision, _ := repository["commitRevision"].(string)
	if result["isError"] != false || revision == "" {
		t.Fatalf("status response=%#v", response)
	}
	return revision
}

func skipUnqualifiedStatus(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("the repository status probe is qualified only on Darwin and Linux")
	}
}

// MMR-V0-001: one plain --root stays the MCPV0-001 single-root server, byte
// for byte; one aliased root is multi-root mode with the alias enum.
func TestMMRV0001SingleRootBytesUnchanged(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	list := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}}}` + "\n"
	var stdout, stderr bytes.Buffer
	if exit := run(context.Background(), []string{"--root", root}, strings.NewReader(list), &stdout, &stderr); exit != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
	}
	want, err := os.ReadFile(filepath.Join("testdata", "tools-list-default.golden.jsonl"))
	if err != nil || !bytes.Equal(stdout.Bytes(), want) {
		t.Fatalf("single-root tools/list changed (%v):\n%s", err, stdout.Bytes())
	}
	if bytes.Contains(stdout.Bytes(), []byte(`"repository":{"enum"`)) {
		t.Fatal("single-root schema gained the repository selector")
	}
}

// MMR-V0-004: every repository-scoped tool requires the closed alias enum;
// corvint.status admits it optionally. Schemas are otherwise the single-root
// schemas.
func TestMMRV0004RepositoryEnumOnEveryTool(t *testing.T) {
	first, second := filepath.Clean(t.TempDir()), filepath.Clean(t.TempDir())
	for _, root := range []string{first, second} {
		if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, profile := range [][]string{nil, {"--tool-profile", "task-review"}, {"--tool-profile", "flows"}} {
		arguments := append([]string{"--root", "web=" + second, "--root", "core=" + first}, profile...)
		single := append([]string{"--root", first}, profile...)
		multiTools := listedTools(t, arguments)
		singleTools := listedTools(t, single)
		if len(multiTools) != len(singleTools) {
			t.Fatalf("%q tools=%d single=%d", profile, len(multiTools), len(singleTools))
		}
		for index, tool := range multiTools {
			schema := tool["inputSchema"].(map[string]any)
			properties := schema["properties"].(map[string]any)
			if !reflect.DeepEqual(properties["repository"], map[string]any{"type": "string", "enum": []any{"core", "web"}}) {
				t.Fatalf("%s repository=%#v", tool["name"], properties["repository"])
			}
			required, _ := schema["required"].([]any)
			if wantRequired := tool["name"] != "corvint.status"; slices.Contains(required, any("repository")) != wantRequired {
				t.Fatalf("%s required=%v", tool["name"], required)
			}
			delete(properties, "repository")
			schema["required"] = slices.DeleteFunc(required, func(value any) bool { return value == "repository" })
			want := singleTools[index]
			if wantRequired, _ := want["inputSchema"].(map[string]any)["required"].([]any); len(wantRequired) == 0 && len(schema["required"].([]any)) == 0 {
				schema["required"] = want["inputSchema"].(map[string]any)["required"]
			}
			if !reflect.DeepEqual(tool, want) {
				t.Fatalf("%s differs beyond the repository selector:\n%#v\n%#v", tool["name"], tool, want)
			}
		}
	}
}

func listedTools(t *testing.T, arguments []string) []map[string]any {
	t.Helper()
	response := multiRootSession(t, arguments, map[string]any{"id": 1, "method": "tools/list"})["1"]
	var tools []map[string]any
	for _, tool := range response["result"].(map[string]any)["tools"].([]any) {
		tools = append(tools, tool.(map[string]any))
	}
	return tools
}

// MMR-V0-002 and MMR-V0-003: argv errors, duplicate aliases, the same
// directory under two aliases or spellings, and more roots than the cap all
// refuse startup before any MCP byte.
func TestMMRV0003StartupRefusals(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	other := filepath.Clean(t.TempDir())
	if err := os.Mkdir(filepath.Join(other, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	tooMany := []string{}
	for index := 0; index <= maxRoots; index++ {
		tooMany = append(tooMany, "--root", fmt.Sprintf("r%d=%s", index, root))
	}
	for _, test := range []struct {
		name      string
		arguments []string
		stderr    string
	}{
		{"duplicate alias", []string{"--root", "core=" + root, "--root", "core=" + other}, "invalid arguments"},
		{"duplicate root", []string{"--root", "core=" + root, "--root", "web=" + root}, "invalid arguments"},
		{"duplicate root through a symlink", []string{"--root", "core=" + root, "--root", "web=" + link}, "invalid arguments"},
		{"more roots than the cap", tooMany, "invalid arguments"},
		{"plain and aliased", []string{"--root", root, "--root", "web=" + other}, "invalid arguments"},
		{"relative aliased root", []string{"--root", "core=relative/path"}, "repository unavailable"},
		{"one unavailable root", []string{"--root", "core=" + root, "--root", "web=/nonexistent"}, "repository unavailable"},
		{"invalid alias is a path", []string{"--root", "Core=" + root}, "repository unavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if exit := run(context.Background(), test.arguments, bytes.NewReader(nil), &stdout, &stderr); exit != 2 || stdout.Len() != 0 ||
				stderr.String() != "corvint-mcp: "+test.stderr+"\n" {
				t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
			}
		})
	}
}

// MMR-V0-005 and MMR-V0-007: each call reaches only the repository its alias
// names, results bind to that root's own revision, interleaved calls across
// roots stay bound, and a missing, unknown, or path-valued alias is refused as
// Invalid params.
func TestMMRV0005AliasRoutingAndRefusal(t *testing.T) {
	skipUnqualifiedStatus(t)
	roots := map[string]string{
		"core": makeHostileRepository(t, "core"),
		"docs": makeHostileRepository(t, "docs"),
		"web":  makeHostileRepository(t, "web"),
	}
	if err := os.WriteFile(filepath.Join(roots["docs"], "README.md"), []byte("docs\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{{"add", "README.md"}, {"commit", "-q", "-m", "second"}} {
		if output, err := exec.Command("git", append([]string{"-C", roots["docs"]}, arguments...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", arguments, err, output)
		}
	}
	revisions := map[string]string{}
	for alias, root := range roots {
		revisions[alias] = headRevision(t, root)
	}
	if revisions["core"] == revisions["docs"] {
		t.Fatal("fixture revisions must differ")
	}
	arguments := []string{"--root", "web=" + roots["web"], "--root", "core=" + roots["core"], "--root", "docs=" + roots["docs"]}
	order := []string{"core", "docs", "web", "core", "web", "docs", "docs", "core"}
	requests := []map[string]any{}
	for index, alias := range order {
		requests = append(requests, toolCall(index, "corvint.status", map[string]any{"repository": alias}))
	}
	refused := []map[string]any{
		toolCall(100, "corvint.query", map[string]any{"task": "orientation"}),
		toolCall(101, "corvint.query", map[string]any{"task": "orientation", "repository": "missing"}),
		toolCall(102, "corvint.query", map[string]any{"task": "orientation", "repository": roots["core"]}),
		toolCall(103, "corvint.status", map[string]any{"repository": 1}),
		toolCall(104, "corvint.status", map[string]any{"repository": "CORE"}),
		toolCall(105, "corvint.impact", map[string]any{"paths": []any{"main.go"}}),
	}
	responses := multiRootSession(t, arguments, append(requests, refused...)...)
	for index, alias := range order {
		if got := statusRevision(t, responses[fmt.Sprint(index)]); got != revisions[alias] {
			t.Fatalf("call %d alias %s revision=%s want %s", index, alias, got, revisions[alias])
		}
	}
	for _, request := range refused {
		response := responses[fmt.Sprint(request["id"])]
		failure, _ := response["error"].(map[string]any)
		if response["result"] != nil || failure == nil || fmt.Sprint(failure["code"]) != "-32602" {
			t.Fatalf("request %v response=%#v", request["id"], response)
		}
	}
}

// MMR-V0-005: neither client roots nor any request field can add or switch a
// repository; the bridge's closed decode still refuses a root argument after
// the alias is removed.
func TestMMRV0005ClientRootsNeverSwitchRepository(t *testing.T) {
	skipUnqualifiedStatus(t)
	core, other := makeHostileRepository(t, "core"), makeHostileRepository(t, "other")
	arguments := []string{"--root", "core=" + core}
	withRoots := map[string]any{
		"io.modelcontextprotocol/protocolVersion": "2026-07-28",
		"io.modelcontextprotocol/clientCapabilities": map[string]any{
			"roots": map[string]any{"listChanged": true},
		},
		"roots": []any{map[string]any{"uri": "file://" + other, "name": "other"}},
	}
	responses := multiRootSession(t, arguments,
		map[string]any{"id": 1, "method": "tools/call", "params": map[string]any{
			"_meta": withRoots, "name": "corvint.status", "arguments": map[string]any{"repository": "core"},
		}},
		toolCall(2, "corvint.status", map[string]any{"repository": "core", "root": other}),
		toolCall(3, "corvint.status", map[string]any{"repository": "core", "roots": []any{"file://" + other}}),
		toolCall(4, "corvint.status", map[string]any{"repository": "other"}),
	)
	if got := statusRevision(t, responses["1"]); got != headRevision(t, core) {
		t.Fatalf("client roots switched the repository: %s", got)
	}
	for _, id := range []string{"2", "3", "4"} {
		failure, _ := responses[id]["error"].(map[string]any)
		if failure == nil || fmt.Sprint(failure["code"]) != "-32602" {
			t.Fatalf("request %s response=%#v", id, responses[id])
		}
	}
}

// MMR-V0-006 and MMR-V0-007: corvint.status without an alias reports every
// declared binding in alias order, each exactly the per-alias status result;
// a later commit in one root moves only that root's binding.
func TestMMRV0006StatusReportsEveryBinding(t *testing.T) {
	skipUnqualifiedStatus(t)
	core, web := makeHostileRepository(t, "core"), makeHostileRepository(t, "web")
	arguments := []string{"--root", "web=" + web, "--root", "core=" + core}
	all := func() []any {
		responses := multiRootSession(t, arguments,
			toolCall(1, "corvint.status", map[string]any{}),
			toolCall(2, "corvint.status", map[string]any{"repository": "core"}),
			toolCall(3, "corvint.status", map[string]any{"repository": "web"}),
		)
		result := responses["1"]["result"].(map[string]any)
		structured := result["structuredContent"].(map[string]any)
		if result["isError"] != false || structured["schema"] != multiRootStatusSchema || structured["tool"] != "corvint.status" || structured["mutates"] != false {
			t.Fatalf("status-all=%#v", result)
		}
		text := result["content"].([]any)[0].(map[string]any)["text"].(string)
		if !strings.Contains(text, `"schema":"`+multiRootStatusSchema+`"`) {
			t.Fatalf("status-all text=%s", text)
		}
		entries := structured["repositories"].([]any)
		for index, alias := range []string{"core", "web"} {
			entry := entries[index].(map[string]any)
			single := responses[fmt.Sprint(index+2)]["result"].(map[string]any)["structuredContent"]
			if entry["repository"] != alias || !reflect.DeepEqual(entry["result"], single) {
				t.Fatalf("entry %d=%#v single=%#v", index, entry, single)
			}
		}
		return entries
	}
	revision := func(entry any) string {
		return entry.(map[string]any)["result"].(map[string]any)["repository"].(map[string]any)["commitRevision"].(string)
	}
	before := all()
	if revision(before[0]) != headRevision(t, core) || revision(before[1]) != headRevision(t, web) {
		t.Fatalf("bindings=%#v", before)
	}
	if output, err := exec.Command("git", "-C", web, "commit", "-q", "--allow-empty", "-m", "next").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}
	after := all()
	if revision(after[0]) != revision(before[0]) || revision(after[1]) == revision(before[1]) || revision(after[1]) != headRevision(t, web) {
		t.Fatalf("per-root binding before=%#v after=%#v", before, after)
	}
}
