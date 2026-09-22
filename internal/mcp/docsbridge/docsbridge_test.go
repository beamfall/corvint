package docsbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

const ownerMarkdown = "# Cache\nDelivery status: experimental\n\n## Agent digest\n- Claim: splits keys\n- Blocked on: behavior validation\n\n## Requirements\nNever promote drafts.\n"

func docsRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":         "module example.test/docsmcp\n\ngo 1.27.0\n",
		"cache/demux.go": "package cache\n\nfunc Split(key string) string { return key }\n",
		"owner.md":       ownerMarkdown,
	}
	for name, text := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commit(t, root, "fixture")
	return root
}

func commit(t *testing.T, root, message string) {
	t.Helper()
	for _, arguments := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", message}} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if _, err := os.Stat(filepath.Join(root, ".git")); arguments[0] == "init" && err == nil {
			continue
		}
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
}

func draftArguments(t *testing.T, source, pkg string) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"source": source, "package": pkg})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func consumeArguments(t *testing.T, source, pkg, task, draft string) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"source": source, "package": pkg, "task": task, "draft": draft})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestRoundTripDraftThenConsumeExactBytes(t *testing.T) {
	root := docsRepository(t)
	registry, err := New(root)
	if err != nil {
		t.Fatalf("New: %#v", err)
	}
	structured, text, toolFailure, transportErr := registry.Call(context.Background(), ToolDraft, draftArguments(t, "owner.md", "cache"))
	if transportErr != nil || toolFailure != nil {
		t.Fatalf("draft failed: transport=%#v tool=%#v", transportErr, toolFailure)
	}
	markdown, ok := structured["markdown"].(string)
	if !ok || markdown != text {
		t.Fatalf("draft structured/text mismatch: %#v vs %q", structured["markdown"], text)
	}

	structured, text, toolFailure, transportErr = registry.Call(context.Background(), ToolConsume, consumeArguments(t, "owner.md", "cache", "Split", markdown))
	if transportErr != nil || toolFailure != nil {
		t.Fatalf("consume failed: transport=%#v tool=%#v", transportErr, toolFailure)
	}
	if structured["state"] != "READY" || structured["validation"] != "SOURCE_REDERIVED" || structured["behavior"] != "UNKNOWN" {
		t.Fatalf("consume result: %#v", structured)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(text), &decoded); err != nil {
		t.Fatalf("consume text is not the canonical JSON object: %v", err)
	}
	if decoded["state"] != "READY" {
		t.Fatalf("consume text mismatch: %s", text)
	}
}

func TestCLIAndMCPAgreeOnClaimsAndProvenance(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("native platform required")
	}
	root := docsRepository(t)
	moduleRoot := findModuleRoot(t)
	cliBinary := filepath.Join(t.TempDir(), "corvint")
	build := exec.Command("go", "build", "-o", cliBinary, "./cmd/corvint")
	build.Dir = moduleRoot
	build.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build corvint: %v\n%s", err, output)
	}

	registry, err := New(root)
	if err != nil {
		t.Fatalf("New: %#v", err)
	}
	structured, mcpDraft, toolFailure, transportErr := registry.Call(context.Background(), ToolDraft, draftArguments(t, "owner.md", "cache"))
	if transportErr != nil || toolFailure != nil {
		t.Fatalf("mcp draft failed: transport=%#v tool=%#v", transportErr, toolFailure)
	}

	draftCmd := exec.Command(cliBinary, "--root", root, "docs", "draft", "--source", "owner.md", "--package", "cache")
	var cliDraft bytes.Buffer
	draftCmd.Stdout = &cliDraft
	if err := draftCmd.Run(); err != nil {
		t.Fatalf("cli draft: %v", err)
	}
	if cliDraft.String() != mcpDraft {
		t.Fatalf("draft bytes differ between CLI and MCP")
	}

	_, mcpConsumeText, toolFailure, transportErr := registry.Call(context.Background(), ToolConsume, consumeArguments(t, "owner.md", "cache", "Split", mcpDraft))
	if transportErr != nil || toolFailure != nil {
		t.Fatalf("mcp consume failed: transport=%#v tool=%#v", transportErr, toolFailure)
	}
	consumeCmd := exec.Command(cliBinary, "--root", root, "docs", "consume", "--source", "owner.md", "--package", "cache", "--task", "Split")
	consumeCmd.Stdin = bytes.NewReader([]byte(cliDraft.String()))
	var cliConsume bytes.Buffer
	consumeCmd.Stdout = &cliConsume
	if err := consumeCmd.Run(); err != nil {
		t.Fatalf("cli consume: %v", err)
	}

	var mcpObject, cliObject map[string]any
	if err := json.Unmarshal([]byte(mcpConsumeText), &mcpObject); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(cliConsume.Bytes(), &cliObject); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"state", "derivation", "validation", "behavior", "commit", "tree", "draft_sha256", "task", "results"} {
		if !reflect.DeepEqual(mcpObject[field], cliObject[field]) {
			t.Fatalf("field %q differs: mcp=%#v cli=%#v", field, mcpObject[field], cliObject[field])
		}
	}
	_ = structured
}

