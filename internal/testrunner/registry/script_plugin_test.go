// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/proctrace"
	"github.com/Beamfall/corvint/internal/sessiontest"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
)

func scriptGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "GIT_") {
			cmd.Env = append(cmd.Env, value)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_AUTHOR_NAME=Russell Lewis", "GIT_COMMITTER_NAME=Russell Lewis", "GIT_AUTHOR_EMAIL=me@russe11.com", "GIT_COMMITTER_EMAIL=me@russe11.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func scriptFixture(t *testing.T) (string, string, *ScriptPlugins) {
	t.Helper()
	raw, err := os.ReadFile("testdata/script/test-commands.json")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	scriptGit(t, root, "init", "-q")
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{ScriptDeclarationPath: raw, "data": []byte("data"), "tool": []byte("tool")} {
		if err := os.WriteFile(filepath.Join(root, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	scriptGit(t, root, "add", ".")
	scriptGit(t, root, "commit", "-qm", "script fixture")
	head := scriptGit(t, root, "rev-parse", "HEAD")
	plugins, err := ReadScriptPlugins(context.Background(), root, head, ScriptDeclarationPath)
	if err != nil {
		t.Fatal(err)
	}
	return root, head, plugins
}

func TestRSPV0050DeclaredCommandInventory(t *testing.T) {
	root, head, plugins := scriptFixture(t)
	expected := map[string][]string{
		"script:go":         {"go", "test", "-count=1", "./..."},
		"script:typescript": {"node", "test/*.e2e.ts"},
		"script:python":     {"python3", "script/*_test.py"},
		"script:shell":      {"script/*_test.sh"},
		"script:kotlin":     {"./gradlew", "fastGate"},
		"script:shader":     {"xcrun", "metal", "shaders/main.metal"},
	}
	if len(plugins.IDs()) != len(expected) {
		t.Fatalf("IDs: %v", plugins.IDs())
	}
	for _, id := range plugins.IDs() {
		plugin, key := plugins.Plugin(id, "linux", "amd64")
		s := NewSelectionSession(plugin, key, true)
		inv := s.Inventory(context.Background(), "")
		if !inv.Complete || len(inv.Tests) != 1 || inv.Tests[0] != (testrunner.InventoryTest{ID: id, Path: ScriptDeclarationPath}) || key.Checkout != head {
			t.Fatalf("%+v", inv)
		}
		a, err := testrunner.InventoryBytes(inv, key)
		if err != nil {
			t.Fatal(err)
		}
		b, err := testrunner.InventoryBytes(s.Inventory(context.Background(), ""), key)
		if err != nil || string(a) != string(b) {
			t.Fatalf("noncanonical inventory: %v", err)
		}
		cmd, err := plugin.Command(context.Background(), CommandRequest{Key: key, Purpose: "run", RunDir: root})
		if err != nil || !reflect.DeepEqual(cmd.Argv, expected[id]) {
			t.Fatalf("%s: %+v %v", id, cmd, err)
		}
		if id == "script:go" && cmd.Env["GOTOOLCHAIN"] != "local" || id != "script:go" && len(cmd.Env) != 0 {
			t.Fatalf("environment: %+v", cmd.Env)
		}
		// Collection support is not qualification. Even a claimed exclusion cannot
		// enable this unqualified runner in production.
		plan := s.Plan(inv, []testrunner.Exclusion{{ID: id, Fresh: true}}, nil)
		if !plan.Full || !strings.Contains(plan.Reason, "selection not qualified") || !slices.Equal(plan.Tests, []string{id}) {
			t.Fatalf("%+v", plan)
		}
		_, foreignKey := plugins.Plugin(id, "darwin", "arm64")
		if _, err := testrunner.CanonicalInventory(inv, foreignKey); err == nil {
			t.Fatal("cross-target inventory admitted")
		}
	}
	// Dirty configuration cannot rewrite the pinned declaration.
	if err := os.WriteFile(filepath.Join(root, ScriptDeclarationPath), []byte("malformed"), 0600); err != nil {
		t.Fatal(err)
	}
	again, err := ReadScriptPlugins(context.Background(), root, head, ScriptDeclarationPath)
	if err != nil || !reflect.DeepEqual(again.IDs(), plugins.IDs()) {
		t.Fatalf("pinned read: %v", err)
	}
}

func TestRSPV0050ExistingDeclarationSchema(t *testing.T) {
	root, _, _ := scriptFixture(t)
	raw, err := os.ReadFile("../../../.github/cishards/test-commands.json")
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(root, filepath.FromSlash(sessiontest.DeclarationPath))
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, raw, 0600); err != nil {
		t.Fatal(err)
	}
	scriptGit(t, root, "add", ".")
	scriptGit(t, root, "commit", "-qm", "existing declaration schema")
	p, err := ReadScriptPlugins(context.Background(), root, scriptGit(t, root, "rev-parse", "HEAD"), sessiontest.DeclarationPath)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(p.IDs(), []string{"script:go", "script:shell", "script:typescript"}) {
		t.Fatalf("existing schema inventory: %v", p.IDs())
	}
}

func TestRSPV0050UndeclaredCommandFull(t *testing.T) {
	_, _, plugins := scriptFixture(t)
	for _, p := range []*ScriptPlugins{plugins, nil} {
		plugin, key := p.Plugin("script:undeclared", "linux", "amd64")
		s := NewSelectionSession(plugin, key, true)
		inv := s.Inventory(context.Background(), "")
		plan := s.Plan(inv, nil, nil)
		if inv.Complete || !plan.Full || !strings.Contains(plan.Reason, "undeclared") {
			t.Fatalf("%+v %+v", inv, plan)
		}
	}
}

func TestRSPV0050MalformedDeclaration(t *testing.T) {
	valid := `{"profile":"corvint-test-commands/0","go":{"env":{},"flags":[]},"languages":{"shell":["make","test"]}}`
	cases := map[string]string{
		"unknown":   strings.Replace(valid, `"languages"`, `"commands"`, 1),
		"duplicate": strings.Replace(valid, `"go":`, `"profile":"corvint-test-commands/0","go":`, 1),
		"empty":     strings.Replace(valid, `["make","test"]`, `[]`, 1),
		"truncated": valid[:len(valid)-1],
		"trailing":  valid + ` {}`,
		"control":   strings.Replace(valid, `"test"`, `"test\u0000"`, 1),
		"cap":       strings.Repeat(" ", sessiontest.MaxDocumentBytes+1),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if p, err := parseScriptPlugins(strings.Repeat("a", 40), ScriptDeclarationPath, []byte(raw)); err == nil || p != nil {
				t.Fatalf("partial inventory: %+v %v", p, err)
			}
		})
	}
	root, head, _ := scriptFixture(t)
	for _, revision := range []string{"HEAD", head[:12], scriptGit(t, root, "rev-parse", head+"^{tree}")} {
		if p, err := ReadScriptPlugins(context.Background(), root, revision, ScriptDeclarationPath); err == nil || p != nil {
			t.Fatal("non-commit/unpinned checkout admitted")
		}
	}
	for _, source := range []string{sessiontest.DeclarationPath, "../test-commands.json"} {
		if p, err := ReadScriptPlugins(context.Background(), root, head, source); err == nil || p != nil {
			t.Fatal("untracked/unsupported declaration admitted")
		}
	}
	if err := os.Remove(filepath.Join(root, ScriptDeclarationPath)); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../data", filepath.Join(root, ScriptDeclarationPath)); err != nil {
		t.Fatal(err)
	}
	scriptGit(t, root, "add", ".")
	scriptGit(t, root, "commit", "-qm", "linked declaration")
	if p, err := ReadScriptPlugins(context.Background(), root, scriptGit(t, root, "rev-parse", "HEAD"), ScriptDeclarationPath); err == nil || p != nil {
		t.Fatal("linked declaration admitted")
	}
}

