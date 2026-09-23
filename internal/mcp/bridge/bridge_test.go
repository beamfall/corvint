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

	"github.com/Beamfall/corvint/internal/cem/workflow"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
)

func TestToolsExposeOnlyDeliveredClosedReadSurface(t *testing.T) {
	registry, err := New(makeRepository(t))
	if err != nil {
		t.Fatal(err)
	}
	tools := registry.Tools()
	want := []string{ToolCEMReport, ToolContext, ToolImpact, ToolQuery, ToolStatus}
	got := make([]string, len(tools))
	for index, tool := range tools {
		got[index] = tool.Name
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
	querySchema := tools[3].InputSchema["properties"].(map[string]any)["task"].(map[string]any)
	if querySchema["pattern"] != `^[ -~]*[!-~][ -~]*$` {
		t.Fatalf("query task schema admits runtime-invalid whitespace: %#v", querySchema)
	}
	contextTask := tools[1].InputSchema["properties"].(map[string]any)["task"].(map[string]any)
	cemMap := tools[0].InputSchema["properties"].(map[string]any)["map"].(map[string]any)
	if contextTask["maxLength"] != 8000 || contextTask["pattern"] != `[^\s\x85]` || cemMap["maxLength"] != 128 {
		t.Fatalf("schema admits runtime-invalid arguments: task=%#v map=%#v", contextTask, cemMap)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tools = %v, want %v", got, want)
	}
	tools[0].InputSchema["type"] = "array"
	if registry.Tools()[0].InputSchema["type"] != "object" {
		t.Fatal("Tools returned shared mutable schema")
	}
	for _, omitted := range []string{"corvint.evidence", "corvint.dashboard-snapshot"} {
		if _, callErr := registry.Call(context.Background(), omitted, []byte(`{}`)); callErr == nil || callErr.Code != "unsupported-tool" {
			t.Fatalf("omitted tool %q error = %#v", omitted, callErr)
		}
	}
}

func TestReadToolsRefuseConfiguredFilterWithoutExecutingIt(t *testing.T) {
	for _, tool := range []string{ToolStatus, ToolImpact, ToolQuery, ToolContext, ToolCEMReport} {
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
			if tool == ToolQuery || tool == ToolContext {
				args = `{"task":"orient contributor roadmap ticket workflow"}`
			}
			if tool == ToolCEMReport {
				args = cemArguments(t, root)
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
			for _, tool := range []string{ToolStatus, ToolImpact, ToolQuery, ToolContext, ToolCEMReport} {
				args := `{}`
				if tool == ToolImpact {
					args = `{"paths":["internal/widget/widget.go"]}`
				}
				if tool == ToolQuery || tool == ToolContext {
					args = `{"task":"orient contributor roadmap ticket workflow"}`
				}
				if tool == ToolCEMReport {
					args = cemArguments(t, root)
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
	registry.operations.buildQuery = func(context.Context, string, string) (*contextindex.Index, error) {
		t.Error("invalid arguments reached query build")
		return nil, errors.New("unexpected query build")
	}
	registry.operations.context = func(context.Context, string, contextInput) (*contextindex.Index, map[string]any, error) {
		t.Error("invalid arguments reached context compilation")
		return nil, nil, errors.New("unexpected context compilation")
	}
	registry.operations.cemReport = func(context.Context, string, workflow.ReadOptions) (map[string]any, error) {
		t.Error("invalid arguments reached the CEM read")
		return nil, errors.New("unexpected CEM read")
	}
	oid := strings.Repeat("a", 40)
	cem := func(mapPath, extra string) string {
		return `{"map":` + mapPath + `,"expectedBase":"` + oid + `","target":"` + oid + `"` + extra + `}`
	}
	tests := []struct {
		name string
		tool string
		raw  string
	}{
		{"query unknown", ToolQuery, `{"task":"roadmap workflow","extra":true}`},
		{"query duplicate", ToolQuery, `{"task":"roadmap workflow","task":"roadmap ticket"}`},
		{"query non ascii", ToolQuery, `{"task":"roadmap café workflow"}`},
		{"query whitespace", ToolQuery, `{"task":"   "}`},
		{"impact absolute", ToolImpact, `{"paths":["/tmp/file.go"]}`},
		{"impact traversal", ToolImpact, `{"paths":["internal/../file.go"]}`},
		{"impact duplicate", ToolImpact, `{"paths":["internal/a.go","internal/a.go"]}`},
		{"impact non go", ToolImpact, `{"paths":["internal/a.py"]}`},
		{"impact unknown", ToolImpact, `{"paths":["internal/a.go"],"root":"/tmp"}`},
		{"impact null limit", ToolImpact, `{"paths":["internal/a.go"],"limit":null}`},
		{"status unknown", ToolStatus, `{"root":"/tmp"}`},
		{"status null", ToolStatus, `null`},
		{"status array", ToolStatus, `[]`},
		{"context unknown", ToolContext, `{"task":"change widget","root":"/tmp"}`},
		{"context blank", ToolContext, `{"task":" \t "}`},
		{"context next line", ToolContext, `{"task":"\u0085"}`},
		{"context oversized", ToolContext, `{"task":"` + strings.Repeat("x", maxContextTaskBytes+1) + `"}`},
		{"context limit zero", ToolContext, `{"task":"change widget","limit":0}`},
		{"context limit high", ToolContext, `{"task":"change widget","limit":51}`},
		{"context null limit", ToolContext, `{"task":"change widget","limit":null}`},
		{"context absolute subject", ToolContext, `{"task":"change widget","subject":"/etc/passwd"}`},
		{"context traversal subject", ToolContext, `{"task":"change widget","subject":"internal/../../x.go"}`},
		{"context empty segment", ToolContext, `{"task":"change widget","subject":"internal//x.go"}`},
		{"context backslash", ToolContext, `{"task":"change widget","subject":"internal\\x.go"}`},
		{"cem traversal", ToolCEMReport, cem(`"../outside.cem.json"`, ``)},
		{"cem absolute", ToolCEMReport, cem(`"/tmp/change.cem.json"`, ``)},
		{"cem dot segment", ToolCEMReport, cem(`"a/./change.cem.json"`, ``)},
		{"cem git dir", ToolCEMReport, cem(`".git/config"`, ``)},
		{"cem folded git dir", ToolCEMReport, cem(`"sub/.GIT/change.cem.json"`, ``)},
		{"cem control byte", ToolCEMReport, cem(`"a\u0001.json"`, ``)},
		{"cem overlong", ToolCEMReport, cem(`"`+strings.Repeat("a", 513)+`"`, ``)},
		{"cem symbolic base", ToolCEMReport, `{"map":"m.cem.json","expectedBase":"HEAD","target":"` + oid + `"}`},
		{"cem uppercase target", ToolCEMReport, `{"map":"m.cem.json","expectedBase":"` + oid + `","target":"` + strings.ToUpper(oid) + `"}`},
		{"cem negative ceiling", ToolCEMReport, cem(`"m.cem.json"`, `,"maxUnknown":-1`)},
		{"cem patch member", ToolCEMReport, cem(`"m.cem.json"`, `,"patch":"x.patch"`)},
		{"cem output member", ToolCEMReport, cem(`"m.cem.json"`, `,"output":"r.md"`)},
		{"cem missing target", ToolCEMReport, `{"map":"m.cem.json","expectedBase":"` + oid + `"}`},
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
		t.Skip("native query and impact are qualified only on Darwin and Linux")
	}
	root := makeRepository(t)
	registry, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	eagerBuilds, queryBuilds, probes := 0, 0, 0
	operations := registry.operations
	build := operations.build
	operations.build = func(ctx context.Context, root string) (*contextindex.Index, error) {
		eagerBuilds++
		return build(ctx, root)
	}
	buildQuery := operations.buildQuery
	operations.buildQuery = func(ctx context.Context, root, task string) (*contextindex.Index, error) {
		queryBuilds++
		return buildQuery(ctx, root, task)
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
	if eagerBuilds != 0 || queryBuilds != 0 || probes != 1 {
		t.Fatalf("status work = eager:%d query:%d probes:%d", eagerBuilds, queryBuilds, probes)
	}
	assertCanonicalObject(t, status)

	query, callErr := registry.Call(context.Background(), ToolQuery, []byte(`{"task":"orient contributor roadmap ticket workflow"}`))
	if callErr != nil {
		t.Fatal(callErr)
	}
	assertObservedBinding(t, query)
	if query.Receipt["mode"] != "query" || query.Receipt["revision"] != query.Repository.TreeRevision {
		t.Fatalf("query receipt is not bound to repository: %#v", query)
	}
	if eagerBuilds != 0 || queryBuilds != 1 || probes != 1 {
		t.Fatalf("query work = eager:%d query:%d probes:%d; expected one query snapshot", eagerBuilds, queryBuilds, probes)
	}
	assertCanonicalObject(t, query)

	impact, callErr := registry.Call(context.Background(), ToolImpact, []byte(`{"paths":["internal/widget/widget.go"],"limit":10}`))
	if callErr != nil {
		t.Fatal(callErr)
	}
	assertObservedBinding(t, impact)
	if impact.Receipt["mode"] != "impact" || impact.Receipt["revision"] != impact.Repository.TreeRevision {
		t.Fatalf("impact receipt is not bound to repository: %#v", impact)
	}
	if eagerBuilds != 1 || queryBuilds != 1 || probes != 1 {
		t.Fatalf("impact work = eager:%d query:%d probes:%d; expected one impact snapshot", eagerBuilds, queryBuilds, probes)
	}
	assertCanonicalObject(t, impact)

	after := gitOutput(t, root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if !bytes.Equal(before, after) {
		t.Fatalf("read tools mutated repository: before=%q after=%q", before, after)
	}
}

func TestQueryRejectedProfilesAvoidIndexBuild(t *testing.T) {
	registry, err := New(makeRepository(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, platform, arguments, reason string
		failure                           string
	}{
		{"unknown option", "darwin", `{"task":"roadmap workflow","extra":true}`, "", "invalid-arguments"},
		{"non-ascii", "darwin", `{"task":"roadmap café workflow"}`, "", "invalid-arguments"},
		{"empty", "darwin", `{"task":"   "}`, "", "invalid-arguments"},
		{"unsupported intent", "darwin", `{"task":"explain image codec"}`, "UNSUPPORTED_QUERY_INTENT", ""},
		{"platform", "windows", `{"task":"orient contributor roadmap ticket workflow"}`, "UNSUPPORTED_PLATFORM", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			builds := 0
			operations := registry.operations
			operations.platform = test.platform
			operations.buildQuery = func(context.Context, string, string) (*contextindex.Index, error) {
				builds++
				return nil, errors.New("unexpected context index build")
			}
			isolated := registryWithOperations(registry, operations)
			result, callErr := isolated.Call(context.Background(), ToolQuery, []byte(test.arguments))
			if test.failure != "" {
				if callErr == nil || callErr.Code != test.failure {
					t.Fatalf("Call error = %#v", callErr)
				}
				if builds != 0 {
					t.Fatalf("rejected query started %d index builds", builds)
				}
				return
			}
			if callErr != nil {
				t.Fatal(callErr)
			}
			if !result.Abstention.Active || result.Abstention.Reason != test.reason || result.Receipt != nil {
				t.Fatalf("rejected query = %#v", result)
			}
			if result.Repository != nil {
				t.Fatal("early rejection captured repository state")
			}
			if builds != 0 {
				t.Fatalf("rejected query started %d index builds", builds)
			}
		})
	}
}

func TestEarlyQueryAbstentionPinsCanonicalAndTransportBytes(t *testing.T) {
	if runtimeUnsupported() {
		t.Skip("native query is qualified only on Darwin and Linux")
	}
	registry, err := New(makeRepository(t))
	if err != nil {
		t.Fatal(err)
	}
	result, callErr := registry.Call(context.Background(), ToolQuery, []byte(`{"task":"explain image codec"}`))
	if callErr != nil {
		t.Fatal(callErr)
	}
	canonical, canonicalErr := result.CanonicalJSON()
	if canonicalErr != nil {
		t.Fatal(canonicalErr)
	}
	const want = `{"abstention":{"active":true,"reason":"UNSUPPORTED_QUERY_INTENT"},"authorityClass":"NONE","epistemicClass":"NOT_OBSERVED","mutates":false,"receipt":null,"repository":null,"schema":"corvint-mcp-bridge-result/0","state":"ABSTAINED","tool":"corvint.query"}`
	if string(canonical) != want {
		t.Fatalf("canonical=%s want=%s", canonical, want)
	}
	object, objectErr := result.Object()
	if objectErr != nil {
		t.Fatal(objectErr)
	}
	transport, marshalErr := json.Marshal(map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": string(canonical)}},
		"isError":           false,
		"resultType":        "complete",
		"structuredContent": object,
	})
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	encodedText, encodeErr := json.Marshal(want)
	if encodeErr != nil {
		t.Fatal(encodeErr)
	}
	if !bytes.Contains(transport, append([]byte(`"text":`), encodedText...)) || !bytes.Contains(transport, []byte(`"repository":null`)) {
		t.Fatalf("transport=%s", transport)
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
	if _, err := New(t.TempDir()); err == nil || err.Code != "invalid-root" {
		t.Fatalf("non-repository root error = %#v", err)
	}
	root := t.TempDir()
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(root, ".git")); err != nil {
		t.Fatal(err)
	}
	if _, err := New(root); err == nil || err.Code != "invalid-root" {
		t.Fatalf("symlink Git marker error = %#v", err)
	}
}

func TestNewRejectsRewritableGitControlFile(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, ".git")
	if err := os.WriteFile(marker, []byte("gitdir: /first/target\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(root); err == nil || err.Code != "invalid-root" {
		t.Fatalf("regular Git control file construction error = %#v", err)
	}
	if err := os.WriteFile(marker, []byte("gitdir: /replacement/target\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(root); err == nil || err.Code != "invalid-root" {
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

// cemArguments names a well-formed map that need not exist: the Git
// admission refusals must fire before the map is read.
func cemArguments(t *testing.T, root string) string {
	t.Helper()
	head := strings.TrimSpace(string(gitOutput(t, root, "rev-parse", "HEAD")))
	return `{"map":".corvint/change.cem.json","expectedBase":"` + head + `","target":"` + head + `"}`
}
