// Package testplatform holds the test-only helpers that make a test's own
// harness portable to native Windows without changing what the test proves
// (V1-2490). It imports only the standard library, so any package's tests can
// use it without an import cycle.
package testplatform

import (
	"runtime"
	"testing"
)

// ExeSuffix is the executable file suffix on the host: ".exe" on Windows and
// empty elsewhere. A test that builds a Go spy or helper with `go build -o`
// appends it, so the binary it then runs or puts on PATH can be found.
func ExeSuffix() string { return exeSuffix(runtime.GOOS) }

func exeSuffix(goos string) string {
	if goos == "windows" {
		return ".exe"
	}
	return ""
}

// Executable returns name with the host executable suffix appended.
func Executable(name string) string { return name + ExeSuffix() }

// SkipPOSIXShell skips t on Windows, where the test depends on a POSIX shell
// script spy or a POSIX tool that the host does not provide. The reason names
// what the test needs, so the skip is visible and explained. It does nothing on
// other platforms.
func SkipPOSIXShell(t testing.TB, reason string) {
	t.Helper()
	if message, skip := posixShellSkip(runtime.GOOS, reason); skip {
		t.Skip(message)
	}
}

func posixShellSkip(goos, reason string) (string, bool) {
	if goos != "windows" {
		return "", false
	}
	if reason == "" {
		reason = "the test runs a POSIX shell script"
	}
	return "requires a POSIX shell on this platform: " + reason, true
}