type scriptBackend struct {
	result   proctrace.Result
	requests []proctrace.Request
}

func (b *scriptBackend) Trace(_ context.Context, q proctrace.Request) proctrace.Result {
	b.requests = append(b.requests, q)
	return b.result
}
func scriptObservation() proctrace.Result {
	return proctrace.Result{Backend: "fixture", Reads: proctrace.Component[proctrace.Path]{State: proctrace.Present}, Listings: proctrace.Component[proctrace.Path]{State: proctrace.Present}, Lookups: proctrace.Component[proctrace.Path]{State: proctrace.Present}, Writes: proctrace.Component[proctrace.Write]{State: proctrace.Present}, Execs: proctrace.Component[proctrace.Exec]{State: proctrace.Present}, External: proctrace.Component[string]{State: proctrace.Present}, Outcome: proctrace.Outcome{Started: true}}
}
func scriptPath(name string) proctrace.Path {
	return proctrace.Path{Repository: "main", Path: name, ResolvedRepository: "main", ResolvedPath: name}
}
func scriptHeader(head string) testevidence.Record {
	r := testevidence.New("main", "script:shell", head)
	digest := strings.Repeat("a", 64)
	r.Runner = testevidence.RunnerIdentity{Plugin: "script", PluginVersion: "1", RunnerVersion: "1", ExecutableDigest: digest, ConfigurationDigest: digest}
	r.Execution = testevidence.ExecutionIdentity{OS: "linux", Architecture: "amd64", TracingBackend: "fixture", TracingVersion: "1", EnvironmentDigest: digest}
	r.Sources = []testevidence.Producer{{CoreDigest: digest, VerifierRevision: head, ConfigurationDigest: digest, Ref: "local", Run: "fixture", ContentDigest: digest}}
	r.CollectionCount = 1
	return r
}

