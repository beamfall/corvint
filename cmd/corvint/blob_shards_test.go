package main

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

func TestBlobShardsReadOnlyTwentyTaskMatrix(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	_, stderr, code := runCandidateWithEnvironment(t, "", []string{"CORVINT_INDEX_SHARDS=1"}, "--root", root, "index")
	if code != 0 {
		t.Fatalf("index exit %d: %s", code, stderr)
	}
	directory := contextindex.SnapshotDirectory(root)
	artifacts, err := filepath.Glob(filepath.Join(directory, "*.gob"))
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("snapshots %v", artifacts)
	}
	snapshot, err := os.ReadFile(artifacts[0])
	if err != nil {
		t.Fatal(err)
	}
	invocations := make([][]string, 0, 40)
	for _, item := range packDogfoodTasks {
		invocations = append(invocations,
			[]string{"--root", root, "context", "--task", item.task},
			[]string{"--root", root, "context", "--task", item.task, "--subject", item.subject})
	}
	run := func(shards string, args []string) string {
		t.Helper()
		out, errout, code := runCandidateWithEnvironment(t, "", []string{"CORVINT_INDEX_SHARDS=" + shards}, args...)
		if code != 0 {
			t.Fatalf("exit %d: %s", code, errout)
		}
		return string(out)
	}
	gob := make([]string, len(invocations))
	for i, args := range invocations {
		gob[i] = run("1", args)
	}
	if err := os.Remove(artifacts[0]); err != nil {
		t.Fatal(err)
	}
	before := blobShardInventory(t, directory)
	for i, args := range invocations {
		if got := run("1", args); got != gob[i] {
			t.Fatalf("shard packet differs for %v", args)
		}
	}
	after := blobShardInventory(t, directory)
	if len(before) != len(after) {
		t.Fatal("read changed file inventory")
	}
	for path, digest := range before {
		if after[path] != digest {
			t.Fatalf("read changed %s", path)
		}
	}
	for i, args := range invocations {
		if got := run("", args); got != gob[i] {
			t.Fatalf("full packet differs for %v", args)
		}
	}
	if err := os.WriteFile(artifacts[0], snapshot, 0o644); err != nil {
		t.Fatal(err)
	}
}

func blobShardInventory(t *testing.T, directory string) map[string][32]byte {
	t.Helper()
	files := make(map[string][32]byte)
	err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[path] = sha256.Sum256(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
