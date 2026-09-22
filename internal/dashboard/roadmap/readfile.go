package roadmap

import (
	"errors"
	"io"
	"os"
	"syscall"
)

// maxInputFileBytes bounds every file this join reads: REQUIREMENTS.tsv, a
// receipt, and the docs-state file. The generated requirements table is a
// few hundred KiB; anything past this bound is refused, not truncated.
const maxInputFileBytes = 8 << 20

var (
	errInputNotRegular = errors.New("not a regular file")
	errInputTooLarge   = errors.New("exceeds the input size limit")
)

// inputOpenFlags opens an input read-only without blocking on a FIFO or
// device before readRegularBounded can refuse it.
const inputOpenFlags = os.O_RDONLY | syscall.O_NONBLOCK

// readRegularBounded reads all of file after confirming, on the open
// descriptor, that it is a regular file no larger than maxInputFileBytes.
func readRegularBounded(file *os.File) ([]byte, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errInputNotRegular
	}
	if info.Size() > maxInputFileBytes {
		return nil, errInputTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(file, maxInputFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxInputFileBytes {
		return nil, errInputTooLarge
	}
	return data, nil
}