func TestRSPV0050TracedSPIExecution(t *testing.T) {
	root, _, plugins := scriptFixture(t)
	for _, id := range plugins.IDs() {
		plugin, key := plugins.Plugin(id, "linux", "amd64")
		s := NewSelectionSession(plugin, key, false)
		backend := &scriptBackend{result: scriptObservation()}
		out := s.Run(context.Background(), s.Inventory(context.Background(), ""), nil, nil, root, false, func(ctx context.Context, cmd CommandResponse) (string, error) {
			_, report, err := TraceScriptCommand(ctx, cmd, proctrace.Request{Executable: "/pinned/tool", Args: append([]string(nil), cmd.Argv[1:]...), Dir: root, Env: []string{"GOTOOLCHAIN=old", "KEEP=yes"}, Repositories: []proctrace.Repository{{ID: "main", Root: root}}, EventCap: 1000, PathCap: 100}, backend)
			return report, err
		})
		if !out.Succeeded || len(backend.requests) != 1 || len(out.Attempts[0].Results) != 1 || out.Attempts[0].Results[0].ID != id {
			t.Fatalf("%s: %+v %+v", id, out, backend.requests)
		}
		if id == "script:go" && (!slices.Contains(backend.requests[0].Env, "GOTOOLCHAIN=local") || slices.Contains(backend.requests[0].Env, "GOTOOLCHAIN=old")) {
			t.Fatal("declared environment not applied")
		}
	}
}

func TestRSPV0050FileComponentsOnly(t *testing.T) {
	root, head, plugins := scriptFixture(t)
	repo, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		t.Fatal(err)
	}
	trace := scriptObservation()
	trace.Reads.Entries = []proctrace.Path{scriptPath("data")}
	trace.Listings.Entries = []proctrace.Path{scriptPath(".")}
	trace.Lookups.Entries = []proctrace.Path{scriptPath("missing")}
	trace.Execs.Entries = []proctrace.Exec{{Path: scriptPath("tool"), Digest: strings.Repeat("b", 64), Instrumented: true}}
	header := scriptHeader(head)
	// Even supplied coverage is discarded; it cannot become a narrowing input.
	header.Executed = testevidence.Component[testevidence.ExecutedFile]{State: testevidence.Present}
	record, err := ScriptTraceRecord(context.Background(), header, trace, repo)
	if err != nil {
		t.Fatal(err)
	}
	if record.Executed.State != testevidence.Absent || record.Executed.Reason != "script-kind" || record.Regions.Reason != "script-kind" || record.Loaded.Reason != "script-kind" || len(record.Executed.Items) != 0 {
		t.Fatalf("native: %+v", record)
	}
	if record.Reads.State != testevidence.Present || len(record.Reads.Items) != 1 || record.Reads.Items[0].OID != scriptGit(t, root, "rev-parse", head+":data") || record.Lookups.Items[0].Lexical != "missing" || record.Listings.Items[0].Lexical != "." || record.Execs.Items[0].Instrumented {
		t.Fatalf("trace: %+v", record)
	}
	plugin, _ := plugins.Plugin("script:shell", "linux", "amd64")
	native, err := plugin.Evidence(context.Background(), EvidenceRequest{Runner: "script"})
	if err != nil || len(native) != 1 {
		t.Fatal(err)
	}
	var executed testevidence.Component[testevidence.ExecutedFile]
	if err := json.Unmarshal(native[0].Executed, &executed); err != nil || executed.State != testevidence.Absent || executed.Reason != "script-kind" {
		t.Fatalf("%+v %v", executed, err)
	}
	// A later selector can match a newly added formerly missing path directly;
	// these observations preserve the edge rather than turning absent coverage
	// into an always-select dependency. Production qualification remains absent.
	if record.Lookups.State != testevidence.Present || record.Lookups.Items[0].Resolved != "missing" {
		t.Fatal("failed lookup lost")
	}
}

