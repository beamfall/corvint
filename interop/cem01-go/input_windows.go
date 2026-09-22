//go:build windows

package main

import "os"

func openInputNonblocking(path string) (*os.File, error) {
	return os.Open(path)
}
