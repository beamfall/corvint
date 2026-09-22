//go:build windows

package gorunner

import "os"

func openCoverageNoFollow(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_RDONLY, 0)
}

func coverageOwnedByCurrentUser(os.FileInfo) bool { return true }

func coverageSingleLink(os.FileInfo) bool { return true }
