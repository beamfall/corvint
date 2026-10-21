//go:build !unix

package sessiontest

import "os"

// openInput falls back to openChecked where the open cannot refuse a final
// symlink or avoid blocking on a FIFO itself.
func openInput(path string) (*os.File, error) {
	return openChecked(path, os.Open)
}
