package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
)

func TestToolsExposeOnlyDeliveredClosedReadSurface(t *testing.T) {
	root := makeRepository(t)
	registry, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	tools := registry.Tools()
	if got := toolNames(t, tools); !reflect.DeepEqual(got, []string{ToolHelp, ToolImpact, ToolStatus}) {
		t.Fatalf("default V0 tools = %v, want exactly help (MCPV0-100), impact and status", got)
	}
	tools[1].InputSchema["type"] = "array"
	if registry.Tools()[1].InputSchema["type"] != "object" {
		t.Fatal("Tools returned shared mutable schema")
	}
	// MCPV0-026: the registry neither advertises nor dispatches omitted tools;
	// a call fails exactly like any other unknown tool.
	for _, omitted := range []string{"corvint.evidence", "corvint.dashboard-snapshot", "corvint.query", "corvint.context"} {
		if _, callErr := registry.Call(context.Background(), omitted, []byte(`{}`)); callErr == nil || callErr.Code != "unsupported-tool" {
			t.Fatalf("omitted tool %q error = %#v", omitted, callErr)
		}
	}
}

// toolNames returns the advertised names after checking each descriptor is
// closed, 2020-12, and read-only.
func toolNames(t *testing.T, tools []ToolDescriptor) []string {
	t.Helper()
	names := make([]string, len(tools))
	for index, tool := range tools {
		names[index] = tool.Name
		if tool.InputSchema["type"] != "object" || tool.InputSchema["additionalProperties"] != false {
			t.Fatalf("tool %s schema is not closed: %#v", tool.Name, tool.InputSchema)
		}
		if tool.InputSchema["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
			t.Fatalf("tool %s schema dialect = %#v", tool.Name, tool.InputSchema["$schema"])
		}
		if !tool.Annotations.ReadOnlyHint || tool.Annotations.DestructiveHint ||
			!tool.Annotations.IdempotentHint || tool.Annotations.OpenWorldHint {
			t.Fatalf("tool %s annotations = %#v", tool.Name, tool.Annotations)
		}
	}
	return names
}

func TestReadToolsRefuseConfiguredFilterWithoutExecutingIt(t *testing.T) {
	for _, tool := range []string{ToolStatus, ToolImpact} {
		t.Run(tool, func(t *testing.T) {
			root := makeRepository(t)
			marker := filepath.Join(t.TempDir(), "executed")
			writeFile(t, filepath.Join(root, ".gitattributes"), "*.go filter=hostile\n")
			gitOutput(t, root, "config", "filter.hostile.clean", "touch '"+marker+"'; cat")
			writeFile(t, filepath.Join(root, "internal", "widget", "widget.go"), "package pkg\nconst Value = 99\n")
			registry, err := New(root)
			if err != nil {
				t.Fatal(err)
			}
			args := `{}`
			if tool == ToolImpact {
				args = `{"paths":["internal/widget/widget.go"]}`
			}
			result, callErr := registry.Call(context.Background(), tool, []byte(args))
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Errorf("repository filter executed: %v", err)
			}
			if callErr == nil || callErr.Code != "repository-unavailable" {
				t.Fatalf("result=%#v error=%#v", result, callErr)
			}
		})
	}
}

func TestReadToolsRejectEffectiveWorktreeRedirectBeforeAndAfterAdmission(t *testing.T) {
	for _, late := range []bool{false, true} {
		t.Run(fmt.Sprint(late), func(t *testing.T) {
			root := makeRepository(t)
			outside := t.TempDir()
			if !late {
				gitOutput(t, root, "config", "core.worktree", outside)
			}
			registry, err := New(root)
			if err != nil {
				t.Fatal(err)
			}
			if late {
				gitOutput(t, root, "config", "core.worktree", outside)
			}
			for _, tool := range []string{ToolStatus, ToolImpact} {
				args := `{}`
				if tool == ToolImpact {
					args = `{"paths":["internal/widget/widget.go"]}`
				}
				result, callErr := registry.Call(context.Background(), tool, []byte(args))
				if callErr == nil || callErr.Code != "repository-unavailable" {
					t.Fatalf("tool=%s result=%#v error=%#v", tool, result, callErr)
				}
			}
		})
	}
}

