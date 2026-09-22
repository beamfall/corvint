package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// packDogfoodTasks are the twenty task shapes the pack must answer exactly as
// the gob snapshot and the build do: the dogfood questions asked of this
// fixture, each run with and without a subject.
var packDogfoodTasks = []struct{ task, subject string }{
	{"does `Split` keep empty keys", "cache/demux.go"},
	{"add a test for Split with an empty key", "cache/demux_test.go"},
	{"rename Split to Partition across the cache package", "cache/demux.go"},
	{"what does the cache package export", "cache/demux.go"},
	{"features.yaml lists no features; add one", "testing/features.yaml"},
	{"the go.mod module path is example.test/ctx", "go.mod"},
	{"why does TestSplit only pass k", "cache/demux_test.go"},
	{"key demux split", "cache/demux.go"},
	{"cache demux", "cache/demux_test.go"},
	{"return key unchanged", "cache/demux.go"},
	{"which files mention Split", "go.mod"},
	{"package cache string key", "cache/demux.go"},
	{"testing features", "testing/features.yaml"},
	{"module ctx go 1.27", "go.mod"},
	{"Split(key string) string", "cache/demux.go"},
	{"TestSplit calls Split", "cache/demux_test.go"},
	{"no such identifier anywhere", "cache/demux.go"},
	{"cache/demux.go returns its key", "cache/demux_test.go"},
	{"demux_test", "cache/demux.go"},
	{"features: []", "cache/demux.go"},
}

// TestPackSnapshotReadsWithoutChangingAByteOnTwentyTasks pins proposed
// IDX-SNAP-V0-015's identity row: on every dogfood task, with and without a
// subject, the pack hit emits the bytes the build and the gob hit emit.
func TestPackSnapshotReadsWithoutChangingAByteOnTwentyTasks(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	invocations := make([][]string, 0, 2*len(packDogfoodTasks))
	for _, item := range packDogfoodTasks {
		invocations = append(invocations,
			[]string{"--root", root, "context", "--task", item.task},
			[]string{"--root", root, "context", "--task", item.task, "--subject", item.subject})
	}
	run := func(t *testing.T, snapshotFormat string, arguments []string) string {
		t.Helper()
		stdout, stderr, code := runCandidateWithEnvironment(t, "", []string{"CORVINT_SNAPSHOT_FORMAT=" + snapshotFormat}, arguments...)
		if code != 0 {
			t.Fatalf("%v: exit %d: %s", arguments, code, stderr)
		}
		return string(stdout)
	}
	cold := make([]string, len(invocations))
	for position, arguments := range invocations {
		cold[position] = run(t, "", arguments)
	}
	receiptOut, stderr, code := runCandidateWithEnvironment(t, "", []string{"CORVINT_SNAPSHOT_FORMAT=pack"}, "--root", root, "index")
	if code != 0 {
		t.Fatalf("index exit %d: %s", code, stderr)
	}
	var receipt map[string]any
	if err := json.Unmarshal(receiptOut, &receipt); err != nil {
		t.Fatal(err)
	}
	packBytes, _ := receipt["pack_bytes"].(float64)
	if packBytes <= 0 {
		t.Fatalf("receipt names no pack: %s", receiptOut)
	}
	packPaths, err := filepath.Glob(filepath.Join(contextindex.SnapshotDirectory(root), "*.aip"))
	if err != nil || len(packPaths) != 1 {
		t.Fatalf("pack paths = %v, err = %v", packPaths, err)
	}
	if _, err := os.Stat(packPaths[0]); err != nil {
		t.Fatal(err)
	}
	for position, arguments := range invocations {
		if pack := run(t, "pack", arguments); pack != cold[position] {
			t.Fatalf("pack hit differs from the build for %v:\n%s\n%s", arguments, cold[position], pack)
		}
	}
	// A corrupt body is refused when a packet reads it: the verb recompiles
	// through the loader that refuses the whole pack and answers from the gob.
	packed, err := os.ReadFile(packPaths[0])
	if err != nil {
		t.Fatal(err)
	}
	body := bytes.Index(packed, []byte("func Split(key string) string { return key }"))
	if body < 0 {
		t.Fatal("the pack holds no demux.go body")
	}
	packed[body] ^= 0xff
	if err := os.WriteFile(packPaths[0], packed, 0o600); err != nil {
		t.Fatal(err)
	}
	for position, arguments := range invocations {
		if refused := run(t, "pack", arguments); refused != cold[position] {
			t.Fatalf("a refused pack body changed the packet for %v:\n%s\n%s", arguments, cold[position], refused)
		}
	}
	for position, arguments := range invocations {
		if gob := run(t, "", arguments); gob != cold[position] {
			t.Fatalf("gob hit differs from the build for %v:\n%s\n%s", arguments, cold[position], gob)
		}
	}
	if _, err := os.Stat(contextindex.SnapshotDirectory(root)); err != nil {
		t.Fatal(err)
	}
}

