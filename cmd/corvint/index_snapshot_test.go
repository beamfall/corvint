package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
)

type snapshotFileState struct {
	contents []byte
	mode     os.FileMode
	modified time.Time
}

func readSnapshotDirectoryState(t *testing.T, root string) map[string]snapshotFileState {
	t.Helper()
	directory := contextindex.SnapshotDirectory(root)
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	state := make(map[string]snapshotFileState, len(entries))
	for _, entry := range entries {
		path := filepath.Join(directory, entry.Name())
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		state[entry.Name()] = snapshotFileState{contents: contents, mode: info.Mode(), modified: info.ModTime()}
	}
	return state
}

func runIndexForTest(t *testing.T, root string, ifStale bool) []byte {
	t.Helper()
	arguments := []string{"--root", root, "index"}
	if ifStale {
		arguments = append(arguments, "--if-stale")
	}
	var stdout, stderr bytes.Buffer
	if code := runContext(context.Background(), arguments, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("index exit %d: %s", code, stderr.String())
	}
	return stdout.Bytes()
}

func writingIndexReceipt(t *testing.T, encoded []byte) map[string]any {
	t.Helper()
	var receipt map[string]any
	if err := json.Unmarshal(encoded, &receipt); err != nil {
		t.Fatal(err)
	}
	wantKeys := []string{"bytes", "command", "commit", "engine", "evicted", "mutates", "ok", "path", "profile", "sources", "symbols", "tree"}
	gotKeys := make([]string, 0, len(receipt))
	for key := range receipt {
		gotKeys = append(gotKeys, key)
	}
	slices.Sort(gotKeys)
	if !reflect.DeepEqual(gotKeys, wantKeys) || receipt["mutates"] != true || receipt["command"] != "index" {
		t.Fatalf("IDX-SNAP-V0-001 receipt = %s", encoded)
	}
	return receipt
}

func TestIndexIfStaleReceiptsAndFreshSnapshotIsUntouched(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	initial := writingIndexReceipt(t, runIndexForTest(t, root, false))
	before := readSnapshotDirectoryState(t, root)
	fresh := runIndexForTest(t, root, true)
	wantFresh, err := json.Marshal(map[string]any{
		"mutates": false,
		"state":   "fresh",
		"path":    initial["path"],
		"tree":    initial["tree"],
		"commit":  initial["commit"],
		"engine":  initial["engine"],
	})
	if err != nil {
		t.Fatal(err)
	}
	wantFresh = append(wantFresh, '\n')
	if !bytes.Equal(fresh, wantFresh) {
		t.Fatalf("IDX-SNAP-V0-011 fresh receipt:\n got %s want %s", fresh, wantFresh)
	}
	if after := readSnapshotDirectoryState(t, root); !reflect.DeepEqual(after, before) {
		t.Fatalf("IDX-SNAP-V0-011: fresh probe changed snapshot directory")
	}

	if err := os.WriteFile(filepath.Join(root, "cache", "demux.go"), []byte("package cache\n\nfunc Split(string) []string { return nil }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{{"add", "cache/demux.go"}, {"commit", "-qm", "change tree"}} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	stale := writingIndexReceipt(t, runIndexForTest(t, root, true))
	if stale["tree"] == initial["tree"] || stale["path"] == initial["path"] {
		t.Fatalf("IDX-SNAP-V0-011 stale receipt did not rebuild: %s", stale)
	}

	missingRoot := taskContextRepository(t)
	missing := writingIndexReceipt(t, runIndexForTest(t, missingRoot, true))
	if _, err := os.Stat(missing["path"].(string)); err != nil {
		t.Fatalf("IDX-SNAP-V0-011 missing snapshot was not built: %v", err)
	}
}

func TestIndexBootstrapsIgnoredObservationLedger(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	runIndexForTest(t, root, false)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--root", root, "impact", "README.csv"}, strings.NewReader(""), &stdout, &stderr); code != 2 {
		t.Fatalf("impact exit=%d stdout=%q stderr=%q", code, &stdout, &stderr)
	}
	ledger, err := os.ReadFile(filepath.Join(root, ".corvint", "self-observations.jsonl"))
	if err != nil || !bytes.Contains(ledger, []byte("unsupported-impact-path-suffix")) {
		t.Fatalf("ledger=%s err=%v", ledger, err)
	}
	for _, relative := range []string{
		".corvint/.gitignore",
		".corvint/index/probe.gob",
		".corvint/self-observations.jsonl",
		".corvint/.self-observations.probe",
	} {
		command := exec.Command("git", "check-ignore", "--no-index", "-q", relative)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("%s is not ignored: %v %s", relative, err, output)
		}
	}
	command := exec.Command("git", "status", "--porcelain", "--untracked-files=all")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil || len(output) != 0 {
		t.Fatalf("index and ledger dirtied repository: err=%v status=%q", err, output)
	}
}

