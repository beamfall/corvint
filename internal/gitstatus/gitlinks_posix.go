//go:build darwin || linux

package gitstatus

import (
	"os"
	"path/filepath"
	"strings"
)

func snapshotGitlink(root, shadow, name string, reader *metadataReader, files *[]capturedFile) error {
	source := filepath.Join(root, filepath.FromSlash(name))
	destination := filepath.Join(shadow, filepath.FromSlash(name))
	// Pin every real parent before inspecting the entry, including empty and
	// missing checkouts. A replaced parent invalidates the observation bracket.
	parent, err := reader.directory(filepath.Dir(source))
	if os.IsNotExist(err) {
		*files = append(*files, capturedFile{path: source})
		return nil
	}
	if err != nil {
		return err
	}
	info, err := parent.Lstat(filepath.Base(source))
	if os.IsNotExist(err) {
		*files = append(*files, capturedFile{path: source})
		return nil
	}
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		return err
	}
	if info.Mode().IsRegular() {
		return os.WriteFile(destination, nil, info.Mode().Perm())
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return os.Symlink("opaque-target", destination)
	}
	if !info.IsDir() {
		return errGitlink
	}
	if err := os.Mkdir(destination, 0700); err != nil {
		return err
	}
	directory, err := reader.directory(source)
	if err != nil {
		return err
	}
	marker, err := directory.Lstat(".git")
	if os.IsNotExist(err) {
		*files = append(*files, capturedFile{path: filepath.Join(source, ".git")})
		return nil
	}
	if err != nil {
		return err
	}
	admin := filepath.Join(source, ".git")
	if marker.Mode().IsRegular() {
		pointer, err := captureGitlink(reader, files, admin, 4096)
		if err != nil {
			return err
		}
		target, ok := strings.CutPrefix(strings.TrimRight(string(pointer), "\r\n"), "gitdir: ")
		if !ok || target == "" || strings.ContainsAny(target, "\r\n\x00") {
			return errGitlink
		}
		admin = target
		if !filepath.IsAbs(admin) {
			admin = filepath.Join(source, admin)
		}
	} else if !marker.IsDir() {
		return errGitlink
	}
	oid, err := gitlinkHEAD(reader, files, admin)
	if err != nil {
		return err
	}
	private := filepath.Join(destination, ".git")
	for _, dir := range []string{"objects", "refs"} {
		if err := os.MkdirAll(filepath.Join(private, dir), 0700); err != nil {
			return err
		}
	}
	config := ""
	if len(oid) == 64 {
		config = "[core]\nrepositoryformatversion = 1\n[extensions]\nobjectformat = sha256\n"
	}
	if err := os.WriteFile(filepath.Join(private, "config"), []byte(config), 0600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(private, "HEAD"), []byte(oid+"\n"), 0600)
}
