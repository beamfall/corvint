// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
)

// followRootsFixture is the GitHub #2010 workspace: an application checkout
// served as is, and a test repository whose branch `integration` is followed
// by a detached linked worktree.
func followRootsFixture(t *testing.T) (app, e2eMain, e2e string) {
	t.Helper()
	app = makeHostileRepository(t, "app")
	e2eMain = makeHostileRepository(t, "e2e")
	gitTest(t, e2eMain, "checkout", "-q", "-b", "integration")
	e2e = filepath.Join(filepath.Clean(t.TempDir()), "e2e")
	gitTest(t, e2eMain, "worktree", "add", "-q", "--detach", e2e, "integration")
	e2e, err := filepath.EvalSymlinks(e2e)
	if err != nil {
		t.Fatal(err)
	}
	return app, e2eMain, e2e
}

// followBlocks returns every follow-ref block a tools/call result carries.
func followBlocks(t *testing.T, result map[string]any) []map[string]any {
	t.Helper()
	content, _ := result["content"].([]any)
	var blocks []map[string]any
	for _, item := range content {
		text, _ := item.(map[string]any)["text"].(string)
		var block map[string]any
		if json.Unmarshal([]byte(text), &block) == nil && block["schema"] == followRefSchema {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

// MMR-V0-010 (GitHub #2010): one multi-root server serves the application
// root from its checkout while the e2e root follows a branch in its own
// dedicated worktree; the follow-ref block names the root it binds.
func TestMMRV0010PerRootFollowRefServesBothRoots(t *testing.T) {
	skipUnqualifiedStatus(t)
	app, e2eMain, e2e := followRootsFixture(t)
	appHead := headRevision(t, app)
	first := headRevision(t, e2eMain)

	// The issue's argv starts.
	issue := []string{"--root", "app=" + app, "--root", "e2e=" + e2e, "--follow-ref", "e2e=integration", "--protocol-version", "2025-11-25", "--tool-profile", "task-review"}
	var stdout, stderr bytes.Buffer
	if exit := run(context.Background(), issue, strings.NewReader(""), &stdout, &stderr); exit != 0 || stderr.Len() != 0 {
		t.Fatalf("issue argv exit=%d stderr=%q", exit, stderr.String())
	}

	arguments := []string{"--root", "app=" + app, "--root", "e2e=" + e2e, "--follow-ref", "e2e=integration", "--tool-profile", "task-review"}
	responses := multiRootSession(t, arguments,
		map[string]any{"id": 1, "method": "tools/list"},
		toolCall(2, "corvint.status", map[string]any{"repository": "e2e"}),
		toolCall(3, "corvint.status", map[string]any{"repository": "app"}),
		toolCall(4, "corvint.status", map[string]any{}),
	)
	tools, _ := responses["1"]["result"].(map[string]any)["tools"].([]any)
	if len(tools) == 0 {
		t.Fatalf("tools/list=%#v", responses["1"])
	}
	for _, raw := range tools {
		tool := raw.(map[string]any)
		annotations := tool["annotations"].(map[string]any)
		description := tool["description"].(string)
		if tool["name"] == bridge.ToolHelp {
			if annotations["readOnlyHint"] != true || strings.Contains(description, "--follow-ref") {
				t.Fatalf("corvint.help changed under --follow-ref: %#v", tool)
			}
			continue
		}
		if annotations["readOnlyHint"] != false || !strings.Contains(description, "naming repository e2e") || strings.Contains(description, "repository app") {
			t.Fatalf("%v under --follow-ref ALIAS=REF: %#v", tool["name"], tool)
		}
	}
	if statusRevision(t, responses["2"]) != first {
		t.Fatal("e2e answer not bound to the followed ref")
	}
	blocks := followBlocks(t, responses["2"]["result"].(map[string]any))
	if len(blocks) != 1 || blocks[0]["repository"] != "e2e" || blocks[0]["ref"] != "integration" || blocks[0]["commit"] != first || blocks[0]["state"] != "CURRENT" {
		t.Fatalf("e2e follow-ref block=%#v", blocks)
	}
	if statusRevision(t, responses["3"]) != appHead || len(followBlocks(t, responses["3"]["result"].(map[string]any))) != 0 {
		t.Fatalf("app root is not served from its checkout: %#v", responses["3"])
	}
	all := responses["4"]["result"].(map[string]any)
	if blocks := followBlocks(t, all); len(blocks) != 1 || blocks[0]["repository"] != "e2e" || blocks[0]["commit"] != first {
		t.Fatalf("status-all follow-ref blocks=%#v", blocks)
	}

	// A moved branch moves only the followed root before its next answer.
	handler := followRootsHandler(t, app, e2e, map[string]string{"e2e": "integration"})
	moved := commitFile(t, e2eMain, "NEW.md", "moved\n")
	result, failure := handler.call(context.Background(), map[string]any{"name": "corvint.status", "arguments": map[string]any{"repository": "e2e"}})
	if failure != nil {
		t.Fatalf("protocol failure: %#v", failure)
	}
	blocks = followBlocks(t, result)
	if len(blocks) != 1 || blocks[0]["state"] != "REFRESHED" || blocks[0]["commit"] != moved || blocks[0]["repository"] != "e2e" || statusRevision(t, map[string]any{"result": result}) != moved {
		t.Fatalf("moved ref: %#v", blocks)
	}
	if headRevision(t, e2e) != moved || headRevision(t, app) != appHead {
		t.Fatal("the move reached the wrong worktree")
	}
	result, _ = handler.call(context.Background(), map[string]any{"name": "corvint.status", "arguments": map[string]any{"repository": "app"}})
	if len(followBlocks(t, result)) != 0 || statusRevision(t, map[string]any{"result": result}) != appHead {
		t.Fatalf("app call bound to the ref: %#v", result)
	}

	// A ref that stops resolving is that root's typed tool error, routed or listed.
	gitTest(t, e2eMain, "update-ref", "-d", "refs/heads/integration")
	result, _ = handler.call(context.Background(), map[string]any{"name": "corvint.status", "arguments": map[string]any{"repository": "e2e"}})
	structured, _ := result["structuredContent"].(map[string]any)
	if blocks = followBlocks(t, result); result["isError"] != true || structured["code"] != followRefMissing || len(blocks) != 1 || blocks[0]["state"] != "MISSING" || blocks[0]["repository"] != "e2e" {
		t.Fatalf("missing ref routed: %#v", result)
	}
	result, _ = handler.call(context.Background(), map[string]any{"name": "corvint.status", "arguments": map[string]any{}})
	listing, _ := result["structuredContent"].(map[string]any)["repositories"].([]any)
	if result["isError"] != false || len(listing) != 2 || listing[1].(map[string]any)["result"].(map[string]any)["code"] != followRefMissing || listing[0].(map[string]any)["result"].(map[string]any)["code"] != nil {
		t.Fatalf("missing ref listed: %#v", result)
	}
	if blocks = followBlocks(t, result); len(blocks) != 1 || blocks[0]["state"] != "MISSING" || headRevision(t, e2e) != moved {
		t.Fatalf("missing ref listed blocks: %#v", blocks)
	}
}

// followRootsHandler builds the multi-root handler run builds, with the given
// aliases following refs.
func followRootsHandler(t *testing.T, app, e2e string, follows map[string]string) *toolHandler {
	t.Helper()
	handler := &toolHandler{}
	var refusal string
	handler.repositories, handler.aliases, refusal = openRepositories([]rootDeclaration{{alias: "app", root: app}, {alias: "e2e", root: e2e}}, bridge.New)
	if refusal != "" {
		t.Fatal(refusal)
	}
	handler.follows = map[string]*followRef{}
	for alias, ref := range follows {
		follow, refusal := newFollowRef(context.Background(), "git", handler.repositories[alias], ref)
		if refusal != "" {
			t.Fatal(refusal)
		}
		follow.alias = alias
		handler.follows[alias] = follow
	}
	return handler
}

// MMR-V0-011 and MCPV0-039: every refusal on the --follow-ref path names its
// reason; the single-root startup refusals stay byte-identical.
func TestMMRV0011FollowRefRefusalsNameTheirReason(t *testing.T) {
	app, e2eMain, e2e := followRootsFixture(t)
	refuse := func(arguments []string, want string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if exit := run(context.Background(), arguments, strings.NewReader(""), &stdout, &stderr); exit != 2 || stdout.Len() != 0 || stderr.String() != want {
			t.Fatalf("%q: exit=%d stderr=%q want %q", arguments, exit, stderr.String(), want)
		}
	}
	invalid := "corvint-mcp: invalid arguments: "
	two := []string{"--root", "app=" + app, "--root", "e2e=" + e2e}
	refuse(append(slices.Clone(two), "--follow-ref", "integration"), invalid+followReasonBareWithAliases+"\n")
	refuse([]string{"--root", "e2e=" + e2e, "--follow-ref", "integration"}, invalid+followReasonBareWithAliases+"\n")
	refuse(append(slices.Clone(two), "--follow-ref", "ci=integration"), invalid+"--follow-ref ci=integration names undeclared repository alias ci\n")
	refuse(append(slices.Clone(two), "--follow-ref", "e2e=integration", "--follow-ref", "e2e=main"), invalid+"--follow-ref repeats repository alias e2e\n")
	refuse([]string{"--root", e2e, "--follow-ref", "e2e=integration"}, invalid+followReasonAliasedWithPlain+"\n")
	refuse([]string{"--root", e2e, "--follow-ref", "integration", "--follow-ref", "integration"}, invalid+followReasonRepeated+"\n")
	refuse([]string{"--root", e2e, "--follow-ref", "-x"}, invalid+followReasonValue+"\n")
	refuse(append(slices.Clone(two), "--follow-ref", "e2e="), invalid+followReasonValue+"\n")
	refuse([]string{"--root", e2e, "--follow-ref"}, invalid+followReasonMissingValue+"\n")
	refuse([]string{"--version", "--follow-ref", "integration"}, invalid+followReasonVersion+"\n")
	refuse([]string{"--root", "app=" + app, "--root", "e2e=" + e2eMain, "--follow-ref", "e2e=integration"}, "corvint-mcp: follow-ref-not-dedicated: repository e2e\n")
	refuse(append(slices.Clone(two), "--follow-ref", "e2e=no-such-branch"), "corvint-mcp: follow-ref-missing: repository e2e\n")
	// Unchanged single-root startup refusals.
	refuse([]string{"--root", e2eMain, "--follow-ref", "integration"}, "corvint-mcp: follow-ref-not-dedicated\n")
	refuse([]string{"--root", e2e, "--follow-ref", "no-such-branch"}, "corvint-mcp: follow-ref-missing\n")
	// A refusal outside the --follow-ref path keeps the bare text.
	refuse([]string{"--root", e2e, "--root", app, "--follow-ref", "integration"}, "corvint-mcp: invalid arguments\n")
	// --help names both selector forms.
	var stdout, stderr bytes.Buffer
	if exit := run(context.Background(), []string{"--help"}, strings.NewReader(""), &stdout, &stderr); exit != 0 || !strings.Contains(stdout.String(), "--follow-ref REF\n") || !strings.Contains(stdout.String(), "--follow-ref ALIAS=REF") {
		t.Fatalf("--help: %q", stdout.String())
	}
}

// MCPV0-041 and MMR-V0-010: the unaliased single-root block keeps its exact
// member set, and a ref containing '=' stays reachable through its full name.
func TestMMRV0010SingleRootBlockUnchanged(t *testing.T) {
	skipUnqualifiedStatus(t)
	main, lane := followFixture(t)
	_, block := followCall(t, followHandler(t, lane, "feature"))
	keys := make([]string, 0, len(block))
	for key := range block {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	if want := []string{"commit", "head", "mutates", "ref", "schema", "stale", "state"}; !slices.Equal(keys, want) {
		t.Fatalf("single-root block members %v, want %v", keys, want)
	}
	gitTest(t, main, "update-ref", "refs/heads/release=1", gitTest(t, main, "rev-parse", "HEAD"))
	var stdout, stderr bytes.Buffer
	if exit := run(context.Background(), []string{"--root", lane, "--follow-ref", "refs/heads/release=1"}, strings.NewReader(""), &stdout, &stderr); exit != 0 || stderr.Len() != 0 {
		t.Fatalf("full ref name with '=': exit=%d stderr=%q", exit, stderr.String())
	}
}

// MMR-V0-010: REF keeps its 256-byte allowance after an ALIAS= prefix; the
// bound applies to the parsed REF, not to the whole selector value.
func TestMMRV0010FollowRefBoundsTheRefNotTheValue(t *testing.T) {
	app, _, e2e := followRootsFixture(t)
	two := []string{"--root", "app=" + app, "--root", "e2e=" + e2e}
	for _, test := range []struct {
		arguments []string
		want      string
	}{
		{append(slices.Clone(two), "--follow-ref", "e2e="+strings.Repeat("a", 254)), "corvint-mcp: follow-ref-missing: repository e2e\n"},
		{append(slices.Clone(two), "--follow-ref", "e2e="+strings.Repeat("a", 256)), "corvint-mcp: follow-ref-missing: repository e2e\n"},
		{append(slices.Clone(two), "--follow-ref", "e2e="+strings.Repeat("a", 257)), "corvint-mcp: invalid arguments: " + followReasonValue + "\n"},
		{[]string{"--root", e2e, "--follow-ref", strings.Repeat("a", 256)}, "corvint-mcp: follow-ref-missing\n"},
		{[]string{"--root", e2e, "--follow-ref", strings.Repeat("a", 257)}, "corvint-mcp: invalid arguments: " + followReasonValue + "\n"},
	} {
		var stdout, stderr bytes.Buffer
		if exit := run(context.Background(), test.arguments, strings.NewReader(""), &stdout, &stderr); exit != 2 || stderr.String() != test.want {
			t.Fatalf("value of %d bytes: exit=%d stderr=%q want %q", len(test.arguments[len(test.arguments)-1]), exit, stderr.String(), test.want)
		}
	}
}

// listedRevision is the commit one status-all entry reports.
func listedRevision(t *testing.T, entry any) string {
	t.Helper()
	result, _ := entry.(map[string]any)["result"].(map[string]any)
	repository, _ := result["repository"].(map[string]any)
	revision, _ := repository["commitRevision"].(string)
	if revision == "" {
		t.Fatalf("status-all entry=%#v", entry)
	}
	return revision
}

// MMR-V0-010: each following alias has its own lock. While one alias's lock
// is held, a call routed to another following alias still answers, and
// status-all waits for that lock before it moves, probes or reports the held
// alias, so its entry and block describe the same followed commit.
func TestMMRV0010FollowersAreIsolatedAndStatusAllHoldsEachLock(t *testing.T) {
	skipUnqualifiedStatus(t)
	app, e2eMain, e2e := followRootsFixture(t)
	libMain := makeHostileRepository(t, "lib")
	gitTest(t, libMain, "checkout", "-q", "-b", "release")
	lib := filepath.Join(filepath.Clean(t.TempDir()), "lib")
	gitTest(t, libMain, "worktree", "add", "-q", "--detach", lib, "release")
	lib, err := filepath.EvalSymlinks(lib)
	if err != nil {
		t.Fatal(err)
	}
	handler := &toolHandler{}
	var refusal string
	handler.repositories, handler.aliases, refusal = openRepositories([]rootDeclaration{{alias: "app", root: app}, {alias: "e2e", root: e2e}, {alias: "lib", root: lib}}, bridge.New)
	if refusal != "" {
		t.Fatal(refusal)
	}
	handler.follows, refusal = openFollowRefs(context.Background(), "git", handler.repositories, handler.aliases, map[string]string{"e2e": "integration", "lib": "release"})
	if refusal != "" {
		t.Fatal(refusal)
	}
	appHead, first, libHead := headRevision(t, app), headRevision(t, e2e), headRevision(t, lib)
	moved := commitFile(t, e2eMain, "NEW.md", "moved\n")

	// Hold e2e's lock, as an in-flight e2e call would, and learn when
	// status-all reaches it.
	held := handler.follows["e2e"]
	held.mu.Lock()
	locked := true
	defer func() {
		if locked {
			held.mu.Unlock()
		}
	}()
	waiting := make(chan struct{})
	var once sync.Once
	held.lockHook = func() { once.Do(func() { close(waiting) }) }
	type answer struct {
		result  map[string]any
		failure *protocol.RPCError
	}
	listed := make(chan answer, 1)
	go func() {
		result, failure := handler.call(context.Background(), map[string]any{"name": "corvint.status", "arguments": map[string]any{}})
		listed <- answer{result, failure}
	}()
	select {
	case <-waiting:
	case <-time.After(2 * time.Minute):
		t.Fatal("status-all never waited for the e2e binding")
	}

	// Another follower answers while e2e's lock is held.
	result, failure := handler.call(context.Background(), map[string]any{"name": "corvint.status", "arguments": map[string]any{"repository": "lib"}})
	if failure != nil {
		t.Fatalf("protocol failure: %#v", failure)
	}
	if blocks := followBlocks(t, result); len(blocks) != 1 || blocks[0]["repository"] != "lib" || blocks[0]["commit"] != libHead || blocks[0]["state"] != "CURRENT" {
		t.Fatalf("lib call under a held e2e lock: %#v", blocks)
	}
	// status-all has neither moved e2e nor answered while the lock is held.
	select {
	case got := <-listed:
		t.Fatalf("status-all answered without the e2e lock: %#v", got)
	default:
	}
	if head := headRevision(t, e2e); head != first {
		t.Fatalf("e2e moved to %s outside its lock", head)
	}
	locked = false
	held.mu.Unlock()

	got := <-listed
	if got.failure != nil {
		t.Fatalf("status-all protocol failure: %#v", got.failure)
	}
	listing, _ := got.result["structuredContent"].(map[string]any)["repositories"].([]any)
	if len(listing) != 3 {
		t.Fatalf("status-all listing: %#v", got.result)
	}
	for index, want := range []string{appHead, moved, libHead} {
		if revision := listedRevision(t, listing[index]); revision != want {
			t.Fatalf("status-all entry %d revision=%s want %s", index, revision, want)
		}
	}
	blocks := followBlocks(t, got.result)
	if len(blocks) != 2 || blocks[0]["repository"] != "e2e" || blocks[0]["commit"] != moved || blocks[0]["state"] != "REFRESHED" || blocks[1]["repository"] != "lib" || blocks[1]["commit"] != libHead || blocks[1]["state"] != "CURRENT" {
		t.Fatalf("status-all blocks: %#v", blocks)
	}
}
