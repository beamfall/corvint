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

// shardedStore returns a store whose requests/ index spans at least
// minShards shard directories, and the shard paths it created.
func shardedStore(t *testing.T, minShards int) (*intent.Repository, []string) {
	t.Helper()
	r := fixture.TempRepo(t)
	fixture.WriteState(t, r)
	fixture.WriteIntent(t, r, fixture.Ticket("A"))
	fixture.CommitPosts(t, r, "MUTATION", "", map[string][]byte{"intent/tickets/A.json": fixture.Ticket("A").Encode()})
	shards := map[string]bool{}
	for i := 0; len(shards) < minShards; i++ {
		id := fmt.Sprintf("held-%d", i)
		reqPath, _ := snapshot.RequestPath(id)
		seq := wire.SizeOf(uint64(3 + i))
		out := mutation.Outcome{RequestID: id, Outcome: mutation.OutcomeCompleted, ReceiptSeq: &seq}
		req := wire.NewObject()
		req.Set("requestId", wire.String(id))
		req.Set("seq", wire.String(string(seq)))
		req.Set("mutationSha256", wire.String(string(wire.Sum([]byte(id)))))
		req.Set("outcome", out.Value())
		fixture.CommitPosts(t, r, "MUTATION", id, map[string][]byte{reqPath: wire.EncodeFile(wire.ObjectValue(req))})
		shards[filepath.Dir(reqPath)] = true
	}
	repo, err := intent.Resolve(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	var list []string
	for s := range shards {
		list = append(list, s)
	}
	return repo, list
}

// TestCTSV0020_ExportHoldsBoundedDirectoryRoots (V1-0903): an export keeps
// at most its two anchors plus maxHeldRoots directory roots open however
// many request shard directories the store has, so a store with all 256
// shards no longer exhausts a 256-descriptor limit, and the bounded export
// is byte-identical to one that keeps every root.
func TestCTSV0020_ExportHoldsBoundedDirectoryRoots(t *testing.T) {
	repo, shards := shardedStore(t, 24)
	oldMax, oldHook := maxHeldRoots, afterHold
	defer func() { maxHeldRoots, afterHold = oldMax, oldHook }()
	peak := 0
	afterHold = func(open int) { peak = max(peak, open) }
	run := func(bound int) []byte {
		t.Helper()
		maxHeldRoots, peak = bound, 0
		stream, _, err := export(t, repo, fixture.TempDirOutside(t))
		if err != nil {
			t.Fatalf("export holding %d roots: %v", bound, err)
		}
		if _, err := Verify(bytes.NewReader(stream)); err != nil {
			t.Fatalf("verify: %v", err)
		}
		return stream
	}
	all := run(1 << 20)
	if peak <= len(shards) {
		t.Fatalf("unbounded export held %d roots for %d shards; the fixture does not exercise the bound", peak, len(shards))
	}
	bounded := run(4)
	if peak > 2+4 {
		t.Fatalf("bounded export held %d roots, want at most %d", peak, 2+4)
	}
	if !bytes.Equal(all, bounded) {
		t.Fatal("bounded export differs from the unbounded one")
	}
}

// TestCTSV0020_ReleasedRootReopenedOnlyAsTheListedDirectory: a shard
// directory replaced after the layout capture is never read through its
// replacement; every attempt is moved and no archive is delivered.
func TestCTSV0020_ReleasedRootReopenedOnlyAsTheListedDirectory(t *testing.T) {
	repo, shards := shardedStore(t, 8)
	oldMax := maxHeldRoots
	defer func() { maxHeldRoots = oldMax }()
	maxHeldRoots = 1
	victim := filepath.Join(repo.StateDir, filepath.FromSlash(shards[0]))
	swaps := 0
	var out bytes.Buffer
	_, err := Export(ExportOptions{Repo: repo, Staging: fixture.TempDirOutside(t), Stdout: &out, afterCapture: func() {
		swaps++
		next := victim + ".copy"
		if err := os.CopyFS(next, os.DirFS(victim)); err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(victim); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(next, victim); err != nil {
			t.Fatal(err)
		}
	}})
	if code(err) != wire.CodeSnapshotMoved || out.Len() != 0 {
		t.Fatalf("export across a replaced shard: %v (%d bytes, %d swaps)", err, out.Len(), swaps)
	}
}
