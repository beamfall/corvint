package dispatch

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// maxHeldPrograms bounds the program directories HoldIdle scans.
const maxHeldPrograms = 1024

// ErrDispatcherActive is HoldIdle's refusal: another process holds a
// program's single-dispatcher lock, or a dispatcher is starting.
var ErrDispatcherActive = errors.New("a dispatcher holds the store")

// ErrRelocationHolds is Open's refusal while a store relocation holds the
// state directory's start gate (TRL-V0-003).
var ErrRelocationHolds = errors.New("a store relocation holds the dispatch state directory")

// HoldIdle proves that no dispatcher for c runs and none can start until the
// returned release is called (TRL-V0-003). It requires c.StateDir to be a
// real path with no symbolic link component (TRL-V0-005), creating it when
// absent. It takes, without waiting, the state directory's exclusive start
// gate, which Open holds shared while it creates a program directory and
// takes its lock, and then every program directory's single-dispatcher lock.
// A symbolic link where a program directory or its lock belongs refuses;
// a held gate or lock refuses with ErrDispatcherActive.
func HoldIdle(c *Config) (release func(), err error) {
	if !platformSupported {
		return nil, errors.New("the dispatcher lock check is supported only on darwin and linux")
	}
	if err := ensureRealDir(c.StateDir); err != nil {
		return nil, err
	}
	dir, err := os.Open(c.StateDir)
	if err != nil {
		return nil, err
	}
	held := []*os.File{dir}
	release = func() {
		for i := len(held) - 1; i >= 0; i-- {
			held[i].Close()
		}
	}
	fail := func(err error) (func(), error) {
		release()
		return nil, err
	}
	if err := lockExclusive(dir); err != nil {
		return fail(fmt.Errorf("%w: a dispatcher is starting under %s: %v", ErrDispatcherActive, c.StateDir, err))
	}
	entries, err := dir.ReadDir(maxHeldPrograms + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return fail(err)
	}
	if len(entries) > maxHeldPrograms {
		return fail(fmt.Errorf("more than %d entries under %s", maxHeldPrograms, c.StateDir))
	}
	for _, e := range entries {
		if !ValidName(e.Name()) {
			continue
		}
		prog := ProgramDir(c, e.Name())
		if e.Type()&fs.ModeSymlink != 0 {
			return fail(fmt.Errorf("program directory %s is a symbolic link", prog))
		}
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(prog, "lock")
		info, err := os.Lstat(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fail(err)
		}
		if !info.Mode().IsRegular() {
			return fail(fmt.Errorf("dispatcher lock %s is not a regular file", path))
		}
		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			return fail(err)
		}
		held = append(held, f)
		if err := lockExclusive(f); err != nil {
			return fail(fmt.Errorf("%w: %s: %v", ErrDispatcherActive, prog, err))
		}
	}
	return release, nil
}

// openStartGate creates c.StateDir when absent and takes its start gate
// shared without waiting, so a relocation holding it exclusive (HoldIdle)
// refuses this start instead of racing it (TRL-V0-003).
func openStartGate(c *Config) (*os.File, error) {
	if err := os.MkdirAll(c.StateDir, 0o700); err != nil {
		return nil, err
	}
	gate, err := os.Open(c.StateDir)
	if err != nil {
		return nil, err
	}
	if err := lockShared(gate); err != nil {
		gate.Close()
		return nil, fmt.Errorf("%w: %s: %v", ErrRelocationHolds, c.StateDir, err)
	}
	return gate, nil
}

// ensureRealDir requires path to resolve to itself, so no component is a
// symbolic link, and creates it with mode 0700 when it is absent. The
// nearest existing ancestor must resolve to itself before anything is made.
func ensureRealDir(path string) error {
	existing := path
	for {
		if _, err := os.Lstat(existing); err == nil {
			break
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			break
		}
		existing = parent
	}
	resolved, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return err
	}
	if resolved != existing {
		return fmt.Errorf("dispatch stateDir %s is not a real path: %s resolves to %s", path, existing, resolved)
	}
	if existing != path {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return err
		}
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("dispatch stateDir %s is not a directory", path)
	}
	return nil
}
