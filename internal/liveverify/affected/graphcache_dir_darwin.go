package affected

import (
	"bytes"
	"os"
	"syscall"
	"unsafe"
)

// directoryPath is the path the kernel reports for an open directory.
func directoryPath(file *os.File) (string, error) {
	connection, err := file.SyscallConn()
	if err != nil {
		return "", err
	}
	var buffer [1024]byte // MAXPATHLEN
	var errno syscall.Errno
	if err := connection.Control(func(descriptor uintptr) {
		_, _, errno = syscall.Syscall(syscall.SYS_FCNTL, descriptor, syscall.F_GETPATH, uintptr(unsafe.Pointer(&buffer[0])))
	}); err != nil {
		return "", err
	}
	if errno != 0 {
		return "", errno
	}
	name, _, _ := bytes.Cut(buffer[:], []byte{0})
	return string(name), nil
}
