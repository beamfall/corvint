//go:build !unix

package main

import (
	"fmt"
	"io"
)

// runLive needs POSIX process groups to observe cancellation and cleanup (HLQ-V1-012).
func runLive(host, hostExecutable, current, source, report string, stdout, stderr io.Writer) int {
	fmt.Fprintln(stderr, "host-lifecycle-v1: live mode needs a POSIX host")
	return 2
}
