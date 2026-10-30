//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package source

import "os"

func openHeldTraceDirectory(root *os.Root, relativeDirectory string) (*heldTraceDirectory, IssueCode) {
	return nil, issuePlatformIdentityUnsupported
}

func openSourceLeaf(root *os.Root, relativePath string) (*os.File, error) {
	return root.Open(relativePath)
}

func (directory *heldTraceDirectory) openMember(name string) (*os.File, error) {
	return nil, os.ErrInvalid
}

func traceDirectoryLstat(parent *os.Root, name string) (os.FileInfo, FileIdentity, error) {
	return nil, FileIdentity{}, os.ErrInvalid
}

func traceDirectoryBindingLstat(binding traceDirectoryBinding) (os.FileInfo, FileIdentity, bool, error) {
	return nil, FileIdentity{}, false, os.ErrInvalid
}

func traceDirectoryChildLstat(directory *heldTraceDirectory, name string) (os.FileInfo, FileIdentity, bool, error) {
	return nil, FileIdentity{}, false, os.ErrInvalid
}
