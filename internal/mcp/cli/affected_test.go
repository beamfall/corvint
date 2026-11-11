// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/rootbind"
	"github.com/Beamfall/corvint/internal/mcp/toolgroup"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

// stubAffectedRunner stands in for the corvint binary's planner: it echoes
// the root and base it was asked for as the summary document, and refuses
// the base "refused" the way the CLI's argument parser would.
func stubAffectedRunner(_ context.Context, root, base string, stdout, stderr io.Writer) int {
	if base == "refused" {
		_, _ = fmt.Fprintln(stderr, `{"code": "invalid-argument", "error": "argument --base: refused", "ok": false}`)
		return 2
	}
	_, _ = fmt.Fprintf(stdout, "{\"base\":%q,\"ok\":true,\"profile\":\"affected-summary/1\",\"root\":%q}\n", base, root)
	return 0
}

// hookedSession is legacySession with the corvint binary's hooks.
func hookedSession(t *testing.T, hooks Hooks, arguments []string, requests ...map[string]any) (int, map[float64]map[string]any, string) {
	t.Helper()
	lines := []string{
		`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
	}
	for index, request := range requests {
		request["jsonrpc"], request["id"] = "2.0", index+1
		raw, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, string(raw))
	}
	var stdout, stderr bytes.Buffer
	exit := runVersioned(context.Background(), serverVersion, append([]string{"--protocol-version", "2025-11-25"}, arguments...), strings.NewReader(strings.Join(lines, "\n")+"\n"), &stdout, &stderr, hooks)
	responses := map[float64]map[string]any{}
	for _, line := range bytes.Split(bytes.TrimSpace(stdout.Bytes()), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var response map[string]any
		if err := json.Unmarshal(line, &response); err != nil {
			t.Fatalf("%v: %s", err, line)
		}
		id, _ := response["id"].(float64)
		responses[id] = response
	}
	return exit, responses, stderr.String()
}

// MMR-V0-1118 (V1-5254): corvint-mcp holds no planner, so it refuses the
// affected group by name before any MCP byte, alone or in a list; the other
// groups and the default surface are unchanged.
func TestMMRV01118AffectedGroupRefusedWithoutHooks(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	gitInit(t, root)
	for _, profile := range []string{"affected", "affected,test-validity", "docs,affected"} {
		var stdout, stderr bytes.Buffer
		if exit := run(context.Background(), []string{"--root", root, "--tool-profile", profile}, strings.NewReader(""), &stdout, &stderr); exit != 2 ||
			stderr.String() != "corvint-mcp: invalid arguments: "+affectedReasonUnserved+"\n" || stdout.Len() != 0 {
			t.Fatalf("%s: exit=%d stdout=%q stderr=%q", profile, exit, stdout.String(), stderr.String())
		}
	}
	if named := toolNames(listedTools(t, []string{"--root", root})); slices.Contains(named, toolgroup.AffectedTool) {
		t.Fatalf("default surface lists %s: %q", toolgroup.AffectedTool, named)
	}
	if named := toolNames(listedTools(t, []string{"--root", root, "--tool-profile", "test-evidence"})); slices.Contains(named, toolgroup.AffectedTool) {
		t.Fatalf("test-evidence group lists %s: %q", toolgroup.AffectedTool, named)
	}
}

// MMR-V0-1118 (V1-5254): with the corvint binary's hook the affected group
// lists corvint.affected as a read-only tool and answers each call with the
// planner's stdout bytes for the declared root, framed, with the base passed
// through; a planner refusal is the group's tool error with the planner's
// code, and an option-like, empty or unknown argument is -32602.
func TestMMRV01118AffectedGroupServedThroughHooks(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	gitInit(t, root)
	hooks := Hooks{Affected: stubAffectedRunner}
	exit, responses, stderr := hookedSession(t, hooks, []string{"--root", root, "--tool-profile", "affected"},
		map[string]any{"method": "tools/list", "params": map[string]any{}},
		toolCall(0, toolgroup.AffectedTool, map[string]any{}),
		toolCall(0, toolgroup.AffectedTool, map[string]any{"base": "abc1234"}),
		toolCall(0, toolgroup.AffectedTool, map[string]any{"base": "refused"}),
		toolCall(0, toolgroup.AffectedTool, map[string]any{"base": "-x"}),
		toolCall(0, toolgroup.AffectedTool, map[string]any{"base": ""}),
		toolCall(0, toolgroup.AffectedTool, map[string]any{"path": "x"}),
		toolCall(0, toolgroup.AffectedTool, map[string]any{"base": 7}),
	)
	if exit != 0 || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", exit, stderr)
	}
	var listed []string
	for _, raw := range resultOf(t, responses[1])["tools"].([]any) {
		tool := raw.(map[string]any)
		listed = append(listed, tool["name"].(string))
		if tool["name"] == toolgroup.AffectedTool {
			annotations, _ := tool["annotations"].(map[string]any)
			if annotations["readOnlyHint"] != true || annotations["idempotentHint"] != true {
				t.Fatalf("annotations=%#v", annotations)
			}
			schema := tool["inputSchema"].(map[string]any)
			properties := schema["properties"].(map[string]any)
			if schema["additionalProperties"] != false || len(properties) != 1 || properties["base"] == nil {
				t.Fatalf("schema=%#v", schema)
			}
		}
	}
	if !slices.Contains(listed, toolgroup.AffectedTool) || !slices.Contains(listed, "corvint.status") {
		t.Fatalf("tools=%q", listed)
	}
	// The planner runs for the declared root spelling, which rootbind.Resolve
	// keeps when it is the toplevel (macOS /var -> /private/var stays /var).
	for id, base := range map[float64]string{2: "", 3: "abc1234"} {
		result := resultOf(t, responses[id])
		structured, _ := result["structuredContent"].(map[string]any)
		if result["isError"] != false || structured["profile"] != "affected-summary/1" || structured["base"] != base || structured["root"] != root {
			t.Fatalf("call %v: %#v", id, result)
		}
		var planner bytes.Buffer
		stubAffectedRunner(context.Background(), root, base, &planner, io.Discard)
		framed, err := repoenvelope.Frame(planner.String())
		if err != nil {
			t.Fatal(err)
		}
		if text := result["content"].([]any)[0].(map[string]any)["text"]; text != framed {
			t.Fatalf("call %v text %q, want the framed planner bytes %q", id, text, framed)
		}
	}
	refused := resultOf(t, responses[4])
	refusedBody, _ := refused["structuredContent"].(map[string]any)
	if refused["isError"] != true || refusedBody["code"] != "invalid-argument" || refusedBody["error"] != "argument --base: refused" ||
		refusedBody["profile"] != toolgroup.AffectedErrorProfile || refusedBody["tool"] != toolgroup.AffectedTool || refusedBody["mutates"] != false {
		t.Fatalf("refused=%#v", refused)
	}
	for _, id := range []float64{5, 6, 7, 8} {
		rpcError, _ := responses[id]["error"].(map[string]any)
		if rpcError["code"] != float64(-32602) {
			t.Fatalf("call %v: %#v", id, responses[id])
		}
	}
}

// MMR-V0-1118 with MMR-V0-013: under aliased roots the tool takes the
// repository alias and the planner runs for that alias's root.
func TestMMRV01118AffectedGroupRoutesByAlias(t *testing.T) {
	app := filepath.Clean(t.TempDir())
	e2e := filepath.Clean(t.TempDir())
	gitInit(t, app)
	gitInit(t, e2e)
	exit, responses, stderr := hookedSession(t, Hooks{Affected: stubAffectedRunner}, []string{"--root", "app=" + app, "--root", "e2e=" + e2e, "--tool-profile", "affected"},
		toolCall(0, toolgroup.AffectedTool, map[string]any{"repository": "e2e", "base": "HEAD~1"}),
		toolCall(0, toolgroup.AffectedTool, map[string]any{"base": "HEAD~1"}),
	)
	if exit != 0 || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", exit, stderr)
	}
	structured, _ := resultOf(t, responses[1])["structuredContent"].(map[string]any)
	if structured["root"] != e2e || structured["base"] != "HEAD~1" {
		t.Fatalf("routed=%#v", structured)
	}
	if rpcError, _ := responses[2]["error"].(map[string]any); rpcError["code"] != float64(-32602) {
		t.Fatalf("missing alias: %#v", responses[2])
	}
}

// MMR-V0-1118 with MCPV0-950: a server whose single root is unusable still
// lists corvint.affected and answers every call invalid-root under the
// group's tool-error profile, so no plan is ever answered for it.
func TestMMRV01118AffectedGroupUnavailableRoot(t *testing.T) {
	scratch := filepath.Clean(t.TempDir())
	plain := filepath.Join(scratch, "plain")
	if err := os.Mkdir(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	// Launch outside any worktree, so the plain root is not replaced by the
	// launch worktree (MCPV0-115) and keeps its MCPV0-950 outcome.
	if _, inside := launchWorktreeCandidate(scratch); inside {
		t.Skip("the temporary directory is inside a Git worktree")
	}
	t.Chdir(scratch)
	exit, responses, stderr := hookedSession(t, Hooks{Affected: stubAffectedRunner}, []string{"--root", plain, "--tool-profile", "affected"},
		map[string]any{"method": "tools/list", "params": map[string]any{}},
		toolCall(0, toolgroup.AffectedTool, map[string]any{}),
	)
	if exit != 0 || !strings.Contains(stderr, "corvint-mcp: repository unavailable: "+rootbind.ReasonNoWorktree) {
		t.Fatalf("exit=%d stderr=%q", exit, stderr)
	}
	var listed []string
	for _, raw := range resultOf(t, responses[1])["tools"].([]any) {
		listed = append(listed, raw.(map[string]any)["name"].(string))
	}
	if !slices.Contains(listed, toolgroup.AffectedTool) {
		t.Fatalf("tools=%q", listed)
	}
	result := resultOf(t, responses[2])
	structured, _ := result["structuredContent"].(map[string]any)
	if result["isError"] != true || structured["code"] != invalidRoot || structured["profile"] != toolgroup.AffectedErrorProfile {
		t.Fatalf("call=%#v", result)
	}
}

// MMR-V0-1118 with MCPV0-039: under --follow-ref the group call is answered
// under the followed root's lock after its worktree is synced to the ref, so
// the planner plans the followed commit, and the answer carries the
// follow-ref block beside the planner's framed bytes.
func TestMMRV01118AffectedGroupUnderFollowRef(t *testing.T) {
	main, lane := followFixture(t)
	moved := strings.TrimSpace(commitFile(t, main, "moved.txt", "moved\n"))
	if head := strings.TrimSpace(gitTest(t, lane, "rev-parse", "HEAD")); head == moved {
		t.Fatalf("the lane already sits on the moved commit %s", moved)
	}
	var plannedHead, plannedRoot string
	runner := func(ctx context.Context, root, base string, stdout, stderr io.Writer) int {
		plannedRoot = root
		plannedHead = strings.TrimSpace(gitTest(t, root, "rev-parse", "HEAD"))
		return stubAffectedRunner(ctx, root, base, stdout, stderr)
	}
	exit, responses, stderr := hookedSession(t, Hooks{Affected: runner}, []string{"--root", lane, "--follow-ref", "feature", "--tool-profile", "affected"},
		toolCall(0, toolgroup.AffectedTool, map[string]any{}),
	)
	if exit != 0 || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", exit, stderr)
	}
	result := resultOf(t, responses[1])
	structured, _ := result["structuredContent"].(map[string]any)
	if result["isError"] != false || structured["profile"] != "affected-summary/1" || plannedRoot != lane || structured["root"] != lane {
		t.Fatalf("result %#v planned root %q (lane %q)", result, plannedRoot, lane)
	}
	if plannedHead != moved {
		t.Fatalf("the planner ran at %s before the worktree followed the ref to %s", plannedHead, moved)
	}
	blocks := followBlocks(t, result)
	if len(blocks) != 1 || blocks[0]["commit"] != moved || blocks[0]["head"] != moved || blocks[0]["state"] != "REFRESHED" || blocks[0]["stale"] != false {
		t.Fatalf("follow-ref blocks %#v, want one REFRESHED block at %s", blocks, moved)
	}
	var planner bytes.Buffer
	stubAffectedRunner(context.Background(), lane, "", &planner, io.Discard)
	framed, err := repoenvelope.Frame(planner.String())
	if err != nil {
		t.Fatal(err)
	}
	if text := result["content"].([]any)[0].(map[string]any)["text"]; text != framed {
		t.Fatalf("first block %q, want the framed planner bytes", text)
	}
}
