package contextindex

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const queryTaskFixture = "Identify the active work queue, required workflow gates, and minimum context needed to safely take the next roadmap ticket"

func TestQueryIntentUsesEvalQueryNormalization(t *testing.T) {
	for _, test := range []struct {
		text, want string
	}{
		{strings.ToUpper(queryTaskFixture), "project-operations"},
		{"  ROADMAP\u2003WORK QUEUE  ", "project-operations"},
		{"CORVINT\u2003AGENT\u2003CONTEXT\u2003ROADMAP", "agent-tooling"},
		{"fix token parser", "repository"},
	} {
		if got := QueryIntent(test.text); got != test.want {
			t.Fatalf("QueryIntent(%q)=%q want=%q", test.text, got, test.want)
		}
		if got := evalInferIntent(test.text).id; got != test.want {
			t.Fatalf("evalInferIntent(%q)=%q want=%q", test.text, got, test.want)
		}
	}
}

func TestTCPV0008NestedInstructionEligibilityIsShared(t *testing.T) {
	for _, candidate := range []string{
		".github/workflows/AGENTS.md",
		"docs/agent-workflows/CLAUDE.md",
		"docs/plans/AGENTS.md",
		"script/GEMINI.md",
	} {
		if documentKind(candidate) != "instructions" {
			t.Fatalf("%s is no longer classified as instructions", candidate)
		}
		if projectOperationPath(candidate) {
			t.Errorf("project operations admitted nested instruction path %s", candidate)
		}
		if _, eligible := instructionRank(candidate); eligible {
			t.Errorf("governing rank admitted nested instruction path %s", candidate)
		}
	}
	for _, candidate := range []string{"AGENTS.md", "CLAUDE.md", "GEMINI.md"} {
		if !projectOperationPath(candidate) {
			t.Errorf("project operations rejected existing root instruction path %s", candidate)
		}
		if _, eligible := instructionRank(candidate); !eligible {
			t.Errorf("governing rank rejected existing root instruction path %s", candidate)
		}
	}
	for _, candidate := range []string{
		"copilot-instructions.md",
		".github/copilot-instructions.md",
		".github/instructions/review.instructions.md",
	} {
		if projectOperationPath(candidate) {
			t.Errorf("project operations newly admitted instruction path %s", candidate)
		}
		if _, eligible := instructionRank(candidate); !eligible {
			t.Errorf("governing rank rejected its existing instruction path %s", candidate)
		}
	}
	for _, candidate := range []string{"Makefile", ".github/workflows/test.yml", "docs/plans/release.md", "script/check.sh"} {
		if !projectOperationPath(candidate) {
			t.Errorf("project operations rejected non-instruction path %s", candidate)
		}
	}
}

func TestQueryTraceSnapshotValidatesStateAndCopiesRecords(t *testing.T) {
	records := []QueryTrace{{
		TraceID: strings.Repeat("a", 64), Task: "token parser", Outcome: "passed",
		OpenedPaths: []string{"a.go"}, ChangedPaths: []string{"b.go"},
	}}
	snapshot, err := NewQueryTraceSnapshot("ready", records)
	if err != nil {
		t.Fatal(err)
	}
	records[0].Task = "mutated"
	records[0].OpenedPaths[0] = "mutated.go"
	if snapshot.records[0].Task != "token parser" || snapshot.records[0].OpenedPaths[0] != "a.go" {
		t.Fatalf("snapshot retained caller-owned storage: %#v", snapshot.records)
	}
	for _, test := range []struct {
		state   string
		records []QueryTrace
	}{
		{"unknown", nil},
		{"absent", []QueryTrace{{TraceID: "x", Task: "task", Outcome: "passed"}}},
		{"blocked-mixed-worktree", []QueryTrace{{TraceID: "x", Task: "task", Outcome: "passed"}}},
		{"ready", []QueryTrace{{TraceID: "", Task: "task", Outcome: "passed"}}},
	} {
		if _, err := NewQueryTraceSnapshot(test.state, test.records); err == nil {
			t.Fatalf("NewQueryTraceSnapshot(%q, %#v) succeeded", test.state, test.records)
		}
	}
	if _, err := EvalQuery(context.Background(), nil, "task", 10, nil, QueryTraceSnapshot{}); err == nil {
		t.Fatal("EvalQuery accepted an unconstructed trace snapshot")
	}
}

func authorityRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	files := map[string]string{
		".gitignore": ".context-corvint/\n",
		"AGENTS.md": "# Project instructions\n\nThe roadmap is the only active work queue.\n" +
			"Run `make orient`, then `script/context-packet.sh --ticket ID`.\n" +
			"Use `script/roadmap.sh` and run the required workflow gates.\n",
		"script/context-packet.sh": "#!/bin/sh\nexit 0\n",
		"script/roadmap.sh":        "#!/bin/sh\nexit 0\n",
	}
	for path, content := range files {
		writeTestFile(t, root, path, content)
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "initial authority fixture")
	return root
}

