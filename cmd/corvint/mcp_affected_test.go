// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/toolgroup"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

// MMR-V0-1118 (V1-5254): `corvint mcp --tool-profile affected` answers
// corvint.affected with the bytes `corvint affected --summary` prints for the
// same root, for the dirty worktree (no base) and for a base, as the framed
// text block and as the identical parsed structuredContent; the tool is listed
// only with the group. The server runs in a child of this test binary because
// a started server pins Git for its whole process (V1-0191).
func TestV15254MCPAffectedMatchesCLI(t *testing.T) {
	root := affectedFixtureRepository(t)
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	appendFile(t, filepath.Join(root, "core", "core.go"), "\n// edited\n")

	_, dirty, stderr, code := runAffectedCLI(t, root, "--summary")
	if code != 0 {
		t.Fatalf("--summary exit %d: %s", code, stderr)
	}
	responses := mcpAffectedChild(t, root,
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"corvint.affected","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"corvint.affected","arguments":{"base":"--full"}}}`,
	)
	var listed []string
	for _, raw := range responses["1"]["result"].(map[string]any)["tools"].([]any) {
		listed = append(listed, raw.(map[string]any)["name"].(string))
	}
	if !slices.Contains(listed, toolgroup.AffectedTool) || !slices.Contains(listed, "corvint.help") {
		t.Fatalf("tools=%q", listed)
	}
	checkMCPAffectedAnswer(t, responses["2"], dirty)
	if rpcError, _ := responses["3"]["error"].(map[string]any); rpcError["code"] != float64(-32602) {
		t.Fatalf("option-like base: %#v", responses["3"])
	}

	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "edit")
	_, ranged, stderr, code := runAffectedCLI(t, root, "--summary", "--base", base)
	if code != 0 {
		t.Fatalf("--summary --base exit %d: %s", code, stderr)
	}
	if bytes.Equal(ranged, dirty) {
		t.Fatalf("the range plan equals the dirty plan:\n%s", ranged)
	}
	responses = mcpAffectedChild(t, root,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"corvint.affected","arguments":{"base":"`+base+`"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"corvint.affected","arguments":{"base":"ffffffffffffffffffffffffffffffffffffffff"}}}`,
	)
	checkMCPAffectedAnswer(t, responses["1"], ranged)
	_, _, cliStderr, cliCode := runAffectedCLI(t, root, "--summary", "--base", "ffffffffffffffffffffffffffffffffffffffff")
	var emitted struct {
		Code  string `json:"code"`
		Error string `json:"error"`
	}
	if cliCode != 2 || json.Unmarshal([]byte(cliStderr), &emitted) != nil || emitted.Error == "" {
		t.Fatalf("CLI unknown base: exit %d stderr %q", cliCode, cliStderr)
	}
	refused := responses["2"]["result"].(map[string]any)
	structured, _ := refused["structuredContent"].(map[string]any)
	if refused["isError"] != true || structured["profile"] != toolgroup.AffectedErrorProfile || structured["code"] != emitted.Code || structured["error"] != emitted.Error {
		t.Fatalf("unknown base: MCP %#v, CLI %#v", refused, emitted)
	}
	if listed := toolNamesOfMCPList(t, root, nil); slices.Contains(listed, toolgroup.AffectedTool) {
		t.Fatalf("default surface lists %s: %q", toolgroup.AffectedTool, listed)
	}
}

// checkMCPAffectedAnswer requires one corvint.affected result to carry the
// CLI's exact bytes: the framed text block and the identical parsed object.
func checkMCPAffectedAnswer(t *testing.T, response map[string]any, cli []byte) {
	t.Helper()
	result, ok := response["result"].(map[string]any)
	if !ok || result["isError"] != false {
		t.Fatalf("result=%#v", response)
	}
	framed, err := repoenvelope.Frame(string(cli))
	if err != nil {
		t.Fatal(err)
	}
	if text := result["content"].([]any)[0].(map[string]any)["text"]; text != framed {
		t.Fatalf("text block:\n%s\nwant the framed CLI bytes:\n%s", text, framed)
	}
	var want map[string]any
	if err := json.Unmarshal(cli, &want); err != nil {
		t.Fatal(err)
	}
	if want["profile"] != "affected-summary/1" || !reflect.DeepEqual(result["structuredContent"], want) {
		t.Fatalf("structuredContent %#v\nwant %#v", result["structuredContent"], want)
	}
}

// toolNamesOfMCPList lists the server's tools with the given extra arguments.
func toolNamesOfMCPList(t *testing.T, root string, extra []string) []string {
	t.Helper()
	responses := mcpAffectedChildWith(t, root, extra, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	var listed []string
	for _, raw := range responses["1"]["result"].(map[string]any)["tools"].([]any) {
		listed = append(listed, raw.(map[string]any)["name"].(string))
	}
	return listed
}

// mcpAffectedChild serves `corvint mcp --root ROOT --tool-profile affected`
// in a child of this test binary, feeds it the initialize handshake and the
// given request lines, and returns each response by its id.
func mcpAffectedChild(t *testing.T, root string, requests ...string) map[string]map[string]any {
	t.Helper()
	return mcpAffectedChildWith(t, root, []string{"--tool-profile", "affected"}, requests...)
}

func mcpAffectedChildWith(t *testing.T, root string, extra []string, requests ...string) map[string]map[string]any {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	lines := append([]string{
		`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
	}, requests...)
	arguments := append([]string{"-test.run=^TestV15254MCPAffectedHelper$", "--", "--v15254-mcp-helper", root}, extra...)
	child := exec.Command(binary, arguments...)
	child.Stdin = strings.NewReader(strings.Join(lines, "\n") + "\n")
	var stdout, stderr bytes.Buffer
	child.Stdout, child.Stderr = &stdout, &stderr
	if err := child.Run(); err != nil {
		t.Fatalf("corvint mcp child: %v\nstdout %s\nstderr %s", err, &stdout, &stderr)
	}
	responses := map[string]map[string]any{}
	for _, line := range bytes.Split(bytes.TrimSpace(stdout.Bytes()), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var response map[string]any
		if err := json.Unmarshal(line, &response); err != nil {
			t.Fatalf("%v: %s", err, line)
		}
		id, _ := response["id"].(float64)
		responses[strconv.FormatFloat(id, 'f', -1, 64)] = response
	}
	if responses["0"] == nil || responses["0"]["result"] == nil {
		t.Fatalf("no initialize result:\n%s\nstderr %s", &stdout, &stderr)
	}
	return responses
}

