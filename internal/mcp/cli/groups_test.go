package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
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
	writeCorpusArtifact(t, app, "corpus.json")
	return app, e2e
}

func writeCorpusArtifact(t *testing.T, root, name string) {
	t.Helper()
	manifest, err := doccorpus.Inventory(context.Background(), root, headRevision(t, root), "source.md", "2026-09-19T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := doccorpus.Build(context.Background(), root, manifest)
	if err != nil {
		t.Fatal(err)
	}
	data, err := doccorpus.Encode(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, name), data, 0o600); err != nil {
		t.Fatal(err)
	}
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

// MMR-V0-012: --tool-profile takes one code profile, or a comma list of at
// most one code profile and distinct tool groups; anything else is the bare
// MCPV0-026 refusal before any MCP byte.
func TestMMRV0012ToolProfileList(t *testing.T) {
	for value, want := range map[string][]string{
		"flows":                     {"flows"},
		"docs":                      {"", "docs"},
		"task-review,docs,corpus":   {"task-review", "docs", "corpus"},
		"test-validity,flows":       {"flows", "test-validity"},
		"corpus,test-validity,docs": {"", "corpus", "test-validity", "docs"},
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
	for _, value := range []string{"", ",", "docs,", ",docs", "docs,docs", "Docs", "flows,task-review", "docs,unknown", "docs ,corpus"} {
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
	tools := toolNames(listedTools(t, []string{"--root", root, "--tool-profile", "task-review,test-validity,docs"}))
	code := toolNames(listedTools(t, []string{"--root", root, "--tool-profile", "task-review"}))
	want := slices.Sorted(slices.Values(append(slices.Clone(code), "corvint.docs_consume", "corvint.docs_draft", "corvint.test_validity")))
	if !slices.Equal(tools, want) {
		t.Fatalf("tools=%q want=%q", tools, want)
	}
}

// MMR-V0-012 and MMR-V0-014: the groups' tools/list is pinned by a byte
// golden, added deliberately beside the unchanged code-profile goldens; a
// missing corpus artifact lists every corpus tool and is noted on stderr.
func TestMMRV0012GroupsToolsListGolden(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	list := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}}}` + "\n"
	var stdout, stderr bytes.Buffer
	arguments := []string{"--root", root, "--tool-profile", "test-validity,docs,corpus", "--corpus-artifact", "corpus.json"}
	if exit := run(context.Background(), arguments, strings.NewReader(list), &stdout, &stderr); exit != 0 || !strings.Contains(stderr.String(), "corvint-mcp: note: --corpus-artifact unavailable (") {
		t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
	}
	want, err := os.ReadFile(filepath.Join("testdata", "tools-list-groups.golden.jsonl"))
	if err != nil || !bytes.Equal(stdout.Bytes(), want) {
		t.Fatalf("tools-list-groups.golden.jsonl mismatch (%v):\n%s", err, stdout.Bytes())
	}
}

// MMR-V0-013 (GitHub #2011 acceptance): one server takes the workspace once
// and lists the union of the selected groups across two roots, one following
// a ref; per-root group calls are routed by alias and bound under that root's
// followed ref, while the workspace-level corpus tools are neither.
func TestMMRV0013UnionAcrossFollowedRoots(t *testing.T) {
	skipUnqualifiedStatus(t)
	app, e2e := groupsFixture(t)
	arguments := []string{"--root", "app=" + app, "--root", "e2e=" + e2e, "--follow-ref", "e2e=integration",
		"--tool-profile", "task-review,test-validity,docs,corpus", "--corpus-artifact", "app=corpus.json"}
	responses := multiRootSession(t, arguments,
		map[string]any{"id": 1, "method": "tools/list"},
		toolCall(2, "corvint.test_validity", map[string]any{"repository": "e2e"}),
		toolCall(3, "corvint.test_validity", map[string]any{"repository": "app"}),
		toolCall(4, "corvint.docs_draft", map[string]any{"repository": "app", "source": "owner.md", "package": "cache"}),
		toolCall(5, "corvint.docs_search", map[string]any{"query": "Original"}),
		toolCall(6, "corvint.test_validity", map[string]any{"repository": "missing"}),
		toolCall(7, "corvint.test_validity", map[string]any{}),
	)
	code := toolNames(listedTools(t, []string{"--root", "app=" + app, "--root", "e2e=" + e2e, "--tool-profile", "task-review"}))
	var names []string
	for _, raw := range resultOf(t, responses["1"])["tools"].([]any) {
		tool := raw.(map[string]any)
		name := tool["name"].(string)
		names = append(names, name)
		description := tool["description"].(string)
		properties := tool["inputSchema"].(map[string]any)["properties"].(map[string]any)
		switch {
		case strings.HasPrefix(name, "corvint.docs_") && name != "corvint.docs_draft" && name != "corvint.docs_consume":
			if strings.Contains(description, "repository e2e") || tool["annotations"].(map[string]any)["readOnlyHint"] != true {
				t.Fatalf("corpus tool %s is follow-marked: %#v", name, tool)
			}
			if repository, present := properties["repository"].(map[string]any); present && repository["enum"] != nil {
				t.Fatalf("corpus tool %s takes a root alias: %#v", name, tool)
			}
		case name == bridge.ToolHelp:
		default:
			enum := properties["repository"].(map[string]any)["enum"]
			if !reflect.DeepEqual(enum, []any{"app", "e2e"}) || !strings.Contains(description, "naming repository e2e") {
				t.Fatalf("per-root tool %s: %#v", name, tool)
			}
		}
	}
	for _, name := range append(code, "corvint.docs_consume", "corvint.docs_draft", "corvint.test_validity", "corvint.docs_search") {
		if !slices.Contains(names, name) {
			t.Fatalf("tools/list misses %s: %q", name, names)
		}
	}
	if len(names) != len(slices.Compact(slices.Clone(names))) {
		t.Fatalf("duplicate tool names: %q", names)
	}
	for _, id := range []string{"2", "3", "4", "5"} {
		if result := resultOf(t, responses[id]); result["isError"] != false {
			t.Fatalf("call %s: %#v", id, result)
		}
	}
	if blocks := followBlocks(t, resultOf(t, responses["2"])); len(blocks) != 1 || blocks[0]["repository"] != "e2e" || blocks[0]["ref"] != "integration" {
		t.Fatalf("e2e group call is not bound to the followed ref: %#v", blocks)
	}
	for _, id := range []string{"3", "4", "5"} {
		if blocks := followBlocks(t, resultOf(t, responses[id])); len(blocks) != 0 {
			t.Fatalf("call %s carries a follow-ref block: %#v", id, blocks)
		}
	}
	if _, timed := resultOf(t, responses["5"])["_meta"].(map[string]any)["corvint/timing"]; !timed {
		t.Fatalf("corpus timing _meta missing: %#v", responses["5"])
	}
	for _, id := range []string{"6", "7"} {
		if responses[id]["error"] == nil {
			t.Fatalf("call %s without a declared alias accepted: %#v", id, responses[id])
		}
	}

	// The followed root's group answer depends on that root at the followed
	// ref: the draft source exists only on the moved branch, and its claim
	// changes when the branch moves again between calls.
	e2eMain := gitTest(t, e2e, "rev-parse", "--path-format=absolute", "--git-common-dir")
	e2eMain = filepath.Dir(e2eMain)
	if err := os.MkdirAll(filepath.Join(e2eMain, "cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	commitFile(t, e2eMain, "cache/demux.go", "package cache\n\nfunc Join(key string) string { return key }\n")
	draftOwner := func(claim string) string {
		return "# Cache\nDelivery status: experimental\n\n## Agent digest\n- Claim: " + claim + "\n- Blocked on: behavior validation\n\n## Requirements\nNever promote drafts.\n"
	}
	first := commitFile(t, e2eMain, "owner.md", draftOwner("e2e-first-claim"))
	session := startPipeSession(t, arguments)
	draft := func(id int, repository string) (string, []map[string]any) {
		t.Helper()
		result := resultOf(t, session.send(toolCall(id, "corvint.docs_draft", map[string]any{"repository": repository, "source": "owner.md", "package": "cache"})))
		if result["isError"] != false {
			t.Fatalf("draft %s: %#v", repository, result)
		}
		raw, err := json.Marshal(result["structuredContent"])
		if err != nil {
			t.Fatal(err)
		}
		return string(raw), followBlocks(t, result)
	}
	answer, blocks := draft(1, "e2e")
	if !strings.Contains(answer, "e2e-first-claim") || strings.Contains(answer, "splits keys") || len(blocks) != 1 || blocks[0]["repository"] != "e2e" || blocks[0]["commit"] != first || headRevision(t, e2e) != first {
		t.Fatalf("followed draft is not answered from e2e at the followed ref %s: blocks=%#v answer=%s", first, blocks, answer)
	}
	second := commitFile(t, e2eMain, "owner.md", draftOwner("e2e-second-claim"))
	answer, blocks = draft(2, "e2e")
	if !strings.Contains(answer, "e2e-second-claim") || strings.Contains(answer, "e2e-first-claim") || len(blocks) != 1 || blocks[0]["state"] != "REFRESHED" || blocks[0]["commit"] != second || headRevision(t, e2e) != second {
		t.Fatalf("advanced ref is not followed before the next answer %s: blocks=%#v answer=%s", second, blocks, answer)
	}
	answer, blocks = draft(3, "app")
	if !strings.Contains(answer, "splits keys") || strings.Contains(answer, "e2e-second-claim") || len(blocks) != 0 {
		t.Fatalf("app draft is not answered from the app checkout: blocks=%#v answer=%s", blocks, answer)
	}
	if exit := session.close(); exit != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, session.stderr.String())
	}
}

// dedicatedSession runs one dedicated server binary over the requests and
// returns each response by its JSON-RPC id.
func dedicatedSession(t *testing.T, binary string, arguments []string, requests ...map[string]any) map[string]map[string]any {
	t.Helper()
	var input bytes.Buffer
	for _, request := range requests {
		request["jsonrpc"] = "2.0"
		params, _ := request["params"].(map[string]any)
		params["_meta"] = map[string]any{"io.modelcontextprotocol/protocolVersion": "2026-07-28", "io.modelcontextprotocol/clientCapabilities": map[string]any{}}
		raw, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		input.Write(append(raw, '\n'))
	}
	command := exec.Command(binary, arguments...)
	command.Dir = t.TempDir()
	command.Stdin = &input
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		t.Fatalf("%s %q: %v: %s", binary, arguments, err, stderr.Bytes())
	}
	responses := map[string]map[string]any{}
	for _, line := range bytes.Split(bytes.TrimSpace(output), []byte("\n")) {
		var response map[string]any
		if err := json.Unmarshal(line, &response); err != nil {
			t.Fatalf("%v: %s", err, line)
		}
		responses[fmt.Sprint(response["id"])] = response
	}
	return responses
}

// withoutMeta drops the server-identity and per-call timing _meta, the only
// members the protocol lets differ between two servers answering one tool.
func withoutMeta(response map[string]any) map[string]any {
	clone := maps.Clone(response)
	if result, ok := clone["result"].(map[string]any); ok {
		trimmed := maps.Clone(result)
		delete(trimmed, "_meta")
		clone["result"] = trimmed
	}
	return clone
}

// MMR-V0-013 and MMR-V0-014 parity: every grouped tool answers the same
// result through the unified server as through its dedicated server, for
// success, tool errors and invalid parameters.
func TestMMRV0013ParityWithDedicatedServers(t *testing.T) {
	skipUnqualifiedStatus(t)
	if testing.Short() {
		t.Skip("builds the three dedicated server binaries")
	}
	app, e2e := groupsFixture(t)
	binaries := t.TempDir()
	// Without VCS stamping the dedicated corpus server has this test binary's
	// unrecorded builder label, so it rederives the artifact the fixture built.
	build := exec.Command("go", "build", "-buildvcs=false", "-o", binaries, "../../../cmd/corvint-test-validity-mcp", "../../../cmd/corvint-docs-mcp", "../../../cmd/corvint-corpus-mcp")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v: %s", err, output)
	}
	type parityCall struct {
		binary, root, alias, name string
		arguments                 map[string]any
	}
	calls := []parityCall{
		{"corvint-test-validity-mcp", app, "app", "corvint.test_validity", map[string]any{}},
		{"corvint-test-validity-mcp", e2e, "e2e", "corvint.test_validity", map[string]any{"discover": true}},
		{"corvint-test-validity-mcp", app, "app", "corvint.test_validity", map[string]any{"receipt": "absent.json"}},
		{"corvint-test-validity-mcp", app, "app", "corvint.test_validity", map[string]any{"unknown": true}},
		{"corvint-docs-mcp", app, "app", "corvint.docs_draft", map[string]any{"source": "owner.md", "package": "cache"}},
		{"corvint-docs-mcp", app, "app", "corvint.docs_draft", map[string]any{"source": "absent.md", "package": "cache"}},
		{"corvint-docs-mcp", e2e, "e2e", "corvint.docs_draft", map[string]any{"source": "AGENTS.md", "package": "absent"}},
		{"corvint-corpus-mcp", app, "", "corvint.docs_search", map[string]any{"query": "Original"}},
		{"corvint-corpus-mcp", app, "", "corvint.docs_search", map[string]any{"query": "Original", "receipt": "full"}},
		{"corvint-corpus-mcp", app, "", "corvint.docs_search", map[string]any{"query": ""}},
	}
	unified := []map[string]any{}
	for index, call := range calls {
		arguments := maps.Clone(call.arguments)
		if call.alias != "" {
			arguments["repository"] = call.alias
		}
		unified = append(unified, toolCall(index, call.name, arguments))
	}
	responses := multiRootSession(t, []string{"--root", "app=" + app, "--root", "e2e=" + e2e,
		"--tool-profile", "test-validity,docs,corpus", "--corpus-artifact", "app=corpus.json"}, unified...)
	for index, call := range calls {
		arguments := []string{"--root", call.root}
		if call.binary == "corvint-corpus-mcp" {
			arguments = append(arguments, "--artifact", "corpus.json")
		}
		dedicated := dedicatedSession(t, filepath.Join(binaries, call.binary), arguments, toolCall(index, call.name, maps.Clone(call.arguments)))
		want, got := withoutMeta(dedicated[fmt.Sprint(index)]), withoutMeta(responses[fmt.Sprint(index)])
		if !reflect.DeepEqual(want, got) {
			wantRaw, _ := json.Marshal(want)
			gotRaw, _ := json.Marshal(got)
			t.Fatalf("%s %s %v differs:\ndedicated %s\nunified   %s", call.binary, call.name, call.arguments, wantRaw, gotRaw)
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

func corpusRefused(t *testing.T, response map[string]any) bool {
	t.Helper()
	result := resultOf(t, response)
	structured, _ := result["structuredContent"].(map[string]any)
	return result["isError"] == true && structured["code"] == "corpus-refused" && structured["profile"] == "corvint-corpus-mcp-tool-error/0"
}

// MMR-V0-014: a missing artifact is noted, not a startup failure: corpus tools
// answer corpus-refused until the artifact can be opened, and a stale one is
// refused, while the other groups keep serving.
func TestMMRV0014CorpusArtifactMissingAndStale(t *testing.T) {
	app, _ := groupsFixture(t)
	missing := startPipeSession(t, []string{"--root", app, "--tool-profile", "test-validity,corpus", "--corpus-artifact", "later.json"})
	if response := missing.send(toolCall(1, "corvint.docs_search", map[string]any{"query": "Original"})); !corpusRefused(t, response) {
		t.Fatalf("missing artifact: %#v", response)
	}
	if response := missing.send(toolCall(2, "corvint.test_validity", map[string]any{})); resultOf(t, response)["isError"] != false {
		t.Fatalf("other group stopped serving: %#v", response)
	}
	writeCorpusArtifact(t, app, "later.json")
	if response := missing.send(toolCall(3, "corvint.docs_search", map[string]any{"query": "Original"})); resultOf(t, response)["isError"] != false {
		t.Fatalf("artifact built after startup is not opened: %#v", response)
	}
	if exit := missing.close(); exit != 0 || !strings.Contains(missing.stderr.String(), "corvint-mcp: note: --corpus-artifact unavailable (") {
		t.Fatalf("exit=%d stderr=%q", exit, missing.stderr.String())
	}

	stale := startPipeSession(t, []string{"--root", app, "--tool-profile", "docs,corpus", "--corpus-artifact", filepath.Join(app, "corpus.json")})
	if response := stale.send(toolCall(1, "corvint.docs_search", map[string]any{"query": "Original"})); resultOf(t, response)["isError"] != false {
		t.Fatalf("absolute artifact path below the root: %#v", response)
	}
	if err := os.WriteFile(filepath.Join(app, "corpus.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if response := stale.send(toolCall(2, "corvint.docs_search", map[string]any{"query": "Original"})); !corpusRefused(t, response) {
		t.Fatalf("stale artifact: %#v", response)
	}
	if response := stale.send(toolCall(3, "corvint.docs_draft", map[string]any{"source": "owner.md", "package": "cache"})); resultOf(t, response)["isError"] != false {
		t.Fatalf("other group stopped serving: %#v", response)
	}
	if exit := stale.close(); exit != 0 || stale.stderr.Len() != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, stale.stderr.String())
	}
}

// MMR-V0-014: --corpus-artifact and the corpus group require each other, and
// a bad value names its reason before any MCP byte.
func TestMMRV0014CorpusArtifactRefusals(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	other := filepath.Clean(t.TempDir())
	for _, refusal := range []struct {
		arguments []string
		reason    string
	}{
		{[]string{"--root", root, "--tool-profile", "corpus"}, corpusReasonMissing},
		{[]string{"--root", root, "--corpus-artifact", "corpus.json"}, corpusReasonUnpaired},
		{[]string{"--root", root, "--tool-profile", "docs", "--corpus-artifact", "corpus.json"}, corpusReasonUnpaired},
		{[]string{"--root", root, "--tool-profile", "corpus", "--corpus-artifact", "a", "--corpus-artifact", "b"}, corpusReasonRepeated},
		{[]string{"--root", root, "--tool-profile", "corpus", "--corpus-artifact", ""}, corpusReasonRepeated},
		{[]string{"--version", "--corpus-artifact", "corpus.json"}, corpusReasonVersion},
		{[]string{"--root", "app=" + root, "--tool-profile", "corpus", "--corpus-artifact", "corpus.json"}, corpusReasonAlias},
		{[]string{"--root", "app=" + root, "--tool-profile", "corpus", "--corpus-artifact", "e2e=corpus.json"}, corpusReasonAlias},
		{[]string{"--root", root, "--tool-profile", "corpus", "--corpus-artifact", filepath.Join(other, "corpus.json")}, "--corpus-artifact: absolute path is not below --root"},
		{[]string{"--root", root, "--tool-profile", "corpus", "--corpus-artifact", "../corpus.json"}, "--corpus-artifact: local input paths must be canonical"},
	} {
		var stdout, stderr bytes.Buffer
		if exit := run(context.Background(), refusal.arguments, bytes.NewReader(nil), &stdout, &stderr); exit != 2 || stdout.Len() != 0 ||
			!strings.HasPrefix(stderr.String(), "corvint-mcp: invalid arguments: "+refusal.reason) {
			t.Fatalf("%q exit=%d stdout=%q stderr=%q", refusal.arguments, exit, stdout.String(), stderr.String())
		}
	}
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
	declared, note := run2("--workspace", file, "--tool-profile", "test-validity")
	explicit, _ := run2("--root", "app="+app, "--root", "e2e="+e2e, "--tool-profile", "test-validity")
	if declared != explicit || note != "corvint-mcp: note: workspace alias e2e declares revision \"integration\", which is not followed; --follow-ref e2e=REF follows a ref\n" {
		t.Fatalf("--workspace differs from its --root expansion:\n%s\n%s\nstderr=%q", declared, explicit, note)
	}
	followed, _ := run2("--workspace", file, "--follow-ref", "e2e=integration", "--tool-profile", "test-validity")
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

// replaceRoot moves root aside and puts a different repository, carrying its
// own built artifact named artifact, at the same path.
func replaceRoot(t *testing.T, root, artifact string) {
	t.Helper()
	other := makeHostileRepository(t, "replacement")
	commitFile(t, other, "source.md", "# Evidence\n\nReplacement source.\n")
	writeCorpusArtifact(t, other, artifact)
	if err := os.Rename(root, root+".replaced"); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(other, root); err != nil {
		t.Fatal(err)
	}
}

func refusedCode(t *testing.T, response map[string]any, reason string) bool {
	t.Helper()
	message, _ := resultOf(t, response)["structuredContent"].(map[string]any)["error"].(string)
	return corpusRefused(t, response) && strings.Contains(message, "("+reason+")")
}

// MMR-V0-017: the root identity is pinned at startup. A retry after a missing
// artifact never opens an artifact under a root replaced since, and an
// artifact opened at startup is refused once its root is replaced.
func TestMMRV0017CorpusRetryRefusesReplacedRoot(t *testing.T) {
	app, _ := groupsFixture(t)
	missing := startPipeSession(t, []string{"--root", app, "--tool-profile", "corpus", "--corpus-artifact", "later.json"})
	if response := missing.send(toolCall(1, "corvint.docs_search", map[string]any{"query": "Replacement"})); !refusedCode(t, response, "corpus-unavailable") {
		t.Fatalf("missing artifact: %#v", response)
	}
	replaceRoot(t, app, "later.json")
	if response := missing.send(toolCall(2, "corvint.docs_search", map[string]any{"query": "Replacement"})); !refusedCode(t, response, "root-changed") {
		t.Fatalf("retry bound the replacement repository: %#v", response)
	}
	if exit := missing.close(); exit != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, missing.stderr.String())
	}

	opened, _ := groupsFixture(t)
	session := startPipeSession(t, []string{"--root", opened, "--tool-profile", "corpus", "--corpus-artifact", "corpus.json"})
	if response := session.send(toolCall(1, "corvint.docs_search", map[string]any{"query": "Original"})); resultOf(t, response)["isError"] != false {
		t.Fatalf("opened artifact: %#v", response)
	}
	replaceRoot(t, opened, "corpus.json")
	if response := session.send(toolCall(2, "corvint.docs_search", map[string]any{"query": "Replacement"})); !refusedCode(t, response, "root-changed") {
		t.Fatalf("opened artifact under a replaced root: %#v", response)
	}
	if exit := session.close(); exit != 0 || session.stderr.Len() != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, session.stderr.String())
	}
}

// MMR-V0-017: an artifact removed after it opened is corpus-refused, as a
// missing one is at startup, never the registry's unavailable-input code.
func TestMMRV0017CorpusArtifactRemovedAfterOpen(t *testing.T) {
	app, _ := groupsFixture(t)
	session := startPipeSession(t, []string{"--root", app, "--tool-profile", "test-validity,corpus", "--corpus-artifact", "corpus.json"})
	if response := session.send(toolCall(1, "corvint.docs_search", map[string]any{"query": "Original"})); resultOf(t, response)["isError"] != false {
		t.Fatalf("opened artifact: %#v", response)
	}
	if err := os.Remove(filepath.Join(app, "corpus.json")); err != nil {
		t.Fatal(err)
	}
	for id, call := range []struct {
		name      string
		arguments map[string]any
	}{{"corvint.docs_search", map[string]any{"query": "Original"}}, {"corvint.docs_info", map[string]any{}}} {
		if response := session.send(toolCall(id+2, call.name, call.arguments)); !refusedCode(t, response, "corpus-unavailable") {
			t.Fatalf("%s after removal: %#v", call.name, response)
		}
	}
	if response := session.send(toolCall(4, "corvint.test_validity", map[string]any{})); resultOf(t, response)["isError"] != false {
		t.Fatalf("other group stopped serving: %#v", response)
	}
	if exit := session.close(); exit != 0 || session.stderr.Len() != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, session.stderr.String())
	}
}

// MMR-V0-017: every corpus tool name takes the corpus path, including one the
// opened artifact does not declare, so none binds the followed ref: the
// worktree stays on its commit after the branch moves.
func TestMMRV0017UndeclaredCorpusToolNeverBindsFollowedRef(t *testing.T) {
	skipUnqualifiedStatus(t)
	repository := makeHostileRepository(t, "follow")
	// The artifact is ignored, so the followed worktree stays clean and a
	// bind would check the moved branch out.
	commitFile(t, repository, ".gitignore", "corpus.json\n")
	commitFile(t, repository, "source.md", "# Evidence\n\nOriginal source.\n")
	gitTest(t, repository, "checkout", "-q", "-b", "integration")
	worktree := filepath.Join(filepath.Clean(t.TempDir()), "lane")
	gitTest(t, repository, "worktree", "add", "-q", "--detach", worktree, "integration")
	worktree, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		t.Fatal(err)
	}
	writeCorpusArtifact(t, worktree, "corpus.json")
	pinned := headRevision(t, worktree)
	arguments := []string{"--root", worktree, "--follow-ref", "integration", "--tool-profile", "task-review,corpus", "--corpus-artifact", "corpus.json"}
	undeclared := "corvint.docs_get_journey"
	if slices.Contains(toolNames(listedTools(t, arguments)), undeclared) {
		t.Fatalf("fixture artifact declares %s", undeclared)
	}
	moved := commitFile(t, repository, "NEW.md", "moved\n")
	session := startPipeSession(t, arguments)
	response := session.send(toolCall(1, undeclared, map[string]any{"id": "journey"}))
	if response["error"] == nil || headRevision(t, worktree) != pinned {
		t.Fatalf("undeclared corpus tool bound the followed ref (head %s, moved %s): %#v", headRevision(t, worktree), moved, response)
	}
	if response := session.send(toolCall(2, "corvint.docs_search", map[string]any{"query": "Original"})); resultOf(t, response)["isError"] != false || len(followBlocks(t, resultOf(t, response))) != 0 || headRevision(t, worktree) != pinned {
		t.Fatalf("declared corpus tool bound the followed ref: %#v", response)
	}
	// A code tool still binds, so the unmoved head above is the corpus path's.
	if response := session.send(toolCall(3, "corvint.status", map[string]any{})); len(followBlocks(t, resultOf(t, response))) != 1 || headRevision(t, worktree) != moved {
		t.Fatalf("code tool did not bind the followed ref: %#v", response)
	}
	if exit := session.close(); exit != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, session.stderr.String())
	}
}

// MCPV0-950 with MMR-V0-014: a subdirectory root binds its toplevel before the
// corpus artifact does, so an absolute artifact under the toplevel and a
// relative one under the declared subdirectory both open.
func TestMCPV0950CorpusArtifactUnderASubdirectoryRoot(t *testing.T) {
	app, _ := groupsFixture(t)
	subdirectory := filepath.Join(app, "cache")
	writeCorpusArtifact(t, app, filepath.Join("cache", "nested.json"))
	for name, artifact := range map[string]string{
		"absolute under the toplevel":     filepath.Join(app, "corpus.json"),
		"relative under the subdirectory": "nested.json",
	} {
		session := startPipeSession(t, []string{"--root", subdirectory, "--tool-profile", "corpus", "--corpus-artifact", artifact})
		if response := session.send(toolCall(1, "corvint.docs_search", map[string]any{"query": "Original"})); resultOf(t, response)["isError"] != false {
			t.Fatalf("%s: %#v", name, response)
		}
		if exit := session.close(); exit != 0 || session.stderr.Len() != 0 {
			t.Fatalf("%s: exit=%d stderr=%q", name, exit, session.stderr.String())
		}
	}
	var stdout, stderr bytes.Buffer
	if exit := run(context.Background(), []string{"--root", subdirectory, "--tool-profile", "corpus", "--corpus-artifact", "/elsewhere/corpus.json"}, bytes.NewReader(nil), &stdout, &stderr); exit != 2 ||
		stderr.String() != "corvint-mcp: invalid arguments: --corpus-artifact: absolute path is not below --root\n" {
		t.Fatalf("outside artifact exit=%d stderr=%q", exit, stderr.String())
	}
}
