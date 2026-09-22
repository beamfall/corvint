//go:build darwin

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net"
	"os"
	"syscall"
	"unsafe"
)

func platformPeerIdentity(connection net.Conn, expectedPath, expectedCDHash string) (peerIdentity, error) {
	unix, ok := connection.(*net.UnixConn)
	if !ok {
		return peerIdentity{}, errors.New("peer-metadata-unavailable")
	}
	raw, err := unix.SyscallConn()
	if err != nil {
		return peerIdentity{}, err
	}
	var pid uint32
	var controlErr error
	err = raw.Control(func(fd uintptr) {
		size := uint32(4)
		_, _, errno := syscall.Syscall6(syscall.SYS_GETSOCKOPT, fd, 0, 2, uintptr(unsafe.Pointer(&pid)), uintptr(unsafe.Pointer(&size)), 0)
		if errno != 0 || size != 4 {
			controlErr = errors.New("peer-metadata-unavailable")
		}
	})
	if err != nil || controlErr != nil {
		return peerIdentity{}, errors.New("peer-metadata-unavailable")
	}
	bsd := make([]byte, 136)
	if count, err := procInfoObserver(pid, 3, bsd); err != nil || count != len(bsd) {
		return peerIdentity{}, errors.New("peer-metadata-unavailable")
	}
	path := make([]byte, 4096)
	if count, err := procInfoObserver(pid, 11, path); err != nil || count <= 0 {
		return peerIdentity{}, errors.New("peer-metadata-unavailable")
	}
	end := bytes.IndexByte(path, 0)
	if end < 1 || string(path[:end]) != expectedPath {
		return peerIdentity{}, errors.New("wrong-peer-image")
	}
	status := make([]byte, 4)
	digest := make([]byte, 20)
	if csopsObserver(pid, 0, status) != nil || csopsObserver(pid, 5, digest) != nil {
		return peerIdentity{}, errors.New("peer-code-unavailable")
	}
	flags := binary.LittleEndian.Uint32(status)
	if flags&1 == 0 || flags&0x10000000 != 0 || hex.EncodeToString(digest) != expectedCDHash {
		return peerIdentity{}, errors.New("wrong-peer-code")
	}
	return peerIdentity{pid: pid, birth: append([]byte(nil), bsd[120:136]...), code: digest}, nil
}

func procInfoObserver(pid, flavor uint32, buffer []byte) (int, error) {
	count, _, errno := syscall.Syscall6(syscall.SYS_PROC_INFO, 2, uintptr(pid), uintptr(flavor), 0, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if errno != 0 {
		return 0, os.NewSyscallError("proc_info", errno)
	}
	return int(count), nil
}
func csopsObserver(pid, operation uint32, buffer []byte) error {
	_, _, errno := syscall.Syscall6(syscall.SYS_CSOPS, uintptr(pid), uintptr(operation), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), 0, 0)
	if errno != 0 {
		return os.NewSyscallError("csops", errno)
	}
	return nil
}
