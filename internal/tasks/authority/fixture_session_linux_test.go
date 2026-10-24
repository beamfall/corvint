//go:build linux

package authority

import (
	"os"
	"strconv"
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

// LGL-V0-002 (proposed): the session's retained fdinfo file is re-read from
// offset 0 on every observation, and procfs regenerates it each time, so a
// change to the descriptor's state after the first read is observed (the
// file position stands in for state that cannot change on an open
// directory). The observed mount equals a fresh per-call observation, and
// closing the session's handles closes the retained file.
func TestLGLV0002_RetainedFDInfoObservesCurrentState(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "observed")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(strings.Repeat("x", 64)); err != nil {
		t.Fatal(err)
	}
	s := &fixtureSession{parents: map[string]*fixtureParent{}}
	fresh, err := fixtureObserveMount(f, fixtureMount{})
	if err != nil {
		t.Fatal(err)
	}
	for i, pos := range []int64{0, 17, 42} {
		if _, err := f.Seek(pos, 0); err != nil {
			t.Fatal(err)
		}
		got, err := s.observeMount(f, fresh)
		if err != nil || got != fresh {
			t.Fatalf("observation %d: %+v %v, fresh %+v", i, got, err, fresh)
		}
		info := s.fdinfo[f]
		if info == nil || len(s.fdinfo) != 1 {
			t.Fatalf("observation %d retained %v", i, s.fdinfo)
		}
		raw, err := readFDInfoAt(info)
		if err != nil {
			t.Fatal(err)
		}
		if want := "pos:\t" + strconv.FormatInt(pos, 10) + "\n"; !strings.HasPrefix(string(raw), want) {
			t.Fatalf("observation %d read stale fdinfo %q, want prefix %q", i, raw, want)
		}
	}
	info := s.fdinfo[f]
	if err := s.closeHandles(); err != nil {
		t.Fatal(err)
	}
	if len(s.fdinfo) != 0 {
		t.Fatalf("closeHandles kept %v", s.fdinfo)
	}
	if _, err := info.ReadAt(make([]byte, 1), 0); err == nil {
		t.Fatal("the retained fdinfo file is still open")
	}
}
