//go:build darwin

package authoritystore

import (
	"bytes"
	"context"
	"encoding/binary"
	"syscall"
	"unsafe"
)

// Socket metadata is a bounded read of kernel object relationships, never
// application payload or a caller-provided connection assertion. ABI offsets
// are from the Darwin 64-bit SDK sys/proc_info.h socket_fdinfo (792 bytes).
type unixSocket struct {
	fd            uint32
	local, peer   uint64
	bound, remote string
}

func socketInfo(pid, fd uint32) (unixSocket, error) {
	b := make([]byte, 792)
	n, _, errno := syscall.Syscall6(syscall.SYS_PROC_INFO, 3, uintptr(pid), 3, uintptr(fd), uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
	if errno != 0 || n != uintptr(len(b)) {
		return unixSocket{}, errUnavailable
	}
	u32 := func(i int) uint32 { return binary.LittleEndian.Uint32(b[i : i+4]) }
	u64 := func(i int) uint64 { return binary.LittleEndian.Uint64(b[i : i+8]) }
	state := binary.LittleEndian.Uint16(b[192:194])
	// AF_UNIX / SOCK_STREAM / SOCKINFO_UN; reject shutdown or transitional peers.
	if u32(176) != 1 || u32(184) != 1 || u32(256) != 3 || state&2 == 0 || state&0x601d != 0 {
		return unixSocket{}, errUnavailable
	}
	address := func(i int) string {
		n := int(b[i])
		if n <= 2 || n > 106 || b[i+1] != 1 {
			return ""
		}
		raw := b[i+2 : i+n]
		if end := bytes.IndexByte(raw, 0); end >= 0 {
			raw = raw[:end]
		}
		return string(raw)
	}
	s := unixSocket{fd: fd, local: u64(160), peer: u64(264), bound: address(280), remote: address(535)}
	if s.local == 0 || s.peer == 0 || s.local == s.peer {
		return unixSocket{}, errUnavailable
	}
	return s, nil
}

func processSockets(ctx context.Context, pid uint32) ([]unixSocket, error) {
	// Refuse truncation rather than silently losing a connection. No global
	// process scan: both process identities come from protected admission/ancestry.
	b := make([]byte, 4096*8)
	n, _, errno := syscall.Syscall6(syscall.SYS_PROC_INFO, 2, uintptr(pid), 1, 0, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
	if errno != 0 || n >= uintptr(len(b)) || n%8 != 0 {
		return nil, errUnavailable
	}
	var sockets []unixSocket
	for i := 0; i < int(n); i += 8 {
		if ctx.Err() != nil {
			return nil, errUnavailable
		}
		if binary.LittleEndian.Uint32(b[i+4:i+8]) != 2 {
			continue
		}
		fd := binary.LittleEndian.Uint32(b[i : i+4])
		// Closed/unrelated sockets are not candidates. A selected candidate is
		// independently reread at both ends below and all process identities checked.
		if s, e := socketInfo(pid, fd); e == nil {
			sockets = append(sockets, s)
		}
	}
	return sockets, nil
}

func verifySocketPair(ctx context.Context, app, engine uint32, path string) error {
	if path == "" {
		return errUnavailable
	}
	left, e := processSockets(ctx, app)
	if e != nil {
		return e
	}
	right, e := processSockets(ctx, engine)
	if e != nil {
		return e
	}
	for _, a := range left {
		for _, b := range right {
			if a.local != b.peer || a.peer != b.local || a.remote != path || b.bound != path {
				continue
			}
			againA, ea := socketInfo(app, a.fd)
			againB, eb := socketInfo(engine, b.fd)
			if ctx.Err() == nil && ea == nil && eb == nil && a == againA && b == againB {
				return nil
			}
		}
	}
	return errUnavailable
}
