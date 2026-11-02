package contextindex

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
)

// These tests count live snapshot mappings, so none runs in parallel.

// TestDeferredGobLeaseLifecycle_IDXSNAPV0815: a deferred gob load holds one
// mapping until Release; a deferred load that the vocabulary check refuses
// after the lease was attached, and one discarded because HEAD moved during
// the read, hold none (IDX-SNAP-V0-815).
func TestDeferredGobLeaseLifecycle_IDXSNAPV0815(t *testing.T) {
	t.Setenv("CORVINT_"+indexStoreSuffix, t.TempDir())
	index := taskContextFixture(t)
	if _, err := WriteSnapshot(index); err != nil {
		t.Fatal(err)
	}
	base := liveSnapshotMappings.Load()
	loaded, hit, err := LoadSnapshotDeferred(context.Background(), index.Root)
	if err != nil || !hit || loaded.lease == nil {
		t.Fatalf("deferred load hit=%v err=%v lease=%v", hit, err, loaded != nil && loaded.lease != nil)
	}
	if live := liveSnapshotMappings.Load(); live != base+1 {
		t.Fatalf("a held deferred load keeps %d mappings, want %d", live, base+1)
	}
	loaded.Release()
	if live := liveSnapshotMappings.Load(); live != base {
		t.Fatalf("after Release %d mappings are live, want %d", live, base)
	}

	// A vocabulary the check refuses: the decode attaches the lease first.
	broken := taskContextFixture(t)
	broken.Vocabulary.Terms.Offsets = append([]uint32(nil), broken.Vocabulary.Terms.Offsets...)
	broken.Vocabulary.Terms.Offsets[len(broken.Vocabulary.Terms.Offsets)-1] = 1 << 30
	if _, err := WriteSnapshot(broken); err != nil {
		t.Fatal(err)
	}
	base = liveSnapshotMappings.Load()
	if _, hit, err := LoadSnapshotDeferred(context.Background(), broken.Root); err != nil || hit {
		t.Fatalf("a refused vocabulary loaded: hit=%v err=%v", hit, err)
	}
	if live := liveSnapshotMappings.Load(); live != base {
		t.Fatalf("a refused deferred load left %d mappings live, want %d", live, base)
	}

	if runtime.GOOS == "windows" {
		t.Skip("the identity drift wrapper uses a POSIX shell")
	}
	drift := taskContextFixture(t)
	if _, err := WriteSnapshot(drift); err != nil {
		t.Fatal(err)
	}
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	alternate := "0" + drift.CommitRevision[1:]
	if drift.CommitRevision[0] == '0' {
		alternate = "1" + drift.CommitRevision[1:]
	}
	counter := filepath.Join(t.TempDir(), "identity-count")
	bin := t.TempDir()
	// The first identity read names the snapshot's commit; every later one
	// names another commit on the same tree, so the closing read disagrees.
	wrapper := fmt.Sprintf(`#!/bin/sh
for argument in "$@"; do
  if [ "$argument" = "rev-parse" ]; then
    count=0
    if [ -f %[1]s ]; then count=$(sed -n '1p' %[1]s); fi
    count=$((count + 1))
    printf '%%s\n' "$count" > %[1]s
    commit=%[2]s
    if [ "$count" -gt 1 ]; then commit=%[3]s; fi
    printf '%%s\nfalse\n%%s\n%%s\n.git/info/grafts\n' %[4]s "$commit" %[5]s
    exit 0
  fi
done
exec %[6]s "$@"
`, strconv.Quote(counter), strconv.Quote(drift.CommitRevision), strconv.Quote(alternate), strconv.Quote(drift.ObjectFormat),
		strconv.Quote(drift.Revision), strconv.Quote(realGit))
	if err := execfile.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	base = liveSnapshotMappings.Load()
	if _, hit, err := LoadSnapshotDeferred(context.Background(), drift.Root); err != nil || hit {
		t.Fatalf("a load across a HEAD move hit: hit=%v err=%v", hit, err)
	}
	if live := liveSnapshotMappings.Load(); live != base {
		t.Fatalf("a discarded deferred load left %d mappings live, want %d", live, base)
	}
}