func TestAuthorityStartQueryReturnsCriticalInstructionClosure(t *testing.T) {
	root := authorityRepository(t)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := QueryAuthorityStart(context.Background(), index, queryTaskFixture, 1)
	if err != nil {
		t.Fatal(err)
	}
	results := anySlice(receipt["results"])
	if len(results) != 1 {
		t.Fatalf("results=%v", results)
	}
	result := results[0].(map[string]any)
	if result["kind"] != "instructions" || result["id"] != "AGENTS.md" {
		t.Fatalf("result=%#v", result)
	}
	coverage := receipt["coverage"].(map[string]any)
	if got := anySlice(coverage["critical"]); len(got) != 1 || got[0] != "instructions:AGENTS.md" {
		t.Fatalf("critical=%v", got)
	}
	intent := receipt["intent"].(map[string]any)
	if intent["id"] != "project-operations" || receipt["state"] != "READY" {
		t.Fatalf("intent=%v state=%v", intent, receipt["state"])
	}
	encoded, err := CanonicalJSON(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if coverage["packet_bytes"] != len(encoded) {
		t.Fatalf("packet_bytes=%v encoded=%d", coverage["packet_bytes"], len(encoded))
	}
}

func TestAuthorityStartQueryRejectsUnsupportedAuthorityAndTraceState(t *testing.T) {
	t.Run("tie", func(t *testing.T) {
		root := authorityRepository(t)
		agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
		if err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, root, "GEMINI.md", string(agents))
		testGit(t, root, "add", "GEMINI.md")
		testGit(t, root, "commit", "-qm", "add tied authority")
		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		_, err = QueryAuthorityStart(context.Background(), index, queryTaskFixture, 1)
		var unsupported *Error
		if !errors.As(err, &unsupported) || unsupported.Code != "unsupported-query-authority" {
			t.Fatalf("error=%#v", err)
		}
	})
	t.Run("clean trace store", func(t *testing.T) {
		root := authorityRepository(t)
		if err := os.MkdirAll(filepath.Join(root, ".context-corvint", "traces"), 0o700); err != nil {
			t.Fatal(err)
		}
		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		if len(index.DirtyPaths) != 0 {
			t.Fatalf("dirty=%v", index.DirtyPaths)
		}
		_, err = QueryAuthorityStart(context.Background(), index, queryTaskFixture, 1)
		var unsupported *Error
		if !errors.As(err, &unsupported) || unsupported.Code != "unsupported-query-trace-state" {
			t.Fatalf("error=%#v", err)
		}
	})
	t.Run("CRLF", func(t *testing.T) {
		root := authorityRepository(t)
		data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
		if err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, root, "AGENTS.md", strings.ReplaceAll(string(data), "\n", "\r\n"))
		testGit(t, root, "add", "AGENTS.md")
		testGit(t, root, "commit", "-qm", "use CRLF authority")
		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		_, err = QueryAuthorityStart(context.Background(), index, queryTaskFixture, 1)
		var unsupported *Error
		if !errors.As(err, &unsupported) || unsupported.Code != "unsupported-query-authority" {
			t.Fatalf("error=%#v", err)
		}
	})
	t.Run("missing instruction reference", func(t *testing.T) {
		root := authorityRepository(t)
		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		record := index.Documents["AGENTS.md"]
		record.Fields["references"] = []string{"missing/tool.sh"}
		index.Documents["AGENTS.md"] = record
		_, err = QueryAuthorityStart(context.Background(), index, queryTaskFixture, 1)
		var unsupported *Error
		if !errors.As(err, &unsupported) || unsupported.Code != "unsupported-query-authority" {
			t.Fatalf("error=%#v", err)
		}
	})
}

func TestAuthorityStartQueryRejectsHistoryTreeDrift(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test wrapper uses a POSIX shell")
	}
	root := authorityRepository(t)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	alternate := "0" + index.Revision[1:]
	if index.Revision[0] == '0' {
		alternate = "1" + index.Revision[1:]
	}
	for _, test := range []struct {
		name, initialTree, postTree, want string
		statusAfter, logMustBeAbsent      bool
	}{
		{"before log", alternate, index.Revision, "repository revision changed before history learning", false, true},
		{"after log", index.Revision, alternate, "repository revision changed during history learning", false, false},
		{"status after log", index.Revision, index.Revision, "repository worktree changed during history learning", true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			log := installHistoryGitWrapper(t, index, test.initialTree, test.postTree, test.statusAfter)
			_, err := QueryAuthorityStart(context.Background(), index, queryTaskFixture, 1)
			if err == nil || err.Error() != test.want {
				t.Fatalf("error=%v", err)
			}
			if test.logMustBeAbsent {
				if _, err := os.Stat(log); !os.IsNotExist(err) {
					t.Fatalf("git log ran before initial tree drift: stat error=%v", err)
				}
			}
		})
	}
}

func TestAuthorityStartQueryHistoryProbeGitProcessCount(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test wrapper uses a POSIX shell")
	}
	root := authorityRepository(t)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	log := installHistoryGitWrapper(t, index, index.Revision, index.Revision, false)
	receipt, err := QueryAuthorityStart(context.Background(), index, queryTaskFixture, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CanonicalJSON(receipt); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(log + ".calls")
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	if len(got) != 5 || strings.Count(got, "s") != 2 || strings.Count(got, "r") != 2 || strings.Count(got, "l") != 1 {
		t.Fatalf("history learning Git calls=%q want two status, two rev-parse, and one log", got)
	}
}

