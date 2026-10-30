//go:build darwin || linux

package repository

import (
	"os"
	"reflect"
)

func identityFromFile(file *os.File, info os.FileInfo) (fileIdentity, bool) {
	if file == nil || info == nil || info.Size() < 0 || info.Sys() == nil {
		return fileIdentity{}, false
	}
	value := reflect.ValueOf(info.Sys())
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return fileIdentity{}, false
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return fileIdentity{}, false
	}
	device, deviceOK := identityUnsignedField(value, "Dev")
	inode, inodeOK := identityUnsignedField(value, "Ino")
	links, linksOK := identityUnsignedField(value, "Nlink")
	if !deviceOK || !inodeOK || !linksOK {
		return fileIdentity{}, false
	}
	return fileIdentity{
		mode: uint32(info.Mode()), modTimeNanoseconds: info.ModTime().UnixNano(), size: uint64(info.Size()),
		platformKind: 1, device: device, inode: inode, linkCount: links,
	}, true
}

func directoryIdentityFromFile(file *os.File, info os.FileInfo) (directoryIdentity, bool) {
	identity, ok := identityFromFile(file, info)
	if !ok {
		return directoryIdentity{}, false
	}
	value := reflect.Indirect(reflect.ValueOf(info.Sys()))
	owner, ownerOK := identityUnsignedField(value, "Uid")
	group, groupOK := identityUnsignedField(value, "Gid")
	if !ownerOK || !groupOK {
		return directoryIdentity{}, false
	}
	return directoryIdentity{
		mode: identity.mode, platformKind: identity.platformKind,
		device: identity.device, inode: identity.inode, owner: owner, group: group,
	}, true
}

func identityUnsignedField(value reflect.Value, name string) (uint64, bool) {
	field := value.FieldByName(name)
	if !field.IsValid() {
		return 0, false
	}
	switch field.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return field.Uint(), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if field.Int() < 0 {
			return 0, false
		}
		return uint64(field.Int()), true
	default:
		return 0, false
	}
}
