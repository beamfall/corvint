//go:build darwin && (arm64 || amd64)

package supervisor

import (
	"encoding/binary"
	"fmt"
	"syscall"
	"unsafe"
)

// Darwin's 64-bit kinfo_proc (648 bytes): extern_proc carries p_stat at 36
// and p_pid at 40; eproc begins at 296 and carries e_ppid at +264 and e_pgid
// at +268. Layout verified against Go 1.27's vendored
// golang.org/x/sys/unix/ztypes_darwin_{arm64,amd64}.go (SizeofKinfoProc).
const (
	kinfoProcSize = 0x288
	kinfoStat     = 36
	kinfoPID      = 40
	kinfoPPID     = 296 + 264
	kinfoPGID     = 296 + 268
	kinfoZombie   = 5 // SZOMB
)

// processRows snapshots the process table with one kern.proc.all sysctl
// instead of forking ps every scan (CAL-V0-136): the same pid, ppid, pgid
// and zombie facts `ps -axo pid=,ppid=,pgid=,stat=` reports, pid 0 excluded
// as ps excludes it.
func processRows() ([]processRow, error) {
	mib := [3]int32{1, 14, 0} // CTL_KERN, KERN_PROC, KERN_PROC_ALL
	for attempt := 0; attempt < 8; attempt++ {
		var n uintptr
		if _, _, errno := syscall.Syscall6(syscall.SYS___SYSCTL, uintptr(unsafe.Pointer(&mib[0])), 3, 0, uintptr(unsafe.Pointer(&n)), 0, 0); errno != 0 {
			return nil, errno
		}
		n += n / 4
		b := make([]byte, n)
		_, _, errno := syscall.Syscall6(syscall.SYS___SYSCTL, uintptr(unsafe.Pointer(&mib[0])), 3, uintptr(unsafe.Pointer(&b[0])), uintptr(unsafe.Pointer(&n)), 0, 0)
		if errno == syscall.ENOMEM {
			continue
		}
		if errno != 0 {
			return nil, errno
		}
		return decodeKinfoProcs(b[:n])
	}
	return nil, fmt.Errorf("process table kept growing")
}

func decodeKinfoProcs(b []byte) ([]processRow, error) {
	if len(b)%kinfoProcSize != 0 {
		return nil, fmt.Errorf("unsupported kern.proc.all layout")
	}
	rows := make([]processRow, 0, len(b)/kinfoProcSize)
	for off := 0; off < len(b); off += kinfoProcSize {
		k := b[off : off+kinfoProcSize]
		r := processRow{
			pid:    int(int32(binary.LittleEndian.Uint32(k[kinfoPID:]))),
			ppid:   int(int32(binary.LittleEndian.Uint32(k[kinfoPPID:]))),
			pgid:   int(int32(binary.LittleEndian.Uint32(k[kinfoPGID:]))),
			zombie: k[kinfoStat] == kinfoZombie,
		}
		if r.pid < 0 || r.ppid < 0 || r.pgid < 0 {
			return nil, fmt.Errorf("unexpected process row")
		}
		if r.pid != 0 {
			rows = append(rows, r)
		}
	}
	return rows, nil
}
