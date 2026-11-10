package contextindex

import (
	"context"
	"encoding/json"
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

func benchmarkHit(b *testing.B, open func() (*Index, error)) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		index, err := open()
		if err != nil {
			b.Fatal(err)
		}
		packet, err := TaskContext(context.Background(), index, "snapshot demux split", "", 20)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := json.Marshal(packet); err != nil {
			b.Fatal(err)
		}
		index.Release()
	}
}

// BenchmarkPackFirstReadOpenRankEmit is BenchmarkPackOpenRankEmit for a
// process's first read of the pack: retention is reset before every open, so
// each iteration maps the file and decodes it from scratch, as one CLI
// invocation does (V1-0983).
func BenchmarkPackFirstReadOpenRankEmit(b *testing.B) {
	_, receipt, identity := packBenchIndex(b)
	b.Cleanup(resetPackRetention)
	benchmarkHit(b, func() (*Index, error) {
		resetPackRetention()
		return readPackSnapshot(receipt.PackPath, identity, analyzerEngine(), loadContext)
	})
}

func BenchmarkPackOpenRankEmit(b *testing.B) {
	_, receipt, identity := packBenchIndex(b)
	benchmarkHit(b, func() (*Index, error) {
		return readPackSnapshot(receipt.PackPath, identity, analyzerEngine(), loadContext)
	})
}

func BenchmarkGobOpenRankEmit(b *testing.B) {
	_, receipt, identity := packBenchIndex(b)
	benchmarkHit(b, func() (*Index, error) {
		file, err := os.Open(receipt.Path)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		return decodeSnapshot(file, identity, receipt.Engine)
	})
}
