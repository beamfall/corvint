//go:build !darwin && !linux

package main

import "os"

func openBoundedFile(path string) (*os.File, error) {
	return os.Open(path)
}
