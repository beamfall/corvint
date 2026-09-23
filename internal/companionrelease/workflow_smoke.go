package companionrelease

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/mcp/protocol"
)

var mcpMeta = map[string]any{"io.modelcontextprotocol/protocolVersion": "2026-07-28", "io.modelcontextprotocol/clientCapabilities": map[string]any{}}

func checkWorkflowTools(ctx context.Context, extracted, repo, scratch string, inventory bundleInventory) ([]SmokeStep, error) {
	var steps []SmokeStep
	for _, name := range []string{"corvint-mcp", "corvint-docs-mcp", "corvint-test-validity-mcp"} {
		bin := filepath.Join(extracted, "bin", inventory.name(name))
		out, _, err := runCaptured(ctx, scratch, minimalRunEnv(scratch), subprocessTimeout, bin, "--version")
		if err == nil && strings.TrimSpace(string(out)) != inventory.name(name)+" 0.1.0-experimental" {
			err = fmt.Errorf("version %q, want %q", strings.TrimSpace(string(out)), inventory.name(name)+" 0.1.0-experimental")
		}
		steps = append(steps, step(name+"-version", err, strings.TrimSpace(string(out))))
		if err != nil {
			return steps, err
		}
	}

	mainResponses, err := runMCP(ctx, filepath.Join(extracted, "bin", inventory.name("corvint-mcp")), repo, scratch,
		mcpRequest(1, "server/discover", map[string]any{"_meta": mcpMeta}),
		mcpRequest(2, "tools/list", map[string]any{"_meta": mcpMeta}),
		mcpRequest(3, "tools/call", map[string]any{"_meta": mcpMeta, "name": "corvint.status", "arguments": map[string]any{}}))
	if err == nil {
		err = requireMCPDiscovery(mainResponses, 0, inventory.name("corvint-mcp"), "0.1.0-experimental")
	}
	if err == nil {
		err = requireMCPTools(mainResponses, 1, []string{"corvint.cem.report", "corvint.context", "corvint.impact", "corvint.query", "corvint.status"})
	}
	if err == nil {
		err = requireMCPSuccess(mainResponses, 2)
	}
	steps = append(steps, step("corvint-mcp-discover-list-status", err, "server/discover, 5 tools, corvint.status"))
	if err != nil {
		return steps, err
	}

	docsRoot := filepath.Join(scratch, "docs-mcp-repo")
	if err = initDocsSmokeRepo(ctx, docsRoot); err != nil {
		return append(steps, step("corvint-docs-mcp-fixture", err, "")), err
	}
	draftResponses, err := runMCP(ctx, filepath.Join(extracted, "bin", inventory.name("corvint-docs-mcp")), docsRoot, scratch,
		mcpRequest(1, "server/discover", map[string]any{"_meta": mcpMeta}),
		mcpRequest(2, "tools/list", map[string]any{"_meta": mcpMeta}),
		mcpRequest(3, "tools/call", map[string]any{"_meta": mcpMeta, "name": "corvint.docs_draft", "arguments": map[string]any{"source": "owner.md", "package": "cache"}}))
	var markdown string
	if err == nil {
		err = requireMCPDiscovery(draftResponses, 0, inventory.name("corvint-docs-mcp"), "0.1.0-experimental")
	}
	if err == nil {
		err = requireMCPTools(draftResponses, 1, []string{"corvint.docs_consume", "corvint.docs_draft"})
	}
	if err == nil {
		markdown, err = mcpStructuredString(draftResponses, 2, "markdown")
	}
	if err == nil {
		consume, runErr := runMCP(ctx, filepath.Join(extracted, "bin", inventory.name("corvint-docs-mcp")), docsRoot, scratch,
			mcpRequest(4, "tools/call", map[string]any{"_meta": mcpMeta, "name": "corvint.docs_consume", "arguments": map[string]any{"source": "owner.md", "package": "cache", "task": "Split", "draft": markdown}}))
		if runErr != nil {
			err = runErr
		} else {
			err = requireMCPSuccess(consume, 0)
		}
	}
	steps = append(steps, step("corvint-docs-mcp-draft-consume", err, "exact draft bytes consumed"))
	if err != nil {
		return steps, err
	}

	tv, err := runMCP(ctx, filepath.Join(extracted, "bin", inventory.name("corvint-test-validity-mcp")), repo, scratch,
		mcpRequest(1, "server/discover", map[string]any{"_meta": mcpMeta}),
		mcpRequest(2, "tools/list", map[string]any{"_meta": mcpMeta}))
	if err == nil {
		err = requireMCPDiscovery(tv, 0, inventory.name("corvint-test-validity-mcp"), "0.1.0-experimental")
	}
	if err == nil {
		err = requireMCPTools(tv, 1, []string{"corvint.test_validity"})
	}
	steps = append(steps, step("corvint-test-validity-mcp-list", err, "server/discover, one read-only tool; retained evidence discovery deferred to installed qualification"))
	if err != nil {
		return steps, err
	}

	js := filepath.Join(extracted, "bin", inventory.name("corvint-js-test-provider"))
	_, stderr, jsErr := runExpectedExit(ctx, scratch, minimalRunEnv(scratch), subprocessTimeout, 2, js)
	if jsErr == nil && strings.TrimSpace(string(stderr)) != "usage: "+inventory.name("corvint-js-test-provider")+" <unit|e2e> [flags]" {
		jsErr = fmt.Errorf("unexpected help admission: %q", stderr)
	}
	steps = append(steps, step("corvint-js-test-provider-help", jsErr, "exit 2 usage admission; execution deferred to installed VSIX qualification"))
	if jsErr != nil {
		return steps, jsErr
	}

	goProvider := filepath.Join(extracted, "bin", inventory.name("corvint-go-test-provider"))
	stdout, _, goErr := runExpectedExit(ctx, scratch, minimalRunEnv(scratch), subprocessTimeout, 2, goProvider)
	// With no arguments, the opt-in provider reports EXPERIMENT_DISABLED through GLTP's hashed detail.
	wantDisabled := `{"code":"IDENTITY_MISMATCH","detailSha256":"eda00f0c506e64f214dd1021539495fb129548eb79d8577d91b9cc94262664e5","phase":"IDENTITY","profile":"go-live-error/0","runId":null}` + "\n"
	if goErr == nil && string(stdout) != wantDisabled {
		goErr = fmt.Errorf("missing EXPERIMENT_DISABLED diagnostic")
	}
	steps = append(steps, step("corvint-go-test-provider-help", goErr, "exit 2 typed admission; foreground session deferred to installed VSIX qualification"))
	return steps, goErr
}

