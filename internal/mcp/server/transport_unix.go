//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package server

import (
	"context"
	"errors"
	"io"
	"os"
	"syscall"
	"time"
)

const transportPollInterval = 10 * time.Millisecond

type contextFileReader struct {
	ctx  context.Context
	file *os.File
}

func interruptibleReader(ctx context.Context, input io.Reader) io.Reader {
	file, ok := input.(*os.File)
	if !ok {
		return input
	}
	return &contextFileReader{ctx: ctx, file: file}
}

// Unix file transports are put into nonblocking mode and poll the serving
// context. Closing one concurrently is both unnecessary and unsafe: it races
// with descriptor access and can let a reused descriptor enter the read loop.
func requiresExternalCancellation(transport any) bool {
	_, isFile := transport.(*os.File)
	return !isFile
}

// preserveBlockingMode records a file transport's O_NONBLOCK flag without
// changing it and returns a function that puts the flag back. Inherited stdio
// shares its open file description with the parent process, so the
// nonblocking mode used while serving must not outlive Serve.
func preserveBlockingMode(transport any) func() {
	file, ok := transport.(*os.File)
	if !ok {
		return func() {}
	}
	raw, err := file.SyscallConn()
	if err != nil {
		return func() {}
	}
	var flags uintptr
	var errno syscall.Errno
	if err := raw.Control(func(fd uintptr) {
		flags, _, errno = syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_GETFL, 0)
	}); err != nil || errno != 0 {
		return func() {}
	}
	nonblocking := flags&syscall.O_NONBLOCK != 0
	return func() {
		_ = raw.Control(func(fd uintptr) { _ = syscall.SetNonblock(int(fd), nonblocking) })
	}
}

func (reader *contextFileReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	fd := int(reader.file.Fd())
	if err := syscall.SetNonblock(fd, true); err != nil {
		return 0, err
	}
	for {
		read, err := syscall.Read(fd, buffer)
		if read > 0 {
			return read, nil
		}
		if err == nil {
			return 0, io.EOF
		}
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if !errors.Is(err, syscall.EAGAIN) && !errors.Is(err, syscall.EWOULDBLOCK) {
			return 0, err
		}
		if err := waitForTransport(reader.ctx); err != nil {
			return 0, err
		}
	}
}

func interruptibleWrite(ctx context.Context, output io.Writer, buffer []byte) (int, error) {
	file, ok := output.(*os.File)
	if !ok {
		return output.Write(buffer)
	}
	fd := int(file.Fd())
	if err := syscall.SetNonblock(fd, true); err != nil {
		return 0, err
	}
	written := 0
	for written < len(buffer) {
		count, err := syscall.Write(fd, buffer[written:])
		// A failed write reports -1; only bytes the kernel accepted advance.
		written += max(count, 0)
		if err == nil {
			continue
		}
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if !errors.Is(err, syscall.EAGAIN) && !errors.Is(err, syscall.EWOULDBLOCK) {
			return written, err
		}
		if err := waitForTransport(ctx); err != nil {
			return written, err
		}
	}
	return written, nil
}

func waitForTransport(ctx context.Context) error {
	timer := time.NewTimer(transportPollInterval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
