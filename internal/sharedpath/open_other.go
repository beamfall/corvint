//go:build !(darwin || linux)

package sharedpath

import "os"

func openInput(path string) (*os.File, error) {
	return os.Open(path)
}
