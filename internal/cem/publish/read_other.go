//go:build windows

package publish

import "os"

func openBoundedInput(path string) (*os.File, error) {
	return os.Open(path)
}
