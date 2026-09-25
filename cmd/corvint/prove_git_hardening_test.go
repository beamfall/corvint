package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// gitHardeningRepository commits a.go twice and returns the root and the
// first commit, so base..HEAD changes one Go file.
func gitHardeningRepository(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"},
		{"config", "maintenance.auto", "false"}, {"config", "gc.auto", "0"},
	} {
		affectedGit(t, root, arguments...)
	}
	path := filepath.Join(root, "a.go")
	if err := os.WriteFile(path, []byte("package a\n\nfunc A() int { return 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "base")
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	if err := os.WriteFile(path, []byte("package a\n\nfunc A() int { return 2 }\n\nfunc B() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "commit", "-qam", "change")
	return root, base
}

func gitHardeningExecutable(t *testing.T) string {
	t.Helper()
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git unavailable")
	}
	return gitExecutable
}

// FPK-V0-052: a repository-configured core.fsmonitor program never runs
// during any cmd/corvint Git read.
func TestProveGitReadsNeverRunRepositoryFSMonitor(t *testing.T) {
	t.Parallel()
	root, base := gitHardeningRepository(t)
	gitExecutable := gitHardeningExecutable(t)
	outside := t.TempDir()
	sentinel := filepath.Join(outside, "fsmonitor-ran")
	script := filepath.Join(outside, "fsmonitor.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\ntouch '"+sentinel+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "config", "core.fsmonitor", script)
	// Control: an unhardened status runs the program, so its absence below is a real result.
	affectedGit(t, root, "status", "--porcelain")
	if err := os.Remove(sentinel); err != nil {
		t.Fatalf("control: core.fsmonitor did not run under plain git status: %v", err)
	}
	ctx := context.Background()
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD^{tree}"))
	reads := []struct {
		name string
		read func() error
	}{
		{"rangeChangedPaths", func() error { _, err := rangeChangedPaths(ctx, gitExecutable, root, base); return err }},
		{"rangeHunkSpans", func() error { _, err := rangeHunkSpans(ctx, gitExecutable, root, base); return err }},
		{"proveTreeRevision", func() error { _, err := proveTreeRevision(ctx, gitExecutable, root); return err }},
		{"readCitedBlobs", func() error { _, err := readCitedBlobs(ctx, gitExecutable, root, head, []string{"a.go"}); return err }},
		{"readCheckpointTree", func() error { _, err := readCheckpointTree(ctx, gitExecutable, root, tree); return err }},
		{"checkpointRevision", func() error { _, _, err := checkpointRevision(ctx, gitExecutable, root); return err }},
		{"affectedRevision", func() error { _, err := affectedRevision(ctx, gitExecutable, root, base); return err }},
		{"bundleObjectsPresent", func() error {
			return bundleObjectsPresent(ctx, gitExecutable, root, bundleRepository{Commit: head, Tree: tree})
		}},
		{"compactionPinMissing", func() error {
			compactionPinMissing(ctx, root, compactionPin{Revision: head, Paths: []string{"a.go"}})
			return nil
		}},
	}
	for _, read := range reads {
		if err := read.read(); err != nil {
			t.Fatalf("%s: %v", read.name, err)
		}
		if _, err := os.Stat(sentinel); err == nil {
			t.Fatalf("%s executed the repository core.fsmonitor program", read.name)
		}
	}
}

// FPK-V0-052: a refs/replace entry for the changed blob changes neither the
// hunk spans nor the cited bytes prove computes.
func TestProveGitReadsIgnoreReplaceObjects(t *testing.T) {
	t.Parallel()
	root, base := gitHardeningRepository(t)
	gitExecutable := gitHardeningExecutable(t)
	ctx := context.Background()
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	original, err := os.ReadFile(filepath.Join(root, "a.go"))
	if err != nil {
		t.Fatal(err)
	}
	wantSpans, err := rangeHunkSpans(ctx, gitExecutable, root, base)
	if err != nil {
		t.Fatal(err)
	}
	decoy := filepath.Join(t.TempDir(), "decoy.go")
	if err := os.WriteFile(decoy, []byte("package a\n\nfunc A() int { return 1 }\n\n\n\n\nfunc Decoy() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	blob := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD:a.go"))
	replacement := strings.TrimSpace(affectedGit(t, root, "hash-object", "-w", decoy))
	affectedGit(t, root, "replace", blob, replacement)
	spans, err := rangeHunkSpans(ctx, gitExecutable, root, base)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(spans, wantSpans) {
		t.Fatalf("replace ref moved the hunk spans: got %v want %v", spans, wantSpans)
	}
	cited, err := readCitedBlobs(ctx, gitExecutable, root, head, []string{"a.go"})
	if err != nil {
		t.Fatal(err)
	}
	if got := cited["a.go"]; got.oid != blob || !bytes.Equal(got.content, original) {
		t.Fatalf("replace ref changed the cited blob: oid %s content %q", got.oid, got.content)
	}
}

// AFP-V0-010: a replacement HEAD commit whose tree equals the base cannot
// empty the committed range affected selects from.
func TestAffectedBaseRangeIgnoresReplaceObjects(t *testing.T) {
	t.Parallel()
	root := affectedFixtureRepository(t)
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	corePath := filepath.Join(root, "core", "core.go")
	body, err := os.ReadFile(corePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(corePath, append(body, []byte("\n// committed edit\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "commit", "-qam", "edit core")
	affectedGit(t, root, "replace", "HEAD", base)
	var stdout, stderr bytes.Buffer
	if code := runContext(context.Background(), []string{"--root", root, "affected", "--base", base}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	var receipt map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if paths := anyStrings(receipt["range"].(map[string]any)["paths"]); !slices.Equal(paths, []string{"core/core.go"}) {
		t.Fatalf("replace ref changed the committed range: %v", paths)
	}
}

// FPK-V0-052: the discovery ceiling never hides the enclosing worktree, so the
// compaction pin check still verifies when CLAUDE_PROJECT_DIR is a subdirectory.
func TestCompactionPinVerifiesFromSubdirectoryRoot(t *testing.T) {
	t.Parallel()
	root, _ := gitHardeningRepository(t)
	subdirectory := filepath.Join(root, "sub")
	if err := os.Mkdir(subdirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	revisionMissing, missing, reason := compactionPinMissing(context.Background(), subdirectory,
		compactionPin{Revision: head, Paths: []string{"a.go"}})
	if reason != "" || revisionMissing || len(missing) != 0 {
		t.Fatalf("subdirectory root: reason %q revisionMissing %v missing %v", reason, revisionMissing, missing)
	}
}
