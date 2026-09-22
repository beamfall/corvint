//go:build darwin || linux

package sqlnative

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"syscall"
)

type heldIdentity struct {
	fd     int
	before fileIdentity
}

var newReadFile = os.NewFile

func ReadNoFollow(ctx context.Context, root *os.File, path string, limit int) ([]byte, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if root == nil || limit <= 0 || limit > maxInputBytes || !validPath(path, "sqlite.migration") && !validPath(path, "sqlite.query") {
		return nil, errors.New("invalid bounded input path")
	}
	fd, err := syscall.Dup(int(root.Fd()))
	if err != nil {
		return nil, err
	}
	if err := contextError(ctx); err != nil {
		_ = syscall.Close(fd)
		return nil, err
	}
	var st syscall.Stat_t
	if err := syscall.Fstat(fd, &st); err != nil || !directory(st) {
		_ = syscall.Close(fd)
		if err != nil {
			return nil, err
		}
		return nil, errors.New("unsafe authority identity")
	}
	held := []heldIdentity{{fd, identity(st)}}
	defer func() {
		for i := len(held) - 1; i >= 0; i-- {
			_ = syscall.Close(held[i].fd)
		}
	}()
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		parent := held[len(held)-1]
		if err := syscall.Fstat(fd, &st); err != nil || identity(st) != parent.before || !directory(st) {
			if err != nil {
				return nil, err
			}
			return nil, errors.New("parent identity changed")
		}
		flags := syscall.O_RDONLY | syscall.O_CLOEXEC | syscall.O_NOFOLLOW
		if i != len(parts)-1 {
			flags |= syscall.O_DIRECTORY
		}
		next, err := openAt(fd, part, flags, 0)
		if err != nil {
			return nil, err
		}
		if err := contextError(ctx); err != nil {
			_ = syscall.Close(next)
			return nil, err
		}
		if err := syscall.Fstat(next, &st); err != nil {
			_ = syscall.Close(next)
			return nil, err
		}
		if i != len(parts)-1 && !directory(st) || i == len(parts)-1 && !regular(st) {
			_ = syscall.Close(next)
			return nil, errors.New("unsafe input identity")
		}
		held = append(held, heldIdentity{next, identity(st)})
		fd = next
	}
	before := held[len(held)-1].before
	if before.size < 0 || before.size > int64(limit) {
		return nil, errors.New("input bound exceeded")
	}
	readFD, err := syscall.Dup(fd)
	if err != nil {
		return nil, err
	}
	file := newReadFile(uintptr(readFD), path)
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = file.Close()
		case <-done:
		}
	}()
	defer close(done)
	defer file.Close()
	data := make([]byte, int(before.size))
	for offset := 0; offset < len(data); {
		n, err := file.Read(data[offset:])
		if err != nil {
			if contextError(ctx) != nil {
				return nil, ctx.Err()
			}
			return nil, err
		}
		if n == 0 {
			return nil, io.ErrUnexpectedEOF
		}
		offset += n
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	var extra [1]byte
	if n, err := file.Read(extra[:]); err != nil && err != io.EOF || n != 0 {
		if contextError(ctx) != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("input bound exceeded")
	}
	for _, item := range held {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		if err := syscall.Fstat(item.fd, &st); err != nil || identity(st) != item.before {
			if err != nil {
				return nil, err
			}
			return nil, errors.New("input identity changed")
		}
	}
	return data, nil
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return errors.New("nil context")
	}
	return ctx.Err()
}
