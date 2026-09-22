package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// proveFixtureRepository is the authority-start query fixture with an ignored
// `.corvint/` ledger seeded, so a digest of the tree (ignored files included)
// proves the command wrote nothing.
func proveFixtureRepository(t *testing.T) string {
	t.Helper()
	root := queryCLIRepository(t)
	ignore := filepath.Join(root, ".gitignore")
	if err := os.WriteFile(ignore, []byte(".context-corvint/\n.corvint/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "add", ".gitignore")
	affectedGit(t, root, "commit", "-qm", "ignore the ledger")
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".corvint", "self-observations.jsonl"), []byte("{\"seed\":1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func runProveArguments(t *testing.T, arguments ...string) ([]byte, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), arguments, strings.NewReader(""), &stdout, &stderr)
	return stdout.Bytes(), stderr.String(), code
}

func runProveCLI(t *testing.T, root string, arguments ...string) (map[string]any, []byte, string, int) {
	t.Helper()
	return runProveCLIContext(t, context.Background(), root, arguments...)
}

func runProveCLIEnvironment(t *testing.T, environment []string, root string, arguments ...string) (map[string]any, []byte, string, int) {
	t.Helper()
	arguments = append([]string{"--root", root, "prove"}, arguments...)
	stdout, stderr, code := runCandidateWithEnvironment(t, "", environment, arguments...)
	var receipt map[string]any
	if code == 0 {
		if err := json.Unmarshal(stdout, &receipt); err != nil {
			t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
		}
	}
	return receipt, stdout, stderr, code
}

// runProveCLIContext is runProveCLI on a caller's context, which may carry
// lowered bounds from proveBoundsContext.
func runProveCLIContext(t *testing.T, ctx context.Context, root string, arguments ...string) (map[string]any, []byte, string, int) {
	t.Helper()
	var stdoutBuffer, stderrBuffer bytes.Buffer
	code := runContext(ctx, append([]string{"--root", root, "prove"}, arguments...), strings.NewReader(""), &stdoutBuffer, &stderrBuffer)
	stdout, stderr := stdoutBuffer.Bytes(), stderrBuffer.String()
	var receipt map[string]any
	if code == 0 {
		if err := json.Unmarshal(stdout, &receipt); err != nil {
			t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
		}
	}
	return receipt, stdout, stderr, code
}

func proofRows(t *testing.T, receipt map[string]any) []map[string]any {
	t.Helper()
	proof, _ := receipt["proof"].(map[string]any)
	rows := mapsFromAny(proof["rows"])
	if len(rows) == 0 {
		t.Fatalf("proof carries no rows: %v", receipt)
	}
	return rows
}

func proofField(t *testing.T, receipt map[string]any, field string) int {
	t.Helper()
	proof, _ := receipt["proof"].(map[string]any)
	return integerAt(proof, field)
}

func assertProofClaims(t *testing.T, receipt map[string]any) {
	t.Helper()
	proof, _ := receipt["proof"].(map[string]any)
	want := map[string]any{
		falsifierHistory:   "the cited blob and line still exist at the packet revision; this is a check of the citation, not evidence that the row is relevant",
		falsifierReference: "the cited line holds the claimed import or identifier and the declaring blob declares it; this is a check of the reference, not evidence that the row is relevant",
		falsifierVerifier:  "the frozen CEM verifier accepted the map and this hunk's evidence is stable or relocated at the target",
		falsifierMutant:    "the cited test failed on a mutant of the changed lines and passed on the baseline",
		falsifierNone:      "nothing about this row was checked",
	}
	if !reflect.DeepEqual(proof["claims"], want) {
		t.Fatalf("proof claims = %#v, want %#v", proof["claims"], want)
	}
}

// proveBoundsContext returns a context whose prove invocations use the
// default bounds as changed by lower, so a test lowers a bound for its own
// invocations without writing a package variable.
func proveBoundsContext(lower func(*proveBounds)) context.Context {
	bounds := proveBoundsFrom(context.Background())
	lower(&bounds)
	return context.WithValue(context.Background(), proveBoundsKey{}, bounds)
}

func checkpointGitWrapper(t *testing.T, body string) []string {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	wrapper := "#!/bin/sh\n" + body + "\nexec " + shellQuote(realGit) + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(directory, "git"), []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	return []string{"PATH=" + directory + string(os.PathListSeparator) + os.Getenv("PATH")}
}

func TestProveEmbedsTheQueryPacketUnchangedAndWritesNothing(t *testing.T) {
	t.Parallel()
	root := proveFixtureRepository(t)
	before := treeDigest(t, root)
	receipt, first, stderr, code := runProveCLI(t, root, "--task", authorityStartPrompt, "--limit", "1")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if receipt["profile"] != proveProfile || receipt["mutates"] != false || receipt["tool"] != "prove" || receipt["state"] != "READY" {
		t.Fatalf("envelope: %v", receipt)
	}
	for _, row := range proofRows(t, receipt) {
		if row["falsifier"] != falsifierHistory || row["falsified"] != falsifiedPass || row["result"] != "AGENTS.md" {
			t.Fatalf("row %v: want %s/%s on AGENTS.md", row, falsifierHistory, falsifiedPass)
		}
	}
	if proofField(t, receipt, "proven_results") != 1 || proofField(t, receipt, "unproven_results") != 0 || proofField(t, receipt, "failed_results") != 0 {
		t.Fatalf("proof: %v", receipt["proof"])
	}
	assertProofClaims(t, receipt)
	queryStdout, _, queryCode := runProveArguments(t, "--root", root, "query", "--task", authorityStartPrompt, "--limit", "1")
	var queryEnvelope map[string]any
	if queryCode != 0 || json.Unmarshal(queryStdout, &queryEnvelope) != nil {
		t.Fatalf("query: exit %d %s", queryCode, queryStdout)
	}
	embedded, _ := json.Marshal(receipt["packet"])
	expected, _ := json.Marshal(queryEnvelope["context"])
	if !bytes.Equal(embedded, expected) {
		t.Fatalf("embedded packet differs from the query packet:\n%s\n%s", embedded, expected)
	}
	secondReceipt, second, _, _ := runProveCLI(t, root, "--task", authorityStartPrompt, "--limit", "1")
	assertProofClaims(t, secondReceipt)
	if !bytes.Equal(first, second) {
		t.Fatal("two runs over one tree differ")
	}
	if after := treeDigest(t, root); after != before {
		t.Fatal("prove changed the repository or its ignored ledger")
	}
}

func TestProveDirtyPrimaryRowLeavesTheResultUnproven(t *testing.T) {
	t.Parallel()
	root := proveFixtureRepository(t)
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("uncommitted replacement\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	receipt, _, stderr, code := runProveCLI(t, root, "--task", authorityStartPrompt, "--limit", "1")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if receipt["state"] != "UNPROVEN" {
		t.Fatalf("state %v, want UNPROVEN", receipt["state"])
	}
	packet := receipt["packet"].(map[string]any)
	freshness := packet["freshness"].(map[string]any)
	if freshness["state"] != "mixed-worktree" || integerAt(freshness, "mixed_path_count") != 1 {
		t.Fatalf("freshness %v", freshness)
	}
	verdicts := map[string]string{}
	for _, row := range proofRows(t, receipt) {
		verdicts[stringAt(row, "path")] = stringAt(row, "falsified")
	}
	if verdicts["AGENTS.md"] != falsifiedNotRun {
		t.Fatalf("dirty AGENTS.md rows: %v", verdicts)
	}
	if verdicts["script/roadmap.sh"] != falsifiedPass || verdicts["script/context-packet.sh"] != falsifiedPass {
		t.Fatalf("clean referenced scripts: %v", verdicts)
	}
	if proofField(t, receipt, "proven_results") != 0 || proofField(t, receipt, "unproven_results") != 1 {
		t.Fatalf("a NOT_RUN primary row must leave the result unproven: %v", receipt["proof"])
	}
}

func TestProveVocabularyOnlyPacketIsCitedNotProven(t *testing.T) {
	t.Parallel()
	root := proveFixtureRepository(t)
	receipt, _, stderr, code := runProveCLI(t, root, "--task", repositoryQueryTask)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	for _, row := range proofRows(t, receipt) {
		if row["authority"] != "syntax" || row["falsifier"] != falsifierHistory || row["falsified"] != falsifiedPass {
			t.Fatalf("row %v: want syntax/%s/%s", row, falsifierHistory, falsifiedPass)
		}
	}
	packet, _ := receipt["packet"].(map[string]any)
	if packet["state"] != "READY" || receipt["state"] != "CITED" {
		t.Fatalf("packet state %v, prove state %v", packet["state"], receipt["state"])
	}
	coverage, _ := packet["coverage"].(map[string]any)
	if integerAt(coverage, "authoritative_results") != 0 || proofField(t, receipt, "proven_results") == 0 {
		t.Fatalf("a syntax-only packet must keep authority at zero while citing its rows: %v %v", coverage, receipt["proof"])
	}
	assertProofClaims(t, receipt)
}

func TestFalsifyRowVerdicts(t *testing.T) {
	t.Parallel()
	cited := map[string]citedBlob{"docs/a.md": {oid: "abc", objectType: "blob", lines: 3}, "docs": {oid: "def", objectType: "tree", lines: 1}}
	dirty := map[string]struct{}{"docs/dirty.md": {}}
	history := func(path, blob string, line int) proveRow {
		return proveRow{Falsifier: falsifierHistory, Path: path, BlobHash: blob, Line: line}
	}
	vocabulary := proveRow{Kind: "symbol", Authority: "syntax", Path: "docs/a.md", BlobHash: "zzz", Line: 1}
	vocabulary.Falsifier = falsifierFor(vocabulary)
	for _, test := range []struct {
		name string
		row  proveRow
		want string
	}{
		{"pass", history("docs/a.md", "abc", 3), falsifiedPass},
		{"wrong blob", history("docs/a.md", "zzz", 1), falsifiedFail},
		{"line past end", history("docs/a.md", "abc", 4), falsifiedFail},
		{"line zero", history("docs/a.md", "abc", 0), falsifiedFail},
		{"missing path", history("docs/gone.md", "abc", 1), falsifiedFail},
		{"not a blob", history("docs", "def", 1), falsifiedFail},
		{"syntax vocabulary citing replaced blob", vocabulary, falsifiedFail},
		{"dirty", history("docs/dirty.md", "abc", 1), falsifiedNotRun},
		{"newline path", history("docs/a\n.md", "abc", 1), falsifiedNotRun},
		{"none", proveRow{Falsifier: falsifierNone, Path: "docs/a.md", BlobHash: "abc", Line: 1}, falsifiedNotRun},
	} {
		if got := falsifyRow(test.row, cited, dirty); got != test.want {
			t.Errorf("%s: got %s, want %s", test.name, got, test.want)
		}
	}
	if paths := citedPaths([]proveRow{history("docs/a\n.md", "abc", 1), history("docs/a.md", "abc", 1)}, false); len(paths) != 1 || paths[0] != "docs/a.md" {
		t.Fatalf("a newline path must never reach git: %q", paths)
	}
	rows := []proveRow{
		{Kind: "spec", Result: "r", Falsifier: falsifierHistory, Falsified: falsifiedPass},
		{Kind: "spec", Result: "r", Falsifier: falsifierNone, Falsified: falsifiedNotRun},
	}
	if resultVerdict("spec", "r", rows) != falsifiedPass {
		t.Error("PASS rows plus vocabulary rows must prove a result")
	}
	if resultVerdict("spec", "r", append(rows, proveRow{Kind: "spec", Result: "r", Falsifier: falsifierHistory, Falsified: falsifiedNotRun})) != falsifiedNotRun {
		t.Error("a NOT_RUN falsifier row must leave a result unproven")
	}
	if resultVerdict("spec", "r", append(rows, proveRow{Kind: "spec", Result: "r", Falsifier: falsifierHistory, Falsified: falsifiedFail})) != falsifiedFail {
		t.Error("any FAIL must fail a result")
	}
	if resultVerdict("spec", "r", []proveRow{{Kind: "spec", Result: "other", Falsifier: falsifierHistory, Falsified: falsifiedPass}}) != falsifiedNotRun {
		t.Error("another result's rows must not prove this one")
	}
	shared := append(rows, proveRow{Kind: "test", Result: "r", Falsifier: falsifierMutant, Falsified: falsifiedFail})
	if resultVerdict("spec", "r", shared) != falsifiedPass || resultVerdict("test", "r", shared) != falsifiedFail {
		t.Error("two results sharing an id must each answer for their own rows")
	}
}

func TestParseCatFileBatchHandlesMissingAndMalformedObjects(t *testing.T) {
	t.Parallel()
	stream := []byte("abc blob 4\nab\nc\ndeadbeef:docs/gone.md missing\n0123 blob 0\n\n")
	cited, err := parseCatFileBatch(stream, "deadbeef", []string{"docs/a.md", "docs/gone.md", "docs/empty.md"})
	if err != nil {
		t.Fatal(err)
	}
	if blob := cited["docs/a.md"]; blob.oid != "abc" || blob.objectType != "blob" || blob.lines != 2 || string(blob.content) != "ab\nc" {
		t.Fatalf("docs/a.md: %+v", cited["docs/a.md"])
	}
	if _, present := cited["docs/gone.md"]; present {
		t.Fatal("a missing object must not be cited")
	}
	if cited["docs/empty.md"].lines != 1 {
		t.Fatalf("an empty blob counts as one line: %+v", cited["docs/empty.md"])
	}
	for _, malformed := range [][]byte{[]byte("abc blob 10\nshort\n"), []byte("abc blob\n"), []byte("abc blob 2\nab")} {
		if _, err := parseCatFileBatch(malformed, "deadbeef", []string{"docs/a.md"}); err == nil {
			t.Errorf("%q: malformed stream accepted", malformed)
		}
	}
}

func TestProveRejectsBadArgumentsWithoutOutputOrLedgerWrite(t *testing.T) {
	t.Parallel()
	root := proveFixtureRepository(t)
	before := treeDigest(t, root)
	for _, arguments := range [][]string{{}, {"--task"}, {"--task", authorityStartPrompt, "--bogus"}, {"--task", authorityStartPrompt, "--limit", "x"}} {
		_, stdout, stderr, code := runProveCLI(t, root, arguments...)
		if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, "invalid-arguments") {
			t.Fatalf("%v: exit=%d stdout=%q stderr=%q", arguments, code, stdout, stderr)
		}
	}
	if _, stdout, stderr, code := runProveCLI(t, filepath.Join(root, "missing"), "--task", authorityStartPrompt); code != 2 || len(stdout) != 0 || stderr == "" {
		t.Fatalf("non-repository root: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if after := treeDigest(t, root); after != before {
		t.Fatal("a rejected prove invocation wrote to the repository or ledger")
	}
}

func TestProveAndAffectedHelpFormsReachTheHelpPath(t *testing.T) {
	t.Parallel()
	root := proveFixtureRepository(t)
	historyClaim := "the cited blob and line still exist at the packet revision; this is a check of the citation, not evidence that the row is relevant"
	if !strings.Contains(proveHelp, historyClaim) {
		t.Fatalf("prove help does not contain the history-consistent claim: %q", proveHelp)
	}
	for _, test := range []struct {
		arguments []string
		topic     string
		marker    string
	}{
		{[]string{"help", "prove"}, "prove", "falsifiable-packet/0"},
		{[]string{"prove", "--help"}, "prove", "falsifiable-packet/0"},
		{[]string{"--root", root, "prove", "--help"}, "prove", "falsifiable-packet/0"},
		{[]string{"--root", root, "affected", "--help"}, "affected", "affected-plan/0"},
	} {
		topic, requested, err := parseHelpInvocation(test.arguments)
		if err != nil || !requested || topic != test.topic {
			t.Fatalf("%v: topic=%q requested=%v err=%v", test.arguments, topic, requested, err)
		}
		stdout, stderr, code := runProveArguments(t, test.arguments...)
		if code != 0 || !strings.Contains(string(stdout), test.marker) {
			t.Fatalf("%v: exit=%d stdout=%q stderr=%q", test.arguments, code, stdout, stderr)
		}
	}
	if !strings.Contains(rootHelp, "prove") {
		t.Fatal("prove is not listed in root help")
	}
}
