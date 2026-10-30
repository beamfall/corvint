//go:build !unix

package gitauth

import "os"

func openAuthorityLeaf(*os.Root, string) (*os.File, error) {
	return nil, unavailable("authority native snapshot unavailable")
}
