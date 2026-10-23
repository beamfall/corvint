//go:build darwin || linux

package archive

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// shardedStore is repoWithStore plus enough journaled requests that the
// `requests/` index holds at least minShards two-character shard directories.
func shardedStore(t *testing.T, minShards int) (*fixture.Repo, *intent.Repository) {
	t.Helper()
	r, _ := repoWithStore(t)
	shards := map[string]bool{}
	for i := 0; len(shards) < minShards; i++ {
		id := fmt.Sprintf("shard-%d", i)
		path, err := snapshot.RequestPath(id)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(filepath.Join(r.StateDir, "head.json"))
		if err != nil {
			t.Fatal(err)
		}
		head, err := snapshot.DecodeHead(raw)
		if err != nil {
			t.Fatal(err)
		}
		seq := wire.SizeOf(head.LastSeq.Uint64() + 1)
		out := mutation.Outcome{RequestID: id, Outcome: mutation.OutcomeCompleted, ReceiptSeq: &seq}
		req := wire.NewObject()
		req.Set("requestId", wire.String(id))
		req.Set("seq", wire.String(string(seq)))
		req.Set("mutationSha256", wire.String(string(wire.Sum([]byte("fixture mutation\n")))))
		req.Set("outcome", out.Value())
		fixture.CommitPosts(t, r, "MUTATION", id, map[string][]byte{path: wire.EncodeFile(wire.ObjectValue(req))})
		shards[filepath.Dir(path)] = true
	}
	repo, err := intent.Resolve(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	return r, repo
}

// openDescriptors counts this process's open descriptors, including the one
// the listing itself uses.
func openDescriptors(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/dev/fd")
	if err != nil {
		t.Skipf("descriptors are not enumerable: %v", err)
	}
	return len(entries)
}

// TestCTSV0020_ExportHoldsBoundedDescriptors proves an export of a store whose
// request index has 200 shard directories never holds a descriptor per shard:
// the peak stays within the root bound plus a small constant, where holding
// every listed parent peaked at the shard count and failed RESOURCE_EXHAUSTED
// under a 256 soft limit (V1-0903).
func TestCTSV0020_ExportHoldsBoundedDescriptors(t *testing.T) {
	const shards = 200
	r, repo := shardedStore(t, shards)
	seen, err := os.ReadDir(filepath.Join(r.StateDir, "requests"))
	if err != nil || len(seen) < shards {
		t.Fatalf("requests index has %d shards, want at least %d: %v", len(seen), shards, err)
	}
	base, peak := openDescriptors(t), 0
	old := afterReadNames
	t.Cleanup(func() { afterReadNames = old })
	afterReadNames = func(string) {
		if n := openDescriptors(t); n > peak {
			peak = n
		}
	}
	stream, res, err := export(t, repo, fixture.TempDirOutside(t))
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if peak == 0 {
		t.Fatal("no directory was listed")
	}
	if held := peak - base; held > 40 {
		t.Fatalf("export held %d descriptors over %d shards, want at most 40", held, len(seen))
	}
	if _, err := Verify(bytes.NewReader(stream)); err != nil {
		t.Fatalf("bounded export does not verify: %v", err)
	}
	if res.Manifest == nil {
		t.Fatal("no manifest")
	}
}

// TestCTSV0020_ReopenedShardMustBeTheListedDirectory proves a parent closed
// for the descriptor bound is reopened only as the directory the listing
// observed: replacing it between the listing and the read is a moved store,
// where an unbounded read would have kept reading the original directory.
func TestCTSV0020_ReopenedShardMustBeTheListedDirectory(t *testing.T) {
	r, _ := shardedStore(t, 60)
	life := newArchiveRead(archiveNative{StateDir: r.StateDir, PrimaryWorktree: filepath.Dir(r.StateDir)})
	t.Cleanup(func() {
		if err := life.close(); err != nil {
			t.Error(err)
		}
	})
	files, _, err := stateLayoutRead(r.StateDir, wire.MaxArchiveScanEntries, life)
	if err != nil {
		t.Fatal(err)
	}
	var victim sourceFile
	for _, f := range files {
		dir := filepath.ToSlash(filepath.Dir(f.path))
		if filepath.Dir(dir) == "requests" && life.roots[dir] == nil {
			victim = f
			break
		}
	}
	if victim.path == "" {
		t.Fatalf("no shard root was closed: %d of %d roots open", len(life.roots), len(files))
	}
	if _, err := life.Read(victim.path, victim.max); err != nil {
		t.Fatalf("reopened shard: %v", err)
	}
	shard := filepath.Join(r.StateDir, filepath.FromSlash(filepath.Dir(victim.path)))
	key := filepath.ToSlash(filepath.Dir(victim.path))
	if err := closeReadRoot(life.roots[key]); err != nil {
		t.Fatal(err)
	}
	delete(life.roots, key)
	if err := os.Rename(shard, shard+"-old"); err != nil {
		t.Fatal(err)
	}
	fixture.Write(t, filepath.Join(shard, filepath.Base(victim.path)), []byte("{}\n"))
	if _, err := life.Read(victim.path, victim.max); wire.CodeOf(err) != wire.CodeSnapshotMoved {
		t.Fatalf("read of a replaced shard: %v, want SNAPSHOT_MOVED", err)
	}
}
