//go:build !(darwin || linux)

package session

import "os"

// openWatched is os.Open where the platform has no O_NONBLOCK open; the
// regular-file check before it still refuses a FIFO.
func openWatched(path string) (*os.File, error) {
	return os.Open(path)
}
