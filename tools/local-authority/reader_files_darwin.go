//go:build darwin

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Beamfall/corvint/internal/authoritystore"
	"golang.org/x/sys/unix"
)

func readerParent() (int, error) {
	if e := auditRootDirectory(authoritystore.RootPath, false); e != nil {
		return -1, e
	}
	return unix.Open(authoritystore.RootPath, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
}
func closeReaderEvidence(fd int) { _ = unix.Close(fd) }
func createReaderEvidence(l *readerLedger) (int, error) {
	parent, e := readerParent()
	if e != nil {
		return -1, e
	}
	defer unix.Close(parent)
	if e = unix.Mkdirat(parent, "evidence", 0700); e != nil {
		return -1, e
	}
	fd, e := unix.Openat(parent, "evidence", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if e != nil {
		return -1, e
	}
	var st unix.Stat_t
	if e = unix.Fstat(fd, &st); e != nil || st.Uid != 0 || st.Mode&0777 != 0700 {
		unix.Close(fd)
		return -1, errors.New("new evidence identity")
	}
	if e = emptyACLFD(fd); e != nil {
		unix.Close(fd)
		return -1, e
	}
	l.Device = strconv.FormatUint(uint64(st.Dev), 10)
	l.Inode = strconv.FormatUint(st.Ino, 10)
	l.State = "DIRECTORY_BOUND"
	if e = unix.Fsync(parent); e != nil {
		unix.Close(fd)
		return -1, e
	}
	return fd, nil
}
func ownReaderEvidence(fd int, a readerAudit) error {
	uid, _ := strconv.Atoi(a.AuthorityUID)
	gid, _ := strconv.Atoi(a.AuthorityGID)
	if e := unix.Fchown(fd, uid, gid); e != nil {
		return e
	}
	return unix.Fsync(fd)
}
func namedReaderEvidence(fd int, l readerLedger) error {
	parent, e := readerParent()
	if e != nil {
		return e
	}
	defer unix.Close(parent)
	var held, named unix.Stat_t
	if e = unix.Fstat(fd, &held); e != nil {
		return e
	}
	if e = unix.Fstatat(parent, "evidence", &named, unix.AT_SYMLINK_NOFOLLOW); e != nil {
		return e
	}
	if held.Dev != named.Dev || held.Ino != named.Ino || held.Mode&unix.S_IFMT != unix.S_IFDIR || strconv.FormatUint(uint64(held.Dev), 10) != l.Device || strconv.FormatUint(held.Ino, 10) != l.Inode {
		return errors.New("evidence named binding")
	}
	return nil
}
func exposeReaderEvidence(fd int, l readerLedger) error {
	if e := namedReaderEvidence(fd, l); e != nil {
		return e
	}
	var st unix.Stat_t
	if e := unix.Fstat(fd, &st); e != nil {
		return e
	}
	uid, _ := strconv.ParseUint(l.Audit.AuthorityUID, 10, 32)
	gid, _ := strconv.ParseUint(l.Audit.AuthorityGID, 10, 32)
	if st.Uid != uint32(uid) || st.Gid != uint32(gid) || st.Mode&0777 != 0700 {
		return errors.New("evidence permission preflight")
	}
	if e := emptyACLFD(fd); e != nil {
		return e
	}
	if e := unix.Fchmod(fd, 0750); e != nil {
		return e
	}
	if e := unix.Fsync(fd); e != nil {
		return e
	}
	return namedReaderEvidence(fd, l)
}
func restrictReaderEvidence(l readerLedger) error {
	parent, e := readerParent()
	if e != nil {
		return e
	}
	defer unix.Close(parent)
	fd, e := unix.Openat(parent, "evidence", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if e == unix.ENOENT && l.State == "INTENT" && l.Inode == "" {
		return nil
	}
	if e != nil {
		return e
	}
	defer unix.Close(fd)
	var st unix.Stat_t
	if e = unix.Fstat(fd, &st); e != nil {
		return e
	}
	if l.Inode == "" {
		if st.Uid != 0 || st.Mode&0777 != 0700 {
			return errors.New("unbound evidence retained")
		}
		l.Device = strconv.FormatUint(uint64(st.Dev), 10)
		l.Inode = strconv.FormatUint(st.Ino, 10)
	}
	return runReaderRestriction(l.Audit.AuthorityUID, readerRestrictionOps{
		binding: func() error { return namedReaderEvidence(fd, l) },
		chmod:   func() error { return unix.Fchmod(fd, 0700) },
		owner:   func() (uint32, error) { var current unix.Stat_t; e := unix.Fstat(fd, &current); return current.Uid, e },
		acl:     func() error { return emptyACLFD(fd) }, sync: func() error { return unix.Fsync(fd) },
	})
}
func writeReaderLedger(l readerLedger) error {
	parent, e := readerParent()
	if e != nil {
		return e
	}
	defer unix.Close(parent)
	// Temporary name is exclusive; a crashed attempt remains explicit operator
	// recovery work, never automatically overwritten or recursively removed.
	const stage = ".installed-reader.next"
	fd, e := unix.Openat(parent, stage, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if e != nil {
		return e
	}
	f := os.NewFile(uintptr(fd), stage)
	defer f.Close()
	if _, e = f.Write(mustJSONLine(l)); e != nil {
		return e
	}
	if e = f.Sync(); e != nil {
		return e
	}
	if e = unix.Renameat(parent, stage, parent, readerLedgerName); e != nil {
		return e
	}
	return unix.Fsync(parent)
}
func retireReaderLedger() error {
	parent, e := readerParent()
	if e != nil {
		return e
	}
	defer unix.Close(parent)
	if e = renameExclusive(parent, readerLedgerName, parent, "retired-reader.json"); e != nil {
		return e
	}
	return unix.Fsync(parent)
}

// Account retirement must be preceded by reader withdrawal, including partial
// admission recovery. Evidence remains archived and never recursively chowned.
func requireReaderWithdrawn() error {
	_, e := readRootFile(filepath.Join(authoritystore.RootPath, readerLedgerName), 32<<10)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	return errors.New("withdraw reader before retiring accounts")
}

func archiveRetiredReaderEvidence() error {
	l, e := readReaderRecord(filepath.Join(authoritystore.RootPath, "retired-reader.json"))
	if os.IsNotExist(e) {
		if _, err := os.Lstat(filepath.Join(authoritystore.RootPath, "evidence")); os.IsNotExist(err) {
			return nil
		}
		return errors.New("unrecorded evidence prevents account retirement")
	}
	if e != nil {
		return e
	}
	parent, e := readerParent()
	if e != nil {
		return e
	}
	defer unix.Close(parent)
	fd, e := unix.Openat(parent, "evidence", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if e == unix.ENOENT && l.Inode == "" {
		return nil
	}
	if e != nil {
		return e
	}
	defer unix.Close(fd)
	if l.Inode == "" {
		return errors.New("unbound retained evidence requires operator recovery")
	}
	if e = namedReaderEvidence(fd, l); e != nil {
		return e
	}
	if e = unix.Fchmod(fd, 0700); e != nil {
		return e
	}
	if e = emptyACLFD(fd); e != nil {
		return e
	}
	if e = unix.Fchown(fd, 0, 0); e != nil {
		return e
	}
	if e = unix.Fsync(fd); e != nil {
		return e
	}
	return namedReaderEvidence(fd, l)
}
