//go:build linux

package testconfine

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

// Linux Landlock (Documentation/userspace-api/landlock.rst), through raw
// system calls so the wrapper stays standard-library only.
const (
	sysCreateRuleset     = 444
	sysAddRule           = 445
	sysRestrictSelf      = 446
	createRulesetVersion = 1
	rulePathBeneath      = 1
	accessReadFile       = 1 << 2
	accessReadDir        = 1 << 3
	accessRefer          = 1 << 13 // ABI 2
	prSetNoNewPrivs      = 38
	openPath             = 0x200000 // O_PATH; landlock_path_beneath_attr is packed, and its 12 bytes lead this struct
	handledReadAccess    = accessReadFile | accessReadDir
	// A ruleset that does not handle REFER denies every cross-directory link
	// and rename with EXDEV, so the ruleset handles it and grants it on the
	// whole file system. Landlock still refuses a reparenting that would give a
	// file more read access than it had, such as a move out of a denied tree.
	handledAccess = handledReadAccess | accessRefer
)

type rulesetAttr struct{ handledAccessFS uint64 }

type pathBeneathAttr struct {
	allowedAccess uint64
	parentFD      int32
}

// ABI reports the kernel's Landlock ABI version, 0 when unavailable.
func ABI() int {
	version, _, errno := syscall.Syscall(sysCreateRuleset, 0, 0, createRulesetVersion)
	if errno != 0 {
		return 0
	}
	return int(version)
}

// ExecConfined restricts file reads and directory listing to rules, then
// replaces the process with argv. It returns only on failure. The calling
// goroutine is locked to its thread because Landlock and no_new_privs bind the
// thread that execve then carries into the new image.
func ExecConfined(rules []Rule, argv0 string, argv, env []string) error {
	runtime.LockOSThread()
	if abi := ABI(); abi < 2 {
		return fmt.Errorf("landlock ABI %d is below 2, which cross-directory renames need", abi)
	}
	attr := rulesetAttr{handledAccessFS: handledAccess}
	ruleset, _, errno := syscall.Syscall(sysCreateRuleset, uintptr(unsafe.Pointer(&attr)), unsafe.Sizeof(attr), 0)
	if errno != 0 {
		return fmt.Errorf("landlock_create_ruleset: %w", errno)
	}
	if err := addRule(int(ruleset), "/", accessRefer); err != nil {
		return err
	}
	for _, rule := range rules {
		access := uint64(accessReadFile)
		if rule.Dir {
			access = handledAccess
		}
		if err := addRule(int(ruleset), rule.Path, access); err != nil {
			return err
		}
	}
	if _, _, errno := syscall.RawSyscall6(syscall.SYS_PRCTL, prSetNoNewPrivs, 1, 0, 0, 0, 0); errno != 0 {
		return fmt.Errorf("prctl(PR_SET_NO_NEW_PRIVS): %w", errno)
	}
	if _, _, errno := syscall.Syscall(sysRestrictSelf, ruleset, 0, 0); errno != 0 {
		return fmt.Errorf("landlock_restrict_self: %w", errno)
	}
	syscall.Close(int(ruleset))
	return syscall.Exec(argv0, argv, env)
}

func addRule(ruleset int, path string, access uint64) error {
	fd, err := syscall.Open(path, openPath|syscall.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer syscall.Close(fd)
	attr := pathBeneathAttr{allowedAccess: access, parentFD: int32(fd)}
	if _, _, errno := syscall.Syscall6(sysAddRule, uintptr(ruleset), rulePathBeneath, uintptr(unsafe.Pointer(&attr)), 0, 0, 0); errno != 0 {
		return fmt.Errorf("landlock_add_rule %s: %w", path, errno)
	}
	return nil
}