// TestPackQueryAndEventVerbsRereadARefusedBodyWithoutChangingAByte pins
// IDX-SNAP-V0-015's deferred query, event, impact and feature loads: with every tracked body in
// the pack corrupted, each verb reads a body, finds it refused, discards what
// it computed, and answers through the eager loader exactly as the build does.
func TestPackQueryAndEventVerbsRereadARefusedBodyWithoutChangingAByte(t *testing.T) {
	queryRoot, eventRoot := queryCLIRepository(t), ledgerRepository(t)
	caller := filepath.Join(eventRoot, "internal", "auth", "revoke.go")
	if err := os.WriteFile(caller, []byte("package auth\n\nfunc Revoke() bool { return EnforceSessionRevocation() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{{"add", "."}, {"commit", "-qm", "add a same-package caller"}} {
		if output, err := exec.Command("git", append([]string{"-C", eventRoot}, arguments...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	invocations := []struct {
		name      string
		arguments []string
		stdin     string
	}{
		{"query", []string{"--root", queryRoot, "query", "--task", authorityStartPrompt, "--limit", "1"}, ""},
		{"user-prompt", cliArguments(queryRoot, "user-prompt"), `{"task":"` + authorityStartPrompt + `"}`},
		{"file-change", cliArguments(eventRoot, "file-change"), `{"paths":["internal/auth/session.go"]}`},
		{"session-start", cliArguments(eventRoot, "session-start"), `{"startSource":"compact"}`},
		{"impact", []string{"--root", eventRoot, "impact", "internal/auth/session.go"}, ""},
		{"feature", []string{"--root", eventRoot, "feature", "session-revocation"}, ""},
		{"prove", []string{"--root", eventRoot, "prove", "internal/auth/session.go"}, ""},
	}
	run := func(t *testing.T, arguments []string, stdin string) string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if code := runContext(context.Background(), arguments, strings.NewReader(stdin), &stdout, &stderr); code != 0 {
			t.Fatalf("%v: exit %d: %s", arguments, code, stderr.String())
		}
		return stdout.String()
	}
	cold := make([]string, len(invocations))
	for position, invocation := range invocations {
		cold[position] = run(t, invocation.arguments, invocation.stdin)
	}
	t.Setenv("CORVINT_SNAPSHOT_FORMAT", "pack")
	for _, root := range []string{queryRoot, eventRoot} {
		run(t, []string{"--root", root, "index"}, "")
		corruptEveryTrackedPackBody(t, root)
	}
	loader := loadSnapshot
	eager := 0
	loadSnapshot = func(ctx context.Context, root string) (*contextindex.Index, bool, error) {
		eager++
		return loader(ctx, root)
	}
	defer func() { loadSnapshot = loader }()
	// A digest mismatch refuses the retained mapping for the rest of the
	// process, so each verb reads the same bytes rewritten under a new
	// modification time: a fresh mapping it has to refuse itself.
	for position, invocation := range invocations {
		rewriteEveryPack(t, queryRoot, eventRoot)
		if refused := run(t, invocation.arguments, invocation.stdin); refused != cold[position] {
			t.Fatalf("a refused pack body changed %s:\n%s\n%s", invocation.name, cold[position], refused)
		}
	}
	// query, user-prompt, impact (IDX-SNAP-V0-019) and prove's impact mode
	// (IDX-SNAP-V0-020) each read a corrupt body and reread; feature reads no
	// body, so its deferred answer stands.
	if eager != 4 {
		t.Fatalf("snapshot verbs reread through the eager loader %d times, want 4", eager)
	}
	// file-change reads the same-package caller's body; compact session-start
	// reads none, so its deferred load has nothing to refuse.
	rewriteEveryPack(t, eventRoot)
	index, hit, err := contextindex.LoadEventSnapshotDeferred(context.Background(), eventRoot, false)
	if err != nil || !hit {
		t.Fatalf("deferred event load: hit=%v err=%v", hit, err)
	}
	if _, err := harnessIndexedBlock(index, "file-change", map[string]any{"paths": []any{"internal/auth/session.go"}}, gokernel.MinOutputBytes); err != nil {
		t.Fatal(err)
	}
	if index.SnapshotRefusal() == nil {
		t.Fatal("file-change read no corrupt body, so its reread went unexercised")
	}
}

// rewriteEveryPack writes each root's pack back unchanged, so its
// modification time, and with it the retained-mapping key, changes.
func rewriteEveryPack(t *testing.T, roots ...string) {
	t.Helper()
	for _, root := range roots {
		packPaths, err := filepath.Glob(filepath.Join(contextindex.SnapshotDirectory(root), "*.aip"))
		if err != nil || len(packPaths) != 1 {
			t.Fatalf("pack paths = %v, err = %v", packPaths, err)
		}
		packed, err := os.ReadFile(packPaths[0])
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(packPaths[0], packed, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// corruptEveryTrackedPackBody flips the first byte of each tracked file's
// contents where it appears in root's pack.
func corruptEveryTrackedPackBody(t *testing.T, root string) {
	t.Helper()
	packPaths, err := filepath.Glob(filepath.Join(contextindex.SnapshotDirectory(root), "*.aip"))
	if err != nil || len(packPaths) != 1 {
		t.Fatalf("pack paths = %v, err = %v", packPaths, err)
	}
	packed, err := os.ReadFile(packPaths[0])
	if err != nil {
		t.Fatal(err)
	}
	listed, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, relative := range strings.Split(strings.TrimSuffix(string(listed), "\x00"), "\x00") {
		contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		if at := bytes.Index(packed, contents); len(contents) > 0 && at >= 0 {
			packed[at] ^= 0xff
		}
	}
	if err := os.WriteFile(packPaths[0], packed, 0o600); err != nil {
		t.Fatal(err)
	}
}
