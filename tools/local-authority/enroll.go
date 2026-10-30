package main

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/authoritystore"
	"golang.org/x/sys/unix"
)

// Root copies bounded opaque files in a root-only staging directory. Every
// destination operation after ownership transfer is descriptor-relative.
func stageEnrollment(handle, source string) error {
	if os.Geteuid() != 0 {
		return errors.New("operator root required")
	}
	if _, e := decodeHex(handle, 32); e != nil {
		return e
	}
	unlock, lockErr := operatorLock()
	if lockErr != nil {
		return lockErr
	}
	defer unlock()
	p, e := account("_corvintauthority")
	if e != nil {
		return e
	}
	input, e := unix.Open(source, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if e != nil {
		return e
	}
	defer unix.Close(input)
	stagingRoot := filepath.Join(authoritystore.RootPath, "staging")
	if e = ensureRootDirectory(stagingRoot); e != nil {
		return e
	}
	if e = os.Chmod(stagingRoot, 0700); e != nil {
		return e
	}
	stage, e := os.MkdirTemp(stagingRoot, ".import-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(stage)
	stageFD, e := unix.Open(stage, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if e != nil {
		return e
	}
	defer unix.Close(stageFD)
	if e = copyEnrollmentFiles(input, stageFD, p); e != nil {
		return e
	}
	if e = unix.Fchown(stageFD, int(p.uid), int(p.gid)); e != nil {
		return e
	}
	if e = unix.Fsync(stageFD); e != nil {
		return e
	}
	destination, e := openAuthorityDirectory("private/enrollments", p.uid)
	if e != nil {
		return e
	}
	defer unix.Close(destination)
	parent, e := unix.Open(stagingRoot, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if e != nil {
		return e
	}
	defer unix.Close(parent)
	if e = renameExclusive(parent, filepath.Base(stage), destination, handle); e != nil {
		return e
	}
	return unix.Fsync(destination)
}
func copyEnrollmentFiles(input, stage int, p principal) error {
	for _, name := range []string{"enrollment.json", "objects.json", "cem.json", "ocm.json", "selection.json", "target.json"} {
		limit := 128 << 10
		if name == "objects.json" {
			limit = 24 << 20
		}
		if name == "cem.json" || name == "ocm.json" {
			limit = 4 << 20
		}
		fd, e := unix.Openat(input, name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
		if e != nil {
			return e
		}
		file := os.NewFile(uintptr(fd), name)
		info, e := file.Stat()
		if e != nil || !info.Mode().IsRegular() {
			file.Close()
			return errors.New("nonregular enrollment file")
		}
		raw, e := readBound(file, limit)
		file.Close()
		if e != nil {
			return e
		}
		output, e := unix.Openat(stage, name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
		if e != nil {
			return e
		}
		written := os.NewFile(uintptr(output), name)
		_, e = written.Write(raw)
		if e == nil {
			e = written.Sync()
		}
		if e == nil {
			e = unix.Fchown(output, int(p.uid), int(p.gid))
		}
		closeErr := written.Close()
		if e != nil {
			return e
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
