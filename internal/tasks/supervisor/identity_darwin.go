//go:build darwin && (arm64 || amd64)

package supervisor

import (
	"encoding/binary"
	"fmt"
	"syscall"
	"unsafe"
)

// Darwin's 64-bit extern_proc begins with timeval, two pointers, flags,
// status/padding and pid. Validate pid before accepting the start identity.
func ProcessIdentity(pid int) (string, error) {
	mib := [4]int32{1, 14, 1, int32(pid)}
	b := make([]byte, 4096)
	n := uintptr(len(b))
	_, _, errno := syscall.Syscall6(syscall.SYS___SYSCTL, uintptr(unsafe.Pointer(&mib[0])), 4, uintptr(unsafe.Pointer(&b[0])), uintptr(unsafe.Pointer(&n)), 0, 0)
	if errno != 0 {
		return "", errno
	}
	if n == 0 {
		return "", nil
	}
	b = b[:n]
	if len(b) < 44 || int(binary.LittleEndian.Uint32(b[40:44])) != pid {
		return "", fmt.Errorf("unsupported kern.proc.pid layout")
	}
	sec := binary.LittleEndian.Uint64(b[:8])
	usec := binary.LittleEndian.Uint32(b[8:12])
	if sec == 0 || usec >= 1000000 {
		return "", fmt.Errorf("invalid process start time")
	}
	return fmt.Sprintf("darwin:%d:%06d", sec, usec), nil
}
