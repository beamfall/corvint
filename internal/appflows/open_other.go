//go:build !(darwin || linux)

package appflows

import "os"

func openInput(path string) (*os.File, error) {
	return os.Open(path)
}

func openRootInput(r *os.Root, name string) (*os.File, error) {
	return r.Open(name)
}
