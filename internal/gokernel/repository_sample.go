package gokernel

import (
	"strconv"
	"strings"
)

// DirtySampleSize bounds the dirty paths a refusal may name (DCW-V0-397).
const DirtySampleSize = 5

// dirtySample is Repository.DirtySample: the first sorted dirty paths, at
// most DirtySampleSize, each Go-quoted and space-separated, so a refusal can
// name them without raw bytes. A string keeps Repository comparable, and the
// sample is derived from the same list DirtyPathsSHA hashes; it is not on the
// wire.
func dirtySample(paths []string) string {
	named := make([]string, 0, DirtySampleSize)
	for _, path := range paths[:min(len(paths), DirtySampleSize)] {
		named = append(named, strconv.Quote(path))
	}
	return strings.Join(named, " ")
}
