//go:build !(darwin || linux || freebsd || openbsd || netbsd || dragonfly)

package companionrelease

import (
	"fmt"
	"os"
)

func openCopyRegular(path string, before os.FileInfo) (*os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	after, err := f.Stat()
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
		_ = f.Close()
		return nil, fmt.Errorf("cache member changed or is not regular: %s", path)
	}
	return f, nil
}
