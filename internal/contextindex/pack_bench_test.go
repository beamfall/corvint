package contextindex

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// The pack benchmarks time one hit: open the snapshot, rank a three-term
// task, and marshal the packet. The pack arm reads with the context verb's
// load, which verifies bodies on first read. CORVINT_PACK_BENCH_ROOT names a repository to
// measure instead of the fixture; its snapshot is written by this binary in
// the setup so the engine id matches.
func packBenchIndex(b *testing.B) (*Index, SnapshotReceipt, repositoryIdentity) {
	b.Helper()
	b.Setenv(snapshotFormatEnv, packFormatValue)
	root := os.Getenv("CORVINT_PACK_BENCH_ROOT")
	if root == "" {
		root = testRepository(b)
		for name, content := range map[string]string{
			"go.mod":              "module example.test/fixture\n\ngo 1.27.0\n",
			"cache/cache.go":      "package cache\n\nfunc Demux() string { return \"demux\" }\n",
			"cache/demux.go":      "package cache\n\nfunc Split(key string) string { return key }\n",
			"cache/demux_test.go": "package cache\n\nfunc TestSplit() { _ = Split(\"k\") }\n",
		} {
			path := filepath.Join(root, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				b.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				b.Fatal(err)
			}
		}
		testGit(b, root, "add", "-A")
		testGit(b, root, "commit", "-qm", "bench fixture")
	}
	index, err := Build(context.Background(), root)
	if err != nil {
		b.Fatal(err)
	}
	receipt, err := WriteSnapshot(index)
	if err != nil {
		b.Fatal(err)
	}
	return index, receipt, repositoryIdentity{objectFormat: index.ObjectFormat, commitRevision: index.CommitRevision, treeRevision: index.Revision}
}
