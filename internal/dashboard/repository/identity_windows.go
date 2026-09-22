//go:build windows

package repository

import (
	"os"
	"syscall"
)

func identityFromFile(file *os.File, info os.FileInfo) (fileIdentity, bool) {
	if file == nil || info == nil || info.Size() < 0 {
		return fileIdentity{}, false
	}
	var native syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(syscall.Handle(file.Fd()), &native); err != nil {
		return fileIdentity{}, false
	}
	return fileIdentity{
		mode: uint32(info.Mode()), modTimeNanoseconds: info.ModTime().UnixNano(), size: uint64(info.Size()),
		platformKind: 2, linkCount: uint64(native.NumberOfLinks),
		fileIndex:    (uint64(native.FileIndexHigh) << 32) | uint64(native.FileIndexLow),
		volumeSerial: uint64(native.VolumeSerialNumber),
	}, true
}

// directoryIdentityFromFile carries no owner or group: the by-handle file
// information has none, and security descriptors are outside this identity.
func directoryIdentityFromFile(file *os.File, info os.FileInfo) (directoryIdentity, bool) {
	identity, ok := identityFromFile(file, info)
	if !ok {
		return directoryIdentity{}, false
	}
	return directoryIdentity{
		mode: identity.mode, platformKind: identity.platformKind,
		fileIndex: identity.fileIndex, volumeSerial: identity.volumeSerial,
	}, true
}
