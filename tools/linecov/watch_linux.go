//go:build linux

// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

const watchMask = syscall.IN_CLOSE_WRITE | syscall.IN_MOVED_TO | syscall.IN_MOVED_FROM | syscall.IN_DELETE | syscall.IN_CREATE

// saveNotifications blocks on native close-write/rename events, with bounded
// coalescing while tests run. Atomic editor replacement is watched at directory
// level; no permanent daemon or polling process is started. Directories created
// or moved in after start are watched as they appear, tracked directories that
// no longer exist are skipped, and events only for Git-ignored paths (such as
// test output rewritten on every run) never schedule a run.
func saveNotifications(ctx context.Context, root string) (<-chan struct{}, error) {
	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC | syscall.IN_NONBLOCK)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), "linecov-inotify")
	tracked, err := output(root, "git", "ls-files")
	if err != nil {
		file.Close()
		return nil, err
	}
	dirs := map[string]bool{root: true}
	for _, p := range strings.Split(tracked, "\n") {
		if p != "" {
			dirs[filepath.Join(root, filepath.Dir(p))] = true
		}
	}
	w := &watcher{fd: fd, root: root, dirs: map[int32]string{}}
	for d := range dirs {
		if err := w.add(d); err != nil {
			file.Close()
			return nil, err
		}
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
			if w.relevant(ctx, b[:n]) {
				select {
				case ch <- struct{}{}:
				default:
				}
			}
		}
	}()
	return ch, nil
}

type watcher struct {
	fd   int
	root string
	dirs map[int32]string
}

// add watches dir; a directory that no longer exists (deleted before start, or
// removed again before its creation event was read) is skipped, not fatal.
func (w *watcher) add(dir string) error {
	wd, err := syscall.InotifyAddWatch(w.fd, dir, watchMask)
	if err != nil {
		if errors.Is(err, syscall.ENOENT) || errors.Is(err, syscall.ENOTDIR) {
			return nil
		}
		return err
	}
	w.dirs[int32(wd)] = dir
	return nil
}

// relevant decodes one read of inotify events, starts watching new directories
// (and those below them, which may already exist after a move or mkdir -p),
// forgets removed ones, and reports whether any event names a path Git does
// not ignore. When Git cannot answer, it reports true rather than miss a save.
func (w *watcher) relevant(ctx context.Context, b []byte) bool {
	gitDir := filepath.Join(w.root, ".git")
	var paths []string
	for len(b) >= syscall.SizeofInotifyEvent {
		wd := int32(binary.NativeEndian.Uint32(b[0:4]))
		mask := binary.NativeEndian.Uint32(b[4:8])
		end := syscall.SizeofInotifyEvent + int(binary.NativeEndian.Uint32(b[12:16]))
		if end > len(b) {
			break
		}
		name := string(bytes.TrimRight(b[syscall.SizeofInotifyEvent:end], "\x00"))
		b = b[end:]
		if mask&syscall.IN_IGNORED != 0 {
			delete(w.dirs, wd)
			continue
		}
		dir, ok := w.dirs[wd]
		if !ok || name == "" {
			continue
		}
		path := filepath.Join(dir, name)
		if path == gitDir || strings.HasPrefix(path, gitDir+string(filepath.Separator)) {
			continue
		}
		if mask&syscall.IN_ISDIR != 0 && mask&(syscall.IN_CREATE|syscall.IN_MOVED_TO) != 0 {
			filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
				if err == nil && d.IsDir() {
					w.add(p)
				}
				return nil
			})
		}
		paths = append(paths, path)
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
