//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

func safeAttachmentFile(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && uint64(stat.Nlink) == 1 && uint32(stat.Uid) == uint32(os.Geteuid())
}

func supportedProviderHost() bool { return supportedProviderPlatform(runtime.GOOS, runtime.GOARCH) }

func supportedProviderPlatform(goos, goarch string) bool {
	return goos == "darwin" && goarch == "arm64"
}

func providerContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func readParentCapability() ([]byte, bool) {
	var stat syscall.Stat_t
	if syscall.Fstat(3, &stat) != nil || stat.Mode&syscall.S_IFMT != syscall.S_IFIFO {
		return nil, false
	}
	defer syscall.Close(3)
	// The parent preloads and closes the pipe before exec; any wait is invalid.
	// Raw nonblocking reads also refuse a runtime pipe that reused an absent fd 3.
	if syscall.SetNonblock(3, true) != nil {
		return nil, false
	}
	var value [33]byte
	for size := 0; size < len(value); {
		count, err := syscall.Read(3, value[size:])
		if err != nil {
			return nil, false
		}
		if count == 0 {
			return value[:size], size == 32
		}
		size += count
	}
	return nil, false
}

func runAuthorityCommand(ctx context.Context, executable string, argv, environment []string, cwd string, timeout time.Duration, capability []byte) (directCommandResult, error) {
	if len(capability) != 32 {
		return directCommandResult{ExitCode: -1}, errDirectCommandInvalid
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		return directCommandResult{ExitCode: -1}, errDirectCommandPipe
	}
	if _, err := writer.Write(capability); err != nil {
		_ = reader.Close()
		_ = writer.Close()
		return directCommandResult{ExitCode: -1}, errDirectCommandPipe
	}
	if err := writer.Close(); err != nil {
		_ = reader.Close()
		return directCommandResult{ExitCode: -1}, errDirectCommandPipe
	}
	result, runErr := runDirectCommandWithFiles(ctx, executable, argv, environment, cwd, timeout, []*os.File{reader})
	closeErr := reader.Close()
	return result, errors.Join(runErr, closeErr)
}
