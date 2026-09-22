package toolchainreceipt

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const domain = "corvint-go-toolchain-tree/v1"

const stableModeMask = os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky

type Result struct {
	Entries int
	SHA256  string
}

type entry struct {
	path string
	info os.FileInfo
}

func Tree(root string) (Result, error) {
	root = filepath.Clean(root)
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return Result{}, err
	}
	if !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return Result{}, errors.New("toolchain root must be a real directory")
	}

	entries, err := collect(root)
	if err != nil {
		return Result{}, err
	}
	if err := unchanged(root, rootInfo); err != nil {
		return Result{}, err
	}
	sort.Slice(entries, func(left, right int) bool { return entries[left].path < entries[right].path })
	if uint64(len(entries)) > uint64(^uint32(0)) {
		return Result{}, errors.New("too many toolchain entries")
	}

	hasher := sha256.New()
	_, _ = io.WriteString(hasher, domain)
	writeUint32(hasher, stableMode(rootInfo.Mode()))
	writeUint32(hasher, uint32(len(entries)))
	for _, current := range entries {
		if err := hashEntry(hasher, root, current); err != nil {
			return Result{}, err
		}
	}
	if err := unchanged(root, rootInfo); err != nil {
		return Result{}, err
	}
	return Result{Entries: len(entries), SHA256: hex.EncodeToString(hasher.Sum(nil))}, nil
}

func collect(root string) ([]entry, error) {
	var entries []entry
	err := filepath.WalkDir(root, func(fullPath string, item os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if fullPath == root {
			return nil
		}
		relative, err := filepath.Rel(root, fullPath)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if relative == "." || strings.HasPrefix(relative, "../") || !utf8.ValidString(relative) || strings.ContainsRune(relative, 0) {
			return fmt.Errorf("invalid toolchain path %q", relative)
		}
		info, err := os.Lstat(fullPath)
		if err != nil {
			return err
		}
		mode := info.Mode()
		if !mode.IsDir() && !mode.IsRegular() && mode&os.ModeSymlink == 0 {
			return fmt.Errorf("unsupported toolchain entry %q", relative)
		}
		entries = append(entries, entry{path: relative, info: info})
		return nil
	})
	return entries, err
}

func hashEntry(hasher hash.Hash, root string, current entry) error {
	if uint64(len(current.path)) > uint64(^uint32(0)) {
		return errors.New("toolchain path too long")
	}
	writeField(hasher, current.path)
	writeUint32(hasher, stableMode(current.info.Mode()))
	fullPath := filepath.Join(root, filepath.FromSlash(current.path))

	switch mode := current.info.Mode(); {
	case mode.IsDir():
		_, _ = hasher.Write([]byte{'D'})
		return unchanged(fullPath, current.info)
	case mode&os.ModeSymlink != 0:
		_, _ = hasher.Write([]byte{'L'})
		target, err := os.Readlink(fullPath)
		if err != nil {
			return err
		}
		if uint64(len(target)) > uint64(^uint32(0)) {
			return errors.New("toolchain link target too long")
		}
		writeField(hasher, target)
		return unchanged(fullPath, current.info)
	case mode.IsRegular():
		_, _ = hasher.Write([]byte{'F'})
		writeUint64(hasher, uint64(current.info.Size()))
		file, err := os.Open(fullPath)
		if err != nil {
			return err
		}
		written, copyErr := io.Copy(hasher, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if written != current.info.Size() {
			return fmt.Errorf("toolchain file changed while hashing: %s", current.path)
		}
		return unchanged(fullPath, current.info)
	default:
		return fmt.Errorf("unsupported toolchain entry %q", current.path)
	}
}

func stableMode(mode os.FileMode) uint32 {
	return uint32(mode & stableModeMask)
}

func unchanged(path string, before os.FileInfo) error {
	after, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return fmt.Errorf("toolchain entry changed while hashing: %s", path)
	}
	return nil
}

func writeField(hasher hash.Hash, value string) {
	writeUint32(hasher, uint32(len(value)))
	_, _ = io.WriteString(hasher, value)
}

func writeUint32(hasher hash.Hash, value uint32) {
	var encoded [4]byte
	binary.BigEndian.PutUint32(encoded[:], value)
	_, _ = hasher.Write(encoded[:])
}

func writeUint64(hasher hash.Hash, value uint64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	_, _ = hasher.Write(encoded[:])
}
