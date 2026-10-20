//go:build !(darwin || linux)

package sessiontest

import "os"

func openInput(path string) (*os.File, error) {
	return os.Open(path)
}
