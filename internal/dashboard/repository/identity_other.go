//go:build !darwin && !linux && !windows

package repository

import "os"

func identityFromFile(*os.File, os.FileInfo) (fileIdentity, bool) {
	return fileIdentity{}, false
}

func directoryIdentityFromFile(*os.File, os.FileInfo) (directoryIdentity, bool) {
	return directoryIdentity{}, false
}
