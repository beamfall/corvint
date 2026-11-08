// SPDX-License-Identifier: Apache-2.0
//go:build linux && amd64

// RSP-V0-021 and RSP-V0-022: syscall observations fail closed on unsupported activity.
package proctrace

import (
	"bytes"
	"crypto/sha256"
	"debug/elf"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type call struct {
	kind                          string
	lexical, resolved             string
	targetLexical, targetResolved string
	flags                         uint64
	mode                          uint64
}

func (c *collector) fail(reason string) { c.r.unavailable(Incomplete, reason) }
func stringAt(pid int, address uint64) (string, error) {
	if address == 0 {
		return "", fmt.Errorf("null path")
	}
	data := make([]byte, 0, 256)
	for len(data) < 4096 {
		buf := make([]byte, 8)
		n, err := syscall.PtracePeekData(pid, uintptr(address)+uintptr(len(data)), buf)
		if err != nil || n != len(buf) {
			return "", fmt.Errorf("unreadable path")
		}
		for _, b := range buf {
			if b == 0 {
				return string(data), nil
			}
			data = append(data, b)
		}
	}
	return "", fmt.Errorf("path string cap")
}

// RSP-V0-007/021: exec.Cmd inherits only these three descriptors. Procfs loses
// their lexical names, so retain caller-supplied file names before any dup/close
// can hide their provenance. Extra dependencies are conservative even if unused.
func (c *collector) inheritedFiles(pid int) {
	for n, input := range []any{c.q.Stdin, c.q.Stdout, c.q.Stderr} {
		name, err := os.Readlink(procPath(pid, "fd/"+strconv.Itoa(n)))
		if err != nil {
			c.fail("unresolved-descriptor")
			continue
		}
		if strings.HasPrefix(name, "pipe:[") || strings.HasPrefix(name, "socket:[") || strings.HasPrefix(name, "anon_inode:") {
			continue // Peer uncertainty is recorded if the descriptor is observed.
		}
		if input == nil && name == "/dev/null" {
			continue // exec.Cmd supplies this explicit default.
		}
		file, ok := input.(*os.File)
		if !ok || file == nil {
			c.fail("unresolved-descriptor-path")
			continue
		}
		if strings.Contains(file.Name(), "/../") || strings.HasPrefix(file.Name(), "../") || strings.HasSuffix(file.Name(), "/..") {
			c.fail("parent-traversal")
			continue
		}
		lexical, err := filepath.Abs(file.Name())
		if err != nil {
			c.fail("unresolved-descriptor-path")
			continue
		}
		resolved, err := resolve(lexical)
		if err != nil || resolved != name {
			c.fail("unresolved-descriptor-path")
			continue
		}
		c.linkDependencies(lexical)
		c.checkMatching(lexical)
		c.checkMatching(resolved)
		c.read(lexical, resolved)
	}
}

func (c *collector) fd(pid int, n uint64) (string, string) {
	p, err := os.Readlink(procPath(pid, "fd/"+strconv.FormatInt(int64(n), 10)))
	if err != nil {
		c.fail("unresolved-descriptor")
		return "", ""
	}
	if strings.HasPrefix(p, "pipe:[") || strings.HasPrefix(p, "socket:[") || strings.HasPrefix(p, "anon_inode:") {
		// Anonymous descriptors may be inherited or passed from an outside process.
		// Without provenance they cannot be used to exclude a test.
		mark(&c.r.External, Incomplete, "descriptor-peer-unknown")
		return "", ""
	}
	if !filepath.IsAbs(p) || strings.HasSuffix(p, " (deleted)") {
		c.fail("unresolved-descriptor-path")
		return "", ""
	}
	// Initial inherited lexical dependencies were captured before execution;
	// ordinary opens record their lexical dependencies at syscall entry.
	return p, p
}
func (c *collector) pathAt(pid int, dir uint64, address uint64, empty, followFinal bool) (string, string) {
	p, err := stringAt(pid, address)
	if err != nil {
		c.fail("malformed-path")
		return "", ""
	}
	if p == "" {
		if empty {
			return c.fd(pid, dir)
		}
		c.fail("empty-path")
		return "", ""
	}
	if !filepath.IsAbs(p) {
		var base string
		if int32(dir) == -100 {
			base, err = os.Readlink(procPath(pid, "cwd"))
		} else {
			base, _ = c.fd(pid, dir)
		}
		if err != nil || base == "" {
			c.fail("unresolved-dirfd")
			return "", ""
		}
		// Cleaning a/../b before resolving a symlink would change its meaning.
		p = base + "/" + p
	}
	name := p
	if !followFinal {
		// RSP-V0-007: rename replaces the directory entry, not a final link's
		// target. Ancestor links still participate in kernel path resolution.
		name = filepath.Dir(p)
	}
	resolved, err := resolve(name)
	if err != nil {
		c.fail("path-resolution-failed")
		return "", ""
	}
	if !followFinal {
		resolved = filepath.Join(resolved, filepath.Base(p))
	}
	// filepath.EvalSymlinks preserves traversal semantics; lexical paths containing
	// symlink/.. are not representable by the normalized evidence path alone.
	if strings.Contains(p, "/../") || strings.HasSuffix(p, "/..") {
		c.fail("parent-traversal")
	}
	lexical := filepath.Clean(p)
	c.linkDependencies(lexical)
	c.checkMatching(lexical)
	c.checkMatching(resolved)
	return lexical, resolved
}
func (c *collector) enter(pid int, regs syscall.PtraceRegs) *call {
	p := &call{}
	// x86 compat and x32 use different argument and structure layouts.
	if regs.Cs != 0x33 || regs.Orig_rax&0x40000000 != 0 {
		c.fail("unsupported-syscall-abi")
		return p
	}
	n := regs.Orig_rax
	a := [6]uint64{regs.Rdi, regs.Rsi, regs.Rdx, regs.R10, regs.R8, regs.R9}
	at := func(dir, addr uint64, empty bool) { p.lexical, p.resolved = c.pathAt(pid, dir, addr, empty, true) }
	fd := func(n uint64) { p.lexical, p.resolved = c.fd(pid, n) }
	cwd := uint64(0xffffffffffffff9c)
	switch n {
	case 2:
		p.kind = "open"
		at(cwd, a[0], false)
		p.flags = a[1]
	case 85:
		p.kind = "open"
		at(cwd, a[0], false)
		p.flags = syscall.O_CREAT | syscall.O_WRONLY | syscall.O_TRUNC
	case 257:
		p.kind = "open"
		at(a[0], a[1], false)
		p.flags = a[2]
	case 437: // openat2: read only the known open_how size, reject extensions.
		p.kind = "open"
		at(a[0], a[1], false)
		if a[3] != 24 {
			c.fail("unsupported-open-how")
			break
		}
		buf := make([]byte, 24)
		if count, e := syscall.PtracePeekData(pid, uintptr(a[2]), buf); e != nil || count != len(buf) {
			c.fail("malformed-open-how")
		} else {
			p.flags = binary.LittleEndian.Uint64(buf)
			if binary.LittleEndian.Uint64(buf[16:]) != 0 {
				c.fail("openat2-resolution-policy")
			}
		}
	case 0, 17, 19, 295, 327:
		p.kind = "read"
		fd(a[0])
	case 1, 18, 20, 296, 328:
		p.kind = "write"
		fd(a[0])
		p.flags = c.fdFlags(pid, a[0])
	case 4, 6, 21, 89, 80, 132, 137, 191, 192, 194, 195:
		p.kind = "read"
		at(cwd, a[0], false)
	case 5, 8, 138: // fstat, lseek (including inherited file-size queries), fstatfs.
		p.kind = "read"
		fd(a[0])
	case 262, 269, 267, 439, 332:
		p.kind = "read"
		flags := a[3]
		if n == 332 {
			flags = a[2]
		}
		at(a[0], a[1], flags&0x1000 != 0)
	case 78, 217:
		p.kind = "listing"
		fd(a[0])
	case 76:
		p.kind = "truncate"
		at(cwd, a[0], false)
	case 77:
		p.kind = "truncate"
		fd(a[0])
	case 82:
		p.kind = "rename"
		p.lexical, p.resolved = c.pathAt(pid, cwd, a[0], false, false)
		p.targetLexical, p.targetResolved = c.pathAt(pid, cwd, a[1], false, false)
	case 264, 316:
		p.kind = "rename"
		p.lexical, p.resolved = c.pathAt(pid, a[0], a[1], false, false)
		p.targetLexical, p.targetResolved = c.pathAt(pid, a[2], a[3], false, false)
		if n == 316 && a[4] != 0 && a[4] != 1 {
			c.fail("unsupported-rename-flags")
		}
	case 87:
		p.kind = "unlink"
		at(cwd, a[0], false)
	case 263:
		p.kind = "unlink"
		at(a[0], a[1], false)
	case 59:
		p.kind = "exec"
		at(cwd, a[0], false)
		c.checkExec(p.resolved)
	case 322:
		p.kind = "exec"
		at(a[0], a[1], a[4]&0x1000 != 0)
		c.checkExec(p.resolved)
	case 42, 43, 44, 45, 46, 47, 49, 50, 288, 299, 307:
		p.kind = "external"
	case 9: // mmap: an inherited file descriptor can introduce reads without open.
		// Only private mappings with these understood flags/protections have
		// no shared-memory peer. A declared temporary root proves no ownership
		// of mappings held by processes outside the traced tree.
		const privateFlags = syscall.MAP_PRIVATE | syscall.MAP_ANON | syscall.MAP_FIXED | syscall.MAP_DENYWRITE | syscall.MAP_EXECUTABLE
		if a[3]&0xf != syscall.MAP_PRIVATE || a[3]&^uint64(privateFlags) != 0 || a[2]&^uint64(7) != 0 {
			mark(&c.r.External, Incomplete, "descriptor-peer-unknown")
			if a[3]&0xf != syscall.MAP_SHARED && a[3]&0xf != 3 {
				c.fail("unsupported-syscall:9")
			}
		}
		if a[3]&syscall.MAP_ANON == 0 {
			p.kind = "read"
			fd(a[4])
		}
	case 56, 57, 58, 435: // descendants are registered by ptrace events, before they run.
		if n == 56 && a[0]&syscall.CLONE_VM != 0 {
			c.fail("shared-memory-path-race")
		}
		if n == 56 && a[0]&syscall.CLONE_UNTRACED != 0 {
			c.fail("clone-untraced")
		}
		if n == 435 {
			c.fail("clone3-configuration")
		}
	case 158: // arch_prctl: only the fixed thread-local segment base operations.
		if a[0] < 0x1001 || a[0] > 0x1004 {
			c.fail("unsupported-syscall:" + strconv.FormatUint(n, 10))
		}
	case 218: // set_tid_address: local thread setup with a fixed pointer argument.
	case 273: // set_robust_list: only the known three-word head layout.
		if a[1] != 24 {
			c.fail("unsupported-syscall:" + strconv.FormatUint(n, 10))
		}
	case 302: // prlimit64: only a query of this process, never another PID.
		if a[0] != 0 || a[2] != 0 {
			c.fail("unsupported-syscall:" + strconv.FormatUint(n, 10))
		}
	case 318: // getrandom: the three understood flag bits, no external descriptor.
		if a[2]&^uint64(7) != 0 {
			c.fail("unsupported-syscall:" + strconv.FormatUint(n, 10))
		}
	case 334: // rseq: the known registration layout and register/unregister forms.
		if a[1] != 32 || a[2] > 1 {
			c.fail("unsupported-syscall:" + strconv.FormatUint(n, 10))
		}
	case 41, 53: // socket/socketpair alone does not reach a peer.
	case 16:
		c.fail("unsupported-ioctl")
	default:
		c.fail("unsupported-syscall:" + strconv.FormatUint(n, 10))
	}
	if p.resolved != "" {
		st, e := os.Stat(p.resolved)
		if e == nil && st.Mode()&(os.ModeNamedPipe|os.ModeSocket) != 0 {
			appendUnique(&c.r.External.Entries, "filesystem-peer-unverified")
		}
		if _, ok := within("/proc", p.resolved); ok {
			c.fail("procfs-process-relative-path")
		}
	}
	return p
}
func (c *collector) fdFlags(pid int, fd uint64) uint64 {
	// fdinfo is a tiny procfs record; cap the read before parsing it.
	f, err := os.Open(procPath(pid, "fdinfo/"+strconv.FormatUint(fd, 10)))
	if err != nil {
		c.fail("descriptor-flags-missing")
		return 0
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 4097))
	if err != nil || len(b) > 4096 {
		c.fail("descriptor-flags-malformed")
		return 0
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "flags:") {
			n, err := strconv.ParseUint(strings.TrimSpace(strings.TrimPrefix(line, "flags:")), 8, 64)
			if err == nil {
				return n
			}
		}
	}
	c.fail("descriptor-flags-malformed")
	return 0
}
func (c *collector) finish(p *call, ret int64) {
	if p.kind == "external" {
		appendUnique(&c.r.External.Entries, "connection-peer-unverified")
	}
	if ret < 0 {
		// The errno does not identify which operand or ancestor failed. Retain
		// every operand, including destination parents whose creation can change
		// the result even when the source already exists.
		for _, operand := range [][2]string{{p.lexical, p.resolved}, {p.targetLexical, p.targetResolved}} {
			lexical, resolved := operand[0], operand[1]
			if lexical == "" {
				continue
			}
			// Only ENOENT admits lookup-only evidence. Every other errno,
			// including unknown ones, retains the existing-path dependency.
			if ret != -int64(syscall.ENOENT) {
				c.read(lexical, resolved)
			}
			if ret != -int64(syscall.ENOENT) && ret != -int64(syscall.ENOTDIR) {
				continue
			}
			for {
				if path, ok := c.path(lexical, resolved, true); ok {
					appendUnique(&c.r.Lookups.Entries, path)
				}
				parent := filepath.Dir(lexical)
				if id, _ := c.repository(parent); id == "" || parent == lexical {
					break
				}
				lexical, resolved = parent, filepath.Dir(resolved)
			}
		}
		return
	}
	if p.kind == "external" {
		appendUnique(&c.r.External.Entries, "connection-peer-unverified")
		return
	}
	if p.lexical == "" {
		return
	}
	switch p.kind {
	case "open":
		// RSP-V0-007/022: only understood ordinary opens can prove creation.
		// O_PATH ignores access/create/truncate flags, including O_CREAT|O_EXCL.
		if !supportedOpenFlags(p.flags) {
			c.fail("unsupported-syscall:2")
			c.read(p.lexical, p.resolved)
			return
		}
		if p.flags&openPath != 0 {
			c.read(p.lexical, p.resolved)
			return
		}
		// Success itself observes the existing path, even without a data read.
		// Only an understood exclusive creation (or prior owned creation in read)
		// proves independence from the base tree.
		if p.flags&(syscall.O_CREAT|syscall.O_EXCL) != syscall.O_CREAT|syscall.O_EXCL {
			c.read(p.lexical, p.resolved)
		}
		if p.flags&syscall.O_CREAT != 0 && p.flags&syscall.O_EXCL != 0 {
			c.write(p.lexical, p.resolved, "create", true)
		} else if p.flags&syscall.O_TRUNC != 0 {
			c.write(p.lexical, p.resolved, "truncate", false)
		} else if p.flags&syscall.O_ACCMODE != syscall.O_RDONLY {
			kind := "overwrite"
			if p.flags&syscall.O_APPEND != 0 {
				kind = "append"
			}
			c.write(p.lexical, p.resolved, kind, false)
		}
	case "read":
		c.read(p.lexical, p.resolved)
	case "write":
		kind := "overwrite"
		if p.flags&syscall.O_APPEND != 0 {
			kind = "append"
		}
		c.write(p.lexical, p.resolved, kind, false)
	case "truncate":
		c.read(p.lexical, p.resolved)
		c.write(p.lexical, p.resolved, "truncate", false)
	case "listing":
		if path, ok := c.path(p.lexical, p.resolved, false); ok {
			appendUnique(&c.r.Listings.Entries, path)
		}
	case "rename":
		if p.targetLexical == "" {
			c.fail("rename-target-missing")
			return
		}
		owned := c.created[p.resolved]
		targetOwned := c.created[p.targetResolved]
		delete(c.created, p.resolved)
		delete(c.created, p.targetResolved)
		c.write(p.targetLexical, p.targetResolved, "replace", owned)
		// Both names affect the operation, even when no data was read. A
		// created source proves only its own contents new: an existing
		// destination's type still decides whether the replacement succeeds.
		if !owned {
			c.read(p.lexical, p.resolved)
		}
		if !targetOwned {
			c.read(p.targetLexical, p.targetResolved)
		}
	case "unlink":
		delete(c.created, p.resolved)
		c.read(p.lexical, p.resolved)
	case "exec":
		c.recordExec(p.lexical, p.resolved)
	}
}

