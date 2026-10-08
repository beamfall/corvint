//go:build darwin && (arm64 || amd64)

package groupreap

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

// RetirementSupported is true only where the kinfo_proc layout below and the
// KERN_PROCARGS2 environment read were qualified (macOS 26 arm64).
const RetirementSupported = true

const sigStop, sigKill = syscall.SIGSTOP, syscall.SIGKILL

// Darwin's 64-bit kinfo_proc (648 bytes): extern_proc starts with p_starttime
// (sec int64, usec int32) and carries p_stat at 36 and p_pid at 40; eproc
// begins at 296 with e_ucred.cr_uid at +124, e_ppid at +264. Layout verified
// against golang.org/x/sys/unix ztypes_darwin_{arm64,amd64}.go.
const (
	kinfoSize   = 0x288
	kinfoStat   = 36
	kinfoPID    = 40
	kinfoUID    = 296 + 124
	kinfoPPID   = 296 + 264
	kinfoZombie = 5 // SZOMB
	kernProc    = 14
	kernArgmax  = 8
	kernArgs2   = 49 // KERN_PROCARGS2
)

func platformPrimitives() retirePrimitives {
	return retirePrimitives{rows: darwinRows, identity: darwinIdentity, environ: darwinEnviron, signal: syscall.Kill, sleep: time.Sleep}
}

func sysctl(mib []int32, buf []byte) ([]byte, error) {
	n := uintptr(len(buf))
	var p unsafe.Pointer
	if len(buf) > 0 {
		p = unsafe.Pointer(&buf[0])
	}
	_, _, errno := syscall.Syscall6(syscall.SYS___SYSCTL, uintptr(unsafe.Pointer(&mib[0])), uintptr(len(mib)), uintptr(p), uintptr(unsafe.Pointer(&n)), 0, 0)
	if errno != 0 {
		return nil, errno
	}
	return buf[:n], nil
}

func decodeKinfo(k []byte) (procRow, error) {
	r := procRow{
		pid:    int(int32(binary.LittleEndian.Uint32(k[kinfoPID:]))),
		ppid:   int(int32(binary.LittleEndian.Uint32(k[kinfoPPID:]))),
		uid:    int(binary.LittleEndian.Uint32(k[kinfoUID:])),
		sec:    int64(binary.LittleEndian.Uint64(k[0:])),
		usec:   int(int32(binary.LittleEndian.Uint32(k[8:]))),
		zombie: k[kinfoStat] == kinfoZombie,
	}
	if r.pid < 0 || r.ppid < 0 || r.usec < 0 || r.usec >= 1000000 {
		return procRow{}, fmt.Errorf("unexpected kinfo_proc row")
	}
	return r, nil
}

func darwinRows() ([]procRow, error) {
	mib := []int32{1, kernProc, 0} // KERN_PROC_ALL
	for attempt := 0; attempt < 8; attempt++ {
		var n uintptr
		if _, _, errno := syscall.Syscall6(syscall.SYS___SYSCTL, uintptr(unsafe.Pointer(&mib[0])), 3, 0, uintptr(unsafe.Pointer(&n)), 0, 0); errno != 0 {
			return nil, errno
		}
		b, err := sysctl(mib, make([]byte, n+n/4))
		if err == syscall.ENOMEM {
			continue
		}
		if err != nil {
			return nil, err
		}
		if len(b)%kinfoSize != 0 {
			return nil, fmt.Errorf("unsupported kern.proc.all layout")
		}
		rows := make([]procRow, 0, len(b)/kinfoSize)
		for off := 0; off < len(b); off += kinfoSize {
			r, err := decodeKinfo(b[off : off+kinfoSize])
			if err != nil {
				return nil, err
			}
			if r.pid != 0 {
				rows = append(rows, r)
			}
		}
		return rows, nil
	}
	return nil, fmt.Errorf("process table kept growing")
}

func darwinIdentity(pid int) (procRow, bool, error) {
	b, err := sysctl([]int32{1, kernProc, 1, int32(pid)}, make([]byte, kinfoSize))
	if err != nil {
		return procRow{}, false, err
	}
	if len(b) == 0 {
		return procRow{}, false, nil
	}
	if len(b) != kinfoSize {
		return procRow{}, false, fmt.Errorf("unsupported kern.proc.pid layout")
	}
	r, err := decodeKinfo(b)
	if err != nil || r.pid != pid {
		return procRow{}, false, fmt.Errorf("unsupported kern.proc.pid layout")
	}
	return r, true, nil
}

// darwinEnviron reports whether pid's initial environment holds entry exactly.
// KERN_PROCARGS2 is argc, the exec path, padding, argc argv strings, then the
// environment strings; only same-user processes are readable.
func darwinEnviron(pid int, entry string) (bool, error) {
	max, err := sysctl([]int32{1, kernArgmax}, make([]byte, 4))
	if err != nil || len(max) != 4 {
		return false, fmt.Errorf("kern.argmax unavailable")
	}
	b, err := sysctl([]int32{1, kernArgs2, int32(pid)}, make([]byte, binary.LittleEndian.Uint32(max)))
	if err != nil {
		return false, err
	}
	if len(b) < 4 {
		return false, fmt.Errorf("short procargs")
	}
	argc := int(int32(binary.LittleEndian.Uint32(b)))
	b = b[4:]
	i := bytes.IndexByte(b, 0) // exec path
	if i < 0 || argc < 0 {
		return false, fmt.Errorf("malformed procargs")
	}
	b = b[i:]
	for len(b) > 0 && b[0] == 0 {
		b = b[1:]
	}
	want := []byte(entry)
	for field := 0; len(b) > 0; field++ {
		i = bytes.IndexByte(b, 0)
		if i < 0 {
			i = len(b)
		}
		if field >= argc {
			if i == 0 {
				return false, nil
			}
			if bytes.Equal(b[:i], want) {
				return true, nil
			}
		}
		if i == len(b) {
			break
		}
		b = b[i+1:]
	}
	return false, nil
}
