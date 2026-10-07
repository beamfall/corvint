//go:build darwin || linux

package affected

import (
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
)

// readSourceFile opens the file no-follow and non-blocking and checks its mode
// and size on the open descriptor: one path resolution per file instead of
// the Lstat-then-Open pair, which on a 200,000-file checkout was 19% of
// affected's CPU (3.7 of 19.7 s, V1-0416). A symlink, directory or FIFO is
// refused exactly as the Lstat refused it, and a FIFO cannot block the open.
// The body is read against the bound, not the stat size, so a file that
// grows past MaxSourceBytes after the stat is refused rather than truncated.
func readSourceFile(full, relative string) ([]byte, error) {
	file, err := os.OpenFile(full, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		if errors.Is(err, syscall.ELOOP) || errors.Is(err, syscall.EMLINK) {
			return nil, fmt.Errorf("%w: %q is not a regular file", ErrInvalidUnit, relative)
		}
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %q is not a regular file", ErrInvalidUnit, relative)
	}
	if info.Size() > MaxSourceBytes {
		return nil, fmt.Errorf("%w: %q is %d bytes", ErrWalkLimit, relative, info.Size())
	}
	body := make([]byte, 0, info.Size()+1)
	for {
		if len(body) == cap(body) {
			body = append(body, 0)[:len(body)]
		}
		read, err := file.Read(body[len(body):cap(body)])
		body = body[:len(body)+read]
		if err == io.EOF {
			return body, nil
		}
		if err != nil {
			return nil, err
		}
		if len(body) > MaxSourceBytes {
			return nil, fmt.Errorf("%w: %q is over %d bytes", ErrWalkLimit, relative, MaxSourceBytes)
		}
	}
}
