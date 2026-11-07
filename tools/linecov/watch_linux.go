//go:build linux

// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// saveNotifications blocks on native close-write/rename events, with bounded
// coalescing while tests run. Atomic editor replacement is watched at directory
// level; no permanent daemon or polling process is started. Git-ignored non-Go
// output does not schedule a run; ignored Go source still compiles and is relevant.
func saveNotifications(ctx context.Context, root string) (<-chan struct{}, error) {
	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC | syscall.IN_NONBLOCK)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), "linecov-inotify")
	watches := &directoryWatches{fd: fd, root: root, paths: map[string]int{}}
	if err := watches.refresh(); err != nil {
		file.Close()
		return nil, err
	}
	ch := make(chan struct{}, 1)
	go func() { <-ctx.Done(); file.Close() }()
	go func() {
		defer close(ch)
		defer file.Close()
		b := make([]byte, 64*1024)
		for {
			n, err := file.Read(b)
			if err != nil {
				return
			}
			if n > 0 {
				relevant := watches.relevant(ctx, b[:n])
				// Install new watches before publishing the save. Overflow means
				// directory changes may have been lost: rescan before replanning.
				if err := watches.events(b[:n]); err != nil {
					// Stop automatic watching rather than continue with gaps.
					return
				}
				if relevant {
					select {
					case ch <- struct{}{}:
					default:
					}
				}
			}
		}
	}()
	return ch, nil
}

type directoryWatches struct {
	fd    int
	root  string
	paths map[string]int
}

// Watch before descending, so a concurrent mkdir is either discovered by the
// walk or queued on its already watched parent. Git metadata is not a save.
func (w *directoryWatches) refresh() error {
	if info, err := os.Stat(w.root); err != nil {
		return err
	} else if !info.IsDir() {
		return fs.ErrInvalid
	}
	previous := w.paths
	paths := map[string]int{}
	descriptors := map[int]bool{}
	err := filepath.WalkDir(w.root, func(p string, d fs.DirEntry, err error) error {
		if os.IsNotExist(err) {
			return nil // concurrently removed; its parent event remains queued
		}
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if d.Name() == ".git" && p != w.root {
			return filepath.SkipDir
		}
		// Always re-add: an overflow may have hidden deletion and recreation
		// at the same path, leaving a descriptor for a different inode.
		wd, err := syscall.InotifyAddWatch(w.fd, p, syscall.IN_CLOSE_WRITE|syscall.IN_MOVED_TO|syscall.IN_MOVED_FROM|syscall.IN_DELETE|syscall.IN_CREATE|syscall.IN_DELETE_SELF|syscall.IN_MOVE_SELF)
		if os.IsNotExist(err) {
			return filepath.SkipDir
		}
		if err != nil {
			return err
		}
		paths[p] = wd
		descriptors[wd] = true
		return nil
	})
	if err != nil {
		return err
	}
	w.paths = paths
	for _, wd := range previous {
		// A rename can reuse the descriptor under the new path.
		if !descriptors[wd] {
			_, _ = syscall.InotifyRmWatch(w.fd, uint32(wd))
		}
	}
	return nil
}

func (w *directoryWatches) events(data []byte) error {
	refresh := false
	for len(data) >= syscall.SizeofInotifyEvent {
		wd := int(int32(binary.NativeEndian.Uint32(data)))
		mask := binary.NativeEndian.Uint32(data[4:])
		n := syscall.SizeofInotifyEvent + int(binary.NativeEndian.Uint32(data[12:]))
		if n > len(data) {
			return fs.ErrInvalid
		}
		if mask&syscall.IN_IGNORED != 0 {
			for p, descriptor := range w.paths {
				if descriptor == wd {
					delete(w.paths, p)
				}
			}
			refresh = true
		}
		if mask&syscall.IN_Q_OVERFLOW != 0 || mask&(syscall.IN_DELETE_SELF|syscall.IN_MOVE_SELF|syscall.IN_UNMOUNT) != 0 || mask&syscall.IN_ISDIR != 0 {
			refresh = true
		}
		data = data[n:]
	}
	if len(data) != 0 {
		return fs.ErrInvalid
	}
	if refresh {
		return w.refresh()
	}
	return nil
}

// relevant decodes one read of inotify events and reports whether any event names
// Go source or a path Git does not ignore. Ignored Go source still compiles into go test (and
// WorkingDiff selects it), so only ignored non-Go paths such as test output are
// suppressed. When Git cannot answer, it reports true rather than miss a save.
func (w *directoryWatches) relevant(ctx context.Context, b []byte) bool {
	gitDir := filepath.Join(w.root, ".git")
	dirs := make(map[int32]string, len(w.paths))
	for path, wd := range w.paths {
		dirs[int32(wd)] = path
	}
	var paths []string
	for len(b) >= syscall.SizeofInotifyEvent {
		wd := int32(binary.NativeEndian.Uint32(b[0:4]))
		mask := binary.NativeEndian.Uint32(b[4:8])
		end := syscall.SizeofInotifyEvent + int(binary.NativeEndian.Uint32(b[12:16]))
		if end > len(b) {
			return true
		}
		name := string(bytes.TrimRight(b[syscall.SizeofInotifyEvent:end], "\x00"))
		b = b[end:]
		if mask&(syscall.IN_Q_OVERFLOW|syscall.IN_IGNORED|syscall.IN_DELETE_SELF|syscall.IN_MOVE_SELF|syscall.IN_UNMOUNT) != 0 {
			return true // lost ownership requires a conservative replan after rescan
		}
		dir, ok := dirs[wd]
		if !ok || name == "" {
			return true
		}
		path := filepath.Join(dir, name)
		if path == gitDir || strings.HasPrefix(path, gitDir+string(filepath.Separator)) {
			continue
		}
		if strings.HasSuffix(name, ".go") {
			return true
		}
		paths = append(paths, path)
	}
	if len(b) != 0 {
		return true
	}
	if len(paths) == 0 {
		return false
	}
	cmd := exec.CommandContext(ctx, "git", append([]string{"check-ignore", "--"}, paths...)...)
	cmd.Dir = w.root
	out, err := cmd.Output()
	var exit *exec.ExitError
	if err != nil && !(errors.As(err, &exit) && exit.ExitCode() == 1) {
		return true
	}
	ignored := map[string]bool{}
	for _, l := range strings.Split(string(out), "\n") {
		if l != "" {
			if !filepath.IsAbs(l) {
				l = filepath.Join(w.root, l)
			}
			ignored[l] = true
		}
	}
	for _, p := range paths {
		if !ignored[p] {
			return true
		}
	}
	return false
}
