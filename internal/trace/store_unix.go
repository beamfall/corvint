//go:build darwin || linux

package trace

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
)

func openNoFollowDirectory(parent *os.Root, name string) (*os.File, *os.Root, error) {
	file, err := parent.OpenFile(name, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	prefix := "/dev/fd"
	if runtime.GOOS == "linux" {
		prefix = "/proc/self/fd"
	}
	root, err := os.OpenRoot(fmt.Sprintf("%s/%d", prefix, file.Fd()))
	if err != nil {
		file.Close()
		return nil, nil, err
	}
	return file, root, nil
}

func openNoFollowMember(parent *os.Root, name string, flags int, mode os.FileMode) (*os.File, error) {
	return parent.OpenFile(name, flags|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, mode)
}

func descriptorLinks(info os.FileInfo) uint64 {
	metadata, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0
	}
	return uint64(metadata.Nlink)
}

func lockDescriptor(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX)
}

func unlockDescriptor(file *os.File) {
	_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
