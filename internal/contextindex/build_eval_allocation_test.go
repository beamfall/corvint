//go:build !race

package contextindex

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
)

// TestBuildEvalFetchesNoBlobForACleanRepository is the structural ratchet on
// the build half of the user-prompt event. A worktree copy is pinned only when
// it hashes to its entry's recorded oid, which is what makes it that blob, so
// over a clean repository every committed body Git was asked to decompress was
// decompressed to be discarded. This pins the invariant directly rather than
// through a timing: a clean repository costs zero blob bodies, whatever else
// changes around it.
func TestBuildEvalFetchesNoBlobForACleanRepository(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("clean-file worktree pinning is Unix-only")
	}
	root := evalQueryRepository(t)
	before := blobBatchEntries.Load()
	index, err := BuildEval(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if fetched := blobBatchEntries.Load() - before; fetched != 0 {
		t.Fatalf("a clean repository fetched %d committed blobs, want 0", fetched)
	}
	if len(index.Sources) == 0 {
		t.Fatal("no source was pinned, so the zero above proves nothing")
	}
	if len(index.DirtyPaths) != 0 {
		t.Fatalf("the fixture is not clean: %v", index.DirtyPaths)
	}
}

// buildEvalAllocationRepository is shaped for the build rather than the
// ranking half: substantial source bytes spread over many files with distinct
// content, because the cost this ratchet bounds is proportional to the bytes a
// build materialises. Distinct content is load-bearing -- one blob batch is
// de-duplicated by oid, so a corpus of identical files would fetch a single
// body and hide the difference entirely.
func buildEvalAllocationRepository(t testing.TB) string {
	t.Helper()
	root := t.TempDir()
	benchmarkGit(t, root, "init", "-q")
	benchmarkGit(t, root, "config", "user.email", "corvint@example.test")
	benchmarkGit(t, root, "config", "user.name", "Corvint Test")
	benchmarkWriteFile(t, root, "go.mod", "module example.test/build\n\ngo 1.27.0\n")
	for file := 0; file < 60; file++ {
		var source strings.Builder
		source.WriteString("package corpus\n\n")
		for declaration := 0; declaration < 40; declaration++ {
			fmt.Fprintf(&source, "func EnforceSessionRevocation%02dIn%02d(deviceSession, heartbeatWriter string) bool {\n", declaration, file)
			for line := 0; line < 60; line++ {
				fmt.Fprintf(&source, "\tsessionRevocationStep%02d := deviceSession + heartbeatWriter // expiry enforcement audit trail %02d\n", line, line)
			}
			source.WriteString("\treturn true\n}\n\n")
		}
		benchmarkWriteFile(t, root, fmt.Sprintf("internal/corpus/file%02d.go", file), source.String())
	}
	benchmarkGit(t, root, "add", ".")
	benchmarkGit(t, root, "commit", "-qm", "build corpus")
	return root
}

// TestBuildEvalAllocationRatchet pins the build half of the user-prompt event,
// where deferring the committed blobs removed its cost. The ranking half is
// pinned separately by TestEvalQueryAllocationRatchet.
//
// Measured 2026-08-29 on go1.27.0 darwin/arm64 over the 14.2 MB corpus above:
// 33,357,916 bytes/op and 4,619 allocs/op, against 47,592,896 and 4,824 with
// buildEagerBlobs set. The bounds below sit ~15% above the measured candidate
// -- loose enough for host variance.
//
// Re-baselined 2026-08-29 when goSymbols moved from a line scanner to the
// go/parser AST, which allocates an AST per source and took the same corpus to
// 106,283,665 bytes/op and 1,674,189 allocs/op. That is a deliberate cost, not a
// regression: the scanner it replaced emitted 10,180 false positives and 4,333
// false negatives over the first-party Go corpus, and no cheaper extractor
// carries the lexical state the corrections need -- a go/scanner token walk
// measured 3.9x the source size in allocations and would not have fit under the
// old bound either.
//
// The re-baseline does not weaken the eager-read guard. The old bound was tight
// enough to catch restoring buildEagerBlobs because that read allocates one
// buffer the size of every admitted body, and a band of ~15% around the new
// baseline no longer discriminates a delta that size. That invariant is pinned
// structurally instead, and more directly, by
// TestBuildEvalFetchesNoBlobForACleanRepository above: a clean repository must
// cost zero committed blob bodies, whatever the byte total around it.
func TestBuildEvalAllocationRatchet(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("clean-file worktree pinning is Unix-only")
	}
	root := buildEvalAllocationRepository(t)
	result := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for iteration := 0; iteration < b.N; iteration++ {
			built, err := BuildEval(context.Background(), root)
			if err != nil {
				b.Fatal(err)
			}
			runtime.KeepAlive(built)
		}
	})
	if bytes := result.AllocedBytesPerOp(); bytes > 122_300_000 {
		t.Fatalf("BuildEval allocation bytes = %d, want <= 122300000", bytes)
	}
	if allocations := result.AllocsPerOp(); allocations > 1_930_000 {
		t.Fatalf("BuildEval allocations/op = %d, want <= 1930000", allocations)
	}
}
