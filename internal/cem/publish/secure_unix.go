//go:build darwin || linux

package publish

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"time"
)

func openSecureDirectory(parent *os.Root, name string) (*os.File, *os.Root, error) {
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

func openSecureMember(parent *os.Root, name string, flags int, mode os.FileMode) (*os.File, error) {
	return parent.OpenFile(name, flags|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, mode)
}

func lockSecureDirectory(ctx context.Context, file *os.File) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	for {
		err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			return err
		}
		retry := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			retry.Stop()
			return ctx.Err()
		case <-deadline.C:
			retry.Stop()
			return errors.New("secure publication lock timed out")
		case <-retry.C:
		}
	}
}

func unlockSecureDirectory(file *os.File) {
	_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
