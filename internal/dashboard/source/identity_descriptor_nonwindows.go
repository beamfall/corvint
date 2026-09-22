//go:build !windows

package source

import "os"

func qualifiedDescriptorIdentity(_ *os.File, info os.FileInfo) (FileIdentity, bool) {
	return qualifiedIdentity(info)
}

func descriptorReparse(_ *os.File) (bool, bool) {
	return false, true
}
