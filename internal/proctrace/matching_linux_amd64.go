// SPDX-License-Identifier: Apache-2.0
//go:build linux && amd64

// RSP-V0-007: preserve intermediate symlinks and refuse unsupported path matching.
package proctrace

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

func (c *collector) linkDependencies(name string) {
	// Expand one ordinary filesystem link at a time. Never collapse a target
	// before inspecting its components: it can itself contain another link.
	if !filepath.IsAbs(name) {
		c.fail("symlink-resolution-failed")
		return
	}
	prefix := "/"
	pending := strings.Split(name, "/")
	hops := 0
	for len(pending) > 0 {
		part := pending[0]
		pending = pending[1:]
		if part == "" || part == "." {
			continue
		}
		prefix = filepath.Join(prefix, part)
		// Procfs magic links are not ordinary textual symlinks.
		if _, ok := within("/proc", prefix); ok {
			c.fail("symlink-resolution-failed")
			return
		}
		c.checkMatching(prefix)
		info, err := os.Lstat(prefix)
		if err != nil {
			if !os.IsNotExist(err) {
				c.fail("symlink-resolution-failed")
				return
			}
			// Only ordinary names in a missing suffix have a known identity.
			// In particular, cleaning a later ".." would erase a component
			// whose absence stops the kernel's traversal. Fail closed instead.
			for _, tail := range pending {
				if tail == "" || tail == "." || tail == ".." {
					c.fail("symlink-resolution-failed")
					return
				}
				prefix = filepath.Join(prefix, tail)
			}
			// resolve's lexical fallback cannot name a dangling link's target.
			// Retain the expanded target and its repository ancestors here:
			// adding any missing directory can change this lookup's outcome.
			for {
				if p, ok := c.path(prefix, prefix, true); ok {
					appendUnique(&c.r.Lookups.Entries, p)
				}
				parent := filepath.Dir(prefix)
				if id, _ := c.repository(parent); id == "" || parent == prefix {
					break
				}
				prefix = parent
			}
			return
		}
		if info.Mode()&os.ModeSymlink != 0 {
			// Retain the link itself even if its target is dangling or cyclic.
			classified := prefix
			if id, _ := c.repository(prefix); id == "" {
				// Outside classes describe resolved paths, as for ordinary reads.
				// A system loader alias into a declared toolchain is not an
				// undeclared read. Repository links retain their own identity.
				if target, err := resolve(prefix); err == nil {
					classified = target
				}
			}
			if p, ok := c.path(prefix, classified, true); ok {
				appendUnique(&c.r.Reads.Entries, p)
			}
			hops++
			target, err := os.Readlink(prefix)
			if err != nil || hops > 40 {
				c.fail("symlink-resolution-failed")
				return
			}
			prefix = filepath.Dir(prefix)
			if filepath.IsAbs(target) {
				prefix = "/"
			}
			pending = append(strings.Split(target, "/"), pending...)
		} else if !info.IsDir() && len(pending) > 0 {
			c.fail("symlink-resolution-failed")
			return
		} else if !info.IsDir() && !info.Mode().IsRegular() {
			c.fail("symlink-resolution-failed")
			return
		}
	}
}

// This backend cannot prove case-fold equivalence on every Linux filesystem.
// Unknown filesystems and casefold directories are incomplete, never silently
// treated as case-sensitive. No probe files are created in the checkout.
func matchingReason(kind int64, flags uint64) string {
	switch uint64(kind) {
	case 0xef53, 0x58465342, 0x9123683e, 0x01021994, 0x858458f6:
		if flags&0x40000000 != 0 {
			return "casefold-filesystem"
		}
		return ""
	default:
		return "filesystem-matching-unsupported"
	}
}
func (c *collector) checkMatching(name string) {
	if id, _ := c.repository(name); id == "" {
		return
	}
	dir := filepath.Dir(name)
	if info, e := os.Stat(name); e == nil && info.IsDir() {
		dir = name
	}
	for {
		if id, _ := c.repository(dir); id == "" {
			return
		}
		f, err := os.Open(dir)
		if os.IsNotExist(err) {
			parent := filepath.Dir(dir)
			if parent == dir {
				return
			}
			dir = parent
			continue
		}
		if err != nil {
			c.fail("filesystem-matching-unavailable")
			return
		}
		var stat syscall.Statfs_t
		err = syscall.Fstatfs(int(f.Fd()), &stat)
		var flags uint64
		var errno syscall.Errno
		if err == nil && uint64(stat.Type) == 0xef53 {
			_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), 0x80086601, uintptr(unsafe.Pointer(&flags)))
			runtime.KeepAlive(&flags)
		}
		_ = f.Close()
		if err != nil || errno != 0 {
			c.fail("filesystem-matching-unavailable")
			return
		}
		if reason := matchingReason(stat.Type, flags); reason != "" {
			c.fail(reason)
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}