func requireMCPDiscovery(responses []map[string]any, index int, name, version string) error {
	if index >= len(responses) {
		return fmt.Errorf("missing discovery response")
	}
	result, ok := responses[index]["result"].(map[string]any)
	if !ok {
		return fmt.Errorf("discovery error: %v", responses[index])
	}
	versions, _ := result["supportedVersions"].([]any)
	if len(versions) != 1 || versions[0] != "2026-07-28" {
		return fmt.Errorf("discovery protocols %v", versions)
	}
	meta, _ := result["_meta"].(map[string]any)
	info, _ := meta["io.modelcontextprotocol/serverInfo"].(map[string]any)
	if info["name"] != name || info["version"] != version {
		return fmt.Errorf("discovery server identity %v", info)
	}
	capabilities, _ := result["capabilities"].(map[string]any)
	if len(capabilities) != 1 || capabilities["tools"] == nil {
		return fmt.Errorf("discovery capabilities %v", capabilities)
	}
	if result["resultType"] != "complete" || result["cacheScope"] != "public" {
		return fmt.Errorf("discovery envelope %v", result)
	}
	if _, ok := result["ttlMs"].(float64); !ok {
		return fmt.Errorf("discovery ttl missing")
	}
	return nil
}

func mcpRequest(id int, method string, params map[string]any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}
}

func runMCP(ctx context.Context, bin, root, scratch string, requests ...map[string]any) ([]map[string]any, error) {
	var responses []map[string]any
	seenIDs := make(map[string]bool, len(requests))
	for _, request := range requests {
		raw, err := json.Marshal(request)
		if err != nil {
			return nil, err
		}
		out, _, err := runCapturedStdin(ctx, scratch, minimalRunEnv(scratch), subprocessTimeout, append(raw, '\n'), bin, "--root", root)
		if err != nil {
			return nil, err
		}
		requestID := fmt.Sprint(request["id"])
		if requestID == "<nil>" || seenIDs[requestID] {
			return nil, fmt.Errorf("MCP request IDs are absent or duplicate")
		}
		seenIDs[requestID] = true
		response, err := decodeMCPResponse(out, request["id"])
		if err != nil {
			return nil, err
		}
		responses = append(responses, response)
	}
	if len(responses) != len(requests) {
		return nil, fmt.Errorf("MCP returned %d responses, want %d", len(responses), len(requests))
	}
	return responses, nil
}

func decodeMCPResponse(out []byte, requestID any) (map[string]any, error) {
	if len(out) == 0 || out[len(out)-1] != '\n' || bytes.Count(out, []byte{'\n'}) != 1 {
		return nil, fmt.Errorf("MCP emitted non-single-line envelope")
	}
	frame := out[:len(out)-1]
	if len(frame) > protocol.MaxMessageBytes || !utf8.Valid(frame) {
		return nil, fmt.Errorf("MCP emitted invalid or oversized UTF-8 envelope")
	}
	var response map[string]any
	if err := json.Unmarshal(frame, &response); err != nil {
		return nil, err
	}
	expectedID, err := json.Marshal(requestID)
	if err != nil {
		return nil, err
	}
	responseID, err := json.Marshal(response["id"])
	if err != nil || response["jsonrpc"] != "2.0" || !bytes.Equal(responseID, expectedID) {
		return nil, fmt.Errorf("MCP response identity mismatch: %v", response)
	}
	return response, nil
}

