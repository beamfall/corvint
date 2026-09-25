package mcp20260728

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// reasonClassArguments selects the opt-in tool-error profile
// corvint-mcp-2026-07-28-conformance/2 (MCPV0-027, decision 0383).
var reasonClassArguments = []string{"--error-profile", "reason-class"}

var reasonClasses = []string{
	"git-filter", "config-include", "attributes-file", "ref-storage", "worktree-config", "config-malformed",
	"submodule", "split-index", "gitdir-pointer", "metadata-unreadable", "metadata-limit", "metadata-directory",
	"metadata-drift", "scratch-dir", "root-unresolved", "unclassified",
}

func TestReasonClassCaseInventoryIsClosed(t *testing.T) {
	type manifest struct {
		Profile           string   `json:"profile"`
		ParentProfile     string   `json:"parentProfile"`
		Selector          []string `json:"selector"`
		ToolErrorProfiles []string `json:"toolErrorProfiles"`
		ReasonClasses     []string `json:"reasonClasses"`
		Cases             []string `json:"cases"`
	}
	raw, err := os.ReadFile(filepath.Join(moduleRoot(t), "conformance", "mcp-2026-07-28", "cases-reason-class.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got manifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&got); err != nil {
		t.Fatal(err)
	}
	wantCases := []string{"selector-closed", "default-tool-error-unchanged", "reason-class-tool-error", "unclassified-tool-error"}
	if got.Profile != "corvint-mcp-2026-07-28-conformance/2" || got.ParentProfile != "corvint-mcp-2026-07-28-conformance/0" ||
		!reflect.DeepEqual(got.Selector, reasonClassArguments) ||
		!reflect.DeepEqual(got.ToolErrorProfiles, []string{"corvint-mcp-tool-error/0", "corvint-mcp-tool-error/1"}) ||
		!reflect.DeepEqual(got.ReasonClasses, reasonClasses) || !reflect.DeepEqual(got.Cases, wantCases) {
		t.Fatalf("invalid reason-class conformance manifest: %#v", got)
	}
}

// MCPV0-027: the selector is closed under the MCPV0-026 rules and fails
// before repository startup.
func TestReasonClassSelectorIsClosed(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	for _, arguments := range [][]string{
		{"--root", missing, "--error-profile"},
		{"--root", missing, "--error-profile", "default"},
		{"--root", missing, "--error-profile", "REASON-CLASS"},
		{"--root", missing, "--error-profile", "reason-class", "--error-profile", "reason-class"},
		{"--root", missing, "--error-profile=reason-class"},
		{"--version", "--error-profile", "reason-class"},
	} {
		command := exec.Command(serverBinary, arguments...)
		var stdout, stderr strings.Builder
		command.Stdout, command.Stderr = &stdout, &stderr
		var exitErr *exec.ExitError
		if err := command.Run(); !errors.As(err, &exitErr) || exitErr.ExitCode() != 2 ||
			stdout.Len() != 0 || stderr.String() != "corvint-mcp: invalid arguments\n" {
			t.Fatalf("%q: err=%v stdout=%q stderr=%q", arguments, err, stdout.String(), stderr.String())
		}
	}
}

// MCPV0-028: over the executable-config and worktree-redirect fixtures, the
// default server returns exactly the /0 object and the selector returns
// exactly the /1 object with the typed class; nothing else differs.
func TestReasonClassToolErrorOverRefusedRepositories(t *testing.T) {
	classes := map[string]string{
		"clean": "git-filter", "process": "git-filter",
		"worktree-before": "worktree-config", "worktree-after": "worktree-config", "worktree-config": "worktree-config",
	}
	for _, kind := range []string{"clean", "process", "worktree-before", "worktree-after", "worktree-config"} {
		t.Run(kind, func(t *testing.T) {
			root := fixtureRepository(t)
			outside := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("*.go filter=hostile\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "clean", "process":
				gitRun(t, root, "config", "filter.hostile."+kind, "cat")
			case "worktree-before":
				gitRun(t, root, "config", "core.worktree", outside)
			case "worktree-config":
				gitRun(t, root, "config", "extensions.worktreeConfig", "true")
				gitRun(t, root, "config", "--worktree", "core.worktree", outside)
			}
			defaultClient := startServer(t, root)
			defer defaultClient.close(t)
			classClient := startServerWithArguments(t, root, reasonClassArguments)
			defer classClient.close(t)
			if kind == "worktree-after" {
				successResult(t, defaultClient.call(t, 1, "tools/list", map[string]any{"_meta": requestMeta()}))
				successResult(t, classClient.call(t, 1, "tools/list", map[string]any{"_meta": requestMeta()}))
				gitRun(t, root, "config", "core.worktree", outside)
			}
			for id, tool := range []string{"corvint.status", "corvint.impact", "corvint.query"} {
				assertToolError(t, defaultClient, id+10, tool, readToolArguments(t, root, tool), "")
				assertToolError(t, classClient, id+10, tool, readToolArguments(t, root, tool), classes[kind])
			}
		})
	}
}

// MCPV0-028: a failure the status refusal did not classify, here Git's own
// index probe over a split index, is "unclassified" under the selector.
func TestReasonClassUnclassifiedToolError(t *testing.T) {
	root := fixtureRepository(t)
	gitRun(t, root, "update-index", "--split-index")
	defaultClient := startServer(t, root)
	defer defaultClient.close(t)
	classClient := startServerWithArguments(t, root, reasonClassArguments)
	defer classClient.close(t)
	assertToolError(t, defaultClient, 10, "corvint.status", map[string]any{}, "")
	assertToolError(t, classClient, 10, "corvint.status", map[string]any{}, "unclassified")
}

// assertToolError requires the exact tool-error object in both content
// forms: /0 when reasonClass is empty, else /1 carrying that class.
func assertToolError(t *testing.T, client *stdioClient, id int, tool string, arguments map[string]any, reasonClass string) {
	t.Helper()
	want := map[string]any{
		"abstention": map[string]any{"active": true, "reason": "OPERATION_FAILED"},
		"code":       "repository-unavailable", "mutates": false, "profile": "corvint-mcp-tool-error/0", "tool": tool,
	}
	if reasonClass != "" {
		want["profile"], want["reasonClass"] = "corvint-mcp-tool-error/1", reasonClass
	}
	result := successResult(t, client.call(t, id, "tools/call", map[string]any{"_meta": requestMeta(), "name": tool, "arguments": arguments}))
	content, _ := result["content"].([]any)
	if result["isError"] != true || !reflect.DeepEqual(result["structuredContent"], want) || len(content) != 1 ||
		!reflect.DeepEqual(object(t, content[0]), map[string]any{"type": "text", "text": canonicalJSON(want)}) {
		t.Fatalf("%s tool error=%s want %s", tool, canonicalJSON(result), canonicalJSON(want))
	}
}
