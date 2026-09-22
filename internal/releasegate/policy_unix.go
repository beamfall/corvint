//go:build !windows

package releasegate

import (
	"errors"
	"io"
	"os"
	"syscall"
)

// readPinnedPolicy reads one regular policy descriptor without following a
// final symlink.  Its identity and size are checked before and after the
// bounded read so a pathname swap cannot be mistaken for pinned policy bytes.
func readPinnedPolicy(name string, limit int) ([]byte, error) {
	fd, err := syscall.Open(name, syscall.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, errors.New("external policy cannot be opened without following links")
	}
	file := os.NewFile(uintptr(fd), name)
	defer file.Close()
	var before syscall.Stat_t
	if err := syscall.Fstat(fd, &before); err != nil || before.Mode&syscall.S_IFMT != syscall.S_IFREG || before.Size < 0 || before.Size > int64(limit) {
		return nil, errors.New("external policy is not a bounded regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil || len(data) > limit {
		return nil, errors.New("external policy exceeds bounded read")
	}
	var after syscall.Stat_t
	if err := syscall.Fstat(fd, &after); err != nil || before.Dev != after.Dev || before.Ino != after.Ino || before.Size != after.Size || int64(len(data)) != before.Size {
		return nil, errors.New("external policy changed while reading")
	}
	return data, nil
}
