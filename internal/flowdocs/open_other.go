//go:build !darwin && !linux

package flowdocs

import "os"

func openRootFile(root *os.Root, name string) (*os.File, error) {
	return nil, fail("regular-file no-follow reader unsupported on this platform")
}
