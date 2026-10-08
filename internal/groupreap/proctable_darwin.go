//go:build darwin

package groupreap

import (
	"encoding/binary"
	"fmt"
	"syscall"
	"unsafe"
)

// Darwin 64-bit struct kinfo_proc layout (sys/sysctl.h, sys/proc.h): the
// same 648 bytes on amd64 and arm64.
const (
	kinfoProcSize   = 0x288
	kinfoStartSec   = 0   // kp_proc.p_starttime.tv_sec (int64)
	kinfoStartUsec  = 8   // kp_proc.p_starttime.tv_usec (int32)
	kinfoStat       = 36  // kp_proc.p_stat (char)
	kinfoPID        = 40  // kp_proc.p_pid (int32)
	kinfoPPID       = 560 // kp_eproc.e_ppid (int32)
	kinfoPGID       = 564 // kp_eproc.e_pgid (int32)
	darwinStateStop = 4   // SSTOP
	darwinStateZomb = 5   // SZOMB
	ctlKern         = 1
	kernProc        = 14
	kernProcAll     = 0
	// maxDarwinProcBytes bounds one process-table read (about 25000 entries).
	maxDarwinProcBytes = 16 << 20
)

// readProcessTable reads every process from sysctl kern.proc.all. It sends
// no signal and grants no signal authority.
func readProcessTable() (map[int]Process, error) {
	mib := [3]int32{ctlKern, kernProc, kernProcAll}
	for attempt := 0; attempt < 8; attempt++ {
		var size uintptr
		if err := sysctlRaw(mib[:], nil, &size); err != nil {
			return nil, err
		}
		// Headroom for processes started between the two calls.
		size += size / 8
		if size == 0 || size > maxDarwinProcBytes {
			return nil, fmt.Errorf("%w: process table size %d", errProcessTable, size)
		}
		buf := make([]byte, size)
		n := size
		if err := sysctlRaw(mib[:], &buf[0], &n); err != nil {
			if err == syscall.ENOMEM {
				continue
			}
			return nil, err
		}
		return parseKinfoProcs(buf[:n])
	}
	return nil, fmt.Errorf("%w: process table kept growing", errProcessTable)
}

func sysctlRaw(mib []int32, old *byte, oldlen *uintptr) error {
	_, _, errno := syscall.Syscall6(syscall.SYS___SYSCTL, uintptr(unsafe.Pointer(&mib[0])), uintptr(len(mib)),
		uintptr(unsafe.Pointer(old)), uintptr(unsafe.Pointer(oldlen)), 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}

func parseKinfoProcs(data []byte) (map[int]Process, error) {
	if len(data)%kinfoProcSize != 0 {
		return nil, fmt.Errorf("%w: %d bytes is not a whole kinfo_proc array", errProcessTable, len(data))
	}
	le := binary.LittleEndian
	table := make(map[int]Process, len(data)/kinfoProcSize)
	for off := 0; off < len(data); off += kinfoProcSize {
		e := data[off : off+kinfoProcSize]
		p := Process{
			PID:   int(int32(le.Uint32(e[kinfoPID:]))),
			PPID:  int(int32(le.Uint32(e[kinfoPPID:]))),
			PGID:  int(int32(le.Uint32(e[kinfoPGID:]))),
			Start: int64(le.Uint64(e[kinfoStartSec:]))*1_000_000 + int64(int32(le.Uint32(e[kinfoStartUsec:]))),
		}
		switch int8(e[kinfoStat]) {
		case darwinStateStop:
			p.State = StateStopped
		case darwinStateZomb:
			p.State = StateZombie
		default:
			p.State = StateRunning
		}
		if p.PID < 0 {
			return nil, fmt.Errorf("%w: negative pid", errProcessTable)
		}
		table[p.PID] = p
	}
	return table, nil
}

func processSession(pid int) (int, error) { return syscall.Getsid(pid) }
