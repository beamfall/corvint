//go:build linux

package authority

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestCALV0074_ProcLocksParse(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "slot")
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	sys := st.Sys().(*syscall.Stat_t)
	dev := uint64(sys.Dev)
	major := (dev>>8)&0xfff | (dev>>32)&^uint64(0xfff)
	minor := dev&0xff | (dev>>12)&^uint64(0xff)
	id := func(maj, min uint64) string {
		return fmt.Sprintf("%02x:%02x:%d", maj, min, sys.Ino)
	}
	for _, c := range []struct {
		name, table string
		held, err   bool
	}{
		{"empty", "", false, false},
		{"granted-flock", "1: FLOCK  ADVISORY  WRITE 42 " + id(major, minor) + " 0 EOF\n", true, false},
		{"blocked-waiter-only", "1: -> FLOCK  ADVISORY  WRITE 42 " + id(major, minor) + " 0 EOF\n", false, false},
		{"lease-ignored", "1: LEASE  ACTIVE    READ  42 " + id(major, minor) + " 0 EOF\n", false, false},
		{"other-device-same-inode", "1: FLOCK  ADVISORY  WRITE 42 " + id(major+1, minor) + " 0 EOF\n", false, true},
	} {
		v, err := parseProcLocks(c.table)
		if err != nil {
			t.Fatal(c.name, err)
		}
		held, err := v.held(nil, st)
		if held != c.held || (err != nil) != c.err {
			t.Errorf("%s: held=%t err=%v", c.name, held, err)
		}
	}
	if _, err := parseProcLocks("1: FLOCK  ADVISORY  WRITE 42 zz:01:5 0 EOF\n"); err == nil {
		t.Error("malformed identity accepted")
	}
}
