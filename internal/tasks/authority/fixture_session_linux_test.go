//go:build linux

package authority

import (
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestTMV0010_AS10_FixtureLinuxMountObservation(t *testing.T) {
	for _, raw := range []string{"", "mnt_id:\n", "mnt_id: 0\n", "mnt_id: x\n", "mnt_id: 1\nmnt_id: 1\n", "mnt_id: 1 extra\n", strings.Repeat("x", 4097)} {
		if _, err := linuxMountID([]byte(raw)); err == nil {
			t.Fatalf("ambiguous mount %q accepted", raw)
		}
	}
	got, err := linuxMountID([]byte("pos:\t0\nflags:\t0100000\nmnt_id:\t27\nino:\t42\n"))
	if err != nil || got != "27" {
		t.Fatalf("mount id %q %v", got, err)
	}
}

func TestTMV0010_AS10_ExtMagicResolvedFromOwnMount(t *testing.T) {
	dir, err := os.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	var st syscall.Statfs_t
	if err := syscall.Fstatfs(int(dir.Fd()), &st); err != nil {
		t.Fatal(err)
	}
	if uint32(st.Type) != magicExt4 {
		t.Skipf("temp dir magic 0x%x is not the shared ext magic", uint32(st.Type))
	}
	fs, err := observeFilesystem(dir)
	if err != nil || !fs.Local || !extTypes[fs.Type] {
		t.Fatalf("ext mount not resolved from mountinfo: %+v %v", fs, err)
	}
}