// TestV15254MCPAffectedHelper is the child of TestV15254MCPAffectedMatchesCLI,
// selected by argv: it serves `corvint mcp --root ROOT [EXTRA...]
// --protocol-version 2025-11-25` from stdin and exits with the verb's code.
func TestV15254MCPAffectedHelper(t *testing.T) {
	index := slices.Index(os.Args, "--v15254-mcp-helper")
	if index < 0 || len(os.Args) < index+2 {
		return
	}
	extra := os.Args[index+2:]
	// --v15254-stale MODULE COMMIT makes the served binary claim COMMIT as its
	// build of MODULE, so the planner judges it stale against the root's HEAD
	// through the real corvint mcp path (MMR-V0-1118).
	if stale := slices.Index(extra, "--v15254-stale"); stale >= 0 && len(extra) >= stale+3 {
		selfBuildInfo = staleBuildInfoFunc(selfBinaryInfo(extra[stale+1], extra[stale+2]))
		extra = slices.Concat(extra[:stale], extra[stale+3:])
	}
	arguments := append([]string{"mcp", "--root", os.Args[index+1], "--protocol-version", "2025-11-25"}, extra...)
	os.Exit(runContext(context.Background(), arguments, os.Stdin, os.Stdout, os.Stderr))
}

func staleBuildInfoFunc(info *debug.BuildInfo) func() (*debug.BuildInfo, bool) {
	return func() (*debug.BuildInfo, bool) { return info, true }
}

// MMR-V0-1118: `affected --summary` reports self-binary-stale as a degradation
// inside its successful verdict, never as a refusal, so a stale corvint binary
// served through corvint mcp answers the CLI's exact degraded bytes with no
// tool error; because those bytes are the CLI's, the restart recovery lives in
// the tool description and the verb's help.
func TestV15254MCPAffectedStaleBinaryDegrades(t *testing.T) {
	root := affectedFixtureRepository(t)
	built := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	appendFile(t, filepath.Join(root, "core", "core.go"), "\n// edited after the build\n")
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "edit")
	stubSelfBuildInfo(t, selfBinaryInfo("example.com/fixture", built))
	receipt, cli, stderr, code := runAffectedCLI(t, root, "--summary")
	if code != 0 || !strings.Contains(stderr, "DEGRADED self-binary-stale") {
		t.Fatalf("CLI exit %d stderr %q", code, stderr)
	}
	if degradations, _ := receipt["degradations"].([]any); !reflect.DeepEqual(degradations, []any{selfBinaryStaleCode}) {
		t.Fatalf("CLI degradations %#v", receipt["degradations"])
	}
	responses := mcpAffectedChildWith(t, root, []string{"--tool-profile", "affected", "--v15254-stale", "example.com/fixture", built},
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"corvint.affected","arguments":{}}}`,
	)
	checkMCPAffectedAnswer(t, responses["2"], cli)
	structured := responses["2"]["result"].(map[string]any)["structuredContent"].(map[string]any)
	details, _ := structured["degradationDetails"].(map[string]any)
	if detail, _ := details[selfBinaryStaleCode].(string); !strings.Contains(detail, "rebuild") {
		t.Fatalf("degradationDetails %#v", structured["degradationDetails"])
	}
	var description string
	for _, raw := range responses["1"]["result"].(map[string]any)["tools"].([]any) {
		if tool := raw.(map[string]any); tool["name"] == toolgroup.AffectedTool {
			description, _ = tool["description"].(string)
		}
	}
	for name, text := range map[string]string{"tool description": description, "corvint mcp help": helpText("mcp")} {
		if !strings.Contains(text, selfBinaryStaleCode) || !strings.Contains(text, "restart") || !strings.Contains(text, "rebuild") {
			t.Fatalf("%s lacks the rebuild-and-restart recovery for %s:\n%s", name, selfBinaryStaleCode, text)
		}
	}
}
