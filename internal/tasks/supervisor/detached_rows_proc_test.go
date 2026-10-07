//go:build darwin || linux

package supervisor

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"syscall"
	"testing"
)

// CAL-V0-136: the Linux process table reader parses a procfs root without
// forking ps, skips non-process and vanished entries, keeps command names
// with spaces and parentheses, and refuses a malformed stat.
func TestCALV0136_ProcRowsReadsAFakeProcRoot(t *testing.T) {
	root := t.TempDir()
	stat := func(dir, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		if body != "" {
			if err := os.WriteFile(filepath.Join(root, dir, "stat"), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	stat("1", "1 (init) S 0 1 1 0 -1 4194560 0 0\n")
	stat("42", "42 (a b) (c) R 1 42 42 34816 42 0\n")
	stat("43", "43 (sh) Z 42 42 42 0 -1 0\n")
	stat("99", "") // exited between the directory read and the stat read
	stat("acpi", "not a process")
	if err := os.Symlink("42", filepath.Join(root, "self")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "uptime"), []byte("1.0 2.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := procRows(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []processRow{{pid: 1, ppid: 0, pgid: 1}, {pid: 42, ppid: 1, pgid: 42}, {pid: 43, ppid: 42, pgid: 42, zombie: true}}
	if !slices.Equal(rows, want) {
		t.Fatalf("rows %+v, want %+v", rows, want)
	}

	for _, bad := range []string{"7 (x) S one 7\n", "7 x S 1 7\n", "8 (x) S 1 7\n", "7 (x)\n", "7 (x) SS 1 7\n"} {
		r := t.TempDir()
		if err := os.MkdirAll(filepath.Join(r, "7"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(r, "7", "stat"), []byte(bad), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := procRows(r); err == nil {
			t.Fatalf("malformed stat %q was accepted", bad)
		}
	}
	if _, err := procRows(filepath.Join(root, "absent")); err == nil {
		t.Fatal("an absent proc root gave rows")
	}
}

// CAL-V0-136: on Linux the real /proc reader sees this process with its
// parent and group.
func TestCALV0136_ProcRowsSeesThisProcess(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("procfs is Linux only")
	}
	rows, err := processRows()
	if err != nil {
		t.Fatal(err)
	}
	pgid, _ := syscall.Getpgid(os.Getpid())
	for _, r := range rows {
		if r.pid == os.Getpid() {
			if r.ppid != os.Getppid() || r.pgid != pgid || r.zombie {
				t.Fatalf("row %+v", r)
			}
			return
		}
	}
	t.Fatal("this process is missing")
}
