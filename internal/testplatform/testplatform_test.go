package testplatform

import (
	"runtime"
	"strings"
	"testing"
)

func TestExeSuffixIsOnlyAppendedOnWindows(t *testing.T) {
	if got := exeSuffix("windows"); got != ".exe" {
		t.Fatalf("windows suffix = %q, want .exe", got)
	}
	for _, goos := range []string{"darwin", "linux", "freebsd"} {
		if got := exeSuffix(goos); got != "" {
			t.Fatalf("%s suffix = %q, want empty", goos, got)
		}
	}
	if runtime.GOOS != "windows" && Executable("spy") != "spy" {
		t.Fatalf("Executable(spy) = %q on %s, want spy", Executable("spy"), runtime.GOOS)
	}
}

func TestPOSIXShellSkipOnlyAppliesOnWindows(t *testing.T) {
	for _, goos := range []string{"darwin", "linux"} {
		if message, skip := posixShellSkip(goos, "spy"); skip || message != "" {
			t.Fatalf("%s: skip=%v message=%q, want no skip", goos, skip, message)
		}
	}
	message, skip := posixShellSkip("windows", "a #!/bin/sh git spy")
	if !skip || !strings.Contains(message, "a #!/bin/sh git spy") {
		t.Fatalf("windows: skip=%v message=%q, want a skip that names the reason", skip, message)
	}
	if runtime.GOOS != "windows" {
		SkipPOSIXShell(t, "must not skip here")
	}
}