const openPath = 0x200000 // Linux O_PATH is absent from the syscall package.

func supportedOpenFlags(flags uint64) bool {
	const known = syscall.O_ACCMODE | syscall.O_CREAT | syscall.O_EXCL | syscall.O_NOCTTY |
		syscall.O_TRUNC | syscall.O_APPEND | syscall.O_NONBLOCK | syscall.O_DSYNC |
		syscall.O_DIRECT | 0x8000 /* O_LARGEFILE */ | syscall.O_DIRECTORY |
		syscall.O_NOFOLLOW | syscall.O_NOATIME | syscall.O_CLOEXEC | syscall.O_SYNC | openPath
	if flags & ^uint64(known) != 0 {
		return false
	}
	if flags&openPath != 0 {
		return true // All other admitted bits except DIRECTORY/NOFOLLOW/CLOEXEC are ignored.
	}
	return flags&syscall.O_ACCMODE != syscall.O_ACCMODE &&
		(flags&syscall.O_EXCL == 0 || flags&syscall.O_CREAT != 0) &&
		(flags&syscall.O_TRUNC == 0 || flags&syscall.O_ACCMODE != syscall.O_RDONLY) &&
		flags&(syscall.O_DIRECTORY|syscall.O_CREAT) != syscall.O_DIRECTORY|syscall.O_CREAT
}
func (c *collector) checkExec(path string) {
	st, err := os.Stat(path)
	if err != nil {
		c.fail("exec-metadata-unavailable")
		return
	}
	if st.Mode()&(os.ModeSetuid|os.ModeSetgid) != 0 {
		c.fail("privileged-executable")
	}
	n, e := syscall.Getxattr(path, "security.capability", nil)
	if n > 0 || (e != nil && e != syscall.ENODATA && e != syscall.ENOTSUP) {
		c.fail("executable-capabilities")
	}
}
func (c *collector) recordExec(lexical, resolved string) {
	c.checkExec(resolved)
	p, ok := c.path(lexical, resolved, true)
	f, err := os.Open(resolved)
	if err != nil {
		c.fail("exec-digest-unavailable")
		return
	}
	defer f.Close()
	if ok {
		h := sha256.New()
		n, err := io.Copy(h, io.LimitReader(f, (64<<20)+1))
		if err != nil || n > 64<<20 {
			c.fail("exec-digest-cap")
			return
		}
		appendUnique(&c.r.Execs.Entries, Exec{Path: p, Digest: hex.EncodeToString(h.Sum(nil))})
	}
	// The kernel loads ELF interpreters without an open syscall in the tracee.
	// Outside classes exempt the executable's path, not its implicit inputs.
	// Only native ELF executables have a supported implicit-dependency model.
	image, err := elf.NewFile(f)
	if err != nil || image.Class != elf.ELFCLASS64 || image.Data != elf.ELFDATA2LSB ||
		image.Machine != elf.EM_X86_64 || image.Version != elf.EV_CURRENT ||
		(image.Type != elf.ET_EXEC && image.Type != elf.ET_DYN) ||
		(image.OSABI != elf.ELFOSABI_NONE && image.OSABI != elf.ELFOSABI_LINUX) {
		c.fail("unsupported-executable-format")
		return
	}
	for _, program := range image.Progs {
		if program.Type != elf.PT_INTERP {
			continue
		}
		raw, e := io.ReadAll(io.LimitReader(program.Open(), 4097))
		if e != nil || len(raw) < 2 || len(raw) > 4096 || raw[len(raw)-1] != 0 || bytes.IndexByte(raw[:len(raw)-1], 0) >= 0 {
			c.fail("malformed-elf-interpreter")
			continue
		}
		name := string(raw[:len(raw)-1])
		c.linkDependencies(name)
		c.checkMatching(name)
		target, e := resolve(name)
		if !filepath.IsAbs(name) || e != nil {
			c.fail("unresolved-elf-interpreter")
			continue
		}
		c.checkMatching(target)
		c.read(name, target)
	}

}
