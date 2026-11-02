//go:build !unix

package extevidence

import "os"

// openSource is a plain open where the platform has no FIFO or O_NONBLOCK;
// the caller's fstat still refuses an opened file that is not regular.
func openSource(path string) (*os.File, error) {
	return os.Open(path)
}
