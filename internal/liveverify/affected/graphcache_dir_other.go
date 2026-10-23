//go:build !darwin && !linux

package affected

import (
	"errors"
	"io/fs"
	"os"
)

const openNonblocking = 0

// directoryPath has no kernel source here; the graph cache is never used on
// this platform (AFP-V0-392).
func directoryPath(*os.File) (string, error) {
	return "", errors.New("unsupported platform")
}

func privateDirectory(fs.FileInfo) bool { return false }
