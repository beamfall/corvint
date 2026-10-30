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
func TestTMV0010_AS10_ExtFromMountinfo(t *testing.T) {
	const table = "22 1 8:1 / / rw,relatime shared:1 - ext4 /dev/sda1 rw\n" +
		"23 22 8:2 /a /mnt/with\\040space rw - ext3 /dev/sda2 rw\n" +
		"24 22 8:3 / /old rw master:2 shared:3 - ext2 /dev/sda3 rw\n" +
		"25 22 0:4 / /fuse rw - fuseblk /dev/sdb1 rw\n" +
		"26 22 8:5 / /bad rw ext4 /dev/sda5 rw\n" +
		"27 22 8:6 / /dup rw - ext4 /dev/sda6 rw\n27 22 8:6 / /dup rw - ext4 /dev/sda6 rw\n"
	for id, want := range map[string]string{"22": "ext4", "23": "ext3", "24": "ext2"} {
		fs := extFromMountinfo([]byte(table), id)
		if fs.Type != want || !fs.Local || Classify(fs.Platform, fs.Type) != (want == "ext4") {
			t.Fatalf("mount %s: %+v, want local %s", id, fs, want)
		}
	}
	for _, id := range []string{"25", "26", "27", "99", "2"} {
		if fs := extFromMountinfo([]byte(table), id); fs != filesystemFromMagic(magicExt4) {
			t.Fatalf("mount %s qualified: %+v", id, fs)
		}
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
