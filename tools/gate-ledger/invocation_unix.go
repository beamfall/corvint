//go:build unix

package main

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// lookPath is exec.LookPath over the invocation's PATH instead of the
// process's (V1-4188): the result and error exec.Command would store in
// Path and Err.
func lookPath(file string, inv invocation) (string, error) {
	if strings.Contains(file, "/") {
		if err := executable(file); err != nil {
			return file, &exec.Error{Name: file, Err: err}
		}
		return file, nil
	}
	for _, dir := range filepath.SplitList(inv.getenv("PATH")) {
		if dir == "" {
			dir = "."
		}
		path := filepath.Join(dir, file)
		if executable(path) == nil {
			if !filepath.IsAbs(path) {
				return path, &exec.Error{Name: file, Err: exec.ErrDot}
			}
			return path, nil
		}
	}
	return file, &exec.Error{Name: file, Err: exec.ErrNotFound}
}

func executable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return errors.New("is a directory")
	}
	if info.Mode()&0o111 == 0 {
		return fs.ErrPermission
	}
	return nil
}

// tempDir is os.TempDir over the invocation's TMPDIR.
func tempDir(inv invocation) string {
	if dir := inv.getenv("TMPDIR"); dir != "" {
		return dir
	}
	return "/tmp"
}

// userHomeDir is os.UserHomeDir over the invocation's HOME.
func userHomeDir(inv invocation) (string, error) {
	if home := inv.getenv("HOME"); home != "" {
		return home, nil
	}
	return "", errors.New("$HOME is not defined")
}
