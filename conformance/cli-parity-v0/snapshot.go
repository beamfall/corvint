package main

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type snapshotEntry struct {
	Path       string `json:"path"`
	Type       string `json:"type"`
	Mode       uint32 `json:"mode"`
	Content    []byte `json:"content,omitempty"`
	LinkTarget string `json:"linkTarget,omitempty"`
}

type repositorySnapshot struct {
	Entries          []snapshotEntry
	Status           []byte
	SHA256           string
	RepositorySHA256 string
	FileModesSHA256  string
	StatusSHA256     string
}

func snapshotRepository(ctx context.Context, root string) (repositorySnapshot, error) {
	git, err := newSanitizedGit(ctx, root)
	if err != nil {
		return repositorySnapshot{}, err
	}
	gitDir, commonDir, err := snapshotGitDirectories(ctx, git)
	if err != nil {
		return repositorySnapshot{}, err
	}
	status, err := git.run(ctx, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return repositorySnapshot{}, err
	}
	entries, err := snapshotTree(root, "worktree", []string{gitDir, commonDir})
	if err != nil {
		return repositorySnapshot{}, err
	}
	gitEntries, err := snapshotTree(gitDir, "git", nil)
	if err != nil {
		return repositorySnapshot{}, err
	}
	entries = append(entries, gitEntries...)
	if !samePath(gitDir, commonDir) {
		commonEntries, err := snapshotTree(commonDir, "common", nil)
		if err != nil {
			return repositorySnapshot{}, err
		}
		entries = append(entries, commonEntries...)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	payload, err := json.Marshal(struct {
		Entries []snapshotEntry `json:"entries"`
		Status  []byte          `json:"status"`
	}{entries, status})
	if err != nil {
		return repositorySnapshot{}, err
	}
	repositorySHA256, fileModesSHA256, err := snapshotComponentDigests(entries)
	if err != nil {
		return repositorySnapshot{}, err
	}
	return repositorySnapshot{
		Entries: entries, Status: status, SHA256: digest(payload), RepositorySHA256: repositorySHA256,
		FileModesSHA256: fileModesSHA256, StatusSHA256: digest(status),
	}, nil
}

func snapshotGitDirectories(ctx context.Context, git sanitizedGit) (string, string, error) {
	gitDirectory := filepath.Join(git.dir, ".git")
	if info, err := os.Lstat(gitDirectory); err == nil && info.IsDir() {
		if physicalDirectory, err := filepath.EvalSymlinks(gitDirectory); err == nil {
			return physicalDirectory, physicalDirectory, nil
		}
	}
	if strings.Contains(git.dir, "\n") {
		return snapshotLegacyGitDirectories(ctx, git)
	}
	output, err := git.run(ctx, "rev-parse", "--path-format=absolute", "--git-dir", "--git-common-dir")
	if err == nil {
		lines := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
		if len(lines) == 2 {
			gitDir, commonDir := strings.TrimSpace(lines[0]), strings.TrimSpace(lines[1])
			if filepath.IsAbs(gitDir) && filepath.IsAbs(commonDir) {
				return gitDir, commonDir, nil
			}
		}
	}
	// Newlines in paths or a failed combined lookup retain the original
	// per-value trimming and error precedence through the bounded adapter.
	return snapshotLegacyGitDirectories(ctx, git)
}

func snapshotLegacyGitDirectories(ctx context.Context, git sanitizedGit) (string, string, error) {
	gitDir, err := git.string(ctx, "rev-parse", "--path-format=absolute", "--git-dir")
	if err != nil {
		return "", "", err
	}
	commonDir, err := git.string(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", "", err
	}
	return gitDir, commonDir, nil
}

func snapshotComponentDigests(entries []snapshotEntry) (string, string, error) {
	type repositoryEntry struct {
		Path       string `json:"path"`
		Type       string `json:"type"`
		Content    []byte `json:"content,omitempty"`
		LinkTarget string `json:"linkTarget,omitempty"`
	}
	type modeEntry struct {
		Path string `json:"path"`
		Type string `json:"type"`
		Mode uint32 `json:"mode"`
	}
	repository := make([]repositoryEntry, 0, len(entries))
	modes := make([]modeEntry, 0, len(entries))
	for _, entry := range entries {
		repository = append(repository, repositoryEntry{entry.Path, entry.Type, entry.Content, entry.LinkTarget})
		modes = append(modes, modeEntry{entry.Path, entry.Type, entry.Mode})
	}
	repositoryJSON, err := json.Marshal(repository)
	if err != nil {
		return "", "", err
	}
	modesJSON, err := json.Marshal(modes)
	if err != nil {
		return "", "", err
	}
	return digest(repositoryJSON), digest(modesJSON), nil
}

func snapshotTree(root, namespace string, excluded []string) ([]snapshotEntry, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	var entries []snapshotEntry
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if samePath(path, root) {
			return nil
		}
		for _, skip := range excluded {
			if samePath(path, skip) {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		item := snapshotEntry{Path: namespace + "/" + filepath.ToSlash(relative), Mode: portableMode(info.Mode())}
		switch {
		case info.Mode().IsRegular():
			item.Type = "file"
			item.Content, err = os.ReadFile(path)
		case info.IsDir():
			item.Type = "directory"
		case info.Mode()&os.ModeSymlink != 0:
			item.Type = "symlink"
			item.LinkTarget, err = os.Readlink(path)
		default:
			return fmt.Errorf("snapshot %q: unsupported file mode %s", path, info.Mode())
		}
		if err != nil {
			return err
		}
		entries = append(entries, item)
		return nil
	})
	return entries, err
}

func portableMode(mode fs.FileMode) uint32 {
	result := uint32(mode.Perm())
	if mode&os.ModeSetuid != 0 {
		result |= 0o4000
	}
	if mode&os.ModeSetgid != 0 {
		result |= 0o2000
	}
	if mode&os.ModeSticky != 0 {
		result |= 0o1000
	}
	return result
}

func samePath(left, right string) bool {
	left, leftErr := filepath.Abs(left)
	right, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && left == right
}

func extractSafeTar(archive []byte, destination string) error {
	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(header.Name, "/")
		if name == "" {
			continue
		}
		if err := validateRelativePath(name); err != nil {
			return fmt.Errorf("archive path %q: %w", header.Name, err)
		}
		path := filepath.Join(destination, filepath.FromSlash(name))
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, fs.FileMode(header.Mode)&os.ModePerm); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fs.FileMode(header.Mode)&os.ModePerm)
			if err != nil {
				return err
			}
			_, copyErr := io.CopyN(file, reader, header.Size)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		case tar.TypeSymlink:
			if err := validateSymlinkTarget(name, header.Linkname); err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(filepath.FromSlash(header.Linkname), path); err != nil {
				return err
			}
		case tar.TypeXGlobalHeader, tar.TypeXHeader:
			continue
		default:
			return fmt.Errorf("archive path %q has unsupported type %d", header.Name, header.Typeflag)
		}
	}
}

func makeTreeReadOnly(root string) error {
	var directories []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if info.IsDir() {
			directories = append(directories, path)
			return nil
		}
		return os.Chmod(path, info.Mode().Perm()&^0o222)
	})
	if err != nil {
		return err
	}
	sort.Slice(directories, func(i, j int) bool { return len(directories[i]) > len(directories[j]) })
	for _, directory := range directories {
		info, err := os.Stat(directory)
		if err != nil {
			return err
		}
		if err := os.Chmod(directory, info.Mode().Perm()&^0o222); err != nil {
			return err
		}
	}
	return nil
}

func makeTreeWritable(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		mode := info.Mode().Perm() | 0o200
		if entry.IsDir() {
			mode |= 0o700
		}
		return os.Chmod(path, mode)
	})
}

func digestTree(root string) (string, error) {
	entries, err := snapshotTree(root, "source", nil)
	if err != nil {
		return "", err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	payload, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	return digest(payload), nil
}

func fullGitSHA1(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			if character < 'a' || character > 'f' {
				return false
			}
		}
	}
	return true
}
