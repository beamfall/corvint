//go:build windows

package appflows

import "os"

func openInput(path string) (*os.File, error) {
	return os.Open(path)
}
