//go:build windows

package main

import "os"

func openInputNonblocking(path string) (*os.File, error) {
	return os.Open(path)
}

func openStableNoFollow(parent *os.Root, part string, directory bool) (*os.File, error) {
	return nil, errNotRegular
}
