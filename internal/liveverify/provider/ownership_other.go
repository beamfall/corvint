//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package provider

import "os"

func ownedByCurrentUser(_ os.FileInfo) bool { return false }

func supportedPlatform() bool { return false }
