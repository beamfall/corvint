//go:build darwin

package groupreap

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// processArguments reads pid's argument vector from KERN_PROCARGS2: argc, the
// exec path, NUL padding, then argc argument strings. Unlike the environment
// that follows them, arguments stay readable for platform binaries.
func processArguments(pid int) ([][]byte, error) {
	mib := [3]int32{ctlKern, 49 /* KERN_PROCARGS2 */, int32(pid)}
	buf := make([]byte, maxArguments)
	n := uintptr(len(buf))
	if err := sysctlRaw(mib[:], &buf[0], &n); err != nil {
		return nil, err
	}
	raw := buf[:n]
	if len(raw) < 4 {
		return nil, fmt.Errorf("%w: short procargs for %d", errProcessTable, pid)
	}
	argc := int(int32(binary.LittleEndian.Uint32(raw)))
	raw = raw[4:]
	i := bytes.IndexByte(raw, 0)
	if i < 0 || argc < 0 {
		return nil, fmt.Errorf("%w: malformed procargs for %d", errProcessTable, pid)
	}
	raw = bytes.TrimLeft(raw[i:], "\x00")
	return splitArguments(raw, argc), nil
}
