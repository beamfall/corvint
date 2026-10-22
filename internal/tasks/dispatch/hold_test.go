//go:build darwin || linux

package dispatch

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func realTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func heldLock(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
}

// TRL-V0-003: a program directory reached through a symbolic link, whose
// lock a running dispatcher holds, refuses instead of being skipped.
func TestTRLV0003_HoldIdleRefusesSymlinkedProgramDir(t *testing.T) {
	root := realTempDir(t)
	state := filepath.Join(root, "state")
	target := filepath.Join(root, "elsewhere")
	heldLock(t, target)
	if err := os.MkdirAll(state, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(state, "prog")); err != nil {
		t.Fatal(err)
	}
	if release, err := HoldIdle(&Config{StateDir: state}); err == nil {
		release()
		t.Fatal("a symlinked program directory with a held lock was skipped")
	}
}

// TRL-V0-005: a stateDir reached through a symbolic link component refuses.
func TestTRLV0005_HoldIdleRefusesSymlinkedStateDir(t *testing.T) {
	root := realTempDir(t)
	real := filepath.Join(root, "real")
	if err := os.MkdirAll(real, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{link, filepath.Join(link, "absent")} {
		if release, err := HoldIdle(&Config{StateDir: state}); err == nil {
			release()
			t.Fatalf("symlinked stateDir %s was accepted", state)
		}
	}
	if _, err := os.Lstat(filepath.Join(real, "absent")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a refused stateDir was created")
	}
}

// TRL-V0-003: while a relocation holds an absent stateDir, a dispatcher
// starting under it is refused before it creates its program directory;
// once released, the start proceeds.
func TestTRLV0003_DispatcherStartRefusedDuringRelocation(t *testing.T) {
	c := &Config{StateDir: filepath.Join(realTempDir(t), "state")}
	release, err := HoldIdle(c)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Open("prog", c, nil, io.Discard)
	if !errors.Is(err, ErrRelocationHolds) {
		release()
		t.Fatalf("start during relocation: %v", err)
	}
	if _, err := os.Lstat(ProgramDir(c, "prog")); !errors.Is(err, os.ErrNotExist) {
		release()
		t.Fatal("a refused start created its program directory")
	}
	release()
	gate, err := openStartGate(c)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := lockProgram(ProgramDir(c, "prog"))
	gate.Close()
	if err != nil {
		t.Fatal(err)
	}
	lock.Close()
}

// TRL-V0-003: a dispatcher between its start gate and its program lock keeps
// a relocation out, so a start the scan cannot see yet is never missed.
func TestTRLV0003_HoldIdleRefusesStartingDispatcher(t *testing.T) {
	c := &Config{StateDir: filepath.Join(realTempDir(t), "state")}
	gate, err := openStartGate(c)
	if err != nil {
		t.Fatal(err)
	}
	defer gate.Close()
	release, err := HoldIdle(c)
	if !errors.Is(err, ErrDispatcherActive) {
		if err == nil {
			release()
		}
		t.Fatalf("relocation during a dispatcher start: %v", err)
	}
}
