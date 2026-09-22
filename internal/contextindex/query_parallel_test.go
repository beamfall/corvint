package contextindex

import (
	"context"
	"fmt"
	"runtime"
	"testing"
)

// TestBuildQueryAgreesAcrossWorkerCounts exercises the concurrent clean-file
// verification. The ordinary fixtures sit below entriesPerWorker and so take the
// single-buffered path, which would leave the parallel fan-out covered only by a
// benchmark against one developer's checkout. This builds a tree large enough to
// fan out and asserts the index is identical to the one a single worker
// produces, so a concurrency fault shows up as disagreement rather than as a
// silent difference in a receipt.
func TestBuildQueryAgreesAcrossWorkerCounts(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("clean-file streaming verification is Unix-only")
	}
	root := t.TempDir()
	benchmarkGit(t, root, "init", "-q")
	benchmarkGit(t, root, "config", "user.email", "corvint@example.test")
	benchmarkGit(t, root, "config", "user.name", "Corvint Test")
	benchmarkWriteFile(t, root, "AGENTS.md", "# Project instructions\n\nThe roadmap is the only active work queue.\n")
	// Spread across directories so consecutive entries exercise both a cached
	// parent chain and the rebuild when the chain changes.
	for index := 0; index < entriesPerWorker*3; index++ {
		relative := fmt.Sprintf("internal/pkg%02d/file%04d.go", index%16, index)
		benchmarkWriteFile(t, root, relative, fmt.Sprintf("package pkg%02d\n\nvar Payload = %d\n", index%16, index))
	}
	benchmarkGit(t, root, "add", ".")
	benchmarkGit(t, root, "commit", "-qm", "parallel verification corpus")

	task := "Identify the active work queue and the required workflow gates"
	parallel, err := BuildQuery(context.Background(), root, task)
	if err != nil {
		t.Fatal(err)
	}
	if len(parallel.Sources) < entriesPerWorker*2 {
		t.Fatalf("fixture too small to fan out: %d sources", len(parallel.Sources))
	}
	sequential, err := buildQuerySingleWorker(root, task)
	if err != nil {
		t.Fatal(err)
	}
	if len(parallel.Sources) != len(sequential.Sources) {
		t.Fatalf("source count: parallel %d, sequential %d", len(parallel.Sources), len(sequential.Sources))
	}
	for path, source := range sequential.Sources {
		other, present := parallel.Sources[path]
		if !present {
			t.Fatalf("parallel build is missing %s", path)
		}
		if other.Path != source.Path || other.BlobHash != source.BlobHash || other.Mode != source.Mode {
			t.Fatalf("source %s: parallel %+v, sequential %+v", path, other, source)
		}
	}
	if len(parallel.Exclusions) != len(sequential.Exclusions) {
		t.Fatalf("exclusion count: parallel %d, sequential %d", len(parallel.Exclusions), len(sequential.Exclusions))
	}
	for index, exclusion := range sequential.Exclusions {
		if parallel.Exclusions[index] != exclusion {
			t.Fatalf("exclusion %d: parallel %+v, sequential %+v", index, parallel.Exclusions[index], exclusion)
		}
	}
}

// buildQuerySingleWorker forces the single-buffered path by raising the
// per-worker threshold above the fixture for the duration of one build.
func buildQuerySingleWorker(root, task string) (*Index, error) {
	restore := entriesPerWorkerOverride
	entriesPerWorkerOverride = 1 << 30
	defer func() { entriesPerWorkerOverride = restore }()
	return BuildQuery(context.Background(), root, task)
}
