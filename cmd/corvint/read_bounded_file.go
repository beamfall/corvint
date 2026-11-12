package main

import (
	"errors"
	"os"
)

func openBoundedRegularFile(path string) (*os.File, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, errors.New("file is not a regular file")
	}
	file, err := openBoundedFile(path)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		file.Close()
		return nil, errors.New("file identity changed")
	}
	return file, nil
}
