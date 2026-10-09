//go:build !unix

package stepnegation

import (
	"errors"
	"io/fs"
	"os"
)

// lockFile creates name exclusively and removes it on release. A crashed
// holder leaves the file, which refuses later writers until it is removed.
func lockFile(directory *os.Root, name string) (func(), error) {
	file, err := directory.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return nil, ErrBusy
	}
	if err != nil {
		return nil, err
	}
	file.Close()
	return func() { _ = directory.Remove(name) }, nil
}
