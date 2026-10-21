//go:build windows

package publish

import "os"

func openBoundedInput(path string) (*os.File, error) {
	return os.Open(path)
}

func openBoundedMember(root *os.Root, name string) (*os.File, error) {
	return root.Open(name)
}
