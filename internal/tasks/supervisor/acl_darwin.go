//go:build darwin

package supervisor

import (
	"encoding/binary"
	"syscall"
	"unsafe"
)

// hasACL reports whether path itself (not a symlink target) carries an
// access control list with any entry. macOS evaluates ACL entries before the
// owner and mode bits, so an entry can grant write access that the mode does
// not show. Any failure to read the attribute reports true, so a caller
// fails closed (CAL-V0-074).
func hasACL(path string) bool {
	const (
		bitmapCount        = 5
		returnedAttrs      = 0x80000000 // ATTR_CMN_RETURNED_ATTRS
		extendedSecurity   = 0x00400000 // ATTR_CMN_EXTENDED_SECURITY
		noFollow           = 0x1        // FSOPT_NOFOLLOW
		header             = 4 + 20     // length, then the returned attribute set
		filesecEntryCount  = 4 + 16 + 16
		filesecNoACL       = 0xffffffff // KAUTH_FILESEC_NOACL
		attributeReference = 8
	)
	request := struct {
		count, reserved                 uint16
		common, volume, dir, file, fork uint32
	}{count: bitmapCount, common: returnedAttrs | extendedSecurity}
	p, e := syscall.BytePtrFromString(path)
	if e != nil {
		return true
	}
	buf := make([]byte, 64<<10)
	if _, _, errno := syscall.Syscall6(syscall.SYS_GETATTRLIST, uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&request)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), noFollow, 0); errno != 0 {
		return true
	}
	n := binary.LittleEndian.Uint32(buf)
	if n < header || int(n) > len(buf) {
		return true
	}
	if binary.LittleEndian.Uint32(buf[4:])&extendedSecurity == 0 {
		return false
	}
	if n < header+attributeReference {
		return true
	}
	ref := buf[header:]
	offset, length := int32(binary.LittleEndian.Uint32(ref)), binary.LittleEndian.Uint32(ref[4:])
	if length == 0 {
		return false
	}
	start := int64(header) + int64(offset)
	if offset < 0 || length < filesecEntryCount+4 || start+int64(length) > int64(n) {
		return true
	}
	count := binary.LittleEndian.Uint32(buf[start+filesecEntryCount:])
	return count != 0 && count != filesecNoACL
}
