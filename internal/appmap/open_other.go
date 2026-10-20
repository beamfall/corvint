//go:build !unix

package appmap

import "os"

// openInput is a plain open where the platform has no O_NOFOLLOW or O_NONBLOCK. readRegular
// Lstat-refuses a symlink or non-regular file before calling it and refuses an opened file that
// is not the one it checked; the remaining limit is that a path swapped between the Lstat and
// this open is still opened (and may follow a symlink or briefly block) before that refusal.
func openInput(path string) (*os.File, error) {
	return os.Open(path)
}
