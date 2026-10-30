//go:build !(darwin || linux)

package corpusindex

import "os"

// openInput is os.Open where the platform has no O_NONBLOCK open; the
// metadata check before it still refuses a FIFO.
func openInput(path string) (*os.File, error) {
	return os.Open(path)
}