func findModuleRoot(t *testing.T) string {
	t.Helper()
	directory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
			return directory
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			t.Fatal("module root not found")
		}
		directory = parent
	}
}

func TestConsumeRefusesEditedSourceAfterCommit(t *testing.T) {
	root := docsRepository(t)
	registry, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	_, draft, toolFailure, transportErr := registry.Call(context.Background(), ToolDraft, draftArguments(t, "owner.md", "cache"))
	if transportErr != nil || toolFailure != nil {
		t.Fatalf("draft failed: %#v %#v", transportErr, toolFailure)
	}
	if err := os.WriteFile(filepath.Join(root, "cache", "demux.go"), []byte("package cache\n\nfunc Split(key string) string { return key + key }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commit(t, root, "edit source")
	structured, text, toolFailure, transportErr := registry.Call(context.Background(), ToolConsume, consumeArguments(t, "owner.md", "cache", "Split", draft))
	if transportErr != nil {
		t.Fatalf("unexpected transport error: %#v", transportErr)
	}
	if toolFailure == nil || toolFailure.Code != "stale-documentation-draft" {
		t.Fatalf("edited source not refused: tool=%#v structured=%#v text=%q", toolFailure, structured, text)
	}
	if structured != nil {
		t.Fatal("failure must not carry a successful structured payload")
	}
}

func TestConsumeRefusesTamperedDraft(t *testing.T) {
	root := docsRepository(t)
	registry, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	_, draft, toolFailure, transportErr := registry.Call(context.Background(), ToolDraft, draftArguments(t, "owner.md", "cache"))
	if transportErr != nil || toolFailure != nil {
		t.Fatalf("draft failed: %#v %#v", transportErr, toolFailure)
	}
	tampered := draft + "tampered"
	structured, _, toolFailure, transportErr := registry.Call(context.Background(), ToolConsume, consumeArguments(t, "owner.md", "cache", "Split", tampered))
	if transportErr != nil {
		t.Fatalf("unexpected transport error: %#v", transportErr)
	}
	if toolFailure == nil || toolFailure.Code != "stale-documentation-draft" {
		t.Fatalf("tampered draft not refused: %#v", toolFailure)
	}
	if structured != nil {
		t.Fatal("failure must not carry a successful structured payload")
	}
}

func TestInvalidPathIsRefusedAsTransportError(t *testing.T) {
	root := docsRepository(t)
	registry, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range []string{"../etc", "/etc", "cache/../../etc"} {
		structured, text, toolFailure, transportErr := registry.Call(context.Background(), ToolDraft, draftArguments(t, "owner.md", pkg))
		if transportErr == nil || transportErr.Code != "invalid-arguments" {
			t.Fatalf("package %q not refused: transport=%#v tool=%#v", pkg, transportErr, toolFailure)
		}
		if structured != nil || text != "" || toolFailure != nil {
			t.Fatalf("invalid path leaked a payload: structured=%#v text=%q tool=%#v", structured, text, toolFailure)
		}
	}
}

// TestMalformedArgumentsAreRefusedAsToolFailure locks in SDD-V0-006's split:
// only an invalid/escaping path or an unsupported tool name is a JSON-RPC
// InvalidParams protocol error (TestInvalidPathIsRefusedAsTransportError);
// malformed or unknown-member arguments must instead be an MCP tool result
// with isError: true, never a protocol error.
func TestMalformedArgumentsAreRefusedAsToolFailure(t *testing.T) {
	root := docsRepository(t)
	registry, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		tool string
		args []byte
	}{
		{"draft-wrong-type", ToolDraft, []byte(`{"source":1,"package":"cache"}`)},
		{"draft-unknown-member", ToolDraft, []byte(`{"source":"owner.md","package":"cache","extra":true}`)},
		{"consume-wrong-type", ToolConsume, []byte(`{"source":"owner.md","package":"cache","task":"Split","draft":1}`)},
		{"consume-unknown-member", ToolConsume, []byte(`{"source":"owner.md","package":"cache","task":"Split","draft":"x","extra":true}`)},
		{"consume-empty-task", ToolConsume, consumeArguments(t, "owner.md", "cache", "", "x")},
		{"consume-empty-draft", ToolConsume, consumeArguments(t, "owner.md", "cache", "Split", "")},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			structured, text, toolFailure, transportErr := registry.Call(context.Background(), testCase.tool, testCase.args)
			if transportErr != nil {
				t.Fatalf("must not be a protocol error: transport=%#v", transportErr)
			}
			if toolFailure == nil || toolFailure.Code != "invalid-arguments" {
				t.Fatalf("must be an invalid-arguments tool failure: %#v", toolFailure)
			}
			if structured != nil || text != "" {
				t.Fatalf("leaked a payload: structured=%#v text=%q", structured, text)
			}
		})
	}
}

