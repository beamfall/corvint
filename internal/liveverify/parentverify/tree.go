package parentverify

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/godiscovery"
)

type treeRow struct {
	Kind      string `json:"kind"`
	Mode      string `json:"mode"`
	Path      string `json:"path"`
	RawSHA256 string `json:"rawSha256"`
}

type inputRow struct {
	Mode       string `json:"mode"`
	PathSHA256 string `json:"pathSha256"`
	RawSHA256  string `json:"rawSha256"`
	logical    string
}

type treeSnapshot struct {
	digest         string
	metadataDigest string
	inputs         []inputRow
}

type metadataRow struct {
	Kind      string `json:"kind"`
	Mode      string `json:"mode"`
	Modified  string `json:"modified"`
	Path      string `json:"path"`
	Size      string `json:"size"`
	TargetSHA string `json:"targetSha256"`
}

func stableTree(ctx context.Context, path string, namespace godiscovery.PathNamespace, prefix string, requireReadOnly bool) (treeSnapshot, error) {
	excludeGitMetadata := namespace == godiscovery.NamespaceSource && prefix == "."
	first, err := scanTree(ctx, path, namespace, prefix, requireReadOnly)
	if err != nil {
		return treeSnapshot{}, err
	}
	second, err := scanTreeMetadata(ctx, path, requireReadOnly, excludeGitMetadata)
	if err != nil {
		return treeSnapshot{}, err
	}
	if first.metadataDigest != second {
		return treeSnapshot{}, ErrDrift
	}
	return first, nil
}

func scanTree(ctx context.Context, path string, namespace godiscovery.PathNamespace, prefix string, requireReadOnly bool) (treeSnapshot, error) {
	if ctx == nil || !cleanAbsolute(path) || !validLogicalPrefix(prefix) {
		return treeSnapshot{}, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return treeSnapshot{}, errors.Join(ErrUnavailable, err)
	}
	before, err := os.Lstat(path)
	if err != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return treeSnapshot{}, ErrUnavailable
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return treeSnapshot{}, ErrUnavailable
	}
	defer root.Close()
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(before, opened) || opened.Mode() != before.Mode() {
		return treeSnapshot{}, ErrDrift
	}
	if requireReadOnly && opened.Mode().Perm()&0o222 != 0 {
		return treeSnapshot{}, ErrUnavailable
	}
	rows := make([]treeRow, 0, 256)
	metadata := make([]metadataRow, 0, 256)
	inputs := make([]inputRow, 0, 256)
	var entries, bytesRead uint64
	var walk func(*os.Root, string) error
	walk = func(directoryRoot *os.Root, directory string) error {
		if err := ctx.Err(); err != nil {
			return errors.Join(ErrUnavailable, err)
		}
		values, err := fs.ReadDir(directoryRoot.FS(), ".")
		if err != nil {
			return ErrUnavailable
		}
		sort.Slice(values, func(i, j int) bool { return values[i].Name() < values[j].Name() })
		for _, entry := range values {
			if err := ctx.Err(); err != nil {
				return errors.Join(ErrUnavailable, err)
			}
			if namespace == godiscovery.NamespaceSource && prefix == "." && directory == "." && entry.Name() == ".git" {
				continue
			}
			if err := validComponent(entry.Name()); err != nil {
				return err
			}
			relative := entry.Name()
			if directory != "." {
				relative = filepath.ToSlash(filepath.Join(directory, entry.Name()))
			}
			entries++
			if entries > MaxTreeEntries {
				return ErrLimit
			}
			info, err := directoryRoot.Lstat(entry.Name())
			if err != nil {
				return ErrDrift
			}
			mode := fmt.Sprintf("%04o", info.Mode().Perm())
			if requireReadOnly && info.Mode().Perm()&0o222 != 0 {
				return ErrUnavailable
			}
			switch {
			case info.IsDir():
				rows = append(rows, treeRow{Kind: "DIRECTORY", Mode: mode, Path: relative, RawSHA256: emptyDigest()})
				metadata = append(metadata, metadataFor("DIRECTORY", relative, info, ""))
				child, err := directoryRoot.OpenRoot(entry.Name())
				if err != nil {
					return ErrDrift
				}
				childOpened, statErr := child.Stat(".")
				if statErr != nil || !os.SameFile(info, childOpened) || info.Mode() != childOpened.Mode() {
					_ = child.Close()
					return ErrDrift
				}
				walkErr := walk(child, relative)
				childAfter, childStatErr := child.Stat(".")
				parentAfter, parentStatErr := directoryRoot.Lstat(entry.Name())
				closeErr := child.Close()
				if walkErr != nil {
					return walkErr
				}
				if childStatErr != nil || parentStatErr != nil || closeErr != nil ||
					!os.SameFile(childOpened, childAfter) || !os.SameFile(childOpened, parentAfter) ||
					childOpened.Mode() != childAfter.Mode() || childOpened.Mode() != parentAfter.Mode() {
					return ErrDrift
				}
			case info.Mode().IsRegular():
				if fileLinkCount(info) != 1 {
					return ErrUnavailable
				}
				digest, count, err := readStableFile(directoryRoot, entry.Name(), info)
				if err != nil {
					return err
				}
				if count > ^uint64(0)-bytesRead || bytesRead+count > MaxTreeBytes {
					return ErrLimit
				}
				bytesRead += count
				rows = append(rows, treeRow{Kind: "REGULAR", Mode: mode, Path: relative, RawSHA256: digest})
				metadata = append(metadata, metadataFor("REGULAR", relative, info, ""))
				logical := relative
				if prefix != "." {
					logical = prefix + "/" + relative
				}
				pathDigest, err := logicalPathDigest(namespace, logical)
				if err != nil {
					return err
				}
				inputs = append(inputs, inputRow{Mode: mode, PathSHA256: pathDigest, RawSHA256: digest, logical: logical})
			case info.Mode()&os.ModeSymlink != 0 && !requireReadOnly:
				target, err := directoryRoot.Readlink(entry.Name())
				if err != nil || target == "" || !utf8.ValidString(target) {
					return ErrUnavailable
				}
				afterTarget, err := directoryRoot.Readlink(entry.Name())
				afterInfo, statErr := directoryRoot.Lstat(entry.Name())
				if err != nil || statErr != nil || target != afterTarget || !os.SameFile(info, afterInfo) || info.Mode() != afterInfo.Mode() {
					return ErrDrift
				}
				targetDigest := sha256.Sum256([]byte(target))
				rows = append(rows, treeRow{Kind: "SYMLINK", Mode: mode, Path: relative, RawSHA256: hex.EncodeToString(targetDigest[:])})
				metadata = append(metadata, metadataFor("SYMLINK", relative, info, hex.EncodeToString(targetDigest[:])))
			default:
				return ErrUnavailable
			}
		}
		return nil
	}
	if err := walk(root, "."); err != nil {
		return treeSnapshot{}, err
	}
	after, err := root.Stat(".")
	if err != nil || !os.SameFile(opened, after) || opened.Mode() != after.Mode() {
		return treeSnapshot{}, ErrDrift
	}
	encoded, err := json.Marshal(rows, json.Deterministic(true))
	if err != nil {
		return treeSnapshot{}, ErrUnavailable
	}
	metadataBytes, err := json.Marshal(metadata, json.Deterministic(true))
	if err != nil {
		return treeSnapshot{}, ErrUnavailable
	}
	return treeSnapshot{
		digest:         bareID("go-parent-tree", "go-parent-tree/0", encoded),
		metadataDigest: bareID("go-parent-tree-metadata", "go-parent-tree-metadata/0", metadataBytes),
		inputs:         inputs,
	}, nil
}

