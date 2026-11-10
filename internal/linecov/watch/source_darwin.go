//go:build darwin

// SPDX-License-Identifier: Apache-2.0
package watch

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// native blocks on kqueue vnode events (LPCV-V0-1086): every directory under
// root and every regular file in it holds one event-only descriptor, so an
// in-place write reaches the file's descriptor and an atomic replacement,
// creation, deletion or rename reaches the directory's. A directory event
// rescans that directory and compares each entry's inode with its watched
// descriptor, so a replaced file is rewatched even when its own delete event
// was coalesced away. Pure Go: no cgo, no FSEvents daemon, no polling. A
// system that cannot grant one descriptor per watched file refuses up front
// (EMFILE/ENFILE) rather than watch a subset.
func native(ctx context.Context, root string) (*Source, error) {
	kq, err := syscall.Kqueue()
	if err != nil {
		return nil, fmt.Errorf("kqueue: %w", err)
	}
	var wake [2]int
	if err := syscall.Pipe(wake[:]); err != nil {
		syscall.Close(kq)
		return nil, fmt.Errorf("kqueue wake pipe: %w", err)
	}
	syscall.CloseOnExec(wake[0])
	syscall.CloseOnExec(wake[1])
	w := &vnodeWatches{kq: kq, root: root, fds: map[string]int{}, paths: map[int]string{}, dirs: map[string]bool{}}
	if err := w.register(wake[0], syscall.EVFILT_READ, 0); err != nil {
		w.closeAll()
		syscall.Close(wake[0])
		syscall.Close(wake[1])
		return nil, fmt.Errorf("kqueue: %w", err)
	}
	if err := w.addTree(root); err != nil {
		w.closeAll()
		syscall.Close(wake[0])
		syscall.Close(wake[1])
		return nil, fmt.Errorf("kqueue: watch %s: %w", root, descriptorHint(err))
	}
	ch := make(chan struct{}, 1)
	s := &Source{C: ch}
	go func() {
		<-ctx.Done()
		_, _ = syscall.Write(wake[1], []byte{0})
	}()
	go func() {
		defer close(ch)
		defer func() {
			w.closeAll()
			syscall.Close(wake[0])
			syscall.Close(wake[1])
		}()
		events := make([]syscall.Kevent_t, 64)
		for {
			n, err := syscall.Kevent(w.kq, nil, events, nil)
			if err == syscall.EINTR {
				continue
			}
			if err != nil {
				s.stop(fmt.Errorf("kevent: %w", err))
				return
			}
			var paths []string
			dirs := map[string]bool{}
			for _, ev := range events[:n] {
				if int(ev.Ident) == wake[0] {
					s.stop(nil)
					return
				}
				if ev.Flags&syscall.EV_ERROR != 0 {
					s.stop(fmt.Errorf("kevent: %w", syscall.Errno(ev.Data)))
					return
				}
				path, ok := w.paths[int(ev.Ident)]
				if !ok {
					continue
				}
				if w.dirs[path] {
					dirs[path] = true
					if ev.Fflags&(syscall.NOTE_DELETE|syscall.NOTE_RENAME|syscall.NOTE_REVOKE) != 0 {
						w.drop(path)
					}
					continue
				}
				paths = append(paths, path)
				if ev.Fflags&(syscall.NOTE_DELETE|syscall.NOTE_RENAME|syscall.NOTE_REVOKE) != 0 {
					// The replacement, if any, is found by its directory's rescan.
					w.drop(path)
				}
			}
			for dir := range dirs {
				changed, err := w.rescan(dir)
				if err != nil {
					s.stop(fmt.Errorf("kqueue watches cannot be restored under %s: %w", root, descriptorHint(err)))
					return
				}
				// The directory itself is a changed path only when neither its
				// rescan nor an event names an entry of it, so Git-ignored output
				// alone stays irrelevant (LPCV-V0-7607).
				if len(changed) == 0 && !namesEntryOf(paths, dir) {
					changed = append(changed, dir)
				}
				paths = append(paths, changed...)
			}
			if _, err := os.Stat(root); err != nil {
				s.stop(fmt.Errorf("kqueue: root %s: %w", root, err))
				return
			}
			if len(paths) > 0 && relevantPaths(ctx, root, paths) {
				select {
				case ch <- struct{}{}:
				default:
				}
			}
		}
	}()
	return s, nil
}

// vnodeWatches is the set of watched paths: one descriptor each, both ways.
type vnodeWatches struct {
	kq    int
	root  string
	fds   map[string]int
	paths map[int]string
	dirs  map[string]bool
}

const vnodeFlags = syscall.NOTE_WRITE | syscall.NOTE_EXTEND | syscall.NOTE_DELETE | syscall.NOTE_RENAME | syscall.NOTE_REVOKE