func TestRSPV0050IncompleteTrace(t *testing.T) {
	root, head, _ := scriptFixture(t)
	repo, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*proctrace.Result){
		"cap": func(r *proctrace.Result) {
			r.Reads.State = proctrace.Incomplete
			r.Reads.Reasons = []string{"event-cap"}
		},
		"unknown": func(r *proctrace.Result) { r.Reads.State = "future" },
		"absent": func(r *proctrace.Result) {
			r.Reads.State = proctrace.Absent
			r.Reads.Reasons = []string{"unsupported-platform"}
		},
		"untracked": func(r *proctrace.Result) { r.Reads.Entries = []proctrace.Path{scriptPath("generated")} },
		"foreign": func(r *proctrace.Result) {
			p := scriptPath("data")
			p.ResolvedRepository = "other"
			r.Reads.Entries = []proctrace.Path{p}
		},
		"malformed": func(r *proctrace.Result) { r.Reads.Entries = []proctrace.Path{scriptPath("../data")} },
		"entry cap": func(r *proctrace.Result) { r.Reads.Entries = make([]proctrace.Path, 4097) },
		"refused":   func(r *proctrace.Result) { r.Outcome.Started = false },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			trace := scriptObservation()
			mutate(&trace)
			record, err := ScriptTraceRecord(context.Background(), scriptHeader(head), trace, repo)
			if err != nil {
				t.Fatal(err)
			}
			if record.Reads.State == testevidence.Present || record.Reads.Reason == "" {
				t.Fatalf("narrow evidence: %+v", record.Reads)
			}
		})
	}
}

func TestRSPV0050RefusedExecution(t *testing.T) {
	root, _, plugins := scriptFixture(t)
	plugin, key := plugins.Plugin("script:shell", "linux", "amd64")
	backend := &scriptBackend{result: scriptObservation()}
	cmd, err := plugin.Command(context.Background(), CommandRequest{Key: key, Purpose: "run", RunDir: root})
	if err != nil {
		t.Fatal(err)
	}
	q := proctrace.Request{Executable: "/pinned/tool", Dir: root, Repositories: []proctrace.Repository{{ID: "main", Root: root}}, EventCap: 1000, PathCap: 100}
	for _, outcome := range []proctrace.Outcome{{ExitCode: -1}, {Started: true, Signal: 9}, {Started: true, Error: "truncated"}} {
		backend.result.Outcome = outcome
		_, report, err := TraceScriptCommand(context.Background(), cmd, q, backend)
		if err == nil || report != "" {
			t.Fatalf("false terminal: %q %v", report, err)
		}
	}
	if _, err := plugin.Parse(context.Background(), ParseRequest{Runner: "script", Report: "stdout says passed"}); err == nil {
		t.Fatal("stdout used as result")
	}
	if _, err := plugin.Command(context.Background(), CommandRequest{Key: key, Purpose: "run", RunDir: root, Tests: []string{"script:undeclared"}}); err == nil {
		t.Fatal("undeclared command executed")
	}
	backend.result.Outcome = proctrace.Outcome{Started: true, ExitCode: 7}
	_, report, err := TraceScriptCommand(context.Background(), cmd, q, backend)
	if err != nil {
		t.Fatal(err)
	}
	results, err := plugin.Parse(context.Background(), ParseRequest{Runner: "script", Report: report})
	if err != nil || results[0].Outcome != "failed" {
		t.Fatalf("lost failure: %+v %v", results, err)
	}
	q.Args = []string{"extra"}
	before := len(backend.requests)
	if _, _, err := TraceScriptCommand(context.Background(), cmd, q, backend); err == nil || len(backend.requests) != before {
		t.Fatal("mismatched command traced")
	}
}