func TestCallRejectsUnknownDuplicateAndHostileArgumentsBeforeRepositoryWork(t *testing.T) {
	registry, err := New(makeRepository(t))
	if err != nil {
		t.Fatal(err)
	}
	registry.operations.probe = func(context.Context, string) (gokernel.Repository, error) {
		t.Error("invalid arguments reached repository probe")
		return gokernel.Repository{}, errors.New("unexpected repository probe")
	}
	registry.operations.build = func(context.Context, string) (*contextindex.Index, error) {
		t.Error("invalid arguments reached repository build")
		return nil, errors.New("unexpected repository build")
	}
	tests := []struct {
		name string
		tool string
		raw  string
	}{
		{"impact absolute", ToolImpact, `{"paths":["/tmp/file.go"]}`},
		{"impact traversal", ToolImpact, `{"paths":["internal/../file.go"]}`},
		{"impact duplicate", ToolImpact, `{"paths":["internal/a.go","internal/a.go"]}`},
		{"impact non go", ToolImpact, `{"paths":["internal/a.py"]}`},
		{"impact unknown", ToolImpact, `{"paths":["internal/a.go"],"root":"/tmp"}`},
		{"impact null limit", ToolImpact, `{"paths":["internal/a.go"],"limit":null}`},
		{"status unknown", ToolStatus, `{"root":"/tmp"}`},
		{"status null", ToolStatus, `null`},
		{"status array", ToolStatus, `[]`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, callErr := registry.Call(context.Background(), test.tool, []byte(test.raw)); callErr == nil || callErr.Code != "invalid-arguments" {
				t.Fatalf("Call error = %#v", callErr)
			}
		})
	}
}

