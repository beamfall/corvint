//go:build unix

package contextindex

import (
	"os"
	"syscall"
)

// mapReadOnly maps the whole file read-only and private, so a sectioned
// snapshot body is paged in only where a verb touches it. A failed or empty
// mapping returns nil and the reader falls back to ReadAt.
func mapReadOnly(file *os.File, size int64) []byte {
	if size <= 0 {
		return nil
	}
	mapping, err := syscall.Mmap(int(file.Fd()), 0, int(size), syscall.PROT_READ, syscall.MAP_PRIVATE)
	if err != nil {
		return nil
	}
	return mapping
}

func unmapReadOnly(mapping []byte) {
	if mapping != nil {
		_ = syscall.Munmap(mapping)
	}
}
