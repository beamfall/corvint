// Package patch implements the frozen CEM 0.1 bounded unified-diff parser
// specified by interop/cem-0.1/ALGORITHMS.md.
package patch

import (
	"bytes"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// Frozen bounds.
const (
	MaxPatchBytes = 8 << 20
	MaxLFRecords  = 262144
)

// SplitLF applies the frozen LF tokenizer: the record count is the LF count
// plus one exactly when the non-empty input does not end in LF; inputs above
// MaxLFRecords records are rejected before splitting. Records retain their LF.
func SplitLF(data []byte) ([][]byte, error) {
	records := bytes.Count(data, []byte{'\n'})
	if len(data) > 0 && data[len(data)-1] != '\n' {
		records++
	}
	if records > MaxLFRecords {
		return nil, cemcode.New(cemcode.TooManyLines, "input exceeds %d LF-tokenizer records", MaxLFRecords)
	}
	var out [][]byte
	start := 0
	for index := 0; index < len(data); index++ {
		if data[index] == '\n' {
			out = append(out, data[start:index+1])
			start = index + 1
		}
	}
	if start < len(data) {
		out = append(out, data[start:])
	}
	return out, nil
}