func TestReadToolsBindRepositoryAndDoNotMutateIt(t *testing.T) {
	if runtimeUnsupported() {
		t.Skip("native impact is qualified only on Darwin and Linux")
	}
	root := makeRepository(t)
	registry, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	eagerBuilds, probes := 0, 0
	operations := registry.operations
	build := operations.build
	operations.build = func(ctx context.Context, root string) (*contextindex.Index, error) {
		eagerBuilds++
		return build(ctx, root)
	}
	probe := operations.probe
	operations.probe = func(ctx context.Context, root string) (gokernel.Repository, error) {
		probes++
		return probe(ctx, root)
	}
	registry = registryWithOperations(registry, operations)
	before := gitOutput(t, root, "status", "--porcelain=v1", "-z", "--untracked-files=all")

	status, callErr := registry.Call(context.Background(), ToolStatus, []byte(`{}`))
	if callErr != nil {
		t.Fatal(callErr)
	}
	assertObservedBinding(t, status)
	if status.Receipt != nil {
		t.Fatal("status invented a native receipt")
	}
	if eagerBuilds != 0 || probes != 1 {
		t.Fatalf("status work = eager:%d probes:%d", eagerBuilds, probes)
	}
	assertCanonicalObject(t, status)

	impact, callErr := registry.Call(context.Background(), ToolImpact, []byte(`{"paths":["internal/widget/widget.go"],"limit":10}`))
	if callErr != nil {
		t.Fatal(callErr)
	}
	assertObservedBinding(t, impact)
	if impact.Receipt["mode"] != "impact" || impact.Receipt["revision"] != impact.Repository.TreeRevision {
		t.Fatalf("impact receipt is not bound to repository: %#v", impact)
	}
	if eagerBuilds != 1 || probes != 1 {
		t.Fatalf("impact work = eager:%d probes:%d; expected one impact snapshot", eagerBuilds, probes)
	}
	assertCanonicalObject(t, impact)

	after := gitOutput(t, root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if !bytes.Equal(before, after) {
		t.Fatalf("read tools mutated repository: before=%q after=%q", before, after)
	}
}

func registryWithOperations(registry *Registry, operations repositoryOperations) *Registry {
	copy := *registry
	copy.operations = operations
	return &copy
}

func TestImpactOutOfScopePreservesNativeReceiptInAbstention(t *testing.T) {
	if runtimeUnsupported() {
		t.Skip("native impact is qualified only on Darwin and Linux")
	}
	root := makeRepository(t)
	writeFile(t, filepath.Join(root, "generated/value.go"), "package generated\n")
	gitOutput(t, root, "add", "generated/value.go")
	gitOutput(t, root, "commit", "-q", "-m", "generated fixture")
	registry, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	result, callErr := registry.Call(context.Background(), ToolImpact, []byte(`{"paths":["generated/value.go"]}`))
	if callErr != nil {
		t.Fatal(callErr)
	}
	if result.State != "ABSTAINED" || result.EpistemicClass != "NOT_OBSERVED" ||
		result.AuthorityClass != "NONE" || !result.Abstention.Active || result.Abstention.Reason != "OUT_OF_SCOPE" ||
		result.Receipt == nil || result.Receipt["state"] != "OUT_OF_SCOPE" {
		t.Fatalf("out-of-scope result = %#v", result)
	}
	assertCanonicalObject(t, result)
}

func TestCancelledCallFailsClosed(t *testing.T) {
	registry, err := New(makeRepository(t))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, callErr := registry.Call(ctx, ToolStatus, []byte(`{}`)); callErr == nil || callErr.Code != "cancelled" {
		t.Fatalf("cancelled call error = %#v", callErr)
	}
}

func assertObservedBinding(t *testing.T, result Result) {
	t.Helper()
	if result.Schema != resultSchema || result.Mutates || result.Repository == nil || result.Repository.CommitRevision == "" ||
		result.Repository.TreeRevision == "" || result.Repository.DirtyPathsSHA256 == "" || result.Abstention.Active || result.Abstention.Reason != "NONE" {
		t.Fatalf("invalid observed result: %#v", result)
	}
}

func assertCanonicalObject(t *testing.T, result Result) {
	t.Helper()
	object, err := result.Object()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := result.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 || raw[len(raw)-1] == '\n' || !reflect.DeepEqual(object["receipt"], result.Receipt) {
		t.Fatalf("invalid canonical object: raw=%q object=%#v", raw, object)
	}
}

func TestInvalidResultCannotCrossTransportBoundary(t *testing.T) {
	result := Result{Schema: resultSchema, Tool: ToolStatus, Mutates: false}
	if _, err := result.Object(); err == nil || err.Code != "internal-error" {
		t.Fatalf("invalid result Object error = %#v", err)
	}
	if _, err := result.CanonicalJSON(); err == nil || err.Code != "internal-error" {
		t.Fatalf("invalid result CanonicalJSON error = %#v", err)
	}
}

func TestWorstCaseEscapingAbstainsBeforeMCPFrameBudget(t *testing.T) {
	binding := RepositoryBinding{
		CommitRevision: strings.Repeat("a", 40), TreeRevision: strings.Repeat("b", 40),
		ObjectFormat: "sha1", ProfileID: "generic", WorktreeState: "CLEAN",
		DirtyPathsSHA256: strings.Repeat("c", 64),
	}
	result, err := boundedObserved(ToolImpact, binding, map[string]any{
		"mode": "impact", "revision": binding.TreeRevision, "payload": strings.Repeat(`"\`, 200*1024),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.State != "ABSTAINED" || result.Abstention.Reason != "OUTPUT_BUDGET_EXCEEDED" || result.Receipt != nil {
		t.Fatalf("oversized result = %#v", result)
	}
	object, objectErr := result.Object()
	if objectErr != nil {
		t.Fatal(objectErr)
	}
	text, textErr := result.CanonicalJSON()
	if textErr != nil {
		t.Fatal(textErr)
	}
	frame, marshalErr := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1,
		"result": map[string]any{
			"content":           []any{map[string]any{"type": "text", "text": string(text)}},
			"structuredContent": object, "isError": false, "resultType": "complete",
		},
	})
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	if len(frame) > 1<<20 {
		t.Fatalf("abstention frame bytes = %d", len(frame))
	}
}

func TestSub384KiBWorstCaseHTMLEscapingStillAbstains(t *testing.T) {
	binding := RepositoryBinding{
		CommitRevision: strings.Repeat("a", 40), TreeRevision: strings.Repeat("b", 40),
		ObjectFormat: "sha1", ProfileID: "generic", WorktreeState: "CLEAN",
		DirtyPathsSHA256: strings.Repeat("c", 64),
	}
	receipt := map[string]any{
		"mode": "impact", "revision": binding.TreeRevision, "payload": strings.Repeat("<", 90*1024),
	}
	unbounded := observed(ToolImpact, binding, receipt)
	canonical, canonicalErr := unbounded.CanonicalJSON()
	if canonicalErr != nil {
		t.Fatal(canonicalErr)
	}
	if len(canonical) >= maxResultBytes || fitsMCPFrame(canonical, unbounded) {
		t.Fatalf("fixture does not isolate duplicate-frame expansion: canonical=%d fits=%v", len(canonical), fitsMCPFrame(canonical, unbounded))
	}
	result, err := boundedObserved(ToolImpact, binding, receipt)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != "ABSTAINED" || result.Abstention.Reason != "OUTPUT_BUDGET_EXCEEDED" {
		t.Fatalf("expanded result = %#v", result)
	}
}

func TestNewRequiresCanonicalRepositoryRoot(t *testing.T) {
	if _, err := New(t.TempDir()); bridgeCode(err) != "invalid-root" {
		t.Fatalf("non-repository root error = %#v", err)
	}
	root := t.TempDir()
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(root, ".git")); err != nil {
		t.Fatal(err)
	}
	if _, err := New(root); bridgeCode(err) != "invalid-root" {
		t.Fatalf("symlink Git marker error = %#v", err)
	}
}

func TestNewRejectsRewritableGitControlFile(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, ".git")
	if err := os.WriteFile(marker, []byte("gitdir: /first/target\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(root); bridgeCode(err) != "invalid-root" {
		t.Fatalf("regular Git control file construction error = %#v", err)
	}
	if err := os.WriteFile(marker, []byte("gitdir: /replacement/target\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(root); bridgeCode(err) != "invalid-root" {
		t.Fatalf("in-place retargeted Git control file error = %#v", err)
	}
}

func TestRootReplacementAbstainsInsteadOfRebinding(t *testing.T) {
	root := makeRepository(t)
	registry, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	moved := root + ".moved"
	if err := os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(moved) })
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	result, callErr := registry.Call(context.Background(), ToolStatus, []byte(`{}`))
	if callErr != nil {
		t.Fatal(callErr)
	}
	if result.State != "ABSTAINED" || result.Repository != nil ||
		result.Abstention.Reason != "ROOT_IDENTITY_CHANGED" || result.EpistemicClass != "NOT_OBSERVED" {
		t.Fatalf("replacement root result = %#v", result)
	}
}

func TestGitMarkerReplacementAbstainsInsteadOfRebinding(t *testing.T) {
	root := makeRepository(t)
	registry, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, ".git")
	moved := filepath.Join(root, ".git.moved")
	if err := os.Rename(marker, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(marker, 0o755); err != nil {
		t.Fatal(err)
	}
	result, callErr := registry.Call(context.Background(), ToolStatus, []byte(`{}`))
	if callErr != nil {
		t.Fatal(callErr)
	}
	if result.State != "ABSTAINED" || result.Repository != nil || result.Abstention.Reason != "ROOT_IDENTITY_CHANGED" {
		t.Fatalf("replacement Git marker result = %#v", result)
	}
}

func runtimeUnsupported() bool {
	return runtime.GOOS != "darwin" && runtime.GOOS != "linux"
}

func makeRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/repository\n\ngo 1.27.0\n")
	writeFile(t, filepath.Join(root, "AGENTS.md"), "# Agent instructions\n\nUse roadmap tickets and workflow gates for contributor orientation.\n")
	writeFile(t, filepath.Join(root, "internal/widget/widget.go"), "package widget\n\nfunc Value() int { return 1 }\n")
	writeFile(t, filepath.Join(root, "internal/widget/widget_test.go"), "package widget\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) { if Value() != 1 { t.Fail() } }\n")
	gitOutput(t, root, "init", "-q")
	gitOutput(t, root, "config", "user.name", "Corvint Test")
	gitOutput(t, root, "config", "user.email", "corvint@example.invalid")
	gitOutput(t, root, "add", "AGENTS.md", "go.mod", "internal/widget/widget.go", "internal/widget/widget_test.go")
	gitOutput(t, root, "commit", "-q", "-m", "fixture")
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func gitOutput(t *testing.T, root string, arguments ...string) []byte {
	t.Helper()
	args := append([]string{"-C", root}, arguments...)
	command := exec.Command("git", args...)
	output, err := command.Output()
	if err != nil {
		t.Fatalf("git %v: %v", arguments, err)
	}
	return output
}