func (w *vnodeWatches) register(fd int, filter int, fflags uint32) error {
	var ev syscall.Kevent_t
	syscall.SetKevent(&ev, fd, filter, syscall.EV_ADD|syscall.EV_CLEAR)
	ev.Fflags = fflags
	_, err := syscall.Kevent(w.kq, []syscall.Kevent_t{ev}, nil, nil)
	return err
}

// add opens path for events only and registers it; a path that vanished
// between listing and opening is skipped, as its directory event remains.
func (w *vnodeWatches) add(path string, dir bool) error {
	if _, ok := w.fds[path]; ok {
		return nil
	}
	fd, err := syscall.Open(path, syscall.O_EVTONLY|syscall.O_CLOEXEC, 0)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("open %s: %w", path, err)
	}
	if err := w.register(fd, syscall.EVFILT_VNODE, vnodeFlags); err != nil {
		syscall.Close(fd)
		return fmt.Errorf("register %s: %w", path, err)
	}
	w.fds[path], w.paths[fd] = fd, path
	if dir {
		w.dirs[path] = true
	}
	return nil
}

// drop closes and forgets one watched path; a directory takes its subtree.
func (w *vnodeWatches) drop(path string) {
	if w.dirs[path] {
		prefix := path + string(filepath.Separator)
		for p := range w.fds {
			if len(p) > len(prefix) && p[:len(prefix)] == prefix {
				w.dropOne(p)
			}
		}
	}
	w.dropOne(path)
}

func (w *vnodeWatches) dropOne(path string) {
	fd, ok := w.fds[path]
	if !ok {
		return
	}
	syscall.Close(fd)
	delete(w.fds, path)
	delete(w.paths, fd)
	delete(w.dirs, path)
}

func (w *vnodeWatches) closeAll() {
	for _, fd := range w.fds {
		syscall.Close(fd)
	}
	w.fds, w.paths, w.dirs = map[string]int{}, map[int]string{}, map[string]bool{}
	syscall.Close(w.kq)
}

// addTree watches dir, every directory below it except .git, and every
// regular file; symbolic links are names in their directory, not watched
// targets. Watch before descending, so a concurrent mkdir is either seen by
// the walk or by its already watched parent.
func (w *vnodeWatches) addTree(dir string) error {
	if info, err := os.Stat(dir); err != nil {
		return err
	} else if !info.IsDir() {
		return fs.ErrInvalid
	}
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" && p != w.root {
				return filepath.SkipDir
			}
			return w.add(p, true)
		}
		if d.Type().IsRegular() {
			return w.add(p, false)
		}
		return nil
	})
}

// namesEntryOf reports whether any path is a direct entry of dir.
func namesEntryOf(paths []string, dir string) bool {
	for _, p := range paths {
		if filepath.Dir(p) == dir {
			return true
		}
	}
	return false
}

// rescan reconciles one directory after its event: new entries are watched
// (a new directory with its whole tree), entries whose inode no longer
// matches their descriptor are rewatched, and entries that are gone are
// dropped. It returns the paths that were added, replaced or dropped.
func (w *vnodeWatches) rescan(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		w.drop(dir)
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	present := map[string]bool{}
	var added []string
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if e.IsDir() && e.Name() == ".git" {
			continue
		}
		if !e.IsDir() && !e.Type().IsRegular() {
			continue
		}
		present[p] = true
		if fd, ok := w.fds[p]; ok {
			if w.sameInode(fd, p) {
				continue
			}
			w.drop(p)
		}
		if e.IsDir() {
			if err := w.addTree(p); err != nil {
				return added, err
			}
		} else if err := w.add(p, false); err != nil {
			return added, err
		}
		added = append(added, p)
	}
	prefix := dir + string(filepath.Separator)
	for p := range w.fds {
		if len(p) > len(prefix) && p[:len(prefix)] == prefix && filepath.Dir(p) == dir && !present[p] {
			w.drop(p)
			added = append(added, p)
		}
	}
	return added, nil
}

// sameInode reports whether the watched descriptor still names the inode at
// path; a replaced file fails this and is rewatched.
func (w *vnodeWatches) sameInode(fd int, path string) bool {
	var open, now syscall.Stat_t
	if err := syscall.Fstat(fd, &open); err != nil {
		return false
	}
	if err := syscall.Lstat(path, &now); err != nil {
		return false
	}
	return open.Ino == now.Ino && open.Dev == now.Dev
}

// descriptorHint names the one failure a user can fix by configuration: a
// descriptor per watched file needs a limit above the tree's file count.
func descriptorHint(err error) error {
	if errors.Is(err, syscall.EMFILE) || errors.Is(err, syscall.ENFILE) {
		return fmt.Errorf("%w (kqueue needs one open descriptor per watched file and directory; raise `ulimit -n` or use -events-stdin or -once)", err)
	}
	return err
}
