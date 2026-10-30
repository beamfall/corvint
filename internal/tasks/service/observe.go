package service

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/tasks/safeopen"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// observeFile resolves declared once, then opens the resolved path through
// descriptor-anchored no-follow traversal and reports owner, mode and the
// streamed digest of exactly the opened file. When keep > 0 the bytes are
// returned too (bounded by keep).
func (h Host) observeFile(declared string, keep int) (FileFacts, []byte, error) {
	f := FileFacts{DeclaredPath: declared, State: "UNKNOWN"}
	resolved, err := filepath.EvalSymlinks(declared)
	if err != nil {
		return f, nil, err
	}
	f.Path = resolved
	file, err := safeopen.File(resolved)
	if err != nil {
		return f, nil, err
	}
	defer file.Close()
	fi, err := file.Stat()
	if err != nil {
		return f, nil, err
	}
	uid, ok := fileOwner(fi)
	if !ok {
		return f, nil, wire.Errorf(wire.CodeUnsupportedFilesystem, resolved, "file owner is not observable")
	}
	f.Owner = wire.SizeOf(uint64(uid))
	f.Mode = uint32(fi.Mode().Perm())
	f.Regular = fi.Mode().IsRegular()
	f.Executable = fi.Mode().Perm()&0o111 != 0
	f.NoReplacementSymlink = true
	if !f.Regular {
		return f, nil, wire.Errorf(wire.CodeUnsupportedFilesystem, resolved, "not a regular file")
	}
	if keep > 0 && fi.Size() > int64(keep) {
		return f, nil, wire.Errorf(wire.CodeLimitExceeded, resolved, "file exceeds %d bytes", keep)
	}
	sum := sha256.New()
	var raw []byte
	var r io.Reader = file
	if keep > 0 {
		raw, err = io.ReadAll(io.LimitReader(file, int64(keep)+1))
		if err != nil {
			return f, nil, err
		}
		if len(raw) > keep {
			return f, nil, wire.Errorf(wire.CodeLimitExceeded, resolved, "file exceeds %d bytes", keep)
		}
		sum.Write(raw)
	} else if _, err := io.Copy(sum, r); err != nil {
		return f, nil, err
	}
	f.Sha256 = wire.Digest(hex.EncodeToString(sum.Sum(nil)))
	f.SafeAncestors = h.safeAncestors(filepath.Dir(resolved))
	if !f.SafeAncestors {
		return f, nil, wire.Errorf(wire.CodeUnsupportedFilesystem, resolved, "an ancestor directory is writable by another user")
	}
	f.State = "VERIFIED"
	return f, raw, nil
}

// safeAncestors requires every ancestor directory to be owned by the bound
// user or root and not group/world writable, except a root-owned sticky
// directory.
func (h Host) safeAncestors(dir string) bool {
	for {
		fi, err := os.Lstat(dir)
		if err != nil || !fi.IsDir() {
			return false
		}
		uid, ok := fileOwner(fi)
		if !ok || (int(uid) != h.UID && uid != 0) {
			return false
		}
		if fi.Mode().Perm()&0o022 != 0 && !(uid == 0 && fi.Mode()&os.ModeSticky != 0) {
			return false
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return true
		}
		dir = parent
	}
}

// unitDir verifies (creating when asked) the user's unit directory: owned by
// the bound user and not group/world writable.
func (h Host) unitDir(dir string, create bool) error {
	if create {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	fi, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	uid, ok := fileOwner(fi)
	if !fi.IsDir() || !ok || int(uid) != h.UID || fi.Mode().Perm()&0o022 != 0 {
		return wire.Errorf(wire.CodeUnsupportedFilesystem, dir, "unit directory must be owned by uid %d and not group/world writable", h.UID)
	}
	return nil
}

// Unit file states reported by status and used for ownership decisions.
const (
	fileOwned    = "OWNED"
	fileAbsent   = "ABSENT"
	fileModified = "MODIFIED"
	fileUnknown  = "UNKNOWN"
)

func (h Host) unitFile(u Unit) string {
	raw, err := h.readPrivate(u.Path, 64*wire.KiB)
	switch {
	case absent(err):
		return fileAbsent
	case err != nil:
		return fileUnknown
	case wire.Sum(raw) == wire.Sum(u.Raw):
		return fileOwned
	}
	return fileModified
}

// systemdUserDropInRoots are the system-wide systemd --user unit roots
// whose per-unit drop-ins also apply to a user unit. Type-wide drop-ins
// (for example service.d) and generator output are not observed.
var systemdUserDropInRoots = []string{"/etc/systemd/user", "/run/systemd/user", "/usr/local/lib/systemd/user", "/usr/lib/systemd/user"}

// dropIns reports whether a systemd drop-in directory exists, or cannot be
// proved absent, for the unit in the user root or a system-wide root.
func dropIns(m *Manifest, u Unit) bool {
	if m.Manager != "systemd-user" {
		return false
	}
	paths := []string{u.Path + ".d"}
	for _, root := range systemdUserDropInRoots {
		paths = append(paths, filepath.Join(root, filepath.Base(u.Path)+".d"))
	}
	for _, p := range paths {
		if _, err := os.Lstat(p); !absent(err) {
			return true
		}
	}
	return false
}

// existing observes one manifest unit as the planner's ExistingUnit.
func (h Host) existing(m *Manifest, u Unit) ExistingUnit {
	e := ExistingUnit{Label: u.Label, Path: u.Path, Registration: h.query(m, u), NoSymlink: true, NoDropIns: !dropIns(m, u)}
	switch h.unitFile(u) {
	case fileOwned:
		e.Ownership, e.Sha256 = "OWNED", wire.Sum(u.Raw)
	case fileAbsent:
		e.Ownership = "ABSENT"
	default:
		e.Ownership = "FOREIGN"
	}
	return e
}
