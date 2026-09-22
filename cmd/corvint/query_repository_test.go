package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/trace"
	"github.com/Beamfall/corvint/internal/tracerecordrepo"
)

const repositoryQueryTask = "fix token parser delimiter validation"

func runRepositoryQueryProcess(t *testing.T, root string, extraArguments ...string) processResult {
	t.Helper()
	arguments := []string{"--root", root, "query", "--task", repositoryQueryTask}
	arguments = append(arguments, extraArguments...)
	candidate := exec.Command(os.Args[0], append([]string{"-test.run=^TestCandidateHelperProcess$", "--"}, arguments...)...)
	candidate.Env = append(os.Environ(), "CORVINT_HELPER_PROCESS=1")
	return execute(t, candidate)
}

func TestFreshProcessRepositoryQueryMatchesPythonAcrossLimitsAndBudget(t *testing.T) {
	t.Parallel()
	root := queryCLIRepository(t)
	before := repositoryBytesDigest(t, root)
	for _, test := range []struct {
		name string
		argv []string
	}{
		{name: "default"},
		{name: "limit-1", argv: []string{"--limit", "1"}},
		{name: "limit-10", argv: []string{"--limit", "10"}},
		{name: "limit-50", argv: []string{"--limit", "50"}},
		{name: "budget", argv: []string{"--limit", "10", "--budget-bytes", "1800"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := runRepositoryQueryProcess(t, root, test.argv...)
			if candidate.exit != 0 || len(candidate.stderr) != 0 || !bytes.Contains(candidate.stdout, []byte(`"mode":"query"`)) {
				t.Fatalf("candidate exit=%d stdout=%s stderr=%s", candidate.exit, candidate.stdout, candidate.stderr)
			}
		})
	}
	if after := repositoryBytesDigest(t, root); before != after {
		t.Fatal("repository query changed repository or Git bytes")
	}
}

func TestFreshProcessRepositoryQueryConsumesMatchingPassedTrace(t *testing.T) {
	t.Parallel()
	root := queryCLIRepository(t)
	absent := runRepositoryQueryProcess(t, root)
	absentContext := decodeRepositoryQueryContext(t, absent.stdout)
	recorded := recordRepositoryQueryTrace(t, root, "token parser", "passed")
	before := repositoryBytesDigest(t, root)

	candidate := runRepositoryQueryProcess(t, root)
	if candidate.exit != 0 || len(candidate.stderr) != 0 {
		t.Fatalf("candidate exit=%d stdout=%s stderr=%s", candidate.exit, candidate.stdout, candidate.stderr)
	}
	context := decodeRepositoryQueryContext(t, candidate.stdout)
	learning := objectField(t, context, "learning")
	assertJSONNumber(t, learning, "local_trace_count", 1)
	assertJSONNumber(t, learning, "matched_local_traces", 1)
	if learning["local_trace_state"] != "ready" {
		t.Fatalf("learning=%#v", learning)
	}
	assertLocalTraceCandidate(t, context, recorded.Record.TraceID, recorded.Record.Revision, "internal/parser/token.go", 260, "changed")

	for _, field := range []string{"request", "revision", "freshness", "exclusions", "intent"} {
		if !reflect.DeepEqual(context[field], absentContext[field]) {
			t.Fatalf("trace changed invariant field %s: got=%#v absent=%#v", field, context[field], absentContext[field])
		}
	}

	budgeted := runRepositoryQueryProcess(t, root, "--budget-bytes", "1800")
	compact := objectField(t, decodeRepositoryQueryContext(t, budgeted.stdout), "learning")
	wantCompact := []string{"advisory_candidates", "history_digest", "history_tip", "local_trace_count", "local_trace_state"}
	gotCompact := make([]string, 0, len(compact))
	for key := range compact {
		gotCompact = append(gotCompact, key)
	}
	sort.Strings(gotCompact)
	if !reflect.DeepEqual(gotCompact, wantCompact) {
		t.Fatalf("compacted learning fields=%v want=%v", gotCompact, wantCompact)
	}
	if after := repositoryBytesDigest(t, root); before != after {
		t.Fatal("repository trace query mutated repository or trace bytes")
	}
}

