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
// program's single-dispatcher lock.
var ErrDispatcherActive = errors.New("a dispatcher holds the store")

// HoldIdle takes, without waiting, the single-dispatcher lock of every
// program directory under c.StateDir that has one, and returns the release
// for them (TRL-V0-003). While they are held no dispatcher for this
// configuration can start. A lock another process holds refuses with
// ErrDispatcherActive naming its directory; a missing state directory holds
// nothing. HoldIdle never creates a directory or a lock file.
func HoldIdle(c *Config) (release func(), err error) {
	if !platformSupported {
		return nil, errors.New("the dispatcher lock check is supported only on darwin and linux")
	}
	dir, err := os.Open(c.StateDir)
	if errors.Is(err, fs.ErrNotExist) {
		return func() {}, nil
	}
	if err != nil {
		return nil, err
	}
	entries, err := dir.ReadDir(maxHeldPrograms + 1)
	dir.Close()
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(entries) > maxHeldPrograms {
		return nil, fmt.Errorf("more than %d entries under %s", maxHeldPrograms, c.StateDir)
	}
	var held []*os.File
	release = func() {
		for _, f := range held {
			f.Close()
		}
	}
	for _, e := range entries {
		if !e.IsDir() || !ValidName(e.Name()) {
			continue
		}
		f, err := os.OpenFile(filepath.Join(ProgramDir(c, e.Name()), "lock"), os.O_RDWR, 0)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			release()
			return nil, err
		}
		if err := lockExclusive(f); err != nil {
			f.Close()
			release()
			return nil, fmt.Errorf("%w: %s: %v", ErrDispatcherActive, ProgramDir(c, e.Name()), err)
		}
		held = append(held, f)
	}
	return release, nil
}
