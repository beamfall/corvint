package affected

import (
	"os"
	"strconv"
)

// directoryPath is the path the kernel reports for an open directory.
func directoryPath(file *os.File) (string, error) {
	connection, err := file.SyscallConn()
	if err != nil {
		return "", err
	}
	var name string
	var readErr error
	if err := connection.Control(func(descriptor uintptr) {
		name, readErr = os.Readlink("/proc/self/fd/" + strconv.FormatUint(uint64(descriptor), 10))
	}); err != nil {
		return "", err
	}
	return name, readErr
}
