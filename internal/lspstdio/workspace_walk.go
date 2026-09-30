// SPDX-License-Identifier: AGPL-3.0-or-later
package lspstdio

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

type workspaceDirectoryReader interface {
	ReadDir(int) ([]os.DirEntry, error)
}

// Read at most the remaining global allowance plus one overflow witness.
func readWorkspaceChunk(ctx context.Context, r workspaceDirectoryReader, remaining int) ([]os.DirEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if remaining < 0 {
		return nil, errWorkspaceObservation
	}
	n := remaining + 1
	if n > 128 {
		n = 128
	}
	rows, err := r.ReadDir(n)
	if len(rows) > remaining {
		return nil, errWorkspaceObservation
	}
	return rows, err
}
func boundedWorkspaceWalk(ctx context.Context, root string, remaining int, visit func(string, os.DirEntry, error) error) error {
	var walk func(string, int) error
	walk = func(dir string, depth int) error {
		if depth >= 64 {
			return errWorkspaceObservation
		}
		before, err := os.Lstat(dir)
		if err != nil || !before.IsDir() {
			return errWorkspaceObservation
		}
		f, err := openWorkspaceFile(dir)
		if err != nil {
			return err
		}
		defer f.Close()
		opened, err := f.Stat()
		if err != nil || !os.SameFile(before, opened) {
			return errWorkspaceObservation
		}
		for {
			rows, readErr := readWorkspaceChunk(ctx, f, remaining)
			if readErr != nil && readErr != io.EOF {
				return readErr
			}
			remaining -= len(rows)
			for _, row := range rows {
				path := filepath.Join(dir, row.Name())
				err := visit(path, row, nil)
				if err == filepath.SkipDir {
					continue
				}
				if err != nil {
					return err
				}
				if row.IsDir() {
					if err := walk(path, depth+1); err != nil {
						return err
					}
				}
			}
			if readErr == io.EOF {
				break
			}
		}
		after, err := os.Lstat(dir)
		if err != nil || !os.SameFile(before, after) || before.ModTime() != after.ModTime() || before.Size() != after.Size() {
			return errWorkspaceObservation
		}
		return nil
	}
	return walk(root, 0)
}
