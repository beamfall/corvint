package gitauth

import (
	"io"
	"os"
)

// readBoundedMetadata reads one regular metadata file with the shared bounded
// no-follow reader: size is checked before open, the opened descriptor is
// revalidated against the pre-open identity, at most bound+1 bytes are read,
// and truncation, growth, or observed metadata change all reject.
func readBoundedMetadata(path string, bound int) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, unavailable("metadata file %q is missing", path)
	}
	if !before.Mode().IsRegular() {
		return nil, unavailable("metadata file %q is not a regular file", path)
	}
	if before.Size() > int64(bound) {
		return nil, unavailable("metadata file %q exceeds its byte bound", path)
	}
	file, err := openNoFollow(path)
	if err != nil {
		return nil, unavailable("metadata file %q cannot be opened without following links", path)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) || opened.Size() != before.Size() {
		return nil, unavailable("metadata file %q changed while being read", path)
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(bound)+1))
	if err != nil || int64(len(data)) != before.Size() {
		return nil, unavailable("metadata file %q was truncated or grew while being read", path)
	}
	final, err := file.Stat()
	if err != nil || final.Size() != before.Size() || !os.SameFile(before, final) {
		return nil, unavailable("metadata file %q changed while being read", path)
	}
	return data, nil
}
