// SPDX-License-Identifier: Apache-2.0
package linecov

import "os"

// FileByPath returns the indexed file. Callers must treat index storage as
// immutable. The same lookup is used by Go and language plugins (LPCV-V0-7602).
func (ix *Index) FileByPath(path string) (*File, bool) {
	i, ok := ix.byPath[path]
	if !ok {
		return nil, false
	}
	return &ix.Files[i], true
}

// BlocksAtLine returns all blocks intersecting line, including overlapping
// method and line blocks. The returned slice is independent of index storage.
func (ix *Index) BlocksAtLine(path string, line int32) []Block {
	f, ok := ix.FileByPath(path)
	if !ok {
		return nil
	}
	var out []Block
	for _, b := range f.Blocks {
		if b.Start > line {
			break
		}
		if b.End >= line {
			b.Tests = append([]int32(nil), b.Tests...)
			b.Segs = append([]int32(nil), b.Segs...)
			out = append(out, b)
		}
	}
	return out
}

// TestsOfFile returns each test that reached any block of path, in index order.
func (ix *Index) TestsOfFile(path string) []Test {
	f, ok := ix.FileByPath(path)
	if !ok {
		return nil
	}
	seen := map[int32]bool{}
	for _, b := range f.Blocks {
		for _, id := range b.Tests {
			seen[id] = true
		}
	}
	var out []Test
	for i, t := range ix.Tests {
		if seen[int32(i)] {
			out = append(out, t)
		}
	}
	return out
}

// Load reads and validates an immutable index from disk.
func Load(path string) (*Index, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Decode(f)
}
