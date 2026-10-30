//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package source

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
)

func openHeldTraceDirectory(root *os.Root, relativeDirectory string) (*heldTraceDirectory, IssueCode) {
	parts := strings.Split(relativeDirectory, "/")
	current := root
	held := &heldTraceDirectory{}
	for index, part := range parts {
		info, identity, issue := inspectTraceDirectoryBinding(current, part)
		if issue != IssueNone {
			held.close()
			return nil, issue
		}
		file, err := current.OpenFile(part, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		if err != nil {
			held.close()
			if os.IsPermission(err) {
				return nil, IssueSourceUnreadable
			}
			return nil, IssueSourceUnstable
		}
		descriptorInfo, err := file.Stat()
		descriptorIdentity, qualified := qualifiedDescriptorIdentity(file, descriptorInfo)
		if err != nil || !qualified || !descriptorInfo.IsDir() || descriptorIdentity != identity || !sameIdentity(info, descriptorInfo, false) {
			_ = file.Close()
			held.close()
			return nil, IssueSourceUnstable
		}
		next, err := os.OpenRoot(traceDescriptorPath(file.Fd()))
		if err != nil {
			_ = file.Close()
			held.close()
			return nil, IssueSourceUnreadable
		}
		held.bindings = append(held.bindings, traceDirectoryBinding{parent: current, name: part, identity: identity})
		held.ownedRoots = append(held.ownedRoots, next)
		current = next
		if index == len(parts)-1 {
			held.file = file
			held.root = next
			return held, IssueNone
		}
		_ = file.Close()
	}
	held.close()
	return nil, IssueInvalidPath
}

func inspectTraceDirectoryBinding(parent *os.Root, name string) (os.FileInfo, FileIdentity, IssueCode) {
	info, identity, err := traceDirectoryLstat(parent, name)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, FileIdentity{}, IssueSourceUnavailable
		}
		return nil, FileIdentity{}, IssueSourceUnreadable
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, FileIdentity{}, IssueSourceSymlink
	}
	if !info.IsDir() {
		return nil, FileIdentity{}, IssueSourceNotRegular
	}
	return info, identity, IssueNone
}

// openSourceLeaf opens a configured source without blocking, so a FIFO or
// device swapped in after the component preflight reaches the descriptor
// regular-file check instead of hanging the open.
func openSourceLeaf(root *os.Root, relativePath string) (*os.File, error) {
	return root.OpenFile(relativePath, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
}

func (directory *heldTraceDirectory) openMember(name string) (*os.File, error) {
	return directory.root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
}

func traceDirectoryLstat(parent *os.Root, name string) (os.FileInfo, FileIdentity, error) {
	info, err := parent.Lstat(name)
	if err != nil {
		return nil, FileIdentity{}, err
	}
	identity, qualified := qualifiedIdentity(info)
	if !qualified {
		return nil, FileIdentity{}, os.ErrInvalid
	}
	return info, identity, nil
}

func traceDirectoryBindingLstat(binding traceDirectoryBinding) (os.FileInfo, FileIdentity, bool, error) {
	info, identity, err := traceDirectoryLstat(binding.parent, binding.name)
	return info, identity, info != nil && info.Mode()&os.ModeSymlink != 0, err
}

func traceDirectoryChildLstat(directory *heldTraceDirectory, name string) (os.FileInfo, FileIdentity, bool, error) {
	info, identity, err := traceDirectoryLstat(directory.root, name)
	return info, identity, info != nil && info.Mode()&os.ModeSymlink != 0, err
}

func traceDescriptorPath(descriptor uintptr) string {
	if runtime.GOOS == "linux" {
		return fmt.Sprintf("/proc/self/fd/%d", descriptor)
	}
	return fmt.Sprintf("/dev/fd/%d", descriptor)
}