func installHistoryGitWrapper(t *testing.T, index *Index, initialTree, postTree string, statusAfter bool) string {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin, state := t.TempDir(), t.TempDir()
	log := filepath.Join(state, "log")
	status := filepath.Join(state, "status-count")
	statusOutput := ""
	if statusAfter {
		statusOutput = " M AGENTS.md\\000"
	}
	wrapper := fmt.Sprintf(`#!/bin/sh
kind=
for argument in "$@"; do
  case "$argument" in
    status|rev-parse|log) kind="$argument" ;;
  esac
done
case "$kind" in
  status) printf s >> %s ;;
  rev-parse) printf r >> %s ;;
  log) printf l >> %s ; : > %s ;;
esac
if [ "$kind" = rev-parse ]; then
  initial=false
  headtree=false
  for argument in "$@"; do
    if [ "$argument" = "HEAD^{commit}" ]; then initial=true; fi
    if [ "$argument" = "HEAD^{tree}" ]; then headtree=true; fi
  done
  if [ "$initial" = true ] && [ "$headtree" = true ]; then
    printf '%%s\n%%s\n' %s %s
    exit 0
  fi
  if [ "$initial" = true ]; then
    printf '%%s\n' %s
    exit 0
  fi
  tree=%s
  if [ -f %s ]; then tree=%s; fi
  printf '%%s\n' "$tree"
  exit 0
fi
if [ "$kind" = status ]; then
  count=0
  if [ -f %s ]; then IFS= read -r count < %s; fi
  count=$((count + 1))
  printf '%%s\n' "$count" > %s
  if [ "$count" -gt 1 ] && [ -n %s ]; then
    printf %s
    exit 0
  fi
fi
exec %s "$@"
`, strconv.Quote(log+".calls"), strconv.Quote(log+".calls"), strconv.Quote(log+".calls"), strconv.Quote(log),
		strconv.Quote(index.CommitRevision), strconv.Quote(initialTree), strconv.Quote(index.CommitRevision),
		strconv.Quote(initialTree), strconv.Quote(log), strconv.Quote(postTree),
		strconv.Quote(status), strconv.Quote(status), strconv.Quote(status), strconv.Quote(statusOutput), strconv.Quote(statusOutput), strconv.Quote(realGit))
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

func TestQueryIntentMatchesProjectOperationsBoundary(t *testing.T) {
	for _, test := range []struct {
		text, want string
	}{
		{queryTaskFixture, "project-operations"},
		{"roadmap only", "repository"},
		{"corvint agent context roadmap", "agent-tooling"},
		{"work queue ticket", "project-operations"},
	} {
		intent, _ := queryIntent(strings.ToLower(test.text), terms(test.text))
		if intent != test.want {
			t.Fatalf("intent(%q)=%q want=%q", test.text, intent, test.want)
		}
	}
}

func TestValidateQueryCommandProfiles(t *testing.T) {
	tests := []struct {
		name, text, wantIntent, wantCode, wantMessage string
		limit                                         int
	}{
		{name: "repository limit one", text: "fix token parser", limit: 1, wantIntent: "repository"},
		{name: "repository default limit", text: "fix token parser", limit: 10, wantIntent: "repository"},
		{name: "repository maximum limit", text: "fix token parser", limit: 50, wantIntent: "repository"},
		{name: "repository maximum task", text: strings.Repeat("x", maxQueryChars), limit: 10, wantIntent: "repository"},
		{name: "project operations", text: queryTaskFixture, limit: 1, wantIntent: "project-operations"},
		{name: "project operations Python ASCII space", text: "\x1c" + queryTaskFixture + "\x1f", limit: 1, wantIntent: "project-operations"},
		{name: "empty", text: "", limit: 10, wantMessage: "query text must be non-empty"},
		{name: "ASCII whitespace", text: " \t\n", limit: 10, wantMessage: "query text must be non-empty"},
		{name: "Python-only whitespace", text: "\x1c\x1d\x1e\x1f", limit: 10, wantMessage: "query text must be non-empty"},
		{name: "Unicode Python whitespace", text: "\u00a0\u2003", limit: 10, wantMessage: "query text must be non-empty"},
		{name: "oversized", text: strings.Repeat("x", maxQueryChars+1), limit: 10, wantMessage: "query text exceeds 8000 characters"},
		{name: "repository non-ASCII", text: "fix café parser", limit: 10, wantIntent: "repository"},
		{name: "repository below limit", text: "fix token parser", limit: 0, wantMessage: "limit must be an integer from 1 to 50"},
		{name: "repository above limit", text: "fix token parser", limit: 51, wantMessage: "limit must be an integer from 1 to 50"},
		{name: "repository non-ASCII above limit", text: "fix café parser", limit: 51, wantMessage: "limit must be an integer from 1 to 50"},
		{name: "project operations default limit", text: queryTaskFixture, limit: 10, wantIntent: "project-operations"},
		{name: "project operations maximum limit", text: queryTaskFixture, limit: 50, wantIntent: "project-operations"},
		{name: "project operations non-ASCII", text: queryTaskFixture + " café", limit: 1, wantIntent: "project-operations"},
		{name: "project operations below limit", text: queryTaskFixture, limit: 0, wantMessage: "limit must be an integer from 1 to 50"},
		{name: "project operations above limit", text: queryTaskFixture + " café", limit: 51, wantMessage: "limit must be an integer from 1 to 50"},
		{name: "agent tooling", text: "corvint agent context roadmap", limit: 10, wantIntent: "agent-tooling"},
		{name: "agent tooling non-ASCII", text: "corvint agent context roadmap café", limit: 50, wantIntent: "agent-tooling"},
		{name: "agent tooling below limit", text: "corvint agent context roadmap café", limit: 0, wantMessage: "limit must be an integer from 1 to 50"},
		{name: "agent tooling above limit", text: "corvint agent context roadmap", limit: 51, wantMessage: "limit must be an integer from 1 to 50"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			intent, err := ValidateQueryCommand(test.text, test.limit)
			if test.wantMessage == "" {
				if err != nil || intent != test.wantIntent {
					t.Fatalf("intent=%q error=%v want intent=%q", intent, err, test.wantIntent)
				}
				return
			}
			var queryErr *Error
			if !errors.As(err, &queryErr) || intent != "" || queryErr.Code != test.wantCode || queryErr.Message != test.wantMessage {
				t.Fatalf("intent=%q error=%#v want code=%q message=%q", intent, err, test.wantCode, test.wantMessage)
			}
		})
	}
}