func scanTreeMetadata(ctx context.Context, path string, requireReadOnly, excludeGitMetadata bool) (string, error) {
	if ctx == nil {
		return "", ErrInvalid
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return "", ErrUnavailable
	}
	defer root.Close()
	rows := make([]metadataRow, 0, 256)
	var entries uint64
	var walk func(*os.Root, string) error
	walk = func(directoryRoot *os.Root, directory string) error {
		if err := ctx.Err(); err != nil {
			return errors.Join(ErrUnavailable, err)
		}
		values, err := fs.ReadDir(directoryRoot.FS(), ".")
		if err != nil {
			return ErrUnavailable
		}
		sort.Slice(values, func(i, j int) bool { return values[i].Name() < values[j].Name() })
		for _, entry := range values {
			if err := ctx.Err(); err != nil {
				return errors.Join(ErrUnavailable, err)
			}
			if excludeGitMetadata && directory == "." && entry.Name() == ".git" {
				continue
			}
			relative := entry.Name()
			if directory != "." {
				relative = filepath.ToSlash(filepath.Join(directory, entry.Name()))
			}
			entries++
			if entries > MaxTreeEntries {
				return ErrLimit
			}
			info, err := directoryRoot.Lstat(entry.Name())
			if err != nil || requireReadOnly && info.Mode().Perm()&0o222 != 0 {
				return ErrDrift
			}
			switch {
			case info.IsDir():
				rows = append(rows, metadataFor("DIRECTORY", relative, info, ""))
				child, err := directoryRoot.OpenRoot(entry.Name())
				if err != nil {
					return ErrDrift
				}
				childOpened, statErr := child.Stat(".")
				if statErr != nil || !os.SameFile(info, childOpened) || info.Mode() != childOpened.Mode() {
					_ = child.Close()
					return ErrDrift
				}
				walkErr := walk(child, relative)
				childAfter, childStatErr := child.Stat(".")
				parentAfter, parentStatErr := directoryRoot.Lstat(entry.Name())
				closeErr := child.Close()
				if walkErr != nil {
					return walkErr
				}
				if childStatErr != nil || parentStatErr != nil || closeErr != nil ||
					!os.SameFile(childOpened, childAfter) || !os.SameFile(childOpened, parentAfter) ||
					childOpened.Mode() != childAfter.Mode() || childOpened.Mode() != parentAfter.Mode() {
					return ErrDrift
				}
			case info.Mode().IsRegular():
				if fileLinkCount(info) != 1 {
					return ErrUnavailable
				}
				rows = append(rows, metadataFor("REGULAR", relative, info, ""))
			case info.Mode()&os.ModeSymlink != 0 && !requireReadOnly:
				target, err := directoryRoot.Readlink(entry.Name())
				if err != nil {
					return ErrDrift
				}
				digest := sha256.Sum256([]byte(target))
				rows = append(rows, metadataFor("SYMLINK", relative, info, hex.EncodeToString(digest[:])))
			default:
				return ErrUnavailable
			}
		}
		return nil
	}
	if err := walk(root, "."); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(rows, json.Deterministic(true))
	if err != nil {
		return "", ErrUnavailable
	}
	return bareID("go-parent-tree-metadata", "go-parent-tree-metadata/0", encoded), nil
}

