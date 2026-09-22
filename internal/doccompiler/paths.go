package doccompiler

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func projectRoot(path string) (string, error) {
	if path == "" {
		return "", failure("invalid-project-root", "project root is required")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", failure("invalid-project-root", "cannot resolve project root")
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", failure("invalid-project-root", "cannot resolve project root")
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", failure("invalid-project-root", "project root is not a directory")
	}
	return filepath.Clean(resolved), nil
}

func cleanRelative(path string) (string, error) {
	if path == "" {
		return "", failure("invalid-path", "empty path")
	}
	if filepath.IsAbs(path) {
		return "", failure("path-escape", "absolute path is not allowed")
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if clean == "." || clean == ".." {
		return "", failure("path-escape", "path must name a contained file")
	}
	if strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", failure("path-escape", "path escapes the project")
	}
	return filepath.ToSlash(clean), nil
}

func relativeFromRoot(root, path string) (string, error) {
	absolute := path
	if !filepath.IsAbs(absolute) {
		absolute = filepath.Join(root, filepath.FromSlash(path))
	}
	relative, err := filepath.Rel(root, filepath.Clean(absolute))
	if err != nil {
		return "", failure("path-escape", "cannot resolve path")
	}
	return cleanRelative(filepath.ToSlash(relative))
}

func contained(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func rejectSymlinkComponents(root, relative string) error {
	clean, err := cleanRelative(relative)
	if err != nil {
		return err
	}
	current := root
	for _, component := range strings.Split(clean, "/") {
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		if statErr != nil {
			return failure("file-unavailable", "cannot inspect %s", clean)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return failure("symlink-rejected", "%s traverses a symlink", clean)
		}
	}
	return nil
}

func rejectExistingSymlinkComponents(root, relative string) error {
	clean, err := cleanRelative(relative)
	if err != nil {
		return err
	}
	current := root
	for _, component := range strings.Split(clean, "/") {
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		if os.IsNotExist(statErr) {
			return nil
		}
		if statErr != nil {
			return failure("file-unavailable", "cannot inspect %s", clean)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return failure("symlink-rejected", "%s traverses a symlink", clean)
		}
	}
	return nil
}

func readPinned(root, relative string, limit int64) ([]byte, SourcePin, error) {
	clean, err := cleanRelative(relative)
	if err != nil {
		return nil, SourcePin{}, err
	}
	if err := rejectSymlinkComponents(root, clean); err != nil {
		return nil, SourcePin{}, err
	}
	file, err := os.Open(filepath.Join(root, filepath.FromSlash(clean)))
	if err != nil {
		return nil, SourcePin{}, failure("file-unavailable", "cannot open %s", clean)
	}
	defer file.Close()
	before, err := file.Stat()
	if err != nil || !before.Mode().IsRegular() {
		return nil, SourcePin{}, failure("invalid-file", "%s is not a regular file", clean)
	}
	if before.Size() < 0 || before.Size() > limit {
		return nil, SourcePin{}, failure("file-too-large", "%s exceeds its byte limit", clean)
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(data)) != before.Size() {
		return nil, SourcePin{}, failure("stale-source", "%s changed while it was read", clean)
	}
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) || before.Size() != after.Size() || before.ModTime() != after.ModTime() {
		return nil, SourcePin{}, failure("stale-source", "%s changed while it was read", clean)
	}
	digest := sha256.Sum256(data)
	return data, SourcePin{Path: clean, SHA256: hex.EncodeToString(digest[:]), Size: int64(len(data))}, nil
}

func pinExecutable(root, path string, allowResolvedOutside bool) (FilePin, string, error) {
	relative, err := relativeFromRoot(root, path)
	if err != nil {
		return FilePin{}, "", err
	}
	logical := filepath.Join(root, filepath.FromSlash(relative))
	info, err := os.Lstat(logical)
	if err != nil {
		return FilePin{}, "", failure("tool-unavailable", "cannot inspect %s", relative)
	}
	if info.Mode()&0111 == 0 {
		return FilePin{}, "", failure("tool-not-executable", "%s is not executable", relative)
	}
	resolved, err := filepath.EvalSymlinks(logical)
	if err != nil {
		return FilePin{}, "", failure("tool-unavailable", "cannot resolve %s", relative)
	}
	if !allowResolvedOutside && !contained(root, resolved) {
		return FilePin{}, "", failure("path-escape", "%s resolves outside the project", relative)
	}
	target, err := os.Open(resolved)
	if err != nil {
		return FilePin{}, "", failure("tool-unavailable", "cannot open %s", relative)
	}
	defer target.Close()
	targetInfo, err := target.Stat()
	if err != nil || !targetInfo.Mode().IsRegular() {
		return FilePin{}, "", failure("invalid-tool", "%s does not resolve to a regular file", relative)
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, io.LimitReader(target, defaultSourceBytes+1)); err != nil {
		return FilePin{}, "", failure("tool-unavailable", "cannot hash %s", relative)
	}
	if targetInfo.Size() > defaultSourceBytes {
		return FilePin{}, "", failure("tool-too-large", "%s exceeds its byte limit", relative)
	}
	resolvedLabel := ""
	if resolved != logical {
		resolvedLabel = filepath.Clean(resolved)
	}
	return FilePin{Path: relative, ResolvedPath: resolvedLabel, SHA256: hex.EncodeToString(hash.Sum(nil)), Size: targetInfo.Size()}, logical, nil
}

func verifyExecutable(root string, expected FilePin, allowResolvedOutside bool) (string, error) {
	actual, absolute, err := pinExecutable(root, expected.Path, allowResolvedOutside)
	if err != nil {
		return "", err
	}
	if actual != expected {
		return "", failure("stale-toolchain", "%s no longer matches its pin", expected.Path)
	}
	return absolute, nil
}
