//go:build darwin || linux

package analyzerswift

import (
	"bytes"
	"crypto/sha256"
	"io"
	"os"
	"syscall"
)

type descriptorHooks struct {
	afterBefore func()
	afterOpen   func()
}

func readDescriptor(path string) ([]byte, error) {
	return readDescriptorWithHooks(path, descriptorHooks{})
}

func readDescriptorWithHooks(path string, hooks descriptorHooks) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil || !safeDescriptorInfo(before) {
		return nil, errDescriptorUnsafe
	}
	if hooks.afterBefore != nil {
		hooks.afterBefore()
	}
	descriptor, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, errDescriptorUnsafe
	}
	file := os.NewFile(uintptr(descriptor), path)
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !sameDescriptor(before, opened) {
		return nil, errDescriptorUnsafe
	}
	if hooks.afterOpen != nil {
		hooks.afterOpen()
	}
	value, digest, err := readDescriptorBytes(file, before.Size())
	if err != nil {
		return nil, errDescriptorUnsafe
	}
	afterRead, err := file.Stat()
	if err != nil || !sameDescriptor(before, afterRead) {
		return nil, errDescriptorUnsafe
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, errDescriptorUnsafe
	}
	again, err := hashDescriptorBytes(file, before.Size())
	if err != nil || !bytes.Equal(digest, again) {
		return nil, errDescriptorUnsafe
	}
	afterPath, err := os.Lstat(path)
	if err != nil || !sameDescriptor(before, afterPath) {
		return nil, errDescriptorUnsafe
	}
	return value, nil
}

func safeDescriptorInfo(info os.FileInfo) bool {
	return info.Mode().IsRegular() && info.Size() >= 2 && info.Size() <= maxWire
}

func sameDescriptor(left, right os.FileInfo) bool {
	return safeDescriptorInfo(left) && safeDescriptorInfo(right) && os.SameFile(left, right) &&
		left.Mode() == right.Mode() && left.Size() == right.Size() && left.ModTime() == right.ModTime()
}

func readDescriptorBytes(file *os.File, size int64) ([]byte, []byte, error) {
	value := make([]byte, size)
	if _, err := io.ReadFull(file, value); err != nil {
		return nil, nil, err
	}
	var extra [1]byte
	if count, err := file.Read(extra[:]); err != io.EOF || count != 0 {
		return nil, nil, errDescriptorUnsafe
	}
	digest := sha256.Sum256(value)
	return value, digest[:], nil
}

func hashDescriptorBytes(file *os.File, size int64) ([]byte, error) {
	hash := sha256.New()
	if _, err := io.CopyN(hash, file, size); err != nil {
		return nil, err
	}
	var extra [1]byte
	if count, err := file.Read(extra[:]); err != io.EOF || count != 0 {
		return nil, errDescriptorUnsafe
	}
	return hash.Sum(nil), nil
}
