//go:build linux

// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// saveNotifications blocks on native close-write/rename events, with bounded
// coalescing while tests run. Atomic editor replacement is watched at directory
// level; no permanent daemon or polling process is started.
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
		dirs[filepath.Join(root, filepath.Dir(p))] = true
	}
	for d := range dirs {
		if _, err := syscall.InotifyAddWatch(fd, d, syscall.IN_CLOSE_WRITE|syscall.IN_MOVED_TO|syscall.IN_DELETE|syscall.IN_CREATE); err != nil {
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
			if n > 0 {
				select {
				case ch <- struct{}{}:
				default:
				}
			}
		}
	}()
	return ch, nil
}
