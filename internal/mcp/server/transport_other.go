//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package server

import (
	"context"
	"io"
)

func interruptibleReader(_ context.Context, input io.Reader) io.Reader { return input }

func requiresExternalCancellation(any) bool { return true }

func preserveBlockingMode(any) func() { return func() {} }

func interruptibleWrite(_ context.Context, output io.Writer, buffer []byte) (int, error) {
	return output.Write(buffer)
}
