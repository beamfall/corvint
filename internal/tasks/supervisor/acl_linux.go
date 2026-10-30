//go:build linux

package supervisor

import "syscall"

// hasACL reports whether path carries a POSIX or NFSv4 access control list,
// which can grant write access that the mode bits do not show. An attribute
// that is absent, or a filesystem without extended attributes, reports
// false; any other failure reports true, so a caller fails closed
// (CAL-V0-074). path is never a symlink here: callers check that first.
func hasACL(path string) bool {
	for _, name := range []string{"system.posix_acl_access", "system.posix_acl_default", "system.nfs4_acl", "system.richacl"} {
		_, e := syscall.Getxattr(path, name, nil)
		if e == nil {
			return true
		}
		if e != syscall.ENODATA && e != syscall.ENOTSUP && e != syscall.EOPNOTSUPP {
			return true
		}
	}
	return false
}
