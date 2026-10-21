package sessiontest

import (
	"fmt"
	"os"
	"path/filepath"
)

// openChecked is the open for a platform without O_NOFOLLOW and O_NONBLOCK:
// it opens name through open only after an Lstat shows a regular file, so a
// symlink, FIFO or device is refused before any open, and it refuses the
// opened file unless it is that same file. Remaining limit: a path swapped
// between the Lstat and the open is still opened (and may follow a symlink or
// briefly block) before the identity check refuses it.
func openChecked(name string, open func(string) (*os.File, error)) (*os.File, error) {
	before, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", filepath.Base(name))
	}
	file, err := open(name)
	if err != nil {
		return nil, err
	}
	if opened, err := file.Stat(); err != nil || !os.SameFile(before, opened) {
		file.Close()
		return nil, fmt.Errorf("%s changed while it was opened", filepath.Base(name))
	}
	return file, nil
}
