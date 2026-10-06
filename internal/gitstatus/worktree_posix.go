//go:build darwin || linux

package gitstatus

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"os"
	"path"
	"slices"
	"strings"
)

// blocking names the file types whose open can wait on another process.
const blocking = os.ModeNamedPipe | os.ModeDevice | os.ModeCharDevice

// worktreeInputsOpen refuses an ignore or attributes file that Git would open
// without O_NONBLOCK and could wait on, such as a FIFO .gitignore, instead of
// letting status run until the caller's deadline (V1-0388). It inspects the
// root and the directory of every index entry. An untracked directory is not
// listed, so a file there stays bounded only by that deadline. A symlink is
// admitted: Git opens in-tree ignore and attributes files without following it.
func worktreeInputsOpen(ctx context.Context, root string, index []byte) error {
	worktree, err := os.OpenRoot(root)
	if err != nil {
		return errRoot
	}
	defer worktree.Close()
	inputs := worktreeInputs(index)
	modes, err := worktreeInputModes(ctx, worktree, inputs)
	if err != nil {
		return err
	}
	for _, name := range inputs {
		if mode, ok := modes[name]; ok && mode&blocking != 0 {
			return unsupported(classMetadataUnreadable, "worktree file "+path.Base(name)+" "+irregular(mode))
		}
	}
	return nil
}

// worktreeInputModes reports the Lstat mode of every input that exists, as
// worktree.Lstat(input) would. A Root Lstat reopens every directory on the
// path, so it opens each listed directory once, from its parent's handle, and
// examines its two inputs there. A directory that cannot be opened that way,
// such as a symlink that leaves its parent, falls back to worktree.Lstat for
// itself and its descendants, so symlink resolution stays the root's. Open
// handles are bounded by the directory depth.
func worktreeInputModes(ctx context.Context, worktree *os.Root, inputs []string) (map[string]os.FileMode, error) {
	modes := make(map[string]os.FileMode, len(inputs))
	type frame struct {
		directory string
		handle    *os.Root // nil: resolve this subtree from the worktree
	}
	stack := []frame{{".", worktree}}
	defer func() {
		for _, open := range stack[1:] {
			if open.handle != nil {
				open.handle.Close()
			}
		}
	}()
	for _, pair := range inputPairs(inputs) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for len(stack) > 1 && !strings.HasPrefix(pair.directory, stack[len(stack)-1].directory+"/") {
			if top := stack[len(stack)-1].handle; top != nil {
				top.Close()
			}
			stack = stack[:len(stack)-1]
		}
		parent := stack[len(stack)-1]
		handle := parent.handle
		if pair.directory != "." {
			handle = nil
			component, nested := strings.CutPrefix(pair.directory, parent.directory+"/")
			if parent.directory == "." {
				component, nested = pair.directory, true
			}
			// worktreeInputs lists every ancestor, so the parent is the
			// directory's own; anything else takes the worktree's resolution.
			if parent.handle != nil && nested && component != "" && !strings.Contains(component, "/") {
				if opened, err := parent.handle.OpenRoot(component); err == nil {
					handle = opened
				}
			}
			stack = append(stack, frame{pair.directory, handle})
		}
		for _, input := range inputs[pair.offset : pair.offset+2] {
			var info os.FileInfo
			var err error
			if handle != nil {
				info, err = handle.Lstat(path.Base(input))
			} else {
				info, err = worktree.Lstat(input)
			}
			if err == nil {
				modes[input] = info.Mode()
			}
		}
	}
	return modes, nil
}

// inputPair is one directory's ignore and attributes inputs, at offset and
// offset+1 in worktreeInputs.
type inputPair struct {
	directory string
	offset    int
}

// inputPairs orders worktreeInputs' pairs with the root first and every other
// directory before its descendants, each subtree contiguous.
func inputPairs(inputs []string) []inputPair {
	pairs := make([]inputPair, 0, len(inputs)/2)
	for offset := 0; offset+1 < len(inputs); offset += 2 {
		pairs = append(pairs, inputPair{path.Dir(inputs[offset]), offset})
	}
	key := func(directory string) string {
		if directory == "." {
			return ""
		}
		return "\x01" + strings.ReplaceAll(directory, "/", "\x00")
	}
	slices.SortFunc(pairs, func(left, right inputPair) int {
		return strings.Compare(key(left.directory), key(right.directory))
	})
	return pairs
}

