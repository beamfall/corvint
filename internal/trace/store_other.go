//go:build !darwin && !linux

package trace

import (
	"fmt"
	"os"
)

func openNoFollowDirectory(*os.Root, string) (*os.File, *os.Root, error) {
	return nil, nil, fmt.Errorf("secure trace storage is unsupported on this platform")
}

func openNoFollowMember(*os.Root, string, int, os.FileMode) (*os.File, error) {
	return nil, fmt.Errorf("secure trace storage is unsupported on this platform")
}

func descriptorLinks(os.FileInfo) uint64 { return 0 }

func lockDescriptor(*os.File) error {
	return fmt.Errorf("secure trace storage is unsupported on this platform")
}

func unlockDescriptor(*os.File) {}

func sameMetadata(left, right os.FileInfo) bool {
	return os.SameFile(left, right) && left.Size() == right.Size() && left.Mode() == right.Mode() && left.ModTime().Equal(right.ModTime())
}
