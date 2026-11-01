// Command run-measure is the portable wall-time and peak-RSS helper for lane hosts that lack
// /usr/bin/time or bc (docs/specs/corvint-self-development-v0.md, ASD-V0-570; V1-3513). It runs
// one command with the caller's stdin, stdout, stderr and environment, then prints one line to
// stderr:
//
//	run-measure: wall=12.345s peak_rss_kib=51200 rc=0 cmd=go vet ./...
//
// peak_rss_kib is the child's maximum resident set size in KiB from getrusage, normalized across
// Linux (KiB) and macOS (bytes), or `unknown` where the platform reports none. The helper exits
// with the command's exit code (128+N for a signal), or 127 when the command cannot start.
//
// usage: go run ./tools/run-measure [--] COMMAND [ARG...]
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: go run ./tools/run-measure [--] COMMAND [ARG...]")
		return 2
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
	start := time.Now()
	err := cmd.Run()
	wall := time.Since(start)
	rc := 0
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		rc = exitErr.ExitCode()
		if rc < 0 {
			rc = 128 + signalNumber(exitErr.ProcessState)
		}
	default:
		fmt.Fprintf(stderr, "run-measure: cannot start %s: %v\n", args[0], err)
		return 127
	}
	rss := "unknown"
	if kib, ok := peakRSSKiB(cmd.ProcessState); ok {
		rss = fmt.Sprint(kib)
	}
	fmt.Fprintf(stderr, "run-measure: wall=%.3fs peak_rss_kib=%s rc=%d cmd=%s\n", wall.Seconds(), rss, rc, strings.Join(args, " "))
	return rc
}