func TestIndexWritesTheSnapshotThatContextReadsWithoutChangingAByte(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	arguments := []string{"--root", root, "context", "--task", "does `Split` keep empty keys", "--subject", "cache/demux.go"}
	var cold, stderr bytes.Buffer
	if code := runContext(context.Background(), arguments, strings.NewReader(""), &cold, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if _, err := os.Stat(contextindex.SnapshotDirectory(root)); !os.IsNotExist(err) {
		t.Fatalf("IDX-SNAP-V0-005: context created the snapshot directory: %v", err)
	}
	var receiptOut bytes.Buffer
	if code := runContext(context.Background(), []string{"--root", root, "index"}, strings.NewReader(""), &receiptOut, &stderr); code != 0 {
		t.Fatalf("index exit %d: %s", code, stderr.String())
	}
	var receipt map[string]any
	if err := json.Unmarshal(receiptOut.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt["mutates"] != true || receipt["sources"].(float64) < 3 {
		t.Fatalf("receipt = %s", receiptOut.String())
	}
	if _, err := os.Stat(receipt["path"].(string)); err != nil {
		t.Fatal(err)
	}
	before := readSnapshotDirectoryState(t, root)
	var warm bytes.Buffer
	if code := runContext(context.Background(), arguments, strings.NewReader(""), &warm, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if warm.String() != cold.String() {
		t.Fatalf("IDX-SNAP-V0-006: packet differs between miss and hit:\n%s\n%s", cold.String(), warm.String())
	}
	after := readSnapshotDirectoryState(t, root)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("IDX-SNAP-V0-005: a warm context run changed the snapshot directory:\nbefore=%+v\nafter=%+v", before, after)
	}
}

// TestQueryVerbsReadTheSnapshotWithoutChangingAByte pins IDX-SNAP-V0-008 for
// the two per-prompt verbs. Each ranks out of a build narrower than the
// snapshot -- `query` out of the authority-only query index, user-prompt out of
// the eval index -- so the receipt the snapshot serves is the assertion that
// the wider index is a substitute, not merely a faster one.
func TestQueryVerbsReadTheSnapshotWithoutChangingAByte(t *testing.T) {
	t.Parallel()
	root := queryCLIRepository(t)
	invocations := map[string]struct {
		arguments []string
		stdin     string
	}{
		"query":       {[]string{"--root", root, "query", "--task", authorityStartPrompt, "--limit", "1"}, ""},
		"user-prompt": {cliArguments(root, "user-prompt"), `{"task":"` + authorityStartPrompt + `"}`},
	}
	cold := map[string]string{}
	for name, invocation := range invocations {
		var stdout, stderr bytes.Buffer
		if code := runContext(context.Background(), invocation.arguments, strings.NewReader(invocation.stdin), &stdout, &stderr); code != 0 {
			t.Fatalf("%s exit %d: %s", name, code, stderr.String())
		}
		cold[name] = stdout.String()
	}
	if _, err := os.Stat(contextindex.SnapshotDirectory(root)); !os.IsNotExist(err) {
		t.Fatalf("IDX-SNAP-V0-005: a read verb created the snapshot directory: %v", err)
	}
	var receipt, stderr bytes.Buffer
	if code := runContext(context.Background(), []string{"--root", root, "index"}, strings.NewReader(""), &receipt, &stderr); code != 0 {
		t.Fatalf("index exit %d: %s", code, stderr.String())
	}
	for name, invocation := range invocations {
		var stdout bytes.Buffer
		if code := runContext(context.Background(), invocation.arguments, strings.NewReader(invocation.stdin), &stdout, &stderr); code != 0 {
			t.Fatalf("%s exit %d: %s", name, code, stderr.String())
		}
		if stdout.String() != cold[name] {
			t.Fatalf("IDX-SNAP-V0-008: %s differs between miss and hit:\n%s\n%s", name, cold[name], stdout.String())
		}
	}
}

// TestImpactAndFeatureReadTheSnapshotWithoutChangingAByte pins IDX-SNAP-V0-019:
// path impact (the reverse-import rule and the eval profile) and feature
// answer a dirty worktree's snapshot hit exactly as their build did, and the
// cold reads create no snapshot directory.
func TestImpactAndFeatureReadTheSnapshotWithoutChangingAByte(t *testing.T) {
	t.Parallel()
	root := ledgerRepository(t)
	if err := os.WriteFile(filepath.Join(root, "internal", "auth", "session.go"), []byte("package auth\n\nfunc EnforceSessionRevocation() bool { return false }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	invocations := [][]string{
		{"--root", root, "impact", "internal/auth/session.go"},
		{"--root", root, "impact", "testing/features.yaml"},
		{"--root", root, "feature", "session-revocation"},
	}
	run := func(arguments []string) string {
		var stdout, stderr bytes.Buffer
		if code := runContext(context.Background(), arguments, strings.NewReader(""), &stdout, &stderr); code != 0 {
			t.Fatalf("%v exit %d: %s", arguments, code, stderr.String())
		}
		return stdout.String()
	}
	cold := make([]string, len(invocations))
	for position, arguments := range invocations {
		cold[position] = run(arguments)
	}
	if _, err := os.Stat(contextindex.SnapshotDirectory(root)); !os.IsNotExist(err) {
		t.Fatalf("IDX-SNAP-V0-005: a read verb created the snapshot directory: %v", err)
	}
	run([]string{"--root", root, "index"})
	for position, arguments := range invocations {
		if warm := run(arguments); warm != cold[position] {
			t.Fatalf("IDX-SNAP-V0-019: %v differs between miss and hit:\n%s\n%s", arguments, cold[position], warm)
		}
	}
}

// TestProveKernelAndWitnessReadTheSnapshotWithoutChangingAByte pins
// IDX-SNAP-V0-020: prove's impact and change modes, kernel, kernel verify and
// witness answer an absent, a stale and a current snapshot with the same
// stdout, stderr and exit code.
func TestProveKernelAndWitnessReadTheSnapshotWithoutChangingAByte(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# Agent contract\n\nEvidence is pinned to immutable Git content.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "add a governing contract")
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(root, "pkg", "main.go"), []byte("package main\n\nfunc StableValue() string { return \"changed\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "commit", "-qam", "change StableValue")
	run := func(arguments []string, stdin string) string {
		var stdout, stderr bytes.Buffer
		code := runContext(context.Background(), append([]string{"--root", root}, arguments...), strings.NewReader(stdin), &stdout, &stderr)
		return fmt.Sprintf("exit %d\nstdout %s\nstderr %s", code, stdout.String(), stderr.String())
	}
	var kernel bytes.Buffer
	if code := runContext(context.Background(), []string{"--root", root, "kernel"}, strings.NewReader(""), &kernel, io.Discard); code != 0 {
		t.Fatalf("kernel exit %d", code)
	}
	invocations := []struct {
		arguments []string
		stdin     string
	}{
		{[]string{"prove", "pkg/main.go"}, ""},
		{[]string{"prove", "--base", base}, ""},
		{[]string{"kernel"}, ""},
		{[]string{"kernel", "verify"}, "summary\n" + kernel.String()},
		{[]string{"witness", "--base", base}, ""},
		{[]string{"witness", "--base", base, "--json"}, ""},
	}
	cold := make([]string, len(invocations))
	for position, invocation := range invocations {
		cold[position] = run(invocation.arguments, invocation.stdin)
	}
	if _, err := os.Stat(contextindex.SnapshotDirectory(root)); !os.IsNotExist(err) {
		t.Fatalf("IDX-SNAP-V0-005: a read verb created the snapshot directory: %v", err)
	}
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	affectedGit(t, root, "checkout", "-q", base)
	run([]string{"index"}, "")
	affectedGit(t, root, "checkout", "-q", head)
	for _, state := range []string{"stale", "current"} {
		if state == "current" {
			run([]string{"index"}, "")
		}
		for position, invocation := range invocations {
			if answer := run(invocation.arguments, invocation.stdin); answer != cold[position] {
				t.Fatalf("IDX-SNAP-V0-020: %v differs on a %s snapshot:\n%s\n%s", invocation.arguments, state, cold[position], answer)
			}
		}
	}
}

// TestImpactRangeAndWorkingTreeProfilesReadTheSnapshotWithoutChangingAByte
// pins IDX-SNAP-V0-021: both range profiles over a clean worktree and the
// working-tree profile over a dirty one answer an absent, a stale and a
// current snapshot with the same stdout, stderr and exit code. The untracked
// target is written once, since its inode and ctime are part of the answer.
func TestImpactRangeAndWorkingTreeProfilesReadTheSnapshotWithoutChangingAByte(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(root, "pkg", "main.go"), []byte("package main\n\nfunc StableValue() string { return \"changed\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "commit", "-qam", "change StableValue")
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	run := func(arguments ...string) string {
		var stdout, stderr bytes.Buffer
		code := runContext(context.Background(), append([]string{"--root", root}, arguments...), strings.NewReader(""), &stdout, &stderr)
		return fmt.Sprintf("exit %d\nstdout %s\nstderr %s", code, stdout.String(), stderr.String())
	}
	compare := func(invocations ...[]string) {
		cold := make([]string, len(invocations))
		for position, arguments := range invocations {
			if cold[position] = run(arguments...); !strings.HasPrefix(cold[position], "exit 0\n") {
				t.Fatalf("%v: %s", arguments, cold[position])
			}
		}
		if _, err := os.Stat(contextindex.SnapshotDirectory(root)); !os.IsNotExist(err) {
			t.Fatalf("IDX-SNAP-V0-005: a read verb created the snapshot directory: %v", err)
		}
		affectedGit(t, root, "checkout", "-q", base)
		run("index")
		affectedGit(t, root, "checkout", "-q", head)
		for _, state := range []string{"stale", "current"} {
			if state == "current" {
				run("index")
			}
			for position, arguments := range invocations {
				if answer := run(arguments...); answer != cold[position] {
					t.Fatalf("IDX-SNAP-V0-021: %v differs on a %s snapshot:\n%s\n%s", arguments, state, cold[position], answer)
				}
			}
		}
		if err := os.RemoveAll(contextindex.SnapshotDirectory(root)); err != nil {
			t.Fatal(err)
		}
	}
	compare([]string{"impact", "--base", base}, []string{"impact", "--base", base, "--range-profile", "expanded-256"})
	if err := os.WriteFile(filepath.Join(root, "pkg", "untracked.go"), []byte("package main\n\nfunc Untracked() string { return StableValue() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	compare([]string{"impact", "--working-tree-untracked", "pkg/untracked.go"})
}

func TestHarnessIndexBuildingEventsReadTheSnapshotWithoutChangingAByte(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		event string
		input string
	}{
		"file-change":           {"file-change", `{"paths":["internal/auth/session.go"]}`},
		"compact-session-start": {"session-start", `{"startSource":"compact"}`},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			root := ledgerRepository(t)
			tracked := filepath.Join(root, "internal", "auth", "session.go")
			if err := os.WriteFile(tracked, []byte("package auth\n\n// feature:session-revocation\nfunc EnforceSessionRevocation() bool { return false }\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			arguments := cliArguments(root, test.event)
			var cold, stderr bytes.Buffer
			if code := runContext(context.Background(), arguments, strings.NewReader(test.input), &cold, &stderr); code != 0 {
				t.Fatalf("cold exit %d: %s", code, stderr.String())
			}
			index, err := contextindex.Build(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := contextindex.WriteSnapshot(index); err != nil {
				t.Fatal(err)
			}
			loaded, hit, err := contextindex.LoadSnapshot(context.Background(), root)
			if err != nil || !hit {
				t.Fatalf("IDX-SNAP-V0-010: dirty snapshot load: hit=%v err=%v", hit, err)
			}
			if len(loaded.DirtyPaths) != 1 || loaded.DirtyPaths[0] != "internal/auth/session.go" {
				t.Fatalf("IDX-SNAP-V0-010: dirty paths = %v", loaded.DirtyPaths)
			}
			var warm bytes.Buffer
			stderr.Reset()
			if code := runContext(context.Background(), arguments, strings.NewReader(test.input), &warm, &stderr); code != 0 {
				t.Fatalf("warm exit %d: %s", code, stderr.String())
			}
			if !bytes.Equal(warm.Bytes(), cold.Bytes()) {
				t.Fatalf("IDX-SNAP-V0-010: output differs between miss and hit:\n%s\n%s", cold.Bytes(), warm.Bytes())
			}
			for _, gitArguments := range [][]string{{"add", "internal/auth/session.go"}, {"commit", "-qm", "change tree"}} {
				command := exec.Command("git", gitArguments...)
				command.Dir = root
				if output, err := command.CombinedOutput(); err != nil {
					t.Fatalf("git %v: %v\n%s", gitArguments, err, output)
				}
			}
			if _, hit, err := contextindex.LoadSnapshot(context.Background(), root); err != nil || hit {
				t.Fatalf("IDX-SNAP-V0-010: stale snapshot load: hit=%v err=%v", hit, err)
			}
		})
	}
}