func TestValidateQueryCommandRunsBeforeRepositoryAccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test canary uses a POSIX shell")
	}
	bin := t.TempDir()
	canary := filepath.Join(t.TempDir(), "git-called")
	wrapper := fmt.Sprintf("#!/bin/sh\n: > %s\nexit 99\n", strconv.Quote(canary))
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	for _, test := range []struct {
		text  string
		limit int
	}{
		{"", 10},
		{strings.Repeat("x", maxQueryChars+1), 10},
		{"fix café parser", 0},
		{"corvint agent context roadmap café", 51},
		{"fix token parser", 51},
		{queryTaskFixture, 0},
	} {
		if _, err := ValidateQueryCommand(test.text, test.limit); err == nil {
			t.Fatalf("ValidateQueryCommand(%q, %d) succeeded", test.text, test.limit)
		}
	}
	if _, err := os.Stat(canary); !os.IsNotExist(err) {
		t.Fatalf("validation accessed Git before adapter routing: %v", err)
	}
}

func TestBuildQueryMatchesEagerAuthorityReceiptAndLoadsOnlyClosure(t *testing.T) {
	root := authorityRepository(t)
	for index := 0; index < 24; index++ {
		writeTestFile(t, root, fmt.Sprintf("internal/corpus/file%02d.go", index), "package corpus\n\nvar Payload = \""+strings.Repeat("x", 4096)+"\"\n")
	}
	writeTestFile(t, root, "docs/agent-workflows/unused.md", "# Unused\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "add representative corpus")

	eager, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	selective, err := BuildQuery(context.Background(), root, queryTaskFixture)
	if err != nil {
		t.Fatal(err)
	}
	eagerReceipt, err := QueryAuthorityStart(context.Background(), eager, queryTaskFixture, 1)
	if err != nil {
		t.Fatal(err)
	}
	selectiveReceipt, err := QueryAuthorityStart(context.Background(), selective, queryTaskFixture, 1)
	if err != nil {
		t.Fatal(err)
	}
	eagerBytes, err := CanonicalJSON(eagerReceipt)
	if err != nil {
		t.Fatal(err)
	}
	selectiveBytes, err := CanonicalJSON(selectiveReceipt)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(selectiveBytes, eagerBytes) {
		t.Fatalf("selective receipt differs from eager\nselective=%s\neager=%s", selectiveBytes, eagerBytes)
	}
	if len(selective.Sources) != len(eager.Sources) {
		t.Fatalf("source inventory differs: selective=%d eager=%d", len(selective.Sources), len(eager.Sources))
	}
	for _, reference := range []string{"AGENTS.md", "script/context-packet.sh", "script/roadmap.sh"} {
		source := selective.Sources[reference]
		if len(source.Data) == 0 {
			t.Fatalf("closure source %q did not retain immutable bytes", reference)
		}
	}
	for index := 0; index < 24; index++ {
		if len(selective.Sources[fmt.Sprintf("internal/corpus/file%02d.go", index)].Data) != 0 {
			t.Fatalf("non-authority corpus blob %d was loaded", index)
		}
	}
}

func TestBuildQueryMatchesEagerSHA256AuthorityReceipt(t *testing.T) {
	root := t.TempDir()
	command := exec.Command("git", "init", "--object-format=sha256", "-q", root)
	if output, err := command.CombinedOutput(); err != nil {
		t.Skipf("Git SHA-256 repository is unavailable: %v: %s", err, output)
	}
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	for relative, content := range map[string]string{
		"AGENTS.md": "# Project instructions\n\nThe roadmap is the only active work queue.\n" +
			"Run `make orient`, then `script/context-packet.sh --ticket ID`.\n" +
			"Use `script/roadmap.sh` and run the required workflow gates.\n",
		"script/context-packet.sh": "#!/bin/sh\nexit 0\n",
		"script/roadmap.sh":        "#!/bin/sh\nexit 0\n",
	} {
		writeTestFile(t, root, relative, content)
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "initial SHA-256 authority fixture")
	eager, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	selective, err := BuildQuery(context.Background(), root, queryTaskFixture)
	if err != nil {
		t.Fatal(err)
	}
	eagerReceipt, err := QueryAuthorityStart(context.Background(), eager, queryTaskFixture, 1)
	if err != nil {
		t.Fatal(err)
	}
	selectiveReceipt, err := QueryAuthorityStart(context.Background(), selective, queryTaskFixture, 1)
	if err != nil {
		t.Fatal(err)
	}
	eagerBytes, err := CanonicalJSON(eagerReceipt)
	if err != nil {
		t.Fatal(err)
	}
	selectiveBytes, err := CanonicalJSON(selectiveReceipt)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(selectiveBytes, eagerBytes) || selective.ObjectFormat != "sha256" {
		t.Fatalf("selective=%s eager=%s format=%s", selectiveBytes, eagerBytes, selective.ObjectFormat)
	}
}

func TestBuildQueryPreservesAuthorityFailuresAndPinnedBytes(t *testing.T) {
	t.Run("generated inventory and reference", func(t *testing.T) {
		root := authorityRepository(t)
		agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
		if err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, root, "AGENTS.md", string(agents)+"See script/generated.sh.\n")
		writeTestFile(t, root, "internal/unrelated.go", "// Code generated by fixture DO NOT EDIT.\npackage internal\n")
		writeTestFile(t, root, "script/generated.sh", "# Code generated by fixture DO NOT EDIT.\n")
		testGit(t, root, "add", ".")
		testGit(t, root, "commit", "-qm", "add generated inventory fixtures")
		eager, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		selective, err := BuildQuery(context.Background(), root, queryTaskFixture)
		if err != nil {
			t.Fatal(err)
		}
		if !exclusionIn(selective.Exclusions, Exclusion{Path: "internal/unrelated.go", Reason: "generated-file header excluded"}) ||
			!exclusionIn(selective.Exclusions, Exclusion{Path: "script/generated.sh", Reason: "generated-file header excluded"}) {
			t.Fatalf("selective exclusions = %#v", selective.Exclusions)
		}
		eagerReceipt, err := QueryAuthorityStart(context.Background(), eager, queryTaskFixture, 1)
		if err != nil {
			t.Fatal(err)
		}
		selectiveReceipt, err := QueryAuthorityStart(context.Background(), selective, queryTaskFixture, 1)
		if err != nil {
			t.Fatal(err)
		}
		eagerBytes, err := CanonicalJSON(eagerReceipt)
		if err != nil {
			t.Fatal(err)
		}
		selectiveBytes, err := CanonicalJSON(selectiveReceipt)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(selectiveBytes, eagerBytes) {
			t.Fatalf("selective receipt differs from eager\nselective=%s\neager=%s", selectiveBytes, eagerBytes)
		}
		result := anySlice(selectiveReceipt["results"])[0].(map[string]any)
		if stringIn(stringsField(result["references"]), "script/generated.sh") {
			t.Fatalf("generated reference was retained: %#v", result["references"])
		}
	})
	t.Run("missing reference", func(t *testing.T) {
		root := authorityRepository(t)
		eager, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		selective, err := BuildQuery(context.Background(), root, queryTaskFixture)
		if err != nil {
			t.Fatal(err)
		}
		for _, index := range []*Index{eager, selective} {
			record := index.Documents["AGENTS.md"]
			record.Fields["references"] = []string{"missing/tool.sh"}
			index.Documents["AGENTS.md"] = record
		}
		_, eagerErr := QueryAuthorityStart(context.Background(), eager, queryTaskFixture, 1)
		_, selectiveErr := QueryAuthorityStart(context.Background(), selective, queryTaskFixture, 1)
		if eagerErr == nil || selectiveErr == nil || eagerErr.Error() != selectiveErr.Error() {
			t.Fatalf("eager=%v selective=%v", eagerErr, selectiveErr)
		}
	})
	t.Run("tied authorities", func(t *testing.T) {
		root := authorityRepository(t)
		agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
		if err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, root, "GEMINI.md", string(agents))
		testGit(t, root, "add", "GEMINI.md")
		testGit(t, root, "commit", "-qm", "add tied authority")
		eager, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		_, eagerErr := QueryAuthorityStart(context.Background(), eager, queryTaskFixture, 1)
		selective, err := BuildQuery(context.Background(), root, queryTaskFixture)
		if err != nil {
			t.Fatal(err)
		}
		_, selectiveErr := QueryAuthorityStart(context.Background(), selective, queryTaskFixture, 1)
		if eagerErr == nil || selectiveErr == nil || eagerErr.Error() != selectiveErr.Error() {
			t.Fatalf("eager=%v selective=%v", eagerErr, selectiveErr)
		}
	})
	t.Run("CRLF authority", func(t *testing.T) {
		root := authorityRepository(t)
		data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
		if err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, root, "AGENTS.md", strings.ReplaceAll(string(data), "\n", "\r\n"))
		testGit(t, root, "add", "AGENTS.md")
		testGit(t, root, "commit", "-qm", "use CRLF authority")
		eager, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		_, eagerErr := QueryAuthorityStart(context.Background(), eager, queryTaskFixture, 1)
		selective, err := BuildQuery(context.Background(), root, queryTaskFixture)
		if err != nil {
			t.Fatal(err)
		}
		_, selectiveErr := QueryAuthorityStart(context.Background(), selective, queryTaskFixture, 1)
		if eagerErr == nil || selectiveErr == nil || eagerErr.Error() != selectiveErr.Error() {
			t.Fatalf("eager=%v selective=%v", eagerErr, selectiveErr)
		}
	})
	t.Run("restored mtime authority edit", func(t *testing.T) {
		root := authorityRepository(t)
		identity, err := readIdentity(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(root, "AGENTS.md")
		metadata, err := os.Stat(file)
		if err != nil {
			t.Fatal(err)
		}
		original, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		changed := bytes.Replace(original, []byte("roadmap"), []byte("Roadmap"), 1)
		if err := os.WriteFile(file, changed, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(file, metadata.ModTime(), metadata.ModTime()); err != nil {
			t.Fatal(err)
		}
		index, err := buildQueryAttempt(context.Background(), root, identity, nil, "", strings.ToLower(queryTaskFixture), terms(queryTaskFixture))
		if err != nil {
			t.Fatal(err)
		}
		if got := string(index.Sources["AGENTS.md"].Data); got != string(original) {
			t.Fatalf("selective index retained mutable authority bytes: %q", got)
		}
		if len(index.DirtyPaths) != 0 {
			t.Fatalf("status-clean path changed freshness: %v", index.DirtyPaths)
		}
	})
	t.Run("restored mtime non-authority edit falls back to the tree blob", func(t *testing.T) {
		root := authorityRepository(t)
		const relative = "internal/corpus/file.go"
		const original = "package corpus\n\nvar Payload = \"old\"\n"
		writeTestFile(t, root, relative, original)
		testGit(t, root, "add", relative)
		testGit(t, root, "commit", "-qm", "add corpus file")
		identity, err := readIdentity(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(root, filepath.FromSlash(relative))
		metadata, err := os.Stat(file)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("package corpus\n\nvar Payload = \"new\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(file, metadata.ModTime(), metadata.ModTime()); err != nil {
			t.Fatal(err)
		}
		entries, err := readTreeEntries(context.Background(), root, identity)
		if err != nil {
			t.Fatal(err)
		}
		exclusions, candidates, _ := admittedEntries(entries)
		dirty := map[string]struct{}{}
		sources, _, loaded, err := querySourceInventory(context.Background(), root, identity, candidates, exclusions, dirty)
		if err != nil {
			t.Fatal(err)
		}
		if got := string(loaded[relative]); got != original {
			t.Fatalf("tree fallback bytes = %q, want %q", got, original)
		}
		if len(dirty) != 0 {
			t.Fatalf("status-clean path changed freshness: %v", keys(dirty))
		}
		if source, exists := sources[relative]; !exists || len(source.Data) != 0 {
			t.Fatalf("non-authority source = %#v", source)
		}
	})
	t.Run("equal-content reference paths are independently pinned", func(t *testing.T) {
		root := authorityRepository(t)
		identity, err := readIdentity(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(root, "script", "context-packet.sh")
		metadata, err := os.Stat(file)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("#!/bin/sh\nexit 1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(file, metadata.ModTime(), metadata.ModTime()); err != nil {
			t.Fatal(err)
		}
		index, err := buildQueryAttempt(context.Background(), root, identity, nil, "", strings.ToLower(queryTaskFixture), terms(queryTaskFixture))
		if err != nil {
			t.Fatal(err)
		}
		for _, reference := range []string{"script/context-packet.sh", "script/roadmap.sh"} {
			if got := string(index.Sources[reference].Data); got != "#!/bin/sh\nexit 0\n" {
				t.Fatalf("reference %q bytes = %q", reference, got)
			}
		}
		if len(index.DirtyPaths) != 0 {
			t.Fatalf("status-clean path changed freshness: %v", index.DirtyPaths)
		}
	})
}

func TestValidateQueryBlobAdmissionUsesFullEagerBound(t *testing.T) {
	entries := []treeEntry{{path: "left.go", size: maxBatchBytes - 128}, {path: "right.go", size: 1}}
	err := validateQueryBlobAdmission(entries)
	var queryErr *Error
	if !errors.As(err, &queryErr) || queryErr.Code != "unsupported-query-repository" || !strings.HasPrefix(queryErr.Message, "authority-start query index sources total 134217601 bytes in 2 files") {
		t.Fatalf("error = %#v", err)
	}
}

func TestBuildQueryBlobAdmissionAndGitProcessCeiling(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test wrapper uses a POSIX shell")
	}
	root := authorityRepository(t)
	for index := 0; index < 16; index++ {
		writeTestFile(t, root, fmt.Sprintf("internal/corpus/file%02d.go", index), "package corpus\n\nvar Payload = \""+strings.Repeat("x", 2048)+"\"\n")
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "add bounded corpus")
	want := map[string]bool{
		testGit(t, root, "rev-parse", "HEAD:AGENTS.md"):                true,
		testGit(t, root, "rev-parse", "HEAD:script/context-packet.sh"): true,
		testGit(t, root, "rev-parse", "HEAD:script/roadmap.sh"):        true,
	}
	blocked := testGit(t, root, "rev-parse", "HEAD:internal/corpus/file00.go")
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin, log := t.TempDir(), t.TempDir()
	wrapper := fmt.Sprintf(`#!/bin/sh
printf . >> %s
for argument in "$@"; do
  if [ "$argument" = "cat-file" ]; then
    serial=0
    if [ -f %s ]; then serial=$(sed -n '1p' %s); fi
    serial=$((serial + 1))
    printf '%%s\n' "$serial" > %s
    tee %s/blob-$serial | %s "$@"
    exit $?
  fi
done
exec %s "$@"
`, strconv.Quote(filepath.Join(log, "calls")), strconv.Quote(filepath.Join(log, "serial")), strconv.Quote(filepath.Join(log, "serial")),
		strconv.Quote(filepath.Join(log, "serial")), strconv.Quote(log), strconv.Quote(realGit), strconv.Quote(realGit))
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if _, err := BuildQuery(context.Background(), root, queryTaskFixture); err != nil {
		t.Fatal(err)
	}
	calls, err := os.ReadFile(filepath.Join(log, "calls"))
	if err != nil {
		t.Fatal(err)
	}
	if count := len(calls); count > 7 {
		t.Fatalf("Git processes = %d, want <= 7", count)
	}
	entries, err := os.ReadDir(log)
	if err != nil {
		t.Fatal(err)
	}
	admitted := map[string]bool{}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "blob-") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(log, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, oid := range strings.Fields(string(raw)) {
			admitted[oid] = true
		}
	}
	if len(admitted) != len(want) {
		t.Fatalf("admitted blob identities = %v, want %v", admitted, want)
	}
	if admitted[blocked] {
		t.Fatalf("non-authority blob %s was admitted", blocked)
	}
	for oid := range want {
		if !admitted[oid] {
			t.Fatalf("required closure blob %s was not admitted", oid)
		}
	}
}

// TestPythonLowerSplitsTheDottedCapitalIAsTheOracleDoes: Python's str.lower
// maps U+0130 to "i" plus U+0307, so the oracle tokenizes "İstanbul" as
// "stanbul"; Go's simple case mapping would keep "istanbul".
func TestPythonLowerSplitsTheDottedCapitalIAsTheOracleDoes(t *testing.T) {
	values := terms(pythonLower("İstanbul Parser fix for the naïve tokenizer AİB"))
	for _, want := range []string{"stanbul", "parser", "fix", "na", "ve", "tokenizer", "ai"} {
		if _, ok := values[want]; !ok {
			t.Errorf("missing %q in %v", want, values)
		}
	}
	for _, unwanted := range []string{"istanbul", "aib", "naïve"} {
		if _, ok := values[unwanted]; ok {
			t.Errorf("unexpected %q in %v", unwanted, values)
		}
	}
}

// TestValidateQueryCommandAdmitsEveryOracleTask: the standalone profiles
// classify the intent and bound the limit; no UTF-8 task and no intent is
// refused before the repository is opened.
func TestValidateQueryCommandAdmitsEveryOracleTask(t *testing.T) {
	for _, test := range []struct {
		task   string
		limit  int
		intent string
	}{
		{"fix café parser", 10, "repository"},
		{"corvint agent context roadmap", 50, "agent-tooling"},
		{queryTaskFixture, 10, "project-operations"},
		{queryTaskFixture + " — café", 50, "project-operations"},
	} {
		intent, err := ValidateQueryCommand(test.task, test.limit)
		if err != nil || intent != test.intent {
			t.Errorf("ValidateQueryCommand(%q, %d) = %q, %v", test.task, test.limit, intent, err)
		}
	}
	for _, test := range []struct {
		task  string
		limit int
	}{
		{queryTaskFixture, 0}, {queryTaskFixture, 51}, {"fix café parser", 51}, {"corvint agent context roadmap", 0}, {"  ", 1},
	} {
		if _, err := ValidateQueryCommand(test.task, test.limit); err == nil {
			t.Errorf("ValidateQueryCommand(%q, %d) accepted", test.task, test.limit)
		}
	}
	var queryErr *Error
	if _, err := ValidateQueryCommand("fix caf\xe9 parser", 10); !errors.As(err, &queryErr) || queryErr.Code != "unsupported-query-task" {
		t.Errorf("malformed UTF-8 accepted: %v", err)
	}
	if err := ValidateQueryAuthorityStart("fix café parser", 1); err == nil {
		t.Error("authority-start accepted a repository task")
	}
}

// TestAuthorityStartQueryAddsAdvisoryLearnedPathsUpToThree pins the oracle's
// project-operations packet above limit 1: the instruction result first, then
// learned paths from history commits sharing two task terms, at most three.
func TestAuthorityStartQueryAddsAdvisoryLearnedPathsUpToThree(t *testing.T) {
	root := authorityRepository(t)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	task := "orient the roadmap workflow gates for the initial authority fixture"
	kinds := func(limit int) []string {
		receipt, err := QueryAuthorityStart(context.Background(), index, task, limit)
		if err != nil {
			t.Fatalf("limit %d: %v", limit, err)
		}
		if receipt["request"].(map[string]any)["limit"] != limit {
			t.Fatalf("limit %d echoed as %v", limit, receipt["request"])
		}
		found := make([]string, 0)
		for _, raw := range anySlice(receipt["results"]) {
			result := raw.(map[string]any)
			found = append(found, result["kind"].(string)+":"+result["id"].(string))
		}
		return found
	}
	if got := kinds(1); strings.Join(got, ",") != "instructions:AGENTS.md" {
		t.Fatalf("limit 1: %v", got)
	}
	two := kinds(2)
	if len(two) != 2 || two[0] != "instructions:AGENTS.md" || !strings.HasPrefix(two[1], "learned-path:") {
		t.Fatalf("limit 2: %v", two)
	}
	ten := kinds(10)
	if len(ten) != 4 || ten[0] != "instructions:AGENTS.md" {
		t.Fatalf("limit 10: %v", ten)
	}
	for _, entry := range ten[1:] {
		if !strings.HasPrefix(entry, "learned-path:") {
			t.Fatalf("limit 10: %v", ten)
		}
	}
	if got := kinds(50); strings.Join(got, ",") != strings.Join(ten, ",") {
		t.Fatalf("limit 50 differs from limit 10: %v vs %v", got, ten)
	}
}

func TestAuthorityStartHistorySkipsTheShallowBoundaryCommit(t *testing.T) {
	source := authorityRepository(t)
	writeTestFile(t, source, "internal/history/boundary.go", "package history\n\nfunc Boundary() {}\n")
	testGit(t, source, "add", ".")
	testGit(t, source, "commit", "-qm", "active work queue workflow gates")
	writeTestFile(t, source, "script/roadmap.sh", "#!/bin/sh\n# refreshed\nexit 0\n")
	testGit(t, source, "add", ".")
	testGit(t, source, "commit", "-qm", "refresh helper")
	root := filepath.Join(t.TempDir(), "clone")
	testGit(t, source, "clone", "-q", "--depth", "2", "file://"+source, root)
	if shallow := testGit(t, root, "rev-parse", "--is-shallow-repository"); shallow != "true" {
		t.Fatalf("fixture is not shallow: %s", shallow)
	}

	index, err := BuildQuery(context.Background(), root, queryTaskFixture)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := QueryAuthorityStart(context.Background(), index, queryTaskFixture, 10)
	if err != nil {
		t.Fatal(err)
	}
	learning := receipt["learning"].(map[string]any)
	if learning["history_commits_considered"] != 1 || learning["matched_history_commits"] != 0 || learning["advisory_candidates"] != 0 {
		t.Fatalf("shallow boundary contributed to history learning: %v", learning)
	}
	for _, raw := range anySlice(receipt["results"]) {
		if result := raw.(map[string]any); result["kind"] == "learned-path" {
			t.Fatalf("shallow boundary produced learned path: %v", result)
		}
	}
}

// TestAuthorityStartQueryRanksASecondInstructionDocumentBeforeLearnedPaths:
// the oracle admits two instruction documents for project operations, then
// the advisory learned paths.
func TestAuthorityStartQueryRanksASecondInstructionDocumentBeforeLearnedPaths(t *testing.T) {
	root := authorityRepository(t)
	writeTestFile(t, root, "CLAUDE.md", "# Claude notes\n\nTake the next roadmap ticket only after the workflow gates pass.\n")
	testGit(t, root, "add", "CLAUDE.md")
	testGit(t, root, "commit", "-qm", "add roadmap workflow notes")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	ids := func(limit int) string {
		receipt, err := QueryAuthorityStart(context.Background(), index, "orient the roadmap workflow gates for the initial authority fixture", limit)
		if err != nil {
			t.Fatalf("limit %d: %v", limit, err)
		}
		found := make([]string, 0)
		for _, raw := range anySlice(receipt["results"]) {
			result := raw.(map[string]any)
			found = append(found, result["kind"].(string)+":"+result["id"].(string))
		}
		return strings.Join(found, ",")
	}
	if got := ids(1); got != "instructions:AGENTS.md" {
		t.Fatalf("limit 1: %s", got)
	}
	if got := ids(2); got != "instructions:AGENTS.md,instructions:CLAUDE.md" {
		t.Fatalf("limit 2: %s", got)
	}
	if got := ids(10); !strings.HasPrefix(got, "instructions:AGENTS.md,instructions:CLAUDE.md,learned-path:") || strings.Count(got, "learned-path:") != 3 {
		t.Fatalf("limit 10: %s", got)
	}
}

// TestEvalQueryKeepsAProjectOperationsInstructionResultOnItsClassifyingWords
// pins decision 0014: AGENTS.md shares one word with the task, but the task's
// project-operations words classify it, so the floor does not withdraw.
func TestEvalQueryKeepsAProjectOperationsInstructionResultOnItsClassifyingWords(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	writeTestFile(t, root, "AGENTS.md", "# Project instructions\n\nRun the gate before merging.\n")
	writeTestFile(t, root, "main.go", "package main\n\nfunc main() {}\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "seed")
	index, err := BuildEval(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := EvalQuery(context.Background(), index, "orient me on the contributor workflow and roadmap gates", 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	results := anySlice(receipt["results"])
	if receipt["state"] != "READY" || len(results) != 1 || results[0].(map[string]any)["id"] != "AGENTS.md" {
		t.Fatalf("state=%v results=%v abstention=%v", receipt["state"], results, receipt["abstention"])
	}
	withdrawn, err := EvalQuery(context.Background(), index, "merging strategy for the release gate", 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if withdrawn["state"] != "OUT_OF_SCOPE" {
		t.Fatalf("repository intent one-word match state=%v results=%v", withdrawn["state"], withdrawn["results"])
	}
}

func TestEvalQueryCreditsProjectOperationsClassifyingWordsAsInstructionSupport(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	writeTestFile(t, root, "AGENTS.md", "# Project instructions\n\nFollow the workflow.\n")
	writeTestFile(t, root, "main.go", "package main\n\nfunc main() {}\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "seed")
	index, err := BuildEval(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}

	task := "review workflow gates"
	intent := evalInferIntent(task)
	queryTerms := evalRelevanceTerms(terms(task), intent)
	documents, err := evalRankDocuments(index, task, queryTerms, intent)
	if err != nil {
		t.Fatal(err)
	}
	if len(documents) != 1 || documents[0].id != "AGENTS.md" {
		t.Fatalf("documents=%v", documents)
	}
	for _, term := range intent.matched {
		if _, credited := documents[0].support[term]; !credited {
			t.Fatalf("support=%v missing classifying term %q", keys(documents[0].support), term)
		}
	}
	if got, want := len(documents[0].support), len(intersectionSet(intent.matched)); got != want {
		t.Fatalf("support=%v count=%d want=%d", keys(documents[0].support), got, want)
	}

	receipt, err := EvalQuery(context.Background(), index, task, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	coverage := receipt["coverage"].(map[string]any)
	if receipt["state"] != "READY" || coverage["included_results"] != 1 {
		t.Fatalf("state=%v coverage=%v results=%v", receipt["state"], coverage, receipt["results"])
	}
}