func TestUnsupportedInputIsRefusedAsToolFailure(t *testing.T) {
	root := docsRepository(t)
	registry, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	structured, text, toolFailure, transportErr := registry.Call(context.Background(), ToolDraft, draftArguments(t, "cache/demux.go", "cache"))
	if transportErr != nil {
		t.Fatalf("unexpected transport error: %#v", transportErr)
	}
	if toolFailure == nil || toolFailure.Code != "unsupported-documentation-source" {
		t.Fatalf("non-Markdown owner source not refused: tool=%#v", toolFailure)
	}
	if structured != nil || text != "" {
		t.Fatalf("unsupported input leaked a payload: structured=%#v text=%q", structured, text)
	}
}

func TestFailuresAreNeverReportedAsSuccess(t *testing.T) {
	root := docsRepository(t)
	registry, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		tool string
		args []byte
	}{
		{"unknown-tool", "corvint.docs_unknown", draftArguments(t, "owner.md", "cache")},
		{"malformed-arguments", ToolDraft, []byte(`{"source":1,"package":"cache"}`)},
		{"unknown-member", ToolDraft, []byte(`{"source":"owner.md","package":"cache","extra":true}`)},
		{"missing-package-directory", ToolDraft, draftArguments(t, "owner.md", "missing")},
		{"empty-arguments", ToolConsume, []byte(`{}`)},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			structured, text, toolFailure, transportErr := registry.Call(context.Background(), testCase.tool, testCase.args)
			if transportErr == nil && toolFailure == nil {
				t.Fatalf("case %q reported success: structured=%#v text=%q", testCase.name, structured, text)
			}
			if structured != nil {
				t.Fatalf("case %q returned a structured payload alongside failure", testCase.name)
			}
		})
	}
}