func TestRepositoryQueryReadyStoreIgnoresNonmatchingAndNonpassedTraces(t *testing.T) {
	t.Parallel()
	root := queryCLIRepository(t)
	recordRepositoryQueryTrace(t, root, "database cache", "passed")
	recordRepositoryQueryTrace(t, root, "token parser", "failed")
	recordRepositoryQueryTrace(t, root, "token parser delimiter", "blocked")
	candidate := runRepositoryQueryProcess(t, root)
	context := decodeRepositoryQueryContext(t, candidate.stdout)
	learning := objectField(t, context, "learning")
	assertJSONNumber(t, learning, "local_trace_count", 3)
	assertJSONNumber(t, learning, "matched_local_traces", 0)
	if learning["local_trace_state"] != "ready" {
		t.Fatalf("learning=%#v", learning)
	}
	for _, raw := range arrayField(t, context, "results") {
		for _, evidence := range arrayField(t, raw.(map[string]any), "evidence") {
			if evidence.(map[string]any)["authority"] == "local-task-trace" {
				t.Fatalf("nonmatching/nonpassed trace emitted evidence: %#v", evidence)
			}
		}
	}
}

func TestRepositoryQueryMixedWorktreeDoesNotReadMalformedTraceStore(t *testing.T) {
	t.Parallel()
	root := queryCLIRepository(t)
	writeRecordStore(t, root, []byte("{malformed\n"))
	target := filepath.Join(root, "internal", "parser", "token.go")
	if err := os.WriteFile(target, []byte("package parser\n// mixed worktree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	candidate := runRepositoryQueryProcess(t, root)
	learning := objectField(t, decodeRepositoryQueryContext(t, candidate.stdout), "learning")
	if learning["local_trace_state"] != "blocked-mixed-worktree" {
		t.Fatalf("learning=%#v", learning)
	}
	assertJSONNumber(t, learning, "local_trace_count", 0)
	assertJSONNumber(t, learning, "matched_local_traces", 0)
}

func TestRepositoryQueryTraceStateFailuresAreTypedAndNonmutating(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		prepare func(*testing.T, string)
	}{
		{"malformed", func(t *testing.T, root string) { writeRecordStore(t, root, []byte("{\n")) }},
		{"unreachable", func(t *testing.T, root string) {
			directory := filepath.Join(root, ".context-corvint", "traces")
			if err := os.MkdirAll(directory, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, strings.Repeat("0", 40)+".jsonl"), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"duplicate", func(t *testing.T, root string) {
			recordRepositoryQueryTrace(t, root, "token parser", "passed")
			path := trace.StorePath(root, recordRevision(t, root))
			row, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, append(row, row...), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"oversized", func(t *testing.T, root string) {
			writeRecordStore(t, root, bytes.Repeat([]byte("x"), trace.MaxTraceRowBytes+1))
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := queryCLIRepository(t)
			test.prepare(t, root)
			before := repositoryBytesDigest(t, root)
			result := execute(t, candidateCommand("--root", root, "query", "--task", repositoryQueryTask))
			assertQueryRefusalCode(t, result, "unsupported-query-trace-state")
			if after := repositoryBytesDigest(t, root); before != after {
				t.Fatal("trace-state refusal mutated repository bytes")
			}
		})
	}
}

func TestRepositoryQueryUnreadableStoreFailsClosed(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("POSIX owner permission semantics required")
	}
	root := queryCLIRepository(t)
	writeRecordStore(t, root, []byte("{}\n"))
	directory := filepath.Join(root, ".context-corvint", "traces")
	if err := os.Chmod(directory, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(directory, 0o700)
	result := execute(t, candidateCommand("--root", root, "query", "--task", repositoryQueryTask))
	assertQueryRefusalCode(t, result, "unsupported-query-trace-state")
}

func TestRepositoryQueryTraceAncestryFailureIsHistory(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("test-only Git wrapper uses a POSIX shell")
	}
	root := queryCLIRepository(t)
	recordRepositoryQueryTrace(t, root, "token parser", "passed")
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	wrapper := fmt.Sprintf(`#!/bin/sh
for argument in "$@"; do
  if [ "$argument" = "--max-count=10001" ]; then
    printf 'injected ancestry failure\n' >&2
    exit 7
  fi
done
exec %s "$@"
`, strconv.Quote(realGit))
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	command := candidateCommand("--root", root, "query", "--task", repositoryQueryTask)
	command.Env = append(command.Env, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	assertQueryRefusalCode(t, execute(t, command), "unsupported-query-history")
}

func TestRepositoryQueryTraceReadRepositoryDriftIsTyped(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("test-only Git wrapper uses a POSIX shell")
	}
	root := queryCLIRepository(t)
	recordRepositoryQueryTrace(t, root, "token parser", "passed")
	target := filepath.Join(root, "internal", "parser", "token.go")
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin, marker := t.TempDir(), filepath.Join(t.TempDir(), "injected")
	wrapper := fmt.Sprintf(`#!/bin/sh
for argument in "$@"; do
  if [ "$argument" = "--max-count=10001" ] && [ ! -e %s ]; then
    : > %s
    printf '\n// trace-read drift\n' >> %s
  fi
done
exec %s "$@"
`, strconv.Quote(marker), strconv.Quote(marker), strconv.Quote(target), strconv.Quote(realGit))
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	command := candidateCommand("--root", root, "query", "--task", repositoryQueryTask)
	command.Env = append(command.Env, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	assertQueryRefusalCode(t, execute(t, command), "unsupported-query-drift")
}

// TestRepositoryTraceGateFollowsTheOracleByIntent: a present clean-tree trace
// store is read for repository and agent-tooling tasks, as the oracle reads it,
// and still refused on the project-operations branch, which the harness alone
// reaches.
func TestRepositoryTraceGateFollowsTheOracleByIntent(t *testing.T) {
	t.Parallel()
	root := queryCLIRepository(t)
	recordRepositoryQueryTrace(t, root, "token parser", "passed")
	_, err := repositoryQueryContext(context.Background(), root, strings.ToUpper(authorityStartPrompt), 10, nil)
	var queryErr *contextindex.Error
	if !errors.As(err, &queryErr) || queryErr.Code != "unsupported-query-trace-state" ||
		err.Error() != "native Go authority-start query requires an absent clean-tree local trace store" {
		t.Fatalf("project-operations error=%v", err)
	}
	receipt, err := repositoryQueryContext(context.Background(), root, "CORVINT\u2003AGENT\u2003CONTEXT\u2003ROADMAP", 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	learning := receipt["learning"].(map[string]any)
	if learning["local_trace_state"] != "ready" || learning["local_trace_count"] != 1 {
		t.Fatalf("agent-tooling learning=%v", learning)
	}
}

func recordRepositoryQueryTrace(t *testing.T, root, task, outcome string) tracerecordrepo.Result {
	t.Helper()
	result, err := tracerecordrepo.Record(context.Background(), root, tracerecordrepo.Input{
		Task: task, OpenedPaths: []string{"AGENTS.md"}, ChangedPaths: []string{"internal/parser/token.go"},
		Verification: []string{"go test ./...", "git diff --check"}, Outcome: outcome,
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func decodeRepositoryQueryContext(t *testing.T, output []byte) map[string]any {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal(output, &envelope); err != nil {
		t.Fatalf("decode query output: %v: %s", err, output)
	}
	return objectField(t, envelope, "context")
}

func objectField(t *testing.T, object map[string]any, field string) map[string]any {
	t.Helper()
	value, ok := object[field].(map[string]any)
	if !ok {
		t.Fatalf("%s=%#v", field, object[field])
	}
	return value
}

func arrayField(t *testing.T, object map[string]any, field string) []any {
	t.Helper()
	value, ok := object[field].([]any)
	if !ok {
		t.Fatalf("%s=%#v", field, object[field])
	}
	return value
}

func assertJSONNumber(t *testing.T, object map[string]any, field string, want float64) {
	t.Helper()
	if object[field] != want {
		t.Fatalf("%s=%#v want=%v in %#v", field, object[field], want, object)
	}
}

func assertLocalTraceCandidate(t *testing.T, context map[string]any, traceID, revision, path string, score float64, role string) {
	t.Helper()
	wantReason := fmt.Sprintf("successful local trace %s %s this path; matched 2 task terms; trace recorded at commit %s", traceID[:12], role, revision[:12])
	for _, raw := range arrayField(t, context, "results") {
		result := raw.(map[string]any)
		if result["kind"] != "learned-path" || result["id"] != path || result["score"] != score {
			continue
		}
		for _, rawEvidence := range arrayField(t, result, "evidence") {
			evidence := rawEvidence.(map[string]any)
			if evidence["authority"] == "local-task-trace" && evidence["confidence"] == "advisory" && evidence["reason"] == wantReason {
				return
			}
		}
	}
	t.Fatalf("missing local trace candidate path=%s score=%v reason=%q in %#v", path, score, wantReason, context["results"])
}

func assertQueryRefusalCode(t *testing.T, result processResult, want string) {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(result.stderr, &payload); err != nil {
		t.Fatalf("decode refusal: %v: %s", err, result.stderr)
	}
	if result.exit != 2 || len(result.stdout) != 0 || payload["code"] != want || payload["ok"] != false {
		t.Fatalf("result=%#v payload=%#v want=%s", result, payload, want)
	}
}

func TestQueryValidationPrecedesRootResolutionAndRepositoryProbe(t *testing.T) {
	t.Parallel()
	notRepository := filepath.Join(t.TempDir(), "missing")
	_, err := parse([]string{"--root", notRepository, "query", "--task", repositoryQueryTask, "--limit", "51"})
	if err == nil || err.Error() != "limit must be an integer from 1 to 50" {
		t.Fatalf("invalid query did not precede root probe: %v", err)
	}
	_, err = parse([]string{"--root", notRepository, "query", "--task", repositoryQueryTask, "--limit", "10"})
	if err == nil || !strings.HasPrefix(err.Error(), "not a Git repository: ") {
		t.Fatalf("valid query did not reach root probe: %v", err)
	}
	repository := queryCLIRepository(t)
	link := filepath.Join(t.TempDir(), "repository-link")
	if err := os.Symlink(repository, link); err != nil {
		t.Fatal(err)
	}
	resolvedRepository, err := filepath.EvalSymlinks(repository)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parse([]string{"--root", link, "query", "--task", repositoryQueryTask, "--limit", "10"})
	if err != nil || parsed.root != resolvedRepository || parsed.queryIntent != "repository" {
		t.Fatalf("deferred root=%q intent=%q err=%v", parsed.root, parsed.queryIntent, err)
	}
}

func TestRepositoryQueryActivelyRejectsPostBuildDrift(t *testing.T) {
	t.Parallel()
	root := queryCLIRepository(t)
	index, err := contextindex.BuildEval(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "internal", "parser", "token.go")
	if err := os.WriteFile(path, []byte("package parser\n// drift\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := repositoryBytesDigest(t, root)
	_, err = contextindex.EvalQuery(context.Background(), index, repositoryQueryTask, 10, nil)
	var queryErr *contextindex.Error
	if !errors.As(err, &queryErr) || queryErr.Code != "unsupported-query-drift" {
		t.Fatalf("post-build drift accepted: %v", err)
	}
	if after := repositoryBytesDigest(t, root); before != after {
		t.Fatal("drift refusal changed repository or Git bytes beyond the test's injected drift")
	}
}

func TestFreshProcessRepositoryQueryRejectsDeterministicDrift(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("test-only Git injection wrapper uses a POSIX shell")
	}
	root := queryCLIRepository(t)
	target := filepath.Join(root, "internal", "parser", "token.go")
	original, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	before, beforePaths := repositoryBytesDigest(t, root), repositoryPathListing(t, root)

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin, state := t.TempDir(), t.TempDir()
	marker := filepath.Join(state, "injected")
	wrapper := fmt.Sprintf(`#!/bin/sh
is_log=false
for argument in "$@"; do
  if [ "$argument" = log ]; then is_log=true; fi
done
if [ "$is_log" = true ] && [ ! -e %s ]; then
  : > %s
  printf '\n// deterministic fresh-process drift\n' >> %s
fi
exec %s "$@"
`, strconv.Quote(marker), strconv.Quote(marker), strconv.Quote(target), strconv.Quote(realGit))
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}

	arguments := []string{"--root", root, "query", "--task", repositoryQueryTask}
	candidate := exec.Command(os.Args[0], append([]string{"-test.run=^TestCandidateHelperProcess$", "--"}, arguments...)...)
	candidate.Env = append(os.Environ(),
		"CORVINT_HELPER_PROCESS=1",
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	result := execute(t, candidate)
	var refusal map[string]any
	if err := json.Unmarshal(result.stderr, &refusal); err != nil {
		t.Fatalf("decode refusal: %v: %s", err, result.stderr)
	}
	if result.exit != 2 || len(result.stdout) != 0 || refusal["code"] != "unsupported-query-drift" {
		t.Fatalf("candidate exit=%d stdout=%s stderr=%s", result.exit, result.stdout, result.stderr)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("test-only drift injection did not run: %v", err)
	}
	injected := repositoryBytesDigest(t, root)
	if injected == before {
		t.Fatal("test-only drift injection did not change the repository snapshot")
	}
	if _, err := os.Stat(filepath.Join(root, ".corvint")); !os.IsNotExist(err) {
		t.Fatalf("drift refusal wrote .corvint state: %v", err)
	}
	if after := repositoryBytesDigest(t, root); after != injected {
		t.Fatal("drift refusal changed repository bytes beyond the test-only injection")
	}
	if err := os.WriteFile(target, original, 0o644); err != nil {
		t.Fatal(err)
	}
	if restored := repositoryBytesDigest(t, root); restored != before {
		t.Fatalf("test fixture did not restore to its exact pre-injection snapshot; differing paths (empty means a same-size content change): %v",
			repositoryPathDifference(beforePaths, repositoryPathListing(t, root)))
	}
}

// repositoryPathListing maps every relative path under root to its mode, size
// and modification time so a digest mismatch can name the paths that changed,
// transient Git files (.git/objects/maintenance.lock, .tmp-*-pack-*) included.
func repositoryPathListing(t *testing.T, root string) map[string]string {
	t.Helper()
	listing := map[string]string{}
	err := filepath.WalkDir(root, func(file string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		listing[filepath.ToSlash(relative)] = fmt.Sprintf("%s %d %s", info.Mode(), info.Size(), info.ModTime().UTC().Format(time.RFC3339Nano))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return listing
}

// repositoryPathDifference names the paths whose listing entry changed, was
// removed, or was added between two repositoryPathListing snapshots.
func repositoryPathDifference(before, after map[string]string) []string {
	differing := []string{}
	for path, entry := range before {
		if after[path] != entry {
			differing = append(differing, fmt.Sprintf("%s: %q -> %q", path, entry, after[path]))
		}
	}
	for path, entry := range after {
		if _, present := before[path]; !present {
			differing = append(differing, fmt.Sprintf("%s: added %q", path, entry))
		}
	}
	sort.Strings(differing)
	return differing
}

func TestRepositoryQuerySharedPathAndAuthoritySeparationAreStructural(t *testing.T) {
	t.Parallel()
	cliQueryCalls := callsInIfStringBranch(t, "main.go", "runContext", "options.command", "query")
	harnessCalls := callsInIfStringBranch(t, "harness_context.go", "harnessIndexedContext", "event", "user-prompt")
	repositoryCalls := callsInSwitchCase(t, "main.go", "standaloneQueryContext", "repository")
	toolingCalls := callsInSwitchCase(t, "main.go", "standaloneQueryContext", "agent-tooling")
	projectCalls := callsInSwitchCase(t, "main.go", "standaloneQueryContext", "project-operations")
	sharedCalls := functionCalls(t, "harness_context.go", "repositoryQueryContext")
	assertCall(t, sharedCalls, "repositoryQueryOver")
	maps.Copy(sharedCalls, functionCalls(t, "harness_context.go", "repositoryQueryOver"))
	authorityCalls := functionCalls(t, "main.go", "authorityStartQueryContext")
	assertCall(t, cliQueryCalls, "standaloneQueryContext")
	assertCall(t, harnessCalls, "repositoryQueryContext")
	assertCall(t, repositoryCalls, "repositoryQueryContext")
	assertCall(t, toolingCalls, "repositoryQueryContext")
	assertCall(t, projectCalls, "authorityStartQueryContext")
	assertCall(t, sharedCalls, "contextindex.BuildEval")
	assertCall(t, sharedCalls, "tracerecordrepo.Read")
	assertCall(t, sharedCalls, "contextindex.EvalQuery")
	assertCall(t, authorityCalls, "contextindex.BuildQuery")
	assertCall(t, authorityCalls, "contextindex.QueryAuthorityStartBudget")
	for _, forbidden := range []string{"contextindex.BuildQuery", "contextindex.QueryAuthorityStartBudget"} {
		if sharedCalls[forbidden] || repositoryCalls[forbidden] {
			t.Fatalf("shared repository path calls narrow authority helper %s", forbidden)
		}
	}
	if projectCalls["repositoryQueryContext"] || authorityCalls["repositoryQueryContext"] {
		t.Fatal("project-operations route calls the repository query path")
	}
}

func functionCalls(t *testing.T, name, function string) map[string]bool {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join(moduleRoot(t), "cmd", "corvint", name), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	result := map[string]bool{}
	for _, declaration := range parsed.Decls {
		item, ok := declaration.(*ast.FuncDecl)
		if !ok || item.Name.Name != function {
			continue
		}
		collectCalls(item.Body, result)
		return result
	}
	t.Fatalf("function %s not found in %s", function, name)
	return nil
}

func callsInIfStringBranch(t *testing.T, name, function, field, value string) map[string]bool {
	t.Helper()
	declaration := parsedFunction(t, name, function)
	result := map[string]bool{}
	ast.Inspect(declaration.Body, func(node ast.Node) bool {
		branch, ok := node.(*ast.IfStmt)
		if !ok || !stringEquality(branch.Cond, field, value) {
			return true
		}
		collectCalls(branch.Body, result)
		return false
	})
	if len(result) == 0 {
		t.Fatalf("if branch %s == %q not found in %s", field, value, function)
	}
	return result
}

func callsInSwitchCase(t *testing.T, name, function, value string) map[string]bool {
	t.Helper()
	declaration := parsedFunction(t, name, function)
	result := map[string]bool{}
	ast.Inspect(declaration.Body, func(node ast.Node) bool {
		clause, ok := node.(*ast.CaseClause)
		if !ok || len(clause.List) != 1 {
			return true
		}
		literal, ok := clause.List[0].(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING || strings.Trim(literal.Value, `"`) != value {
			return true
		}
		for _, statement := range clause.Body {
			collectCalls(statement, result)
		}
		return false
	})
	if len(result) == 0 {
		t.Fatalf("switch case %q not found in %s", value, function)
	}
	return result
}

func parsedFunction(t *testing.T, name, function string) *ast.FuncDecl {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join(moduleRoot(t), "cmd", "corvint", name), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, declaration := range parsed.Decls {
		if item, ok := declaration.(*ast.FuncDecl); ok && item.Name.Name == function {
			return item
		}
	}
	t.Fatalf("function %s not found in %s", function, name)
	return nil
}

func stringEquality(expression ast.Expr, field, value string) bool {
	binary, ok := expression.(*ast.BinaryExpr)
	if !ok || binary.Op != token.EQL {
		return false
	}
	left := expressionName(binary.X)
	literal, ok := binary.Y.(*ast.BasicLit)
	return ok && left == field && literal.Kind == token.STRING && strings.Trim(literal.Value, `"`) == value
}

func expressionName(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		return expressionName(value.X) + "." + value.Sel.Name
	default:
		return ""
	}
}

func collectCalls(node ast.Node, result map[string]bool) {
	ast.Inspect(node, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch target := call.Fun.(type) {
		case *ast.Ident:
			result[target.Name] = true
		case *ast.SelectorExpr:
			if owner, ok := target.X.(*ast.Ident); ok {
				result[owner.Name+"."+target.Sel.Name] = true
			}
		}
		return true
	})
}

func assertCall(t *testing.T, calls map[string]bool, want string) {
	t.Helper()
	if !calls[want] {
		t.Fatalf("call edge %s missing from %v", want, calls)
	}
}
