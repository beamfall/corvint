//go:build windows

package source

import (
	"os"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

const (
	windowsOpenReparsePoint        = 0x00200000
	windowsFileGenericRead         = 0x00120089
	windowsFileShareAll            = 0x00000007
	windowsFileOpen                = 0x00000001
	windowsFileDirectory           = 0x00000001
	windowsFileSynchronousNonalert = 0x00000020
	windowsObjectCaseInsensitive   = 0x00000040
)

type windowsUnicodeString struct {
	Length        uint16
	MaximumLength uint16
	Buffer        *uint16
}

type windowsObjectAttributes struct {
	Length                   uint32
	RootDirectory            syscall.Handle
	ObjectName               *windowsUnicodeString
	Attributes               uint32
	SecurityDescriptor       unsafe.Pointer
	SecurityQualityOfService unsafe.Pointer
}

type windowsIOStatusBlock struct {
	Status      uintptr
	Information uintptr
}

var windowsNtCreateFile = syscall.NewLazyDLL("ntdll.dll").NewProc("NtCreateFile")

func openHeldTraceDirectory(root *os.Root, relativeDirectory string) (*heldTraceDirectory, IssueCode) {
	parts := strings.Split(relativeDirectory, "/")
	held := &heldTraceDirectory{}
	parentFile, err := root.Open(".")
	if err != nil {
		return nil, IssueSourceUnreadable
	}
	held.ownedFiles = append(held.ownedFiles, parentFile)
	for index, part := range parts {
		file, err := openWindowsRelative(parentFile, part, true)
		if err != nil {
			held.close()
			if os.IsNotExist(err) {
				return nil, IssueSourceUnavailable
			}
			if os.IsPermission(err) {
				return nil, IssueSourceUnreadable
			}
			return nil, IssueSourceUnstable
		}
		info, err := file.Stat()
		identity, reparse, qualified := windowsDescriptorIdentity(file, info)
		if err != nil || !qualified {
			_ = file.Close()
			held.close()
			return nil, IssueSourceUnstable
		}
		if reparse || info.Mode()&os.ModeSymlink != 0 {
			_ = file.Close()
			held.close()
			return nil, IssueSourceSymlink
		}
		if !info.IsDir() {
			_ = file.Close()
			held.close()
			return nil, IssueSourceNotRegular
		}
		held.bindings = append(held.bindings, traceDirectoryBinding{parentFile: parentFile, name: part, identity: identity})
		if index == len(parts)-1 {
			held.file = file
			return held, IssueNone
		}
		held.ownedFiles = append(held.ownedFiles, file)
		parentFile = file
	}
	held.close()
	return nil, IssueInvalidPath
}

func openSourceLeaf(root *os.Root, relativePath string) (*os.File, error) {
	return root.Open(relativePath)
}

func (directory *heldTraceDirectory) openMember(name string) (*os.File, error) {
	return openWindowsRelative(directory.file, name, false)
}

func traceDirectoryBindingLstat(binding traceDirectoryBinding) (os.FileInfo, FileIdentity, bool, error) {
	var file *os.File
	var err error
	file, err = openWindowsRelative(binding.parentFile, binding.name, true)
	if err != nil {
		return nil, FileIdentity{}, false, err
	}
	defer file.Close()
	return windowsFileIdentity(file)
}

func traceDirectoryChildLstat(directory *heldTraceDirectory, name string) (os.FileInfo, FileIdentity, bool, error) {
	file, err := openWindowsRelative(directory.file, name, false)
	if err != nil {
		return nil, FileIdentity{}, false, err
	}
	defer file.Close()
	return windowsFileIdentity(file)
}

func windowsFileIdentity(file *os.File) (os.FileInfo, FileIdentity, bool, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, FileIdentity{}, false, err
	}
	identity, reparse, qualified := windowsDescriptorIdentity(file, info)
	if !qualified {
		return nil, FileIdentity{}, reparse, os.ErrInvalid
	}
	return info, identity, reparse, nil
}

func openWindowsRelative(parent *os.File, name string, directory bool) (*os.File, error) {
	encoded, err := syscall.UTF16FromString(name)
	if err != nil || len(encoded) < 1 || len(encoded) > 32767 {
		return nil, os.ErrInvalid
	}
	objectName := windowsUnicodeString{
		Length: uint16((len(encoded) - 1) * 2), MaximumLength: uint16(len(encoded) * 2), Buffer: &encoded[0],
	}
	attributes := windowsObjectAttributes{
		RootDirectory: syscall.Handle(parent.Fd()), ObjectName: &objectName, Attributes: windowsObjectCaseInsensitive,
	}
	attributes.Length = uint32(unsafe.Sizeof(attributes))
	options := uintptr(windowsFileSynchronousNonalert | windowsOpenReparsePoint)
	if directory {
		options |= windowsFileDirectory
	}
	var handle syscall.Handle
	statusBlock := windowsIOStatusBlock{}
	status, _, _ := windowsNtCreateFile.Call(
		uintptr(unsafe.Pointer(&handle)), windowsFileGenericRead, uintptr(unsafe.Pointer(&attributes)),
		uintptr(unsafe.Pointer(&statusBlock)), 0, syscall.FILE_ATTRIBUTE_NORMAL,
		windowsFileShareAll, windowsFileOpen, options, 0, 0,
	)
	runtime.KeepAlive(encoded)
	runtime.KeepAlive(parent)
	if status != 0 {
		return nil, syscall.Errno(status)
	}
	return os.NewFile(uintptr(handle), name), nil
}
