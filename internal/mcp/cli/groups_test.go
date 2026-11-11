package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
)

// groupsFixture is the GitHub #2011 workspace: an application root carrying a
// documented Go package and a built corpus artifact, and the e2e test root
// whose branch `integration` is followed by a detached linked worktree.
func groupsFixture(t *testing.T) (app, e2e string) {
	t.Helper()
	app, _, e2e = followRootsFixture(t)
	files := map[string]string{
		"cache/demux.go": "package cache\n\nfunc Split(key string) string { return key }\n",
		"owner.md":       "# Cache\nDelivery status: experimental\n\n## Agent digest\n- Claim: splits keys\n- Blocked on: behavior validation\n\n## Requirements\nNever promote drafts.\n",
		"source.md":      "# Evidence\n\nOriginal source.\n",
	}
	for name, text := range files {
		path := filepath.Join(app, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	gitTest(t, app, "add", "-A")
	gitTest(t, app, "-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "groups")
	return app, e2e
}

func toolNames(tools []map[string]any) []string {
	var names []string
	for _, tool := range tools {
		names = append(names, tool["name"].(string))
	}
	return names
}

func resultOf(t *testing.T, response map[string]any) map[string]any {
	t.Helper()
	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result: %#v", response)
	}
	return result
}

// MMR-V0-012 as narrowed by MTT-V1-005: --tool-profile takes one code
// profile, or a comma list of at most one code profile and the distinct
// groups test-evidence and affected; any other value is the bare MCPV0-026
// refusal before any MCP byte.
func TestMMRV0012ToolProfileList(t *testing.T) {
	for value, want := range map[string][]string{
		"flows":                        {"flows"},
		"test-evidence":                {"", "test-evidence"},
		"flows,test-evidence,affected": {"flows", "test-evidence", "affected"},
		"affected,flows":               {"flows", "affected"},
	} {
		code, groups, ok := splitToolProfile(value)
		if !ok || code != want[0] || !slices.Equal(groups, want[1:]) && len(groups)+len(want[1:]) != 0 {
			t.Fatalf("%q: code=%q groups=%q ok=%v", value, code, groups, ok)
		}
	}
	root := filepath.Clean(t.TempDir())
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"", ",", "flows,", ",flows", "flows,flows", "Flows", "flows,task-review", "flows,unknown", "flows ,test-evidence"} {
		if _, _, ok := splitToolProfile(value); ok {
			t.Fatalf("%q accepted", value)
		}
		var stdout, stderr bytes.Buffer
		arguments := []string{"--root", root, "--tool-profile", value}
		if exit := run(context.Background(), arguments, bytes.NewReader(nil), &stdout, &stderr); exit != 2 || stdout.Len() != 0 || stderr.String() != "corvint-mcp: invalid arguments\n" {
			t.Fatalf("%q exit=%d stdout=%q stderr=%q", arguments, exit, stdout.String(), stderr.String())
		}
	}
	// Code tools stay served beside the groups, and no group duplicates a name.
	tools := toolNames(listedTools(t, []string{"--root", root, "--tool-profile", "flows,test-evidence"}))
	code := toolNames(listedTools(t, []string{"--root", root, "--tool-profile", "flows"}))
	want := slices.Sorted(slices.Values(append(slices.Clone(code), "corvint.test_evidence")))
	if !slices.Equal(tools, want) {
		t.Fatalf("tools=%q want=%q", tools, want)
	}
}

// MMR-V0-012: the groups' tools/list is pinned by a byte golden beside the
// unchanged code-profile goldens.
func TestMMRV0012GroupsToolsListGolden(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	list := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}}}` + "\n"
	var stdout, stderr bytes.Buffer
	arguments := []string{"--root", root, "--tool-profile", "test-evidence"}
	if exit := run(context.Background(), arguments, strings.NewReader(list), &stdout, &stderr); exit != 0 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
	}
	checkToolsListGolden(t, "tools-list-groups.golden.jsonl", stdout.Bytes())
}

// MMR-V0-013 (GitHub #2011 acceptance): one server lists the union of the
// selected groups across two roots, one following a ref; per-root group
// calls are routed by alias and bound under that root's followed ref.
func TestMMRV0013UnionAcrossFollowedRoots(t *testing.T) {
	skipUnqualifiedStatus(t)
	app, e2e := groupsFixture(t)
	arguments := []string{"--root", "app=" + app, "--root", "e2e=" + e2e, "--follow-ref", "e2e=integration",
		"--tool-profile", "test-evidence"}
	responses := multiRootSession(t, arguments,
		map[string]any{"id": 1, "method": "tools/list"},
		toolCall(2, "corvint.test_evidence", map[string]any{"repository": "e2e", "path": "source.md"}),
		toolCall(3, "corvint.test_evidence", map[string]any{"repository": "app", "path": "source.md"}),
		toolCall(6, "corvint.test_evidence", map[string]any{"repository": "missing", "path": "source.md"}),
		toolCall(7, "corvint.test_evidence", map[string]any{"path": "source.md"}),
	)
	code := toolNames(listedTools(t, []string{"--root", "app=" + app, "--root", "e2e=" + e2e}))
	var names []string
	for _, raw := range resultOf(t, responses["1"])["tools"].([]any) {
		tool := raw.(map[string]any)
		name := tool["name"].(string)
		names = append(names, name)
		if name == bridge.ToolHelp {
			continue
		}
		properties := tool["inputSchema"].(map[string]any)["properties"].(map[string]any)
		enum := properties["repository"].(map[string]any)["enum"]
		if !reflect.DeepEqual(enum, []any{"app", "e2e"}) || !strings.Contains(tool["description"].(string), "naming repository e2e") {
			t.Fatalf("per-root tool %s: %#v", name, tool)
		}
	}
	for _, name := range append(code, "corvint.test_evidence") {
		if !slices.Contains(names, name) {
			t.Fatalf("tools/list misses %s: %q", name, names)
		}
	}
	if len(names) != len(slices.Compact(slices.Clone(names))) {
		t.Fatalf("duplicate tool names: %q", names)
	}
	for _, id := range []string{"2", "3"} {
		if result := resultOf(t, responses[id]); result["isError"] != false {
			t.Fatalf("call %s: %#v", id, result)
		}
	}
	if blocks := followBlocks(t, resultOf(t, responses["2"])); len(blocks) != 1 || blocks[0]["repository"] != "e2e" || blocks[0]["ref"] != "integration" {
		t.Fatalf("e2e group call is not bound to the followed ref: %#v", blocks)
	}
	if blocks := followBlocks(t, resultOf(t, responses["3"])); len(blocks) != 0 {
		t.Fatalf("app call carries a follow-ref block: %#v", blocks)
	}
	for _, id := range []string{"6", "7"} {
		if responses[id]["error"] == nil {
			t.Fatalf("call %s without a declared alias accepted: %#v", id, responses[id])
		}
	}
}

// pipeSession drives one in-process server one request at a time, so a test
// can change files between calls.
type pipeSession struct {
	t       *testing.T
	input   *io.PipeWriter
	decoder *json.Decoder
	done    chan int
	stderr  *bytes.Buffer
}

func startPipeSession(t *testing.T, arguments []string) *pipeSession {
	t.Helper()
	serverInput, clientInput := io.Pipe()
	clientOutput, serverOutput := io.Pipe()
	session := &pipeSession{t: t, input: clientInput, decoder: json.NewDecoder(clientOutput), done: make(chan int, 1), stderr: &bytes.Buffer{}}
	go func() {
		session.done <- run(context.Background(), arguments, serverInput, serverOutput, session.stderr)
		_ = serverOutput.Close()
	}()
	return session
}

func (session *pipeSession) send(request map[string]any) map[string]any {
	session.t.Helper()
	request["jsonrpc"] = "2.0"
	request["params"].(map[string]any)["_meta"] = map[string]any{"io.modelcontextprotocol/protocolVersion": "2026-07-28", "io.modelcontextprotocol/clientCapabilities": map[string]any{}}
	raw, err := json.Marshal(request)
	if err != nil {
		session.t.Fatal(err)
	}
	if _, err := session.input.Write(append(raw, '\n')); err != nil {
		session.t.Fatal(err)
	}
	var response map[string]any
	if err := session.decoder.Decode(&response); err != nil {
		session.t.Fatalf("%v (stderr %q)", err, session.stderr.String())
	}
	return response
}

func (session *pipeSession) close() int {
	_ = session.input.Close()
	return <-session.done
}

// MMR-V0-015: --workspace FILE takes the corvint-workspace/0 roots once, as
// --root ALIAS=ROOT in alias order; a declared revision is noted, never
// followed, and --follow-ref follows a ref explicitly.
func TestMMRV0015WorkspaceFile(t *testing.T) {
	skipUnqualifiedStatus(t)
	app, e2e := groupsFixture(t)
	file := filepath.Join(t.TempDir(), "workspace.json")
	declaration := fmt.Sprintf(`{"profile":"corvint-workspace/0","repos":{"e2e":{"root":%q,"revision":"integration"},"app":{"root":%q}},"manifest_repo":"e2e"}`, filepath.ToSlash(e2e), filepath.ToSlash(app))
	if err := os.WriteFile(file, []byte(declaration), 0o600); err != nil {
		t.Fatal(err)
	}
	list := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}}}` + "\n"
	run2 := func(arguments ...string) (string, string) {
		var stdout, stderr bytes.Buffer
		if exit := run(context.Background(), arguments, strings.NewReader(list), &stdout, &stderr); exit != 0 {
			t.Fatalf("%q exit=%d stderr=%q", arguments, exit, stderr.String())
		}
		return stdout.String(), stderr.String()
	}
	declared, note := run2("--workspace", file, "--tool-profile", "test-evidence")
	explicit, _ := run2("--root", "app="+app, "--root", "e2e="+e2e, "--tool-profile", "test-evidence")
	if declared != explicit || note != "corvint-mcp: note: workspace alias e2e declares revision \"integration\", which is not followed; --follow-ref e2e=REF follows a ref\n" {
		t.Fatalf("--workspace differs from its --root expansion:\n%s\n%s\nstderr=%q", declared, explicit, note)
	}
	followed, _ := run2("--workspace", file, "--follow-ref", "e2e=integration", "--tool-profile", "test-evidence")
	if !strings.Contains(followed, "naming repository e2e") || strings.Contains(declared, "naming repository e2e") {
		t.Fatalf("--follow-ref beside --workspace: %s", followed)
	}
	for _, refusal := range []struct {
		arguments []string
		reason    string
	}{
		{[]string{"--workspace", file, "--root", "app=" + app}, workspaceReasonWithRoots},
		{[]string{"--workspace", file, "--version"}, workspaceReasonWithRoots},
		{[]string{"--workspace", file, "--workspace", file}, workspaceReasonRepeated},
		{[]string{"--workspace", filepath.Join(t.TempDir(), "absent.json")}, "--workspace: "},
	} {
		var stdout, stderr bytes.Buffer
		if exit := run(context.Background(), refusal.arguments, bytes.NewReader(nil), &stdout, &stderr); exit != 2 || stdout.Len() != 0 ||
			!strings.HasPrefix(stderr.String(), "corvint-mcp: invalid arguments: "+refusal.reason) {
			t.Fatalf("%q exit=%d stdout=%q stderr=%q", refusal.arguments, exit, stdout.String(), stderr.String())
		}
	}
}
