//go:build windows

package source

import (
	"os"
	"syscall"
)

func qualifiedDescriptorIdentity(file *os.File, info os.FileInfo) (FileIdentity, bool) {
	identity, reparse, qualified := windowsDescriptorIdentity(file, info)
	return identity, qualified && !reparse
}

func descriptorReparse(file *os.File) (bool, bool) {
	if file == nil {
		return false, false
	}
	var handleInfo syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(syscall.Handle(file.Fd()), &handleInfo); err != nil {
		return false, false
	}
	return windowsFileAttributesReparse(handleInfo.FileAttributes), true
}

func windowsDescriptorIdentity(file *os.File, info os.FileInfo) (FileIdentity, bool, bool) {
	if file == nil || info == nil || info.Size() < 0 {
		return FileIdentity{}, false, false
	}
	var handleInfo syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(syscall.Handle(file.Fd()), &handleInfo); err != nil {
		return FileIdentity{}, false, false
	}
	reparse := windowsFileAttributesReparse(handleInfo.FileAttributes)
	platform, qualified := windowsPlatformIdentity(
		handleInfo.VolumeSerialNumber,
		handleInfo.NumberOfLinks,
		handleInfo.FileIndexHigh,
		handleInfo.FileIndexLow,
	)
	if !qualified {
		return FileIdentity{}, reparse, false
	}
	return FileIdentity{
		mode:               uint32(info.Mode()),
		modTimeNanoseconds: info.ModTime().UnixNano(),
		platform:           platform,
		size:               uint64(info.Size()),
	}, reparse, true
}
