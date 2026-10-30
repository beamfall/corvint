package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSnapshotMissNoticeNamesTheBuild is V1-1640 (IDX-SNAP-V0-102, and
// IDX-SNAP-V0-104 for both `query` paths): a `context`, path `impact` or `query` call that finds no snapshot and pays an in-memory
// build at least snapshotMissNoticeAfter long names that build, its duration
// and the `index` argv on stderr; stdout, the exit code and the snapshot store
// are unchanged, a build under the threshold stays silent, and a hit says
// nothing. Not parallel: it moves the package threshold.
func TestSnapshotMissNoticeNamesTheBuild(t *testing.T) {
	saved := snapshotMissNoticeAfter
	t.Cleanup(func() { snapshotMissNoticeAfter = saved })
	root := queryCLIRepository(t)
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/fixture\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, root, "add", "go.mod")
	gitFixture(t, root, "commit", "-qm", "declare the module impact needs")
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	argv, _ := json.Marshal([]string{"corvint", "--root", resolved, "index", "--if-stale"})
	invoke := func(threshold time.Duration, arguments ...string) (string, string) {
		t.Helper()
		snapshotMissNoticeAfter = threshold
		var stdout, stderr bytes.Buffer
		if code := runContext(context.Background(), append([]string{"--root", root}, arguments...), strings.NewReader(""), &stdout, &stderr); code != 0 {
			t.Fatalf("%v: exit %d stderr %q", arguments, code, stderr.String())
		}
		return stdout.String(), stderr.String()
	}
	contextArgs := []string{"context", "--task", "where is ParseToken defined", "--limit", "3"}
	impactArgs := []string{"impact", "internal/parser/token.go"}
	queryArgs := []string{"query", "--task", "where is ParseToken defined", "--limit", "3"}
	authorityArgs := []string{"query", "--task", authorityStartPrompt, "--limit", "3"}
	readers := [][]string{contextArgs, impactArgs, queryArgs, authorityArgs}
	for _, arguments := range readers {
		quiet, quietErr := invoke(time.Hour, arguments...)
		if quietErr != "" {
			t.Fatalf("%s: a build under the threshold wrote stderr %q", arguments[0], quietErr)
		}
		loud, notice := invoke(0, arguments...)
		if arguments[0] != "impact" && loud != quiet {
			t.Fatalf("%s: the notice changed stdout:\n%s\n%s", arguments[0], quiet, loud)
		}
		want := "corvint " + arguments[0] + ": no index snapshot matches this tree, so this call built the full index in memory ("
		if !strings.HasPrefix(notice, want) || !strings.HasSuffix(notice, " ms); run "+string(argv)+" once per committed tree, which every worktree of this repository then reuses\n") || strings.Count(notice, "\n") != 1 {
			t.Fatalf("%s: notice %q", arguments[0], notice)
		}
	}
	if entries, err := os.ReadDir(filepath.Join(root, ".git", "corvint", "index")); err == nil && len(entries) != 0 {
		t.Fatalf("a read wrote the snapshot store: %v", entries)
	}
	var stdout, stderr bytes.Buffer
	if code := runContext(context.Background(), []string{"--root", root, "index"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("index: %d %s", code, stderr.String())
	}
	for _, arguments := range readers {
		if _, hitErr := invoke(0, arguments...); hitErr != "" {
			t.Fatalf("%s: a snapshot hit wrote stderr %q", arguments[0], hitErr)
		}
	}
	// IDX-SNAP-V0-115 (V1-3133): the snapshot is keyed by tree in the shared
	// store, so a fresh linked worktree of the same commit hits it with no
	// index run of its own.
	linked := filepath.Join(t.TempDir(), "linked")
	gitFixture(t, root, "worktree", "add", "-q", "--detach", linked, "HEAD")
	for _, arguments := range readers {
		snapshotMissNoticeAfter = 0
		var stdout, stderr bytes.Buffer
		if code := runContext(context.Background(), append([]string{"--root", linked}, arguments...), strings.NewReader(""), &stdout, &stderr); code != 0 || stderr.Len() != 0 {
			t.Fatalf("%s in a linked worktree: exit %d stderr %q; want a silent snapshot hit", arguments[0], code, stderr.String())
		}
	}
}
