//go:build darwin

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/authoritystore"
	"golang.org/x/sys/unix"
)

func evidenceOpenDirectory(parent int, name string) (*os.File, error) {
	fd, err := unix.Openat(parent, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), name), nil
}
func evidenceDirectory(root int, relative string, create bool) (*os.File, error) {
	current, err := evidenceOpenDirectory(root, ".")
	if err != nil {
		return nil, err
	}
	if relative == "." || relative == "" {
		return current, nil
	}
	for _, name := range strings.Split(relative, "/") {
		if name == "" || name == "." || name == ".." {
			current.Close()
			return nil, errors.New("evidence path")
		}
		if create {
			if err := unix.Mkdirat(int(current.Fd()), name, 0700); err != nil && err != unix.EEXIST {
				current.Close()
				return nil, err
			}
		}
		next, err := evidenceOpenDirectory(int(current.Fd()), name)
		current.Close()
		if err != nil {
			return nil, err
		}
		current = next
	}
	return current, nil
}
func evidenceWriteFile(root int, p string, raw []byte, owner, gid uint32) error {
	parent, err := evidenceDirectory(root, path.Dir(p), true)
	if err != nil {
		return err
	}
	defer parent.Close()
	fd, err := unix.Openat(int(parent.Fd()), path.Base(p), unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0600)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), p)
	defer file.Close()
	if _, err = file.Write(raw); err != nil {
		return err
	}
	if err = unix.Fchown(fd, int(owner), int(gid)); err != nil {
		return err
	}
	if err = unix.Fchmod(fd, 0440); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	return parent.Sync()
}
func evidenceDirectories(m authoritystore.EvidenceManifest) []string {
	seen := map[string]bool{".": true, "repo": true, "repo/.git": true, "repo/.git/refs": true, "repo/.git/objects": true}
	for _, f := range m.Files {
		for p := path.Dir("repo/" + f.Path); p != "."; p = path.Dir(p) {
			seen[p] = true
		}
	}
	dirs := make([]string, 0, len(seen))
	for p := range seen {
		dirs = append(dirs, p)
	}
	sort.Slice(dirs, func(i, j int) bool {
		if len(dirs[i]) == len(dirs[j]) {
			return dirs[i] > dirs[j]
		}
		return len(dirs[i]) > len(dirs[j])
	})
	return dirs
}
func writeEvidenceStageFD(stage *os.File, m authoritystore.EvidenceManifest, files map[string][]byte, raw []byte, owner, gid uint32) error {
	dirs := evidenceDirectories(m)
	for _, p := range dirs {
		d, err := evidenceDirectory(int(stage.Fd()), p, true)
		if err != nil {
			return err
		}
		if err = d.Close(); err != nil {
			return err
		}
	}
	for _, f := range m.Files {
		if err := evidenceWriteFile(int(stage.Fd()), "repo/"+f.Path, files[f.Path], owner, gid); err != nil {
			return err
		}
	}
	if err := evidenceWriteFile(int(stage.Fd()), "manifest.json", raw, owner, gid); err != nil {
		return err
	}
	// Child directories and their final metadata reach stable storage before
	// parents, then before the exclusive descriptor-relative publication rename.
	for _, p := range dirs {
		d, err := evidenceDirectory(int(stage.Fd()), p, false)
		if err != nil {
			return err
		}
		fd := int(d.Fd())
		if err = unix.Fchown(fd, int(owner), int(gid)); err == nil {
			err = unix.Fchmod(fd, 0550)
		}
		if err == nil {
			err = d.Sync()
		}
		closeErr := d.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
func stageAndPublishEvidence(parent *os.File, handle string, m authoritystore.EvidenceManifest, files map[string][]byte, raw []byte, owner, gid uint32) error {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return err
	}
	name := ".pending-" + hex.EncodeToString(random)
	if err := unix.Mkdirat(int(parent.Fd()), name, 0700); err != nil {
		return err
	}
	stage, err := evidenceOpenDirectory(int(parent.Fd()), name)
	if err != nil {
		_ = unix.Unlinkat(int(parent.Fd()), name, unix.AT_REMOVEDIR)
		return err
	}
	defer stage.Close()
	published := false
	defer func() {
		if !published {
			_ = removeEvidenceStageFD(stage)
			var named, held unix.Stat_t
			if unix.Fstat(int(stage.Fd()), &held) == nil && unix.Fstatat(int(parent.Fd()), name, &named, unix.AT_SYMLINK_NOFOLLOW) == nil && held.Dev == named.Dev && held.Ino == named.Ino {
				_ = unix.Unlinkat(int(parent.Fd()), name, unix.AT_REMOVEDIR)
			}
		}
	}()
	if err = writeEvidenceStageFD(stage, m, files, raw, owner, gid); err != nil {
		return err
	}
	if err = authoritystore.AuditEvidenceStageHandle(context.Background(), stage, m, owner, gid); err != nil {
		return err
	}
	var named, held unix.Stat_t
	if unix.Fstat(int(stage.Fd()), &held) != nil || unix.Fstatat(int(parent.Fd()), name, &named, unix.AT_SYMLINK_NOFOLLOW) != nil || held.Dev != named.Dev || held.Ino != named.Ino || held.Mode != named.Mode {
		return errors.New("evidence stage replaced")
	}
	if err = unix.RenameatxNp(int(parent.Fd()), name, int(parent.Fd()), handle, unix.RENAME_EXCL); err != nil {
		return err
	}
	published = true
	return parent.Sync()
}

func removeEvidenceStageFD(stage *os.File) error {
	root, err := evidenceOpenDirectory(int(stage.Fd()), ".")
	if err != nil {
		return err
	}
	defer root.Close()
	var remove func(*os.File, int) error
	remove = func(dir *os.File, depth int) error {
		if depth > 32 {
			return errors.New("stage cleanup depth")
		}
		if err := unix.Fchmod(int(dir.Fd()), 0700); err != nil {
			return err
		}
		names, err := dir.Readdirnames(4097)
		if err != nil && err != io.EOF {
			return err
		}
		if len(names) > 4096 {
			return errors.New("stage cleanup entries")
		}
		for _, name := range names {
			fd, e := unix.Openat(int(dir.Fd()), name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
			flags := 0
			if e == nil {
				file := os.NewFile(uintptr(fd), name)
				var st unix.Stat_t
				if e = unix.Fstat(fd, &st); e == nil && st.Mode&unix.S_IFMT == unix.S_IFDIR {
					flags = unix.AT_REMOVEDIR
					e = remove(file, depth+1)
				}
				closeErr := file.Close()
				if e != nil {
					return e
				}
				if closeErr != nil {
					return closeErr
				}
			} else if e != unix.ELOOP {
				return e
			}
			if e = unix.Unlinkat(int(dir.Fd()), name, flags); e != nil {
				return e
			}
		}
		return nil
	}
	return remove(root, 0)
}
