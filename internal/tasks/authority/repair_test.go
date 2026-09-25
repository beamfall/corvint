package authority

import (
	"context"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"path/filepath"
	"testing"
)

func TestTMV0010_AS10_AmbiguousExtMagic(t *testing.T) {
	for _, name := range []string{"ext2", "ext3", "ext4"} {
		fs := filesystemFromMagic(0xef53)
		if fs.Type == "ext4" || fs.Local || Classify(fs.Platform, fs.Type) {
			t.Fatalf("%s shared magic qualified: %+v", name, fs)
		}
	}
	for _, magic := range []uint32{magicXFS, magicBtrfs, magicTmpfs} {
		fs := filesystemFromMagic(magic)
		if !fs.Local || !Classify(fs.Platform, fs.Type) {
			t.Fatalf("known magic refused: %+v", fs)
		}
	}
	if !Classify("linux", "ext4") {
		t.Fatal("must preserve ext4 policy; only its unproved observation is refused")
	}
}
func TestTMV0010_AS10_UnsupportedPlatformBeforeEffects(t *testing.T) {
	old := supportedPlatform
	supportedPlatform = false
	t.Cleanup(func() { supportedPlatform = old })
	r := fixture.TempRepo(t)
	repo := &intent.Repository{CommonDir: r.CommonDir, StateDir: r.StateDir, LockPath: filepath.Join(r.CommonDir, LockFileName), PrimaryWorktree: r.Root}
	before := fixture.TreeSnapshot(t, r.Root)
	if l, err := AcquireLock(context.Background(), repo, LockOptions{}); l != nil || wire.CodeOf(err) != wire.CodeUnsupportedFilesystem {
		t.Fatalf("lock: %v %v", l, err)
	}
	if d, err := OpenDir(repo, ""); d != nil || wire.CodeOf(err) != wire.CodeUnsupportedFilesystem {
		t.Fatalf("dir: %v %v", d, err)
	}
	if q, err := Qualify(r.CommonDir); wire.CodeOf(err) != wire.CodeUnsupportedFilesystem || q.ProbeName != "" {
		t.Fatalf("qualify: %v %v", q, err)
	}
	// Even a stale handle must refuse before dereferencing its root or creating a temp.
	if err := (&Dir{}).LinkIn("1.json", []byte("{}\n")); wire.CodeOf(err) != wire.CodeUnsupportedFilesystem {
		t.Fatal(err)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.Root)) {
		t.Fatal("unsupported entry mutated tree")
	}
	if _, err := os.Lstat(repo.LockPath); !os.IsNotExist(err) {
		t.Fatalf("lock created: %v", err)
	}
}
