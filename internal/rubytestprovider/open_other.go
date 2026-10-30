//go:build !darwin && !linux

package rubytestprovider

import "os"

// openNoFollow opens name read-only; readBounded's identity check refuses a
// file other than the one its Lstat admitted.
func openNoFollow(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_RDONLY, 0)
}