func requireMCPTools(responses []map[string]any, index int, want []string) error {
	if index >= len(responses) {
		return fmt.Errorf("missing tools/list response")
	}
	result, ok := responses[index]["result"].(map[string]any)
	if !ok {
		return fmt.Errorf("tools/list error: %v", responses[index])
	}
	items, ok := result["tools"].([]any)
	if !ok {
		return fmt.Errorf("tools/list has no tools")
	}
	var got []string
	for _, item := range items {
		object, _ := item.(map[string]any)
		name, _ := object["name"].(string)
		got = append(got, name)
	}
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("tools %v, want %v", got, want)
	}
	return nil
}

func requireMCPSuccess(responses []map[string]any, index int) error {
	if index >= len(responses) {
		return fmt.Errorf("missing MCP response")
	}
	result, ok := responses[index]["result"].(map[string]any)
	if !ok || result["isError"] == true {
		return fmt.Errorf("MCP call failed: %v", responses[index])
	}
	return nil
}

func mcpStructuredString(responses []map[string]any, index int, key string) (string, error) {
	if err := requireMCPSuccess(responses, index); err != nil {
		return "", err
	}
	result := responses[index]["result"].(map[string]any)
	structured, _ := result["structuredContent"].(map[string]any)
	value, _ := structured[key].(string)
	if value == "" {
		return "", fmt.Errorf("MCP result lacks %s", key)
	}
	return value, nil
}

func initDocsSmokeRepo(ctx context.Context, root string) error {
	files := map[string]string{"go.mod": "module example.test/companion-docs\n\ngo 1.27.1\n", "cache/demux.go": "package cache\n\nfunc Split(key string) string { return key }\n", "owner.md": "# Cache\nDelivery status: experimental\n\n## Agent digest\n- Claim: splits keys\n- Blocked on: behavior validation\n\n## Requirements\nNever promote drafts.\n"}
	if err := writeFiles(root, files, 0o600); err != nil {
		return err
	}
	gitPath, err := lookGit()
	if err != nil {
		return err
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=Corvint Smoke", "-c", "user.email=smoke@corvint.invalid", "commit", "-qm", "fixture"}} {
		if _, _, err := runCaptured(ctx, root, closedGitEnv(root), subprocessTimeout, append([]string{gitPath}, args...)...); err != nil {
			return err
		}
	}
	return nil
}

func bindSmokeEvidence(steps []SmokeStep, bundleDigest string, components []ComponentManifest, extracted string, inventory bundleInventory) {
	byName := make(map[string]ComponentManifest, len(components))
	for _, component := range components {
		byName[component.Name] = component
	}
	for i := range steps {
		name := ""
		switch {
		case strings.HasPrefix(steps[i].Name, "atm-"):
			name = "atm"
		case strings.HasPrefix(steps[i].Name, "console-"):
			name = "corvint-console"
		case strings.HasPrefix(steps[i].Name, "corvint-dashboard-snapshot"):
			name = "corvint-dashboard-snapshot"
		case strings.HasPrefix(steps[i].Name, "corvint-docs-mcp"):
			name = "corvint-docs-mcp"
		case strings.HasPrefix(steps[i].Name, "corvint-test-validity-mcp"):
			name = "corvint-test-validity-mcp"
		case strings.HasPrefix(steps[i].Name, "corvint-js-test-provider"):
			name = "corvint-js-test-provider"
		case strings.HasPrefix(steps[i].Name, "corvint-go-test-provider"):
			name = "corvint-go-test-provider"
		case strings.HasPrefix(steps[i].Name, "corvint-mcp"):
			name = "corvint-mcp"
		case strings.HasPrefix(steps[i].Name, "corvint-version"),
			strings.HasPrefix(steps[i].Name, "corvint-affected-selection"),
			strings.HasPrefix(steps[i].Name, "corvint-playwright-external-discovery"),
			strings.HasPrefix(steps[i].Name, "corvint-documentation-corpus-discovery"),
			strings.HasPrefix(steps[i].Name, "corvint-work-queue-observation"):
			name = "corvint"
		}
		component, ok := byName[inventory.name(name)]
		steps[i].BundleSHA256 = bundleDigest
		if ok {
			steps[i].ComponentSHA256 = component.BinarySHA256
			steps[i].SourceCommit = component.Commit
			steps[i].SourceTree = component.Tree
			steps[i].InvokedPath = filepath.Join(extracted, component.BinaryPath)
		}
	}
}