func metadataFor(kind, path string, info os.FileInfo, target string) metadataRow {
	return metadataRow{
		Kind: kind, Mode: info.Mode().String(), Modified: info.ModTime().UTC().Format("2006-01-02T15:04:05.999999999Z"),
		Path: path, Size: fmt.Sprintf("%d", info.Size()), TargetSHA: target,
	}
}

func readStableFile(root *os.Root, relative string, expected os.FileInfo) (string, uint64, error) {
	file, err := root.Open(relative)
	if err != nil {
		return "", 0, ErrUnavailable
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(expected, opened) || opened.Mode() != expected.Mode() || opened.Size() != expected.Size() || !opened.ModTime().Equal(expected.ModTime()) {
		return "", 0, ErrDrift
	}
	digest := sha256.New()
	count, err := io.CopyBuffer(digest, io.LimitReader(file, int64(MaxTreeBytes)+1), make([]byte, 32<<10))
	if err != nil || count < 0 || uint64(count) > MaxTreeBytes {
		return "", 0, ErrLimit
	}
	after, err := file.Stat()
	final, finalErr := root.Lstat(relative)
	if err != nil || finalErr != nil || !os.SameFile(opened, after) || !os.SameFile(opened, final) || opened.Mode() != after.Mode() || opened.Mode() != final.Mode() || opened.Size() != after.Size() || opened.Size() != final.Size() || !opened.ModTime().Equal(after.ModTime()) || !opened.ModTime().Equal(final.ModTime()) {
		return "", 0, ErrDrift
	}
	return hex.EncodeToString(digest.Sum(nil)), uint64(count), nil
}

func inputSetDigest(rows []inputRow) (string, error) {
	return rowSetDigest("go-input-set", "go-input-set/0", rows)
}

func treeWalkDigest(rows []inputRow) (string, error) {
	return rowSetDigest("go-tree-walk", "go-tree-walk/0", rows)
}

func invokedToolSetDigest(rows []inputRow) (string, error) {
	return rowSetDigest("go-invoked-tool-set", "go-invoked-tool-set/0", rows)
}

func rowSetDigest(kind, profile string, rows []inputRow) (string, error) {
	sort.Slice(rows, func(i, j int) bool {
		left, _ := json.Marshal(rows[i], json.Deterministic(true))
		right, _ := json.Marshal(rows[j], json.Deterministic(true))
		return bytes.Compare(left, right) < 0
	})
	for index := range rows {
		if index != 0 && rows[index].Mode == rows[index-1].Mode && rows[index].PathSHA256 == rows[index-1].PathSHA256 && rows[index].RawSHA256 == rows[index-1].RawSHA256 {
			return "", ErrUnavailable
		}
	}
	encoded, err := json.Marshal(rows, json.Deterministic(true))
	if err != nil {
		return "", ErrUnavailable
	}
	return bareID(kind, profile, encoded), nil
}

func logicalPathDigest(namespace godiscovery.PathNamespace, logical string) (string, error) {
	body, err := json.Marshal(map[string]any{"namespace": string(namespace), "path": logical}, json.Deterministic(true))
	if err != nil {
		return "", ErrUnavailable
	}
	return bareID("go-logical-path", "go-logical-path/0", body), nil
}

func emptyDigest() string {
	digest := sha256.Sum256(nil)
	return hex.EncodeToString(digest[:])
}

func cleanAbsolute(path string) bool {
	return path != "" && filepath.IsAbs(path) && filepath.Clean(path) == path && utf8.ValidString(path)
}

func validComponent(name string) error {
	if name == "" || name == "." || name == ".." || !utf8.ValidString(name) {
		return ErrUnavailable
	}
	for _, character := range name {
		if unicode.IsControl(character) || character == '/' || character == '\\' {
			return ErrUnavailable
		}
	}
	return nil
}

func validLogicalPrefix(prefix string) bool {
	if prefix == "." {
		return true
	}
	if prefix == "" || strings.HasPrefix(prefix, "/") || strings.Contains(prefix, "\\") {
		return false
	}
	for _, component := range strings.Split(prefix, "/") {
		if validComponent(component) != nil {
			return false
		}
	}
	return true
}
