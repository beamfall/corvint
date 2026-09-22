//go:build windows

package gitauth

import "os"

// Windows has no O_NOFOLLOW; the pre-open Lstat regular-file check plus the
// post-open SameFile revalidation carry the no-follow guarantee here.
func openNoFollow(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY, 0)
}
