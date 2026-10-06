package authority

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ChangeGuard binds an optimistic store observation to a later writer lock.
// It is transient and never writes files. Check re-reads files a kernel watch
// does not cover (macOS files beyond the descriptor budget); a check under the
// writer lock calls Sweep before acquiring it and CheckEvents while holding it,
// whose cost does not depend on the number of watched files (CAL-V0-026).
// Close must run after releasing the writer lock.
type ChangeGuard struct {
	watch      changeWatch
	membership wire.Digest
	closed     bool
}

// poll reports kernel events and any difference an earlier sweep found, in
// time independent of the number of watched paths. sweep re-reads what no
// kernel event covers; a difference it finds is kept for every later poll.
type changeWatch interface {
	add(string, bool) (os.FileInfo, error)
	poll() (bool, error)
	sweep() bool
	close() error
}

// WatchChanges registers ancestors before descendants, and directories before
// enumeration, so replacement and membership changes during setup invalidate
// the observation too. Content reads must start only after this returns.
func WatchChanges(repo *intent.Repository) (_ *ChangeGuard, err error) {
	w, err := newChangeWatch()
	if err != nil {
		return nil, err
	}
	g := &ChangeGuard{watch: w}
	defer func() {
		if err != nil {
			if cleanup := g.Close(); cleanup != nil {
				err = errors.Join(err, cleanup)
			}
		}
	}()
	ancestors := map[string]bool{}
	var ancestor func(string) error
	ancestor = func(path string) error {
		if ancestors[path] {
			return nil
		}
		parent := filepath.Dir(path)
		if parent != path {
			if err := ancestor(parent); err != nil {
				return err
			}
		}
		if _, err := w.add(path, false); err != nil {
			return err
		}
		ancestors[path] = true
		return nil
	}
	h := sha256.New()
	count := 0
	var visit func(string) error
	visit = func(path string) error {
		count++
		if count > wire.MaxArchiveScanEntries+wire.MaxTicketsPerQueue+wire.MaxReleasesPerQueue+256 {
			return wire.Errorf(wire.CodeLimitExceeded, path, "change observation entry bound")
		}
		info, err := w.add(path, true)
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s\x00%d\n", path, info.Mode())
		if !info.IsDir() {
			return nil
		}
		dir, err := os.Open(path)
		if err != nil {
			return err
		}
		remaining := wire.MaxArchiveScanEntries + wire.MaxTicketsPerQueue + wire.MaxReleasesPerQueue + 256 - count
		entries, err := dir.ReadDir(remaining + 1)
		if errors.Is(err, io.EOF) {
			err = nil
		}
		if closeErr := dir.Close(); closeErr != nil {
			return errors.Join(err, closeErr)
		}
		if err != nil {
			return err
		}
		if len(entries) > remaining {
			return wire.Errorf(wire.CodeLimitExceeded, path, "change observation entry bound")
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			if err := visit(filepath.Join(path, entry.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	for _, root := range []string{repo.StateDir, filepath.Join(repo.IntentRoot(), intent.Dir)} {
		if err := ancestor(filepath.Dir(root)); err != nil {
			return nil, err
		}
		if err := visit(root); err != nil {
			return nil, err
		}
	}
	g.membership = wire.Digest(fmt.Sprintf("%x", h.Sum(nil)))
	return g, g.Check()
}

// Membership identifies every observed path and type, including entries that
// a valid inventory cannot contain. It is supplementary to content digests.
func (g *ChangeGuard) Membership() wire.Digest { return g.membership }

// Check reports any change since registration, re-reading uncovered files.
func (g *ChangeGuard) Check() error { return g.check(true) }

// Sweep re-reads the files no kernel event covers and keeps any difference for
// the next check. Lease commits call it just before taking the writer lock, so
// that CheckEvents under the lock still reports a change made before the sweep.
// Store writers (authority.Session) change files only by creating, linking,
// renaming or removing entries, which watched directories report at any time.
// Only an in-place write or mode change to an uncovered file by an actor
// outside the writer lock, made after the sweep read it, goes unreported; no
// Tasks writer edits a watched file in place, and the next journal audit
// refuses such an edit to an intent projection as INTENT_DIVERGED.
func (g *ChangeGuard) Sweep() {
	if g != nil && !g.closed {
		g.watch.sweep()
	}
}

// CheckEvents is Check without the sweep. Its cost does not depend on the
// number of watched files, so it is the check made under the writer lock.
func (g *ChangeGuard) CheckEvents() error { return g.check(false) }

func (g *ChangeGuard) check(sweep bool) error {
	if g == nil || g.closed {
		return wire.Errorf(wire.CodeSnapshotMoved, "change guard", "closed observation")
	}
	changed, err := g.watch.poll()
	if err == nil && !changed && sweep {
		changed = g.watch.sweep()
	}
	if err != nil {
		return wire.Errorf(wire.CodeUnsupportedFilesystem, "change guard", "%v", err)
	}
	if changed {
		return wire.Errorf(wire.CodeSnapshotMoved, "change guard", "store changed during preparation")
	}
	return nil
}

func (g *ChangeGuard) Close() error {
	if g == nil || g.closed {
		return nil
	}
	g.closed = true
	return g.watch.close()
}

func watchable(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() && !info.IsDir() {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, path, "cannot watch mode %s", info.Mode())
	}
	return info, nil
}

func watchMoved(path string) error {
	return wire.Errorf(wire.CodeSnapshotMoved, path, "watch identity changed")
}