// worktreeInputs lists the ignore and attributes files Git may open in the
// root and in each directory of an index entry. An index that frames with
// neither SHA-1 nor SHA-256 object IDs lists only the root's; Git still
// validates the index itself.
func worktreeInputs(index []byte) []string {
	names, _ := indexNames(index, sha1.Size)
	if names == nil {
		names, _ = indexNames(index, sha256.Size)
	}
	directories := map[string]bool{".": true}
	inputs := []string{".gitignore", ".gitattributes"}
	for _, name := range names {
		for directory := path.Dir(name); !directories[directory]; directory = path.Dir(directory) {
			directories[directory] = true
			inputs = append(inputs, directory+"/.gitignore", directory+"/.gitattributes")
		}
	}
	return inputs
}

// indexNames decodes the entry paths of a v2, v3 or v4 index whose object IDs
// are size bytes, or reports false when any entry does not frame.
func indexNames(index []byte, size int) ([]string, bool) {
	if len(index) < 12 || string(index[:4]) != "DIRC" {
		return nil, false
	}
	version := binary.BigEndian.Uint32(index[4:8])
	if version < 2 || version > 4 {
		return nil, false
	}
	remaining := index[12:]
	names := []string{}
	previous := ""
	for count := binary.BigEndian.Uint32(index[8:12]); count > 0; count-- {
		name, length := indexEntry(remaining, version, size, previous)
		if length == 0 {
			return nil, false
		}
		names = append(names, name)
		previous = name
		remaining = remaining[length:]
	}
	return names, true
}

// indexEntry returns one entry's path and on-disk length, or zero length when
// the entry does not frame. The name length in the flags must match the path.
func indexEntry(data []byte, version uint32, size int, previous string) (string, int) {
	fixed := 40 + size
	if len(data) < fixed+2 {
		return "", 0
	}
	flags := binary.BigEndian.Uint16(data[fixed : fixed+2])
	start := fixed + 2
	if flags&0x4000 != 0 {
		start += 2
	}
	if len(data) < start {
		return "", 0
	}
	var name string
	var length int
	if version == 4 {
		name, length = prefixedName(data, start, previous)
	} else {
		name, length = paddedName(data, start)
	}
	if length == 0 || int(flags&0xfff) != min(len(name), 0xfff) {
		return "", 0
	}
	return name, length
}

// paddedName reads a v2 or v3 path: NUL-terminated and NUL-padded so the entry
// ends on an 8-byte boundary.
func paddedName(data []byte, start int) (string, int) {
	length := bytes.IndexByte(data[start:], 0)
	end := (start + length + 8) &^ 7
	if length < 1 || end > len(data) {
		return "", 0
	}
	return string(data[start : start+length]), end
}

// prefixedName reads a v4 path: a varint count of bytes removed from the end
// of the previous path, then a NUL-terminated suffix, with no padding.
func prefixedName(data []byte, start int, previous string) (string, int) {
	strip, width := varint(data[start:])
	if width == 0 || strip > uint64(len(previous)) {
		return "", 0
	}
	suffix := data[start+width:]
	length := bytes.IndexByte(suffix, 0)
	if length < 0 {
		return "", 0
	}
	name := previous[:len(previous)-int(strip)] + string(suffix[:length])
	if name == "" {
		return "", 0
	}
	return name, start + width + length + 1
}

// varint decodes Git's offset varint, in which each continuation byte adds one
// before the shift, and returns its value and width, or zero width.
func varint(data []byte) (uint64, int) {
	var value uint64
	for index, octet := range data[:min(len(data), 9)] {
		value = value<<7 + uint64(octet&127)
		if octet&128 == 0 {
			return value, index + 1
		}
		value++
	}
	return 0, 0
}
