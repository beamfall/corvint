//go:build !(darwin || linux)

package appmap

import "os"

func openInput(path string) (*os.File, error) {
	return os.Open(path)
}
