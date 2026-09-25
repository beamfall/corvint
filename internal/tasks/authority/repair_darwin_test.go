//go:build darwin

package authority

import (
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestTMV0010_AS10_DarwinStrictFullSyncBoundary(t *testing.T) {
	old := fullSyncCall
	t.Cleanup(func() { fullSyncCall = old })
	r := fixture.TempRepo(t)
	repo, err := intent.Resolve(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Join(r.StateDir, "receipts"), 0755); err != nil {
		t.Fatal(err)
	}
	d, err := OpenDir(repo, "receipts")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	calls := 0
	fullSyncCall = func(fd int) error {
		calls++
		if calls == 1 {
			return syscall.EINTR
		}
		return syscall.ENOTSUP
	}
	if err = d.LinkIn("1.json", []byte("{}\n")); wire.CodeOf(err) != wire.CodeUnsupportedFilesystem || calls != 2 {
		t.Fatalf("strict boundary calls=%d err=%v", calls, err)
	}
	if entries, _ := os.ReadDir(d.Path()); len(entries) != 0 {
		t.Fatal("publication after unsupported full sync")
	}
	fullSyncCall = func(int) error { t.Fatal("directory sync used F_FULLFSYNC"); return nil }
	if err = d.Sync(); err != nil {
		t.Fatal(err)
	}
	fullSyncCall = old
	f, err := os.CreateTemp(r.Root, "sync-")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err = fullSync(f); err == nil {
		t.Fatal("closed descriptor accepted")
	}
}
