//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package parentverify

import "os"

func fileLinkCount(os.FileInfo) uint64 { return 1 }
