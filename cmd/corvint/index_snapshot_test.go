package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
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
	wantKeys := []string{"build_ms", "bytes", "command", "commit", "engine", "evicted", "evicted_snapshots", "live_heads", "live_trees", "mutates", "ok", "path", "profile", "source_bytes", "sources", "store", "store_shared", "symbols", "tree"}
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
		"store":   initial["store"],
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
		command := fixtureGitCommand(arguments...)
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
		command := fixtureGitCommand("check-ignore", "--no-index", "-q", relative)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("%s is not ignored: %v %s", relative, err, output)
		}
	}
	command := fixtureGitCommand("status", "--porcelain", "--untracked-files=all")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil || len(output) != 0 {
		t.Fatalf("index and ledger dirtied repository: err=%v status=%q", err, output)
	}
}

func TestIndexWritesTheSnapshotThatContextReadsWithoutChangingAByte(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	arguments := []string{"--root", root, "instructions", "--subject", "cache/demux.go", "--host", "codex",
		"--host-version", "0.153.2", "--cwd", "cache", "--profile", "default"}
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

// TestImpactReadsTheSnapshotWithoutChangingAByte pins IDX-SNAP-V0-019: path
// impact (the reverse-import rule and the eval profile) answers a dirty
// worktree's snapshot hit exactly as its build did, and the cold reads create
// no snapshot directory.
func TestImpactReadsTheSnapshotWithoutChangingAByte(t *testing.T) {
	t.Parallel()
	root := ledgerRepository(t)
	if err := os.WriteFile(filepath.Join(root, "internal", "auth", "session.go"), []byte("package auth\n\nfunc EnforceSessionRevocation() bool { return false }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	invocations := [][]string{
		{"--root", root, "impact", "internal/auth/session.go"},
		{"--root", root, "impact", "testing/features.yaml"},
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
}

// IDX-SNAP-V0-027 (proposed): the index receipt names the superseded
// per-worktree store and every file it removed from it or left there; a
// write with no such store carries none of the three keys.
func TestIndexReceiptNamesTheSweptLegacyStore(t *testing.T) {
	t.Parallel()
	indexReceiptNamesTheSweptLegacyStore(t, taskContextRepository(t))
}

// TestIndexReceiptNamesTheSweptLegacyStoreThroughSymlinkedRoot passes --root
// through a symlink, as a macOS temp directory under /var/folders is. The
// receipt names the resolved repository paths (V1-1112).
func TestIndexReceiptNamesTheSweptLegacyStoreThroughSymlinkedRoot(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("creating a symlink needs a privilege Windows does not grant by default")
	}
	link := filepath.Join(t.TempDir(), "linked-repository")
	if err := os.Symlink(taskContextRepository(t), link); err != nil {
		t.Fatal(err)
	}
	indexReceiptNamesTheSweptLegacyStore(t, link)
}

func indexReceiptNamesTheSweptLegacyStore(t *testing.T, root string) {
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".corvint", "index"), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(resolved, ".corvint", "index")
	snapshot := filepath.Join(legacy, "sha1-"+strings.Repeat("e", 40)+"-0123456789abcdef.gob")
	if err := os.WriteFile(snapshot, []byte("superseded"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "notes.txt"), []byte("kept"), 0o600); err != nil {
		t.Fatal(err)
	}
	var receipt map[string]any
	if err := json.Unmarshal(runIndexForTest(t, root, false), &receipt); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"legacy_store": legacy,
		"legacy_removed": []any{map[string]any{
			"kind": "snapshot", "path": snapshot, "bytes": float64(len("superseded")),
			"tree": strings.Repeat("e", 40), "engine": "0123456789abcdef", "live_head": false,
		}},
		"legacy_left": []any{map[string]any{"path": filepath.Join(legacy, "notes.txt"), "reason": "unrecognised entry"}},
	}
	for key, value := range want {
		if !reflect.DeepEqual(receipt[key], value) {
			t.Fatalf("%s = %#v, want %#v", key, receipt[key], value)
		}
	}
	if err := os.Remove(filepath.Join(legacy, "notes.txt")); err != nil {
		t.Fatal(err)
	}
	// The emptied directory goes on the next write; the one after has no
	// legacy store and keeps the frozen key set.
	var next map[string]any
	if err := json.Unmarshal(runIndexForTest(t, root, false), &next); err != nil {
		t.Fatal(err)
	}
	if removed, _ := next["legacy_removed"].([]any); len(removed) != 1 || removed[0].(map[string]any)["kind"] != "directory" {
		t.Fatalf("emptied legacy store not removed: %v", next["legacy_removed"])
	}
	writingIndexReceipt(t, runIndexForTest(t, root, false))
}

// TestIndexReceiptReportsSourceBytes_IDXSNAPV0911: the receipt's source_bytes
// is the Git tree size of exactly the admitted sources, and build_ms is a
// whole, non-negative millisecond count.
func TestIndexReceiptReportsSourceBytes_IDXSNAPV0911(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	var receiptOut, stderr bytes.Buffer
	if code := runContext(context.Background(), []string{"--root", root, "index"}, strings.NewReader(""), &receiptOut, &stderr); code != 0 {
		t.Fatalf("index exit %d: %s", code, stderr.String())
	}
	var receipt map[string]any
	if err := json.Unmarshal(receiptOut.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	index, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	sizes := map[string]int{}
	for _, line := range strings.Split(strings.TrimSpace(gitOutput(t, root, "ls-tree", "-r", "-l", "HEAD")), "\n") {
		tab := strings.IndexByte(line, '\t')
		fields := strings.Fields(line[:tab])
		size, _ := strconv.Atoi(fields[3])
		sizes[line[tab+1:]] = size
	}
	want := 0
	for path := range index.Sources {
		want += sizes[path]
	}
	if want == 0 || receipt["source_bytes"] != float64(want) || receipt["sources"] != float64(len(index.Sources)) {
		t.Fatalf("receipt = %s, want source_bytes %d over %d sources", receiptOut.String(), want, len(index.Sources))
	}
	if elapsed, ok := receipt["build_ms"].(float64); !ok || elapsed < 0 || elapsed != float64(int64(elapsed)) {
		t.Fatalf("build_ms = %v", receipt["build_ms"])
	}
}
